//go:build unix

package editor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core/editor"
)

func TestOpenStartsEditorAndWaitsForBridge(t *testing.T) {
	root := testProject(t)
	t.Setenv("GODOT_BIN", installFakeEditor(t))

	result, err := editor.Open(root, editor.OpenParams{TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("opening editor: %v", err)
	}

	if !result.Opened {
		t.Fatal("Open did not report opening the editor")
	}

	if result.AlreadyRunning {
		t.Fatal("Open reported already_running for a new editor")
	}

	if result.PID <= 0 || result.Port != 49152 || result.Version != "fake" {
		t.Fatalf("result = %+v; want a pid, port 49152, version fake", result)
	}

	t.Cleanup(func() {
		process, err := os.FindProcess(result.PID)
		if err == nil {
			_ = process.Kill()
		}
	})
}

func TestOpenStartsEditorWhenRegistrationIsStale(t *testing.T) {
	root := testProject(t)
	writeInstance(t, root, deadPID, 49152, "stale")
	t.Setenv("GODOT_BIN", installFakeEditor(t))

	result, err := editor.Open(root, editor.OpenParams{TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("opening editor: %v", err)
	}

	if !result.Opened {
		t.Fatal("Open reused a stale registration instead of starting the editor")
	}

	if result.AlreadyRunning {
		t.Fatal("Open reported already_running for a stale registration")
	}

	if result.PID <= 0 || result.PID == deadPID {
		t.Fatalf("result = %+v; want the new editor's pid", result)
	}

	t.Cleanup(func() {
		process, err := os.FindProcess(result.PID)
		if err == nil {
			_ = process.Kill()
		}
	})
}

func installFakeEditor(t *testing.T) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "godot")

	content := `#!/bin/sh
root=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--path" ]; then
    root="$2"
    shift 2
    continue
  fi
  shift
done
mkdir -p "$root/.godot/godot-cli/instances"
printf '{"pid":%s,"port":49152,"project":"%s","version":"fake"}\n' "$$" "$root" > "$root/.godot/godot-cli/instances/$$.json"
sleep 10
`

	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}

	return script
}
