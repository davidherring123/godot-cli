package bridge_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
)

// deadPID is above every supported platform's maximum PID so liveness checks
// reliably report it as missing.
const deadPID = 1 << 30

func TestFindInstanceReturnsLiveEditor(t *testing.T) {
	root := t.TempDir()
	writeInstance(t, root, os.Getpid(), 49152, "test")

	instance, err := bridge.FindInstance(root)
	if err != nil {
		t.Fatalf("finding instance: %v", err)
	}

	if instance.PID != os.Getpid() || instance.Port != 49152 {
		t.Fatalf("instance = %+v; want pid %d, port 49152", instance, os.Getpid())
	}
}

func TestFindInstanceIgnoresStaleRegistration(t *testing.T) {
	root := t.TempDir()
	writeInstance(t, root, deadPID, 49152, "stale")

	_, err := bridge.FindInstance(root)
	if err != bridge.ErrNoEditor {
		t.Fatalf("FindInstance error = %v; want ErrNoEditor", err)
	}
}

func TestFindInstanceCountsOnlyLiveRegistration(t *testing.T) {
	root := t.TempDir()
	writeInstance(t, root, deadPID, 49152, "stale")
	writeInstance(t, root, os.Getpid(), 49153, "test")

	instance, err := bridge.FindInstance(root)
	if err != nil {
		t.Fatalf("finding instance: %v", err)
	}

	if instance.PID != os.Getpid() {
		t.Fatalf("instance = %+v; want the live editor", instance)
	}
}

func TestFindInstanceRejectsMultipleLiveEditors(t *testing.T) {
	root := t.TempDir()
	writeInstance(t, root, os.Getpid(), 49152, "test")
	writeInstance(t, root, os.Getppid(), 49153, "test")

	_, err := bridge.FindInstance(root)
	if err != bridge.ErrMultipleEditors {
		t.Fatalf("FindInstance error = %v; want ErrMultipleEditors", err)
	}
}

func TestCallTimesOutWhenEditorDoesNotRespond(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan net.Conn, 1)

	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()

	root := t.TempDir()
	port := listener.Addr().(*net.TCPAddr).Port
	writeInstance(t, root, os.Getpid(), port, "test")

	client, err := bridge.Connect(root)
	if err != nil {
		t.Fatalf("connecting to fake editor: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	client.SetTimeout(100 * time.Millisecond)

	started := time.Now()
	_, err = bridge.Call[struct{}, struct{}](client, "scene.current", struct{}{})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("Call succeeded; want a timeout error")
	}

	if !strings.Contains(err.Error(), "timed out waiting for Godot editor") {
		t.Fatalf("Call error = %v; want a timeout error", err)
	}

	if elapsed > 2*time.Second {
		t.Fatalf("Call took %s; want it to honor the request timeout", elapsed)
	}

	select {
	case connection := <-accepted:
		_ = connection.Close()
	case <-time.After(time.Second):
		t.Fatal("fake editor never accepted the connection")
	}
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
