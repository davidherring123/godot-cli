package editor_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
	"github.com/davidherring123/godot-cli/internal/core/editor"
)

// deadPID is above every supported platform's maximum PID so liveness checks
// reliably report it as missing.
const deadPID = 1 << 30

func TestStatusReportsNoEditor(t *testing.T) {
	root := testProject(t)

	result, err := editor.Status(root, editor.StatusParams{})
	if err != nil {
		t.Fatalf("checking status: %v", err)
	}

	if result.Running {
		t.Fatalf("status = %+v; want not running", result)
	}
}

func TestStatusReportsRegisteredEditor(t *testing.T) {
	root := testProject(t)
	writeInstance(t, root, os.Getpid(), 49152, "test")

	result, err := editor.Status(root, editor.StatusParams{})
	if err != nil {
		t.Fatalf("checking status: %v", err)
	}

	if !result.Running {
		t.Fatal("status reported not running; want running")
	}

	if result.PID != os.Getpid() || result.Port != 49152 || result.Version != "test" {
		t.Fatalf("status = %+v; want pid %d, port 49152, version test", result, os.Getpid())
	}
}

func TestStatusIgnoresStaleRegistration(t *testing.T) {
	root := testProject(t)
	writeInstance(t, root, deadPID, 49152, "stale")

	result, err := editor.Status(root, editor.StatusParams{})
	if err != nil {
		t.Fatalf("checking status: %v", err)
	}

	if result.Running {
		t.Fatalf("status = %+v; want not running for a dead process", result)
	}
}

func TestStatusIgnoresStaleRegistrationAlongsideLiveEditor(t *testing.T) {
	root := testProject(t)
	writeInstance(t, root, deadPID, 49152, "stale")
	writeInstance(t, root, os.Getpid(), 49153, "test")

	result, err := editor.Status(root, editor.StatusParams{})
	if err != nil {
		t.Fatalf("checking status: %v", err)
	}

	if !result.Running || result.PID != os.Getpid() {
		t.Fatalf("status = %+v; want the live editor", result)
	}
}

func TestStatusRejectsMultipleEditors(t *testing.T) {
	root := testProject(t)
	writeInstance(t, root, os.Getpid(), 49152, "test")
	writeInstance(t, root, os.Getppid(), 49153, "test")

	_, err := editor.Status(root, editor.StatusParams{})
	if !errors.Is(err, bridge.ErrMultipleEditors) {
		t.Fatalf("status error = %v; want ErrMultipleEditors", err)
	}
}

func TestOpenReturnsAlreadyRunningEditor(t *testing.T) {
	root := testProject(t)
	writeInstance(t, root, os.Getpid(), 49152, "test")

	result, err := editor.Open(root, editor.OpenParams{})
	if err != nil {
		t.Fatalf("opening editor: %v", err)
	}

	if result.Opened {
		t.Fatal("Open reported opened; want an existing editor")
	}

	if !result.AlreadyRunning {
		t.Fatal("Open did not report already_running")
	}

	if result.PID != os.Getpid() || result.Port != 49152 || result.Version != "test" {
		t.Fatalf("result = %+v; want pid %d, port 49152, version test", result, os.Getpid())
	}
}

func testProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte("[application]\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	return root
}

func writeInstance(t *testing.T, root string, pid int, port int, version string) {
	t.Helper()

	dir := filepath.Join(root, ".godot", "godot-cli", "instances")

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(bridge.Instance{
		PID:     pid,
		Port:    port,
		Project: root,
		Version: version,
	})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, fmt.Sprintf("%d.json", pid))

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
