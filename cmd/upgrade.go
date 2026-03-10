package cmd

import (
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the phpToro CLI",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Info("Current version: " + Version)
		// TODO: Check GitHub releases for phpToro/cli, download latest binary, replace self
		ui.Info("Self-upgrade coming soon — reinstall from phptoro.com/install")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}
