package cli

import (
	"errors"

	"github.com/davidherring123/godot-cli/internal/core/validation"
	"github.com/spf13/cobra"
)

var errValidationFailed = errors.New("validation failed")

func NewValidateCommand() *cobra.Command {
	var flags editorSceneFlags

	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Validate the active editor scene",
		Long:    "Validate the active editor scene, including unsaved changes, through the running editor addon. The command checks script parsing and scene serialization without saving, opening, switching, or running the scene. Use runtime start for gameplay and early-runtime verification.",
		Example: "godot-cli validate\ngodot-cli validate --expect-scene res://main.tscn",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runValidate(flags.expectedScene)
		},
	}

	flags.bind(cmd)

	return cmd
}

func runValidate(expectedScene string) error {
	project, err := FetchProjectContext()
	if err != nil {
		return err
	}

	result, err := validate.Scene(project.Root, validate.SceneParams{
		ExpectedScene: expectedScene,
	})
	if err != nil {
		return err
	}

	if err := PrintJSON(result); err != nil {
		return err
	}

	if !result.Valid {
		return errValidationFailed
	}

	return nil
}
