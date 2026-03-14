package ios

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/phpToro/cli/internal/ui"
)

// BuildOptions configures the xcodebuild invocation.
type BuildOptions struct {
	ProjectRoot string
	AppName     string
	SDK         string // "iphonesimulator" or "iphoneos"
	Debug       bool
}

// Build runs xcodebuild for the generated iOS project.
func Build(opts BuildOptions) (string, error) {
	projectPath := filepath.Join(opts.ProjectRoot, "ios", opts.AppName+".xcodeproj")

	config := "Release"
	if opts.Debug {
		config = "Debug"
	}

	ui.Dim("  xcodebuild " + opts.AppName + " (" + config + ")...")

	args := []string{
		"-project", projectPath,
		"-scheme", opts.AppName,
		"-sdk", opts.SDK,
		"-configuration", config,
		"-derivedDataPath", filepath.Join(opts.ProjectRoot, "ios", "build"),
		"-arch", "arm64",
		"ONLY_ACTIVE_ARCH=YES",
		"-allowProvisioningUpdates",
		"build",
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = filepath.Join(opts.ProjectRoot, "ios")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Print last 50 lines of output on failure
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
		opts.ProjectRoot, "ios", "build", "Build", "Products",
		config+"-"+opts.SDK, opts.AppName+".app",
	)
	if _, err := os.Stat(appPath); err != nil {
		return "", fmt.Errorf("built app not found at %s", appPath)
	}

	ui.Dim("  Build succeeded")
	return appPath, nil
}

// SimulatorDevice represents an available iOS simulator.
type SimulatorDevice struct {
	UDID      string `json:"udid"`
	Name      string `json:"name"`
	State     string `json:"state"`
	IsAvailable bool `json:"isAvailable"`
}

// FindSimulator finds a booted simulator or boots one.
func FindSimulator() (string, error) {
	// Check for already booted simulator
	udid, err := findBootedSimulator()
	if err == nil && udid != "" {
		return udid, nil
	}

	// Find an available iPhone simulator
	udid, name, err := findAvailableSimulator()
	if err != nil {
		return "", err
	}

	ui.Dim("  Booting simulator: " + name)
	cmd := exec.Command("xcrun", "simctl", "boot", udid)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to boot simulator: %w", err)
	}

	// Open Simulator.app
	exec.Command("open", "-a", "Simulator").Run()

	return udid, nil
}

func findBootedSimulator() (string, error) {
	cmd := exec.Command("xcrun", "simctl", "list", "devices", "booted", "-j")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var result struct {
		Devices map[string][]SimulatorDevice `json:"devices"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	for _, devices := range result.Devices {
		for _, d := range devices {
			if d.State == "Booted" && d.IsAvailable {
				return d.UDID, nil
			}
		}
	}
	return "", fmt.Errorf("no booted simulator")
}

func findAvailableSimulator() (string, string, error) {
	cmd := exec.Command("xcrun", "simctl", "list", "devices", "available", "-j")
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	var result struct {
		Devices map[string][]SimulatorDevice `json:"devices"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", "", err
	}

	// Prefer iPhone simulators, newest runtime first
	for runtime, devices := range result.Devices {
		if !strings.Contains(runtime, "iOS") {
			continue
		}
		for _, d := range devices {
			if d.IsAvailable && strings.Contains(d.Name, "iPhone") {
				return d.UDID, d.Name, nil
			}
		}
	}

	return "", "", fmt.Errorf("no available iPhone simulator found — install one via Xcode")
}

// InstallAndLaunch installs the .app on a simulator and launches it.
func InstallAndLaunch(udid, appPath, bundleID string) error {
	ui.Dim("  Installing on simulator...")

	cmd := exec.Command("xcrun", "simctl", "install", udid, appPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install failed: %s — %w", string(out), err)
	}

	ui.Dim("  Launching " + bundleID + "...")

	cmd = exec.Command("xcrun", "simctl", "launch", udid, bundleID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launch failed: %s — %w", string(out), err)
	}

	ui.Dim("  App launched")
	return nil
}

