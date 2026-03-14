package cmd

import (
	"fmt"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/plugin"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage native plugins",
}

var pluginAddCmd = &cobra.Command{
	Use:   "add <repo>",
	Short: "Install a plugin from GitHub (e.g. phpToro/plugin-storage)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		repo := args[0]
		ui.Info("Installing plugin: " + repo)

		if err := plugin.Add(root, repo); err != nil {
			return err
		}

		ui.Success("Plugin installed: " + repo)
		ui.Dim("  Added to phptoro.json — will be included on next build.")
		return nil
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:   "remove <repo>",
	Short: "Remove an installed plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		repo := args[0]
		ui.Info("Removing plugin: " + repo)

		if err := plugin.Remove(root, repo); err != nil {
			return err
		}

		ui.Success("Plugin removed: " + repo)
		return nil
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		plugins, err := plugin.List(root)
		if err != nil {
			return err
		}

		if len(plugins) == 0 {
			ui.Info("No plugins installed")
			ui.Line("")
			ui.Dim("  Install one with: phptoro plugin add phpToro/plugin-storage")
			return nil
		}

		ui.Header("Installed plugins")
		ui.Line("")
		for _, p := range plugins {
			fmt.Printf("  %s (%s)\n", p.Repo, p.Version)
		}
		ui.Line("")
		return nil
	},
}

func init() {
	pluginCmd.AddCommand(pluginAddCmd)
	pluginCmd.AddCommand(pluginRemoveCmd)
	pluginCmd.AddCommand(pluginListCmd)
	rootCmd.AddCommand(pluginCmd)
}
