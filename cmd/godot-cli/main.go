// Command godot-cli controls a running Godot editor from the command line.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/davidherring123/godot-cli/internal/cli"
	"github.com/davidherring123/godot-cli/internal/compose"
)

// VERSION is replaced with -ldflags "-X main.VERSION=..." in release builds.
var VERSION = "dev"

func main() {
	compose.Install()

	root := cli.NewRootCommand(VERSION)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
