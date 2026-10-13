# Instance bootstrap scripts

Tooling for standing up a self-hosted instance from a database export plus an
image archive. Not part of the API build — nothing here is referenced by the
backend or frontend.

## Layout

| File | Purpose |
|---|---|
| `loadenv.py` | Reads `.env`, shared by every script |
| `import_supabase.py` | Imports `waifu-public.sql` into Postgres, then verifies row counts |
| `upload_to_s3.py` | Mirrors local images to any S3-compatible bucket, resumable |
| `make_public.py` | Applies a public-read bucket policy (not supported by R2) |
| `mkconn.py` | Converts a `postgres://` URI to the keyword format Npgsql expects |
| `worker/` | Cloudflare Worker that fronts a private bucket and proxies API reads |

## Setup

```bash
cp .env.example .env && chmod 600 .env
```

`.env` is gitignored. Never commit it.

Required values:

- `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` — scoped to one bucket
- `S3_BUCKET`, `S3_ENDPOINT`, `S3_REGION` — region is fixed at bucket creation
- `DATABASE_URL` — direct connection, port 5432

## Import a database

```bash
uv run import_supabase.py --check   # test connection, writes nothing
uv run import_supabase.py           # import, then verify counts
```

Handles two things that bite otherwise:

- Strips the `\restrict` line, which older psql rejects
- Forces `ON_ERROR_STOP=1`, so a mid-import failure can't leave you with a
  half-populated database that looks fine

The final `COUNT` queries are the real check. Expect 4279 images if you are
importing the current export.

Re-running against a non-empty database fails on purpose. Drop it first.

## Mirror images

```bash
uv run upload_to_s3.py --dry-run
uv run upload_to_s3.py --limit 5     # smoke test
uv run upload_to_s3.py               # all
```

Keys are `{id}.{ext}`, matching `CdnUrlHelper.GetImageUrl`. The image list is
parsed out of the SQL dump, so filenames cannot drift from what you imported.

Resumable: objects already in the bucket are skipped via `HEAD`. Re-run after
any interruption.

## Serving from a private bucket

Providers that block new public buckets still allow private ones. A Cloudflare
Worker signs origin requests and caches at the edge:

```bash
cd worker
npx wrangler login
npx wrangler deploy
npx wrangler secret put S3_ACCESS_KEY_ID
npx wrangler secret put S3_SECRET_ACCESS_KEY
```

Set `Cdn__BaseUrl` in the API to the printed `workers.dev` URL. The Worker
streams, so large images don't buffer, and rejects `..` in the key before
anything reaches the origin.

### Proxying the API

Anything that isn't an image extension is forwarded to `API_ORIGIN`, set in
`worker/wrangler.toml`. Anonymous `GET`s are cached for 60 seconds, so a page
load's `/images` + `/tags` + `/stats` is one trip to Oregon instead of three.

This replaced an earlier 302 to `API_ORIGIN`. Redirecting sent the browser
straight to the API host, so the edge could never help.

Cached reads only. A request with an `Authorization` or `Cookie` header is
passed through and never stored, so two users can't be served each other's
favourites or album membership. `/users`, `/review`, `/auth`, `/stats/admin`,
`/reports` and `/albums` are never cached even anonymously — they're listed
explicitly so a future change that drops authentication on one of them can't
start serving it from the cache. Only a clean 200 is stored, so a transient
500 isn't replayed to everyone for the next minute.

### Invalidating the cache

Bump `CACHE_VERSION` in `worker.js` and redeploy:

```bash
# edit CACHE_VERSION, then
npx wrangler deploy
```

This is the *only* way to clear the cache. The Worker runs on `workers.dev`
and stores entries through `caches.default`, which Cloudflare cannot purge by
host and for which there is no dashboard control. Bumping the version orphans
the previous generation, and those entries expire on their own.

Bump it whenever the bytes served for an unchanged URL change — a format
migration, a different encoder, a content-type fix. Deploying alone will not
do it.

## Connection strings

Npgsql wants keyword format, not a URI:

```
Host=db.example.com;Port=5432;Database=postgres;Username=postgres;Password=...;SSL Mode=Require
```

`mkconn.py` converts one to the other:

```bash
uv run mkconn.py --env
uv run mkconn.py 'postgresql://user:pass@host:6543/db'
```

Render's free tier has no IPv6 route. If you see
`Failed to connect to [...]:5432 / Network is unreachable`, use Supabase's
session pooler on port 6543 instead of the direct host.