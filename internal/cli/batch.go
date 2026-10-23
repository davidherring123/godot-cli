package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// parseOperations decodes the JSON array accepted by the apply commands. It
// rejects unknown object fields so typos fail before the editor is contacted.
func parseOperations[T any](raw string) ([]T, error) {
	var operations []T

	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&operations); err != nil {
		return nil, fmt.Errorf("operations must be a JSON array: %w", err)
	}

	if operations == nil {
		return nil, fmt.Errorf("operations must be a JSON array")
	}

	return operations, nil
}
