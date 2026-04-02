package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

var printCmd *cobra.Command

func GetPrintCmd() *cobra.Command {
	if printCmd == nil {
		printCmd = &cobra.Command{
			Use:          "print <filename>",
			Aliases:      []string{"p"},
			Args:         cobra.ExactArgs(1),
			SilenceUsage: true,
			RunE:         runPrintCmd,
		}
	}

	return printCmd
}

func runPrintCmd(cmd *cobra.Command, args []string) error {
	f, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	fam, err := rolo.LoadJSON(f)
	if err != nil {
		return err
	}

	for _, f := range fam {
		fmt.Fprintln(cmd.OutOrStdout(), f.Table(0))
	}

	return nil
}
