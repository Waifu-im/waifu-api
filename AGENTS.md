# AGENTS.md

Notes for agents and contributors working in this repo. These are the things
that have actually cost time, not generic advice.

## Build and test

**Always verify in Release, because Render publishes in Release.** Debug builds
and `dotnet test` pass in situations where the deploy breaks.

```bash
# What Render actually runs
cd backend
docker run --rm -v "$PWD":/src -w /src mcr.microsoft.com/dotnet/sdk:10.0 \
  dotnet publish WaifuApi.Web/WaifuApi.Web.csproj -c Release -o /app/publish \
  /p:UseAppHost=false /p:Version=0.0.0-local /p:OpenApiGenerateDocumentsOnBuild=false

# Test suite
docker run --rm -v "$PWD":/src -w /src mcr.microsoft.com/dotnet/sdk:10.0 dotnet test
```

There is no local `dotnet`; use the SDK container. It is the same SDK Render
builds with.

### Known licensing trap

`SixLabors.ImageSharp` **4.x is commercially licensed** and fails Release
publish without a `SixLaborsLicenseKey`. Do not upgrade to it casually — the
project is deliberately pinned to **3.1.12**, which already ships
`SixLabors.ImageSharp.Formats.Webp.WebpEncoder`. Upgrading buys nothing and
breaks the deploy. 3.1.12 does carry open NuGet security advisories (NU1902 /
NU1903); that is a known, accepted trade-off pending a decision on either a
Six Labors license or replacing ImageSharp entirely.

## Storage and images

- Images live in a **private** bucket; there is no public read path. The
  Cloudflare Worker in `scripts/mirror/worker` signs SigV4 and caches at the
  edge. Backend code and this worker are what stand between you and a broken
  public site.
- Worker secrets are **not** in `wrangler.toml`. They are set with
  `npx wrangler secret put` and sourced from `scripts/mirror/.env`, which is
  gitignored.
- `Image.Extension` is the single source of truth for both the S3 key
  (`{id}{ext}`) and the URL the API hands out. If you change one path, check
  the other still agrees — a mismatch produces images that 404.
- The perceptual hash behind duplicate detection is computed from the
  **original** upload bytes, never from a re-encoded copy. Keep it that way;
  hashing a lossy re-encode makes dedupe unreliable across uploads.

## Data and migrations

- `waifu-public.sql`-style dumps are **not** guaranteed current. Reconcile
  counts against the live API and against the local mirror before trusting a
  dump as a manifest. A stale dump silently omits recent images.
- `Image.PerceptualHash` is a Postgres `bit(64)`, which `pg_dump` renders as
  **64 binary characters**, not hex. The API exposes it as 16 lowercase hex
  chars via `BitArrayHelper.ToHex`. Mixing the two up silently breaks
  comparison.

## Verifying your own work

- Do not tell someone a command or a cache-buster works before reading the code
  that will run it.
- When a probe or script fails, suspect the probe first. Several "findings"
  during the Tigris migration were bugs in the measurement, not the system.
- Reconcile totals from independent sources (dashboard, live API, object
  store) before declaring a migration complete — that is how six missing images
  surfaced.
