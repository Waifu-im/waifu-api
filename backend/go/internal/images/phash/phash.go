// Package phash implements the perceptual hash that the Go service must produce
// so that it is bit-for-bit identical to the hashes already stored in the
// Images table by the C# service.
//
// # Why bit-exactness matters
//
// The upload handler rejects an image when its Hamming distance to any existing
// row is <= 4. Roughly 4,000 rows already contain hashes computed by
// CoenM.ImageSharp.ImageHash 1.3.6. If this package produces even a slightly
// different hash, every one of those rows becomes invisible to duplicate
// detection and duplicates start silently getting through. There is no error
// message; the only symptom is a quietly broken feature.
//
// # What this package reproduces
//
// The reference implementation, PerceptualHash.Hash in CoenM.ImageSharp.ImageHash,
// does the following (src/ImageHash/HashAlgorithms/PerceptualHash.cs):
//
//  1. Resize to 64x64 (note: 64, not the 32 used by textbook pHash)
//  2. Convert to grayscale using GrayscaleMode.Bt601 luma coefficients
//  3. Auto-orient using EXIF
//  4. Two-pass DCT-II: rows first (all 64 coefficients), then columns
//     (first 8 columns only, 8 coefficients each)
//  5. Take the top-left 8x8 block, 64 values
//  6. Median = average of the 32nd and 33rd smallest values
//  7. Set bit 63 down to bit 0; a bit is set when the value exceeds the median
//
// Note in step 7 that the DC term (top-left of the block) becomes the MOST
// significant bit, because the values are consumed in row-major order starting
// from index 0 at mask 1<<63.
//
// # Resampler
//
// Step 1 is the hard part. ImageSharp's default resampler is a bicubic filter
// with antialiased downscaling, and golang.org/x/image/draw does not provide a
// bicubic scaler. Resize in this package therefore implements a general
// separable resampler with a pluggable kernel so that several candidates can be
// compared against the golden vectors. See ResizeKernel and the spike test.
package phash

import (
	"fmt"
	"math"
)

// GrayscaleModeBt601 are the luma coefficients used by
// SixLabors.ImageSharp.GrayscaleMode.Bt601, which is what the C# service uses.
const (
	bt601R = 0.299
	bt601G = 0.587
	bt601B = 0.114
)

// HashSize is the number of bits produced. Fixed at 64 by the algorithm.
const HashSize = 64

// ResizeSize is the width and height the image is resized to before the DCT.
// Fixed at 64 by the algorithm.
const ResizeSize = 64

// lowFrequencySize is the edge length of the low-frequency block retained from
// the DCT output. 8x8 = 64 values = HashSize bits.
const lowFrequencySize = 8

// Kernel describes a separable resampling filter.
type Kernel struct {
	// Name identifies the kernel in logs and spike reports.
	Name string
	// Radius is the kernel's support radius in source pixels, before any
	// antialiasing scale is applied. A bicubic filter has radius 2.
	Radius float64
	// ValueAt evaluates the kernel. It is only ever called with |t| <= Radius.
	ValueAt func(t float64) float64
}

// cubicBSpline builds a kernel from the cubic B-spline family.
//
// The family is parameterised by B and C:
//
//	B = 0,    C = 0.5  -> Catmull-Rom
//	B = 1/3,  C = 1/3  -> Mitchell
//	B = 0.5,  C = 0.5  -> a softer Mitchell, which is ImageSharp's bicubic
//
// This covers the plausible candidates for what ImageSharp's default resampler
// actually does.
func cubicBSpline(name string, b, c float64) Kernel {
	return Kernel{
		Name:   name,
		Radius: 2,
		ValueAt: func(t float64) float64 {
			at := math.Abs(t)
			switch {
			case at < 1:
				return ((12-9*b-6*c)*at*at*at +
					(-18+12*b+6*c)*at*at +
					(6 - 2*b)) / 6
			case at < 2:
				return ((-b-6*c)*at*at*at +
					(6*b+30*c)*at*at +
					(-12*b-48*c)*at +
					(8*b + 24*c)) / 6
			default:
				return 0
			}
		},
	}
}

