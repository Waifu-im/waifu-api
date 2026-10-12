# Perceptual-hash oracle

This tool generates the **reference hashes** that the Go implementation must
reproduce bit-for-bit. See `internal/images/phash` for what it concluded.

## Why it exists

`UploadImage/Command.cs:64-67` rejects an upload when its Hamming distance to
any row in the `Images` table is <= 4. Roughly 4,000 rows already hold hashes
computed by the C# service. If the Go implementation produces a different hash,
every one of those rows becomes invisible to duplicate detection and duplicates
silently start getting through. The failure mode is silence.

## What it is

A .NET 10 console app that calls the exact library versions pinned in
`backend/WaifuApi.Infrastructure/WaifuApi.Infrastructure.csproj`:

- `CoenM.ImageSharp.ImageHash` 1.3.6
- `SixLabors.ImageSharp` 3.1.12

and calls `new PerceptualHash().Hash(image)` exactly as
`ImageProcessingService.cs` does.

## Running it

```bash
# 1. generate the corpus (deterministic)
go run ./tools/corpusgen /tmp/corpus

# 2. generate reference hashes inside the .NET SDK container
docker run --rm -v "$PWD/tools/oracle:/src" -v /tmp/corpus:/input:ro -v /tmp/out:/out \
  -w /src mcr.microsoft.com/dotnet/sdk:10.0 \
  bash -c 'dotnet restore -v quiet && dotnet build -c Release -o /out/bin -v quiet \
           && dotnet /out/bin/HashGen.dll /input /out/golden.tsv'
```

Output is a TSV: `<path>\t<hash:X16>\t<width>x<height>\t<dominant>\t<isAnimated>`.

## Current state

**Bit-exact porting was ruled out.** The findings below are why.

Established so far:

| Finding | Evidence |
|---|---|
| Default resampler is `KnownResamplers.Bicubic` | 554/554 planes identical to the reference |
| Bicubic is Catmull-Rom (`a = -0.5`) | `BicubicResampler.GetValue`, verified algebraically |
| Algorithm resizes to **64x64**, not 32 | `PerceptualHash.SIZE = 64` |
| Grayscale is **Bt601** luma | matches: RGB(220,30,30) -> 87 in both |
| DC term is the **most significant bit** | `mask = 1UL << 63` |
| Median is mean of 32nd and 33rd | `OrderBy().Skip(31).Take(2).Average()` |
| Window bounds use an **integer** radius | `ResizeKernelMap.BuildKernel` |

Two genuine bugs were found and fixed on the Go side: a transposed output
plane, and window bounds computed from a float radius with `Floor`/`Ceiling`
instead of ImageSharp's integer radius with `Ceiling`/`Floor`.

## Outcome

**The reference does not reproduce itself.** Running the oracle twice on the
same 554-image corpus — once normally, once with `DOTNET_EnableHWIntrinsic=0` —
with identical library versions:

| | |
|---|---|
| Hashes identical between the SIMD and scalar runs | 248 / 554 |
| Hashes differing | 306 / 554 |
| Largest divergence | 38 bits |
| Diverging hashes still within the dedup threshold (<= 4) | 40 / 306 |

The cause is `ResizeKernel.ConvolveCore`, which accumulates with
`Vector256.MultiplyAddEstimate` when AVX2 is present and a plain float loop when
it is not. Those paths do not agree, so the ~4,000 hashes already stored were
produced by at least two algorithms, decided by whichever machine computed them.
There was never a stable target to port to.

Consequence: the Go hash does not match historical rows, so duplicate detection
is blind across the boundary until a rehash backfill runs. It is worth knowing
that this was a property of the C# service, not a defect in the port.

## Do not resume filter tuning

The work above was worth doing — it found two genuine bugs — but it is closed.
Tuning the kernel further cannot close a gap whose far side moves with the CPU
it ran on. `internal/images/phash` is the shipped implementation: deterministic
and self-consistent, which is what a backfill actually needs.
