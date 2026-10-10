// Command corpus generates a diverse image corpus for the perceptual-hash spike.
//
// The goal is not "pretty pictures" — it is images that expose differences
// between resampling filters. Two filters agree on smooth gradients and
// disagree violently on high-frequency detail, so the corpus is weighted
// heavily toward sharp edges, fine patterns and hard transitions.
//
// Everything is deterministic (fixed seeds) so the corpus can be regenerated
// byte-for-byte on any machine and the golden vectors stay valid.

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: corpus <output-dir>")
		os.Exit(1)
	}
	out := os.Args[1]
	must(os.MkdirAll(out, 0o755))

	// Sizes chosen to span the interesting resize ratios, including cases where
	// the source is already near 32x32 and cases where it is far above it.
	sizes := []int{8, 17, 32, 33, 64, 128, 256, 800, 1920, 3000}

	patterns := []struct {
		name string
		fn   func(x, y int, w, h int) color.RGBA
	}{
		{"solid-red", solid(color.RGBA{220, 30, 30, 255})},
		{"solid-grey", solid(color.RGBA{128, 128, 128, 255})},
		{"checker-1px", checker(1)},
		{"checker-2px", checker(2)},
		{"checker-7px", checker(7)},
		{"checker-31px", checker(31)},
		{"hstripes-1px", stripes(1, true)},
		{"hstripes-3px", stripes(3, true)},
		{"vstripes-1px", stripes(1, false)},
		{"checker-noise", noise(1)},
		{"noise-fine", noise(2)},
		{"grad-linear", gradLinear},
		{"grad-radial", gradRadial},
		{"rings", rings},
		{"diagonal", diagonal},
		{"blocks", blocks},
		{"circle-sharp", circle},
	}

	for _, p := range patterns {
		for _, s := range sizes {
			img := render(p.fn, s, s)

			// PNG is lossless; JPEG introduces its own artefacts. Both matter,
			// because the production corpus is overwhelmingly JPEG.
			writePNG(filepath.Join(out, fmt.Sprintf("%s_%dx%d.png", p.name, s, s)), img)
			writeJPEG(filepath.Join(out, fmt.Sprintf("%s_%dx%d.jpg", p.name, s, s)), img, 90)
			writeJPEG(filepath.Join(out, fmt.Sprintf("%s_%dx%d_q60.jpg", p.name, s, s)), img, 60)
		}
	}

	// Non-square images: aspect ratio changes how the resize maps source pixels
	// onto the 32x32 grid, which is a distinct code path.
	rects := [][2]int{{1024, 100}, {100, 1024}, {1600, 400}, {400, 1600}, {37, 911}}
	for _, r := range rects {
		for _, p := range patterns[:8] {
			img := render(p.fn, r[0], r[1])
			writePNG(filepath.Join(out, fmt.Sprintf("rect_%dx%d_%s.png", r[0], r[1], p.name)), img)
		}
	}

	// Animated GIF: frame count changes how the C# service reads the image.
	for _, frames := range []int{2, 5} {
		writeGIF(filepath.Join(out, fmt.Sprintf("anim_%dframes.gif", frames)), frames)
	}

	// Grayscale and odd alpha handling.
	writePNG(filepath.Join(out, "grey-ramp.png"), greyRamp(256, 256))
	writePNG(filepath.Join(out, "alpha-gradient.png"), alphaGradient(256, 256))

	count := 0
	_ = filepath.Walk(out, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	fmt.Printf("corpus: wrote %d files to %s\n", count, out)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type patternFunc func(x, y, w, h int) color.RGBA

func render(fn patternFunc, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, fn(x, y, w, h))
		}
	}
	return img
}

func solid(c color.RGBA) patternFunc {
	return func(int, int, int, int) color.RGBA { return c }
}

func checker(px int) patternFunc {
	return func(x, y, w, h int) color.RGBA {
		if ((x/px)+(y/px))%2 == 0 {
			return color.RGBA{255, 255, 255, 255}
		}
		return color.RGBA{0, 0, 0, 255}
	}
}

