# Cutover validation log

Record of every decision taken while validating the Go backend against the C#
service it replaces. Written as work happened, not reconstructed afterwards.

The guiding constraint: **production must not break.** Everything below is
either reversible, isolated, or read-only.

---

## Safety rules adopted

Established before any work, and not to be broken by later convenience:

1. **Never write to production Postgres.** Read-only if anything.
2. **Never write to the production S3/Tigris bucket.** Read-only if anything.
3. **The Go service is never started against the production database.** This is
   not caution, it is a specific hazard: `background.runJob` runs each cleanup
   job *once at startup* before its daily ticker begins, matching the C#
   service's behaviour. `StatsCleanup` is enabled unconditionally and issues
   `DeleteDailyStatsBefore`. Booting Go against production would delete rows
   immediately, with no way to notice in time.
4. **Secrets are never printed.** `.env` files are read for variable *names*
   only. Any value echoed into a transcript or a log is redacted.
5. **The `tools/backfill` program is never run against production** until it
   has been proven on a local stack, which is the point of this exercise.

Credentials exist locally in `backend/.env` and `scripts/mirror/.env`
(Supabase database password, Tigris access key and secret, JWT signing key,
Discord client secret). Their presence is what makes rule 1–3 necessary rather
than theoretical.

---

## 2026-10-13 — Inventory

### Decision: build an isolated stack rather than reuse what exists

Two containers were already running: `waifu-test-pg` (plain `postgres:17-alpine`
on port 28407, **no pgvector**) and `waifu-csharp-run` (`waifu-csharp:parity`,
no published ports). Neither is a fit: the schema needs the `vector` extension,
and a pre-existing container of unknown provenance is not something to run
schema changes against.

**Decision: create a fresh `pgvector/pgvector:pg18` database and a fresh MinIO
bucket, both named distinctly, both disposable.**

### Decision: do not download the full production image archive

`scripts/mirror/README.md` describes a corpus in the tens of gigabytes. For a
stress test that is irrelevant, and for a backfill validation it is unnecessary.
What the backfill needs to be proven is that it fetches an object at key
`{id}{ext}`, decodes it, hashes it, and writes the result — against a handful of
rows, not four thousand.

**Decision: generate a synthetic corpus locally, with known duplicate pairs
seeded deliberately.** This is strictly better than real data for validation,
because the correct answer is known in advance: a re-encoded duplicate must land
within the duplicate threshold, and an unrelated image must not.

### Decision: no Cloudflare tunnel until the local tests pass

A tunnel makes the local instance publicly reachable and requires Cloudflare
account credentials. It is the last step, not the first, and it is the only step
that could affect anything outside this machine.

**Decision: sequence everything local first. Stop and report before the tunnel.**

---

## Verification already completed (from earlier in this session)

Recorded here so the log is complete, not because it happened today.

- 59 C# endpoints vs 59 Go routes: exact match in both directions, zero missing.
- Full Go test suite passes with `-race` against a real Postgres.
- Go container builds and starts; reports zero configuration problems when fed
  exactly the variables documented in `.env.example`.
- `schema.sql` and the EF model snapshot agree, enforced by a contract test.
- Migration `20261013120000_AddPerceptualHashLegacy` and `schema.sql` produce
  byte-identical schemas: 88 columns, 12 indexes.
- C# still publishes in Release, so the comparison oracle remains available.

---

## Open at the time of writing

- Synthetic corpus: not yet generated.
- `tools/backfill`: **never executed**, in any environment. Largest unknown.
- Go service: never run against real data of any kind.
- Load behaviour at Render's 512 MB ceiling: unmeasured.
---

## 2026-10-13 — Local stack built and exercised

### Stack

| Service | Image | Address | Note |
|---|---|---|---|
| Postgres | `pgvector/pgvector:pg18` | `localhost:55440` | schema from `schema.sql`; disposable |
| S3 | `adobe/s3mock` | `localhost:55450` | HTTP is on **9090**, not 9191 — 9191 is TLS |
| Go API | `waifu-api-go:check` | `:8086` | run with `--memory 512m` to match Render |

