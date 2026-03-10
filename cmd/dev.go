package cmd

import (
	"fmt"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/runtime"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var devDebug bool

var devIOSSimCmd = &cobra.Command{
	Use:   "dev:ios-sim",
	Short: "Run on iOS Simulator with hot reload",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		ui.Header(fmt.Sprintf("phpToro dev — %s", cfg.App.Name))
		ui.Line("")

		// Ensure runtime is available
		if err := runtime.EnsureRuntime("ios-arm64-sim"); err != nil {
			return err
		}

		// TODO: Build, launch simulator, start file watcher, start WebSocket server
		ui.Info("Dev server coming soon — runtime is ready")
		if devDebug {
			ui.Dim("  Debug mode enabled")
		}

		return nil
	},
}

var devIOSDeviceCmd = &cobra.Command{
	Use:   "dev:ios-device",
	Short: "Run on iOS device with hot reload",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		ui.Header(fmt.Sprintf("phpToro dev — %s", cfg.App.Name))
		ui.Line("")

		if err := runtime.EnsureRuntime("ios-arm64"); err != nil {
			return err
		}

		ui.Info("Dev server coming soon — runtime is ready")
		return nil
	},
}

var devAndroidEmuCmd = &cobra.Command{
	Use:   "dev:android-emu",
	Short: "Run on Android emulator with hot reload",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		ui.Header(fmt.Sprintf("phpToro dev — %s", cfg.App.Name))
		ui.Line("")

		if err := runtime.EnsureRuntime("android-x86_64"); err != nil {
			return err
		}

		ui.Info("Dev server coming soon — runtime is ready")
		return nil
	},
}

func init() {
	devIOSSimCmd.Flags().BoolVar(&devDebug, "debug", false, "enable debug logging")
	devIOSDeviceCmd.Flags().BoolVar(&devDebug, "debug", false, "enable debug logging")
	devAndroidEmuCmd.Flags().BoolVar(&devDebug, "debug", false, "enable debug logging")

	rootCmd.AddCommand(devIOSSimCmd)
	rootCmd.AddCommand(devIOSDeviceCmd)
	rootCmd.AddCommand(devAndroidEmuCmd)
}
