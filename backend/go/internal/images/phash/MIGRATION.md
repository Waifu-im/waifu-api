# Perceptual hash migration

How to make duplicate detection work after the Go backend replaces the C# one.
Steps 1 and 2 are written and have never been run against production. The rest
is written down so the reasoning survives until there is time to act.

## The problem

`Images.PerceptualHash` holds about 4,285 hashes. They were produced by the C#
service, and **the C# service does not reproduce its own hashes.**

`ResizeKernel.ConvolveCore` accumulates with `Vector256.MultiplyAddEstimate` when
AVX2 is present and a plain float loop when it is not. Running the oracle twice
over one corpus, identical library versions, once normally and once with
`DOTNET_EnableHWIntrinsic=0`:

| | |
|---|---|
| Identical hashes | 248 / 554 |
| Differing | 306 / 554 |
| Largest divergence | 38 bits |

So the stored column is a **mixture of at least two algorithms**, decided by
whichever machine computed each row. There is no stable target to port to, and
no threshold that makes a mixed column consistent.

### Measured consequence

Go's hash, from `internal/images/phash`, against the stored value for the same
image — 150 real images, distances in bits:

```
distance 22:  1     distance 32: 26     distance 38: 18
distance 24:  4     distance 34: 24     distance 40:  1
distance 26: 10     distance 36: 19     distance 42:  1
distance 28: 18
distance 30: 28

within threshold (<= 4): 0 of 150
```

The threshold is 4. Observed distances cluster at 22–38, which is close to
random. **Duplicate detection against every historical row is dead on cutover,
and it fails silently** — a re-upload succeeds and nobody is told.

### What does not fix it

Running the C# code as a sidecar does not help. It would match only the rows
written on machines like this one, so dedup would go from 0% matched to roughly
half, at the cost of a .NET runtime in production, a network hop per upload, and
a second failure mode inside a 512 MB budget.

## The approach: two columns

Add a column rather than overwriting, so either implementation can be used and
the switch is reversible.

| Column | Written by | Read by |
|---|---|---|
| `PerceptualHash` (existing) | Go after backfill | Go |
| `PerceptualHashLegacy` (new) | C#, one-off tool | C# only, if rolled back |

Both are backfilled once. After that, whichever service is live maintains only
its own column, and the other is frozen history.

The cutover is not going to be undone, so the legacy column exists only to
preserve the C#-produced values while the Go column is being proven. It is
expected to be dropped once the Go column has demonstrably worked; that is a
deliberate decision, not an oversight, and `schema.sql` records it next to the
column definition.

## Steps

### 1. Migration — written

```sql
ALTER TABLE "Images" ADD COLUMN "PerceptualHashLegacy" bit(64);
```

Exists as `20261013120000_AddPerceptualHashLegacy`, with the entity property and
the model snapshot updated to match. The Go `schema.sql` was changed in step with
it and the two were checked against each other by applying the migration to a
database created from the old schema and diffing the result against one created
from the new: 88 columns and 12 indexes, identical.

Note that `schema.sql` uses `CREATE TABLE IF NOT EXISTS`, so it is documentation
rather than a migration mechanism. An existing database gets the column from the
EF migration, not from re-running it.

Populated in step 3, so the migration alone is safe.

### 2. Go backfill — written, never run

`tools/backfill` imports `internal/images/phash`, the same package the service
uses. Agreement is by construction, so nothing needs verifying — which is the
entire reason this is Go and not a script language.

- Reads the row ids from the database.
- Fetches each image from object storage. **Not** from the local mirror: that
  covers 4,279 rows plus six recovered, and misses anything uploaded since.
- Hashes and writes back in batches of 50, not one transaction for the whole
  run. Hashing happens outside any transaction; holding one open across ~4,285
  object reads would pin a pool slot for minutes on a backend still serving
  traffic.
- `-dry-run` writes nothing and prints before/after pairs.
- Reports the distribution of distances between the stored and recomputed value.
  That histogram is the real check: 22-38 bits confirms it hashed the images the
  column describes, and distances near zero mean it is re-reading something else.
- Refuses to write until `PerceptualHashLegacy` has been populated, because the
  values being overwritten cannot be recomputed once lost.
- Non-zero exit if any row fails, so an incomplete run is not mistaken for a
  complete one. `-after-id` resumes.
- Keyed by `{id}{extension}`, matching what the upload handler writes.

Cost is about 4,285 GETs, egress-free.

### 3. C# legacy backfill — the new column

`tools/oracle/HashGen.csproj` already does exactly this job: a .NET 10 console
app pinned to `ImageSharp 3.1.12` and `ImageHash 1.3.6`, calling
`new PerceptualHash().Hash(image)` as `ImageProcessingService.cs` does.

Point it at the image corpus and write to `PerceptualHashLegacy`. One-off, run
locally, never deployed. This is the only reason a C# component is involved at
all — a permanent service would be strictly worse for the same result.

### 4. Verify

After each backfill, re-read every row and re-compare. The distance histogram
should collapse to 0 for Go and to the 248/554 profile for C#. If it does not,
nothing has been written.

### 5. Re-validate the threshold

The value 4 was chosen assuming a stable hash. With a single Go-written column
it is meaningful again, but it has never been tuned against real data. Pick it
from the actual distribution of same-image versus different-image distances
rather than inheriting the old number.

## Sequencing

Run both backfills **after** Go is live, or immediately before. An upload in
between is corrected by the backfill, which recomputes from the image rather
than trusting the column.

Running them before cutover is also fine: C# would reject some duplicates in
that window, which is current behaviour and no worse.

## Open decisions

- ~~**Is reversibility actually required?**~~ Settled: the Go cutover is
  permanent. The legacy column is kept only to preserve the C# values while the
  Go column is proven, and is expected to be dropped afterwards.
- **When to drop the legacy column.** Not a date — a condition. Once duplicate
  detection has been seen working against real uploads with a threshold chosen
  from measured distances. `schema.sql` records this next to the column.
- **Column name.** `PerceptualHashLegacy` is a placeholder.
- **Whether the C# tool writes directly** or emits a TSV that is applied
  separately. It already emits a TSV (`tools/oracle` writes
  `relative<TAB>value` lines), so the missing piece is whatever loads that into
  the column. The TSV is safer and reviewable.
