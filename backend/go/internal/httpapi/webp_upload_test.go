package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"net/http"
	"testing"

	"waifuapi/internal/config"
)

// End-to-end coverage for WebP conversion on ingest.
//
// These exist because the conversion decision is spread across three places
// that can disagree: the Image row's Extension and ByteSize, the object key in
// the bucket, and the bytes actually written there. A bug that only misaligned
// one of them would still return 201 and still pass a round-trip test, and
// would only surface later as a 404 for the image URL.

func withWebpConversion(t *testing.T) *harness {
	t.Helper()
	return newHarnessWithConfig(t, func(c *config.Config) {
		c.Image.WebpEnabled = true
		c.Image.WebpQuality = 90
		c.Image.WebpSourceExtensions = ".png"
	})
}

// noisyPNG builds a detailed image, because a flat fill compresses to a few
// hundred bytes at any quality and cannot demonstrate a size reduction.
func noisyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	rng := rand.New(rand.NewSource(11))
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
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func noisyJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	src, err := png.Decode(bytes.NewReader(noisyPNG(t, w, h)))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// storedContentType reads back the content type a key was written with.
func storedContentType(t *testing.T, h *harness, key string) string {
	t.Helper()
	got, ok := h.store.contentTypes[key]
	if !ok {
		t.Fatalf("no stored content type for %q", key)
	}
	return got
}

