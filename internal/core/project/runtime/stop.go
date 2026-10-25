package runtime

import (
	"time"

	"github.com/davidherring123/godot-cli/internal/protocol/apierr"
)

// Stop asks a runtime to quit, then kills it if it does not exit within the
// requested timeout. Starting sessions are stopped gracefully because they
// have not had a chance to report a heartbeat yet. Only unresponsive sessions
// require Force, so a reused or unrelated process is never killed implicitly.
func Stop(projectRoot string, params StopParams) (StopResult, error) {
	info, err := stopTarget(projectRoot, params.Session)
	if err != nil {
		return StopResult{}, err
	}

	if info == nil {
		return StopResult{
			AlreadyStopped: true,
			State:          StateNone,
			Errors:         []RuntimeError{},
		}, nil
	}

	if info.state == StateExited ||
		!sameProcess(info.metadata.PID, info.metadata.ProcessIdentity) {
		return stoppedResult(projectRoot, info.metadata.Session, false)
	}

	if info.state == StateUnresponsive {
		if !params.Force {
			return StopResult{}, newError(
				string(apierr.CodeFailedPrecondition),
				"runtime session %s is alive but not responding; pass force=true to kill pid %d",
				info.metadata.Session,
				info.metadata.PID,
			)
		}
	}

	timeout := defaultStopWait

	if params.TimeoutSeconds > 0 {
		timeout = time.Duration(params.TimeoutSeconds * float64(time.Second))
	}

	if !terminate(info.metadata, info.directory, timeout) {
		return StopResult{}, newError(
			string(apierr.CodeActionFailed),
			"runtime session %s did not exit after %s; pid %d is still alive",
			info.metadata.Session,
			timeout,
			info.metadata.PID,
		)
	}

	return stoppedResult(projectRoot, info.metadata.Session, true)
}

func stopTarget(projectRoot string, session string) (*sessionInfo, error) {
	if session != "" {
		return inspectSession(projectRoot, session)
	}

	info, err := activeSession(projectRoot)
	if err != nil {
		return nil, err
	}

	if info != nil {
		return info, nil
	}

	return latestSession(projectRoot)
}

func stoppedResult(
	projectRoot string,
	session string,
	stopped bool,
) (StopResult, error) {
	info, err := inspectSession(projectRoot, session)
	if err != nil {
		return StopResult{}, err
	}

	status := buildStatus(projectRoot, info)

	return StopResult{
		Stopped:        stopped,
		AlreadyStopped: !stopped,
		Session:        status.Session,
		State:          status.State,
		PID:            status.PID,
		ExitCode:       status.ExitCode,
		ExitReason:     status.ExitReason,
		Errors:         status.Errors,
		ErrorCount:     status.ErrorCount,
		ArtifactsDir:   status.ArtifactsDir,
	}, nil
}
