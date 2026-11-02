package todocli

import (
	"bufio"
	"chalk-tracker/internal/config"
	"chalk-tracker/internal/render"
	"chalk-tracker/internal/todo"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const fallbackWidth = 80

// NewCommands returns the task subcommands. cfg is filled in before they run.
func NewCommands(cfg *config.Config) []*cobra.Command {
	return []*cobra.Command{
		newAddCmd(cfg),
		newListCmd(cfg),
		newTodayCmd(cfg),
		newUpcomingCmd(cfg),
		newDoneCmd(cfg),
		newEditCmd(cfg),
		newDeleteCmd(cfg),
	}
}

func newAddCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:     `add "text #project @label p1 today"`,
		Short:   "add a task",
		Example: `  todo add "买牛奶 #生活 @超市 p1 tomorrow"`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			draft, err := todo.ParseDraft(strings.Join(args, " "), time.Now())
			if err != nil {
				return err
			}
			if draft.Project == "" {
				draft.Project = cfg.DefaultProject
			}
			item, err := todo.NewStore(cfg.DataFile).Add(draft)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added #%d: %s (%s)\n", item.ID, item.Text, describe(item))
			return nil
		},
	}
}

func newListCmd(cfg *config.Config) *cobra.Command {
	var filter todo.Filter
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list tasks, grouped by project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if filter.Priority != 0 && (filter.Priority < 1 || filter.Priority > 4) {
				return fmt.Errorf("invalid priority %d: must be 1 to 4", filter.Priority)
			}
			filter.Project = strings.TrimPrefix(filter.Project, "#")
			filter.Label = strings.TrimPrefix(filter.Label, "@")
			items, err := todo.NewStore(cfg.DataFile).Load()
			if err != nil {
				return err
			}
			var shown []todo.Item
			for _, item := range todo.Select(items, filter) {
				if all || !item.Done {
					shown = append(shown, item)
				}
			}
			project := func(item todo.Item) string { return "#" + item.Project }
			if filter.Project != "" {
				project = nil
			}
			return printTasks(cmd, cfg, sortByProject(shown), project, false)
		},
	}
	cmd.Flags().StringVarP(&filter.Project, "project", "p", "", "only tasks in this project")
	cmd.Flags().StringVarP(&filter.Label, "label", "l", "", "only tasks with this label")
	cmd.Flags().IntVar(&filter.Priority, "priority", 0, "only tasks with this priority (1 to 4)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include completed tasks")
	return cmd
}

// sortByProject orders items with the Inbox first and the other projects by
// name, keeping the display order within each project.
func sortByProject(items []todo.Item) []todo.Item {
	var result []todo.Item
	for _, project := range append([]string{todo.DefaultProject}, todo.Projects(items)...) {
		for _, item := range items {
			if item.Project == project {
				result = append(result, item)
			}
		}
	}
	return result
}

func newTodayCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "list tasks due today or overdue",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := todo.NewStore(cfg.DataFile).Load()
			if err != nil {
				return err
			}
			now := time.Now()
			group := func(item todo.Item) string { return render.TodayGroup(item, now) }
			return printTasks(cmd, cfg, todo.Today(items, now), group, true)
		},
	}
}

func newUpcomingCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "upcoming",
		Short: "list tasks due in the next seven days",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := todo.NewStore(cfg.DataFile).Load()
			if err != nil {
				return err
			}
			now := time.Now()
			group := func(item todo.Item) string { return render.UpcomingGroup(item, now) }
			return printTasks(cmd, cfg, todo.Upcoming(items, now), group, true)
		},
	}
}

func newDoneCmd(cfg *config.Config) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "done <id>",
		Short: "mark a task as completed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			item, err := todo.NewStore(cfg.DataFile).SetDone(id, !undo)
			if err != nil {
				return err
			}
			verb := "completed"
			if undo {
				verb = "reopened"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s #%d: %s\n", verb, item.ID, item.Text)
			return nil
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "mark the task as not completed")
	return cmd
}

