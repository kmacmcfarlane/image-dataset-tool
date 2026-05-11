package atomicfile_test

import (
	"os"
	"path/filepath"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/atomicfile"
)

var _ = Describe("FileLock", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "flock-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("BasicLockUnlock", func() {
		It("acquires and releases a lock without error", func() {
			target := filepath.Join(tmpDir, "test.json")
			Expect(os.WriteFile(target, []byte("{}"), 0644)).To(Succeed())

			lock, err := atomicfile.Lock(target)
			Expect(err).NotTo(HaveOccurred())

			Expect(lock.Unlock()).To(Succeed())

			// Lock file is intentionally kept (prevents flock race).
			_, err = os.Stat(target + ".lock")
			Expect(err).NotTo(HaveOccurred(), "lock file should still exist after Unlock")
		})
	})

	Describe("ConcurrentAccess", func() {
		It("serialises concurrent increments via file lock", func() {
			target := filepath.Join(tmpDir, "counter.json")
			Expect(os.WriteFile(target, []byte("0"), 0644)).To(Succeed())

			var wg sync.WaitGroup
			iterations := 20

			for i := 0; i < iterations; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()

					lock, err := atomicfile.Lock(target)
					Expect(err).NotTo(HaveOccurred())
					defer lock.Unlock()

					// Read current value.
					data, err := os.ReadFile(target)
					Expect(err).NotTo(HaveOccurred())

					// Parse and increment.
					val := 0
					for _, b := range data {
						if b >= '0' && b <= '9' {
							val = val*10 + int(b-'0')
						}
					}
					val++

					// Write back.
					s := make([]byte, 0, 8)
					if val == 0 {
						s = append(s, '0')
					} else {
						tmp := val
						digits := make([]byte, 0, 8)
						for tmp > 0 {
							digits = append(digits, byte('0'+tmp%10))
							tmp /= 10
						}
						for i := len(digits) - 1; i >= 0; i-- {
							s = append(s, digits[i])
						}
					}
					Expect(os.WriteFile(target, s, 0644)).To(Succeed())
				}()
			}

			wg.Wait()

			// Final value should be exactly iterations.
			data, err := os.ReadFile(target)
			Expect(err).NotTo(HaveOccurred())

			val := 0
			for _, b := range data {
				if b >= '0' && b <= '9' {
					val = val*10 + int(b-'0')
				}
			}
			Expect(val).To(Equal(iterations), "race condition detected: counter mismatch")
		})
	})
})
