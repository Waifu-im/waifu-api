// Package imageproc extracts the metadata the Images table stores for every
// upload.
//
// This is the Go equivalent of ImageProcessingService.cs. It deliberately
// produces the same fields, in the same formats, because those values are
// already persisted for roughly 4,000 existing rows and are part of the public
// API response shape.
//
// # What is measured
//
//   - width and height, used for both response payloads and dimension limits
//   - whether the image is animated, which selects a different size limit
//   - a dominant colour, used for placeholder backgrounds in the frontend
//   - a perceptual hash, used for duplicate detection
//
// # Perceptual hash
//
// The hash is produced by internal/images/phash. See that package: the C#
// reference is not reproducible against itself, so this implementation does not
// attempt to match a particular machine's output. It is deterministic and
// internally consistent, which is what a corpus backfill depends on.
package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"

	"waifuapi/internal/images/phash"
)

// AllowedContentTypes lists the formats accepted for upload.
//
// Mirrors UploadImage/Validator.cs, which allowlists exactly these four. It is
// an allowlist rather than a blocklist so an unexpected format is rejected
// rather than half-handled.
var AllowedContentTypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
}

// Metadata is the result of processing one uploaded image.
type Metadata struct {
	// PerceptualHash is the 64-bit duplicate-detection hash, as a
	// zero-padded uppercase hex string. The column stores text, matching
	// ImageProcessingService which calls hash.ToString("X16").
	PerceptualHash string

	// DominantColor is formatted as "#RRGGBB", matching the C# service.
	DominantColor string

	Width  int
	Height int

	// ByteSize is the length of the original upload, not the decoded size.
	ByteSize int64

	// Extension is the lower-cased file extension including the dot.
	Extension string

	// IsAnimated is true when the source contains more than one frame.
	IsAnimated bool
}

// Limits carries the configured dimension bounds.
type Limits struct {
	MinWidth          int
	MinHeight         int
	MaxWidth          int
	MaxHeight         int
	AnimatedMinWidth  int
	AnimatedMinHeight int
	AnimatedMaxWidth  int
	AnimatedMaxHeight int
}

// ErrUnsupportedFormat reports a content type that is not accepted.
var ErrUnsupportedFormat = errors.New("unsupported image format")

// ErrTooLarge reports an image outside the configured dimension bounds.
type ErrTooLarge struct {
	Width    int
	Height   int
	Min      [2]int
	Max      [2]int
	Animated bool
}

// Error renders the failure the same way the C# service does, so existing
// clients see familiar wording.
func (e *ErrTooLarge) Error() string {
	kind := ""
	if e.Animated {
		kind = " for animated images"
	}
	if e.Width < e.Min[0] || e.Height < e.Min[1] {
		return fmt.Sprintf("Image resolution is too low (%dx%d). Minimum is %dx%d%s.",
			e.Width, e.Height, e.Min[0], e.Min[1], kind)
	}
	return fmt.Sprintf("Image resolution is too high (%dx%d). Maximum is %dx%d%s.",
		e.Width, e.Height, e.Max[0], e.Max[1], kind)
}

// IsAllowedContentType reports whether the MIME type may be uploaded.
func IsAllowedContentType(contentType string) bool {
	for _, allowed := range AllowedContentTypes {
		if strings.EqualFold(contentType, allowed) {
			return true
		}
	}
	return false
}

