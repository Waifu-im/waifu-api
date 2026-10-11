#!/usr/bin/env python3
"""Upload mirrored images to any S3-compatible bucket via SigV4.

Tested against IDrive e2, Backblaze B2, Cloudflare R2 and Tigris. Set the
S3_* values in .env; nothing else is provider-specific.

Reads the id/extension list from waifu-public.sql, so it uploads exactly the
files download.py produced - no drift possible. Object keys are {id}.{ext},
which is what CdnUrlHelper.GetImageUrl computes from Cdn__BaseUrl.

Resumable: a key already present in the bucket is skipped. Stdlib only.
"""

import argparse
import collections
import hashlib
import hmac
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from pathlib import Path

from loadenv import die, load_env

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1] if HERE.parent.name == "mirror" else HERE.parent


def _find(name):
    """Search up from this file for a data file that lives outside the repo."""
    for base in HERE.parents:
        hit = base / name
        if hit.exists():
            return hit
    return REPO / name
DEFAULT_SQL = _find("waifu-public.sql")
DEFAULT_IMAGES = _find("image downloader/images")

ALGORITHM = "AWS4-HMAC-SHA256"
SERVICE = "s3"

CONTENT_TYPES = {
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".png": "image/png",
    ".gif": "image/gif",
    ".webp": "image/webp",
}

lock = threading.Lock()
uploaded = 0
skipped = 0
failed = collections.Counter()
bytes_sent = 0
errors = []
START = 0.0


def parse_images(sql_path):
    """Yield (id, extension) from the Images COPY block in the pg_dump."""
    in_block = False
    with open(sql_path, "r", encoding="utf-8", errors="replace") as fh:
        for line in fh:
            if line.startswith('COPY public."Images"'):
                in_block = True
                continue
            if not in_block:
                continue
            if line.startswith("\\."):
                return
            parts = line.rstrip("\n").split("\t")
            if len(parts) < 3:
                continue
            try:
                yield int(parts[0]), parts[2]
            except ValueError:
                continue


def sign(key, msg):
    return hmac.new(key, msg.encode("utf-8"), hashlib.sha256).digest()


def sigv4_headers(method, host, path, payload_hash, region, access_key, secret_key):
    """Return the auth headers for one S3 request. Standard AWS SigV4."""
    now = datetime.now(timezone.utc)
    amz_date = now.strftime("%Y%m%dT%H%M%SZ")
    date_stamp = now.strftime("%Y%m%d")

    signed_headers = "host;x-amz-content-sha256;x-amz-date"
    canonical_headers = (
        f"host:{host}\n"
        f"x-amz-content-sha256:{payload_hash}\n"
        f"x-amz-date:{amz_date}\n"
    )
    canonical_request = "\n".join(
        [method, path, "", canonical_headers, signed_headers, payload_hash]
    )

    scope = f"{date_stamp}/{region}/{SERVICE}/aws4_request"
    string_to_sign = "\n".join(
        [
            ALGORITHM,
            amz_date,
            scope,
            hashlib.sha256(canonical_request.encode("utf-8")).hexdigest(),
        ]
    )

    k_date = sign(f"AWS4{secret_key}".encode("utf-8"), date_stamp)
    k_region = sign(k_date, region)
    k_service = sign(k_region, SERVICE)
    k_signing = sign(k_service, "aws4_request")
    signature = hmac.new(
        k_signing, string_to_sign.encode("utf-8"), hashlib.sha256
    ).hexdigest()

    return {
        "x-amz-date": amz_date,
        "x-amz-content-sha256": payload_hash,
        "Authorization": (
            f"{ALGORITHM} Credential={access_key}/{scope}, "
            f"SignedHeaders={signed_headers}, Signature={signature}"
        ),
    }


def build_url(endpoint, bucket, key):
    host = urllib.parse.urlparse(endpoint).netloc
    # Path-style: R2 requires the bucket in the path, not as a subdomain.
    path = f"/{bucket}/{urllib.parse.quote(key)}"
    return f"https://{host}{path}", host, path


