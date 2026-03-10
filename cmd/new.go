package cmd

import (
	"github.com/phpToro/cli/internal/project"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var newTemplate string

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new phpToro project",
	Long:  "Create a new phpToro project with the given name.\n\nTemplates: default, tabs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		ui.Header("Creating " + name + "...")
		ui.Line("")

		path, err := project.Scaffold(name, newTemplate)
		if err != nil {
			return err
		}

		ui.Line("")
		ui.Header("Done!")
		ui.Line("")
		ui.Line("  cd " + name)
		ui.Line("  phptoro dev:ios-sim")
		ui.Line("")
		ui.Dim("  Project created at " + path)

		return nil
	},
}

func init() {
	newCmd.Flags().StringVar(&newTemplate, "template", "default", "project template (default, tabs)")
	rootCmd.AddCommand(newCmd)
}
