package api_test

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/api"
)

var _ = Describe("MediaHandler", func() {
	var (
		tmpDir    string
		resolver  *api.FileMediaResolver
		handler   *api.MediaHandler
		sampleDir string
		sampleID  string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "media-test-*")
		Expect(err).NotTo(HaveOccurred())

		sampleID = "550e8400-e29b-41d4-a716-446655440000"
		sampleDir = filepath.Join(tmpDir, "projects", "p1", "subjects", "s1", "samples")
		Expect(os.MkdirAll(sampleDir, 0755)).To(Succeed())

		resolver = api.NewFileMediaResolver(tmpDir)
		handler = api.NewMediaHandler(resolver)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	createTestJPEG := func(path string) {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		for y := 0; y < 100; y++ {
			for x := 0; x < 100; x++ {
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			}
		}
		f, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		Expect(jpeg.Encode(f, img, nil)).To(Succeed())
	}

	createTestPNG := func(path string) {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		for y := 0; y < 100; y++ {
			for x := 0; x < 100; x++ {
				img.Set(x, y, color.RGBA{R: 0, G: 255, A: 255})
			}
		}
		f, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()
		Expect(png.Encode(f, img)).To(Succeed())
	}

	Describe("ServeImage", func() {
		It("serves a JPEG image with correct Content-Type", func() {
			imgPath := filepath.Join(sampleDir, sampleID+".jpg")
			createTestJPEG(imgPath)

			req := httptest.NewRequest("GET", "/v1/media/"+sampleID, nil)
			rec := httptest.NewRecorder()
			handler.ServeImage(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("image/jpeg"))
			Expect(rec.Body.Len()).To(BeNumerically(">", 0))
		})

		It("returns 404 for unknown sample", func() {
			req := httptest.NewRequest("GET", "/v1/media/00000000-0000-0000-0000-000000000000", nil)
			rec := httptest.NewRecorder()
			handler.ServeImage(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for sample ID with invalid characters", func() {
			req := httptest.NewRequest("GET", "/v1/media/test*glob[0]", nil)
			rec := httptest.NewRecorder()
			handler.ServeImage(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("ServeThumbnail", func() {
		It("generates a thumbnail when missing and serves it", func() {
			imgPath := filepath.Join(sampleDir, sampleID+".jpg")
			createTestJPEG(imgPath)

			thumbPath := filepath.Join(sampleDir, sampleID+".thumb.jpg")
			// Verify thumbnail doesn't exist yet.
			_, err := os.Stat(thumbPath)
			Expect(os.IsNotExist(err)).To(BeTrue())

			req := httptest.NewRequest("GET", "/v1/media/"+sampleID+"/thumb", nil)
			rec := httptest.NewRecorder()
			handler.ServeThumbnail(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("image/jpeg"))

			// Verify thumbnail was generated on disk.
			_, err = os.Stat(thumbPath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("serves existing thumbnail without regeneration", func() {
			imgPath := filepath.Join(sampleDir, sampleID+".jpg")
			createTestJPEG(imgPath)

			// Pre-create thumbnail.
			thumbPath := filepath.Join(sampleDir, sampleID+".thumb.jpg")
			createTestJPEG(thumbPath)
			originalInfo, err := os.Stat(thumbPath)
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest("GET", "/v1/media/"+sampleID+"/thumb", nil)
			rec := httptest.NewRecorder()
			handler.ServeThumbnail(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))

			// File should not have been modified (same mod time).
			newInfo, err := os.Stat(thumbPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(newInfo.ModTime()).To(Equal(originalInfo.ModTime()))
		})

		It("regenerates thumbnail from PNG source", func() {
			imgPath := filepath.Join(sampleDir, sampleID+".png")
			createTestPNG(imgPath)

			req := httptest.NewRequest("GET", "/v1/media/"+sampleID+"/thumb", nil)
			rec := httptest.NewRecorder()
			handler.ServeThumbnail(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("image/jpeg"))
		})

		It("returns 404 for unknown sample", func() {
			req := httptest.NewRequest("GET", "/v1/media/00000000-0000-0000-0000-000000000000/thumb", nil)
			rec := httptest.NewRecorder()
			handler.ServeThumbnail(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for sample ID with glob metacharacters", func() {
			req := httptest.NewRequest("GET", "/v1/media/test*glob[0]/thumb", nil)
			rec := httptest.NewRecorder()
			handler.ServeThumbnail(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
