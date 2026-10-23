package runtime_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidherring123/godot-cli/internal/core/godot"
	gdruntime "github.com/davidherring123/godot-cli/internal/core/runtime"
)

func TestStatusWithoutRuntime(t *testing.T) {
	root := newProject(t, false)

	status, err := gdruntime.Status(root, gdruntime.StatusParams{})
	if err != nil {
		t.Fatalf("checking status: %v", err)
	}

	if status.Active || status.State != gdruntime.StateNone {
		t.Fatalf("status = %+v; want no active runtime", status)
	}

	if status.Errors == nil {
		t.Fatal("status errors should be an empty list, not null")
	}
}

func TestSessionPathTraversalRejected(t *testing.T) {
	root := newProject(t, false)

	_, err := gdruntime.Status(root, gdruntime.StatusParams{Session: "../escape"})

	var runtimeError *gdruntime.Error

	if !errors.As(err, &runtimeError) {
		t.Fatalf("status error = %v; want a runtime error", err)
	}

	if runtimeError.Code != "invalid_argument" {
		t.Fatalf("error code = %q; want invalid_argument", runtimeError.Code)
	}
}

func TestHeadlessLifecycleAndErrors(t *testing.T) {
	requireGodot(t)

	root := newProject(t, true)
	start := startHeadlessRuntime(t, root)

	t.Cleanup(func() {
		_, _ = gdruntime.Stop(root, gdruntime.StopParams{
			Session: start.Session,
			Force:   true,
		})
	})

	assertStarted(t, start)
	assertStartConflict(t, root)

	status := waitForRuntimeError(t, root, start.Session)
	assertRuntimeStatus(t, status)

	assertHeadlessCaptureUnsupported(t, root, start.Session)
	assertStop(t, root, start.Session, true)
	assertExitedStatus(t, root, start.Session)
	assertAlreadyStopped(t, root, start.Session)
}

