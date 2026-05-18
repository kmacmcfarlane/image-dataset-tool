package export

import (
	"image"
	"image/color"
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// newTestImage creates a solid-color RGBA image of the given dimensions.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	return img
}

var _ = Describe("Image Operations", func() {
	Describe("rotateImage", func() {
		Context("cardinal rotations", func() {
			It("returns the same dimensions for 0 degrees", func() {
				src := newTestImage(100, 60)
				result := rotateImage(src, 0)
				Expect(result.Bounds().Dx()).To(Equal(100))
				Expect(result.Bounds().Dy()).To(Equal(60))
			})

			It("swaps dimensions for 90 degrees", func() {
				src := newTestImage(100, 60)
				result := rotateImage(src, 90)
				Expect(result.Bounds().Dx()).To(Equal(60))
				Expect(result.Bounds().Dy()).To(Equal(100))
			})

			It("preserves dimensions for 180 degrees", func() {
				src := newTestImage(100, 60)
				result := rotateImage(src, 180)
				Expect(result.Bounds().Dx()).To(Equal(100))
				Expect(result.Bounds().Dy()).To(Equal(60))
			})

			It("swaps dimensions for 270 degrees", func() {
				src := newTestImage(100, 60)
				result := rotateImage(src, 270)
				Expect(result.Bounds().Dx()).To(Equal(60))
				Expect(result.Bounds().Dy()).To(Equal(100))
			})
		})

		Context("free-angle rotation", func() {
			It("expands canvas for 45-degree rotation", func() {
				src := newTestImage(100, 100)
				result := rotateImage(src, 45)
				// A 100x100 square rotated 45 degrees has a bounding box of
				// ~141x141 (100*sqrt(2)).
				expected := int(math.Ceil(100 * math.Sqrt(2)))
				Expect(result.Bounds().Dx()).To(Equal(expected))
				Expect(result.Bounds().Dy()).To(Equal(expected))
			})

			It("handles negative angles by normalizing", func() {
				src := newTestImage(100, 60)
				// -90 is equivalent to 270.
				result := rotateImage(src, -90)
				Expect(result.Bounds().Dx()).To(Equal(60))
				Expect(result.Bounds().Dy()).To(Equal(100))
			})

			It("handles angles > 360 by normalizing", func() {
				src := newTestImage(100, 60)
				result := rotateImage(src, 450) // 450 = 90
				Expect(result.Bounds().Dx()).To(Equal(60))
				Expect(result.Bounds().Dy()).To(Equal(100))
			})

			DescribeTable("asymmetric rectangle rotations expand canvas correctly",
				func(degrees float64) {
					src := newTestImage(200, 100)
					result := rotateImage(src, degrees)
					rad := degrees * math.Pi / 180.0
					absSin := math.Abs(math.Sin(rad))
					absCos := math.Abs(math.Cos(rad))
					expectedW := int(math.Ceil(200*absCos + 100*absSin))
					expectedH := int(math.Ceil(200*absSin + 100*absCos))
					Expect(result.Bounds().Dx()).To(Equal(expectedW))
					Expect(result.Bounds().Dy()).To(Equal(expectedH))
				},
				Entry("30 degrees", 30.0),
				Entry("60 degrees", 60.0),
				Entry("15 degrees", 15.0),
				Entry("120 degrees", 120.0),
			)
		})
	})

	Describe("cropImage", func() {
		It("crops to the specified rectangle", func() {
			src := newTestImage(200, 150)
			box := image.Rect(10, 20, 110, 95)
			result := cropImage(src, box)
			Expect(result.Bounds().Dx()).To(Equal(100))
			Expect(result.Bounds().Dy()).To(Equal(75))
		})

		It("clamps crop to source bounds", func() {
			src := newTestImage(100, 100)
			// Crop extends beyond bounds.
			box := image.Rect(50, 50, 200, 200)
			result := cropImage(src, box)
			Expect(result.Bounds().Dx()).To(Equal(50))
			Expect(result.Bounds().Dy()).To(Equal(50))
		})

		It("returns empty image for fully out-of-bounds crop", func() {
			src := newTestImage(100, 100)
			box := image.Rect(200, 200, 300, 300)
			result := cropImage(src, box)
			Expect(result.Bounds().Dx()).To(Equal(0))
			Expect(result.Bounds().Dy()).To(Equal(0))
		})
	})

	Describe("rotation then crop (combined)", func() {
		It("produces expected dimensions for 90-degree rotation + crop", func() {
			// Start with 200x100, rotate 90 → 100x200, crop 50x50 region.
			src := newTestImage(200, 100)
			rotated := rotateImage(src, 90)
			Expect(rotated.Bounds().Dx()).To(Equal(100))
			Expect(rotated.Bounds().Dy()).To(Equal(200))

			box := image.Rect(10, 10, 60, 60)
			cropped := cropImage(rotated, box)
			Expect(cropped.Bounds().Dx()).To(Equal(50))
			Expect(cropped.Bounds().Dy()).To(Equal(50))
		})

		It("produces expected dimensions for free-angle rotation + crop", func() {
			// Start with 200x100, rotate 30 degrees, then crop a 50x50 region from center.
			src := newTestImage(200, 100)
			rotated := rotateImage(src, 30)

			// Verify expanded canvas.
			rad := 30.0 * math.Pi / 180.0
			expectedW := int(math.Ceil(200*math.Cos(rad) + 100*math.Sin(rad)))
			expectedH := int(math.Ceil(200*math.Sin(rad) + 100*math.Cos(rad)))
			Expect(rotated.Bounds().Dx()).To(Equal(expectedW))
			Expect(rotated.Bounds().Dy()).To(Equal(expectedH))

			// Crop a 50x50 region from center.
			cx := expectedW / 2
			cy := expectedH / 2
			box := image.Rect(cx-25, cy-25, cx+25, cy+25)
			cropped := cropImage(rotated, box)
			Expect(cropped.Bounds().Dx()).To(Equal(50))
			Expect(cropped.Bounds().Dy()).To(Equal(50))
		})
	})
})
