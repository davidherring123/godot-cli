package skill

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/davidherring123/godot-mcp/internal/core"
)

// Action describes one canonical MCP action in the generated reference.
type Action struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ReadOnly     bool   `json:"readOnly"`
	Destructive  bool   `json:"destructive"`
	Idempotent   bool   `json:"idempotent"`
	InputSchema  string `json:"inputSchema"`
	OutputSchema string `json:"outputSchema"`
}

// Reference is the machine-readable action reference installed alongside the
// portable skill.
type Reference struct {
	Version string   `json:"version"`
	Actions []Action `json:"actions"`
}

// Generate builds the canonical action reference and JSON schemas directly
// from the core action registry. The host no longer derives references from
// terminal commands, so every registered action is documented with the same
// schemas the MCP server exposes.
func Generate(version string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	reference := Reference{Version: version}
	actions := core.All()

	sort.Slice(actions, func(left, right int) bool {
		return actions[left].Name < actions[right].Name
	})

	for _, action := range actions {
		if action.Input == nil || action.Output == nil {
			return nil, fmt.Errorf("action %q has no input or output schema", action.Name)
		}

		definition := Action{
			Name:         action.Name,
			Description:  action.Description,
			ReadOnly:     action.ReadOnly,
			Destructive:  action.Destructive,
			Idempotent:   action.Idempotent,
			InputSchema:  "schemas/" + action.Name + ".input.json",
			OutputSchema: "schemas/" + action.Name + ".output.json",
		}

		for path, schema := range map[string]any{
			definition.InputSchema:  action.Input,
			definition.OutputSchema: action.Output,
		} {
			data, err := marshal(schema)
			if err != nil {
				return nil, fmt.Errorf("generating %s: %w", path, err)
			}

			files["references/"+path] = data
		}

		reference.Actions = append(reference.Actions, definition)
	}

	data, err := marshal(reference)
	if err != nil {
		return nil, err
	}

	files["references/actions.json"] = data
	files["references/actions.md"] = []byte(render(reference))

	return files, nil
}

func marshal(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(data, '\n'), nil
}

func render(reference Reference) string {
	var text strings.Builder

	text.WriteString("# Godot MCP actions\n\n")
	text.WriteString("Generated from godot-mcp ")

	if reference.Version == "" {
		text.WriteString("dev")
	} else {
		text.WriteString(reference.Version)
	}

	text.WriteString(". Each action is exposed as an MCP tool with dots replaced by underscores. ")
	text.WriteString("Input and output schemas live at `schemas/<action>.input.json` and `.output.json`.\n\n")

	for _, action := range reference.Actions {
		fmt.Fprintf(
			&text,
			"- `%s` — %s\n",
			action.Name,
			firstSentence(action.Description),
		)
	}

	return text.String()
}

func firstSentence(text string) string {
	text = strings.Join(strings.Fields(text), " ")

	if index := strings.Index(text, ". "); index >= 0 {
		return text[:index+1]
	}

	return text
}
