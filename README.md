<div align="center">
  <a href="https://waifu-api-eosin.vercel.app">
    <img src="frontend/public/brand/logo.jpg" alt="Waifu.im Logo" width="200" style="border-radius: 20px; box-shadow: 0 8px 30px rgba(0,0,0,0.12);">
  </a>

  <h1>Waifu.im API</h1>

  <p>
    <strong>The versatile waifu image provider.</strong><br>
    Access a curated archive of over 4000 anime-style images with powerful filtering.
  </p>

  <p>
    <a href="https://waifu-api-eosin.vercel.app"><strong>Website</strong></a> •
    <a href="https://waifu-api-docs.vercel.app"><strong>Documentation</strong></a> •
    <a href="https://discord.gg/qceZ92p2xK"><strong>Support</strong></a>
  </p>

  <p>
    <a href="https://stats.uptimerobot.com/AbgWCVWsKT"><img src="https://img.shields.io/badge/instance-live-22c55e?style=flat-square" alt="Instance status"></a>
    <img src="https://img.shields.io/github/license/waifu-im/waifu-api?style=flat-square&color=5865F2" alt="License">
    <img src="https://img.shields.io/badge/Docker-Ready-blue?style=flat-square&logo=docker&logoColor=white" alt="Docker">
  </p>
</div>

