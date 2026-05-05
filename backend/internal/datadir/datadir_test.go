package datadir_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/datadir"
)

func TestDatadir(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datadir Suite")
}

var _ = Describe("Datadir", func() {
	Describe("Resolve", func() {
		It("returns $DATA_DIR when set", func() {
			os.Setenv("DATA_DIR", "/custom/path")
			defer os.Unsetenv("DATA_DIR")

			Expect(datadir.Resolve()).To(Equal("/custom/path"))
		})

		It("returns default when $DATA_DIR is not set", func() {
			os.Unsetenv("DATA_DIR")
			result := datadir.Resolve()
			Expect(result).To(HaveSuffix("image-dataset-tool"))
		})
	})

	Describe("Bootstrap", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "datadir-test-*")
			Expect(err).NotTo(HaveOccurred())

			dataPath := filepath.Join(tmpDir, "data")
			os.Setenv("DATA_DIR", dataPath)
		})

		AfterEach(func() {
			os.Unsetenv("DATA_DIR")
			os.RemoveAll(tmpDir)
		})

		It("creates data dir and all required subdirectories", func() {
			dir, err := datadir.Bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(dir).To(Equal(filepath.Join(tmpDir, "data")))

			for _, sub := range datadir.RequiredSubdirs {
				subPath := filepath.Join(dir, sub)
				info, err := os.Stat(subPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.IsDir()).To(BeTrue())
			}
		})

		It("is idempotent - does not fail if dirs already exist", func() {
			_, err := datadir.Bootstrap()
			Expect(err).NotTo(HaveOccurred())

			_, err = datadir.Bootstrap()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("EnsureSecretKey", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "datadir-ensurekey-test-*")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("generates a valid secret.key when none exists", func() {
			err := datadir.EnsureSecretKey(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			keyPath := datadir.SecretKeyPath(tmpDir)
			info, err := os.Stat(keyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))

			data, err := os.ReadFile(keyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveLen(crypto.KeySize))
		})

		It("is idempotent — does not overwrite an existing key", func() {
			keyPath := datadir.SecretKeyPath(tmpDir)
			existing := make([]byte, crypto.KeySize)
			for i := range existing {
				existing[i] = 0xAB
			}
			Expect(os.WriteFile(keyPath, existing, 0600)).To(Succeed())

			Expect(datadir.EnsureSecretKey(tmpDir)).To(Succeed())

			data, err := os.ReadFile(keyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(Equal(existing))
		})

		It("generated key is loadable by crypto.LoadKey", func() {
			Expect(datadir.EnsureSecretKey(tmpDir)).To(Succeed())

			key, err := crypto.LoadKey(datadir.SecretKeyPath(tmpDir))
			Expect(err).NotTo(HaveOccurred())
			Expect(key).To(HaveLen(crypto.KeySize))
		})
	})

	Describe("SecretKeyPath", func() {
		It("returns the correct path", func() {
			Expect(datadir.SecretKeyPath("/data")).To(Equal("/data/secret.key"))
		})
	})

	Describe("DBPath", func() {
		It("returns the correct path", func() {
			Expect(datadir.DBPath("/data")).To(Equal("/data/db.sqlite"))
		})
	})
})
