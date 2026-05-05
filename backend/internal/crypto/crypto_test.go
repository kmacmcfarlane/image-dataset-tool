package crypto_test

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/crypto"
)

func TestCrypto(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Crypto Suite")
}

var _ = Describe("Crypto", func() {
	var testKey []byte

	BeforeEach(func() {
		testKey = make([]byte, crypto.KeySize)
		_, err := rand.Read(testKey)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Encrypt/Decrypt round-trip", func() {
		It("encrypts and decrypts plaintext correctly", func() {
			plaintext := []byte("hello, world! this is secret data")

			ciphertext, err := crypto.Encrypt(testKey, plaintext)
			Expect(err).NotTo(HaveOccurred())
			Expect(ciphertext).NotTo(Equal(plaintext))

			decrypted, err := crypto.Decrypt(testKey, ciphertext)
			Expect(err).NotTo(HaveOccurred())
			Expect(decrypted).To(Equal(plaintext))
		})

		It("handles empty plaintext", func() {
			plaintext := []byte("")

			ciphertext, err := crypto.Encrypt(testKey, plaintext)
			Expect(err).NotTo(HaveOccurred())

			decrypted, err := crypto.Decrypt(testKey, ciphertext)
			Expect(err).NotTo(HaveOccurred())
			Expect(decrypted).To(Equal(plaintext))
		})

		It("produces different ciphertext for same plaintext (random nonce)", func() {
			plaintext := []byte("same input")

			ct1, err := crypto.Encrypt(testKey, plaintext)
			Expect(err).NotTo(HaveOccurred())

			ct2, err := crypto.Encrypt(testKey, plaintext)
			Expect(err).NotTo(HaveOccurred())

			Expect(ct1).NotTo(Equal(ct2))
		})
	})

	Describe("Decrypt with wrong key", func() {
		It("fails with ErrDecryptionFailed", func() {
			plaintext := []byte("sensitive data")

			ciphertext, err := crypto.Encrypt(testKey, plaintext)
			Expect(err).NotTo(HaveOccurred())

			wrongKey := make([]byte, crypto.KeySize)
			_, err = rand.Read(wrongKey)
			Expect(err).NotTo(HaveOccurred())

			_, err = crypto.Decrypt(wrongKey, ciphertext)
			Expect(err).To(MatchError(crypto.ErrDecryptionFailed))
		})
	})

	Describe("Decrypt with corrupted ciphertext", func() {
		It("fails with ErrDecryptionFailed for truncated data", func() {
			_, err := crypto.Decrypt(testKey, []byte("short"))
			Expect(err).To(MatchError(crypto.ErrDecryptionFailed))
		})
	})

	Describe("GenerateKey", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "crypto-genkey-test-*")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("creates a 32-byte key file with mode 0600", func() {
			keyPath := filepath.Join(tmpDir, "secret.key")
			err := crypto.GenerateKey(keyPath)
			Expect(err).NotTo(HaveOccurred())

			info, err := os.Stat(keyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))

			data, err := os.ReadFile(keyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(data).To(HaveLen(crypto.KeySize))
		})

		It("returns an error if the file already exists", func() {
			keyPath := filepath.Join(tmpDir, "secret.key")
			Expect(os.WriteFile(keyPath, make([]byte, crypto.KeySize), 0600)).To(Succeed())

			err := crypto.GenerateKey(keyPath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("refusing to overwrite"))
		})

		It("produces a key loadable by LoadKey", func() {
			keyPath := filepath.Join(tmpDir, "secret.key")
			Expect(crypto.GenerateKey(keyPath)).To(Succeed())

			key, err := crypto.LoadKey(keyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(key).To(HaveLen(crypto.KeySize))
		})
	})

	Describe("LoadKey", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "crypto-test-*")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("loads a valid key file", func() {
			keyPath := filepath.Join(tmpDir, "secret.key")
			key := make([]byte, crypto.KeySize)
			_, err := rand.Read(key)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(keyPath, key, 0600)
			Expect(err).NotTo(HaveOccurred())

			loaded, err := crypto.LoadKey(keyPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(Equal(key))
		})

		It("fails with ErrKeyMissing when file does not exist", func() {
			_, err := crypto.LoadKey(filepath.Join(tmpDir, "nonexistent"))
			Expect(err).To(MatchError(ContainSubstring("secret.key file not found")))
		})

		It("fails with ErrKeyPermissions when mode is not 0600", func() {
			keyPath := filepath.Join(tmpDir, "secret.key")
			key := make([]byte, crypto.KeySize)
			_, err := rand.Read(key)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(keyPath, key, 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = crypto.LoadKey(keyPath)
			Expect(err).To(MatchError(ContainSubstring("must have mode 0600")))
		})

		It("fails with ErrInvalidKeySize when key is wrong length", func() {
			keyPath := filepath.Join(tmpDir, "secret.key")
			err := os.WriteFile(keyPath, []byte("too-short"), 0600)
			Expect(err).NotTo(HaveOccurred())

			_, err = crypto.LoadKey(keyPath)
			Expect(err).To(MatchError(ContainSubstring("must be exactly 32 bytes")))
		})
	})
})