> **This is a community-run fork.** The original project by
> [Buco7854](https://github.com/Waifu-im/waifu-api) is being migrated off its
> original infrastructure. The API contract is unchanged, so existing
> integrations keep working — but the endpoints moved, and some features are
> unavailable while the migration is in progress.

## 🚧 Instance status

| | |
|---|---|
| API | `https://waifu-api-031n.onrender.com` |
| Website | `https://waifu-api-eosin.vercel.app` |
| Documentation | `https://waifu-api-docs.vercel.app` |
| Uploads | Open — requires a Discord login; new images go to a review queue |
| Users | Discord OAuth works; existing accounts did not carry over |

Image URLs are served through a Cloudflare Worker in front of a private object
store, so `Cdn__BaseUrl` points at the Worker rather than at storage directly.
Response shapes are unchanged.

## ✨ Features

Waifu.im provides a robust REST API designed for ease of use and flexibility.

- 🖼️ **Extensive Archive**: Over 4000 high-quality images.
- 🏷️ **Tag-Based Search**: Filter by specific character tags, styles, or themes.
- 📁 **Albums**: Organize and share collections of images.
- 👯 **Duplicate Detection**: Automatic detection of duplicate images to maintain quality.
- 🔍 **Advanced Filtering**:
  - Filter by orientation (Landscape/Portrait)
  - Filter by resolution (width/height) and file size
  - Toggle NSFW/Adult content
  - Include/Exclude GIFs
  - Filter by specific Artists
  - Exclude specific files
  - Force inclusion of specific files
- 👤 **User Accounts**: Discord-based authentication to manage favorites and albums.
- 🛡️ **Moderation Tools**: Report system and review queues for community safety.
- ⚡ **Performance**: Optimized for speed with caching and CDN integration.
- 📊 **Statistics**: Track API usage and popular tags.
- 🎲 **Flexible Sorting**: Sort by date, popularity, or get random results.

## 🚀 Getting Started

Deploy your own instance of the Waifu.im API using Docker. See
[`scripts/mirror/`](scripts/mirror/) for tooling that imports a database export
and mirrors an image archive into any S3-compatible bucket.

A Discord bot lives in [`bot/`](bot/): random images, tag and orientation
filters, and an NSFW channel gate. It talks to the Cloudflare Worker rather
than the API host, so commands stay fast and survive Render free-tier cold
starts. See [`bot/README.md`](bot/README.md).

#### Which host should your bot use?

This is the same trade-off the frontend makes, and it is worth picking
deliberately:

| What your bot does | Point it at |
|---|---|
| Fetches images (`GET /images`, `/images/sfw`, ...) | `https://waifu-cdn.waifu-api.workers.dev` — edge-cached, and image bytes never touch Render |
| Anything authenticated | `https://waifu-api-031n.onrender.com` — read below first |

For anonymous reads the Worker is the faster host: a response is served from the
edge for 60 seconds and a cold Render free-tier instance is bypassed
altogether. But **the Worker strips the `X-Api-Key` header** — it only forwards
`Authorization`, `Accept` and `Accept-Language` (`worker.js:332`). An API-key
client pointed at the Worker is silently unauthenticated and gets `401`.

Bearer tokens *are* forwarded, so they work through the Worker — they just
bypass the cache, and requests to `/users`, `/albums` and friends are never
cached regardless. If your bot authenticates in any form, use the API host.
It is the only host that passes every credential through.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/install/)
- [.NET 10 SDK](https://dotnet.microsoft.com/download/dotnet/10.0) (for local backend dev)
- [Node.js 20+](https://nodejs.org/) (for local frontend dev)
- A [Discord Application](https://discord.com/developers/applications) (for authentication)
- S3-Compatible Storage (AWS S3, MinIO, Scaleway, etc.)

### 🛠️ Installation (Production)

1. **Clone the repository**
   ```bash
   git clone https://github.com/KorryKatti/waifu-api.git
   cd waifu-api
   ```

2. **Configure Environment**
   Create your `.env` file from the example:
   ```bash
   cp .env.example .env
   ```

   Edit `.env` with your credentials.

   **🌐 Production (Example)**
   ```ini
   # Backend
   API_BASE_PATH="/"

   # Frontend
   VITE_API_URL="https://waifu-api-031n.onrender.com"
   VITE_DISCORD_REDIRECT_URI="https://waifu-api-eosin.vercel.app/auth/callback"
   ```

3. **Launch**
   ```bash
   docker-compose up -d
   ```

### 💻 Local Development

For active development, we recommend running the database via Docker and the services natively.

1. **Start Database & Infrastructure**
   ```bash
   docker-compose -f docker-compose.dev.yml up -d
   ```
   This starts PostgreSQL (with pgvector) and Adminer.

2. **Configure Environment**
   Ensure your `.env` file is set up for local development:
   ```ini
   # Backend
   API_BASE_PATH="/api"

   # Frontend
   VITE_API_URL="http://localhost:5261/api"
   VITE_DISCORD_REDIRECT_URI="http://localhost:5173/auth/callback"
   ```

3. **Run Backend (.NET)**
   ```bash
   cd backend/WaifuApi.Web
   dotnet run
   ```
   The API will be available at `http://localhost:5261`.

4. **Run Frontend (Vite)**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```
   The frontend will be available at `http://localhost:5173`.

### 🔌 Hosting notes

The reference deployment in this fork does not run the API and its images from
the same host. `Cdn__BaseUrl` points at a Cloudflare Worker rather than at
storage directly:

| | |
|---|---|
| API | Render — JSON metadata, Postgres queries |
| Images | Cloudflare Worker — signs SigV4 to a private object store, caches at the edge |
| Anonymous API reads | Cloudflare Worker — proxies to Render, caches for 60s |

This matters if you self-host. IDrive (among others) no longer allows public
buckets to be created, so the Worker exists to fetch from a private one. AWS S3
or MinIO can be made public directly and don't need it.

The Worker fronts API reads too, not just images. A request without an image
extension is proxied to the API host, and an anonymous `GET` is cached for 60
seconds so a page load's `/images` + `/tags` + `/stats` is one round trip
instead of three. Authenticated requests are never stored, so users can't be
served each other's favourites or albums.

Invalidating that cache needs a `CACHE_VERSION` bump in
[`scripts/mirror/worker/worker.js`](scripts/mirror/worker/worker.js) followed by
a redeploy. There is no purge for a `workers.dev` Worker — see
[`scripts/mirror/README.md`](scripts/mirror/README.md).

#### Storage backend

The Worker is pointed at **Tigris** (`https://t3.storage.dev`, region `auto`,
bucket `waifu-api`, private). This replaced IDrive, whose free tier caps egress
at 1:1 with stored data — a public image site exhausts that within a month and
then fails every cache miss. Tigris includes free egress. Credentials are set as
Worker secrets, never in `wrangler.toml`:

```bash
cd scripts/mirror/worker
grep '^S3_ACCESS_KEY_ID='     ../.env | cut -d= -f2- | npx wrangler secret put S3_ACCESS_KEY_ID
grep '^S3_SECRET_ACCESS_KEY=' ../.env | cut -d= -f2- | npx wrangler secret put S3_SECRET_ACCESS_KEY
npx wrangler deploy
```

#### WebP

Two mechanisms produce WebP, and they are deliberately independent:

- **Existing images** were converted offline with `cwebp` and uploaded under
  `.webp` keys. The Worker maps a `.png` URL to the `.webp` object and reports
  the origin's real `content-type`, so the legacy 1,162 PNG rows keep working
  without a database change. Controlled by `WEBP_FALLBACK` in `wrangler.toml`;
  set it to `"false"` and redeploy to revert to plain `.png` → `.png`.
- **New uploads** are converted by the API at ingest. Off by default, PNG only,
  animated images always skipped, and any conversion failure stores the
  original bytes rather than failing the upload:

  ```bash
  Image__WebpConversionEnabled=true
  Image__WebpQuality=90              # 1-100
  Image__WebpSourceExtensions=.png   # comma-separated
  ```

  Metadata — including the perceptual hash behind duplicate detection — is still
  read from the *original* bytes, so re-encoding never weakens dedupe.

JPEG is excluded by default: it is already lossy-compressed, so re-encoding is
lossy-to-lossy. At q90 the encoder produces output ~3% smaller than `libwebp`
across this project's own PNGs.

### ⚠️ Build in Release, not just Debug

**Render publishes in Release. Always verify with the exact command it runs:**

```bash
dotnet publish WaifuApi.Web/WaifuApi.Web.csproj -c Release -o /app/publish \
  /p:UseAppHost=false /p:Version=0.0.0-local /p:OpenApiGenerateDocumentsOnBuild=false
```

Debug builds and `dotnet test` pass while Release fails. This is not
hypothetical — `SixLabors.ImageSharp` 4.x requires a commercial license key that
is only enforced in Release, so an upgrade that looked fine locally took down
the deploy. Note also that 3.1.12 already ships `WebpEncoder`, so there is no
reason to move off it.

To check a dependency change the way production sees it, build in the SDK
container:

```bash
cd backend
docker run --rm -v "$PWD":/src -w /src mcr.microsoft.com/dotnet/sdk:10.0 \
  dotnet test
```

### 👑 Setting up an Admin User

To access administrative features (moderation, tag management, etc.), you need to promote your user account.

1. **Login**: Go to your frontend URL and log in with Discord.
2. **Access Database**: Connect to your PostgreSQL database (using the `adminer` service at `http://localhost:8081` or CLI).
3. **Promote User**: Run the following SQL query, replacing `YOUR_DISCORD_ID` with your actual Discord User ID:

   ```sql
   UPDATE "Users" 
   SET "Role" = 3 
   WHERE "DiscordId" = 'YOUR_DISCORD_ID';
   ```
   *(Role 3 corresponds to Admin)*

### Special Thanks

A huge thank you to our community contributors who help expand our database:

- **[Ruhannn](https://github.com/Ruhannn)** - Curated the `kamisato-ayaka` tag collection.
- **[Buco7854](https://github.com/Buco7854)** - Original author and maintainer of this project for over three years.

## License

[MPL-2.0](LICENSE) — file-level copyleft. Modify freely; publish any file you
change. Forking and self-hosting is explicitly welcome.

Images are third-party artist work, credited per record in the `Images.Source`
column. This repository distributes the catalogue and the metadata, not a claim
on the artwork.

<div align="center">
  <sub>Built with ❤️ by the Waifu.im team and its community fork maintainers.</sub>
</div>
