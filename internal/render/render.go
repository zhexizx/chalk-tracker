// Package render holds the task styles and row layout shared by the full-screen
// interface and the subcommands.
package render

import (
	"fmt"
	"strings"
	"time"

	"chalk-tracker/internal/todo"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	dueWidth = 10
	// fixedWidth covers the cursor, marker, priority and the gaps between columns.
	fixedWidth = 2 + 2 + 3 + 2
	// minMetaWidth is the row width below which the meta column is omitted to
	// leave room for the task text.
	minMetaWidth = 50
)

// ANSI colors follow the user's terminal theme on dark and light backgrounds.
var (
	Red    = lipgloss.Color("1")
	Yellow = lipgloss.Color("3")
	Blue   = lipgloss.Color("4")
	Accent = lipgloss.Color("5")
	Gray   = lipgloss.Color("8")

	Title   = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Heading = lipgloss.NewStyle().Bold(true)
	Muted   = lipgloss.NewStyle().Foreground(Gray)
	Error   = lipgloss.NewStyle().Foreground(Red)
	Cursor  = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Picked  = lipgloss.NewStyle().Bold(true)

	doneStyle    = lipgloss.NewStyle().Faint(true)
	overdueStyle = lipgloss.NewStyle().Bold(true).Foreground(Red)

	priorityStyles = map[int]lipgloss.Style{
		1: lipgloss.NewStyle().Bold(true).Foreground(Red),
		2: lipgloss.NewStyle().Foreground(Yellow),
		3: lipgloss.NewStyle().Foreground(Blue),
		4: Muted,
	}
)

// Columns is the layout of a set of rows, so that they align with each other.
type Columns struct {
	text, meta  int
	showProject bool
}

// FitColumns lays out rows to fit width, truncating long values.
func FitColumns(items []todo.Item, width int, showProject bool) Columns {
	meta := 0
	if width >= minMetaWidth {
		meta = min(widestMeta(items, showProject), width/3)
	}
	return Columns{
		text:        max(1, width-fixedWidth-meta-dueWidth),
		meta:        meta,
		showProject: showProject,
	}
}

// NaturalColumns lays out rows wide enough that nothing is truncated.
func NaturalColumns(items []todo.Item, showProject bool) Columns {
	text := 1
	for _, item := range items {
		text = max(text, lipgloss.Width(item.Text))
	}
	return Columns{text: text, meta: widestMeta(items, showProject), showProject: showProject}
}

func widestMeta(items []todo.Item, showProject bool) int {
	width := 0
	for _, item := range items {
		width = max(width, lipgloss.Width(metaText(item, showProject)))
	}
	return width
}

func metaText(item todo.Item, showProject bool) string {
	var parts []string
	if showProject {
		parts = append(parts, "#"+item.Project)
	}
	for _, label := range item.Labels {
		parts = append(parts, "@"+label)
	}
	return strings.Join(parts, " ")
}

// Row renders one task. cursor must be two columns wide.
func (c Columns) Row(item todo.Item, cursor string, picked bool, now time.Time) string {
	marker := "○"
	if item.Done {
		marker = "●"
	}
	priority := fmt.Sprintf("p%d", item.Priority)

	text := ansi.Truncate(item.Text, c.text, "…")
	text += strings.Repeat(" ", c.text-lipgloss.Width(text))
	meta := ansi.Truncate(metaText(item, c.showProject), c.meta, "…")
	meta = strings.Repeat(" ", c.meta-lipgloss.Width(meta)) + meta
	due := DueLabel(item, now)
	due = strings.Repeat(" ", dueWidth-lipgloss.Width(due)) + due

	if item.Done {
		return cursor + doneStyle.Render(marker+" "+priority+" "+text+" "+meta+" "+due)
	}
	if picked {
		text = Picked.Render(text)
	}
	dueStyle := Muted
	if isOverdue(item, now) {
		dueStyle = overdueStyle
	}
	return cursor + marker + " " + priorityStyles[item.Priority].Render(priority) + " " +
		text + " " + Muted.Render(meta) + " " + dueStyle.Render(due)
}

// TodayGroup returns an item's group heading in the Today view.
func TodayGroup(item todo.Item, now time.Time) string {
	if item.Due < now.Format(todo.DueLayout) {
		return "已逾期"
	}
	return "今天"
}

var weekdays = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// UpcomingGroup returns an item's group heading in the Upcoming view.
func UpcomingGroup(item todo.Item, now time.Time) string {
	date, err := time.ParseInLocation(todo.DueLayout, item.Due, now.Location())
	if err != nil {
		return item.Due
	}
	return shortDate(date, now) + " " + weekdays[date.Weekday()]
}

func shortDate(date, now time.Time) string {
	if date.Year() != now.Year() {
		return date.Format(todo.DueLayout)
	}
	return fmt.Sprintf("%d月%d日", date.Month(), date.Day())
}

// DueLabel returns the text of an item's due date column.
func DueLabel(item todo.Item, now time.Time) string {
	if item.Due == "" {
		return ""
	}
	switch item.Due {
	case now.Format(todo.DueLayout):
		return "今天"
	case now.AddDate(0, 0, 1).Format(todo.DueLayout):
		return "明天"
	}
	if isOverdue(item, now) {
		return "已逾期"
	}
	date, err := time.ParseInLocation(todo.DueLayout, item.Due, now.Location())
	if err != nil {
		return item.Due
	}
	return shortDate(date, now)
}

func isOverdue(item todo.Item, now time.Time) bool {
	return !item.Done && item.Due != "" && item.Due < now.Format(todo.DueLayout)
}
