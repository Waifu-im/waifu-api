package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

// testLimits mirrors the .env.example dimensions.
func testLimits() Limits {
	return Limits{
		MinWidth: 800, MinHeight: 1000, MaxWidth: 9000, MaxHeight: 9000,
		AnimatedMinWidth: 400, AnimatedMinHeight: 200,
		AnimatedMaxWidth: 2000, AnimatedMaxHeight: 2000,
	}
}

// solidImage builds a uniformly coloured PNG of the given size.
func solidImage(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// animatedGIF builds a multi-frame GIF.
//
// gif.EncodeAll requires paletted frames, so the RGB image is quantised to a
// grey ramp palette before being added.
func animatedGIF(t *testing.T, width, height, frames int, colour func(frame, x, y int) color.RGBA) []byte {
	t.Helper()

	palette := make(color.Palette, 256)
	for i := range palette {
		palette[i] = color.RGBA{uint8(i), uint8(i), uint8(i), 255}
	}

	g := &gif.GIF{}
	for frame := 0; frame < frames; frame++ {
		paletted := image.NewPaletted(image.Rect(0, 0, width, height), palette)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				c := colour(frame, x, y)
				paletted.SetColorIndex(x, y, uint8(palette.Index(c)))
			}
		}
		g.Image = append(g.Image, paletted)
		g.Delay = append(g.Delay, 10)
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return buf.Bytes()
}

func TestProcessExtractsMetadata(t *testing.T) {
	data := solidImage(t, 1000, 1200, color.RGBA{200, 100, 50, 255})

	meta, err := Process(data, "photo.PNG", "image/png", testLimits())
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if meta.Width != 1000 || meta.Height != 1200 {
		t.Errorf("dimensions = %dx%d, want 1000x1200", meta.Width, meta.Height)
	}
	if meta.ByteSize != int64(len(data)) {
		t.Errorf("byte size = %d, want %d", meta.ByteSize, len(data))
	}
	if meta.Extension != ".png" {
		t.Errorf("extension = %q, want .png (lower-cased)", meta.Extension)
	}
	if meta.IsAnimated {
		t.Error("a single-frame PNG must not be reported as animated")
	}

	// A solid image must report exactly its own colour.
	if meta.DominantColor != "#C86432" {
		t.Errorf("dominant colour = %q, want #C86432", meta.DominantColor)
	}

	// The hash is persisted as 16 upper-case hex digits.
	if len(meta.PerceptualHash) != 16 {
		t.Errorf("hash %q has length %d, want 16", meta.PerceptualHash, len(meta.PerceptualHash))
	}
	if strings.ToUpper(meta.PerceptualHash) != meta.PerceptualHash {
		t.Errorf("hash %q is not upper case", meta.PerceptualHash)
	}
}

func TestProcessIsDeterministic(t *testing.T) {
	data := solidImage(t, 900, 1000, color.RGBA{10, 200, 90, 255})

	first, err := Process(data, "a.png", "image/png", testLimits())
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := Process(data, "a.png", "image/png", testLimits())
		if err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
		if again.PerceptualHash != first.PerceptualHash {
			t.Fatalf("hash is not deterministic: %s vs %s", again.PerceptualHash, first.PerceptualHash)
		}
	}
}

func TestJPEGMetadata(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 850, 1100))
	for y := 0; y < 1100; y++ {
		for x := 0; x < 850; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 40, 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	meta, err := Process(buf.Bytes(), "photo.jpg", "image/jpeg", testLimits())
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if meta.Width != 850 || meta.Height != 1100 {
		t.Errorf("dimensions = %dx%d, want 850x1100", meta.Width, meta.Height)
	}
	if meta.Extension != ".jpg" {
		t.Errorf("extension = %q, want .jpg", meta.Extension)
	}
}

