# Go Rewrite Plan — waifu-api backend

**Branch:** `go-rewrite`
**Target:** 4–6 months
**Status of existing backend:** stays in production for the entire rewrite. No cutover until the Go service has carried real traffic.

---

## 1. Why

The fork works. There is no bug driving this. The driver is maintainer
sustainability: the sole maintainer does not want to work in C#, and a fork
nobody wants to open rots faster than a mediocre one they enjoy maintaining.
Secondary benefit: the rewrite doubles as Go practice.

This is a legitimate reason, but it is also the kind of reason that produces
failed rewrites — specifically ones with no forcing function, no behavioural
specification, and no fixed end date. Everything below exists to compensate for
those three weaknesses.

---

## 2. Ground rules

These are non-negotiable. They are what make the rewrite survivable.

1. **The HTTP contract is frozen.** 58 endpoints, exact request/response shapes.
   Downstream integrations depend on it. The API's entire value proposition is
   "same contract, drop-in."
2. **The database is frozen.** Same Postgres 18 + pgvector, same 13 tables, same
   data. No schema migration, no dual-write, no backfill.
3. **Strangler, never big-bang.** Both services run side by side. Routes move
   one at a time. There is never a cutover weekend.
4. **The C# service is never deleted** until the Go service has handled
   production traffic through a full traffic cycle with no parity failures.
5. **Contract tests gate every route**, not just the final ones.

---

## 3. Non-goals

- Changing the feature set. Port what exists. New features go to whichever
  service currently owns that route.
- Re-implementing the Cloudflare Worker or CDN layer. That already works and is
  not in this repo's hot path.
- Re-deriving the API contract from the controllers. `openapi/WaifuApi.Web.json`
  is the authoritative spec and is already committed.

---

## 4. Scope inventory

Baseline measured at `c9c8ff6`:

| | |
|---|---|
| Handwritten C# (excl. migrations) | ~12,500 lines |
| Endpoints | 58 across 12 controllers |
| DB tables | 13 |
| Expected Go output | ~6,000–9,000 lines |

### 4.1 Controller parity checklist

| Controller | Route prefix | Endpoints | Phase |
|---|---|---|---|
| `PingController` | `/ping` | 1 | 2 |
| `ImagesController` | `/images` | 5 | 3 |
| `TagsController` | `/tags` | 6 | 3 |
| `ArtistsController` | `/artists` | 6 | 3 |
| `UsersController` | `/users` | 6 | 3 |
| `StatsController` | `/stats` | 2 | 4 |
| `BannerController` | `/banner` | 3 | 4 |
| `AuthController` | `/auth` | 5 | 4 |
| `AlbumsController` | `/users/{userId}/albums` | 8 | 5 |
| `ReportsController` | `/reports` | 6 | 5 |
| `StorageDiffController` | `/storage-diff` | 3 | 5 |
| `ReviewController` | `/review/tasks` | 7 | 6 |

Read path (`/images`, `/tags`, `/artists`, `/stats` reads) is Phase 3 — that is
where ~100% of traffic is, and it is the lowest-risk, highest-value work.

### 4.2 Infrastructure services to port

| .NET | Go equivalent | Risk |
|---|---|---|
| `ImageProcessingService` | own pHash + `x/image` | **HIGH** — see §6.1 |
| `S3Service` | `aws-sdk-go-v2` | Low |
| `TokenService` | `golang-jwt/jwt/v5` | Low |
| `DiscordService` | `net/http` + `encoding/json` | Low |
| `CloudflareCacheService` | `net/http` | Low |
| `BlacklistMiddleware` | `net/http` middleware | Low |
| `ExceptionHandlingMiddleware` | one error mapper | **MEDIUM** — see §6.3 |
| `RequestLoggingMiddleware` | `slog` middleware | Low |
| `StatsCleanupService` | `time.Ticker` goroutine | Low |
| `ReviewTaskCleanupService` | `time.Ticker` goroutine | Low |

---

## 5. Stack decisions