def head_object(url, host, path, region, access_key, secret_key):
    """Return True if the object exists. Used for resume.

    `path` must be the bucket-qualified path, matching the canonical request.
    """
    req = urllib.request.Request(url, method="HEAD")
    req.add_header("Host", host)
    for k, v in sigv4_headers(
        "HEAD", host, path, "UNSIGNED-PAYLOAD", region, access_key, secret_key
    ).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return 200 <= resp.status < 300
    except urllib.error.HTTPError as exc:
        if exc.code == 404:
            return False
        raise


def put_object(key, local_path, endpoint, bucket, region, access_key, secret_key,
               retries, timeout, content_type, skip_existing):
    """Upload one file. Returns (status, detail)."""
    url, host, path = build_url(endpoint, bucket, key)
    size = local_path.stat().st_size

    if skip_existing:
        try:
            if head_object(url, host, path, region, access_key, secret_key):
                return "skip", size
        except Exception:
            pass  # fall through and let PUT surface the real error

    payload_hash = "UNSIGNED-PAYLOAD"
    last_err = None
    for attempt in range(retries):
        try:
            data = local_path.read_bytes()
            req = urllib.request.Request(url, data=data, method="PUT")
            req.add_header("Host", host)
            req.add_header("Content-Type", content_type)
            req.add_header("Content-Length", str(len(data)))
            for k, v in sigv4_headers(
                "PUT", host, path, payload_hash, region, access_key, secret_key
            ).items():
                req.add_header(k, v)
            # No CannedACL / x-amz-acl header. IDrive, B2 and Tigris all allow
            # per-object public-read, but setting it on the bucket/dashboard is
            # more portable across providers, so we leave it out here.
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                if not (200 <= resp.status < 300):
                    return "error", f"HTTP {resp.status}"
                return "ok", len(data)
        except urllib.error.HTTPError as exc:
            body = ""
            try:
                body = exc.read().decode("utf-8", "replace")[:300]
            except Exception:
                pass
            last_err = f"HTTP {exc.code} {body}".strip()
            if exc.code == 403:
                return "error", last_err
        except Exception as exc:  # noqa: BLE001
            last_err = f"{type(exc).__name__}: {exc}"
        time.sleep(min(2**attempt, 8))

    return "error", last_err


