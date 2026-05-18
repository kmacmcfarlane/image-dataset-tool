package export_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/export"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// createTestJPEG writes a solid-color JPEG of the given dimensions to path.
func createTestJPEG(path string, w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	err = jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
	Expect(err).NotTo(HaveOccurred())
}

// createTestPNG writes a solid-color PNG of the given dimensions to path.
func createTestPNG(path string, w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 50, G: 100, B: 150, A: 255})
		}
	}
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	err = png.Encode(f, img)
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("EditMaterializer", func() {
	var (
		dataDir     string
		materializer *export.EditMaterializer
	)

	BeforeEach(func() {
		dataDir = GinkgoT().TempDir()
		materializer = export.NewEditMaterializer(dataDir)
	})

	Describe("Materialize", func() {
		Context("no edits (raw copy)", func() {
			It("returns raw bytes without re-encoding for nil edit", func() {
				imgPath := filepath.Join(dataDir, "sample.jpg")
				createTestJPEG(imgPath, 200, 100)

				rawData, err := os.ReadFile(imgPath)
				Expect(err).NotTo(HaveOccurred())

				sample := &model.Sample{
					ID:       "test-sample",
					FilePath: "sample.jpg",
					Width:    200,
					Height:   100,
				}

				result, err := materializer.Materialize(sample, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Format).To(Equal("jpeg"))
				Expect(result.Width).To(Equal(200))
				Expect(result.Height).To(Equal(100))
				// Raw bytes should be identical (no re-encoding).
				Expect(result.Data).To(Equal(rawData))
			})

			It("returns raw bytes for empty edit (zero rotation, no crop)", func() {
				imgPath := filepath.Join(dataDir, "sample.jpg")
				createTestJPEG(imgPath, 150, 150)

				rawData, err := os.ReadFile(imgPath)
				Expect(err).NotTo(HaveOccurred())

				sample := &model.Sample{
					ID:       "test-sample",
					FilePath: "sample.jpg",
					Width:    150,
					Height:   150,
				}
				edit := &model.Edit{
					SampleID:    "test-sample",
					RotationDeg: 0,
					CropBox:     nil,
				}

				result, err := materializer.Materialize(sample, edit)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Data).To(Equal(rawData))
			})
		})

		Context("rotation only", func() {
			It("applies 90-degree rotation and verifies output dimensions", func() {
				imgPath := filepath.Join(dataDir, "sample.jpg")
				createTestJPEG(imgPath, 200, 100)

				sample := &model.Sample{
					ID:       "test-sample",
					FilePath: "sample.jpg",
					Width:    200,
					Height:   100,
				}
				edit := &model.Edit{
					SampleID:    "test-sample",
					RotationDeg: 90,
				}

				result, err := materializer.Materialize(sample, edit)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Width).To(Equal(100))
				Expect(result.Height).To(Equal(200))
				Expect(result.Format).To(Equal("jpeg"))

				// Verify the output is a valid JPEG.
				decoded, err := jpeg.Decode(bytes.NewReader(result.Data))
				Expect(err).NotTo(HaveOccurred())
				Expect(decoded.Bounds().Dx()).To(Equal(100))
				Expect(decoded.Bounds().Dy()).To(Equal(200))
			})

			It("applies free-angle rotation (45 degrees)", func() {
				imgPath := filepath.Join(dataDir, "sample.png")
				createTestPNG(imgPath, 100, 100)

				sample := &model.Sample{
					ID:       "test-sample",
					FilePath: "sample.png",
					Width:    100,
					Height:   100,
				}
				edit := &model.Edit{
					SampleID:    "test-sample",
					RotationDeg: 45,
				}

				result, err := materializer.Materialize(sample, edit)
				Expect(err).NotTo(HaveOccurred())
				// 100x100 rotated 45 degrees → ~142x142.
				Expect(result.Width).To(BeNumerically(">=", 141))
				Expect(result.Height).To(BeNumerically(">=", 141))
				Expect(result.Format).To(Equal("png"))
			})
		})

		Context("crop only", func() {
			It("applies crop and verifies output dimensions", func() {
				imgPath := filepath.Join(dataDir, "sample.jpg")
				createTestJPEG(imgPath, 200, 150)

				sample := &model.Sample{
					ID:       "test-sample",
					FilePath: "sample.jpg",
					Width:    200,
					Height:   150,
				}
				edit := &model.Edit{
					SampleID: "test-sample",
					CropBox: &model.CropBox{
						X: 10,
						Y: 20,
						W: 100,
						H: 80,
					},
				}

				result, err := materializer.Materialize(sample, edit)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Width).To(Equal(100))
				Expect(result.Height).To(Equal(80))
			})
		})

		Context("rotation + crop combined", func() {
			It("applies rotation then crop in correct order", func() {
				// AC: Known rotation + crop → verify output dimensions.
				// 200x100 → rotate 90 → 100x200 → crop (10,10,60,60) → 50x50.
				imgPath := filepath.Join(dataDir, "sample.jpg")
				createTestJPEG(imgPath, 200, 100)

				sample := &model.Sample{
					ID:       "test-sample",
					FilePath: "sample.jpg",
					Width:    200,
					Height:   100,
				}
				edit := &model.Edit{
					SampleID:    "test-sample",
					RotationDeg: 90,
					CropBox: &model.CropBox{
						X: 10,
						Y: 10,
						W: 50,
						H: 50,
					},
				}

				result, err := materializer.Materialize(sample, edit)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Width).To(Equal(50))
				Expect(result.Height).To(Equal(50))

				// Verify the re-encoded output.
				decoded, err := jpeg.Decode(bytes.NewReader(result.Data))
				Expect(err).NotTo(HaveOccurred())
				Expect(decoded.Bounds().Dx()).To(Equal(50))
				Expect(decoded.Bounds().Dy()).To(Equal(50))
			})

			It("applies free-angle rotation then crop", func() {
				// AC: Free-angle rotation support.
				// 200x100 → rotate 30 → expanded canvas → crop center 50x50.
				imgPath := filepath.Join(dataDir, "sample.jpg")
				createTestJPEG(imgPath, 200, 100)

				sample := &model.Sample{
					ID:       "test-sample",
					FilePath: "sample.jpg",
					Width:    200,
					Height:   100,
				}
				edit := &model.Edit{
					SampleID:    "test-sample",
					RotationDeg: 30,
					CropBox: &model.CropBox{
						X: 50,
						Y: 30,
						W: 80,
						H: 60,
					},
				}

				result, err := materializer.Materialize(sample, edit)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Width).To(Equal(80))
				Expect(result.Height).To(Equal(60))
			})
		})

		Context("error cases", func() {
			It("returns error for missing file", func() {
				sample := &model.Sample{
					ID:       "missing",
					FilePath: "nonexistent.jpg",
					Width:    100,
					Height:   100,
				}
				edit := &model.Edit{
					SampleID:    "missing",
					RotationDeg: 90,
				}

				_, err := materializer.Materialize(sample, edit)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("open image"))
			})

			It("returns error for corrupt image file", func() {
				imgPath := filepath.Join(dataDir, "corrupt.jpg")
				err := os.WriteFile(imgPath, []byte("not an image"), 0644)
				Expect(err).NotTo(HaveOccurred())

				sample := &model.Sample{
					ID:       "corrupt",
					FilePath: "corrupt.jpg",
					Width:    100,
					Height:   100,
				}
				edit := &model.Edit{
					SampleID:    "corrupt",
					RotationDeg: 90,
				}

				_, err = materializer.Materialize(sample, edit)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("decode image"))
			})
		})
	})
})