// ConnectedDevice represents a physical iOS device.
type ConnectedDevice struct {
	UDID         string // CoreDevice identifier (for devicectl install/launch)
	HardwareUDID string // Hardware UDID (for idevicesyslog)
	Name         string
}

// FindConnectedDevice finds a connected physical iOS device via devicectl.
func FindConnectedDevice() (*ConnectedDevice, error) {
	tmpFile, err := os.CreateTemp("", "devicectl-*.json")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cmd := exec.Command("xcrun", "devicectl", "list", "devices", "--json-output", tmpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("devicectl failed: %s — %w — is a device connected?", string(out), err)
	}

	output, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read devicectl output: %w", err)
	}

	var result struct {
		Result struct {
			Devices []struct {
				Identifier string `json:"identifier"`
				DeviceProperties struct {
					Name          string `json:"name"`
					DeveloperMode string `json:"developerModeStatus"`
				} `json:"deviceProperties"`
				HardwareProperties struct {
					UDID    string `json:"udid"`
					Reality string `json:"reality"`
				} `json:"hardwareProperties"`
				ConnectionProperties struct {
					TransportType string `json:"transportType"`
				} `json:"connectionProperties"`
				VisibilityClass string `json:"visibilityClass"`
			} `json:"devices"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse devicectl output: %w", err)
	}

	for _, d := range result.Result.Devices {
		// Only consider physical devices that are available with developer mode enabled
		if d.VisibilityClass != "default" {
			continue
		}
		if d.HardwareProperties.Reality != "physical" {
			continue
		}
		if d.DeviceProperties.DeveloperMode != "enabled" {
			continue
		}

		return &ConnectedDevice{
			UDID:         d.Identifier,
			HardwareUDID: d.HardwareProperties.UDID,
			Name:         d.DeviceProperties.Name,
		}, nil
	}

	return nil, fmt.Errorf("no connected iOS device found — plug in your iPhone via USB or connect over Wi-Fi")
}

// InstallOnDevice installs the .app on a physical device via devicectl.
func InstallOnDevice(udid, appPath string) error {
	ui.Dim("  Installing on device...")

	cmd := exec.Command("xcrun", "devicectl", "device", "install", "app",
		"--device", udid, appPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("device install failed: %w", err)
	}
	return nil
}

// LaunchOnDevice launches an app on a physical device via devicectl.
func LaunchOnDevice(udid, bundleID string) error {
	ui.Dim("  Launching " + bundleID + " on device...")

	cmd := exec.Command("xcrun", "devicectl", "device", "process", "launch",
		"--device", udid, bundleID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("device launch failed: %w", err)
	}

	ui.Dim("  App launched on device")
	return nil
}

// StreamDeviceLogs starts streaming logs from a physical device.
// Tries idevicesyslog first (USB), falls back to printing a hint.
func StreamDeviceLogs(hwUDID string, bundleID string) func() {
	// Try idevicesyslog (libimobiledevice, requires USB connection + trust)
	cmd := exec.Command("idevicesyslog", "-u", hwUDID)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ui.Warning("  Device log streaming not available")
		ui.Dim("    Install libimobiledevice: brew install libimobiledevice")
		ui.Dim("    Or view logs in Xcode → Window → Devices and Simulators")
		return func() {}
	}
	if err := cmd.Start(); err != nil {
		ui.Warning("  Device log streaming not available")
		ui.Dim("    Ensure device is connected via USB and trusted")
		ui.Dim("    Or view logs in Xcode → Window → Devices and Simulators")
		return func() {}
	}

	processName := bundleName(bundleID)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			// Filter for our process and phpToro messages
			if strings.Contains(line, processName) && strings.Contains(line, "[phpToro") {
				formatLogLine(line)
			}
		}
	}()

	return func() {
		cmd.Process.Kill()
		cmd.Wait()
	}
}

