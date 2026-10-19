package cli

import (
	todocli "chalk-tracker/internal/cli/todo"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "ct",
		Short:        "chalk-tracker: my personal toolbox",
		SilenceUsage: true,
	}
	rootCmd.AddCommand(todocli.NewCmd())
	return rootCmd
}
