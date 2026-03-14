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
	"github.com/phpToro/cli/internal/ios"
	"github.com/phpToro/cli/internal/runtime"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var devDebug bool

var devIOSSimCmd = &cobra.Command{
	Use:   "dev:ios-sim",
	Short: "Run on iOS Simulator with hot reload",
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

		// App info
		ui.Table([][]string{
			{"  App:", cfg.App.Name},
			{"  Bundle:", cfg.App.BundleID},
			{"  Version:", cfg.App.Version},
			{"  Platform:", "iOS Simulator (arm64)"},
			{"  Root:", absRoot},
		})
		ui.Line("")

		// 1. Runtime
		start := time.Now()
		target := "ios-arm64-sim"
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
			if err := ios.EnsureNativeLibs(runtimeDir, runtimesSource); err != nil {
				return fmt.Errorf("compile native libs: %w", err)
			}
			ui.Success(fmt.Sprintf("Native libraries compiled (%s)", since(start)))
		}

		// 3. Generate iOS project
		start = time.Now()
		deployTarget := "16.0"
		teamID := ""
		if cfg.Platforms.IOS != nil {
			if cfg.Platforms.IOS.DeploymentTarget != "" {
				deployTarget = cfg.Platforms.IOS.DeploymentTarget
			}
			teamID = cfg.Platforms.IOS.TeamID
		}

		if err := ios.Generate(ios.GenerateOptions{
			ProjectRoot:  root,
			AppName:      appName,
			BundleID:     cfg.App.BundleID,
			TeamID:       teamID,
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
		devConfigPath := filepath.Join(root, "ios", appName, "app", "phptoro_dev.json")
		if err := os.WriteFile(devConfigPath, devConfigJSON, 0644); err != nil {
			return fmt.Errorf("write dev config: %w", err)
		}
		ui.Success(fmt.Sprintf("Project generated (%s)", since(start)))

		// 4. Build
		start = time.Now()
		appPath, err := ios.Build(ios.BuildOptions{
			ProjectRoot: root,
			AppName:     appName,
			SDK:         "iphonesimulator",
			Debug:       true,
		})
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		ui.Success(fmt.Sprintf("Build succeeded (%s)", since(start)))

		// 5. Find or boot simulator
		start = time.Now()
		udid, err := ios.FindSimulator()
		if err != nil {
			return fmt.Errorf("simulator: %w", err)
		}

		// 6. Install and launch
		if err := ios.InstallAndLaunch(udid, appPath, cfg.App.BundleID); err != nil {
			return fmt.Errorf("launch: %w", err)
		}
		ui.Success(fmt.Sprintf("Launched on simulator (%s)", since(start)))

		// 7. Start debug log stream (before hot reload, so we see all messages)
		var stopLogs func()
		if devDebug {
			ui.Line("")
			ui.Info("Debug mode — streaming app logs")
			stopLogs = ios.StreamSimulatorLogs(udid, cfg.App.BundleID)
		}

		// 8. Start hot reload server
		server := devserver.New(root, devserver.DefaultPort)
		if err := server.Start(); err != nil {
			return fmt.Errorf("dev server: %w", err)
		}

		// Final summary
		ui.Line("")
		localIP := devserver.GetLocalIP()
		ui.Success(fmt.Sprintf("Ready in %s", since(totalStart)))
		ui.Line("")
		ui.Table([][]string{
			{"  Local:", fmt.Sprintf("ws://localhost:%d", devserver.DefaultPort)},
			{"  Network:", fmt.Sprintf("ws://%s:%d", localIP, devserver.DefaultPort)},
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
		if stopLogs != nil {
			stopLogs()
		}
		server.Stop()
		return nil
	},
}

var devIOSDeviceCmd = &cobra.Command{
	Use:   "dev:ios-device",
	Short: "Run on iOS device with hot reload",
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
			{"  Platform:", "iOS Device (arm64)"},
			{"  Root:", absRoot},
		})
		ui.Line("")

		// 0. Find connected device first (fail fast)
		device, err := ios.FindConnectedDevice()
		if err != nil {
			return err
		}
		ui.Success(fmt.Sprintf("Found device: %s", device.Name))

		// Check network for hot reload
		localIP := devserver.GetLocalIP()
		if localIP == "localhost" {
			ui.Warning("  No network IP found — hot reload will not work on device")
			ui.Warning("  Connect to the same Wi-Fi as your device for hot reload")
		}

		// 1. Runtime
		start := time.Now()
		target := "ios-arm64"
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
			if err := ios.EnsureNativeLibs(runtimeDir, runtimesSource); err != nil {
				return fmt.Errorf("compile native libs: %w", err)
			}
			ui.Success(fmt.Sprintf("Native libraries compiled (%s)", since(start)))
		}

		// 3. Generate iOS project
		start = time.Now()
		deployTarget := "16.0"
		teamID := ""
		if cfg.Platforms.IOS != nil {
			if cfg.Platforms.IOS.DeploymentTarget != "" {
				deployTarget = cfg.Platforms.IOS.DeploymentTarget
			}
			teamID = cfg.Platforms.IOS.TeamID
		}

		if teamID == "" {
			return fmt.Errorf("teamId required for device builds — add platforms.ios.teamId to phptoro.json")
		}

		if err := ios.Generate(ios.GenerateOptions{
			ProjectRoot:  root,
			AppName:      appName,
			BundleID:     cfg.App.BundleID,
			TeamID:       teamID,
			DeployTarget: deployTarget,
			RuntimeDir:   runtimeDir,
		}); err != nil {
			return fmt.Errorf("generate: %w", err)
		}

		// 3b. Inject dev server config (use network IP for device)
		hotReloadHost := localIP
		devConfig := map[string]interface{}{
			"projectRoot": absRoot,
			"debug":       devDebug,
			"hotReload": map[string]interface{}{
				"host": hotReloadHost,
				"port": devserver.DefaultPort,
			},
		}
		devConfigJSON, _ := json.MarshalIndent(devConfig, "", "  ")
		devConfigPath := filepath.Join(root, "ios", appName, "app", "phptoro_dev.json")
		if err := os.WriteFile(devConfigPath, devConfigJSON, 0644); err != nil {
			return fmt.Errorf("write dev config: %w", err)
		}
		ui.Success(fmt.Sprintf("Project generated (%s)", since(start)))

		// 4. Build for device
		start = time.Now()
		appPath, err := ios.Build(ios.BuildOptions{
			ProjectRoot: root,
			AppName:     appName,
			SDK:         "iphoneos",
			Debug:       true,
		})
		if err != nil {
			return fmt.Errorf("build: %w", err)
		}
		ui.Success(fmt.Sprintf("Build succeeded (%s)", since(start)))

		// 5. Install on device
		start = time.Now()
		if err := ios.InstallOnDevice(device.UDID, appPath); err != nil {
			return err
		}

		// 6. Launch on device
		if err := ios.LaunchOnDevice(device.UDID, cfg.App.BundleID); err != nil {
			return err
		}
		ui.Success(fmt.Sprintf("Launched on device (%s)", since(start)))

		// 7. Start debug log stream
		var stopLogs func()
		if devDebug {
			ui.Line("")
			ui.Info("Debug mode — streaming device logs")
			stopLogs = ios.StreamDeviceLogs(device.HardwareUDID, cfg.App.BundleID)
		}

		// 8. Start hot reload server
		server := devserver.New(root, devserver.DefaultPort)
		if err := server.Start(); err != nil {
			return fmt.Errorf("dev server: %w", err)
		}

		// Final summary
		ui.Line("")
		ui.Success(fmt.Sprintf("Ready in %s", since(totalStart)))
		ui.Line("")
		ui.Table([][]string{
			{"  Device:", device.Name},
			{"  Local:", fmt.Sprintf("ws://localhost:%d", devserver.DefaultPort)},
			{"  Network:", fmt.Sprintf("ws://%s:%d", localIP, devserver.DefaultPort)},
		})
		ui.Line("")
		if devDebug {
			ui.Dim("  Debug logs streaming... press Ctrl+C to stop")
		} else {
			ui.Dim("  Watching for changes... press Ctrl+C to stop")
			ui.Dim("  Tip: use --debug for detailed device logs")
		}
		ui.Line("")

		// Wait for interrupt
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		ui.Line("")
		ui.Dim("  Shutting down...")
		if stopLogs != nil {
			stopLogs()
		}
		server.Stop()
		return nil
	},
}

