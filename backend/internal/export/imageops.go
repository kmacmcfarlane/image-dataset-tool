package export

import (
	"image"
	"image/color"
	"math"
)

// rotateImage rotates an image by the given angle in degrees (counterclockwise).
// For non-90-degree rotations the canvas expands to fit the full rotated image,
// with transparent/black fill for uncovered areas.
func rotateImage(src image.Image, degrees float64) image.Image {
	// Normalize to [0, 360).
	degrees = math.Mod(degrees, 360)
	if degrees < 0 {
		degrees += 360
	}

	// Fast paths for cardinal rotations (no quality loss from interpolation).
	switch degrees {
	case 0:
		return src
	case 90:
		return rotate90(src)
	case 180:
		return rotate180(src)
	case 270:
		return rotate270(src)
	}

	return rotateFreeAngle(src, degrees)
}

// rotate90 rotates an image 90 degrees counterclockwise.
func rotate90(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < w; y++ {
		for x := 0; x < h; x++ {
			dst.Set(x, y, src.At(b.Min.X+(w-1-y), b.Min.Y+x))
		}
	}
	return dst
}

// rotate180 rotates an image 180 degrees.
func rotate180(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+(w-1-x), b.Min.Y+(h-1-y)))
		}
	}
	return dst
}

// rotate270 rotates an image 270 degrees counterclockwise (= 90 clockwise).
func rotate270(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < w; y++ {
		for x := 0; x < h; x++ {
			dst.Set(x, y, src.At(b.Min.X+y, b.Min.Y+(h-1-x)))
		}
	}
	return dst
}

// rotateFreeAngle performs arbitrary-angle rotation using bilinear interpolation.
// The output canvas is expanded to contain the entire rotated image.
func rotateFreeAngle(src image.Image, degrees float64) image.Image {
	b := src.Bounds()
	srcW := float64(b.Dx())
	srcH := float64(b.Dy())

	// Convert to radians (counterclockwise).
	rad := degrees * math.Pi / 180.0
	sinA := math.Sin(rad)
	cosA := math.Cos(rad)

	// Compute new bounding box that encloses the rotated rectangle.
	absSin := math.Abs(sinA)
	absCos := math.Abs(cosA)
	newW := int(math.Ceil(srcW*absCos + srcH*absSin))
	newH := int(math.Ceil(srcW*absSin + srcH*absCos))

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	// Center of source and destination.
	srcCX := srcW / 2.0
	srcCY := srcH / 2.0
	dstCX := float64(newW) / 2.0
	dstCY := float64(newH) / 2.0

	// For each destination pixel, find the corresponding source pixel
	// using inverse rotation.
	for dy := 0; dy < newH; dy++ {
		for dx := 0; dx < newW; dx++ {
			// Offset from destination center.
			ox := float64(dx) - dstCX + 0.5
			oy := float64(dy) - dstCY + 0.5

			// Inverse-rotate to source coordinates.
			sx := ox*cosA + oy*sinA + srcCX - 0.5
			sy := -ox*sinA + oy*cosA + srcCY - 0.5

			c := bilinearSample(src, sx, sy)
			dst.Set(dx, dy, c)
		}
	}
	return dst
}

// bilinearSample samples a pixel from src at fractional coordinates using
// bilinear interpolation. Returns transparent black for out-of-bounds.
func bilinearSample(src image.Image, x, y float64) color.Color {
	b := src.Bounds()
	maxX := float64(b.Dx() - 1)
	maxY := float64(b.Dy() - 1)

	if x < -0.5 || y < -0.5 || x > maxX+0.5 || y > maxY+0.5 {
		return color.RGBA{}
	}

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	xFrac := x - float64(x0)
	yFrac := y - float64(y0)

	c00 := safeAt(src, b.Min.X+x0, b.Min.Y+y0)
	c10 := safeAt(src, b.Min.X+x1, b.Min.Y+y0)
	c01 := safeAt(src, b.Min.X+x0, b.Min.Y+y1)
	c11 := safeAt(src, b.Min.X+x1, b.Min.Y+y1)

	return lerpColor(c00, c10, c01, c11, xFrac, yFrac)
}

// safeAt returns the color at (x, y), or transparent black if out of bounds.
func safeAt(src image.Image, x, y int) color.RGBA64 {
	b := src.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return color.RGBA64{}
	}
	r, g, bb, a := src.At(x, y).RGBA()
	return color.RGBA64{R: uint16(r), G: uint16(g), B: uint16(bb), A: uint16(a)}
}

// lerpColor performs bilinear interpolation between four corner colors.
func lerpColor(c00, c10, c01, c11 color.RGBA64, xFrac, yFrac float64) color.RGBA64 {
	lerp := func(a, b, c, d uint16) uint16 {
		fa := float64(a)
		fb := float64(b)
		fc := float64(c)
		fd := float64(d)
		top := fa*(1-xFrac) + fb*xFrac
		bot := fc*(1-xFrac) + fd*xFrac
		v := top*(1-yFrac) + bot*yFrac
		return uint16(math.Round(v))
	}
	return color.RGBA64{
		R: lerp(c00.R, c10.R, c01.R, c11.R),
		G: lerp(c00.G, c10.G, c01.G, c11.G),
		B: lerp(c00.B, c10.B, c01.B, c11.B),
		A: lerp(c00.A, c10.A, c01.A, c11.A),
	}
}

// cropImage extracts a sub-rectangle from an image.
// Coordinates are in pixels, relative to the image bounds.
func cropImage(src image.Image, box image.Rectangle) image.Image {
	b := src.Bounds()

	// Clamp to source bounds.
	box = box.Intersect(b)
	if box.Empty() {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}

	w := box.Dx()
	h := box.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(box.Min.X+x, box.Min.Y+y))
		}
	}
	return dst
}
