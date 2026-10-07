package cli

import "github.com/spf13/cobra"

type editorSceneFlags struct {
	expectedScene string
}

func (flags *editorSceneFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flags.expectedScene, "expect-scene", "", "Fail if a different scene is active")
}

type mutationFlags struct {
	editorSceneFlags
	save bool
}

func (flags *mutationFlags) bind(cmd *cobra.Command) {
	flags.editorSceneFlags.bind(cmd)
	cmd.Flags().BoolVar(&flags.save, "save", false, "Save the active scene after applying the change")
}
