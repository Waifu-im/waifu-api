/**
 * Cloudflare Worker: serves images from a private S3 bucket.
 *
 * Cloudflare fetches from the origin with SigV4 and caches at the edge, so the
 * bucket never needs public access. This is the way out of IDrive's
 * "no new public buckets" restriction, and it also removes origin egress as
 * a concern since each object is fetched roughly once.
 *
 * Deploy:
 *   npx wrangler deploy
 * Secrets (do NOT put these in wrangler.toml):
 *   npx wrangler secret put S3_ACCESS_KEY_ID
 *   npx wrangler secret put S3_SECRET_ACCESS_KEY
 */

const ALGO = "AWS4-HMAC-SHA256";
const SERVICE = "s3";

const enc = new TextEncoder();

async function hmac(key, msg) {
  const k = await crypto.subtle.importKey(
    "raw",
    key,
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"]
  );
  return new Uint8Array(await crypto.subtle.sign("HMAC", k, enc.encode(msg)));
}

function hex(buf) {
  return [...new Uint8Array(buf)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function sha256Hex(text) {
  return hex(await crypto.subtle.digest("SHA-256", enc.encode(text)));
}

async function signingKey(secret, dateStamp, region) {
  const kDate = await hmac(enc.encode(`AWS4${secret}`), dateStamp);
  const kRegion = await hmac(kDate, region);
  const kService = await hmac(kRegion, SERVICE);
  return hmac(kService, "aws4_request");
}

/** Sign a GET for one object and return signed headers. */
async function signGet(host, path, region, accessKey, secretKey) {
  const now = new Date();
  const amzDate = now.toISOString().replace(/[:-]|\.\d{3}/g, "");
  const dateStamp = amzDate.slice(0, 8);

  // GET with no body.
  const payloadHash = await sha256Hex("");
  const signedHeaders = "host;x-amz-content-sha256;x-amz-date";
  const canonicalHeaders =
    `host:${host}\n` +
    `x-amz-content-sha256:${payloadHash}\n` +
    `x-amz-date:${amzDate}\n`;

  const canonicalRequest = [
    "GET",
    path,
    "",
    canonicalHeaders,
    signedHeaders,
    payloadHash,
  ].join("\n");

  const scope = `${dateStamp}/${region}/${SERVICE}/aws4_request`;
  const stringToSign = [
    ALGO,
    amzDate,
    scope,
    await sha256Hex(canonicalRequest),
  ].join("\n");

  const key = await signingKey(secretKey, dateStamp, region);
  const signature = hex(await hmac(key, stringToSign));

  return {
    "x-amz-date": amzDate,
    "x-amz-content-sha256": payloadHash,
    Authorization:
      `${ALGO} Credential=${accessKey}/${scope}, ` +
      `SignedHeaders=${signedHeaders}, Signature=${signature}`,
  };
}

// CACHE_VERSION namespaces the edge cache.
//
// This Worker runs on workers.dev and stores entries through the Cache API
// (caches.default), which Cloudflare cannot purge by host and which no
// zone-level purge reaches. There is also no dashboard control for it. Bumping
// this constant is therefore the only way to invalidate everything already
// cached: the new generation is written under different keys, so the previous
// entries become unreachable and expire on their own.
//
// Bump it whenever the bytes served for an unchanged URL change - a format
// migration, a different encoder, a content-type fix. Nothing else will.
const CACHE_VERSION = "v2";

// cacheKeyFor builds the key an entry is stored under.
//
// Only the path and query are preserved; the origin is included so two
// deployments sharing a Worker never collide. The version prefix is what makes
// the whole store discardable.
function cacheKeyFor(request) {
  const url = new URL(request.url);
  return new Request(
    `${url.origin}${CACHE_VERSION}/cache${url.pathname}${url.search}`,
    { method: request.method }
  );
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response("Method Not Allowed", { status: 405 });
    }

    // Query strings on non-image paths (/images?limit=...) belong to the API.
    //
    // An image path with a query string is still an image request, though, so a
    // "?fresh=1" style cache-buster reaches the image logic below instead of
    // being bounced to the API. Keying on the extension is what makes that
    // distinction safe.
    const looksLikeImage = /\.(png|jpe?g|gif|webp)$/i.test(url.pathname);
    if (url.search && !looksLikeImage) {
      // API_ORIGIN already carries its scheme; prefixing one produced a
      // "https://https://" URL that the browser mangled into https//.
      return Response.redirect(`${env.API_ORIGIN}${url.pathname}${url.search}`, 302);
    }

    // Reject traversal before it reaches the origin.
    const key = decodeURIComponent(url.pathname.replace(/^\/+/, ""));
    if (!key || key.includes("..") || key.includes("/")) {
      return new Response("Not Found", { status: 404 });
    }

    // Serve from edge cache when warm; otherwise fetch and cache.
    //
    // The cache is keyed on a versioned path rather than the request URL. See
    // CACHE_VERSION for why that indirection is necessary.
    const cache = caches.default;
    const cacheKey = cacheKeyFor(request);
    const cached = await cache.match(cacheKey);
    if (cached) {
      return cached;
    }

    const host = new URL(env.S3_ENDPOINT).host;

    /**
     * Signed GET for one object key. Returns null on a miss so the caller can
     * fall through to the next candidate rather than having to inspect the
     * response itself.
     */
    const getObject = async (objectKey) => {
      const path = `/${env.S3_BUCKET}/${objectKey}`;
      const headers = await signGet(
        host,
        path,
        env.S3_REGION,
        env.S3_ACCESS_KEY_ID,
        env.S3_SECRET_ACCESS_KEY
      );
      const res = await fetch(`https://${host}${path}`, { headers });
      return res.ok ? res : null;
    };

    // The API's Image.Extension column still says ".png" for every image that
    // was originally uploaded as one, and clients cache those URLs hard. Rather
    // than rewriting the API and invalidating every stored link, a .png request
    // is served from the converted .webp object.
    //
    // content-type is taken from the origin below, so the response honestly
    // reports image/webp - it is not pretending to be a PNG. A client that
    // sniffs (browsers, Discord, sharp, Pillow) is unaffected; only a client
    // that asserts content-type against the .png extension would object.
    //
    // WEBP_FALLBACK is a kill switch: set it to "false" and this whole path is
    // skipped, restoring plain .png -> .png in one deploy.
    const useWebp = env.WEBP_FALLBACK === "true";
    const wantsPng = key.toLowerCase().endsWith(".png");
    const webpKey = wantsPng ? key.slice(0, -4) + ".webp" : null;

    let origin = null;
    let servedKey = key;

    if (useWebp && webpKey) {
      origin = await getObject(webpKey);
      if (origin) servedKey = webpKey;
    }

    // Either the request was not a .png, or there is no converted object yet
    // (an un-migrated image, or a newer upload). Falling back keeps this a pure
    // optimisation: it can never turn a working image into a 404.
    if (!origin) {
      origin = await getObject(key);
      servedKey = key;
    }

    if (!origin) {
      return new Response("Not Found", { status: 404 });
    }

    const body = origin.body;
    const contentType = origin.headers.get("content-type") || "application/octet-stream";

    // Stream through so large images never buffer in memory.
    const response = new Response(request.method === "HEAD" ? null : body, {
      status: 200,
      headers: {
        "content-type": contentType,
        "cache-control": "public, max-age=31536000, immutable",
        "access-control-allow-origin": "*",
        etag: origin.headers.get("etag") || "",
      },
    });

    // Cached under the versioned key, not the request URL, so that bumping
    // CACHE_VERSION orphans the previous generation instead of requiring a purge
    // that cannot be performed against a workers.dev Worker.
    ctx.waitUntil(cache.put(cacheKey, response.clone()));
    return response;
  },
};