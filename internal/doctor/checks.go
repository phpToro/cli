package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Check struct {
	Name     string
	Required bool
	Run      func() (string, error)
}

func AllChecks() []Check {
	return []Check{
		{
			Name:     "PHP runtime",
			Required: true,
			Run: func() (string, error) {
				home, _ := os.UserHomeDir()
				runtimeDir := filepath.Join(home, ".phptoro", "runtimes")
				entries, err := os.ReadDir(runtimeDir)
				if err != nil {
					return "", fmt.Errorf("not found — run 'phptoro new' to download")
				}
				var targets []string
				for _, e := range entries {
					if e.IsDir() {
						targets = append(targets, e.Name())
					}
				}
				if len(targets) == 0 {
					return "", fmt.Errorf("no runtimes installed at %s", runtimeDir)
				}
				return strings.Join(targets, ", "), nil
			},
		},
		{
			Name:     "Composer",
			Required: true,
			Run: func() (string, error) {
				out, err := exec.Command("composer", "--version").Output()
				if err != nil {
					return "", fmt.Errorf("not found — install from https://getcomposer.org")
				}
				return strings.TrimSpace(strings.Split(string(out), "\n")[0]), nil
			},
		},
		{
			Name:     "Xcode",
			Required: runtime.GOOS == "darwin",
			Run: func() (string, error) {
				if runtime.GOOS != "darwin" {
					return "skipped (not macOS)", nil
				}
				out, err := exec.Command("xcodebuild", "-version").Output()
				if err != nil {
					return "", fmt.Errorf("not found — install from App Store")
				}
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				if len(lines) > 0 {
					return lines[0], nil
				}
				return "installed", nil
			},
		},
		{
			Name:     "Go",
			Required: false,
			Run: func() (string, error) {
				out, err := exec.Command("go", "version").Output()
				if err != nil {
					return "", fmt.Errorf("not found (optional)")
				}
				return strings.TrimSpace(string(out)), nil
			},
		},
	}
}
