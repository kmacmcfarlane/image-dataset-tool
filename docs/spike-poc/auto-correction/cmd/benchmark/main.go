// benchmark measures CPU cost of auto-correction algorithms at various image sizes.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"math/rand"
	"time"
)

func main() {
	sizes := []struct{ w, h int }{
		{640, 480},
		{1024, 768},
		{1920, 1080},
		{3840, 2160},
	}

	fmt.Println("=== Auto-Correction CPU Benchmark ===")
	fmt.Printf("%-16s %-24s %-12s\n", "Size", "Algorithm", "Time")
	fmt.Println("-----------------------------------------------------------")

	for _, sz := range sizes {
		img := makeTestImage(sz.w, sz.h)
		label := fmt.Sprintf("%dx%d", sz.w, sz.h)

		// Solid border detection
		bench(label, "solid-border-detect", func() {
			detectSolidBorder(img, 15.0)
		})

		// Gradient border detection
		bench(label, "gradient-border-detect", func() {
			detectGradientBorder(img)
		})

		// Sobel edge detection
		bench(label, "sobel-edges", func() {
			gray := toGray(img)
			sobelEdges(gray, sz.w, sz.h)
		})

		// Hough angle detection
		bench(label, "hough-angle", func() {
			gray := toGray(img)
			edges := sobelEdges(gray, sz.w, sz.h)
			houghAngleDetect(edges, sz.w, sz.h)
		})

		// Full artwork correction pipeline
		bench(label, "full-artwork-pipeline", func() {
			detectArtworkRotation(img)
		})

		fmt.Println()
	}
}

func bench(size, name string, fn func()) {
	// Warmup
	fn()

	runs := 5
	var total time.Duration
	for range runs {
		start := time.Now()
		fn()
		total += time.Since(start)
	}
	avg := total / time.Duration(runs)
	fmt.Printf("%-16s %-24s %-12v\n", size, name, avg)
}

func makeTestImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Add border
	border := 20
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 30; i++ {
		rx := border + rng.Intn(w-2*border)
		ry := border + rng.Intn(h-2*border)
		rw := 30 + rng.Intn(w/4)
		rh := 30 + rng.Intn(h/4)
		c := color.RGBA{uint8(60 + rng.Intn(180)), uint8(60 + rng.Intn(180)), uint8(60 + rng.Intn(180)), 255}
		draw.Draw(img, image.Rect(rx, ry, rx+rw, ry+rh), &image.Uniform{c}, image.Point{}, draw.Src)
	}
	return img
}

// Copied from POC for benchmarking (to keep this self-contained)

func detectSolidBorder(img image.Image, threshold float64) (top, bottom, left, right int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	borderColor := dominantCornerColor(img)
	for y := 0; y < h; y++ {
		if !isRowSolidColor(img, y, borderColor, threshold) {
			break
		}
		top = y + 1
	}
	for y := h - 1; y >= top; y-- {
		if !isRowSolidColor(img, y, borderColor, threshold) {
			break
		}
		bottom = h - y
	}
	for x := 0; x < w; x++ {
		if !isColSolidColor(img, x, top, h-bottom, borderColor, threshold) {
			break
		}
		left = x + 1
	}
	for x := w - 1; x >= left; x-- {
		if !isColSolidColor(img, x, top, h-bottom, borderColor, threshold) {
			break
		}
		right = w - x
	}
	return
}

func dominantCornerColor(img image.Image) color.RGBA {
	b := img.Bounds()
	corners := []image.Point{{b.Min.X, b.Min.Y}, {b.Max.X - 1, b.Min.Y}, {b.Min.X, b.Max.Y - 1}, {b.Max.X - 1, b.Max.Y - 1}}
	var rSum, gSum, bSum float64
	for _, p := range corners {
		r, g, bl, _ := img.At(p.X, p.Y).RGBA()
		rSum += float64(r >> 8)
		gSum += float64(g >> 8)
		bSum += float64(bl >> 8)
	}
	n := float64(len(corners))
	return color.RGBA{uint8(rSum / n), uint8(gSum / n), uint8(bSum / n), 255}
}

func colorDistance(c1, c2 color.RGBA) float64 {
	dr := float64(c1.R) - float64(c2.R)
	dg := float64(c1.G) - float64(c2.G)
	db := float64(c1.B) - float64(c2.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

func isRowSolidColor(img image.Image, y int, target color.RGBA, threshold float64) bool {
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		r, g, bl, _ := img.At(x, b.Min.Y+y).RGBA()
		c := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255}
		if colorDistance(c, target) > threshold {
			return false
		}
	}
	return true
}

func isColSolidColor(img image.Image, x, yStart, yEnd int, target color.RGBA, threshold float64) bool {
	b := img.Bounds()
	for y := yStart; y < yEnd; y++ {
		r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		c := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255}
		if colorDistance(c, target) > threshold {
			return false
		}
	}
	return true
}

type scanDir int

const (
	scanTop scanDir = iota
	scanBottom
	scanLeft
	scanRight
)

func detectGradientBorder(img image.Image) (top, bottom, left, right int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	top = findGradientEdge(img, w, h, scanTop)
	bottom = findGradientEdge(img, w, h, scanBottom)
	left = findGradientEdge(img, w, h, scanLeft)
	right = findGradientEdge(img, w, h, scanRight)
	return
}

