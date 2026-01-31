<div align="center">
  <a href="https://waifu.im">
    <img src="https://cdn.waifu.im/7892.jpg" alt="Waifu.im Logo" width="200" style="border-radius: 20px; box-shadow: 0 8px 30px rgba(0,0,0,0.12);">
  </a>

  <h1>Waifu.im API</h1>

  <p>
    <strong>The versatile waifu image provider.</strong><br>
    Access a curated archive of over 4000 anime-style images with powerful filtering.
  </p>

  <p>
    <a href="https://waifu.im"><strong>Website</strong></a> •
    <a href="https://docs.waifu.im"><strong>Documentation</strong></a> •
    <a href="https://waifu.im/contact/"><strong>Support</strong></a>
  </p>

  <p>
    <img src="https://img.shields.io/github/license/waifu-im/waifu-api?style=flat-square&color=5865F2" alt="License">
    <img src="https://img.shields.io/github/stars/waifu-im/waifu-api?style=flat-square&color=5865F2" alt="Stars">
    <img src="https://img.shields.io/badge/Docker-Ready-blue?style=flat-square&logo=docker&logoColor=white" alt="Docker">
  </p>
</div>

---

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

Deploy your own instance of the Waifu.im API using Docker.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/install/)
- [.NET 10 SDK](https://dotnet.microsoft.com/download/dotnet/10.0) (for local backend dev)
- [Node.js 20+](https://nodejs.org/) (for local frontend dev)
- A [Discord Application](https://discord.com/developers/applications) (for authentication)
- S3-Compatible Storage (AWS S3, MinIO, Scaleway, etc.)

### 🛠️ Installation (Production)

1. **Clone the repository**
   ```bash
   git clone https://github.com/waifu-im/waifu-api.git
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
   Frontend__BaseUrl="https://www.waifu.im"

   # Frontend
   VITE_API_URL="https://api.waifu.im"
   VITE_DISCORD_REDIRECT_URI="https://www.waifu.im/auth/callback"
   ```

3. **Launch**
   ```bash
   docker-compose up --build -d
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
   Frontend__BaseUrl="http://localhost:5173"

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

---

<div align="center">
  <sub>Built with ❤️ by the Waifu.im team.</sub>
</div>
