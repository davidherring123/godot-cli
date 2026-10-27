package cli

import (
	"encoding/json"
	"io"
	"os"
)

func PrintJSON(value any) error {
	return WriteJSON(os.Stdout, value)
}

func WriteJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	return encoder.Encode(value)
}
