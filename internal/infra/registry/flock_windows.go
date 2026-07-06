//go:build windows

package registry

type fileLock struct{}

func newFileLock(_ string) (*fileLock, error) { return &fileLock{}, nil }
func (l *fileLock) lock() error               { return nil }
func (l *fileLock) unlock() error             { return nil }
func (l *fileLock) close() error              { return nil }
