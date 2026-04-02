package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func Execute() {
	err := GetRootCommand().Execute()
	if err != nil {
		os.Exit(1)
	}
}

func GetRootCommand() *cobra.Command {
	if rootCmd == nil {
		rootCmd = &cobra.Command{
			Use:   "wherefolk [command]",
			Short: "wherefolk is a way to manage contact information",
		}

		rootCmd.AddCommand(GetPrintCmd())
	}

	return rootCmd
}
