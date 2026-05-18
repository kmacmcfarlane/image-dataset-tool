// poc runs proof-of-concept auto-correction algorithms on test images and
// measures accuracy and performance.
//
// Categories evaluated:
//   1. Solid-color border removal
//   2. Gradient border detection
//   3. Photo-of-artwork correction (rotation detection)
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	testDir := "testdata"
	outDir := "output"
	if len(os.Args) > 1 {
		testDir = os.Args[1]
	}
	if len(os.Args) > 2 {
		outDir = os.Args[2]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	entries, err := os.ReadDir(testDir)
	if err != nil {
		panic(err)
	}

	fmt.Println("=== Auto-Correction POC Results ===")
	fmt.Println()

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		path := filepath.Join(testDir, e.Name())
		img := loadImage(path)

		fmt.Printf("--- %s (%dx%d) ---\n", e.Name(), img.Bounds().Dx(), img.Bounds().Dy())

		if strings.HasPrefix(e.Name(), "solid_") {
			evaluateSolidBorderRemoval(img, e.Name(), outDir)
		} else if strings.HasPrefix(e.Name(), "gradient_") {
			evaluateGradientBorderDetection(img, e.Name(), outDir)
		} else if strings.HasPrefix(e.Name(), "art_") {
			evaluateArtworkCorrection(img, e.Name(), outDir)
		}
		fmt.Println()
	}
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	return img
}

func saveResult(img image.Image, dir, name string) {
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

// ============================================================================
// Category 1: Solid-color border removal
// ============================================================================

// detectSolidBorder scans rows/columns from each edge to find uniform-color
// borders. It uses a color distance threshold to tolerate minor variations
// (JPEG artifacts, slight color shifts).
func detectSolidBorder(img image.Image, threshold float64) (top, bottom, left, right int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Determine the dominant border color by sampling corners
	borderColor := dominantCornerColor(img)

	// Scan from top
	for y := 0; y < h; y++ {
		if !isRowSolidColor(img, y, borderColor, threshold) {
			break
		}
		top = y + 1
	}

	// Scan from bottom
	for y := h - 1; y >= top; y-- {
		if !isRowSolidColor(img, y, borderColor, threshold) {
			break
		}
		bottom = h - y
	}

	// Scan from left
	for x := 0; x < w; x++ {
		if !isColSolidColor(img, x, top, h-bottom, borderColor, threshold) {
			break
		}
		left = x + 1
	}

	// Scan from right
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
	corners := []image.Point{
		{b.Min.X, b.Min.Y},
		{b.Max.X - 1, b.Min.Y},
		{b.Min.X, b.Max.Y - 1},
		{b.Max.X - 1, b.Max.Y - 1},
	}
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

func removeSolidBorder(img image.Image, top, bottom, left, right int) image.Image {
	b := img.Bounds()
	cropRect := image.Rect(
		b.Min.X+left,
		b.Min.Y+top,
		b.Max.X-right,
		b.Max.Y-bottom,
	)
	if cropRect.Empty() {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, cropRect.Min, draw.Src)
	return dst
}

func evaluateSolidBorderRemoval(img image.Image, name, outDir string) {
	start := time.Now()
	top, bottom, left, right := detectSolidBorder(img, 15.0)
	detectTime := time.Since(start)

	fmt.Printf("  Detected borders: top=%d bottom=%d left=%d right=%d\n", top, bottom, left, right)
	fmt.Printf("  Detection time: %v\n", detectTime)

	if top > 0 || bottom > 0 || left > 0 || right > 0 {
		start = time.Now()
		result := removeSolidBorder(img, top, bottom, left, right)
		cropTime := time.Since(start)
		fmt.Printf("  Result size: %dx%d (crop time: %v)\n",
			result.Bounds().Dx(), result.Bounds().Dy(), cropTime)
		saveResult(result, outDir, "corrected_"+name)
	} else {
		fmt.Println("  No solid border detected")
	}
}

// ============================================================================
// Category 2: Gradient border detection
// ============================================================================

// detectGradientBorder detects gradient fade borders by analyzing the variance
// of pixel values along each edge. It scans inward from each edge, measuring
// how quickly pixel variance increases (indicating transition from gradient to content).
func detectGradientBorder(img image.Image) (top, bottom, left, right int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// For each edge, compute row/column entropy moving inward.
	// A gradient border has low entropy near the edge that increases sharply.

	// Top edge
	top = findGradientEdge(img, w, h, scanTop)
	bottom = findGradientEdge(img, w, h, scanBottom)
	left = findGradientEdge(img, w, h, scanLeft)
	right = findGradientEdge(img, w, h, scanRight)

	return
}

type scanDir int

const (
	scanTop scanDir = iota
	scanBottom
	scanLeft
	scanRight
)

// findGradientEdge scans from one edge inward, looking for the transition
// point where row/column variance jumps, indicating the gradient boundary.
func findGradientEdge(img image.Image, w, h int, dir scanDir) int {
	b := img.Bounds()
	maxScan := w
	if dir == scanTop || dir == scanBottom {
		maxScan = h
	}
	// Don't scan more than 25% of the dimension
	maxScan = maxScan / 4

	// Compute variance for each row/column from the edge
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

	// Find the point where variance jumps significantly
	// Use a ratio-based approach: when variance exceeds 3x the edge variance
	if len(variances) == 0 {
		return 0
	}

	edgeVar := variances[0]
	if edgeVar < 1.0 {
		edgeVar = 1.0 // avoid division by zero
	}

	// Also look for a sustained high-variance region
	borderEnd := 0
	for i := 1; i < len(variances); i++ {
		if variances[i] > edgeVar*5 && variances[i] > 500 {
			borderEnd = i
			break
		}
	}

	return borderEnd
}

func rowVariance(img image.Image, y, xStart, xEnd int) float64 {
	n := float64(xEnd - xStart)
	if n <= 1 {
		return 0
	}
	var sumR, sumG, sumB float64
	var sum2R, sum2G, sum2B float64
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
	varR := sum2R/n - (sumR/n)*(sumR/n)
	varG := sum2G/n - (sumG/n)*(sumG/n)
	varB := sum2B/n - (sumB/n)*(sumB/n)
	return (varR + varG + varB) / 3
}

func colVariance(img image.Image, x, yStart, yEnd int) float64 {
	n := float64(yEnd - yStart)
	if n <= 1 {
		return 0
	}
	var sumR, sumG, sumB float64
	var sum2R, sum2G, sum2B float64
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
	varR := sum2R/n - (sumR/n)*(sumR/n)
	varG := sum2G/n - (sumG/n)*(sumG/n)
	varB := sum2B/n - (sumB/n)*(sumB/n)
	return (varR + varG + varB) / 3
}

func evaluateGradientBorderDetection(img image.Image, name, outDir string) {
	start := time.Now()
	top, bottom, left, right := detectGradientBorder(img)
	detectTime := time.Since(start)

	fmt.Printf("  Detected gradient borders: top=%d bottom=%d left=%d right=%d\n", top, bottom, left, right)
	fmt.Printf("  Detection time: %v\n", detectTime)

	if top > 0 || bottom > 0 || left > 0 || right > 0 {
		result := removeSolidBorder(img, top, bottom, left, right)
		fmt.Printf("  Result size: %dx%d\n", result.Bounds().Dx(), result.Bounds().Dy())
		saveResult(result, outDir, "corrected_"+name)
	} else {
		fmt.Println("  No gradient border detected")
	}
}

// ============================================================================
// Category 3: Photo-of-artwork rotation detection
// ============================================================================

// detectArtworkRotation attempts to detect the rotation angle of a rectangular
// artwork photographed against a background. It uses edge detection (Sobel)
// followed by a simplified Hough transform to find dominant line angles.
func detectArtworkRotation(img image.Image) float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Convert to grayscale
	gray := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			gray[y*w+x] = 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
		}
	}

	// Sobel edge detection
	edgeMag := sobelEdges(gray, w, h)

	// Simple Hough transform for near-horizontal and near-vertical lines
	// We only care about angles near 0 or 90 degrees (artwork edges)
	angle := houghAngleDetect(edgeMag, w, h)

	return angle
}

