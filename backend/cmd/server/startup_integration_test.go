package main_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/datadir"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

func TestStartup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Startup Integration Suite")
}

// writeKey writes key bytes to path with the given mode.
func writeKey(path string, key []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, key, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

var _ = Describe("Startup initialization", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "startup-test-*")
		Expect(err).NotTo(HaveOccurred())
		// Point DATA_DIR at our temp dir so datadir.Bootstrap() uses it.
		DeferCleanup(func() {
			os.Unsetenv("DATA_DIR")
			os.RemoveAll(tmpDir)
		})
		os.Setenv("DATA_DIR", tmpDir)
	})

	Describe("datadir.Bootstrap", func() {
		It("creates required subdirectories", func() {
			dir, err := datadir.Bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(dir).To(Equal(tmpDir))

			for _, sub := range datadir.RequiredSubdirs {
				info, err := os.Stat(filepath.Join(dir, sub))
				Expect(err).NotTo(HaveOccurred(), "subdir %s should exist", sub)
				Expect(info.IsDir()).To(BeTrue())
			}
		})

		It("is idempotent when dirs already exist", func() {
			_, err := datadir.Bootstrap()
			Expect(err).NotTo(HaveOccurred())
			_, err = datadir.Bootstrap()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("full startup sequence", func() {
		var validKey []byte

		BeforeEach(func() {
			validKey = make([]byte, crypto.KeySize)
			for i := range validKey {
				validKey[i] = byte(i + 1)
			}
		})

		Context("with a valid key (32 bytes, mode 0600)", func() {
			It("succeeds and migrates the database", func() {
				dir, err := datadir.Bootstrap()
				Expect(err).NotTo(HaveOccurred())

				keyPath := datadir.SecretKeyPath(dir)
				Expect(writeKey(keyPath, validKey, 0600)).To(Succeed())

				key, err := crypto.LoadKey(keyPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(key).To(HaveLen(crypto.KeySize))

				db, err := store.OpenDB(datadir.DBPath(dir))
				Expect(err).NotTo(HaveOccurred())
				defer db.Close()

				Expect(store.Migrate(db)).To(Succeed())

				// Verify a representative table exists.
				var name string
				err = db.QueryRow(
					"SELECT name FROM sqlite_master WHERE type='table' AND name='projects'",
				).Scan(&name)
				Expect(err).NotTo(HaveOccurred())
				Expect(name).To(Equal("projects"))
			})
		})

		Context("with a missing secret.key", func() {
			It("returns ErrKeyMissing", func() {
				dir, err := datadir.Bootstrap()
				Expect(err).NotTo(HaveOccurred())

				keyPath := datadir.SecretKeyPath(dir)
				// File does not exist.
				_, err = crypto.LoadKey(keyPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("secret.key file not found"))
			})
		})

		Context("with secret.key mode 0644 (world-readable)", func() {
			It("returns ErrKeyPermissions", func() {
				dir, err := datadir.Bootstrap()
				Expect(err).NotTo(HaveOccurred())

				keyPath := datadir.SecretKeyPath(dir)
				Expect(writeKey(keyPath, validKey, 0644)).To(Succeed())

				_, err = crypto.LoadKey(keyPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("secret.key must have mode 0600"))
			})
		})
	})
})
