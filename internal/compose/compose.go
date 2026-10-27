// Package compose installs the process-wide adapters used by core actions.
//
// It is the single place where transport and editor instance discovery
// implementations are bound to their neutral ports. Domain packages never
// import adapters.
package compose

import (
	adapterbridge "github.com/davidherring123/godot-cli/internal/adapter/bridge"
	"github.com/davidherring123/godot-cli/internal/port/editorinstance"
	"github.com/davidherring123/godot-cli/internal/port/invocation"
)

// Install binds the production adapters to the core ports. It is idempotent.
func Install() {
	invocation.SetTransport(adapterbridge.Transport{})
	editorinstance.SetFinder(editorinstance.FinderFunc(adapterbridge.FindInstance))
}
