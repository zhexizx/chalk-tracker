package todocli

import (
	"chalk-tracker/internal/todo"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "manage todo list",
	}
	cmd.AddCommand(newAddCmd())
	return cmd
}

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   `add "text"`,
		Short: "add a todo item",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.TrimSpace(strings.Join(args, " "))
			if text == "" {
				return fmt.Errorf("text is required")
			}
			path, err := todo.DefaultStore()
			if err != nil {
				return err
			}
			item, err := todo.NewStore(path).Add(text)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added #%d: %s\n", item.ID, item.Text)
			return nil
		},
	}
}
