package cmd

import (
	"fmt"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/runtime"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build <platform>",
	Short: "Build a release binary",
	Long:  "Build a release binary for the given platform.\n\nPlatforms: ios, android",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := args[0]

		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		ui.Header(fmt.Sprintf("Building %s for %s...", cfg.App.Name, platform))
		ui.Line("")

		switch platform {
		case "ios":
			if err := runtime.EnsureRuntime("ios-arm64"); err != nil {
				return err
			}
		case "android":
			if err := runtime.EnsureRuntime("android-arm64"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown platform %q (use: ios, android)", platform)
		}

		// TODO: Generate platform folder, compile, sign, produce IPA/AAB
		ui.Info("Build pipeline coming soon — runtime is ready")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
