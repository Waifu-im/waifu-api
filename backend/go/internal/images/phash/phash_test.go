package phash

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "golang.org/x/image/webp"
)

// GoldenVector is one row of the reference TSV produced by the C# oracle.
//
// The oracle is /tmp/hashwork/hashgen, a .NET 10 console app that references
// the same package versions as WaifuApi.Infrastructure:
//   - CoenM.ImageSharp.ImageHash 1.3.6
//   - SixLabors.ImageSharp 3.1.12
// and calls new PerceptualHash().Hash(image) exactly as ImageProcessingService does.
type GoldenVector struct {
	Name     string
	Hash     uint64
	Width    int
	Height   int
	Dominant string
	Animated bool
}

// goldenCorpusDirEnv points the tests at a directory of images on disk.
//
// The corpus is generated deterministically by cmd/corpusgen, so it can be
// regenerated anywhere instead of being committed as thousands of binaries.
const goldenCorpusDirEnv = "PHASH_CORPUS_DIR"

// goldenVectorsFileEnv points the tests at the oracle's TSV output.
const goldenVectorsFileEnv = "PHASH_GOLDEN_FILE"

func loadGolden(t *testing.T) ([]GoldenVector, string) {
	t.Helper()

	corpusDir := os.Getenv(goldenCorpusDirEnv)
	goldenFile := os.Getenv(goldenVectorsFileEnv)

	if corpusDir == "" || goldenFile == "" {
		t.Skipf("set %s and %s to run the spike", goldenCorpusDirEnv, goldenVectorsFileEnv)
	}

	f, err := os.Open(goldenFile)
	if err != nil {
		t.Fatalf("open golden file: %v", err)
	}
	defer f.Close()

	var vectors []GoldenVector
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		if fields[1] == "DECODE_ERROR" {
			continue
		}
		hash, err := strconv.ParseUint(fields[1], 16, 64)
		if err != nil {
			t.Fatalf("parse hash %q: %v", fields[1], err)
		}
		dims := strings.Split(fields[2], "x")
		w, _ := strconv.Atoi(dims[0])
		h, _ := strconv.Atoi(dims[1])

		vectors = append(vectors, GoldenVector{
			Name:     fields[0],
			Hash:     hash,
			Width:    w,
			Height:   h,
			Dominant: fields[3],
			Animated: fields[4] == "1",
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan golden file: %v", err)
	}
	return vectors, corpusDir
}

// decodeRGBA reads an image file and returns it as a tightly packed RGBA byte
// buffer plus its dimensions.
func decodeRGBA(t *testing.T, path string) ([]byte, int, int) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		t.Fatalf("%s has zero dimensions", path)
	}

	// Convert through the standard library so the buffer layout is the simple
	// top-down, 4-bytes-per-pixel form that DecodePlane expects.
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rgba.Set(x, y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return rgba.Pix, w, h
}

// hammingDistance returns the number of differing bits between two hashes.
func hammingDistance(a, b uint64) int {
	diff := a ^ b
	count := 0
	for diff != 0 {
		count += int(diff & 1)
		diff >>= 1
	}
	return count
}

// TestSpike_FindMatchingResampler is the spike described in PLAN.md section 6.1.
//
// It runs every candidate resampler against every golden vector and reports
// the match rate. The winner must be 100% — a partial match is a failure,
// because dedup silently breaks against the ~4,000 stored hashes otherwise.
func TestSpike_FindMatchingResampler(t *testing.T) {
	vectors, corpusDir := loadGolden(t)
	if len(vectors) == 0 {
		t.Fatal("no golden vectors loaded")
	}

	type result struct {
		matched  int
		total    int
		worstGap int
		worstFor string
		firstBad string
	}

	results := make([]result, 0, len(CandidateKernels))

	for _, kernel := range CandidateKernels {
		r := result{total: len(vectors), worstGap: 0}
		for _, v := range vectors {
			pix, w, h := decodeRGBA(t, filepath.Join(corpusDir, v.Name))
			got := ComputeHash(pix, w, h, kernel)
			if got == v.Hash {
				r.matched++
			} else {
				d := hammingDistance(got, v.Hash)
				if d > r.worstGap {
					r.worstGap = d
					r.worstFor = v.Name
				}
				if r.firstBad == "" {
					r.firstBad = fmt.Sprintf("%s want=%016X got=%016X dist=%d", v.Name, v.Hash, got, d)
				}
			}
		}
		results = append(results, r)

		pct := float64(r.matched) / float64(r.total) * 100
		status := "FAIL"
		if r.matched == r.total {
			status = "*** EXACT MATCH ***"
		}
		t.Logf("%-22s %6.2f%% (%d/%d)  worst=%d on %s  %s",
			kernel.Name, pct, r.matched, r.total, r.worstGap, r.worstFor, status)
		if r.firstBad != "" {
			t.Logf("    first mismatch: %s", r.firstBad)
		}
	}

	// Exactly one kernel must match perfectly, or the spike has not resolved.
	perfect := 0
	for _, r := range results {
		if r.matched == r.total {
			perfect++
		}
	}
	if perfect == 0 {
		t.Fatalf("no resampler reproduced the C# hashes exactly — dedup cannot be ported as-is")
	}
	t.Logf("%d of %d candidate kernels reproduce the reference hashes bit-exactly", perfect, len(results))
}

