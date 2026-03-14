package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

var Verbose bool

var rootCmd = &cobra.Command{
	Use:   "phptoro",
	Short: "phpToro — Build native mobile apps with PHP",
	Long:  "phpToro CLI — Develop and build native iOS apps using PHP.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose output")
}
