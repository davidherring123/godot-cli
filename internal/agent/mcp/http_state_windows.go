//go:build windows

package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

type serverStateLock struct {
	file       *os.File
	overlapped windows.Overlapped
}

func lockServerState(path string) (*serverStateLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	lock := &serverStateLock{file: file}
	err = windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&lock.overlapped,
	)
	if err != nil {
		_ = file.Close()

		return nil, fmt.Errorf("state file is already managed by another process: %w", err)
	}

	return lock, nil
}

func (lock *serverStateLock) close() {
	_ = windows.UnlockFileEx(windows.Handle(lock.file.Fd()), 0, 1, 0, &lock.overlapped)
	_ = lock.file.Close()
}

func replaceFile(source, destination string) error {
	sourcePointer, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}

	destinationPointer, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}

	return windows.MoveFileEx(
		sourcePointer,
		destinationPointer,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
