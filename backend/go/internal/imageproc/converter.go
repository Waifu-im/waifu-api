package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"sort"
	"strings"

	"github.com/gen2brain/webp"
)

// WebP conversion at ingest.
//
// This is the Go equivalent of ImageConverter.cs. The two must agree exactly,
// because both write Image.Extension and Image.ByteSize and both decide which
// object key lands in the bucket: if they disagree, the row and the stored
// object describe different files.
//
// # Why it is off by default
//
// Conversion rewrites pixels that have already been validated and measured.
// Turning it on changes what is stored for every future upload, so it is opt-in
// and reversible rather than baked in.
//
// # Why PNG only
//
// JPEG is already lossy-compressed. Re-encoding it is lossy-to-lossy and risks
// generation loss that users notice at high zoom, for a far smaller gain than
// the same operation on PNG. The source list is configurable for anyone who
// wants to make that trade deliberately.

// WebpExtension is the extension recorded on converted images.
const WebpExtension = ".webp"

// WebpContentType is the content type recorded on converted images.
const WebpContentType = "image/webp"

// DefaultWebpQuality matches the offline migration, which used cwebp -q 90.
const DefaultWebpQuality = 90

// DefaultWebpSourceExtensions is the comma-separated default source list.
const DefaultWebpSourceExtensions = ".png"

// DefaultWebpMethod mirrors libwebp's default effort and cwebp's default -m.
// libwebp ranges 0 (fastest) to 6 (slowest, smallest); 4 is the balance the
// migration corpus was measured at.
const DefaultWebpMethod = 4

// ConversionConfig carries the WebP settings.
//
// It is separate from Limits because the C# service reads these through a
// different configuration path, and conflating them would make the off-by-
// default behaviour depend on upload validation settings.
type ConversionConfig struct {
	// Enabled gates everything. False means uploads are stored verbatim.
	Enabled bool

	// Quality is 1-100. Values outside the range are clamped.
	Quality int

	// SourceExtensions lists eligible extensions, lower-cased, including the
	// dot.
	SourceExtensions []string

	// Method is the libwebp effort level, 0-6.
	Method int
}

// ConversionResult describes a successful conversion.
type ConversionResult struct {
	// Content is the encoded WebP. It is a separate buffer from the input, so
	// the caller can keep using the original for metadata.
	Content []byte

	Extension   string
	ContentType string
	ByteSize    int64
}

// DefaultConversionConfig returns the settings used when nothing is
// configured: conversion off.
func DefaultConversionConfig() ConversionConfig {
	return ConversionConfig{
		Enabled:          false,
		Quality:          DefaultWebpQuality,
		SourceExtensions: []string{".png"},
		Method:           DefaultWebpMethod,
	}
}

// ParseConversionSourceExtensions splits the comma-separated configuration
// value into normalised extensions.
//
// An empty or absent value yields the default rather than an empty list,
// because an empty list would silently disable conversion for everyone who set
// the key to a blank string.
func ParseConversionSourceExtensions(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = DefaultWebpSourceExtensions
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		ext := strings.ToLower(strings.TrimSpace(p))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if _, dup := seen[ext]; dup {
			continue
		}
		seen[ext] = struct{}{}
		out = append(out, ext)
	}

	if len(out) == 0 {
		out = []string{DefaultWebpSourceExtensions}
	}

	// Sorted so behaviour is deterministic regardless of configuration order,
	// which keeps tests and logs stable.
	sort.Strings(out)
	return out
}

// ErrNoConverter is returned by callers that need conversion to have happened
// but cannot proceed without it. Nothing in the upload path uses it; it exists
// so a future bulk tool can demand conversion rather than silently storing
// the original.
var ErrNoConverter = errors.New("webp conversion was not performed")

// TryConvertWebP re-encodes data as WebP when it is eligible.
//
// It returns (nil, nil) when conversion does not apply: the feature is off, the
// source format is not eligible, or the image is animated. A nil result with a
// nil error is not a failure, and the caller must store the original bytes.
//
// A non-nil error means conversion was attempted and failed. The caller is
// expected to log it and store the original bytes anyway, mirroring the C#
// service, which catches the exception, warns, and returns null. Failing the
// upload over a re-encode would turn an optimisation into an outage.
func TryConvertWebP(data []byte, extension string, isAnimated bool, cfg ConversionConfig) (*ConversionResult, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Animated images are skipped deliberately. Animated WebP encoding is a
	// larger compatibility question than static encoding, and re-compressing a
	// GIF palette into a lossy format is the exact regression this avoids.
	if isAnimated {
		return nil, nil
	}

	if !containsExtension(cfg.SourceExtensions, extension) {
		return nil, nil
	}

	if len(data) == 0 {
		return nil, errors.New("image data is empty")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("could not decode image for conversion: %w", err)
	}

	var buf bytes.Buffer
	err = webp.Encode(&buf, img, webp.Options{
		Quality:  clampQuality(cfg.Quality),
		Lossless: false,
		Method:   clampMethod(cfg.Method),
	})
	if err != nil {
		return nil, fmt.Errorf("could not encode webp: %w", err)
	}

	return &ConversionResult{
		Content:     buf.Bytes(),
		Extension:   WebpExtension,
		ContentType: WebpContentType,
		ByteSize:    int64(buf.Len()),
	}, nil
}

// containsExtension reports whether ext appears in the eligible list,
// comparing case-insensitively.
func containsExtension(list []string, ext string) bool {
	ext = strings.ToLower(ext)
	for _, candidate := range list {
		if strings.ToLower(candidate) == ext {
			return true
		}
	}
	return false
}

// clampQuality keeps the quality inside the range libwebp accepts.
//
// Out-of-range values are clamped rather than rejected: a typo in configuration
// should not stop the service starting, and the caller logs the effective
// value either way.
func clampQuality(q int) int {
	switch {
	case q < 1:
		return 1
	case q > 100:
		return 100
	default:
		return q
	}
}

// clampMethod keeps the effort level inside 0-6, substituting libwebp's
// default when unset.
func clampMethod(m int) int {
	if m < 0 || m > 6 {
		return DefaultWebpMethod
	}
	return m
}
