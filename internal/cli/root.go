package cli

import (
	todocli "chalk-tracker/internal/cli/todo"
	"chalk-tracker/internal/config"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cfg := &config.Config{}
	rootCmd := &cobra.Command{
		Use:          "todo",
		Short:        "a Todoist-like task manager for the terminal",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := config.Load(cmd.Root().PersistentFlags())
			if err != nil {
				return err
			}
			*cfg = loaded
			return nil
		},
	}
	config.AddFlags(rootCmd.PersistentFlags())
	rootCmd.AddCommand(todocli.NewCommands(cfg)...)
	return rootCmd
}
