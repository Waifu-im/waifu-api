# waifu.im Discord bot

A minimal bot for the [waifu.im API](https://github.com/KorryKatti/waifu-api).

It talks to the Cloudflare Worker (`https://waifu-cdn.waifu-api.workers.dev`) by
default rather than the API host. The worker caches anonymous reads at the edge
for 60 seconds, which keeps commands fast and means a sleeping Render free-tier
instance cannot return a 502 mid-command. Set `WAIFU_API` to the API host to
bypass the cache.

That default is safe *because this bot is anonymous*. If you adapt it to send
credentials, point `WAIFU_API` at the API host instead — see
[Which host to use](#which-host-to-use) below.

## Which host to use

| What your bot does | Point it at |
|---|---|
| Fetches images (`GET /images`, `/images/sfw`, ...) | `https://waifu-cdn.waifu-api.workers.dev` — edge-cached, and image bytes never touch Render |
| Anything authenticated | `https://waifu-api-031n.onrender.com` — read below first |

Anonymous reads are faster through the Worker, and an image URL is served
straight from the edge without a database round trip at all.

But the Worker forwards only `Authorization`, `Accept` and `Accept-Language`
(`worker.js:332`). It **drops `X-Api-Key`**, which is one of the two ways this
API authenticates — an API-key client pointed at the Worker gets a silent
`401`, with no error explaining why. Bearer tokens are forwarded correctly,
they just aren't cached; `/users`, `/albums`, `/reports` and friends are never
cached even anonymously.

Use the API host whenever your bot carries any credential.

## Commands

| Command | What it does |
| --- | --- |
| `!help` | List all commands; `!help waifu` for one in detail |
| `!waifu` | Random SFW waifu |
| `!waifu maid` | Random waifu with that tag (multiple tags = AND) |
| `!waifu landscape` | Filter by orientation (`landscape`/`portrait`/`square`) |
| `!waifu gif` | Only animated images |
| `!waifu nsfw` | NSFW, only in an NSFW channel (see below) |
| `!tags` | List tags with image counts |
| `!ping` | Bot + API health check |

Flags can be combined: `!waifu nsfw gif portrait maid`.

## NSFW channels

`nsfw` is refused everywhere except a channel that is age-restricted. A channel
qualifies if either:

- Discord marks it age-restricted (*Channel Settings > Age Restriction*), which also
  makes Discord apply its own age gate, **or**
- its ID is listed in `NSFW_CHANNELS` in the bot config.

Using Discord's own setting is the stronger option — it gates non-bot members too.
`NSFW_CHANNELS` is a fallback for servers where you can't flip that setting.

If someone tries `!waifu nsfw` in a normal channel, the bot refuses and tells them
which channel ID to add.

## Setup

```bash
uv venv
uv pip install -r requirements.txt
cp .env.example .env   # then fill in DISCORD_TOKEN
uv run python bot.py
```

## Getting a token

1. Create an app at https://discord.com/developers/applications
2. **Bot** → **Reset Token** → copy it into `DISCORD_TOKEN` in `.env`
3. **Bot** → **Privileged Gateway Intents** → turn **MESSAGE CONTENT** on.
   Without this the bot connects but silently ignores every command.
4. Invite the bot to your server with the `bot` scope and `Send Messages` permission

## Config

| Var | Default | Notes |
| --- | --- | --- |
| `DISCORD_TOKEN` | required | Bot token |
| `PREFIX` | `!` | Command prefix |
| `WAIFU_API` | the Cloudflare Worker | Use the API host instead if your bot sends credentials |
| `NSFW_CHANNELS` | empty | Extra channel IDs allowed to get NSFW results (adds to Discord's own age-restricted channels) |
