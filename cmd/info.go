package cmd

import (
	"fmt"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show project information",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		ui.Header("phpToro Project")
		ui.Line("")
		ui.Table([][]string{
			{"Name:", cfg.App.Name},
			{"Version:", cfg.App.Version},
			{"Bundle ID:", cfg.App.BundleID},
			{"Entry:", cfg.App.Entry},
			{"Orientation:", cfg.Orientation},
			{"Root:", root},
		})

		if cfg.Platforms.IOS != nil {
			ui.Line("")
			ui.Header("iOS")
			rows := [][]string{
				{"Deployment Target:", cfg.Platforms.IOS.DeploymentTarget},
			}
			if cfg.Platforms.IOS.TeamID != "" {
				rows = append(rows, []string{"Team ID:", cfg.Platforms.IOS.TeamID})
			}
			ui.Table(rows)
		}

		if cfg.Platforms.Android != nil {
			ui.Line("")
			ui.Header("Android")
			ui.Table([][]string{
				{"Min SDK:", fmt.Sprintf("%d", cfg.Platforms.Android.MinSdk)},
				{"Target SDK:", fmt.Sprintf("%d", cfg.Platforms.Android.TargetSdk)},
			})
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