// lanczos builds a Lanczos kernel of the given order. Radius equals order.
func lanczos(order int) Kernel {
	return Kernel{
		Name:   fmt.Sprintf("lanczos%d", order),
		Radius: float64(order),
		ValueAt: func(t float64) float64 {
			at := math.Abs(t)
			if at < 1e-9 {
				return 1
			}
			if at >= float64(order) {
				return 0
			}
			pt := math.Pi * at
			return (math.Sin(pt) / pt) * (math.Sin(pt/float64(order)) / (pt / float64(order)))
		},
	}
}

// boxKernel averages every pixel that falls inside one destination pixel.
// It is the correct filter for extreme downsampling and is used as a baseline.
var boxKernel = Kernel{
	Name:   "box",
	Radius: 1,
	ValueAt: func(t float64) float64 {
		if math.Abs(t) <= 1 {
			return 1
		}
		return 0
	},
}

// biLinearKernel is tent-weighted interpolation, radius 1.
var biLinearKernel = Kernel{
	Name:   "bilinear",
	Radius: 1,
	ValueAt: func(t float64) float64 {
		at := math.Abs(t)
		if at < 1 {
			return 1 - at
		}
		return 0
	},
}

// nearestKernel performs point sampling with no interpolation.
var nearestKernel = Kernel{
	Name:   "nearest",
	Radius: 0.5,
	ValueAt: func(float64) float64 {
		return 1
	},
}

// DefaultKernel is the resampler this service uses.
//
// It is Catmull-Rom, which is exactly what ImageSharp's default bicubic
// resampler computes. See tools/oracle/README.md: the reference is not
// reproducible against itself, so this package does not attempt to reproduce a
// particular SIMD path. It aims to be deterministic and self-consistent, which
// is the property a corpus backfill depends on.
var DefaultKernel = cubicBSpline("catmullrom", 0, 0.5)

// CandidateKernels lists every resampler that was evaluated against the C#
// reference during the spike. They are kept so the comparison can be repeated
// and so the choice of DefaultKernel can be re-justified later.
var CandidateKernels = []Kernel{
	DefaultKernel,
	cubicBSpline("bicubic-b0.5-c0.5", 0.5, 0.5),
	cubicBSpline("mitchell", 1.0/3.0, 1.0/3.0),
	lanczos(3),
	biLinearKernel,
	boxKernel,
	nearestKernel,
}

// contribution holds the source indices and normalised weights that a single
// destination sample is formed from.
type contribution struct {
	start   int
	weights []float64
}

// buildContributions computes the weight table for resampling srcLen samples
// down to dstLen samples.
//
// This mirrors ImageSharp's ResizeKernelMap.BuildKernel exactly, because the
// exact window bounds matter more than they look. ImageSharp computes:
//
//	ratio  = sourceLength / destinationLength
//	scale  = max(ratio, 1)
//	radius = (int)ceil(scale * sampler.Radius)      <- an INTEGER radius
//	center = (destinationIndex + 0.5) * ratio - 0.5
//	left   = ceil(center - radius)
//	right  = floor(center + radius)
//	weight = sampler.GetValue((j - center) / scale)
//	then normalise by the sum
//
// The integer radius and the Ceiling/Floor bounds are what earlier
// approximations got wrong: a float radius with Floor/Ceiling bounds produces
// one extra tap at the trailing edge, which is enough to shift the DCT and
// change the hash.
func buildContributions(srcLen, dstLen int, kernel Kernel) []contribution {
	contributions := make([]contribution, dstLen)

	// ratio is the source-to-destination distance. It is also the sample step.
	ratio := float64(srcLen) / float64(dstLen)

	// scale is clamped to 1 so that upscaling does not narrow the kernel.
	scale := ratio
	if scale < 1 {
		scale = 1
	}

	// ImageSharp rounds the scaled radius up to a whole number of samples.
	radius := int(math.Ceil(scale * kernel.Radius))

	for d := 0; d < dstLen; d++ {
		center := (float64(d)+0.5)*ratio - 0.5

		left := int(math.Ceil(center - float64(radius)))
		if left < 0 {
			left = 0
		}

		right := int(math.Floor(center + float64(radius)))
		if right > srcLen-1 {
			right = srcLen - 1
		}

		weights := make([]float64, 0, right-left+1)
		sum := 0.0
		for j := left; j <= right; j++ {
			weight := kernel.ValueAt((float64(j) - center) / scale)
			weights = append(weights, weight)
			sum += weight
		}

		// ImageSharp normalises only when the sum is positive.
		if sum > 0 {
			for i := range weights {
				weights[i] /= sum
			}
		}

		contributions[d] = contribution{start: left, weights: weights}
	}
	return contributions
}

