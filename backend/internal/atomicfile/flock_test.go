package atomicfile_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/atomicfile"
)

func TestFileLock_BasicLockUnlock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flock-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	target := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(target, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	lock, err := atomicfile.Lock(target)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	// Lock file is intentionally kept (prevents flock race).
	if _, err := os.Stat(target + ".lock"); os.IsNotExist(err) {
		t.Error("lock file should still exist after Unlock")
	}
}

func TestFileLock_ConcurrentAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "flock-concurrent-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	target := filepath.Join(tmpDir, "counter.json")
	if err := os.WriteFile(target, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use file lock to protect concurrent increments.
	var wg sync.WaitGroup
	iterations := 20

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			lock, err := atomicfile.Lock(target)
			if err != nil {
				t.Errorf("Lock failed: %v", err)
				return
			}
			defer lock.Unlock()

			// Read current value.
			data, err := os.ReadFile(target)
			if err != nil {
				t.Errorf("Read failed: %v", err)
				return
			}

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
			if err := os.WriteFile(target, s, 0644); err != nil {
				t.Errorf("Write failed: %v", err)
			}
		}()
	}

	wg.Wait()

	// Final value should be exactly iterations.
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	val := 0
	for _, b := range data {
		if b >= '0' && b <= '9' {
			val = val*10 + int(b-'0')
		}
	}
	if val != iterations {
		t.Errorf("Expected counter=%d, got %d (race condition detected)", iterations, val)
	}
}
