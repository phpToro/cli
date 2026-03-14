package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/devserver"
	"github.com/phpToro/cli/internal/macos"
	"github.com/phpToro/cli/internal/runtime"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var devMacOSCmd = &cobra.Command{
	Use:   "dev:macos",
	Short: "Run on macOS with hot reload",
	RunE: func(cmd *cobra.Command, args []string) error {
		totalStart := time.Now()

		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		cfg, err := config.Load(root)
		if err != nil {
			return err
		}

		appName := sanitizeAppName(cfg.App.Name)
		absRoot, _ := filepath.Abs(root)

		// Header
		ui.Line("")
		ui.Header("  phpToro dev")
		ui.Line("")

		ui.Table([][]string{
			{"  App:", cfg.App.Name},
			{"  Bundle:", cfg.App.BundleID},
			{"  Version:", cfg.App.Version},
			{"  Platform:", "macOS (arm64)"},
			{"  Root:", absRoot},
		})
		ui.Line("")

		// 1. Runtime
		start := time.Now()
		target := "macos-arm64"
		if err := runtime.EnsureRuntime(target); err != nil {
			return err
		}
		runtimeDir := runtime.FindRuntimeDir(target)
		if runtimeDir == "" {
			return fmt.Errorf("runtime directory not found for %s", target)
		}
		ui.Success(fmt.Sprintf("Runtime ready (%s)", since(start)))

		// 2. Compile native libraries
		runtimesSource := findRuntimesSource(root)
		if runtimesSource != "" {
			start = time.Now()
			if err := macos.EnsureNativeLibs(runtimeDir, runtimesSource); err != nil {
				return fmt.Errorf("compile native libs: %w", err)
			}
			ui.Success(fmt.Sprintf("Native libraries compiled (%s)", since(start)))
		}

		// 3. Generate macOS project
		start = time.Now()
		deployTarget := "12.0"
		if cfg.Platforms.MacOS != nil {
			if cfg.Platforms.MacOS.DeploymentTarget != "" {
				deployTarget = cfg.Platforms.MacOS.DeploymentTarget
			}
		}

		if err := macos.Generate(macos.GenerateOptions{
			ProjectRoot:  root,
			AppName:      appName,
			BundleID:     cfg.App.BundleID,
			DeployTarget: deployTarget,
			RuntimeDir:   runtimeDir,
		}); err != nil {
			return fmt.Errorf("generate: %w", err)
		}

		// 3b. Inject dev server config
		devConfig := map[string]interface{}{
			"projectRoot": absRoot,
			"debug":       devDebug,
			"hotReload": map[string]interface{}{
				"host": "localhost",
				"port": devserver.DefaultPort,
			},
		}
		devConfigJSON, _ := json.MarshalIndent(devConfig, "", "  ")
		devConfigPath := filepath.Join(root, "macos", appName, "app", "phptoro_dev.json")
		if err := os.WriteFile(devConfigPath, devConfigJSON, 0644); err != nil {
			return fmt.Errorf("write dev config: %w", err)
		}
		ui.Success(fmt.Sprintf("Project generated (%s)", since(start)))

		// 4. Build
		start = time.Now()
		appPath, err := macos.Build(macos.BuildOptions{
			ProjectRoot: root,
			AppName:     appName,
			Debug:       true,
		})
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		ui.Success(fmt.Sprintf("Build succeeded (%s)", since(start)))

		// 5. Launch
		start = time.Now()
		var stopApp func()
		if devDebug {
			// Launch directly to capture stderr (NSLog output)
			stop, err := macos.LaunchAppDirect(appPath)
			if err != nil {
				return fmt.Errorf("launch: %w", err)
			}
			stopApp = stop
		} else {
			if err := macos.LaunchApp(appPath); err != nil {
				return fmt.Errorf("launch: %w", err)
			}
		}
		ui.Success(fmt.Sprintf("Launched macOS app (%s)", since(start)))

		// 7. Start hot reload server
		server := devserver.New(root, devserver.DefaultPort)
		if err := server.Start(); err != nil {
			return fmt.Errorf("dev server: %w", err)
		}

		// Final summary
		ui.Line("")
		ui.Success(fmt.Sprintf("Ready in %s", since(totalStart)))
		ui.Line("")
		ui.Table([][]string{
			{"  Local:", fmt.Sprintf("ws://localhost:%d", devserver.DefaultPort)},
		})
		ui.Line("")
		if devDebug {
			ui.Dim("  Debug logs streaming... press Ctrl+C to stop")
		} else {
			ui.Dim("  Watching for changes... press Ctrl+C to stop")
			ui.Dim("  Tip: use --debug for detailed app logs")
		}
		ui.Line("")

		// Wait for interrupt
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		ui.Line("")
		ui.Dim("  Shutting down...")
		if stopApp != nil {
			stopApp()
		}
		server.Stop()
		return nil
	},
}

func init() {
	devMacOSCmd.Flags().BoolVar(&devDebug, "debug", false, "enable debug logging")
	rootCmd.AddCommand(devMacOSCmd)
}
