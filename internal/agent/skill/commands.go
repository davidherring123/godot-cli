package skill

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/davidherring123/godot-cli/internal/core"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const ActionKey = "godot-cli/action"

type Command struct {
	Path         string   `json:"path"`
	Usage        string   `json:"usage"`
	Summary      string   `json:"summary"`
	Description  string   `json:"description,omitempty"`
	Aliases      []string `json:"aliases,omitempty"`
	Deprecated   string   `json:"deprecated,omitempty"`
	Examples     string   `json:"examples,omitempty"`
	Flags        []Flag   `json:"flags,omitempty"`
	Action       string   `json:"action,omitempty"`
	InputSchema  string   `json:"inputSchema,omitempty"`
	OutputSchema string   `json:"outputSchema,omitempty"`
}

type Flag struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Usage     string `json:"usage"`
	Required  bool   `json:"required"`
	Inherited bool   `json:"inherited"`
}

type Reference struct {
	Version  string    `json:"version"`
	Commands []Command `json:"commands"`
}

func Generate(root *cobra.Command) (map[string][]byte, error) {
	files := make(map[string][]byte)
	reference := Reference{Version: root.Version}
	if err := collect(root, &reference, files); err != nil {
		return nil, err
	}
	data, err := marshal(reference)
	if err != nil {
		return nil, err
	}
	files["references/commands.json"] = data
	files["references/commands.md"] = []byte(render(reference))
	return files, nil
}

func collect(cmd *cobra.Command, reference *Reference, files map[string][]byte) error {
	if cmd.Hidden {
		return nil
	}
	if cmd.Parent() != nil && (cmd.Name() == "help" || cmd.Name() == "completion") {
		return nil
	}
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()
	definition := Command{
		Path: cmd.CommandPath(), Usage: cmd.UseLine(), Summary: cmd.Short,
		Description: cmd.Long, Aliases: cmd.Aliases, Deprecated: cmd.Deprecated,
		Examples: cmd.Example, Action: cmd.Annotations[ActionKey],
	}
	addFlags := func(flags *pflag.FlagSet, inherited bool) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden || flag.Deprecated != "" {
				return
			}
			required := flag.Annotations[cobra.BashCompOneRequiredFlag]
			definition.Flags = append(definition.Flags, Flag{
				Name: flag.Name, Shorthand: flag.Shorthand, Type: flag.Value.Type(),
				Default: flag.DefValue, Usage: flag.Usage, Inherited: inherited,
				Required: len(required) > 0 && required[0] == "true",
			})
		})
	}
	addFlags(cmd.LocalFlags(), false)
	addFlags(cmd.InheritedFlags(), true)
	sort.Slice(definition.Flags, func(i, j int) bool {
		return definition.Flags[i].Name < definition.Flags[j].Name
	})
	if definition.Action != "" {
		action, ok := core.Lookup(definition.Action)
		if !ok {
			return fmt.Errorf("command %q references unknown action %q", definition.Path, definition.Action)
		}
		definition.InputSchema = "schemas/" + action.Name + ".input.json"
		definition.OutputSchema = "schemas/" + action.Name + ".output.json"
		for path, schema := range map[string]any{
			definition.InputSchema:  action.Input,
			definition.OutputSchema: action.Output,
		} {
			data, err := marshal(schema)
			if err != nil {
				return fmt.Errorf("generating %s: %w", path, err)
			}
			files["references/"+path] = data
		}
	}
	reference.Commands = append(reference.Commands, definition)
	children := append([]*cobra.Command(nil), cmd.Commands()...)
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	for _, child := range children {
		if err := collect(child, reference, files); err != nil {
			return err
		}
	}
	return nil
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
	fmt.Fprintf(&text, "# Command reference\n\nGenerated from godot-cli %s. Refresh with `godot-cli init --agent codex` or `godot-cli init --agent opencode`.\n\n", reference.Version)
	text.WriteString("Run commands from within a Godot project. Positional arguments appear in usage order; `<name>` is required and `[name]` is optional.\n\n")
	text.WriteString("Schema paths are relative to this reference. Input schemas describe the named parameters supplied by each command; use CLI usage for argument order and flags. Output schemas describe successful JSON results; failures use stderr and a nonzero exit code. Setup, status, and help output are text.\n\n")
	text.WriteString("Machine-readable definitions: [commands.json](commands.json).\n\n## Commands\n\n")
	for _, cmd := range reference.Commands {
		fmt.Fprintf(&text, "- `%s` — %s\n", cmd.Path, cmd.Summary)
	}
	for _, cmd := range reference.Commands {
		fmt.Fprintf(&text, "\n## `%s`\n\n```text\n%s\n```\n\n", cmd.Path, cmd.Usage)
		description := cmd.Description
		if description == "" {
			description = cmd.Summary
		}
		fmt.Fprintf(&text, "%s\n", description)
		if len(cmd.Aliases) > 0 {
			fmt.Fprintf(&text, "\nAliases: %s\n", strings.Join(cmd.Aliases, ", "))
		}
		if cmd.Deprecated != "" {
			fmt.Fprintf(&text, "\nDeprecated: %s\n", cmd.Deprecated)
		}
		if len(cmd.Flags) > 0 {
			text.WriteString("\n### Flags\n\n")
		}
		for _, flag := range cmd.Flags {
			fmt.Fprintf(&text, "- `--%s`", flag.Name)
			if flag.Shorthand != "" {
				fmt.Fprintf(&text, " / `-%s`", flag.Shorthand)
			}
			fmt.Fprintf(&text, " (%s, default: `%s`)", flag.Type, flag.Default)
			if flag.Required {
				text.WriteString(" **required**")
			}
			if flag.Inherited {
				text.WriteString(" (inherited)")
			}
			fmt.Fprintf(&text, ": %s\n", flag.Usage)
		}
		if cmd.Examples != "" {
			fmt.Fprintf(&text, "\n### Examples\n\n```shell\n%s\n```\n", cmd.Examples)
		}
		if cmd.Action != "" {
			fmt.Fprintf(&text, "\n### Schemas\n\n- [Input](%s)\n- [Output](%s)\n", cmd.InputSchema, cmd.OutputSchema)
		}
	}
	return text.String()
}
