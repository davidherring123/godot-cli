// Command godot-mcp-host serves MCP for a Godot project on behalf of the
// godot-cli editor addon. It is bundled in release addons; terminal project
// editing remains available through cmd/godot-cli during the transition.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/davidherring123/godot-cli/internal/compose"
	"github.com/davidherring123/godot-cli/internal/hostcmd"
)

// VERSION is replaced with -ldflags "-X main.VERSION=..." in release builds.
var VERSION = "dev"

func main() {
	compose.Install()

	root := hostcmd.NewRootCommand(VERSION)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
