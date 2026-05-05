package fileformat_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
)

func TestFileformat(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fileformat Suite")
}

var _ = Describe("Fileformat", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "fileformat-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("ProjectManifest", func() {
		It("writes and reads back with matching fields", func() {
			manifest := &fileformat.ProjectManifest{
				ID:          "550e8400-e29b-41d4-a716-446655440000",
				Slug:        "my-project",
				Name:        "My Project",
				Description: "A test project",
				CreatedAt:   "2026-05-01T12:00:00Z",
				UpdatedAt:   "2026-05-01T12:00:00Z",
			}

			path := filepath.Join(tmpDir, "manifest.json")
			err := fileformat.WriteProjectManifest(path, manifest)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := fileformat.ReadProjectManifest(path)
			Expect(err).NotTo(HaveOccurred())

			Expect(loaded.ID).To(Equal(manifest.ID))
			Expect(loaded.Slug).To(Equal(manifest.Slug))
			Expect(loaded.Name).To(Equal(manifest.Name))
			Expect(loaded.Description).To(Equal(manifest.Description))
			Expect(loaded.CreatedAt).To(Equal(manifest.CreatedAt))
			Expect(loaded.UpdatedAt).To(Equal(manifest.UpdatedAt))
		})

		It("uses atomic write (file exists after write)", func() {
			manifest := &fileformat.ProjectManifest{
				ID:   "test-id",
				Slug: "test",
				Name: "Test",
			}

			path := filepath.Join(tmpDir, "manifest.json")
			err := fileformat.WriteProjectManifest(path, manifest)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(path)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("SampleMetadata", func() {
		It("writes and reads back with all fields including nested captions", func() {
			sample := &fileformat.SampleMetadata{
				ID: "660e8400-e29b-41d4-a716-446655440001",
				Source: fileformat.SampleSource{
					Platform:   "instagram",
					PostID:     "ABC123",
					SlideIndex: 0,
					URL:        "https://example.com/image.jpg",
					TakenAt:    "2026-04-15T08:30:00Z",
				},
				File: fileformat.SampleFile{
					Path:   "660e8400-e29b-41d4-a716-446655440001.jpg",
					SHA256: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
					PHash:  "d4e6f8a0b2c4e6f8",
					Width:  1080,
					Height: 1350,
				},
				Status:      "kept",
				IsDuplicate: false,
				DuplicateOf: []string{},
				Edits: fileformat.SampleEdits{
					RotationDeg: 0.0,
					CropBox:     nil,
					AutoActions: []string{},
				},
				Captions: map[string]fileformat.Caption{
					"natural-claude": {
						StudyID:   "770e8400-e29b-41d4-a716-446655440002",
						Text:      "A woman standing on a beach at sunset...",
						CreatedAt: "2026-05-02T14:00:00Z",
					},
					"detailed-local": {
						StudyID:   "880e8400-e29b-41d4-a716-446655440003",
						Text:      "Detailed description of the scene",
						CreatedAt: "2026-05-02T15:00:00Z",
					},
				},
				FetchedAt: "2026-05-01T12:00:00Z",
				CreatedAt: "2026-05-01T12:00:00Z",
				UpdatedAt: "2026-05-02T15:00:00Z",
			}

			path := filepath.Join(tmpDir, "sample.json")
			err := fileformat.WriteSampleMetadata(path, sample)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := fileformat.ReadSampleMetadata(path)
			Expect(err).NotTo(HaveOccurred())

			// Verify top-level fields
			Expect(loaded.ID).To(Equal(sample.ID))
			Expect(loaded.Status).To(Equal("kept"))
			Expect(loaded.IsDuplicate).To(BeFalse())
			Expect(loaded.DuplicateOf).To(BeEmpty())
			Expect(loaded.FetchedAt).To(Equal(sample.FetchedAt))
			Expect(loaded.CreatedAt).To(Equal(sample.CreatedAt))
			Expect(loaded.UpdatedAt).To(Equal(sample.UpdatedAt))

			// Verify source
			Expect(loaded.Source.Platform).To(Equal("instagram"))
			Expect(loaded.Source.PostID).To(Equal("ABC123"))
			Expect(loaded.Source.SlideIndex).To(Equal(0))
			Expect(loaded.Source.URL).To(Equal("https://example.com/image.jpg"))
			Expect(loaded.Source.TakenAt).To(Equal("2026-04-15T08:30:00Z"))

			// Verify file
			Expect(loaded.File.Path).To(Equal(sample.File.Path))
			Expect(loaded.File.SHA256).To(Equal(sample.File.SHA256))
			Expect(loaded.File.PHash).To(Equal(sample.File.PHash))
			Expect(loaded.File.Width).To(Equal(1080))
			Expect(loaded.File.Height).To(Equal(1350))

			// Verify edits
			Expect(loaded.Edits.RotationDeg).To(Equal(0.0))
			Expect(loaded.Edits.CropBox).To(BeNil())
			Expect(loaded.Edits.AutoActions).To(BeEmpty())

			// Verify nested captions
			Expect(loaded.Captions).To(HaveLen(2))
			Expect(loaded.Captions["natural-claude"].StudyID).To(Equal("770e8400-e29b-41d4-a716-446655440002"))
			Expect(loaded.Captions["natural-claude"].Text).To(Equal("A woman standing on a beach at sunset..."))
			Expect(loaded.Captions["natural-claude"].CreatedAt).To(Equal("2026-05-02T14:00:00Z"))
			Expect(loaded.Captions["detailed-local"].StudyID).To(Equal("880e8400-e29b-41d4-a716-446655440003"))
			Expect(loaded.Captions["detailed-local"].Text).To(Equal("Detailed description of the scene"))
		})

		It("handles sample with crop box", func() {
			sample := &fileformat.SampleMetadata{
				ID: "test-crop",
				Source: fileformat.SampleSource{
					Platform: "instagram",
					PostID:   "XYZ",
				},
				File: fileformat.SampleFile{
					Path: "test.jpg",
				},
				Status:      "pending",
				DuplicateOf: []string{},
				Edits: fileformat.SampleEdits{
					RotationDeg: 90.0,
					CropBox: &fileformat.CropBox{
						X: 10, Y: 20, W: 100, H: 200,
					},
					AutoActions: []string{"auto-level"},
				},
				Captions: map[string]fileformat.Caption{},
			}

			path := filepath.Join(tmpDir, "crop-sample.json")
			err := fileformat.WriteSampleMetadata(path, sample)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := fileformat.ReadSampleMetadata(path)
			Expect(err).NotTo(HaveOccurred())

			Expect(loaded.Edits.RotationDeg).To(Equal(90.0))
			Expect(loaded.Edits.CropBox).NotTo(BeNil())
			Expect(loaded.Edits.CropBox.X).To(Equal(10.0))
			Expect(loaded.Edits.CropBox.Y).To(Equal(20.0))
			Expect(loaded.Edits.CropBox.W).To(Equal(100.0))
			Expect(loaded.Edits.CropBox.H).To(Equal(200.0))
			Expect(loaded.Edits.AutoActions).To(ConsistOf("auto-level"))
		})
	})
})
