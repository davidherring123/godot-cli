package node

import (
	"encoding/json"

	"github.com/davidherring123/godot-cli/internal/bridge"
	"github.com/davidherring123/godot-cli/internal/commands"
	"github.com/spf13/cobra"
)

type Property struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Hint  *PropertyHint   `json:"hint,omitempty"`
	Value json.RawMessage `json:"value"`
}

type PropertyHint struct {
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
	Types    []string `json:"types,omitempty"`
	Value    string   `json:"value,omitempty"`
	Selected []Layer  `json:"selected,omitempty"`
}

type Layer struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

type InspectResult struct {
	Scene      string     `json:"scene"`
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Properties []Property `json:"properties"`
}

type InspectParams struct {
	Scene string `json:"scene"`
	Node  string `json:"node"`
}

func NewInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <scene> <node>",
		Short: "Inspect a node and its properties",
		Args:  cobra.ExactArgs(2),
		RunE:  runInspect,
	}
}

func runInspect(
	cmd *cobra.Command,
	args []string,
) error {
	context, err := commands.FetchProjectContext()
	if err != nil {
		return err
	}

	client, err := context.ConnectEditor()
	if err != nil {
		return err
	}
	defer client.Close()

	result, err := bridge.Call[
		InspectParams,
		InspectResult,
	](
		client,
		"node.inspect",
		InspectParams{
			Scene: args[0],
			Node:  args[1],
		},
	)

	if err != nil {
		return err
	}

	return commands.PrintJSON(result)
}