func findGradientEdge(img image.Image, w, h int, dir scanDir) int {
	b := img.Bounds()
	maxScan := w
	if dir == scanTop || dir == scanBottom {
		maxScan = h
	}
	maxScan = maxScan / 4
	variances := make([]float64, maxScan)
	for i := 0; i < maxScan; i++ {
		switch dir {
		case scanTop:
			variances[i] = rowVariance(img, b.Min.Y+i, b.Min.X, b.Max.X)
		case scanBottom:
			variances[i] = rowVariance(img, b.Max.Y-1-i, b.Min.X, b.Max.X)
		case scanLeft:
			variances[i] = colVariance(img, b.Min.X+i, b.Min.Y, b.Max.Y)
		case scanRight:
			variances[i] = colVariance(img, b.Max.X-1-i, b.Min.Y, b.Max.Y)
		}
	}
	if len(variances) == 0 {
		return 0
	}
	edgeVar := variances[0]
	if edgeVar < 1.0 {
		edgeVar = 1.0
	}
	for i := 1; i < len(variances); i++ {
		if variances[i] > edgeVar*5 && variances[i] > 500 {
			return i
		}
	}
	return 0
}

func rowVariance(img image.Image, y, xStart, xEnd int) float64 {
	n := float64(xEnd - xStart)
	if n <= 1 {
		return 0
	}
	var sumR, sumG, sumB, sum2R, sum2G, sum2B float64
	for x := xStart; x < xEnd; x++ {
		r, g, b, _ := img.At(x, y).RGBA()
		fr, fg, fb := float64(r>>8), float64(g>>8), float64(b>>8)
		sumR += fr
		sumG += fg
		sumB += fb
		sum2R += fr * fr
		sum2G += fg * fg
		sum2B += fb * fb
	}
	return ((sum2R/n - (sumR/n)*(sumR/n)) + (sum2G/n - (sumG/n)*(sumG/n)) + (sum2B/n - (sumB/n)*(sumB/n))) / 3
}

func colVariance(img image.Image, x, yStart, yEnd int) float64 {
	n := float64(yEnd - yStart)
	if n <= 1 {
		return 0
	}
	var sumR, sumG, sumB, sum2R, sum2G, sum2B float64
	for y := yStart; y < yEnd; y++ {
		r, g, b, _ := img.At(x, y).RGBA()
		fr, fg, fb := float64(r>>8), float64(g>>8), float64(b>>8)
		sumR += fr
		sumG += fg
		sumB += fb
		sum2R += fr * fr
		sum2G += fg * fg
		sum2B += fb * fb
	}
	return ((sum2R/n - (sumR/n)*(sumR/n)) + (sum2G/n - (sumG/n)*(sumG/n)) + (sum2B/n - (sumB/n)*(sumB/n))) / 3
}

func toGray(img image.Image) []float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	gray := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			gray[y*w+x] = 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
		}
	}
	return gray
}

func sobelEdges(gray []float64, w, h int) []float64 {
	edges := make([]float64, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			gx := -gray[(y-1)*w+(x-1)] + gray[(y-1)*w+(x+1)] - 2*gray[y*w+(x-1)] + 2*gray[y*w+(x+1)] - gray[(y+1)*w+(x-1)] + gray[(y+1)*w+(x+1)]
			gy := -gray[(y-1)*w+(x-1)] - 2*gray[(y-1)*w+x] - gray[(y-1)*w+(x+1)] + gray[(y+1)*w+(x-1)] + 2*gray[(y+1)*w+x] + gray[(y+1)*w+(x+1)]
			edges[y*w+x] = math.Sqrt(gx*gx + gy*gy)
		}
	}
	return edges
}

func houghAngleDetect(edges []float64, w, h int) float64 {
	threshold := edgeThreshold(edges, 0.90)
	numBins := 81
	bins := make([]float64, numBins)
	for y := 2; y < h-2; y++ {
		for x := 2; x < w-2; x++ {
			if edges[y*w+x] < threshold {
				continue
			}
			gx := -edges[y*w+(x-1)] + edges[y*w+(x+1)]
			gy := -edges[(y-1)*w+x] + edges[(y+1)*w+x]
			angle := math.Atan2(gy, gx) * 180 / math.Pi
			skew := angle
			for skew > 45 {
				skew -= 90
			}
			for skew < -45 {
				skew += 90
			}
			if skew >= -20 && skew <= 20 {
				bin := int((skew + 20) / 0.5)
				if bin >= 0 && bin < numBins {
					bins[bin] += edges[y*w+x]
				}
			}
		}
	}
	maxBin := 0
	maxVal := 0.0
	for i, v := range bins {
		if v > maxVal {
			maxVal = v
			maxBin = i
		}
	}
	return float64(maxBin)*0.5 - 20.0
}

func edgeThreshold(edges []float64, percentile float64) float64 {
	maxEdge := 0.0
	for _, e := range edges {
		if e > maxEdge {
			maxEdge = e
		}
	}
	if maxEdge == 0 {
		return 0
	}
	numBins := 256
	hist := make([]int, numBins)
	total := 0
	for _, e := range edges {
		bin := int(e / maxEdge * float64(numBins-1))
		if bin >= numBins {
			bin = numBins - 1
		}
		hist[bin]++
		total++
	}
	target := int(float64(total) * percentile)
	cumulative := 0
	for i, count := range hist {
		cumulative += count
		if cumulative >= target {
			return float64(i) / float64(numBins-1) * maxEdge
		}
	}
	return maxEdge
}

func detectArtworkRotation(img image.Image) float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	gray := toGray(img)
	edgeMag := sobelEdges(gray, w, h)
	return houghAngleDetect(edgeMag, w, h)
}