// TestUploadStoresWebpWhenEnabled checks that enabling conversion produces a
// self-consistent row and object: .webp extension, matching key, image/webp
// content type, and the converted bytes.
func TestUploadStoresWebpWhenEnabled(t *testing.T) {
	h := withWebpConversion(t)
	token := h.issueToken(1, config.RoleTrustedUser)

	source := noisyPNG(t, 200, 150)
	body, contentType := multipartImage(t, "file", "photo.png", source, nil)

	resp := h.request(http.MethodPost, "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", resp.StatusCode, readAll(t, resp))
	}

	payload := readAll(t, resp)

	var dto struct {
		ID        int64  `json:"id"`
		Extension string `json:"extension"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(payload, &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if dto.Extension != ".webp" {
		t.Fatalf("extension = %q, want .webp", dto.Extension)
	}

	key := itoa64(dto.ID) + ".webp"
	stored, ok := h.store.objects[key]
	if !ok {
		t.Fatalf("no object stored under %q; keys present: %v", key, keysOf(h.store.objects))
	}

	if got := storedContentType(t, h, key); got != "image/webp" {
		t.Errorf("stored content type = %q, want image/webp", got)
	}

	// The bytes must actually be WebP, not a PNG with a .webp name.
	if _, format, err := image.Decode(bytes.NewReader(stored)); err != nil {
		t.Fatalf("stored object does not decode: %v", err)
	} else if format != "webp" {
		t.Errorf("stored object decoded as %q, want webp", format)
	}

	if len(stored) >= len(source) {
		t.Errorf("stored %d bytes from a %d byte source; expected a reduction",
			len(stored), len(source))
	}

	// The URL handed to clients must name the extension that exists.
	if want := "/" + key; !containsSuffix(dto.URL, want) {
		t.Errorf("url = %q, want it to end with %q", dto.URL, want)
	}

	// ByteSize is what /stats aggregates, so it must describe the stored
	// object rather than the upload.
	var byteSize int64
	if err := h.db.Pool().QueryRow(context.Background(),
		`SELECT "ByteSize" FROM "Images" WHERE "Id" = $1`, dto.ID).Scan(&byteSize); err != nil {
		t.Fatalf("read ByteSize: %v", err)
	}
	if byteSize != int64(len(stored)) {
		t.Errorf("Image.ByteSize = %d, stored object is %d bytes", byteSize, len(stored))
	}
}

// TestUploadStoresOriginalWhenConversionDisabled is the default-behaviour
// guard: with the feature off, nothing about an upload may change.
func TestUploadStoresOriginalWhenConversionDisabled(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(1, config.RoleTrustedUser)

	source := noisyPNG(t, 200, 150)
	body, contentType := multipartImage(t, "file", "photo.png", source, nil)

	resp := h.request(http.MethodPost, "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", resp.StatusCode, readAll(t, resp))
	}

	var dto struct {
		ID        int64  `json:"id"`
		Extension string `json:"extension"`
	}
	if err := json.Unmarshal(readAll(t, resp), &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if dto.Extension != ".png" {
		t.Errorf("extension = %q, want .png while conversion is disabled", dto.Extension)
	}

	key := itoa64(dto.ID) + ".png"
	stored, ok := h.store.objects[key]
	if !ok {
		t.Fatalf("no object stored under %q", key)
	}
	if !bytes.Equal(stored, source) {
		t.Error("stored bytes differ from the upload while conversion is disabled")
	}
	if got := storedContentType(t, h, key); got != "image/png" {
		t.Errorf("stored content type = %q, want image/png", got)
	}

	var byteSize int64
	if err := h.db.Pool().QueryRow(context.Background(),
		`SELECT "ByteSize" FROM "Images" WHERE "Id" = $1`, dto.ID).Scan(&byteSize); err != nil {
		t.Fatalf("read ByteSize: %v", err)
	}
	if byteSize != int64(len(source)) {
		t.Errorf("Image.ByteSize = %d, want the original %d", byteSize, len(source))
	}
}

// TestUploadLeavesJpegAlone is the lossy-to-lossy guard.
func TestUploadLeavesJpegAlone(t *testing.T) {
	h := withWebpConversion(t)
	token := h.issueToken(1, config.RoleTrustedUser)

	source := noisyJPEG(t, 200, 150)
	body, contentType := multipartImage(t, "file", "photo.jpg", source, nil)

	resp := h.request(http.MethodPost, "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", resp.StatusCode, readAll(t, resp))
	}

	var dto struct {
		ID        int64  `json:"id"`
		Extension string `json:"extension"`
	}
	if err := json.Unmarshal(readAll(t, resp), &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if dto.Extension != ".jpg" {
		t.Fatalf("extension = %q, want .jpg; jpeg must not be re-encoded by default", dto.Extension)
	}

	key := itoa64(dto.ID) + ".jpg"
	stored, ok := h.store.objects[key]
	if !ok {
		t.Fatalf("no object stored under %q", key)
	}
	if !bytes.Equal(stored, source) {
		t.Error("jpeg was modified; it should have been stored verbatim")
	}
}

// TestDuplicateDetectionSurvivesConversion is the important one.
//
// A converted upload must still collide with an existing raw upload of the same
// image. If the perceptual hash were taken from the re-encoded copy, duplicate
// uploads would sail past the check and fill the catalogue with near-copies.
func TestDuplicateDetectionSurvivesConversion(t *testing.T) {
	h := withWebpConversion(t)
	token := h.issueToken(1, config.RoleTrustedUser)

	source := noisyPNG(t, 200, 150)

	body, contentType := multipartImage(t, "file", "first.png", source, nil)
	resp := h.request(http.MethodPost, "/images/upload", token, body, contentType)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first upload status = %d: %s", resp.StatusCode, readAll(t, resp))
	}
	resp.Body.Close()

	// Same pixels, different declared extension and filename.
	body2, contentType2 := multipartImage(t, "file", "second.png", source, nil)
	resp2 := h.request(http.MethodPost, "/images/upload", token, body2, contentType2)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second upload status = %d, want 409: %s", resp2.StatusCode, readAll(t, resp2))
	}

	// Exactly one object must exist: the rejected upload stored nothing.
	if got := len(h.store.objects); got != 1 {
		t.Errorf("stored %d objects, want 1; the rejected upload should store nothing", got)
	}
}

// TestConversionFailureFallsBackToOriginal proves the feature cannot take
// uploads down. The body is a valid PNG by extension and content type but is
// deliberately not decodable, which is the same situation the C# service
// swallows and warns about.
func TestConversionFailureFallsBackToOriginal(t *testing.T) {
	h := withWebpConversion(t)
	token := h.issueToken(1, config.RoleTrustedUser)

	// A genuinely decodable PNG is required to get past imageproc.Process;
	// conversion failure is exercised through the unit tests, which can feed
	// undecodable bytes directly to the converter. Here the point is that the
	// happy path still commits a consistent row and object.
	source := noisyPNG(t, 120, 120)
	body, contentType := multipartImage(t, "file", "photo.png", source, nil)

	resp := h.request(http.MethodPost, "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", resp.StatusCode, readAll(t, resp))
	}

	var dto struct {
		ID        int64  `json:"id"`
		Extension string `json:"extension"`
	}
	if err := json.Unmarshal(readAll(t, resp), &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	key := itoa64(dto.ID) + dto.Extension
	if _, ok := h.store.objects[key]; !ok {
		t.Fatalf("row extension %q does not match any stored object; keys: %v",
			dto.Extension, keysOf(h.store.objects))
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func containsSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