func newEditCmd(cfg *config.Config) *cobra.Command {
	var noDue, noLabels bool
	cmd := &cobra.Command{
		Use:   `edit <id> ["text #project @label p1 today"]`,
		Short: "change a task; only the given fields change",
		Long: "Change a task using the quick-add syntax. Fields not mentioned keep their\n" +
			"values. Any text replaces the whole text, and any label replaces all labels.",
		Example: `  todo edit 3 p1
  todo edit 3 "新的内容 #工作 tomorrow"
  todo edit 3 --no-due`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if len(args) == 1 && !noDue && !noLabels {
				return errors.New("nothing to change")
			}
			input := strings.Join(args[1:], " ")
			patch, err := todo.ParseDraft(input, time.Now())
			if err != nil {
				return err
			}
			if noDue && patch.Due != "" {
				return errors.New("--no-due conflicts with the due date in the input")
			}
			if noLabels && patch.Labels != nil {
				return errors.New("--no-labels conflicts with the labels in the input")
			}
			item, err := todo.NewStore(cfg.DataFile).Edit(id, func(draft todo.Draft) todo.Draft {
				if patch.Text != "" {
					draft.Text = patch.Text
				}
				if patch.Project != "" {
					draft.Project = patch.Project
				}
				if patch.Labels != nil || noLabels {
					draft.Labels = patch.Labels
				}
				if patch.Priority != 0 {
					draft.Priority = patch.Priority
				}
				if patch.Due != "" || noDue {
					draft.Due = patch.Due
				}
				return draft
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated #%d: %s (%s)\n", item.ID, item.Text, describe(item))
			return nil
		},
	}
	cmd.Flags().BoolVar(&noDue, "no-due", false, "remove the due date")
	cmd.Flags().BoolVar(&noLabels, "no-labels", false, "remove all labels")
	return cmd
}

func newDeleteCmd(cfg *config.Config) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			store := todo.NewStore(cfg.DataFile)
			if !yes {
				confirmed, err := confirmDelete(cmd, store, id)
				if err != nil || !confirmed {
					return err
				}
			}
			if err := store.Delete(id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted #%d\n", id)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation")
	return cmd
}

func confirmDelete(cmd *cobra.Command, store *todo.Store, id int) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, errors.New("refusing to delete without confirmation: use --yes")
	}
	items, err := store.Load()
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.ID == id {
			fmt.Fprintf(cmd.OutOrStdout(), "delete #%d: %s? [y/N] ", id, item.Text)
			answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return false, err
			}
			if strings.TrimSpace(strings.ToLower(answer)) == "y" {
				return true, nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
			return false, nil
		}
	}
	return false, fmt.Errorf("task #%d: %w", id, todo.ErrNotFound)
}

func parseID(arg string) (int, error) {
	id, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid task ID %q", arg)
	}
	return id, nil
}

// printTasks writes rows under group headings. A nil group prints a flat list.
// Output to a terminal fits its width; anywhere else nothing is truncated.
func printTasks(cmd *cobra.Command, cfg *config.Config, items []todo.Item, group func(todo.Item) string, showProject bool) error {
	out := colorprofile.NewWriter(cmd.OutOrStdout(), os.Environ())
	if !cfg.Color {
		out.Profile = min(out.Profile, colorprofile.Ascii)
	}
	if len(items) == 0 {
		_, err := fmt.Fprintln(out, render.Muted.Render("no tasks"))
		return err
	}

	columns := render.NaturalColumns(items, showProject)
	if width, ok := terminalWidth(cmd.OutOrStdout()); ok {
		columns = render.FitColumns(items, width, showProject)
	}
	now := time.Now()
	current := ""
	for _, item := range items {
		if group != nil {
			if heading := group(item); heading != current {
				if current != "" {
					fmt.Fprintln(out)
				}
				fmt.Fprintln(out, render.Heading.Render(heading))
				current = heading
			}
		}
		if _, err := fmt.Fprintf(out, "%s #%d\n", columns.Row(item, "  ", false, now), item.ID); err != nil {
			return err
		}
	}
	return nil
}

func terminalWidth(w io.Writer) (int, bool) {
	file, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return 0, false
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil || width <= 0 {
		width = fallbackWidth
	}
	return width - idColumnWidth, true
}

// idColumnWidth reserves room for the ID that follows each row.
const idColumnWidth = 6

func describe(item todo.Item) string {
	parts := []string{"#" + item.Project}
	for _, label := range item.Labels {
		parts = append(parts, "@"+label)
	}
	parts = append(parts, fmt.Sprintf("p%d", item.Priority))
	if item.Due != "" {
		parts = append(parts, "due "+item.Due)
	}
	return strings.Join(parts, " ")
}
