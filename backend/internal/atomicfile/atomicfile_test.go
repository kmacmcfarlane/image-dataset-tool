package atomicfile_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/atomicfile"
)

func TestAtomicFile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AtomicFile Suite")
}

var _ = Describe("AtomicFile", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "atomicfile-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Write", func() {
		It("writes data atomically to the target path", func() {
			target := filepath.Join(tmpDir, "output.json")
			data := []byte(`{"hello": "world"}`)

			err := atomicfile.Write(target, data, 0644)
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal(data))
		})

		It("sets the correct file permissions", func() {
			target := filepath.Join(tmpDir, "perms.json")
			err := atomicfile.Write(target, []byte("test"), 0600)
			Expect(err).NotTo(HaveOccurred())

			info, err := os.Stat(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})

		It("overwrites existing file atomically", func() {
			target := filepath.Join(tmpDir, "existing.json")
			err := os.WriteFile(target, []byte("old content"), 0644)
			Expect(err).NotTo(HaveOccurred())

			newData := []byte("new content")
			err = atomicfile.Write(target, newData, 0644)
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal(newData))
		})

		It("does not leave temp files on success", func() {
			target := filepath.Join(tmpDir, "clean.json")
			err := atomicfile.Write(target, []byte("data"), 0644)
			Expect(err).NotTo(HaveOccurred())

			entries, err := os.ReadDir(tmpDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Name()).To(Equal("clean.json"))
		})

		It("fails if directory does not exist", func() {
			target := filepath.Join(tmpDir, "nonexistent", "file.json")
			err := atomicfile.Write(target, []byte("data"), 0644)
			Expect(err).To(HaveOccurred())
		})
	})
})
