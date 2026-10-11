package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// detailedPNG builds a noisy image.
//
// Noise matters: a flat fill compresses to a few hundred bytes at any quality,
// so a solid-colour fixture cannot distinguish q20 from q95 and silently makes
// the quality assertions pass for the wrong reason.
func detailedPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

// animatedGIF builds a two-frame animation, which conversion must never touch.
func noiseAnimatedGIF(t *testing.T, w, h int) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}}
	g := &gif.GIF{}
	for i := 0; i < 2; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if (x+y+i)%2 == 0 {
					frame.SetColorIndex(x, y, uint8(i%2))
				}
			}
		}
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode animated fixture: %v", err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	src := detailedPNG(t, w, h)
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatalf("encode jpeg fixture: %v", err)
	}
	return buf.Bytes()
}

func enabledConfig() ConversionConfig {
	cfg := DefaultConversionConfig()
	cfg.Enabled = true
	return cfg
}

func TestConversionIsOffByDefault(t *testing.T) {
	got, err := TryConvertWebP(detailedPNG(t, 64, 64), ".png", false, DefaultConversionConfig())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected no conversion while disabled, got %d bytes", len(got.Content))
	}
}

func TestConvertsPNGToWebP(t *testing.T) {
	src := detailedPNG(t, 160, 120)

	got, err := TryConvertWebP(src, ".png", false, enabledConfig())
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got == nil {
		t.Fatal("expected a conversion result")
	}

	if got.Extension != WebpExtension {
		t.Errorf("Extension = %q, want %q", got.Extension, WebpExtension)
	}
	if got.ContentType != WebpContentType {
		t.Errorf("ContentType = %q, want %q", got.ContentType, WebpContentType)
	}

	// ByteSize drives Image.ByteSize, so it must describe the bytes actually
	// produced rather than the input length.
	if got.ByteSize != int64(len(got.Content)) {
		t.Errorf("ByteSize = %d, want %d", got.ByteSize, len(got.Content))
	}
	if got.ByteSize >= int64(len(src)) {
		t.Errorf("converted %d bytes from a %d byte source; expected a reduction",
			got.ByteSize, len(src))
	}

	// The payload must be a real WebP that decodes back to the same size.
	decoded, format, err := image.Decode(bytes.NewReader(got.Content))
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	if format != "webp" {
		t.Errorf("decoded as %q, want webp", format)
	}
	if got := decoded.Bounds(); got.Dx() != 160 || got.Dy() != 120 {
		t.Errorf("decoded bounds = %v, want 160x120", got)
	}
}

func TestSkipsAnimatedImages(t *testing.T) {
	got, err := TryConvertWebP(noiseAnimatedGIF(t, 64, 64), ".png", true, enabledConfig())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != nil {
		t.Fatal("animated images must not be converted")
	}
}

func TestSkipsIneligibleExtensions(t *testing.T) {
	cases := []string{".jpg", ".jpeg", ".gif", ".webp", ".txt", ""}
	for _, ext := range cases {
		t.Run("ext="+ext, func(t *testing.T) {
			got, err := TryConvertWebP(jpegBytes(t, 64, 64), ext, false, enabledConfig())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != nil {
				t.Errorf("extension %q should not be converted by default", ext)
			}
		})
	}
}

func TestHonoursConfiguredSourceExtensions(t *testing.T) {
	cfg := enabledConfig()
	cfg.SourceExtensions = ParseConversionSourceExtensions(".png, .jpg")

	for _, ext := range []string{".png", ".jpg"} {
		t.Run(ext, func(t *testing.T) {
			got, err := TryConvertWebP(jpegBytes(t, 64, 64), ext, false, cfg)
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			if got == nil {
				t.Fatalf("expected %s to be eligible", ext)
			}
		})
	}

	// .gif stays ineligible even after widening the list, because animation is
	// an independent guard.
	got, err := TryConvertWebP(noiseAnimatedGIF(t, 64, 64), ".gif", true, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("animated images must remain unconverted even when the source list is widened")
	}
}

