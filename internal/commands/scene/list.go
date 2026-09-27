package scene

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/davidherring123/godot-cli/internal/commands"
	"github.com/spf13/cobra"
)

type ListOutput struct {
	Scenes []string `json:"scenes"`
}

func NewListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List scenes in the project",
		Args:  cobra.NoArgs,
		RunE:  runList,
	}
}

func runList(
	cmd *cobra.Command,
	args []string,
) error {
	context, err := commands.FetchProjectContext()
	if err != nil {
		return err
	}

	scenes, err := findScenes(context.Root)
	if err != nil {
		return err
	}

	return commands.PrintJSON(
		ListOutput{
			Scenes: scenes,
		},
	)
}

func findScenes(projectRoot string) ([]string, error) {
	var scenes []string

	err := filepath.WalkDir(
		projectRoot,
		func(
			path string,
			entry fs.DirEntry,
			err error,
		) error {
			if err != nil {
				return err
			}

			if entry.IsDir() && entry.Name() == ".godot" {
				return filepath.SkipDir
			}

			if entry.IsDir() {
				return nil
			}

			if strings.ToLower(filepath.Ext(path)) != ".tscn" {
				return nil
			}

			relativePath, err := filepath.Rel(projectRoot, path)

			if err != nil {
				return err
			}

			scenes = append(
				scenes,
				"res://"+filepath.ToSlash(relativePath),
			)

			return nil
		})

	if err != nil {
		return nil, err
	}

	return scenes, nil
}
