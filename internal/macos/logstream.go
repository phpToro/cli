package macos

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
)

var (
	logPhp     = color.New(color.FgYellow)
	logBridge  = color.New(color.FgCyan)
	logKernel  = color.New(color.FgGreen)
	logScreen  = color.New(color.FgBlue)
	logRender  = color.New(color.FgWhite)
	logErr     = color.New(color.FgRed, color.Bold)
	logTap     = color.New(color.FgHiCyan)
	logDefault = color.New(color.FgHiBlack)
)

// StreamLogs starts streaming macOS app logs filtered to phpToro debug messages.
// Runs in a goroutine. Returns a stop function.
func StreamLogs(bundleID string) func() {
	cmd := exec.Command("/usr/bin/log", "stream",
		"--predicate", `eventMessage CONTAINS "[phpToro"`,
		"--style", "compact",
		"--level", "debug",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ! Failed to start log stream: %v\n", err)
		return func() {}
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "  ! Failed to start log stream: %v\n", err)
		return func() {}
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			formatLogLine(line)
		}
	}()

	return func() {
		cmd.Process.Kill()
		cmd.Wait()
	}
}

func formatLogLine(line string) {
	idx := strings.Index(line, "[phpToro")
	if idx < 0 {
		return
	}
	msg := line[idx:]

	switch {
	case strings.Contains(msg, "[phpToro.PHP]"):
		logPhp.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "ERROR"):
		logErr.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "[phpToro.Kernel]"):
		logKernel.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "[phpToro.Screen]"):
		logScreen.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "[phpToro.Bridge]"):
		logBridge.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "[phpToro.Renderer]"):
		logRender.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "[phpToro.Tap]"):
		logTap.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "[phpToro.PluginHost]"):
		logBridge.Fprintf(os.Stderr, "  %s\n", msg)
	case strings.Contains(msg, "[phpToro.PhpEngine]"):
		logKernel.Fprintf(os.Stderr, "  %s\n", msg)
	default:
		logDefault.Fprintf(os.Stderr, "  %s\n", msg)
	}
}
