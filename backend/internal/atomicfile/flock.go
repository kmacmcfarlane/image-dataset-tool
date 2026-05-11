package atomicfile

import (
	"fmt"
	"os"
	"syscall"
)

// FileLock provides file-level locking for concurrent manifest access.
// Uses flock(2) to prevent lost updates when UI and pipeline write to the same sample.
type FileLock struct {
	file *os.File
}

// Lock acquires an exclusive file lock on the given path.
// Creates a .lock file alongside the target. The caller must call Unlock when done.
func Lock(path string) (*FileLock, error) {
	lockPath := path + ".lock"

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquire file lock: %w", err)
	}

	return &FileLock{file: f}, nil
}

// Unlock releases the file lock.
// The .lock file is intentionally NOT removed to avoid a race condition where
// another process creates a new file descriptor, bypassing the held lock.
func (fl *FileLock) Unlock() error {
	if fl.file == nil {
		return nil
	}

	// Release the flock.
	if err := syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN); err != nil {
		fl.file.Close()
		return fmt.Errorf("release file lock: %w", err)
	}

	if err := fl.file.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}

	fl.file = nil
	return nil
}
