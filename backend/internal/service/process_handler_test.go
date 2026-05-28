package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/fileformat"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/pipeline"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service"
)

// --- Fake stores for testing ---

type fakeSampleStore struct {
	samples      []*model.Sample
	sha256Dups   map[string][]string // key: "subjectID:sha256" -> ids
	pHashDups    map[string][]string // key: "subjectID:phash" -> ids
	projectIDs   map[string]string   // subjectID -> projectID
	insertCalled bool
	lastInserted *model.Sample
}

func newFakeSampleStore() *fakeSampleStore {
	return &fakeSampleStore{
		sha256Dups: make(map[string][]string),
		pHashDups:  make(map[string][]string),
		projectIDs: map[string]string{"sub1": "proj1"},
	}
}

func (f *fakeSampleStore) InsertOrIgnore(sample *model.Sample) (bool, error) {
	f.insertCalled = true
	f.lastInserted = sample
	f.samples = append(f.samples, sample)
	return true, nil
}

func (f *fakeSampleStore) FindDuplicatesBySHA256(subjectID, sha string) ([]string, error) {
	return f.sha256Dups[subjectID+":"+sha], nil
}

func (f *fakeSampleStore) FindDuplicatesByPHash(subjectID, targetPHash string, threshold int) ([]string, error) {
	return f.pHashDups[subjectID+":"+targetPHash], nil
}

func (f *fakeSampleStore) GetProjectIDForSubject(subjectID string) (string, error) {
	return f.projectIDs[subjectID], nil
}

type fakeSubjectStore struct{}

func (f *fakeSubjectStore) Get(id string) (*model.Subject, error) {
	return &model.Subject{ID: id, ProjectID: "proj1", Slug: "test-subject", Name: "Test Subject"}, nil
}

type fakeProjectStore struct{}

func (f *fakeProjectStore) Get(id string) (*model.Project, error) {
	return &model.Project{ID: id, Slug: "test-project", Name: "Test Project"}, nil
}

type fakeJobMsgStore struct {
	messages []*model.JobMessage
}

func (f *fakeJobMsgStore) CreateJobMessage(msg *model.JobMessage) error {
	f.messages = append(f.messages, msg)
	return nil
}

type fakeEventSink struct {
	events []pipeline.SSEEvent
}

func (f *fakeEventSink) Emit(event pipeline.SSEEvent) {
	f.events = append(f.events, event)
}

// --- Helpers ---

func createTestJPEG(path string, width, height int) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(jpeg.Encode(f, img, &jpeg.Options{Quality: 80})).To(Succeed())
	f.Close()
}

func createTestPNG(path string, width, height int) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 50, A: 255})
		}
	}
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	Expect(png.Encode(f, img)).To(Succeed())
	f.Close()
}

