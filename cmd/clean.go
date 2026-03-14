package cmd

import (
	"os"
	"path/filepath"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clear build cache and generated platforms",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		ui.Header("Cleaning...")
		ui.Line("")

		targets := []string{"ios", ".phptoro-cache"}
		for _, t := range targets {
			p := filepath.Join(root, t)
			if _, err := os.Stat(p); err == nil {
				os.RemoveAll(p)
				ui.Success("Removed " + t + "/")
			}
		}

		ui.Line("")
		ui.Success("Clean!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
