package cmd

import (
	"github.com/phpToro/cli/internal/composer"
	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage native plugins",
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <package>",
	Short: "Install a native plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pkg := args[0]
		ui.Info("Installing plugin: " + pkg)

		if err := composer.Require(root, pkg); err != nil {
			return err
		}

		ui.Success("Plugin installed: " + pkg)
		ui.Dim("  Run 'phptoro dev:ios-sim' to regenerate platform folders with new permissions.")
		return nil
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:   "remove <package>",
	Short: "Remove a native plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pkg := args[0]
		ui.Info("Removing plugin: " + pkg)

		if err := composer.Remove(root, pkg); err != nil {
			return err
		}

		ui.Success("Plugin removed: " + pkg)
		return nil
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		// TODO: Read composer.lock, filter phptoro plugins by checking for plugin.json
		ui.Info("No plugins installed")
		return nil
	},
}

func init() {
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginRemoveCmd)
	pluginCmd.AddCommand(pluginListCmd)
	rootCmd.AddCommand(pluginCmd)
}
