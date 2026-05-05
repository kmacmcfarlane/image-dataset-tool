package store_test

import (
	"database/sql"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// testEncKey is a fixed 32-byte AES-256 key for tests.
var testEncKey = []byte("01234567890123456789012345678901")

func openMigratedDB(dir string) (*sql.DB, string) {
	path := filepath.Join(dir, "secrets-test.sqlite")
	db, err := store.OpenDB(path)
	Expect(err).NotTo(HaveOccurred())
	err = store.Migrate(db)
	Expect(err).NotTo(HaveOccurred())
	return db, path
}

var _ = Describe("SecretsStore", func() {
	var (
		tmpDir string
		db     *sql.DB
		s      *store.SecretsStore
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "secrets-store-test-*")
		Expect(err).NotTo(HaveOccurred())

		db, _ = openMigratedDB(tmpDir)
		s = store.NewSecretsStore(db, testEncKey)
	})

	AfterEach(func() {
		db.Close()
		os.RemoveAll(tmpDir)
	})

	Describe("List", func() {
		It("returns empty list when no secrets exist", func() {
			entries, err := s.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("returns keys in alphabetical order", func() {
			Expect(s.Set("zzz_key", "val1")).To(Succeed())
			Expect(s.Set("aaa_key", "val2")).To(Succeed())
			Expect(s.Set("mmm_key", "val3")).To(Succeed())

			entries, err := s.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(3))
			Expect(entries[0].Key).To(Equal("aaa_key"))
			Expect(entries[1].Key).To(Equal("mmm_key"))
			Expect(entries[2].Key).To(Equal("zzz_key"))
		})

		It("returns timestamps with each entry", func() {
			Expect(s.Set("test_key", "value")).To(Succeed())

			entries, err := s.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].CreatedAt).NotTo(BeEmpty())
			Expect(entries[0].UpdatedAt).NotTo(BeEmpty())
		})
	})

	Describe("Set", func() {
		It("creates a new secret", func() {
			Expect(s.Set("my_api_key", "super-secret")).To(Succeed())

			entries, err := s.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
			Expect(entries[0].Key).To(Equal("my_api_key"))
		})

		It("updates an existing secret", func() {
			Expect(s.Set("api_key", "original")).To(Succeed())
			Expect(s.Set("api_key", "updated")).To(Succeed())

			entries, err := s.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))

			// Verify the value was updated by decrypting
			val, err := s.Get("api_key")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("updated"))
		})

		It("stores values encrypted (value is not stored in plaintext)", func() {
			plaintext := "my-secret-api-key-value"
			Expect(s.Set("enc_test", plaintext)).To(Succeed())

			// Directly inspect the raw DB row
			var raw []byte
			err := db.QueryRow("SELECT value_encrypted FROM secrets WHERE key = 'enc_test'").Scan(&raw)
			Expect(err).NotTo(HaveOccurred())

			// Encrypted value should not contain the plaintext
			Expect(string(raw)).NotTo(ContainSubstring(plaintext))
		})
	})

	Describe("Get", func() {
		It("retrieves and decrypts a stored secret", func() {
			Expect(s.Set("fetch_key", "decrypted-value")).To(Succeed())

			val, err := s.Get("fetch_key")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal("decrypted-value"))
		})

		It("returns sql.ErrNoRows for a missing key", func() {
			_, err := s.Get("nonexistent")
			Expect(err).To(HaveOccurred())
		})

		It("round-trips arbitrary byte values", func() {
			original := "value with\nnewlines\tand\x00null bytes"
			Expect(s.Set("rt_key", original)).To(Succeed())

			val, err := s.Get("rt_key")
			Expect(err).NotTo(HaveOccurred())
			Expect(val).To(Equal(original))
		})
	})

	Describe("Delete", func() {
		It("removes a secret", func() {
			Expect(s.Set("del_key", "val")).To(Succeed())
			Expect(s.Delete("del_key")).To(Succeed())

			entries, err := s.List()
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("is idempotent for non-existent keys", func() {
			Expect(s.Delete("nonexistent")).To(Succeed())
		})
	})
})