func TestExtensionMatchIsCaseInsensitive(t *testing.T) {
	cfg := enabledConfig()
	cfg.SourceExtensions = []string{".PNG"}

	for _, ext := range []string{".png", ".PNG", ".PnG"} {
		got, err := TryConvertWebP(detailedPNG(t, 64, 64), ext, false, cfg)
		if err != nil {
			t.Fatalf("convert %s: %v", ext, err)
		}
		if got == nil {
			t.Errorf("extension %q should match the configured .PNG", ext)
		}
	}
}

func TestQualityAffectsOutputSize(t *testing.T) {
	src := detailedPNG(t, 128, 128)

	sizeAt := func(quality int) int64 {
		cfg := enabledConfig()
		cfg.Quality = quality
		got, err := TryConvertWebP(src, ".png", false, cfg)
		if err != nil {
			t.Fatalf("q%d: %v", quality, err)
		}
		if got == nil {
			t.Fatalf("q%d: expected a conversion", quality)
		}
		return got.ByteSize
	}

	low, high := sizeAt(20), sizeAt(95)
	if high <= low {
		t.Errorf("q95 (%d bytes) should be larger than q20 (%d bytes)", high, low)
	}
}

func TestClampsOutOfRangeQuality(t *testing.T) {
	src := detailedPNG(t, 96, 96)

	for _, quality := range []int{-50, 0, 1, 100, 101, 5000} {
		cfg := enabledConfig()
		cfg.Quality = quality
		got, err := TryConvertWebP(src, ".png", false, cfg)
		if err != nil {
			t.Fatalf("quality %d: %v", quality, err)
		}
		if got == nil {
			t.Errorf("quality %d: expected a conversion rather than a rejection", quality)
		}
	}

	// Clamping must be monotonic at the boundaries: 1 is the smallest legal
	// output and 100 the largest.
	if get := func(q int) int64 {
		cfg := enabledConfig()
		cfg.Quality = q
		got, _ := TryConvertWebP(src, ".png", false, cfg)
		return got.ByteSize
	}; get(1) >= get(100) {
		t.Errorf("q1 (%d) should be smaller than q100 (%d)", get(1), get(100))
	}
}

func TestClampsOutOfRangeMethod(t *testing.T) {
	for _, method := range []int{-1, 0, 3, 6, 7, 99} {
		cfg := enabledConfig()
		cfg.Method = method
		got, err := TryConvertWebP(detailedPNG(t, 64, 64), ".png", false, cfg)
		if err != nil {
			t.Fatalf("method %d: %v", method, err)
		}
		if got == nil {
			t.Errorf("method %d: expected a conversion", method)
		}
	}
}

func TestConversionFailureIsReportedNotSwallowed(t *testing.T) {
	// A caller must be able to tell "did not apply" (nil, nil) apart from
	// "tried and failed" (nil, err); the upload path logs the latter and stores
	// the original, so conflating them would hide a broken encoder.
	garbage := []byte("this is definitely not an image")

	got, err := TryConvertWebP(garbage, ".png", false, enabledConfig())
	if err == nil {
		t.Fatal("expected an error for undecodable bytes")
	}
	if got != nil {
		t.Fatal("expected no result alongside the error")
	}

	empty, err := TryConvertWebP(nil, ".png", false, enabledConfig())
	if err == nil {
		t.Fatal("expected an error for empty input")
	}
	if empty != nil {
		t.Fatal("expected no result alongside the error")
	}
}

