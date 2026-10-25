// Package runtime launches and inspects a Godot project's running game process.
package runtime

import (
	"fmt"

	"github.com/davidherring123/godot-cli/internal/core/artifact"
)

const (
	StartName   = "runtime.start"
	StatusName  = "runtime.status"
	CaptureName = "runtime.capture"
	StopName    = "runtime.stop"
)

// SessionState describes a runtime session's lifecycle.
type SessionState string

const (
	StateNone         SessionState = "none"
	StateStarting     SessionState = "starting"
	StateRunning      SessionState = "running"
	StateUnresponsive SessionState = "unresponsive"
	StateExited       SessionState = "exited"
)

// Error is a Go-side runtime failure with an MCP-compatible error code.
type Error struct {
	Code    string
	Message string
}

func (err *Error) Error() string {
	return fmt.Sprintf("%s: %s", err.Code, err.Message)
}

func newError(code string, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Size is a window or image size in pixels.
type Size struct {
	Width  int `json:"width" jsonschema:"description=Width in pixels."`
	Height int `json:"height" jsonschema:"description=Height in pixels."`
}

// LogPaths are res:// paths to the runtime process's captured output.
type LogPaths struct {
	Stdout string `json:"stdout" jsonschema:"description=Project-relative res:// path to stdout captured from the game process."`
	Stderr string `json:"stderr" jsonschema:"description=Project-relative res:// path to stderr captured from the game process."`
}

// RuntimeError is one distinct engine or script error observed in the runtime
// logs.
type RuntimeError struct {
	Source  string `json:"source" jsonschema:"description=Log stream that produced the error: stdout, stderr, or runtime."`
	Message string `json:"message" jsonschema:"description=Error line reported by Godot or the runtime runner."`
	Count   int    `json:"count" jsonschema:"description=Number of times this error line appeared."`
}

// StartParams configures a runtime process.
type StartParams struct {
	Scene          string  `json:"scene,omitempty" jsonschema:"description=Scene resource path such as res://levels/arena.tscn, or a path inside the project. Defaults to the project's configured main scene."`
	Headless       bool    `json:"headless,omitempty" jsonschema:"description=Run without a window. Headless sessions can report errors and frame counts but cannot capture screenshots. Defaults to false."`
	TimeoutSeconds float64 `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum seconds to wait for the scene to become ready; defaults to 20."`
}

// StatusParams selects a session. An empty session inspects the active session,
// or the most recent session when none is running.
type StatusParams struct {
	Session string `json:"session,omitempty" jsonschema:"description=Runtime session ID. Omit to inspect the active session or the most recent one."`
}

// StatusResult describes a runtime session. Start returns the same shape.
type StatusResult struct {
	Active        bool           `json:"active" jsonschema:"description=Whether the game process is still alive."`
	State         SessionState   `json:"state" jsonschema:"description=One of none, starting, running, unresponsive, or exited."`
	Responsive    bool           `json:"responsive" jsonschema:"description=Whether the runtime heartbeat is fresh."`
	Session       string         `json:"session,omitempty" jsonschema:"description=Runtime session ID."`
	PID           int            `json:"pid,omitempty" jsonschema:"description=Game process ID."`
	Scene         string         `json:"scene,omitempty" jsonschema:"description=Scene resource path that the runtime loaded."`
	Headless      bool           `json:"headless,omitempty" jsonschema:"description=Whether the runtime was started without a window."`
	StartedAt     string         `json:"started_at,omitempty" jsonschema:"description=Session start time in RFC 3339 format."`
	UptimeMS      int64          `json:"uptime_ms,omitempty" jsonschema:"description=Session uptime from the most recent heartbeat or exit report."`
	Frames        int            `json:"frames" jsonschema:"description=Rendered or processed frame count from the most recent heartbeat."`
	DisplayServer string         `json:"display_server,omitempty" jsonschema:"description=Godot display server reported by the runtime, such as macOS or headless."`
	WindowSize    *Size          `json:"window_size,omitempty" jsonschema:"description=Runtime window size when known."`
	ArtifactsDir  string         `json:"artifacts_dir,omitempty" jsonschema:"description=Project-relative res:// directory containing runtime logs, requests, and captures."`
	Logs          *LogPaths      `json:"logs,omitempty" jsonschema:"description=Captured runtime log paths."`
	Errors        []RuntimeError `json:"errors" jsonschema:"description=Distinct ERROR, SCRIPT ERROR, USER ERROR, or FATAL lines observed since the session started."`
	ErrorCount    int            `json:"error_count" jsonschema:"description=Total error line occurrences, including duplicates."`
	LastCapture   string         `json:"last_capture,omitempty" jsonschema:"description=Project-relative res:// path to the most recent screenshot."`
	ExitCode      *int           `json:"exit_code,omitempty" jsonschema:"description=Process exit code when the runtime reported one."`
	ExitReason    string         `json:"exit_reason,omitempty" jsonschema:"description=Why the runtime ended, such as stop or closed."`
}

// CaptureParams selects a runtime and bounds the screenshot request.
type CaptureParams struct {
	Session        string  `json:"session,omitempty" jsonschema:"description=Runtime session ID. Omit to use the active session."`
	TimeoutSeconds float64 `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum seconds to wait for the screenshot; defaults to 10."`
}

// CaptureResult is a PNG screenshot and its metadata. The MCP server attaches
// Image as image content when it fits the embedding limit.
type CaptureResult struct {
	Session    string `json:"session" jsonschema:"description=Runtime session ID."`
	Artifact   string `json:"artifact" jsonschema:"description=Project-relative res:// path to the PNG under .godot/godot-cli/runtime."`
	MIMEType   string `json:"mime_type" jsonschema:"description=Image MIME type, currently image/png."`
	Width      int    `json:"width" jsonschema:"description=Image width in pixels."`
	Height     int    `json:"height" jsonschema:"description=Image height in pixels."`
	Bytes      int64  `json:"bytes" jsonschema:"description=PNG file size in bytes."`
	Frame      int    `json:"frame" jsonschema:"description=Runtime frame count when the screenshot was taken."`
	CapturedAt string `json:"captured_at" jsonschema:"description=Screenshot time in RFC 3339 format."`
	Embedded   bool   `json:"embedded" jsonschema:"description=Whether the MCP result includes the image bytes as image content."`

	Image []byte `json:"-"`
}

// Images implements artifact.ImageSource.
func (result CaptureResult) Images() []artifact.Image {
	if len(result.Image) == 0 {
		return nil
	}

	return []artifact.Image{{MIMEType: result.MIMEType, Data: result.Image}}
}

// StopParams selects a runtime and controls shutdown.
type StopParams struct {
	Session        string  `json:"session,omitempty" jsonschema:"description=Runtime session ID. Omit to stop the active session."`
	TimeoutSeconds float64 `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum seconds to wait for a graceful exit before killing the process; defaults to 5."`
	Force          bool    `json:"force,omitempty" jsonschema:"description=Kill a session whose heartbeat has stopped responding. Without force, unresponsive sessions are left untouched."`
}

// StopResult reports the outcome of a stop request.
type StopResult struct {
	Stopped        bool           `json:"stopped" jsonschema:"description=Whether this call terminated the runtime."`
	AlreadyStopped bool           `json:"already_stopped" jsonschema:"description=Whether no live runtime needed to be stopped."`
	Session        string         `json:"session,omitempty" jsonschema:"description=Runtime session ID."`
	State          SessionState   `json:"state" jsonschema:"description=Final session state, normally exited or none."`
	PID            int            `json:"pid,omitempty" jsonschema:"description=Game process ID."`
	ExitCode       *int           `json:"exit_code,omitempty" jsonschema:"description=Process exit code when the runtime reported one."`
	ExitReason     string         `json:"exit_reason,omitempty" jsonschema:"description=Why the runtime ended."`
	Errors         []RuntimeError `json:"errors" jsonschema:"description=Runtime errors observed before shutdown."`
	ErrorCount     int            `json:"error_count" jsonschema:"description=Total runtime error line occurrences."`
	ArtifactsDir   string         `json:"artifacts_dir,omitempty" jsonschema:"description=Project-relative res:// directory containing runtime logs and captures."`
}
