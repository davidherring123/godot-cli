// Package godot defines the neutral port for locating the Godot executable.
//
// The filesystem/PATH implementation lives in internal/adapter/godot and is
// installed once by the composition root.
package godot

import (
	"errors"
	"fmt"
)

// ErrNotFound reports that no Godot executable could be located.
var ErrNotFound = errors.New("godot executable not found")

// Finder locates the Godot executable.
type Finder interface {
	FindExecutable() (string, error)
}

// FinderFunc adapts a function to Finder.
type FinderFunc func() (string, error)

func (fn FinderFunc) FindExecutable() (string, error) {
	return fn()
}

var finder Finder

// SetFinder installs the process-wide executable finder. Passing nil restores
// the unconfigured state and is intended for tests.
func SetFinder(next Finder) {
	finder = next
}

// FindExecutable locates Godot using the installed finder.
func FindExecutable() (string, error) {
	if finder == nil {
		return "", fmt.Errorf("%w; set GODOT_BIN to its path", ErrNotFound)
	}

	return finder.FindExecutable()
}