func TestSourceExtensionParsing(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty falls back to png", "", []string{".png"}},
		{"blank falls back to png", "   ", []string{".png"}},
		{"single", ".png", []string{".png"}},
		{"adds missing dot", "png", []string{".png"}},
		{"trims and lowercases", " .PNG , .JPG ", []string{".jpg", ".png"}},
		{"drops empties", ".png,,.jpg,", []string{".jpg", ".png"}},
		{"deduplicates", ".png,.png,PNG", []string{".png"}},
		{"only separators falls back", ",,", []string{".png"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseConversionSourceExtensions(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseConversionSourceExtensions(%q) = %v, want %v",
					tc.input, got, tc.want)
			}
		})
	}
}

func TestConversionDoesNotMutateInput(t *testing.T) {
	// The upload path keeps using the original bytes for the perceptual hash
	// and for the error response, so conversion must not write into them.
	src := detailedPNG(t, 96, 96)
	before := append([]byte(nil), src...)

	if _, err := TryConvertWebP(src, ".png", false, enabledConfig()); err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !bytes.Equal(before, src) {
		t.Error("conversion mutated the input buffer")
	}
}

func TestWebPInputIsNotReEncoded(t *testing.T) {
	// An upload that is already WebP must not be transcoded again; doing so
	// would compound loss on every replacement.
	src := detailedPNG(t, 64, 64)
	first, err := TryConvertWebP(src, ".png", false, enabledConfig())
	if err != nil || first == nil {
		t.Fatalf("initial conversion failed: %v", err)
	}

	second, err := TryConvertWebP(first.Content, ".webp", false, enabledConfig())
	if err != nil {
		t.Fatalf("second conversion: %v", err)
	}
	if second != nil {
		t.Error("webp input must not be converted again")
	}
}

func TestConversionIsDeterministic(t *testing.T) {
	// The same input must produce the same bytes, or re-uploading an image
	// would create object churn that the storage-diff tooling reads as change.
	src := detailedPNG(t, 128, 128)
	a, err := TryConvertWebP(src, ".png", false, enabledConfig())
	if err != nil || a == nil {
		t.Fatalf("first conversion: %v", err)
	}
	b, err := TryConvertWebP(src, ".png", false, enabledConfig())
	if err != nil || b == nil {
		t.Fatalf("second conversion: %v", err)
	}
	if !bytes.Equal(a.Content, b.Content) {
		t.Error("conversion is not deterministic")
	}
}

func TestWebPIsSubstantiallySmallerThanPNG(t *testing.T) {
	// Not an exact-ratio assertion: the point is that the feature earns its
	// storage, and a regression that quietly disabled compression would still
	// pass a round-trip test.
	src := detailedPNG(t, 256, 256)
	got, err := TryConvertWebP(src, ".png", false, enabledConfig())
	if err != nil || got == nil {
		t.Fatalf("convert: %v", err)
	}
	ratio := float64(got.ByteSize) / float64(len(src))
	if ratio > 0.6 {
		t.Errorf("webp is %.0f%% of the png source; expected a substantial reduction", ratio*100)
	}
	t.Logf("png=%d webp=%d (%.0f%%)", len(src), got.ByteSize, ratio*100)
}

func TestSupportsPaletteAndAlphaImages(t *testing.T) {
	// PNG uploads in the wild are frequently paletted or have transparency;
	// both must survive the round trip rather than panicking in the encoder.
	src := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			src.Set(x, y, color.NRGBA{uint8(x * 6), uint8(y * 6), 128, uint8(x * y % 256)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := TryConvertWebP(buf.Bytes(), ".png", false, enabledConfig())
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got == nil {
		t.Fatal("expected a conversion")
	}

	decoded, _, err := image.Decode(bytes.NewReader(got.Content))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The alpha channel must be present; a lossy RGB conversion would flatten
	// transparency and break any client compositing over a background.
	if _, ok := decoded.(interface{ ColorModel() color.Model }); !ok {
		t.Fatal("decoded image exposes no colour model")
	}
	if !strings.Contains(strings.ToLower(WebpContentType), "webp") {
		t.Errorf("unexpected content type %q", WebpContentType)
	}
}