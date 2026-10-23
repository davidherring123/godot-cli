package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// parseOperations decodes the JSON array accepted by the apply commands. It
// rejects unknown object fields so typos fail before the editor is contacted,
// and rejects a second JSON value or trailing garbage after the array.
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

	var trailing any

	err := decoder.Decode(&trailing)
	if err == nil {
		return nil, fmt.Errorf("operations must be a single JSON array")
	}

	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf(
			"operations must be a single JSON array: %w",
			err,
		)
	}

	return operations, nil
}