def report(total):
    processed = uploaded + skipped + sum(failed.values())
    if not processed:
        return
    pct = 100.0 * (uploaded + skipped) / processed
    mb = bytes_sent / 1e6
    elapsed = max(time.time() - START, 0.001)
    rate = mb / elapsed
    remaining = total - uploaded - skipped
    eta = remaining / rate if rate > 0.01 else 0
    sys.stdout.write(
        f"\r{uploaded + skipped}/{total} ({pct:5.1f}%) "
        f"{mb:8.1f} MB  {rate:6.1f} MB/s  "
        f"fail {sum(failed.values())}  eta {eta / 60:5.1f}m   "
    )
    sys.stdout.flush()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--sql", type=Path, default=DEFAULT_SQL)
    ap.add_argument("--images", type=Path, default=DEFAULT_IMAGES)
    ap.add_argument("--workers", type=int, default=8)
    ap.add_argument("--retries", type=int, default=4)
    ap.add_argument("--timeout", type=int, default=120)
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument(
        "--no-skip-existing",
        action="store_true",
        help="upload everything, ignoring what is already in the bucket",
    )
    ap.add_argument(
        "--prefix", default="", help="optional key prefix, e.g. images/"
    )
    ap.add_argument(
        "--ext",
        default="",
        help=(
            "upload a converted format instead of the extension recorded in the "
            "dump, e.g. --ext .webp after converting PNGs to WebP. The dump's "
            "Extension column still selects WHICH ids to upload; this only "
            "changes the key suffix, so --images must point at the converted "
            "files."
        ),
    )
    ap.add_argument(
        "--skip-ext",
        default="",
        help=(
            "comma-separated dump extensions to leave alone, e.g. "
            "--skip-ext .png when the PNGs are being uploaded as WebP via "
            "--ext instead. Avoids storing both formats."
        ),
    )
    ap.add_argument(
        "--from-ext",
        default=".png",
        help=(
            "which dump extension --ext replaces. Only rows carrying this "
            "extension are selected, so --ext .webp uploads converted PNGs and "
            "leaves the JPEGs and GIFs alone. Animated GIFs have no still "
            "equivalent and are excluded by default."
        ),
    )
    args = ap.parse_args()

    global uploaded, skipped, bytes_sent, START
    START = time.time()

    env = load_env(
        required=(
            "S3_ACCESS_KEY_ID",
            "S3_SECRET_ACCESS_KEY",
            "S3_BUCKET",
            "S3_ENDPOINT",
            "S3_REGION",
        )
    )
    access_key = env["S3_ACCESS_KEY_ID"]
    secret_key = env["S3_SECRET_ACCESS_KEY"]
    bucket = env["S3_BUCKET"]
    endpoint = env["S3_ENDPOINT"].rstrip("/")
    region = env["S3_REGION"]

    if not args.sql.exists():
        die(f"SQL dump not found: {args.sql}")
    if not args.images.exists():
        die(f"Image directory not found: {args.images}\n  Run download.py first.")

    images = list(parse_images(args.sql))
    if args.skip_ext:
        skip = {e.strip().lower() for e in args.skip_ext.split(",") if e.strip()}
        images = [(i, e) for i, e in images if e.lower() not in skip]
    if args.ext:
        want = args.from_ext.lower()
        images = [(i, args.ext) for i, e in images if e.lower() == want]
    if args.limit:
        images = images[: args.limit]
    if not images:
        die("No Images rows parsed.")

    # Fail early if local files are missing, before burning upload attempts.
    local_missing = [
        f"{i}{e}" for i, e in images if not (args.images / f"{i}{e}").exists()
    ]

    print(f"bucket     : {bucket} @ {endpoint}")
    print(f"images     : {len(images)} from {args.sql.name}")
    print(f"workers    : {args.workers}")

    if args.dry_run:
        for image_id, extension in images[:10]:
            key = f"{args.prefix}{image_id}{extension}"
            url, _, _ = build_url(endpoint, bucket, key)
            print(f"  {url}")
        print(f"  ... ({len(images)} total)")
        return

    if local_missing:
        die(
            f"{len(local_missing)} file(s) missing locally, e.g. "
            f"{local_missing[:5]}. Re-run download.py first."
        )

    def task(image_id, extension):
        key = f"{args.prefix}{image_id}{extension}"
        ctype = CONTENT_TYPES.get(extension.lower(), "application/octet-stream")
        return put_object(
            key,
            args.images / f"{image_id}{extension}",
            endpoint,
            bucket,
            region,
            access_key,
            secret_key,
            args.retries,
            args.timeout,
            ctype,
            not args.no_skip_existing,
        )

    with ThreadPoolExecutor(max_workers=args.workers) as pool:
        futures = [pool.submit(task, i, e) for i, e in images]
        for future in as_completed(futures):
            status, detail = future.result()
            with lock:
                if status == "ok":
                    uploaded += 1
                    bytes_sent += detail
                elif status == "skip":
                    skipped += 1
                else:
                    failed["error"] += 1
                    if len(errors) < 30:
                        errors.append(detail)
                report(len(images))

    report(len(images))
    print("\n")
    print(f"uploaded : {uploaded}")
    print(f"skipped  : {skipped} (already in bucket)")
    print(f"sent     : {bytes_sent / 1e9:.2f} GB")
    print(f"failed   : {sum(failed.values())}")
    if errors:
        print("\nfirst errors:")
        for err in errors[:15]:
            print(f"  {err}")
        print("\nRe-run the same command to resume.")


if __name__ == "__main__":
    main()