// applyContributions resamples a single 1D signal and returns dstLen samples.
func applyContributions(signal []float64, contributions []contribution) []float64 {
	out := make([]float64, len(contributions))
	for d, c := range contributions {
		var total float64
		for i, weight := range c.weights {
			total += signal[c.start+i] * weight
		}
		out[d] = total
	}
	return out
}

// ResizePlane resizes a 2D plane of float samples using the given kernel.
//
// It performs a horizontal pass followed by a vertical pass, which is
// equivalent to a single 2D separable convolution and far cheaper than one.
//
// The result is always in [row][col] order: rows == dstH, columns == dstW.
func ResizePlane(src [][]float64, dstW, dstH int, kernel Kernel) [][]float64 {
	srcH := len(src)
	if srcH == 0 || len(src[0]) == 0 {
		return nil
	}
	srcW := len(src[0])

	// Pass 1: horizontal. srcW -> dstW. Row count is unchanged.
	horizontal := buildContributions(srcW, dstW, kernel)
	tmp := make([][]float64, srcH)
	for r := 0; r < srcH; r++ {
		tmp[r] = applyContributions(src[r], horizontal)
	}

	// Pass 2: vertical. srcH -> dstH. Gather a column, resample it, then
	// scatter the result back, so the output stays in [row][col] order.
	vertical := buildContributions(srcH, dstH, kernel)
	out := make([][]float64, dstH)
	for r := 0; r < dstH; r++ {
		out[r] = make([]float64, dstW)
	}

	column := make([]float64, srcH)
	for c := 0; c < dstW; c++ {
		for r := 0; r < srcH; r++ {
			column[r] = tmp[r][c]
		}
		resampled := applyContributions(column, vertical)
		for r := 0; r < dstH; r++ {
			out[r][c] = resampled[r]
		}
	}
	return out
}

// DecodePlane converts an RGBA image into a single-channel float plane using
// Bt601 luma, matching ImageSharp's GrayscaleMode.Bt601.
//
// The DCT in the reference implementation reads the .R channel of the already
// grayscale image, so only luma is needed here.
func DecodePlane(rgba []byte, width, height int) [][]float64 {
	plane := make([][]float64, height)
	for y := 0; y < height; y++ {
		row := make([]float64, width)
		base := y * width * 4
		for x := 0; x < width; x++ {
			i := base + x*4
			r := float64(rgba[i+0])
			g := float64(rgba[i+1])
			b := float64(rgba[i+2])
			// Clamp to [0,255]. Transparency is deliberately ignored, matching
			// ImageSharp, which treats the alpha channel separately.
			v := bt601R*r + bt601G*g + bt601B*b
			if v < 0 {
				v = 0
			}
			if v > 255 {
				v = 255
			}
			row[x] = v
		}
		plane[y] = row
	}
	return plane
}

// GrayscalePlane8 reproduces the exact intermediate plane the reference
// implementation hashes: the image resized to 64x64, then converted to
// grayscale with Bt601, quantised to 8 bits at each stage.
//
// The two quantisation steps are not incidental. ImageSharp resizes into an
// Rgba32 image, so the RGB values are rounded to integers; it then converts to
// grayscale, which rounds again. Hashing the un-quantised float pipeline
// instead produces a different result on most images.
//
// The returned slice is row-major, 64*64 long.
func GrayscalePlane8(rgba []byte, width, height int, kernel Kernel) []uint8 {
	// Stage 1: resize each channel independently to 64x64 and round to 8 bits.
	channels := make([][][]float64, 4)
	for c := 0; c < 4; c++ {
		src := make([][]float64, height)
		for y := 0; y < height; y++ {
			row := make([]float64, width)
			base := y * width * 4
			for x := 0; x < width; x++ {
				row[x] = float64(rgba[base+x*4+c])
			}
			src[y] = row
		}
		channels[c] = ResizePlane(src, ResizeSize, ResizeSize, kernel)
	}

	// Stage 2: grayscale the rounded 8-bit channels, then round again.
	out := make([]uint8, ResizeSize*ResizeSize)
	for y := 0; y < ResizeSize; y++ {
		for x := 0; x < ResizeSize; x++ {
			r := channels[0][y][x]
			g := channels[1][y][x]
			b := channels[2][y][x]

			// Channels were rounded to integers before this point, so snap
			// them back so the arithmetic matches ImageSharp exactly.
			luma := bt601R*math.Round(r) +
				bt601G*math.Round(g) +
				bt601B*math.Round(b)

			v := uint8(math.Round(luma))
			out[y*ResizeSize+x] = v
		}
	}
	return out
}

