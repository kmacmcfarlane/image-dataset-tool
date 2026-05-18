// generate_testdata creates synthetic test images for auto-correction POC evaluation.
// It generates 5+ images per category: solid borders, gradient borders, and photo-of-artwork.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
)

func main() {
	outDir := "testdata"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	// Category 1: Solid-color borders
	generateSolidBorder(outDir, "solid_white_even.png", 400, 300, 20, color.White)
	generateSolidBorder(outDir, "solid_black_even.png", 400, 300, 20, color.Black)
	generateSolidBorder(outDir, "solid_white_uneven.png", 400, 300, 0, color.White) // uneven borders
	generateSolidBorderUneven(outDir, "solid_white_uneven2.png", 400, 300, 10, 30, 5, 25, color.White)
	generateSolidBorderUneven(outDir, "solid_gray_thin.png", 400, 300, 2, 2, 2, 2, color.RGBA{128, 128, 128, 255})
	generateSolidBorderUneven(outDir, "solid_mixed_thick.png", 500, 400, 40, 40, 40, 40, color.RGBA{240, 240, 240, 255})

	// Category 2: Gradient borders
	generateGradientBorder(outDir, "gradient_white_fade.png", 400, 300, 30, true)
	generateGradientBorder(outDir, "gradient_black_fade.png", 400, 300, 30, false)
	generateGradientBorderUneven(outDir, "gradient_top_only.png", 400, 300, 30, 0, 0, 0, true)
	generateGradientBorderUneven(outDir, "gradient_sides.png", 400, 300, 0, 0, 25, 25, true)
	generateGradientBorder(outDir, "gradient_thin.png", 400, 300, 8, true)
	generateGradientBorderRadial(outDir, "gradient_vignette.png", 400, 300)

	// Category 3: Photo-of-artwork (rotated/skewed content on a background)
	generatePhotoOfArtwork(outDir, "art_rotated_5deg.png", 500, 400, 5.0)
	generatePhotoOfArtwork(outDir, "art_rotated_neg3deg.png", 500, 400, -3.0)
	generatePhotoOfArtwork(outDir, "art_rotated_10deg.png", 500, 400, 10.0)
	generatePhotoOfArtworkPerspective(outDir, "art_perspective_mild.png", 500, 400, 0.03)
	generatePhotoOfArtworkPerspective(outDir, "art_perspective_strong.png", 500, 400, 0.08)
	generatePhotoOfArtworkWithBorder(outDir, "art_with_frame.png", 500, 400, 8.0)

	fmt.Println("Generated test images in", outDir)
}

func saveImage(dir, name string, img image.Image) {
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	fmt.Printf("  Created %s (%dx%d)\n", path, img.Bounds().Dx(), img.Bounds().Dy())
}

// makeContentImage creates a colorful content image with recognizable patterns.
func makeContentImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(42))
	// Fill with random colored rectangles to simulate content
	for i := 0; i < 20; i++ {
		rx := rng.Intn(w)
		ry := rng.Intn(h)
		rw := 30 + rng.Intn(w/3)
		rh := 30 + rng.Intn(h/3)
		c := color.RGBA{
			R: uint8(60 + rng.Intn(180)),
			G: uint8(60 + rng.Intn(180)),
			B: uint8(60 + rng.Intn(180)),
			A: 255,
		}
		draw.Draw(img, image.Rect(rx, ry, rx+rw, ry+rh), &image.Uniform{c}, image.Point{}, draw.Src)
	}
	return img
}

func generateSolidBorder(dir, name string, w, h, border int, bg color.Color) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	content := makeContentImage(w-2*border, h-2*border)
	draw.Draw(img, image.Rect(border, border, w-border, h-border), content, image.Point{}, draw.Src)
	saveImage(dir, name, img)
}

func generateSolidBorderUneven(dir, name string, w, h, top, bottom, left, right int, bg color.Color) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	cw := w - left - right
	ch := h - top - bottom
	if cw <= 0 || ch <= 0 {
		cw, ch = w/2, h/2
	}
	content := makeContentImage(cw, ch)
	draw.Draw(img, image.Rect(left, top, left+cw, top+ch), content, image.Point{}, draw.Src)
	saveImage(dir, name, img)
}

func generateGradientBorder(dir, name string, w, h, border int, toWhite bool) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	content := makeContentImage(w, h)
	draw.Draw(img, img.Bounds(), content, image.Point{}, draw.Src)

	// Apply gradient fade on all edges
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			distTop := float64(y)
			distBot := float64(h - 1 - y)
			distLeft := float64(x)
			distRight := float64(w - 1 - x)
			minDist := math.Min(math.Min(distTop, distBot), math.Min(distLeft, distRight))

			if minDist < float64(border) {
				t := minDist / float64(border) // 0 at edge, 1 at border depth
				r, g, b, a := img.At(x, y).RGBA()
				var tr, tg, tb uint8
				if toWhite {
					tr, tg, tb = 255, 255, 255
				}
				nr := uint8(float64(tr)*(1-t) + float64(r>>8)*t)
				ng := uint8(float64(tg)*(1-t) + float64(g>>8)*t)
				nb := uint8(float64(tb)*(1-t) + float64(b>>8)*t)
				na := uint8(float64(a>>8)*t + 255*(1-t))
				img.SetRGBA(x, y, color.RGBA{nr, ng, nb, na})
			}
		}
	}
	saveImage(dir, name, img)
}