func stripes(px int, horizontal bool) patternFunc {
	return func(x, y, w, h int) color.RGBA {
		on := false
		if horizontal {
			on = (y/px)%2 == 0
		} else {
			on = (x/px)%2 == 0
		}
		if on {
			return color.RGBA{250, 250, 250, 255}
		}
		return color.RGBA{10, 10, 10, 255}
	}
}

// noise uses a fixed seed per size so output is reproducible but still
// broadband across all frequencies.
func noise(scale int) patternFunc {
	return func(x, y, w, h int) color.RGBA {
		r := rand.New(rand.NewSource(int64(x/scale)*7919 + int64(y/scale)*104729 + int64(w)))
		v := uint8(r.Intn(256))
		return color.RGBA{v, v, v, 255}
	}
}

func gradLinear(x, y, w, h int) color.RGBA {
	t := float64(x) / float64(max(w-1, 1))
	return color.RGBA{uint8(t * 255), uint8(t * 255), uint8(t * 255), 255}
}

func gradRadial(x, y, w, h int) color.RGBA {
	cx, cy := float64(w)/2, float64(h)/2
	d := math.Hypot(float64(x)-cx, float64(y)-cy)
	t := d / math.Hypot(cx, cy)
	v := uint8(math.Min(255, t*255))
	return color.RGBA{v, v, v, 255}
}

func rings(x, y, w, h int) color.RGBA {
	cx, cy := float64(w)/2, float64(h)/2
	d := math.Hypot(float64(x)-cx, float64(y)-cy)
	if math.Mod(d, 9) < 4.5 {
		return color.RGBA{255, 255, 255, 255}
	}
	return color.RGBA{0, 0, 0, 255}
}

func diagonal(x, y, w, h int) color.RGBA {
	if math.Mod(float64(x+y), 11) < 5.5 {
		return color.RGBA{255, 255, 255, 255}
	}
	return color.RGBA{0, 0, 0, 255}
}

func blocks(x, y, w, h int) color.RGBA {
	bx, by := x/13, y/17
	if (bx*31+by*17)%3 == 0 {
		return color.RGBA{20, 60, 200, 255}
	}
	if (bx*17+by*31)%3 == 0 {
		return color.RGBA{200, 40, 20, 255}
	}
	return color.RGBA{240, 240, 200, 255}
}

func circle(x, y, w, h int) color.RGBA {
	cx, cy := float64(w)/2, float64(h)/2
	if math.Hypot(float64(x)-cx, float64(y)-cy) < math.Min(cx, cy)*0.8 {
		return color.RGBA{255, 0, 0, 255}
	}
	return color.RGBA{0, 0, 255, 255}
}

func greyRamp(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / (w - 1))
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func alphaGradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(x * 255 / (w - 1))
			img.Set(x, y, color.RGBA{200, 100, 50, a})
		}
	}
	return img
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	must(png.Encode(f, img))
}

func writeJPEG(path string, img image.Image, q int) {
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	must(jpeg.Encode(f, img, &jpeg.Options{Quality: q}))
}

func writeGIF(path string, frameCount int) {
	g := &gif.GIF{}
	// Build a small shared palette; gif.EncodeAll requires paletted frames.
	pal := color.Palette{}
	for i := 0; i < 256; i++ {
		pal = append(pal, color.RGBA{uint8(i), uint8(i), uint8(i), 255})
	}
	for i := 0; i < frameCount; i++ {
		img := render(func(x, y, w, h int) color.RGBA {
			v := uint8((x + i*17) % 256)
			return color.RGBA{v, uint8(y % 256), 128, 255}
		}, 64, 64)
		p := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				p.SetColorIndex(x, y, color.RGBAModel.Convert(img.At(x, y)).(color.RGBA).R)
			}
		}
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 10)
	}
	f, err := os.Create(path)
	must(err)
	defer f.Close()
	must(gif.EncodeAll(f, g))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