func TestAnimatedGIFDetected(t *testing.T) {
	// An animated GIF is allowed smaller dimensions than a static image, which
	// is why frame detection changes which limit set applies.
	data := animatedGIF(t, 500, 300, 3, func(frame, x, y int) color.RGBA {
		return color.RGBA{uint8(x), uint8(y), uint8(frame * 40), 255}
	})

	meta, err := Process(data, "loop.gif", "image/gif", testLimits())
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !meta.IsAnimated {
		t.Error("a multi-frame GIF must be reported as animated")
	}
	if meta.Width != 500 || meta.Height != 300 {
		t.Errorf("dimensions = %dx%d, want 500x300", meta.Width, meta.Height)
	}
}

func TestTooSmallIsRejected(t *testing.T) {
	// 100x100 is below the static minimum of 800x1000.
	data := solidImage(t, 100, 100, color.RGBA{1, 2, 3, 255})

	_, err := Process(data, "tiny.png", "image/png", testLimits())
	if err == nil {
		t.Fatal("an undersized image must be rejected")
	}

	tooLarge, ok := err.(*ErrTooLarge)
	if !ok {
		t.Fatalf("error type = %T, want *ErrTooLarge", err)
	}
	if !strings.Contains(tooLarge.Error(), "too low") {
		t.Errorf("message %q should say the resolution is too low", tooLarge.Error())
	}
}

func TestTooLargeIsRejected(t *testing.T) {
	data := solidImage(t, 10000, 10000, color.RGBA{1, 2, 3, 255})

	_, err := Process(data, "huge.png", "image/png", testLimits())
	if err == nil {
		t.Fatal("an oversized image must be rejected")
	}
	if !strings.Contains(err.Error(), "too high") {
		t.Errorf("message %q should say the resolution is too high", err.Error())
	}
}

func TestAnimatedUsesAnimatedLimits(t *testing.T) {
	// 500x300 passes the animated minimum (400x200) but would fail the static
	// minimum (800x1000). A multi-frame GIF must therefore be accepted.
	data := animatedGIF(t, 500, 300, 2, func(frame, x, y int) color.RGBA {
		return color.RGBA{uint8(x % 256), uint8(y % 256), 255, 255}
	})

	if _, err := Process(data, "loop.gif", "image/gif", testLimits()); err != nil {
		t.Fatalf("animated image within animated limits was rejected: %v", err)
	}
}

func TestUnsupportedContentTypeRejected(t *testing.T) {
	data := solidImage(t, 1000, 1000, color.RGBA{1, 2, 3, 255})

	for _, contentType := range []string{"image/tiff", "image/bmp", "application/pdf", "text/plain"} {
		_, err := Process(data, "x", contentType, testLimits())
		if err == nil {
			t.Errorf("content type %q should be rejected", contentType)
		}
	}
}

func TestAllowedContentTypes(t *testing.T) {
	for _, contentType := range []string{"image/jpeg", "image/png", "image/gif", "image/webp", "IMAGE/PNG"} {
		if !IsAllowedContentType(contentType) {
			t.Errorf("%q should be allowed", contentType)
		}
	}
}

func TestCorruptDataRejected(t *testing.T) {
	_, err := Process([]byte("this is not an image"), "x.png", "image/png", testLimits())
	if err == nil {
		t.Fatal("corrupt data must be rejected")
	}
}

func TestEmptyDataRejected(t *testing.T) {
	if _, err := Process(nil, "x.png", "image/png", testLimits()); err == nil {
		t.Fatal("empty data must be rejected")
	}
}

func TestDominantColourRounding(t *testing.T) {
	// A 50/50 mix of black and white averages to 127.5, which must round to 128
	// (away from zero), not 127 (banker's rounding).
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{0, 0, 0, 255})
	img.Set(1, 0, color.RGBA{255, 255, 255, 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	// This test is about rounding, not size, so the dimension limits are
	// relaxed to admit the 2x1 fixture.
	relaxed := testLimits()
	relaxed.MinWidth, relaxed.MinHeight = 1, 1

	meta, err := Process(buf.Bytes(), "mix.png", "image/png", relaxed)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if meta.DominantColor != "#808080" {
		t.Errorf("dominant colour = %q, want #808080", meta.DominantColor)
	}
}