func startHeadlessRuntime(t *testing.T, root string) gdruntime.StatusResult {
	t.Helper()

	start, err := gdruntime.Start(root, gdruntime.StartParams{
		Headless:       true,
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("starting runtime: %v", err)
	}

	return start
}

func assertStarted(t *testing.T, start gdruntime.StatusResult) {
	t.Helper()

	if !start.Active || start.State != gdruntime.StateRunning {
		t.Fatalf("start = %+v; want a running session", start)
	}

	if start.Session == "" || start.PID <= 0 || start.Scene != "res://main.tscn" {
		t.Fatalf("start = %+v; want session, pid, and scene", start)
	}

	if !strings.HasPrefix(start.ArtifactsDir, "res://.godot/godot-cli/runtime/") {
		t.Fatalf("artifacts directory = %q; want a runtime res:// path", start.ArtifactsDir)
	}

	if start.Logs == nil || !strings.HasPrefix(start.Logs.Stdout, "res://") {
		t.Fatalf("start logs = %+v; want res:// log paths", start.Logs)
	}
}

func assertStartConflict(t *testing.T, root string) {
	t.Helper()

	_, err := gdruntime.Start(root, gdruntime.StartParams{Headless: true})

	var runtimeError *gdruntime.Error

	if !errors.As(err, &runtimeError) {
		t.Fatalf("second start error = %v; want a runtime error", err)
	}

	if runtimeError.Code != "conflict" {
		t.Fatalf("second start code = %q; want conflict", runtimeError.Code)
	}
}

func assertRuntimeStatus(t *testing.T, status gdruntime.StatusResult) {
	t.Helper()

	if status.ErrorCount == 0 || len(status.Errors) == 0 {
		t.Fatal("runtime did not report the scene's push_error")
	}

	if status.Frames < 2 {
		t.Fatalf("frames = %d; want at least the readiness frames", status.Frames)
	}
}

func assertHeadlessCaptureUnsupported(t *testing.T, root string, session string) {
	t.Helper()

	_, err := gdruntime.Capture(root, gdruntime.CaptureParams{Session: session})

	var captureError *gdruntime.Error

	if !errors.As(err, &captureError) {
		t.Fatalf("capture error = %v; want a runtime error", err)
	}

	if captureError.Code != "unsupported" {
		t.Fatalf("capture error code = %q; want unsupported", captureError.Code)
	}

	if !strings.Contains(captureError.Message, "headless=true") {
		t.Fatalf("capture error = %q; want an actionable headless explanation", captureError.Message)
	}
}

func assertStop(t *testing.T, root string, session string, wantStopped bool) {
	t.Helper()

	stop, err := gdruntime.Stop(root, gdruntime.StopParams{
		Session:        session,
		TimeoutSeconds: 10,
	})
	if err != nil {
		t.Fatalf("stopping runtime: %v", err)
	}

	if stop.Stopped != wantStopped || stop.State != gdruntime.StateExited {
		t.Fatalf("stop = %+v; want stopped=%v and an exited session", stop, wantStopped)
	}

	if wantStopped && stop.ExitCode != nil && *stop.ExitCode != 0 {
		t.Fatalf("stop exit code = %v; want 0", stop.ExitCode)
	}

	if stop.ErrorCount == 0 || len(stop.Errors) == 0 {
		t.Fatalf("stop = %+v; want the runtime's errors", stop)
	}
}

func assertExitedStatus(t *testing.T, root string, session string) {
	t.Helper()

	status, err := gdruntime.Status(root, gdruntime.StatusParams{Session: session})
	if err != nil {
		t.Fatalf("checking stopped status: %v", err)
	}

	if status.Active || status.State != gdruntime.StateExited {
		t.Fatalf("stopped status = %+v; want an exited session", status)
	}
}

func assertAlreadyStopped(t *testing.T, root string, session string) {
	t.Helper()

	again, err := gdruntime.Stop(root, gdruntime.StopParams{Session: session})
	if err != nil {
		t.Fatalf("stopping twice: %v", err)
	}

	if again.Stopped || !again.AlreadyStopped {
		t.Fatalf("second stop = %+v; want already_stopped", again)
	}
}

func TestStartReportsMissingScene(t *testing.T) {
	requireGodot(t)

	root := newProject(t, false)

	_, err := gdruntime.Start(root, gdruntime.StartParams{
		Scene:          "res://missing.tscn",
		Headless:       true,
		TimeoutSeconds: 30,
	})

	var runtimeError *gdruntime.Error

	if !errors.As(err, &runtimeError) {
		t.Fatalf("start error = %v; want a runtime error", err)
	}

	if runtimeError.Code != "invalid_argument" {
		t.Fatalf("error code = %q; want invalid_argument", runtimeError.Code)
	}

	if !strings.Contains(runtimeError.Message, "not found") {
		t.Fatalf("error = %q; want the missing scene path", runtimeError.Message)
	}

	status, err := gdruntime.Status(root, gdruntime.StatusParams{})
	if err != nil {
		t.Fatalf("checking status: %v", err)
	}

	if status.Active {
		t.Fatalf("status = %+v; want no active runtime after a failed start", status)
	}
}

func TestWindowedCaptureProducesImage(t *testing.T) {
	if os.Getenv("GODOT_MODE") != "graphical" {
		t.Skip("set GODOT_MODE=graphical to exercise windowed screenshot capture")
	}

	requireGodot(t)

	root := newProject(t, false)

	start, err := gdruntime.Start(root, gdruntime.StartParams{TimeoutSeconds: 30})
	if err != nil {
		t.Fatalf("starting windowed runtime: %v", err)
	}

	t.Cleanup(func() {
		_, _ = gdruntime.Stop(root, gdruntime.StopParams{
			Session: start.Session,
			Force:   true,
		})
	})

	if start.Headless {
		t.Fatal("start reported a headless session; want a windowed runtime")
	}

	capture, err := gdruntime.Capture(root, gdruntime.CaptureParams{Session: start.Session})
	if err != nil {
		t.Fatalf("capturing screenshot: %v", err)
	}

	if capture.Width <= 0 || capture.Height <= 0 || capture.Bytes <= 0 {
		t.Fatalf("capture = %+v; want a non-empty image", capture)
	}

	if capture.MIMEType != "image/png" || !capture.Embedded {
		t.Fatalf("capture = %+v; want an embedded PNG", capture)
	}

	images := capture.Images()
	if len(images) != 1 || !bytes.HasPrefix(images[0].Data, []byte("\x89PNG")) {
		t.Fatal("capture did not provide PNG image data")
	}

	if !strings.HasPrefix(capture.Artifact, "res://.godot/godot-cli/runtime/") {
		t.Fatalf("artifact = %q; want a runtime res:// path", capture.Artifact)
	}

	encoded, err := json.Marshal(capture)
	if err != nil {
		t.Fatalf("marshaling capture: %v", err)
	}

	if strings.Contains(string(encoded), root) {
		t.Fatalf("capture JSON leaked an absolute project path: %s", encoded)
	}

	artifactPath := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(capture.Artifact, "res://")))

	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("screenshot artifact is not readable: %v", err)
	}
}

func waitForRuntimeError(t *testing.T, root string, session string) gdruntime.StatusResult {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		status, err := gdruntime.Status(root, gdruntime.StatusParams{Session: session})
		if err != nil {
			t.Fatalf("checking runtime: %v", err)
		}

		if status.ErrorCount > 0 {
			return status
		}

		if time.Now().After(deadline) {
			t.Fatalf("runtime did not report its scene error: %+v", status)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func requireGodot(t *testing.T) {
	t.Helper()

	if _, err := godot.FindExecutable(); err != nil {
		t.Skip(err)
	}
}

func newProject(t *testing.T, sceneError bool) string {
	t.Helper()

	root := t.TempDir()

	project := `config_version=5

[application]
config/name="godot-cli runtime test"
run/main_scene="res://main.tscn"

[rendering]
renderer/rendering_method="gl_compatibility"
renderer/rendering_method.mobile="gl_compatibility"
`

	if err := os.WriteFile(
		filepath.Join(root, "project.godot"),
		[]byte(project),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	script := `extends Node2D

func _ready() -> void:
	pass
`

	if sceneError {
		script = `extends Node2D

func _ready() -> void:
	push_error("godot-cli runtime test error")
`
	}

	if err := os.WriteFile(filepath.Join(root, "main.gd"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	scene := `[gd_scene load_steps=2 format=3]

[ext_resource type="Script" path="res://main.gd" id="1"]

[node name="Main" type="Node2D"]
script = ExtResource("1")

[node name="Rect" type="ColorRect" parent="."]
offset_left = 64.0
offset_top = 64.0
offset_right = 192.0
offset_bottom = 192.0
color = Color(0.2, 0.4, 0.9, 1)
`

	if err := os.WriteFile(filepath.Join(root, "main.tscn"), []byte(scene), 0644); err != nil {
		t.Fatal(err)
	}

	return root
}
