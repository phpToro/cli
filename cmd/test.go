package cmd

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/phpToro/cli/internal/config"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run tests",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		phpunit := filepath.Join(root, "vendor", "bin", "phpunit")
		if _, err := os.Stat(phpunit); err != nil {
			ui.Warning("PHPUnit not found — install with: composer require --dev phpunit/phpunit")
			return nil
		}

		c := exec.Command(phpunit)
		c.Dir = root
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}