func generateGradientBorderUneven(dir, name string, w, h, top, bottom, left, right int, toWhite bool) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	content := makeContentImage(w, h)
	draw.Draw(img, img.Bounds(), content, image.Point{}, draw.Src)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var t float64 = 1.0

			if top > 0 && y < top {
				t = math.Min(t, float64(y)/float64(top))
			}
			if bottom > 0 && y >= h-bottom {
				t = math.Min(t, float64(h-1-y)/float64(bottom))
			}
			if left > 0 && x < left {
				t = math.Min(t, float64(x)/float64(left))
			}
			if right > 0 && x >= w-right {
				t = math.Min(t, float64(w-1-x)/float64(right))
			}

			if t < 1.0 {
				r, g, b, a := img.At(x, y).RGBA()
				var tr, tg, tb uint8
				if toWhite {
					tr, tg, tb = 255, 255, 255
				}
				nr := uint8(float64(tr)*(1-t) + float64(r>>8)*t)
				ng := uint8(float64(tg)*(1-t) + float64(g>>8)*t)
				nb := uint8(float64(tb)*(1-t) + float64(b>>8)*t)
				na := uint8(float64(a>>8)*t + 255*(1-t))
				img.SetRGBA(x, y, color.RGBA{nr, ng, nb, na})
			}
		}
	}
	saveImage(dir, name, img)
}

func generateGradientBorderRadial(dir, name string, w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	content := makeContentImage(w, h)
	draw.Draw(img, img.Bounds(), content, image.Point{}, draw.Src)

	cx, cy := float64(w)/2, float64(h)/2
	maxR := math.Sqrt(cx*cx + cy*cy)
	fadeStart := maxR * 0.7

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > fadeStart {
				t := 1.0 - (dist-fadeStart)/(maxR-fadeStart)
				if t < 0 {
					t = 0
				}
				r, g, b, _ := img.At(x, y).RGBA()
				nr := uint8(float64(r>>8) * t)
				ng := uint8(float64(g>>8) * t)
				nb := uint8(float64(b>>8) * t)
				img.SetRGBA(x, y, color.RGBA{nr, ng, nb, 255})
			}
		}
	}
	saveImage(dir, name, img)
}

func generatePhotoOfArtwork(dir, name string, w, h int, angleDeg float64) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Background: off-white wall
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{220, 215, 210, 255}}, image.Point{}, draw.Src)

	// Content (the "artwork")
	artW, artH := w*2/3, h*2/3
	art := makeContentImage(artW, artH)

	// Place rotated artwork in center using rotation
	rad := angleDeg * math.Pi / 180.0
	sinA := math.Sin(rad)
	cosA := math.Cos(rad)
	cx, cy := float64(w)/2, float64(h)/2
	acx, acy := float64(artW)/2, float64(artH)/2

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			ox := float64(dx) - cx
			oy := float64(dy) - cy
			// Inverse rotate to get source art coordinate
			sx := ox*cosA + oy*sinA + acx
			sy := -ox*sinA + oy*cosA + acy
			ix, iy := int(sx), int(sy)
			if ix >= 0 && ix < artW && iy >= 0 && iy < artH {
				img.Set(dx, dy, art.At(ix, iy))
			}
		}
	}
	saveImage(dir, name, img)
}

func generatePhotoOfArtworkPerspective(dir, name string, w, h int, skewFactor float64) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{220, 215, 210, 255}}, image.Point{}, draw.Src)

	artW, artH := w*2/3, h*2/3
	art := makeContentImage(artW, artH)

	cx, cy := float64(w)/2, float64(h)/2
	acx, acy := float64(artW)/2, float64(artH)/2

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			ox := float64(dx) - cx
			oy := float64(dy) - cy
			// Simple perspective: horizontal shift proportional to y
			sx := ox + oy*skewFactor*ox/cx + acx
			sy := oy + acy
			ix, iy := int(sx), int(sy)
			if ix >= 0 && ix < artW && iy >= 0 && iy < artH {
				img.Set(dx, dy, art.At(ix, iy))
			}
		}
	}
	saveImage(dir, name, img)
}

func generatePhotoOfArtworkWithBorder(dir, name string, w, h int, angleDeg float64) {
	// Artwork with a visible frame/border, then rotated
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{200, 195, 190, 255}}, image.Point{}, draw.Src)

	artW, artH := w*2/3, h*2/3
	frameBorder := 8
	art := image.NewRGBA(image.Rect(0, 0, artW, artH))
	// Frame: dark brown border
	draw.Draw(art, art.Bounds(), &image.Uniform{color.RGBA{80, 50, 30, 255}}, image.Point{}, draw.Src)
	content := makeContentImage(artW-2*frameBorder, artH-2*frameBorder)
	draw.Draw(art, image.Rect(frameBorder, frameBorder, artW-frameBorder, artH-frameBorder), content, image.Point{}, draw.Src)

	rad := angleDeg * math.Pi / 180.0
	sinA := math.Sin(rad)
	cosA := math.Cos(rad)
	cx, cy := float64(w)/2, float64(h)/2
	acx, acy := float64(artW)/2, float64(artH)/2

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			ox := float64(dx) - cx
			oy := float64(dy) - cy
			sx := ox*cosA + oy*sinA + acx
			sy := -ox*sinA + oy*cosA + acy
			ix, iy := int(sx), int(sy)
			if ix >= 0 && ix < artW && iy >= 0 && iy < artH {
				img.Set(dx, dy, art.At(ix, iy))
			}
		}
	}
	saveImage(dir, name, img)
}
