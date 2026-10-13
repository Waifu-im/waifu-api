# Plan: Instance index

A public page listing other people's hosted instances of this software, plus a
Discord bot command that registers them and a periodic structural check that
scores each entry.

Motivation: the fork runs for its own sake. Registration earns the operator a
Discord role. That is the whole reward loop.

## Status

Design agreed. Not started. One decision outstanding (storage, see
[Storage](#storage)).

## Scope

**In:**

- `/.well-known/waifu-im` on every deployment, reporting version + one owner
  Discord ID
- `GET /instances` — public, serves the index
- `POST /instances` — admin-authenticated, probes, verifies, stores
- A Discord `/register` slash command
- A periodic re-check producing a trust score
- A frontend page rendering `GET /instances`

**Explicitly out:**

- Public/anonymous submission. Registration is admin-gated; the bot is the
  only writer.
- Proof of domain ownership beyond the `.well-known` fetch. No DNS TXT.
- Federation, nodeinfo, ActivityPub, anything peer-to-peer.
- Team instances (one owner Discord ID per instance).
- A badge system. The reward is a Discord role.

## Max's journey

Max runs an instance, finds the Discord server, runs `/register <baseUrl>`.

1. **Prerequisites are his problem.** Docker, S3 bucket, a backend deployment,
   a Worker or resolvable `Cdn:BaseUrl`, and `scripts/mirror/import_supabase.py`
   to import the ~4,279-image export. The command does not walk him through
   these.
2. **`/register <baseUrl>`** — bot defers an ephemeral reply, because a
   multi-endpoint probe will exceed Discord's 3-second interaction ack window.
3. **Probe** runs (below), then compares the instance's reported owner Discord
   ID against the invoker's. Mismatch fails with both IDs named and the
   `SITE_OWNER_DISCORD_ID` env var called out.
4. **Store** the entry, with `ownerDiscordId` and `verifiedAt` recorded at
   registration — the re-check compares against these, not against whatever
   answers later.
5. **Grant the Discord role.**
6. **He appears on the index page.**

## Verification

Probed against the instance's base URL:

| Check | What it proves |
| --- | --- |
| `GET /ping` | liveness only — near-worthless as proof, but cheap |
| `GET /.well-known/waifu-im` | domain ownership + running this software |
| `GET /stats/public` | shape compatibility, image count |
| `GET /openapi/v1.json` | `Info.Version` within the accepted range |
| `GET /images?limit=1`, then `HEAD` its `Url` | S3 bucket + Worker + `Cdn:BaseUrl` wiring agree |

The last row is the one that earns the name trust score. It exercises the
`Image.Extension` ↔ `{id}{ext}` key invariant end to end — the mismatch
described in `AGENTS.md` that produces images which 404.

**Fetching is untrusted input.** Every Discord member who can run `/register` can
make the backend issue an HTTP request. Block `169.254.169.254`, RFC1918,
`127/8` and `::1`; force HTTPS; cap response size, timeout and redirects.

## Trust score

Signals are stored alongside the score, so a drop is explainable:

- `signals: [{ name, ok }]` persisted next to `TrustScore`
- a failed check marks the entry `Stale`, it never deletes it — a Render
  free-tier instance sleeps for hours and that is not delisting-worthy
- score is recomputable by anyone from the published signals

## Storage

**Open — the one blocking decision.** Recommended: a single Postgres row holding
a JSONB array. `get → mutate → set` on one column, atomic, auditable, no new
infra.

Alternatives and why they are worse:

- **S3 JSON file.** The Worker will not serve it: `worker.js:120`'s
  `looksLikeImage` regex requires `.png|.jpg|.gif|.webp`, so `instances.json`
  falls through to `handleApi` and is proxied to `API_ORIGIN`, returning 404.
  Needs a new Worker branch for a signed GET of a non-image object.
- **Local file.** Render's filesystem is ephemeral; the list vanishes on the
  next deploy.

## Components

| Piece | Notes |
| --- | --- |
| `/.well-known/waifu-im` controller | serves `{ version, ownerDiscordId }`; ~40 lines |
| `ConfigurationKeys` | new `Site:OwnerDiscordId` key; env var `SITE_OWNER_DISCORD_ID`, documented in `.env.example` |
| `POST` / `GET /instances` | admin policy on writes, public read |
| prober | an injectable interface so tests can fake it — `TestHarness` runs on an in-memory DB and cannot make real HTTP calls |
| re-check | `BackgroundService` alongside `StatsCleanupService`, same 24h loop and `CreateScope()` pattern |
| bot `/register` | `discord.app_commands`; the existing `bot/bot.py` is prefix-command based with the `MESSAGE_CONTENT` privileged intent |

Existing conventions to match: `PaginatedList<T>` + `PaginationRequest` for
list responses, `StorageDiffController` for class-level
`[Authorize(Policy = AuthorizationPolicies.Admin)]`, `Role` gates via
`Permissions:*`, request DTOs declared below the controller class.

## Notes on hosting

- The bot must call the **API host directly**, not the Worker: `worker.js:351`
  forwards only `Authorization`, `Accept` and `Accept-Language`, so `X-Api-Key`
  is dropped and the call comes back 401. `README.md` already documents this.
- `.well-known` paths proxy correctly through the Worker — `worker.js:120`'s
  regex does not match them, so they take the `handleApi` branch. No Worker
  change needed for this feature.
- The endpoint is public, so it falls in the Worker's cacheable set at
  `JSON_TTL_SECONDS = 60`. Acceptable; revisit only if curation needs to be
  instant.
- `/.well-known/waifu-im` must ship on candidate instances **before**
  registration means anything. Early entries will be verified by hand.