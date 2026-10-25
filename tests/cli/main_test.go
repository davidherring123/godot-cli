package cli_test

import (
	"os"
	"testing"

	"github.com/davidherring123/godot-cli/internal/compose"
)

func TestMain(m *testing.M) {
	compose.Install()
	os.Exit(m.Run())
}
