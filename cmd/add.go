package cmd

import (
	"github.com/phpToro/cli/internal/composer"
	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <package>",
	Short: "Add a PHP component",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pkg := args[0]
		ui.Info("Adding component: " + pkg)

		if err := composer.Require(root, pkg); err != nil {
			return err
		}

		ui.Success("Component added: " + pkg)
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <package>",
	Short: "Remove a PHP component",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pkg := args[0]
		ui.Info("Removing component: " + pkg)

		if err := composer.Remove(root, pkg); err != nil {
			return err
		}

		ui.Success("Component removed: " + pkg)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
}
