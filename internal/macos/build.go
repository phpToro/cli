package macos

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/phpToro/cli/internal/ui"
)

// BuildOptions configures the xcodebuild invocation for macOS.
type BuildOptions struct {
	ProjectRoot string
	AppName     string
	Debug       bool
}

// Build runs xcodebuild for the generated macOS project.
func Build(opts BuildOptions) (string, error) {
	projectPath := filepath.Join(opts.ProjectRoot, "macos", opts.AppName+".xcodeproj")

	config := "Release"
	if opts.Debug {
		config = "Debug"
	}

	ui.Dim("  xcodebuild " + opts.AppName + " macOS (" + config + ")...")

	args := []string{
		"-project", projectPath,
		"-scheme", opts.AppName,
		"-configuration", config,
		"-derivedDataPath", filepath.Join(opts.ProjectRoot, "macos", "build"),
		"-arch", "arm64",
		"ONLY_ACTIVE_ARCH=YES",
		"build",
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = filepath.Join(opts.ProjectRoot, "macos")

	output, err := cmd.CombinedOutput()
	if err != nil {
		lines := strings.Split(string(output), "\n")
		start := 0
		if len(lines) > 50 {
			start = len(lines) - 50
		}
		for _, line := range lines[start:] {
			fmt.Fprintln(os.Stderr, line)
		}
		return "", fmt.Errorf("xcodebuild failed: %w", err)
	}

	// Find the .app bundle
	appPath := filepath.Join(
		opts.ProjectRoot, "macos", "build", "Build", "Products",
		config, opts.AppName+".app",
	)
	if _, err := os.Stat(appPath); err != nil {
		return "", fmt.Errorf("built app not found at %s", appPath)
	}

	ui.Dim("  Build succeeded")
	return appPath, nil
}

// LaunchApp opens the built .app bundle using `open`.
func LaunchApp(appPath string) error {
	ui.Dim("  Launching macOS app...")

	cmd := exec.Command("open", appPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launch failed: %w", err)
	}

	ui.Dim("  App launched")
	return nil
}

// LaunchAppDirect runs the app binary directly and streams its stderr for debug logs.
// Returns a stop function to kill the process.
func LaunchAppDirect(appPath string) (func(), error) {
	ui.Dim("  Launching macOS app (direct)...")

	binaryPath := filepath.Join(appPath, "Contents", "MacOS")
	entries, err := os.ReadDir(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("read MacOS dir: %w", err)
	}

	// Find the executable (skip dylibs)
	var execName string
	for _, e := range entries {
		if !e.IsDir() && !strings.HasSuffix(e.Name(), ".dylib") && e.Name() != "__preview.dylib" {
			execName = e.Name()
			break
		}
	}
	if execName == "" {
		return nil, fmt.Errorf("no executable found in %s", binaryPath)
	}

	cmd := exec.Command(filepath.Join(binaryPath, execName))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch failed: %w", err)
	}

	ui.Dim("  App launched (PID " + fmt.Sprintf("%d", cmd.Process.Pid) + ")")

	return func() {
		cmd.Process.Kill()
		cmd.Wait()
	}, nil
}