func sobelEdges(gray []float64, w, h int) []float64 {
	edges := make([]float64, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			// Sobel X
			gx := -gray[(y-1)*w+(x-1)] + gray[(y-1)*w+(x+1)] +
				-2*gray[y*w+(x-1)] + 2*gray[y*w+(x+1)] +
				-gray[(y+1)*w+(x-1)] + gray[(y+1)*w+(x+1)]
			// Sobel Y
			gy := -gray[(y-1)*w+(x-1)] - 2*gray[(y-1)*w+x] - gray[(y-1)*w+(x+1)] +
				gray[(y+1)*w+(x-1)] + 2*gray[(y+1)*w+x] + gray[(y+1)*w+(x+1)]

			edges[y*w+x] = math.Sqrt(gx*gx + gy*gy)
		}
	}
	return edges
}

// houghAngleDetect uses a simplified angle accumulator to find the dominant
// line angle. Only searches near 0 degrees (horizontal) and 90 degrees (vertical)
// since we expect rectangular artwork.
func houghAngleDetect(edges []float64, w, h int) float64 {
	// Find edge magnitude threshold (top 10%)
	threshold := edgeThreshold(edges, 0.90)

	// Accumulate angles from strong edge pixels using gradient direction
	// We bin angles into 0.5-degree buckets from -20 to +20 degrees
	numBins := 81 // -20 to +20 in 0.5 degree steps
	bins := make([]float64, numBins)

	for y := 2; y < h-2; y++ {
		for x := 2; x < w-2; x++ {
			if edges[y*w+x] < threshold {
				continue
			}
			// Compute gradient direction
			gx := -edges[(y)*w+(x-1)] + edges[(y)*w+(x+1)]
			gy := -edges[(y-1)*w+(x)] + edges[(y+1)*w+(x)]
			angle := math.Atan2(gy, gx) * 180 / math.Pi

			// Normalize to range near 0 (horizontal edges) or near 90 (vertical edges)
			// We want the skew from horizontal/vertical
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

	// Find peak bin
	maxBin := 0
	maxVal := 0.0
	for i, v := range bins {
		if v > maxVal {
			maxVal = v
			maxBin = i
		}
	}

	detectedAngle := float64(maxBin)*0.5 - 20.0
	return detectedAngle
}

func edgeThreshold(edges []float64, percentile float64) float64 {
	// Simple histogram-based threshold
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

// detectArtworkBounds attempts to find the bounding rectangle of the artwork
// within the photo using edge detection and contour analysis.
func detectArtworkBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Convert to grayscale
	gray := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			gray[y*w+x] = 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
		}
	}

	edges := sobelEdges(gray, w, h)
	threshold := edgeThreshold(edges, 0.85)

	// Find bounding box of strong edges (simple approach)
	minX, minY := w, h
	maxX, maxY := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if edges[y*w+x] > threshold {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	// Add small margin
	margin := 2
	minX = max(0, minX-margin)
	minY = max(0, minY-margin)
	maxX = min(w-1, maxX+margin)
	maxY = min(h-1, maxY+margin)

	return image.Rect(b.Min.X+minX, b.Min.Y+minY, b.Min.X+maxX+1, b.Min.Y+maxY+1)
}

func evaluateArtworkCorrection(img image.Image, name, outDir string) {
	// Rotation detection
	start := time.Now()
	angle := detectArtworkRotation(img)
	rotDetectTime := time.Since(start)
	fmt.Printf("  Detected rotation: %.2f degrees\n", angle)
	fmt.Printf("  Rotation detection time: %v\n", rotDetectTime)

	// Artwork bounds detection
	start = time.Now()
	bounds := detectArtworkBounds(img)
	boundsDetectTime := time.Since(start)
	fmt.Printf("  Detected artwork bounds: %v\n", bounds)
	fmt.Printf("  Bounds detection time: %v\n", boundsDetectTime)

	// Apply corrections: rotate then crop
	if math.Abs(angle) > 0.5 {
		start = time.Now()
		corrected := rotateImageBilinear(img, -angle)
		rotateTime := time.Since(start)
		fmt.Printf("  Rotation correction time: %v\n", rotateTime)

		// Re-detect bounds after rotation
		newBounds := detectArtworkBounds(corrected)
		if !newBounds.Empty() {
			result := cropToRect(corrected, newBounds)
			fmt.Printf("  Final result size: %dx%d\n", result.Bounds().Dx(), result.Bounds().Dy())
			saveResult(result, outDir, "corrected_"+name)
		} else {
			saveResult(corrected, outDir, "corrected_"+name)
		}
	} else {
		// Just crop to detected bounds
		if !bounds.Empty() {
			result := cropToRect(img, bounds)
			fmt.Printf("  Cropped result size: %dx%d\n", result.Bounds().Dx(), result.Bounds().Dy())
			saveResult(result, outDir, "corrected_"+name)
		}
	}
}

func rotateImageBilinear(src image.Image, degrees float64) image.Image {
	b := src.Bounds()
	srcW := float64(b.Dx())
	srcH := float64(b.Dy())

	rad := degrees * math.Pi / 180.0
	sinA := math.Sin(rad)
	cosA := math.Cos(rad)

	absSin := math.Abs(sinA)
	absCos := math.Abs(cosA)
	newW := int(math.Ceil(srcW*absCos + srcH*absSin))
	newH := int(math.Ceil(srcW*absSin + srcH*absCos))

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	srcCX := srcW / 2.0
	srcCY := srcH / 2.0
	dstCX := float64(newW) / 2.0
	dstCY := float64(newH) / 2.0

	bg := dominantCornerColor(src)

	for dy := 0; dy < newH; dy++ {
		for dx := 0; dx < newW; dx++ {
			ox := float64(dx) - dstCX + 0.5
			oy := float64(dy) - dstCY + 0.5
			sx := ox*cosA + oy*sinA + srcCX - 0.5
			sy := -ox*sinA + oy*cosA + srcCY - 0.5

			ix := int(math.Floor(sx))
			iy := int(math.Floor(sy))
			if ix >= b.Min.X && ix < b.Max.X && iy >= b.Min.Y && iy < b.Max.Y {
				dst.Set(dx, dy, src.At(ix, iy))
			} else {
				dst.Set(dx, dy, bg)
			}
		}
	}
	return dst
}

func cropToRect(img image.Image, r image.Rectangle) image.Image {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	return dst
}
