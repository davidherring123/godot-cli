// Package editorinstance defines the neutral model and discovery port for
// running Godot editor registrations.
//
// The addon writes .godot/godot-mcp/instances/<pid>.json. An adapter provides
// the filesystem/process implementation; domain packages only depend on this
// port.
package editorinstance

import "errors"

var (
	// ErrNoEditor reports that no running Godot editor is registered for the
	// project.
	ErrNoEditor = errors.New("no running Godot editor found")

	// ErrMultipleEditors reports that more than one editor is registered for
	// the project.
	ErrMultipleEditors = errors.New("multiple Godot editor instances found")
)

// Instance is one editor registration.
type Instance struct {
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	Project    string `json:"project"`
	Version    string `json:"version"`
	Executable string `json:"executable,omitempty"`
}

// Finder discovers the single running editor registered for a project.
type Finder interface {
	FindInstance(projectRoot string) (Instance, error)
}

// FinderFunc adapts a function to Finder.
type FinderFunc func(projectRoot string) (Instance, error)

func (fn FinderFunc) FindInstance(projectRoot string) (Instance, error) {
	return fn(projectRoot)
}

var finder Finder

// SetFinder installs the process-wide editor instance finder. Passing nil
// restores the unconfigured state and is intended for tests.
func SetFinder(next Finder) {
	finder = next
}

// Find returns the single running editor registered for the project.
func Find(projectRoot string) (Instance, error) {
	if finder == nil {
		return Instance{}, ErrNoEditor
	}

	return finder.FindInstance(projectRoot)
}