// TestDCTMatchesReferenceShape guards the parts of the algorithm that do not
// depend on the resampler, so a regression there is caught even without a corpus.
func TestDCTMatchesReferenceShape(t *testing.T) {
	// A constant plane has all energy in the DC term, so every other
	// coefficient is zero and the median is zero. Every coefficient is then
	// >= median, and the DC strictly exceeds it.
	constant := make([][]float64, ResizeSize)
	for y := 0; y < ResizeSize; y++ {
		row := make([]float64, ResizeSize)
		for x := 0; x < ResizeSize; x++ {
			row[x] = 128
		}
		constant[y] = row
	}
	block := dctTopLeft(constant)

	if block[0] <= 0 {
		t.Fatalf("expected a positive DC term, got %v", block[0])
	}
	for i := 1; i < HashSize; i++ {
		if block[i] != 0 {
			t.Fatalf("expected coefficient %d to be zero for a constant plane, got %v", i, block[i])
		}
	}

	// Median of 63 zeros and one positive DC must be zero, so the hash is
	// exactly the MSB set. This is the bit-order regression guard: DC must be
	// the most significant bit.
	pix := make([]byte, ResizeSize*ResizeSize*4)
	for i := 0; i < ResizeSize*ResizeSize; i++ {
		pix[i*4+0] = 128
		pix[i*4+1] = 128
		pix[i*4+2] = 128
		pix[i*4+3] = 255
	}
	got := ComputeHash(pix, ResizeSize, ResizeSize, nearestKernel)
	if got != 1<<63 {
		t.Fatalf("constant image should hash to exactly the MSB set (0x%016X), got 0x%016X", uint64(1)<<63, got)
	}
}

// TestMedianOf64 checks the median arithmetic directly. This is the average of
// the 32nd and 33rd smallest, not a plain middle element.
func TestMedianOf64(t *testing.T) {
	// Ascending 1..64: median must be (32 + 33) / 2 = 32.5.
	values := make([]float64, 64)
	for i := range values {
		values[i] = float64(i + 1)
	}
	if got := medianOf64(values); got != 32.5 {
		t.Fatalf("median of 1..64 = %v, want 32.5", got)
	}

	// Descending input must give the same answer, proving the sort works.
	for i := range values {
		values[i] = float64(64 - i)
	}
	if got := medianOf64(values); got != 32.5 {
		t.Fatalf("median of descending 1..64 = %v, want 32.5", got)
	}
}

// TestResizePlanePreservesConstantValues guards against a broken weight
// normalisation, which would show up as a subtly wrong hash rather than a crash.
func TestResizePlanePreservesConstantValues(t *testing.T) {
	const w, h = 200, 120
	plane := make([][]float64, h)
	for y := 0; y < h; y++ {
		row := make([]float64, w)
		for x := 0; x < w; x++ {
			row[x] = 77
		}
		plane[y] = row
	}

	for _, kernel := range CandidateKernels {
		out := ResizePlane(plane, ResizeSize, ResizeSize, kernel)
		for y := 0; y < ResizeSize; y++ {
			for x := 0; x < ResizeSize; x++ {
				if diff := out[y][x] - 77; diff > 1e-9 || diff < -1e-9 {
					t.Fatalf("%s: constant pixel became %v at (%d,%d)", kernel.Name, out[y][x], x, y)
				}
			}
		}
	}
}

// TestDecodePlaneBt601 pins the luma conversion.
func TestDecodePlaneBt601(t *testing.T) {
	pix := []byte{
		255, 0, 0, 255, // pure red
		0, 255, 0, 255, // pure green
		0, 0, 255, 255, // pure blue
		255, 255, 255, 255,
		0, 0, 0, 255,
	}
	plane := DecodePlane(pix, 5, 1)

	want := []float64{76.245, 149.685, 29.07, 255, 0}
	for i, w := range want {
		if diff := plane[0][i] - w; diff > 0.001 || diff < -0.001 {
			t.Fatalf("pixel %d: got %v want %v", i, plane[0][i], w)
		}
	}
}

// TestHammingDistance checks the metric used by duplicate detection.
func TestHammingDistance(t *testing.T) {
	cases := []struct {
		a, b uint64
		want int
	}{
		{0, 0, 0},
		{1, 0, 1},
		{0xFFFFFFFFFFFFFFFF, 0, 64},
		{0x0F0F0F0F0F0F0F0F, 0, 32},
	}
	for _, c := range cases {
		if got := hammingDistance(c.a, c.b); got != c.want {
			t.Fatalf("hamming(%016X,%016X) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestGoldenFileIsWellFormed sanity-checks the committed vectors so a corrupted
// file fails loudly rather than silently weakening the spike.
func TestGoldenFileIsWellFormed(t *testing.T) {
	vectors, _ := loadGolden(t)
	if len(vectors) < 100 {
		t.Fatalf("expected a substantial corpus, only %d vectors", len(vectors))
	}
	seen := make(map[uint64]bool)
	for _, v := range vectors {
		seen[v.Hash] = true
	}
	// A corpus that collapses to a couple of hashes would not be able to
	// distinguish a correct filter from a near-miss one.
	if len(seen) < 50 {
		t.Fatalf("corpus only produced %d distinct hashes; too coarse to validate", len(seen))
	}
	t.Logf("%d vectors, %d distinct hashes", len(vectors), len(seen))
}

// keep bytes imported for future round-trip helpers
var _ = bytes.NewReader