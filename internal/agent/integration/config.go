package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadJSONObject(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("reading %q: %w", path, err)
	}

	if strings.HasSuffix(strings.ToLower(path), ".jsonc") {
		data = normalizeJSONC(data)
	}

	var value any

	if err := json.Unmarshal(data, &value); err != nil {
		return nil, true, fmt.Errorf("parsing %q: %w", path, err)
	}

	object, ok := value.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%q must contain a JSON object", path)
	}

	return object, true, nil
}

func saveJSONObject(path string, object map[string]any) (bool, error) {
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encoding %q: %w", path, err)
	}

	return writeFileIfChanged(path, append(data, '\n'))
}

func ensureObject(parent map[string]any, key, path string) (map[string]any, error) {
	value, ok := parent[key]
	if !ok || value == nil {
		child := map[string]any{}
		parent[key] = child

		return child, nil
	}

	child, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q field %q must be a JSON object", path, key)
	}

	return child, nil
}

func jsonCommand(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	command := make([]string, 0, len(items))

	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil
		}

		command = append(command, text)
	}

	return command
}

func jsonString(value any) string {
	text, _ := value.(string)

	return text
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

func writeFileIfChanged(path string, data []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}

	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("reading %q: %w", path, err)
	}

	mode := os.FileMode(0644)

	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("creating %q: %w", filepath.Dir(path), err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".godot-cli-*")
	if err != nil {
		return false, fmt.Errorf("writing %q: %w", path, err)
	}

	tempPath := temp.Name()
	removeTemp := true

	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return false, fmt.Errorf("setting mode on %q: %w", tempPath, err)
	}

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return false, fmt.Errorf("writing %q: %w", path, err)
	}

	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return false, fmt.Errorf("syncing %q: %w", path, err)
	}

	if err := temp.Close(); err != nil {
		return false, fmt.Errorf("closing %q: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return false, fmt.Errorf("replacing %q: %w", path, err)
	}

	removeTemp = false

	return true, nil
}

// normalizeJSONC removes comments and trailing commas while preserving string
// contents, so JSONC configuration can be merged with encoding/json.
func normalizeJSONC(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	withoutComments := make([]byte, 0, len(data))

	inString := false
	escaped := false
	lineComment := false
	blockComment := false

	for index := 0; index < len(data); index++ {
		character := data[index]

		if lineComment {
			if character == '\n' {
				lineComment = false
				withoutComments = append(withoutComments, character)
			}

			continue
		}

		if blockComment {
			if character == '*' && index+1 < len(data) && data[index+1] == '/' {
				blockComment = false
				index++
			} else if character == '\n' {
				withoutComments = append(withoutComments, character)
			}

			continue
		}

		if inString {
			withoutComments = append(withoutComments, character)

			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				inString = false
			}

			continue
		}

		if character == '"' {
			inString = true
			withoutComments = append(withoutComments, character)
			continue
		}

		if character == '/' && index+1 < len(data) {
			switch data[index+1] {
			case '/':
				lineComment = true
				index++
				continue
			case '*':
				blockComment = true
				index++
				continue
			}
		}

		withoutComments = append(withoutComments, character)
	}

	return removeJSONTrailingCommas(withoutComments)
}

func removeJSONTrailingCommas(data []byte) []byte {
	result := make([]byte, 0, len(data))

	inString := false
	escaped := false

	for index := 0; index < len(data); index++ {
		character := data[index]

		if inString {
			result = append(result, character)

			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				inString = false
			}

			continue
		}

		if character == '"' {
			inString = true
			result = append(result, character)
			continue
		}

		if character == ',' {
			next := index + 1

			for next < len(data) && isJSONSpace(data[next]) {
				next++
			}

			if next < len(data) && (data[next] == '}' || data[next] == ']') {
				continue
			}
		}

		result = append(result, character)
	}

	return result
}

func isJSONSpace(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}
