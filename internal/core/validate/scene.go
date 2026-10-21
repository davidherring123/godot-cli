package validate

import "time"

// Scene validates a saved scene in a fresh headless Godot process. An empty
// scene path uses the project's configured main scene.
func Scene(projectRoot string, params SceneParams) (RunResult, error) {
	var timeout time.Duration

	if params.TimeoutSeconds > 0 {
		timeout = time.Duration(params.TimeoutSeconds * float64(time.Second))
	}

	return Run(projectRoot, RunParams{
		Scene:   params.Scene,
		Timeout: timeout,
	})
}
