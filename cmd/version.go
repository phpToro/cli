package cmd

import (
	"fmt"

	"github.com/phpToro/cli/internal/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI and runtime versions",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("phptoro %s\n", Version)

		root, err := config.FindProjectRoot()
		if err != nil {
			return
		}
		cfg, err := config.Load(root)
		if err != nil {
			return
		}
		fmt.Printf("project: %s v%s\n", cfg.App.Name, cfg.App.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
