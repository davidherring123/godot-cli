//go:build !windows

package mcp

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type serverStateLock struct {
	file *os.File
}

func lockServerState(path string) (*serverStateLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()

		return nil, fmt.Errorf("state file is already managed by another process: %w", err)
	}

	return &serverStateLock{file: file}, nil
}

func (lock *serverStateLock) close() {
	_ = unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
	_ = lock.file.Close()
}

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
