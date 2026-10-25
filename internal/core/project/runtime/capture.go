package runtime

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/davidherring123/godot-cli/internal/core/bridge"
)

const capturePoll = 25 * time.Millisecond

// Capture asks the running game to save a screenshot and returns it as a
// project-scoped artifact. The PNG bytes are attached to the MCP result when
// they fit the embedding limit.
func Capture(projectRoot string, params CaptureParams) (CaptureResult, error) {
	info, err := requireRunningSession(projectRoot, params.Session)
	if err != nil {
		return CaptureResult{}, err
	}

	if info.metadata.Headless {
		return CaptureResult{}, newError(
			string(bridge.ErrorCodeUnsupported),
			"screenshots require a rendering display, but session %s was started with headless=true; stop it and start a windowed session",
			info.metadata.Session,
		)
	}

	identifier := fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(3))
	relative := filepath.ToSlash(filepath.Join(
		runtimeDirName,
		sessionsDirName,
		info.metadata.Session,
		"captures",
		"capture-"+identifier+".png",
	))
	output := "res://" + relative
	absoluteOutput := filepath.Join(projectRoot, filepath.FromSlash(relative))
	requestPath := filepath.Join(info.directory, "requests", identifier+".json")
	responsePath := filepath.Join(info.directory, "responses", identifier+".json")

	// Send res:// rather than an absolute path so Godot resolves the project
	// root consistently even when the host path contains symbolic links.
	if err := writeJSONAtomic(requestPath, map[string]string{"path": output}); err != nil {
		return CaptureResult{}, newError(
			string(bridge.ErrorCodeInternalError),
			"writing capture request: %v",
			err,
		)
	}

	timeout := captureTimeout

	if params.TimeoutSeconds > 0 {
		timeout = time.Duration(params.TimeoutSeconds * float64(time.Second))
	}

	return waitForCapture(projectRoot, info, absoluteOutput, responsePath, timeout)
}

func waitForCapture(
	projectRoot string,
	info *sessionInfo,
	absoluteOutput string,
	responsePath string,
	timeout time.Duration,
) (CaptureResult, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(capturePoll)
	defer ticker.Stop()

	for {
		var response captureResponse

		found, err := readJSON(responsePath, &response)
		if err != nil {
			return CaptureResult{}, newError(
				string(bridge.ErrorCodeInternalError),
				"reading capture response: %v",
				err,
			)
		}

		if found {
			if !response.OK {
				return CaptureResult{}, captureError(response)
			}

			return captureResult(
				projectRoot,
				info.metadata.Session,
				absoluteOutput,
				response,
			)
		}

		if !sameProcess(info.metadata.PID, info.metadata.ProcessIdentity) {
			return CaptureResult{}, newError(
				string(bridge.ErrorCodeFailedPrecondition),
				"runtime session %s exited before the screenshot completed; call runtime.status for errors",
				info.metadata.Session,
			)
		}

		select {
		case <-deadline.C:
			return CaptureResult{}, newError(
				string(bridge.ErrorCodeActionFailed),
				"timed out after %s waiting for a screenshot from session %s; the runtime may be stalled",
				timeout,
				info.metadata.Session,
			)
		case <-ticker.C:
		}
	}
}

func captureError(response captureResponse) error {
	code := response.Code

	if code == "" {
		code = string(bridge.ErrorCodeActionFailed)
	}

	message := response.Message

	if message == "" {
		message = "the runtime could not capture a screenshot"
	}

	return newError(code, "%s", message)
}

func captureResult(
	projectRoot string,
	session string,
	absoluteOutput string,
	response captureResponse,
) (CaptureResult, error) {
	data, err := os.ReadFile(absoluteOutput)
	if err != nil {
		return CaptureResult{}, newError(
			string(bridge.ErrorCodeActionFailed),
			"reading captured screenshot: %v",
			err,
		)
	}

	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return CaptureResult{}, newError(
			string(bridge.ErrorCodeActionFailed),
			"Godot wrote a screenshot that is not a readable PNG: %v",
			err,
		)
	}

	artifactPath, err := resPath(projectRoot, absoluteOutput)
	if err != nil {
		return CaptureResult{}, newError(
			string(bridge.ErrorCodeInternalError),
			"resolving screenshot artifact path: %v",
			err,
		)
	}

	result := CaptureResult{
		Session:    session,
		Artifact:   artifactPath,
		MIMEType:   "image/png",
		Width:      config.Width,
		Height:     config.Height,
		Bytes:      int64(len(data)),
		Frame:      response.Frame,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Embedded:   len(data) <= maxEmbeddedImage,
	}

	if result.Embedded {
		result.Image = data
	}

	return result, nil
}

func requireRunningSession(projectRoot string, session string) (*sessionInfo, error) {
	var (
		info *sessionInfo
		err  error
	)

	if session != "" {
		info, err = inspectSession(projectRoot, session)
	} else {
		info, err = activeSession(projectRoot)
	}

	if err != nil {
		return nil, err
	}

	if info == nil {
		return nil, newError(
			string(bridge.ErrorCodeFailedPrecondition),
			"no runtime session is active for this project; call runtime.start first",
		)
	}

	switch info.state {
	case StateExited:
		return nil, newError(
			string(bridge.ErrorCodeFailedPrecondition),
			"runtime session %s has exited; call runtime.status for errors or runtime.start for a new session",
			info.metadata.Session,
		)
	case StateStarting:
		return nil, newError(
			string(bridge.ErrorCodeFailedPrecondition),
			"runtime session %s is still starting; retry after runtime.start returns",
			info.metadata.Session,
		)
	case StateUnresponsive:
		return nil, newError(
			string(bridge.ErrorCodeFailedPrecondition),
			"runtime session %s is alive but not responding; stop it with force=true and start a new session",
			info.metadata.Session,
		)
	}

	if !info.responsive {
		return nil, newError(
			string(bridge.ErrorCodeFailedPrecondition),
			"runtime session %s has not reported a recent heartbeat; stop it with force=true and start a new session",
			info.metadata.Session,
		)
	}

	return info, nil
}
