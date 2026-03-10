package cmd

import (
	"github.com/phpToro/cli/internal/doctor"
	"github.com/phpToro/cli/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check your development environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Header("phpToro Doctor")
		ui.Line("")

		checks := doctor.AllChecks()
		hasError := false

		for _, c := range checks {
			result, err := c.Run()
			if err != nil {
				if c.Required {
					ui.Error(c.Name + ": " + err.Error())
					hasError = true
				} else {
					ui.Warning(c.Name + ": " + err.Error())
				}
			} else {
				ui.Success(c.Name + ": " + result)
			}
		}

		ui.Line("")
		if hasError {
			ui.Error("Some required checks failed")
		} else {
			ui.Success("All good!")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
