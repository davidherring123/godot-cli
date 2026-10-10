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

	text.WriteString("# godot-cli commands\n\n")
	text.WriteString("Run from within a Godot project. `<name>` is required and `[name]` is optional. ")
	text.WriteString("Input and output schemas live at `schemas/<command.with.dots>.input.json` and `.output.json`.\n\n")

	for _, cmd := range reference.Commands {
		description := cmd.Description
		if description == "" {
			description = cmd.Summary
		}

		fmt.Fprintf(
			&text,
			"- `%s` — %s\n",
			compactSignature(cmd),
			firstSentence(description),
		)
	}

	return text.String()
}

func compactSignature(cmd Command) string {
	signature := strings.TrimSuffix(cmd.Usage, " [flags]")

	for _, flag := range cmd.Flags {
		if flag.Inherited {
			continue
		}

		token := "--" + flag.Name

		if flag.Type != "bool" {
			token += " <value>"

			if flag.Type == "stringArray" || flag.Type == "stringSlice" {
				token += "..."
			}
		}

		if !flag.Required {
			token = "[" + token + "]"
		}

		signature += " " + token
	}

	return signature
}

func firstSentence(text string) string {
	text = strings.Join(strings.Fields(text), " ")

	if index := strings.Index(text, ". "); index >= 0 {
		return text[:index+1]
	}

	return text
}
