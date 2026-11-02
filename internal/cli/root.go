package cli

import (
	todocli "chalk-tracker/internal/cli/todo"
	"chalk-tracker/internal/config"
	"chalk-tracker/internal/todo"
	"chalk-tracker/internal/tui"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewRootCommand() *cobra.Command {
	cfg := &config.Config{}
	rootCmd := &cobra.Command{
		Use:          "todo",
		Short:        "a Todoist-like task manager for the terminal",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
				return cmd.Help()
			}
			return tui.Run(todo.NewStore(cfg.DataFile), cfg.DefaultProject, cfg.Color)
		},
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