// Process decodes an upload and measures it, enforcing the dimension limits.
//
// contentType is the declared MIME type and is checked against the allowlist.
// It is checked rather than sniffed from the bytes because the C# service does
// the same, and because accepting a declared type and decoding it separately
// would allow a mismatch between what was validated and what was stored.
func Process(data []byte, fileName, contentType string, limits Limits) (Metadata, error) {
	if !IsAllowedContentType(contentType) {
		return Metadata{}, fmt.Errorf("%w: %q; allowed: %s",
			ErrUnsupportedFormat, contentType, strings.Join(AllowedContentTypes, ", "))
	}
	if len(data) == 0 {
		return Metadata{}, errors.New("image data is empty")
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Metadata{}, fmt.Errorf("could not decode image: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return Metadata{}, errors.New("image has zero dimensions")
	}

	isAnimated, err := detectAnimation(data, format, img)
	if err != nil {
		return Metadata{}, err
	}

	if err := checkDimensions(width, height, isAnimated, limits); err != nil {
		return Metadata{}, err
	}

	// Copy into a tightly packed, top-down RGBA buffer. Every downstream
	// consumer assumes this layout, and normalising it here means the rest of
	// the package never has to reason about strides or pixel origin.
	rgba, err := toPackedRGBA(img, width, height, bounds)
	if err != nil {
		return Metadata{}, err
	}

	hash := phash.ComputeHash(rgba, width, height, phash.DefaultKernel)
	dominant := dominantColour(rgba, width, height)

	return Metadata{
		PerceptualHash: fmt.Sprintf("%016X", hash),
		DominantColor:  dominant,
		Width:          width,
		Height:         height,
		ByteSize:       int64(len(data)),
		Extension:      strings.ToLower(filepath.Ext(fileName)),
		IsAnimated:     isAnimated,
	}, nil
}

// Hash computes the perceptual hash of image bytes, applying no upload limits.
//
// It exists for the corpus backfill, which must fingerprint images that are
// already stored — including ones that no longer satisfy the current size
// limits, and would therefore be rejected by Process. Calling Process with
// permissive limits instead would silently skip those rows and leave them
// holding a fingerprint from an algorithm that no longer exists.
//
// This deliberately repeats the decode-and-hash sequence from Process rather
// than refactoring it out, so the upload path keeps its exact current shape.
// The two agree because both call toPackedRGBA and then
// phash.ComputeHash with phash.DefaultKernel. If Process is ever changed to
// hash differently, this must change with it, or the backfill fills the column
// with values the running service would never produce.
func Hash(data []byte) (uint64, error) {
	if len(data) == 0 {
		return 0, errors.New("image data is empty")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("could not decode image: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return 0, errors.New("image has zero dimensions")
	}

	rgba, err := toPackedRGBA(img, width, height, bounds)
	if err != nil {
		return 0, err
	}

	return phash.ComputeHash(rgba, width, height, phash.DefaultKernel), nil
}

// detectAnimation reports whether the source has more than one frame.
//
// The C# service reads image.Frames.Count from ImageSharp. The standard
// library exposes frame counts only for GIF, so other formats are reported as
// static. That is a small behaviour difference confined to uploads that claim
// to be animated in a non-GIF container, which the allowlist largely prevents.
func detectAnimation(data []byte, format string, img image.Image) (bool, error) {
	if format != "gif" {
		return false, nil
	}
	reader, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("could not read gif frames: %w", err)
	}
	return len(reader.Image) > 1, nil
}

// checkDimensions enforces the size limits, applying the animated bounds when
// the image has multiple frames.
//
// The animated limits are used with no fallback to the static ones, matching
// ImageProcessingService.cs which selects one set or the other rather than
// combining them.
func checkDimensions(width, height int, animated bool, limits Limits) error {
	minW, minH := limits.MinWidth, limits.MinHeight
	maxW, maxH := limits.MaxWidth, limits.MaxHeight

	if animated {
		minW, minH = limits.AnimatedMinWidth, limits.AnimatedMinHeight
		maxW, maxH = limits.AnimatedMaxWidth, limits.AnimatedMaxHeight
	}

	if width < minW || height < minH {
		return &ErrTooLarge{
			Width: width, Height: height,
			Min:      [2]int{minW, minH},
			Max:      [2]int{maxW, maxH},
			Animated: animated,
		}
	}
	if width > maxW || height > maxH {
		return &ErrTooLarge{
			Width: width, Height: height,
			Min:      [2]int{minW, minH},
			Max:      [2]int{maxW, maxH},
			Animated: animated,
		}
	}
	return nil
}

// toPackedRGBA normalises an image into a contiguous top-down RGBA buffer.
func toPackedRGBA(img image.Image, width, height int, bounds image.Rectangle) ([]byte, error) {
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			out.Set(x, y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return out.Pix, nil
}

// dominantColour approximates an image's dominant colour by averaging the whole
// frame.
//
// ImageSharp does the same thing by resizing to a single 1x1 pixel, which is
// an area-average. Averaging directly is equivalent and avoids a resampling
// pass. The result is formatted "#RRGGBB" with upper-case hex, matching
// GetDominantColor in the C# service.
func dominantColour(rgba []byte, width, height int) string {
	if width == 0 || height == 0 {
		return "#000000"
	}

	var sumR, sumG, sumB float64
	pixelCount := float64(width * height)

	for i := 0; i < width*height; i++ {
		base := i * 4
		sumR += float64(rgba[base+0])
		sumG += float64(rgba[base+1])
		sumB += float64(rgba[base+2])
	}

	r := uint8(roundHalfAway(sumR / pixelCount))
	g := uint8(roundHalfAway(sumG / pixelCount))
	b := uint8(roundHalfAway(sumB / pixelCount))

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// roundHalfAway rounds to the nearest integer, with halves going away from
// zero rather than toward even.
//
// Go's math.Round already does this. It is named here so the intent is
// explicit at the call site, since rounding mode changes the dominant colour
// of many images by one unit.
func roundHalfAway(v float64) float64 {
	if v < 0 {
		return -float64(int(-v + 0.5))
	}
	return float64(int(v + 0.5))
}
