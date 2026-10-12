# Perceptual hash migration

How to make duplicate detection work after the Go backend replaces the C# one.
Not yet done. Written down so the reasoning survives until there is time to act.

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

## Steps

### 1. Migration

```sql
ALTER TABLE "Images" ADD COLUMN "PerceptualHashLegacy" bit(64);
```

Populated in step 3, so the migration alone is safe.

### 2. Go backfill — the primary column

A Go program importing `internal/images/phash`, the same package the service
uses. Agreement is by construction, so nothing needs verifying — which is the
entire reason this is Go and not a script language.

- Read the row ids from the database.
- Fetch each image from Tigris. **Not** from the local mirror: that covers 4,279
  rows plus six recovered, and misses anything uploaded since.
- Hash and write back inside a single transaction.
- `--dry-run` prints the first few before/after pairs.

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

- **Is reversibility actually required?** If the cutover is permanent, the legacy
  column and the C# tool can both be skipped. That is half the work and one less
  column to reason about.
- **Column name.** `PerceptualHashLegacy` is a placeholder.
- **Whether the C# tool writes directly** or emits a TSV that is applied
  separately. The TSV is safer and reviewable.
- **Retention.** After the Go cutover is stable, the legacy column is dead weight
  on every row. Decide when to drop it.