| Concern | Choice | Why | Rejected alternative |
|---|---|---|---|
| HTTP router | stdlib `ServeMux` (1.22+) | Path patterns built in, no dep. Only `StorageDiffController` needs a catch-all | chi (unnecessary until routing gets complex) |
| DB access | **sqlc** + pgx | Compile-time-checked SQL from type annotations. Closest to what EF was doing underneath, zero magic | GORM (slow, weak pgvector, you'll fight it) |
| Migrations | **goose** | Owns the existing schema; no ORM coupling | atlas (heavier) |
| Config | `envconfig` or hand-rolled + structs | Typed config — see §6.4, this is a real upgrade | Viper (heavy, well-known footguns) |
| Image decode/resize | `golang.org/x/image` | Decodes jpeg/png/gif/webp. **Encoding not needed** — pipeline is read-only | imaging (unmaintained) |
| Perceptual hash | **hand-written pHash** | See §6.1 | goimagehash (abandoned), Python sidecar (adds a service) |
| Validation | `go-playground/validator` | Struct tags, closest to FluentValidation feel | zod (JS-shaped, awkward in Go) |
| JWT | `golang-jwt/jwt/v5` + hand-rolled middleware | Exact control over string-role claim semantics | lestrrat-go/jwx (heavier) |
| Logging | stdlib `slog` | Structured, no dep | zerolog (marginal gain) |
| API spec | **oapi-codegen** + kin-openapi | Spec-first, generate types + validation middleware | swaggo (annotation-based, runtime cost) |
| Testing | stdlib `testing` + httptest | stdlib is sufficient | testify (marginal) |
| DI | **plain struct-of-constructors** | At 58 endpoints this is ~0 effort and simpler than DI | wire, dig (unnecessary codegen) |

---

## 6. Risk register

### 6.1 HIGH — Perceptual hash consistency with stored data
`ImageProcessingService.cs:24-83` uses `CoenM.ImageSharp.ImageHash`.
`UploadImage/Command.cs:64-67` searches the **entire** `Images` table and rejects
any row within **Hamming distance ≤ 4**.

**The question this spike answers is not "is my pHash correct."** It is: **do my
hashes match the hashes already stored in the database?**

This distinction is the whole risk. The table already holds roughly 4,000
hashes computed by ImageSharp. A Go pHash that is algorithmically valid but
produces different bits is a **total failure**, not a partial pass — every one of
those historical rows becomes invisible to dedup, duplicate uploads start
succeeding, and nothing in the system raises an error. The failure mode is
*silence*.

---

#### Spike status: IN PROGRESS — not closed

An oracle was built and run (`backend/go/tools/oracle`, a .NET 10 console app
using the exact pinned package versions) together with a deterministic 554-image
corpus (`backend/go/tools/corpusgen`). Golden vectors are committed at
`backend/go/internal/images/phash/testdata/golden-vectors.tsv`.

**Established with evidence:**

| Finding | Evidence |
|---|---|
| Default resampler is `KnownResamplers.Bicubic` | 554/554 planes byte-identical to reference |
| ImageSharp Bicubic **is Catmull-Rom** (`a = -0.5`) | `BicubicResampler.GetValue`, verified algebraically equivalent |
| The algorithm resizes to **64×64**, not 32 | `PerceptualHash.SIZE = 64` (textbook pHash uses 32) |
| Grayscale is **Bt601** luma, not a flat average | RGB(220,30,30) → 87 in both |
| The DC term is the **most significant bit** | `mask = 1UL << 63`, values consumed row-major from index 0 |
| Median is the **mean of the 32nd and 33rd** values | `OrderBy().Skip(31).Take(2).Average()` |
| Window bounds use an **integer** radius | `ResizeKernelMap.BuildKernel`: `radius = (int)Ceiling(scale*2)` |

**Two real bugs found and fixed in the Go implementation:**
1. The vertical resize pass returned the plane **transposed**, so every row
   read walked a single line. This produced flat output and looked like a
   filter problem.
2. Window bounds were computed from a **float** radius with
   `Floor`/`Ceiling`. ImageSharp uses an **integer** radius with
   `Ceiling`/`Floor`, which yields one fewer trailing tap on downscales.

**Where it currently stands:** lossless PNG images still mismatch at
**107/212** with Catmull-Rom. JPEG is 115/340, so lossy decoding is *not* the
dominant cause. Identity-resize and solid-colour cases match exactly.

**Leading hypothesis for the residual mismatch:** `ResizeKernel.ConvolveCore`
accumulates with `Vector256.MultiplyAddEstimate`, which is documented as a
lower-precision *estimate* rather than a true FMA. If so, the reference resize
may not be bit-reproducible even across different .NET machines — which would
mean exact reproduction is impossible without matching their exact SIMD path.

**This must be resolved before Phase 5**, because it determines whether Fallback
A/B are even reachable:

- *If the reference is reproducible:* finish matching the resize arithmetic.
- *If it is not:* the stored hashes were themselves produced by whatever SIMD
  path that machine used, so dedup cannot be bit-preserved at all. The decision
  then becomes an explicit product choice — re-hash the whole corpus (Fallback
  B) and accept a one-off dedup gap — not a porting problem to grind on.

#### Fallback paths, in order of preference

**A — Re-tune threshold.** Only valid together with B.

**B — Backfill all stored hashes (expensive, restores full function).**
Pick one algorithm, re-fetch every original from S3, recompute, write.
This is the fallback that actually recovers correct behaviour, and it is the
single largest operational risk in the rewrite:

- ~4,000 images, one S3 round-trip and hash computation each
- On Render's free tier (**0.1 CPU**, §6.6) this is not a script — budget in
  hours, not minutes
- **Free services do not support one-off jobs** (§6.6), so the backfill cannot
  run as a Render job. It must run from a self-hosted machine or a paid service
- Must be **resumable** (checkpointed by ID) and **rate-limited** — and
  checkpoints cannot live on the instance filesystem, which is ephemeral and
  wiped on every spin-down
- Must be **observable** — progress, error count, hash-change count
- Must be **reversible** — a buggy backfill corrupts dedup across the entire
  corpus, and undoing that means restoring from backup, not from git
- **Verify against the golden vectors before writing anything.** Run the new
  implementation in "dry run" mode against stored hashes and confirm the
  distribution of Hamming distances looks sane before the first write.
- Keep a pre-backfill dump of the existing `PerceptualHash` column. It is 4,000
  values; storing it costs nothing and it is the only cheap rollback.

**C — C# hash sidecar (guarantees consistency; defers the problem).**
Run the existing `ImageProcessingService` in a small container that the Go service
calls for hashing. This *guarantees* bit-exactness against stored data and makes
the problem disappear.

**Only choose this if A+B are judged worse than owning C#.** It does not remove
the C# maintenance burden — it reduces it to ~200 lines while adding a network
hop and a new failure mode on the upload path. Hashing is CPU-bound, and two
containers would compete for Render's 0.1 CPU (§6.6), so separating it makes
uploads *slower*, not faster.

**On Render free tier this option is effectively unavailable.** Free web services
cannot receive private network traffic (§6.6), so the sidecar could only be
reached over public HTTPS with authentication — strictly worse than the in-process
Go implementation. Fallback C now requires a paid plan or self-hosting.

If taken: document it as a deliberate, dated decision; keep it behind an
interface with a pure-Go implementation alongside; plan to delete it.

**Note on Python as the offload target:** it does not avoid this decision.
Python's `imagehash` implements the same pHash structure and will almost
certainly differ in resize filter, DC-term median handling, and DCT float
rounding — so it fails the same golden-vector test. If C# fails and nothing else
matches, Python is only worth choosing as the *new canonical algorithm* for a
backfill (B), not as a service to offload to.

- **Not offload candidates:** `S3Service`, `TokenService`, `DiscordService` and
  `CloudflareCacheService` all have first-class or trivial Go equivalents
  (§4.2). `EditPreparer`/`EditApplier` are per-request business logic, not
  isolated computation — offloading them buys no isolation and costs a round trip.

### 6.2 MEDIUM — EF Core → SQL
Biggest volume change. No change tracking, no lazy loading, no LINQ. Everything
becomes explicit SQL.

Pain is concentrated, not spread:
- `GetImages/Query.cs` — 25+ optional filter params → one explicit query builder
- `GetAdminStats/Query.cs` — 179 lines of `GroupBy`/`Sum`/`Average`
- `EditPreparer.cs` (213 lines) + `EditApplier.cs` (122 lines) — the most
  intricate business logic in the codebase, mutating four entity types behind a diff

`EditPreparer`/`EditApplier` are pure domain logic with no EF dependency, so they
translate almost mechanically once understood. **Understand them first** — that
is the actual work. The translation is not.

- **Mitigation:** port the complex query logic to Go early but behind the
  contract tests, so mistakes surface as diffs rather than as user reports.

### 6.3 MEDIUM — ProblemDetails contract
`frontend/src/services/api.ts` has a global interceptor that parses RFC7807
ProblemDetails and renders a toast for every failure. Break this shape and every
error message in the React frontend silently degrades. It is invisible in
backend-only testing.

- **Mitigation:** parity test covering 401/403/404/409/413/422/500 response
  bodies, byte-compared against C# output. Non-negotiable gate.

### 6.4 MEDIUM — Role and claim semantics
`Role.cs:3-9` is ordinal-compared (`ReviewPolicy.cs:82`,
`PermissionService.cs:59`): `User=0 < TrustedUser=1 < Moderator=2 < Admin=3`.
Both JWT (`TokenService.cs:26-32`) and API keys
(`ApiKeyAuthenticationHandler.cs:57-63`) emit the role as a **string name**, not
an int.

Preserve exactly: the enum ordering, the string claim values, and the ordinal
comparison. Add a Go test asserting the ordering so a future refactor can't
quietly reorder it.

Approve/reject on `/review/tasks/{id}/approve|reject` is **Moderator+** and that
is a product decision, not an implementation detail (`ReviewController.cs:189-212`).
Preserve.

### 6.5 MEDIUM — Upstream silence
No behavioural spec exists. The C# service *is* the specification, and it stops
existing the day it is deleted.

- **Mitigation:** golden-file corpus approach (§7). Contract tests run
  continuously against both implementations for the entire project, so the C#
  service stays valuable as an oracle right up to cutover.

### 6.6 MEDIUM — Free-tier constraints (verified 2026-10-11)
Render free web service specs, from Render's own docs:

| | |
|---|---|
| CPU | **0.1 CPU** (not 0.5 — corrected) |
| RAM | 512 MB |
| Spin-down | 15 min idle |
| Spin-up | **~1 minute** |
| Instance hours | **750 per workspace per month** |
| Bandwidth | 100 GB/mo, 500 build min/mo |
| Persistent disk | **Not supported** |
| Private network traffic | **Not supported** |
| One-off jobs | **Not supported** |
| SSH shell | Not available |

Four consequences, in order of importance:

**1. The AOT-vs-JIT cold-start advantage is largely irrelevant here.**
I previously claimed Go's ~50ms start beats .NET's ~1–5s JIT as a free-tier win.
That was overstated: Render's spin-up takes **~1 minute** on either runtime, so
the runtime difference is noise against it. **The real free-tier Go win is
memory** — .NET's ~150–300MB is a large fraction of the 512MB ceiling, which is
a genuine OOM risk on upload (ImageSharp + AWS SDK allocations), whereas Go at
~30MB has ample headroom. Keep the cold-start measurement for the paid tier,
where it does matter.

**2. Two services share ONE 750-hour monthly budget.** 750 hours ≈ 31 days of
continuous uptime. Running C# and Go simultaneously **doubles consumption**.
Spun-down services don't consume hours, so low-traffic periods are fine — but any
period where you want both services warm is capped at roughly half a month.
Budget this explicitly before committing to a strangler timeline.

**3. Free services cannot receive private network traffic.** This **eliminates**
the in-cluster sidecar option from §6.1 Fallback C. A C# or Python sidecar on
Render would have to be reached over public HTTPS with authentication — strictly
worse than the in-process Go implementation, so Fallback C is now effectively
ruled out on free tier.

**4. No one-off jobs and no persistent disk.** The §6.1 backfill cannot run as
a Render job and cannot stage checkpoints locally; it must run from a
self-hosted machine or a paid service. Any intermediate state (golden vectors,
the pre-backfill hash dump) must live in git or S3, never on the instance
filesystem — free instances lose local changes on every spin-down and redeploy.

- **Mitigation:** measure cold-start p95 before and after; track instance-hour
  consumption weekly during the overlap period.

### 6.7 MEDIUM — Oracle rot
The C# service is the only behavioural specification this project has (§6.5).
It now sits **unmaintained for six months** while all development happens in Go.

The failure mode is subtle and expensive: if the C# service drifts — a patch
here, a config change there — the contract harness starts failing on the
*oracle's* bugs rather than the port's. The natural response is to "fix the
baseline", which silently converts a spec into a moving target and erodes
confidence in every diff that reported green.

- **Mitigation:**
  - **Freeze the C# image.** Pin the deployed tag. No new deploys except
    security fixes.
  - **Never patch the oracle to make a test pass.** If the C# behaviour looks
    wrong, that is a *finding* — record it, decide deliberately whether Go
    should preserve the bug or fix it, and document the decision.
  - Run the full contract suite against the oracle **on a schedule** (weekly)
    as well as on demand, so rot is detected within days rather than months.
  - Keep the oracle runnable locally and in CI at all times. An oracle you cannot
    start is not an oracle.
  - Log every oracle behaviour that Go intentionally diverges from, in one file.
    That file is the honest record of the rewrite's behaviour changes.

### 6.8 LOW — Database expiry (orthogonal to language)
**RESOLVED** — production Postgres is **Supabase**, not Render. No Render free-tier
expiry, so the dated-outage risk is gone.

New considerations that come with Supabase:

- **Connection pooling.** Supabase caps direct connections (≈15 on free tier).
  `pgx`'s pool must be explicitly sized, or connect through Supavisor.
- **Supavisor transaction mode breaks prepared statements**, and `pgx` uses them
  by default. Use the session pooler, direct connections, or set
  `QueryExecModeSimpleProtocol`. Decide in Phase 0 — this is a day-one decision
  because every query path depends on it.
- **pgvector is available**, so `PerceptualHash` and its `HammingDistance`
  query keep working unchanged.
- **Free-tier inactivity pause** — if enabled on the project, an instance with no
  traffic can be paused by Supabase. Verify the project's pause behaviour and
  keep the uptime monitor in place. This is the same class of risk as the Render
  expiry, just a different trigger.

### 6.9 Accepted / not worth solving
- No server-side cache, no rate limiter in the C# service, so nothing to port.
  (Whether the public API *should* have them is a separate question — out of
  scope, revisit post-cutover.)
- EF migration safety net is genuinely lost. Mitigated by goose + the frozen
  schema + contract tests.

---

## 7. Contract testing strategy

The single most important mechanism in this plan. With no behavioural spec, this
is the only thing preventing a silent regression that surfaces months later in
someone else's integration.

**Harness:** a shared fixture corpus of request/response pairs. Each fixture
declares a request against both implementations and records the normalized JSON
response.

**Execution:**
- A Go test suite runs against the Go implementation.
- A containerised C# instance runs the same fixtures as an oracle.
- `parity-check` diffs the two, normalising only volatile fields (timestamps,
  IDs, tokens).

**Normalisation rules** — normalise exactly, no more:
- Timestamps → sentinel
- Auto-increment IDs → sentinel, but **relationships between IDs preserved**
- JWT/API keys → sentinel, but **decoded claim sets compared**

**Gate:** a route may only be moved to the Go service once its fixtures pass at
100% with zero unexplained diffs.

**Coverage:** every endpoint, every documented error path, every filter
permutation on `/images`, and the dedup rejection path specifically.

---

## 8. Phases

Each phase has an explicit exit criterion. Do not start the next phase until the
current one is met.

### Phase 0 — Foundation (Week 1–2)
**Exit: `/healthz` and `/ping` served by Go, deployed to Render, parity-verified.**

- [ ] Repo layout: `cmd/server`, `internal/{http,domain,store,auth,images,config}`
- [ ] Postgres + pgvector connection via pgx, health check
- [ ] sqlc codegen against the frozen schema (read-only queries first)
- [ ] Typed config — **no** silent fallbacks. Unknown role value = startup panic.
      (This is a deliberate upgrade over `ReviewPolicy.cs:77`, where a typo
      silently degrades to `TrustedUser`.)
- [ ] slog structured logging + request logging middleware
- [ ] ProblemDetails error type + mapper — **port this early**, it is load-bearing
      for the frontend
- [ ] **Spike perceptual hash against stored-hash golden vectors (§6.1). Blocking.**
      Vectors must come from images already in the `Images` table, not freshly
      computed — the target is the stored hashes.
- [ ] Decide the Supabase connection mode: direct vs Supavisor session pooler
      vs transaction pooler with `QueryExecModeSimpleProtocol` (§6.8). This is a
      day-one decision because every query path depends on it.
- [ ] Decide repo layout (§12) and stand up the Go CI pipeline

### Phase 1 — Contract harness (Week 3–4)
**Exit: fixture runner green against C#; Go stubs wired but returning 501.**

- [ ] Fixture format and runner
- [ ] C# oracle container
- [ ] Normalisation rules implemented and reviewed
- [ ] Route registry so fixtures bind to whichever service currently owns a route

### Phase 2 — Read path, scaffold (Week 5–7)
**Exit: `/ping` + the simplest `/images` reads routed to Go in production.**

- [ ] `GET /images` — the 25+ parameter filter as explicit sqlc queries
- [ ] Tag and artist resolution/expansion
- [ ] CDN URL construction (`Cdn:BaseUrl`)
- [ ] First live route move, with automatic rollback if error rate rises

### Phase 3 — Read path, complete (Week 8–10)
**Exit: all GET routes on `/images`, `/tags`, `/artists` moved; parity green.**

- [ ] Nsfw / orientation / animated / resolution / size filters
- [ ] Exclude / force-include file lists
- [ ] Sort modes incl. random
- [ ] `GET /tags`, `GET /artists` incl. pagination
- [ ] Public stats aggregation

### Phase 4 — Auth, users, banner (Week 11–13)
**Exit: Discord login works end to end; JWT and API keys issued by Go.**

- [ ] Discord OAuth (`AuthController`, 5 endpoints)
- [ ] JWT issue/validate with exact string-role claim semantics (§6.4)
- [ ] API key authentication handler
- [ ] Blacklist middleware
- [ ] `GET /users`, `GET /users/me`, role reporting
- [ ] Banner read/write incl. **`GET /banner` returns 204 when empty** —
      `useBanner.ts` depends on that distinction
- [ ] Upload permission gate — mirror `Permissions__*` semantics, including the
      fail-**open** behaviour on blank values (`PermissionService.cs:53`).
      Decide deliberately whether to preserve or fix, and document the choice.

### Phase 5 — Write path, non-review (Week 14–17)
**Exit: uploads accepted by Go; images visible in the API after review.**

- [ ] `POST /images/upload` — multipart, validation, dimension checks
- [ ] Perceptual hash + Hamming dedup (must match C# exactly)
- [ ] S3 upload, rollback on failure
- [ ] Transactional image + review-task creation
- [ ] Albums (8 endpoints)
- [ ] Reports (6 endpoints)
- [ ] Storage diff (3 endpoints)

### Phase 6 — Review workflow (Week 18–20)
**Exit: full submit → queue → approve/reject cycle working on Go.**

- [ ] `EditPreparer` and `EditApplier` ported
- [ ] Review policy: 3 master switches + 6 bypass keys, `"None"` magic string
- [ ] Approve/reject, Moderator+ enforced
- [ ] Task listing, scoping non-mods to their own tasks
- [ ] Cleanup goroutines

### Phase 7 — Hardening + cutover (Week 21–24)
**Exit: 100% of routes on Go; C# still running, unowned, retained.**

- [ ] Full contract suite green, zero unexplained diffs
- [ ] Load test vs C# baseline — Go must not regress on p95
- [ ] Cold-start p95 measured (§6.6)
- [ ] Dockerfile: multi-stage, `distroless`/`scratch`, ~20MB target
- [ ] Graceful shutdown, connection draining
- [ ] Structured logging, error tracking, uptime alerting
- [ ] Go test coverage on domain logic — target 80%+
- [ ] Docs update, `.env.example` rewrite for the Go service
- [ ] **C# service retained but unowned for one full traffic cycle**

### Phase 8 — Decommission (post-stability, not on the rewrite timeline)
- [ ] Only after a stable cycle: archive the C# service, keep the repo branch
      as historical reference
- [ ] Close the contract harness against the frozen C# golden files

---

## 9. Deployment topology

Both services deployed simultaneously. Traffic split by route prefix via an
edge router (Cloudflare Worker — already in front for images, so no new
infrastructure).

- C# owns the routes not yet migrated
- Go owns migrated routes
- Either can be reverted per-route by changing the routing table, without a
  redeploy
- Shared Postgres throughout

---

## 10. Definition of done

The rewrite is complete when:

1. All 58 endpoints served by Go, passing contract tests at 100%
2. ProblemDetails error shapes byte-identical
3. Perceptual hash bit-identical on the 1,000-vector corpus
4. Role/claim semantics preserved, ordering asserted by test
5. `GET /banner` 204 behaviour preserved
6. Multipart field names preserved (`tags` = slugs, `artists` = IDs)
7. p95 latency no worse than C# baseline
8. Cold-start p95 improved, not regressed
9. Go service has served production traffic through a full traffic cycle
10. C# service retained and revertible

---

## 11. Schedule risk

The 4–6 month estimate assumes Go fluency. Learning Go concurrently roughly
doubles it. This is the single largest schedule risk and it is **not** mitigated
by any technical measure — only by starting Phase 0 now and by being honest about
the difference between "6 months to a working system" and "6 months to a working
system plus everything in §8 that slipped."

Budget explicitly for: the pHash spike (§6.1) going wrong, contract drift found
late, and behaviours discoverable only by reading C#.

### 11.1 What AI assistance does and does not compress

Working with AI agents will make this finish faster than 6 months, but not
uniformly. The split matters because it determines which phases are safe to
accelerate and which still need unhurried human attention.

**Compresses well** (mechanical translation, well-specified inputs):
- Handler and DTO boilerplate — the 58 endpoints are highly regular
- sqlc query generation from the frozen schema
- S3 / JWT / Discord / Cloudflare wrappers against known-good Go libraries
- Test scaffolding for the contract harness
- Migrating `ReviewPolicy`'s switch statements to typed Go config

**Does not compress** (these are the actual risk):
- **The pHash spike (§6.1).** Requires generating golden vectors from stored
  hashes and experimenting across four resize filters. An agent cannot verify
  bit-exactness without running it — and getting a plausible-looking-but-wrong
  hash is the exact failure mode here. Budget it at full human time.
- **The contract harness (§7).** The harness *is* the specification. Rushing it
  because an agent scaffolded it fast is how silent drift gets through.
- **Triaging contract diffs.** Distinguishing a real regression from benign
  variance requires product judgement, not pattern matching.
- **The backfill (§6.1 Fallback B).** Operational discipline, not code.
- **Reading `EditPreparer`/`EditApplier`.** Understanding 335 lines of diff
  semantics before porting it is comprehension work.

**Therefore:** treat §8's Phase 0 and Phase 1 as fixed-cost regardless of
assistance, and expect Phases 2–5 to compress substantially. Do not let a
compressed timeline compress the spike — it is the one item where being wrong is
silent and expensive.

---

## 12. Open decision — repo layout and CI

**This blocks Phase 0 and has not been decided.** It determines where the Go
source lives, how it builds, and whether the existing pipeline serves one
backend or two.

### Option A — same repo, new top-level directory

```
backend/
  WaifuApi.*          # C#, frozen, oracle only
  go/                 # or api-go/
    cmd/server
    internal/
```

- **Pro:** one clone, one place to read the C# while porting it. Cross-language
  diffs and shared docs are trivial. The contract harness can live in the same
  repo and reference both implementations directly.
- **Con:** two build systems in one repo; the Dockerfile and CI need to know
  which is which. Slight risk of accidentally "fixing" the frozen C#.

### Option B — separate repository

- **Pro:** clean separation. The Go repo contains only Go. Go-native tooling
  (`golangci-lint`, `go test ./...`) is uncontaminated. The C# repo stays frozen
  by construction, which is a real defence against oracle rot (§6.7).
- **Con:** the contract harness must either live in the Go repo (and shell out
  to the C# service) or in a third repo. Cross-repo reading during the port is
  worse — you are constantly comparing the two.

### Recommendation

**Option B, with the contract harness in the Go repo.** The strongest argument is
§6.7: making the frozen C# structurally impossible to edit is worth more than
the convenience of a shared checkout. During the port you are reading C#
constantly, and a `git blame` on the old repo is a perfectly good way to do that.

If Option A is chosen instead, enforce the freeze with CODEOWNERS or branch
protection rather than convention — a same-repo freeze held only by discipline
will not survive six months.

### CI, either way

- [ ] Go pipeline: `go build`, `go vet`, `gofmt -l`, `golangci-lint`, `go test ./...`
- [ ] Contract suite runs on **every PR**, against a live C# oracle container
- [ ] Weekly scheduled contract run to detect oracle rot (§6.7)
- [ ] Multi-stage Dockerfile → `distroless`/`scratch`, ~20MB target
- [ ] Existing C# CI keeps running unchanged — the oracle must stay buildable
- [ ] Image published to GHCR alongside the existing backend image, separately tagged

### Deployment topology note

Both services are deployed simultaneously for the whole rewrite. Verified Render
free-tier constraints (§6.6) that this must respect:

- **0.1 CPU / 512 MB per service.** .NET at ~150–300MB is a large share of the
  ceiling; Go at ~30MB is not. This is the strongest free-tier argument for the
  rewrite and it should be stated as such.
- **750 instance hours are shared per workspace, not per service.** Two services
  alive simultaneously halve each one's effective uptime. Spun-down services cost
  nothing, so the overlap is only affordable while traffic is low enough to
  keep both idle most of the time. **Track this weekly from Phase 2 onward** — it
  is the constraint most likely to bite first.
- **Free services cannot receive private network traffic.** The two services
  cannot talk over Render's private network; they are separate public origins, or
  fronted by the existing Cloudflare Worker as the routing layer (§9).
- **Go is natively supported** by Render's build detection, so no custom build
  command is needed for the non-Docker path — though the Dockerfile is still
  preferred for image-size control.

**Consider a paid plan for the overlap period.** A single 0.5c-512mb or
1c-2g service would remove the instance-hour pressure, the CPU starvation, and
the spin-down cold start in one move. At ~$7/month it is cheap insurance
against a rewrite timeline that silently stalls on a free-tier limit.

### Frontend

Out of scope and frozen. The React frontend consumes the API and must not
require changes for this rewrite — the contract freeze (§2) is what guarantees
that. If a frontend change ever becomes necessary, that is a signal the contract
was violated, not a task to pick up.