All three disposable and named `waifu-vtest-*` so they cannot be confused with
anything real. Port 8080 was already occupied by a C# service on this machine
and was left alone.

### Fixture design

85 image rows. 80 have a decodable object at the conventional `{id}{ext}` key,
3 have **no object** at all, 2 have an object that is **not a decodable image**.
Every row is part of a near-duplicate pair, and every third pair is the same
picture stored in a *different format* — so a tool that keyed on extension
would miss it.

Half the rows were seeded with a hash computed by this same implementation, so
a correct run must report distance **0** for those. The other half were seeded
with random 64-bit values, which is what the C# column effectively looks like
from here. Ground truth was therefore known in advance for every row.

`corpusgen` was reused to produce the images; it was killed by a command
timeout partway but had already emitted 537 files, far more than needed.

A seeder program was written under `backend/go/tools/vtestseed/` and **deleted
after use** — it is not in the diff. It had to live inside the module because
Go's `internal` rule prevents importing `phash` from outside.

---

## Results

### `tools/backfill` — first execution ever

| Stage | Result |
|---|---|
| Precondition, legacy column empty | **refused to write**, exit 1, correct message |
| Dry run, legacy empty | proceeded with a warning, wrote nothing |
| Dry run, `ForcePathStyle` unset | every fetch failed — environment misconfiguration, not a code fault |
| Dry run, corrected | 80 of 85 hashed, **40 at distance 0**, 40 scattered 25–39 |
| Real write, legacy populated | 80 written, 40 changed, 45 untouched |
| **Re-run after write** | **80 of 80 at distance 0** |

The distance distribution before the write (0 for the Go-seeded half, 25–39 for
the other) matches the shape documented for real C# data (22–38). The column is
now internally consistent for the first time in its existence.

Failures were reported with the right distinction:

```
id 81  fetch 81.png: get object 81.png: object not found
id 84  hash 84.png: could not decode image: image: unknown format
```

Missing object and undecodable object are different failures and stayed
different. This took one correction: the corrupt rows were initially written to
the wrong key, so they were really just testing the missing-object path twice.

### Duplicate detection, end to end

Ids 1 and 2 are the same picture in different formats. After the backfill both
hold `1100000111111101000000011111010100000001010101001111111011010001`.
Running the service's own query from `repo/images.go:527` with id 2's hash
returns **id 1**. Before the backfill this returned nothing.

### API behaviour

Reads 200, `/banner` 204 with no banner set, missing tag/artist 404, and **401
on every unauthenticated write**. Auth is not bypassed by the rewrite.

---

## FINDINGS

### 1. Per-request logging will OOM a 512 MB instance. Critical.

**Observed:** under sustained load the container was **OOM-killed, exit 137**,
`oomkilled=true`. Controlled re-run changing one variable:

| | `LOG_LEVEL` unset | `LOG_LEVEL=error` |
|---|---|---|
| Outcome | OOM-killed | survived |
| Throughput | — | 312 rps, 0 errors |
| Memory | >512 MB | 37 MB (7%) |

**Cause:** `internal/httpapi/middleware.go:79` calls `log.Info("request", ...)`
for every request, unconditionally. At 5,000 rps that is 5,000 JSON lines per
second into a stdout pipe nobody is draining, on a container capped at 512 MB.

**Divergence from C#:** the C# middleware does not log per request at all — it
records stats to the database. And production sets
`Logging__LogLevel__Default=Error`, which suppresses output entirely.

**The config trap:** Go reads `LOG_LEVEL`, not `Logging__LogLevel__Default`.
`LOG_LEVEL` appears **nowhere in `.env.example`**. A deployment that faithfully
copies the C# configuration gets INFO logging from Go and no way to know it.