func init() {
	devIOSSimCmd.Flags().BoolVar(&devDebug, "debug", false, "enable debug logging")
	devIOSDeviceCmd.Flags().BoolVar(&devDebug, "debug", false, "enable debug logging")

	rootCmd.AddCommand(devIOSSimCmd)
	rootCmd.AddCommand(devIOSDeviceCmd)
}

func since(t time.Time) string {
	d := time.Since(t)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// sanitizeAppName removes spaces and special chars for Xcode target name.
func sanitizeAppName(name string) string {
	var result []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "PhpToroApp"
	}
	return string(result)
}

// findRuntimesSource looks for the runtimes source directory.
// Checks PHPTORO_RUNTIMES_SOURCE env var first, then common monorepo locations.
func findRuntimesSource(projectRoot string) string {
	// Check env var first
	if src := os.Getenv("PHPTORO_RUNTIMES_SOURCE"); src != "" {
		if info, err := os.Stat(filepath.Join(src, "ext")); err == nil && info.IsDir() {
			return src
		}
	}

	// Check common monorepo locations
	candidates := []string{
		filepath.Join(projectRoot, "..", "runtimes"),       // sibling
		filepath.Join(projectRoot, "..", "..", "runtimes"), // two levels up
	}

	for _, c := range candidates {
		extDir := filepath.Join(c, "ext")
		if info, err := os.Stat(extDir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	return ""
}
