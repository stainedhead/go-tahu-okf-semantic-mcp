//go:build !windows

package registry

import (
	"fmt"
	"os"
	"syscall"
)

// fileLock is a flock-based advisory file lock for the registry YAML.
// Uses syscall.Flock (Unix-only, no external dependencies).
// Windows is deferred (NG8); see flock_windows.go for the no-op stub.
type fileLock struct {
	f *os.File
}

func newFileLock(lockPath string) (*fileLock, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // lockPath is config-derived
	if err != nil {
		return nil, fmt.Errorf("registry: open lock file %s: %w", lockPath, err)
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) lock() error {
	return syscall.Flock(int(l.f.Fd()), syscall.LOCK_EX) //nolint:gosec // G115: Fd() returns uintptr which fits int on all supported platforms
}

func (l *fileLock) unlock() error {
	return syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN) //nolint:gosec // G115: same as above
}

func (l *fileLock) close() error {
	_ = l.unlock()
	return l.f.Close()
}