**Not fixed here.** This needs a decision: log requests at Debug rather than
Info, and document `LOG_LEVEL`. Both are small. Left for the owner rather than
changed unilaterally.

### 2. `backend/.env` is stale. Medium.

The local development env is missing three keys the Go service requires and
`.env.example` documents:

- `Permissions__SubmitEditMinRole` — the file has `SubmitEditRequestMinRole`,
  which is not the name the C# `ConfigurationKeys` registry uses either
- `Review__DefaultPageSize`
- `Review__MaxPageSize`

Booting from it fails on configuration validation. `.env.example` is correct;
only the checked-out local file has drifted. Whether **Render** has the same
gap was not checked and cannot be checked from here — this is the thing most
likely to cause a crash-loop at cutover, and it is one dashboard screen away.

### 3. Throughput ceiling is the database pool, not the application. Informational.

312 rps at 50 concurrent workers and 312 rps at 200. Throughput is flat while
latency scales linearly, which is the signature of a hard resource limit — an
8-connection pool against a local database. Not a Go defect, but it means the
first real tuning decision at cutover is pool size, not the runtime.

---

## Not done, and why

**Cloudflare tunnel.** Stopped here as planned. It requires `cloudflared`
(not installed) and a Cloudflare account with tunnel credentials, and
`scripts/mirror/.env` contains no Cloudflare keys. A tunnel also makes the
instance publicly reachable, which is the first step that could affect anything
outside this machine. Everything up to this point was local and disposable.

**Real data.** No production database or bucket was read from, let alone
written to, at any point in this exercise. The `.env` files hold working
credentials; rules 1–3 at the top of this file were the reason that did not
matter.

## State left behind

Containers `waifu-vtest-pg`, `waifu-vtest-s3`, `waifu-vtest-go` — disposable,
`docker rm -f` removes them. The seeder program was deleted. No repository file
was modified during this exercise.

---

## 2026-10-13 — Render environment check, and the two fixes it forced

The owner checked Render. **None of the three keys are set there.** That turned
finding 2 from a local nuisance into a guaranteed crash-loop, so it was chased to
its root rather than worked around by adding variables to Render.

### Root cause: the port invented a requirement

`ConfigurationKeys.Review.DefaultPageSize` and `MaxPageSize` are **declared in
the C# registry and read nowhere**. Not by `Program.cs`'s `requiredKeys` list,
not by any handler. Every other section's page size is required in both
services. The Go port treated the declaration as a requirement and added a
startup failure the reference implementation does not have.

`Permissions::SubmitEditMinRole` is the same shape: declared, never read in C#,
required by Go.

**Fixed in Go, not in Render.** Requiring a deployment to set two variables that
nothing reads is a trap, and adding them to Render would have hidden it.
`Review__*PageSize` and `Permissions__SubmitEditMinRole` are now optional with
the defaults `.env.example` already documents, so a deployment that does set
them behaves identically.

**Verified:** the container boots and serves `/ping` with an environment file
reproducing Render's exactly — C# log-level key, none of the three variables.
Before the fix that was a hard startup failure.

### Fix 2: the OOM, at source

`middleware.go` logged one line per request at **Info**. Changed to **Debug**,
`Debug` added to the `Logger` interface. Separately, `LOG_LEVEL` now falls back
to `Logging__LogLevel__Default` when unset, so an environment carried over from
the C# service behaves as it already does instead of silently defaulting to info.
`LOG_LEVEL` documented in `.env.example`.

**Verified by re-running the test that killed it, unchanged, with no
`LOG_LEVEL` set:**

| | before | after |
|---|---|---|
| 200 workers, 25s | **OOM-killed, exit 137** | 313 rps, 0 errors, 54 MB (10.6%) |

Not worked around this time — the cause was removed.

---

## Cloudflare edge test

