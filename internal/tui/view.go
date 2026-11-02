package tui

import (
	"fmt"
	"strings"
	"time"

	"chalk-tracker/internal/render"
	"chalk-tracker/internal/todo"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	minWidth, minHeight = 60, 10

	navWidth = 22

	titleHeight, statusHeight, helpHeight, borderSize = 1, 1, 1, 2

	// taskHeaderHeight is the pane title and the blank line below it.
	taskHeaderHeight = 2
)

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "chalk-tracker"
	return v
}

func (m *Model) render() string {
	if m.width == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("终端太小，至少需要 %d×%d", minWidth, minHeight)
	}
	panes := lipgloss.JoinHorizontal(lipgloss.Top, m.renderNav(), m.renderTasks())
	return lipgloss.JoinVertical(lipgloss.Left,
		render.Title.Render(" chalk-tracker"),
		panes,
		m.renderStatus(),
		" "+m.help.View(m.keys),
	)
}

func (m *Model) innerHeight() int {
	return m.height - titleHeight - statusHeight - helpHeight - borderSize
}

func (m *Model) taskWidth() int {
	return m.width - navWidth - borderSize*2
}

// pane draws a bordered box, clipping content that does not fit so that it can
// never break the layout.
func pane(content string, width, height int, focused bool) string {
	lines := strings.Split(content, "\n")
	lines = lines[:min(len(lines), height)]
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	content = strings.Join(lines, "\n")
	border := render.Gray
	if focused {
		border = render.Accent
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(width + borderSize).
		Height(height + borderSize).
		Render(content)
}

func (m *Model) renderNav() string {
	var lines []string
	for i, entry := range m.nav {
		if i == 3 {
			lines = append(lines, "", render.Muted.Render(" 项目"))
		}
		count := fmt.Sprint(m.pendingCount(entry))
		label := ansi.Truncate(entry.label, navWidth-lipgloss.Width(count)-3, "…")
		gap := strings.Repeat(" ", max(1, navWidth-lipgloss.Width(label)-lipgloss.Width(count)-2))
		line := label + gap + render.Muted.Render(count)
		switch {
		case i == m.navCursor && m.navFocused:
			line = render.Cursor.Render("❯") + " " + render.Picked.Render(label) + gap + render.Muted.Render(count)
		case i == m.navCursor:
			line = "› " + render.Picked.Render(label) + gap + render.Muted.Render(count)
		default:
			line = "  " + line
		}
		lines = append(lines, line)
	}
	height := m.innerHeight()
	if len(lines) > height {
		// Keep the selected view on screen when there are many projects.
		lines = lines[max(0, min(m.navCursor+2-height+1, len(lines)-height)):]
		lines = lines[:height]
	}
	return pane(strings.Join(lines, "\n"), navWidth, height, m.navFocused)
}

func (m *Model) pendingCount(entry navEntry) int {
	var items []todo.Item
	now := m.now()
	switch entry.kind {
	case viewToday:
		items = todo.Today(m.items, now)
	case viewUpcoming:
		items = todo.Upcoming(m.items, now)
	default:
		items = todo.Select(m.items, todo.Filter{Project: entry.project})
	}
	count := 0
	for _, item := range items {
		if !item.Done {
			count++
		}
	}
	return count
}

func (m *Model) renderTasks() string {
	width, height := m.taskWidth(), m.innerHeight()
	if m.mode == modeHelp {
		content := render.Heading.Render("快捷键") + "\n\n" + m.help.FullHelpView(m.keys.FullHelp()) +
			"\n\n" + render.Heading.Render("快速语法") +
			"\n买牛奶 #项目 @标签 p1 tomorrow\n\n" + render.Muted.Render("按任意键返回")
		return pane(content, width, height, true)
	}

	entry := m.currentNav()
	heading := render.Heading.Render(entry.label)
	if m.filter != "" {
		heading += render.Muted.Render(" · 筛选: " + m.filter)
	}
	if len(m.tasks) > 0 {
		heading += render.Muted.Render(fmt.Sprintf("  %d/%d", m.taskCursor+1, len(m.tasks)))
	}
	heading = ansi.Truncate(heading, width, "…")

	lines, _ := m.taskLines()
	visible := height - taskHeaderHeight
	end := min(m.taskOffset+visible, len(lines))
	start := min(m.taskOffset, end)
	content := heading + "\n\n" + strings.Join(lines[start:end], "\n")
	return pane(content, width, height, !m.navFocused)
}

// taskLines returns the task list's lines, including group headers, and the
// line of the selected task.
func (m *Model) taskLines() ([]string, int) {
	if len(m.tasks) == 0 {
		message := "这里还没有任务，按 a 添加"
		if m.filter != "" {
			message = "没有匹配的任务"
		}
		return []string{render.Muted.Render(message)}, 0
	}
	entry := m.currentNav()
	now := m.now()
	columns := render.FitColumns(m.tasks, m.taskWidth(), entry.kind != viewProject)
	var lines []string
	cursorLine, group := 0, ""
	for i, item := range m.tasks {
		if g := groupName(entry, item, now); g != group {
			if group != "" {
				lines = append(lines, "")
			}
			lines = append(lines, render.Muted.Render(g))
			group = g
		}
		if i == m.taskCursor {
			cursorLine = len(lines)
		}
		selected := i == m.taskCursor
		lines = append(lines, columns.Row(item, m.taskCursorMark(selected), selected, now))
	}
	return lines, cursorLine
}

func (m *Model) taskCursorMark(selected bool) string {
	switch {
	case !selected:
		return "  "
	case m.navFocused:
		return "› "
	}
	return render.Cursor.Render("❯") + " "
}

func groupName(entry navEntry, item todo.Item, now time.Time) string {
	switch entry.kind {
	case viewToday:
		return render.TodayGroup(item, now)
	case viewUpcoming:
		return render.UpcomingGroup(item, now)
	}
	return ""
}

func (m *Model) renderStatus() string {
	var line string
	switch {
	case m.inputActive():
		m.input.SetWidth(max(1, m.width-lipgloss.Width(m.input.Prompt)-2))
		line = m.input.View()
	case m.mode == modeConfirmDelete:
		item, _ := m.selected()
		line = render.Error.Render(fmt.Sprintf("删除 #%d %s？按 y 确认，其他键取消", item.ID, item.Text))
	case m.isError:
		line = render.Error.Render(m.status)
	default:
		line = render.Muted.Render(m.status)
	}
	return " " + ansi.Truncate(line, m.width-1, "…")
}

// syncScroll keeps the selected task on screen.
func (m *Model) syncScroll() {
	if m.height == 0 {
		return
	}
	lines, cursor := m.taskLines()
	visible := max(1, m.innerHeight()-taskHeaderHeight)
	if cursor < m.taskOffset {
		m.taskOffset = cursor
	}
	if cursor >= m.taskOffset+visible {
		m.taskOffset = cursor - visible + 1
	}
	// Show the group header directly above the first visible task.
	if m.taskCursor == 0 {
		m.taskOffset = 0
	}
	m.taskOffset = clamp(m.taskOffset, 0, max(0, len(lines)-visible))
}