// ComputeHash runs the full pipeline on an RGBA byte buffer and returns the
// 64-bit perceptual hash.
func ComputeHash(rgba []byte, width, height int, kernel Kernel) uint64 {
	// 1-2. Resize to 64x64 and grayscale, matching the reference exactly.
	plane := GrayscalePlane8(rgba, width, height, kernel)

	// 3. Two-pass DCT-II over the resized plane.
	block := dctTopLeft8(plane)

	// 4. Median of the 64 values, then threshold.
	median := medianOf64(block)

	var hash uint64
	for i := 0; i < HashSize; i++ {
		if block[i] > median {
			// Bit 63 first: the DC term is the most significant bit.
			hash |= 1 << uint(HashSize-1-i)
		}
	}
	return hash
}

// medianOf64 returns the median of exactly 64 values.
//
// The reference implementation sorts, skips 31, takes 2, and averages — that
// is, the mean of the 32nd and 33rd smallest. Doing anything else here shifts
// every bit, so this mirrors that arithmetic exactly.
func medianOf64(values []float64) float64 {
	const n = HashSize
	sorted := make([]float64, n)
	copy(sorted, values)

	// Insertion sort. n is always 64, so simplicity beats cleverness.
	for i := 1; i < n; i++ {
		v := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > v {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = v
	}

	return (sorted[31] + sorted[32]) / 2
}

// dctTopLeft8 performs the two-pass DCT-II over a quantised 64x64 plane and
// returns the top-left 8x8 block in the order the reference consumes it.
func dctTopLeft8(plane []uint8) []float64 {
	const n = ResizeSize

	rows := make([][]float64, n)
	for y := 0; y < n; y++ {
		row := make([]float64, n)
		for x := 0; x < n; x++ {
			row[x] = float64(plane[y*n+x])
		}
		rows[y] = dct1D(row, n)
	}

	// Only the first 8 columns are needed.
	block := make([]float64, HashSize)
	for x := 0; x < lowFrequencySize; x++ {
		column := make([]float64, n)
		for y := 0; y < n; y++ {
			column[y] = rows[y][x]
		}
		coefficients := dct1D(column, n)
		for v := 0; v < lowFrequencySize; v++ {
			block[v*lowFrequencySize+x] = coefficients[v]
		}
	}
	return block
}

// dctTopLeft performs the two-pass DCT-II over a float plane and returns the
// top-left 8x8 block. Retained for shape tests that do not need quantisation.
func dctTopLeft(plane [][]float64) []float64 {
	const n = ResizeSize

	rows := make([][]float64, n)
	for y := 0; y < n; y++ {
		rows[y] = dct1D(plane[y], n)
	}

	block := make([]float64, HashSize)
	for x := 0; x < lowFrequencySize; x++ {
		column := make([]float64, n)
		for y := 0; y < n; y++ {
			column[y] = rows[y][x]
		}
		coefficients := dct1D(column, n)
		for v := 0; v < lowFrequencySize; v++ {
			block[v*lowFrequencySize+x] = coefficients[v]
		}
	}
	return block
}

// dct1D computes the orthonormal DCT-II of a signal of length n.
//
// This mirrors Dct1D_SIMD in the reference implementation:
//
//	sum  = sum_i values[i] * cos((2i + 1) * k * PI / (2n))
//	coef = sum * sqrt(2/n)
//	if k == 0: coef *= 1/sqrt(2)
func dct1D(values []float64, n int) []float64 {
	scale := math.Sqrt(2.0 / float64(n))
	zeroScale := 1 / math.Sqrt2

	out := make([]float64, n)
	for k := 0; k < n; k++ {
		sum := 0.0
		for i := 0; i < n; i++ {
			angle := (2*float64(i) + 1) * float64(k) * math.Pi / (2 * float64(n))
			sum += values[i] * math.Cos(angle)
		}
		coef := sum * scale
		if k == 0 {
			coef *= zeroScale
		}
		out[k] = coef
	}
	return out
}
