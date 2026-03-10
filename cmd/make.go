package cmd

import (
	"github.com/phpToro/cli/internal/generator"
	"github.com/spf13/cobra"
)

var makeCmd = &cobra.Command{
	Use:   "make",
	Short: "Generate screens and components",
}

var makeScreenCmd = &cobra.Command{
	Use:   "screen <Name>",
	Short: "Generate a new screen",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return generator.GenerateScreen(args[0])
	},
}

var makeComponentCmd = &cobra.Command{
	Use:   "component <Name>",
	Short: "Generate a new component",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return generator.GenerateComponent(args[0])
	},
}

func init() {
	makeCmd.AddCommand(makeScreenCmd)
	makeCmd.AddCommand(makeComponentCmd)
	rootCmd.AddCommand(makeCmd)
}