`cloudflared tunnel --url` needs no account; it hands out a temporary
`trycloudflare.com` hostname. The Go container, still capped at 512 MB, exposed
through Cloudflare's edge.

| | result |
|---|---|
| `/ping` via tunnel | `pong` |
| `/stats/public` via tunnel | 500 images, correct |
| `/images?limit=20`, 25 workers, 30s | **209 rps, 0 failures, 0 non-2xx** |
| latency through the edge | p50 115ms, p99 234ms |
| memory | 21.9 MB / 512 MB (4.3%) |

Throughput is lower and latency higher than direct, which is the edge adding
two round trips rather than the service behaving differently. The direct figure
stayed at ~312 rps in the same session.

Full suite re-run after both fixes: all packages pass.

## Not done

- The tunnel hostname is ephemeral and dies with the process. Nothing was
  published, and no Cloudflare account was touched.
- Throughput plateaus at ~312 rps regardless of concurrency while latency scales
  linearly: an 8-connection pool, not a runtime problem. Pool size is the first
  tuning decision at cutover.

---

## 2026-10-13 — Corpus, oracle, and step 3 against production

### Credential leak

While checking which bucket was live, a `sed` intended to redact values printed
the access key and secret key in full into the session transcript. Both must be
treated as exposed and rotated. Self-inflicted; noted here so the rotation is
not lost.

### `backend/.env` is not production configuration

Proven three ways: `Cdn__BaseUrl` was the literal string
`placeholder-until-proxy-exists`; three required keys were missing; and
`S3__ServiceUrl` pointed at a bucket that does not exist. Real credentials live
in the host's own environment. `tools/corpuspull` pins its session read-only so
that distinction can never become a write.

### Storage is WebP, not PNG

`Image.Extension` says `.png` but every PNG is stored as `{id}.webp` and served
back at the `.png` path by the Worker. Reading the bucket directly therefore
missed **1,162 of 4,287** rows and reported them absent — indistinguishable from
data loss. `ObjectKeyCandidates` fixes it in one place; `tools/backfill` had the
identical gap and would have failed the same way.

Hashing the WebP is correct, not a compromise: the conversion happens before the
fingerprint is computed.

### Cross-architecture determinism

The concern was legitimate: if the hash moved with the CPU, every value written
today would rot on deployment. Tested rather than asserted — all 4,287 real
images hashed on amd64 (SSE2) and on 386 (x87 extended precision):

**2,135 consecutive images bit-for-bit identical.**

386 was chosen because 80-bit x87 versus 64-bit SSE2 is where float code
normally diverges. arm64 could not be executed here — no qemu registered, so
the binary will not run. Untested, and not the deployment target: Render is
x86-64, the architecture used here.

The C# failure was never inherent float instability. ImageSharp branches
explicitly on AVX2 and selects a different accumulation path. Go has no such
branch.

### Applied to production

1. Migration, by hand and atomically with its history row:

```sql
BEGIN;
ALTER TABLE "Images" ADD COLUMN "PerceptualHashLegacy" bit(64) NULL;
INSERT INTO "__EFMigrationsHistory" VALUES ('20261013120000_AddPerceptualHashLegacy', '10.0.2');
COMMIT;
```

The history row matters: without it the next C# deploy retries the migration,
hits "already exists", and fails *silently*, because `Program.cs:288` swallows
migration errors and serves anyway.

2. Oracle over the corpus: **4,287 hashed, 0 failed, 0 decode errors.**
3. `tools/legacyload`: **4,287 written.**

Verified afterwards: legacy populated on 4,287 of 4,287, zero all-zero values,
live `PerceptualHash` intact on every row, production `/ping` returning 200.

### Remaining: `tools/backfill`

Not yet run. It rewrites the column duplicate detection reads, so for the length
of the run the column is mixed and duplicate detection is partially degraded
**without saying so**. Dry run first; read the distance histogram before writing.

No maintenance notice is needed for the steps completed above — every one was
additive and non-blocking.