func testSHA256(path string) string {
	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

var _ = Describe("ProcessHandler", func() {
	var (
		dataDir     string
		sampleStore *fakeSampleStore
		msgStore    *fakeJobMsgStore
		eventSink   *fakeEventSink
		handler     *service.ProcessHandler
	)

	BeforeEach(func() {
		var err error
		dataDir, err = os.MkdirTemp("", "process-handler-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Create required directories.
		Expect(os.MkdirAll(filepath.Join(dataDir, "tmp", "job1"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(dataDir, "projects", "test-project", "subjects", "test-subject", "samples"), 0755)).To(Succeed())

		sampleStore = newFakeSampleStore()
		msgStore = &fakeJobMsgStore{}
		eventSink = &fakeEventSink{}

		handler = service.NewProcessHandler(
			sampleStore, &fakeSubjectStore{}, &fakeProjectStore{},
			msgStore, eventSink, dataDir,
			service.DefaultProcessConfig(),
		)
	})

	AfterEach(func() {
		os.RemoveAll(dataDir)
	})

	makeEnvelope := func(payload service.ProcessPayload) *pipeline.Envelope {
		data, err := json.Marshal(payload)
		Expect(err).NotTo(HaveOccurred())
		return &pipeline.Envelope{
			JobID:   "job1",
			TraceID: "trace1",
			Payload: data,
		}
	}

	Describe("Handle", func() {
		Context("happy path with JPEG", func() {
			It("processes image, creates sample, manifest, and thumbnail", func() {
				tmpFile := filepath.Join(dataDir, "tmp", "job1", "test.jpg")
				createTestJPEG(tmpFile, 1024, 768)

				payload := service.ProcessPayload{
					SubjectID:      "sub1",
					SourcePlatform: "instagram",
					SourcePostID:   "post123",
					SlideIndex:     0,
					SourceURL:      "https://example.com/img.jpg",
					TakenAt:        "2026-01-01T00:00:00Z",
					FetchedAt:      "2026-01-02T00:00:00Z",
					TmpFilePath:    tmpFile,
				}

				err := handler.Handle(context.Background(), makeEnvelope(payload))
				Expect(err).NotTo(HaveOccurred())

				// Sample inserted.
				Expect(sampleStore.insertCalled).To(BeTrue())
				inserted := sampleStore.lastInserted
				Expect(inserted.SubjectID).To(Equal("sub1"))
				Expect(inserted.SourcePostID).To(Equal("post123"))
				Expect(inserted.Status).To(Equal("pending"))
				Expect(inserted.SHA256).NotTo(BeEmpty())
				Expect(inserted.PHash).NotTo(BeEmpty())
				Expect(inserted.Width).To(Equal(1024))
				Expect(inserted.Height).To(Equal(768))
				Expect(inserted.IsDuplicate).To(BeFalse())

				// Verify files were created in final directory.
				samplesDir := filepath.Join(dataDir, "projects", "test-project", "subjects", "test-subject", "samples")
				entries, err := os.ReadDir(samplesDir)
				Expect(err).NotTo(HaveOccurred())
				// 3 files: image, thumbnail, manifest.
				Expect(entries).To(HaveLen(3))

				// Find the manifest and verify.
				var manifestPath string
				for _, e := range entries {
					if filepath.Ext(e.Name()) == ".json" {
						manifestPath = filepath.Join(samplesDir, e.Name())
					}
				}
				Expect(manifestPath).NotTo(BeEmpty())

				manifest, err := fileformat.ReadSampleMetadata(manifestPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(manifest.Source.PostID).To(Equal("post123"))
				Expect(manifest.File.SHA256).NotTo(BeEmpty())
				Expect(manifest.File.PHash).NotTo(BeEmpty())
				Expect(manifest.File.Width).To(Equal(1024))
				Expect(manifest.File.Height).To(Equal(768))
				Expect(manifest.Status).To(Equal("pending"))
				Expect(manifest.IsDuplicate).To(BeFalse())

				// SSE event emitted.
				Expect(eventSink.events).To(HaveLen(1))
				Expect(eventSink.events[0].Type).To(Equal("sample.new"))
				Expect(eventSink.events[0].Data["subject_id"]).To(Equal("sub1"))
				Expect(eventSink.events[0].Data["status"]).To(Equal("pending"))
				Expect(eventSink.events[0].Data["is_duplicate"]).To(BeFalse())

				// Original tmp file moved (no longer exists).
				_, err = os.Stat(tmpFile)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		Context("with PNG format", func() {
			It("processes PNG images successfully", func() {
				tmpFile := filepath.Join(dataDir, "tmp", "job1", "test.png")
				createTestPNG(tmpFile, 640, 480)

				payload := service.ProcessPayload{
					SubjectID:      "sub1",
					SourcePlatform: "instagram",
					SourcePostID:   "post-png",
					SlideIndex:     0,
					FetchedAt:      "2026-01-02T00:00:00Z",
					TmpFilePath:    tmpFile,
				}

				err := handler.Handle(context.Background(), makeEnvelope(payload))
				Expect(err).NotTo(HaveOccurred())
				Expect(sampleStore.insertCalled).To(BeTrue())
				Expect(sampleStore.lastInserted.Width).To(Equal(640))
				Expect(sampleStore.lastInserted.Height).To(Equal(480))
			})
		})

		Context("file size exceeds limit", func() {
			It("skips the file and logs a job message", func() {
				smallHandler := service.NewProcessHandler(
					sampleStore, &fakeSubjectStore{}, &fakeProjectStore{},
					msgStore, eventSink, dataDir,
					service.ProcessConfig{
						MaxFileSizeMB:   0, // 0 MB = skip everything
						ThumbnailMaxDim: 768,
						PHashThreshold:  6,
						AcceptedFormats: []string{"jpeg", "png"},
					},
				)

				tmpFile := filepath.Join(dataDir, "tmp", "job1", "big.jpg")
				createTestJPEG(tmpFile, 100, 100)

				payload := service.ProcessPayload{
					SubjectID:      "sub1",
					SourcePlatform: "instagram",
					SourcePostID:   "post-big",
					SlideIndex:     0,
					FetchedAt:      "2026-01-02T00:00:00Z",
					TmpFilePath:    tmpFile,
				}

				err := smallHandler.Handle(context.Background(), makeEnvelope(payload))
				Expect(err).NotTo(HaveOccurred()) // Skipped, not errored.

				Expect(sampleStore.insertCalled).To(BeFalse())
				Expect(msgStore.messages).To(HaveLen(1))
				Expect(msgStore.messages[0].Level).To(Equal("error"))
				Expect(msgStore.messages[0].Message).To(ContainSubstring("exceeds max size"))
			})
		})

		Context("unsupported format", func() {
			It("skips non-image files and logs a job message", func() {
				tmpFile := filepath.Join(dataDir, "tmp", "job1", "bad.txt")
				Expect(os.WriteFile(tmpFile, []byte("not an image"), 0644)).To(Succeed())

				payload := service.ProcessPayload{
					SubjectID:      "sub1",
					SourcePlatform: "instagram",
					SourcePostID:   "post-bad",
					SlideIndex:     0,
					FetchedAt:      "2026-01-02T00:00:00Z",
					TmpFilePath:    tmpFile,
				}

				err := handler.Handle(context.Background(), makeEnvelope(payload))
				Expect(err).NotTo(HaveOccurred()) // Skipped, not errored.
				Expect(sampleStore.insertCalled).To(BeFalse())
				Expect(msgStore.messages).To(HaveLen(1))
				Expect(msgStore.messages[0].Level).To(Equal("error"))
			})
		})

		Context("sha256 duplicate detection", func() {
			It("flags sample as duplicate when sha256 matches existing", func() {
				tmpFile := filepath.Join(dataDir, "tmp", "job1", "dup.jpg")
				createTestJPEG(tmpFile, 200, 200)

				// Pre-compute sha256 to set up the fake store.
				hash := testSHA256(tmpFile)
				sampleStore.sha256Dups["sub1:"+hash] = []string{"existing-sample-id"}

				payload := service.ProcessPayload{
					SubjectID:      "sub1",
					SourcePlatform: "instagram",
					SourcePostID:   "post-dup",
					SlideIndex:     0,
					FetchedAt:      "2026-01-02T00:00:00Z",
					TmpFilePath:    tmpFile,
				}

				err := handler.Handle(context.Background(), makeEnvelope(payload))
				Expect(err).NotTo(HaveOccurred())

				Expect(sampleStore.lastInserted.IsDuplicate).To(BeTrue())
				Expect(sampleStore.lastInserted.DuplicateOf).To(ContainSubstring("existing-sample-id"))

				// Warning job message for duplicate.
				hasWarning := false
				for _, m := range msgStore.messages {
					if m.Level == "warning" {
						hasWarning = true
					}
				}
				Expect(hasWarning).To(BeTrue())
			})
		})

		Context("thumbnail generation", func() {
			It("generates a JPEG thumbnail with correct dimensions", func() {
				tmpFile := filepath.Join(dataDir, "tmp", "job1", "thumb-test.jpg")
				createTestJPEG(tmpFile, 2000, 1500)

				payload := service.ProcessPayload{
					SubjectID:      "sub1",
					SourcePlatform: "instagram",
					SourcePostID:   "post-thumb",
					SlideIndex:     0,
					FetchedAt:      "2026-01-02T00:00:00Z",
					TmpFilePath:    tmpFile,
				}

				err := handler.Handle(context.Background(), makeEnvelope(payload))
				Expect(err).NotTo(HaveOccurred())

				// Find the thumbnail file.
				samplesDir := filepath.Join(dataDir, "projects", "test-project", "subjects", "test-subject", "samples")
				entries, err := os.ReadDir(samplesDir)
				Expect(err).NotTo(HaveOccurred())

				var thumbPath string
				for _, e := range entries {
					name := e.Name()
					if len(name) > 10 && name[len(name)-10:] == "_thumb.jpg" {
						thumbPath = filepath.Join(samplesDir, name)
					}
				}
				Expect(thumbPath).NotTo(BeEmpty())

				// Decode and verify dimensions.
				tf, err := os.Open(thumbPath)
				Expect(err).NotTo(HaveOccurred())
				defer tf.Close()

				thumbImg, err := jpeg.Decode(tf)
				Expect(err).NotTo(HaveOccurred())
				bounds := thumbImg.Bounds()
				// Original 2000x1500, shortest edge 1500 > 768.
				// Scale: height=768, width=2000*768/1500=1024.
				Expect(bounds.Dx()).To(Equal(1024))
				Expect(bounds.Dy()).To(Equal(768))
			})
		})

		Context("small image (no resize needed)", func() {
			It("keeps original dimensions for thumbnail", func() {
				tmpFile := filepath.Join(dataDir, "tmp", "job1", "small.jpg")
				createTestJPEG(tmpFile, 200, 150)

				payload := service.ProcessPayload{
					SubjectID:      "sub1",
					SourcePlatform: "instagram",
					SourcePostID:   "post-small",
					SlideIndex:     0,
					FetchedAt:      "2026-01-02T00:00:00Z",
					TmpFilePath:    tmpFile,
				}

				err := handler.Handle(context.Background(), makeEnvelope(payload))
				Expect(err).NotTo(HaveOccurred())

				// Find thumbnail.
				samplesDir := filepath.Join(dataDir, "projects", "test-project", "subjects", "test-subject", "samples")
				entries, err := os.ReadDir(samplesDir)
				Expect(err).NotTo(HaveOccurred())

				var thumbPath string
				for _, e := range entries {
					name := e.Name()
					if len(name) > 10 && name[len(name)-10:] == "_thumb.jpg" {
						thumbPath = filepath.Join(samplesDir, name)
					}
				}
				Expect(thumbPath).NotTo(BeEmpty())

				tf, err := os.Open(thumbPath)
				Expect(err).NotTo(HaveOccurred())
				defer tf.Close()

				thumbImg, err := jpeg.Decode(tf)
				Expect(err).NotTo(HaveOccurred())
				bounds := thumbImg.Bounds()
				// Both dimensions smaller than 768, no resize.
				Expect(bounds.Dx()).To(Equal(200))
				Expect(bounds.Dy()).To(Equal(150))
			})
		})
	})
})
