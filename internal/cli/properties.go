package cli

import (
	"encoding/json"
	"fmt"

	"github.com/davidherring123/godot-cli/internal/core/project"
)

func runProperties(
	rawProperties string,
	call func(*project.Project, map[string]json.RawMessage) (any, error),
) error {
	var properties map[string]json.RawMessage

	if rawProperties != "" {
		if err := json.Unmarshal([]byte(rawProperties), &properties); err != nil {
			return fmt.Errorf("properties must be a JSON object: %w", err)
		}
	}

	project, err := FetchProjectContext()
	if err != nil {
		return err
	}

	result, err := call(project, properties)
	if err != nil {
		return err
	}

	return PrintJSON(result)
}
