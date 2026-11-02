// Package tui implements the full-screen task interface.
package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"chalk-tracker/internal/todo"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// reloadInterval controls how quickly changes from other processes appear.
const reloadInterval = 2 * time.Second

type viewKind int

const (
	viewProject viewKind = iota
	viewToday
	viewUpcoming
)

type navEntry struct {
	kind    viewKind
	project string
	label   string
}

type mode int

const (
	modeBrowse mode = iota
	modeAdd
	modeEdit
	modeFilter
	modeConfirmDelete
	modeHelp
)

type reloadMsg struct{}

// Model is the Bubble Tea model. Every change goes straight to the store, and
// the model reloads afterwards, so the file is always the source of truth.
type Model struct {
	store          *todo.Store
	defaultProject string
	now            func() time.Time

	items []todo.Item
	nav   []navEntry
	tasks []todo.Item

	navCursor  int
	taskCursor int
	taskOffset int
	navFocused bool

	mode    mode
	filter  string
	editID  int
	status  string
	isError bool

	// lingering holds items completed in the current view. They stay visible
	// in Today and Upcoming so that a mistaken completion can be undone, and
	// are released when the view changes.
	lingering map[int]bool

	input textinput.Model
	help  help.Model
	keys  keyMap

	width, height int
}

// Run starts the interface and blocks until the user quits.
func Run(store *todo.Store, defaultProject string, color bool) error {
	var options []tea.ProgramOption
	if !color {
		options = append(options, tea.WithColorProfile(colorprofile.Ascii))
	}
	_, err := tea.NewProgram(New(store, defaultProject), options...).Run()
	return err
}

func New(store *todo.Store, defaultProject string) *Model {
	input := textinput.New()
	m := &Model{
		store:          store,
		defaultProject: defaultProject,
		now:            time.Now,
		lingering:      map[int]bool{},
		input:          input,
		help:           help.New(),
		keys:           newKeyMap(),
	}
	m.reload()
	return m
}

func (m *Model) Init() tea.Cmd { return scheduleReload() }

func scheduleReload() tea.Cmd {
	return tea.Tick(reloadInterval, func(time.Time) tea.Msg { return reloadMsg{} })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
	case reloadMsg:
		m.reload()
		cmd = scheduleReload()
	case tea.KeyPressMsg:
		cmd = m.handleKey(msg)
	default:
		if m.inputActive() {
			m.input, cmd = m.input.Update(msg)
		}
	}
	m.syncScroll()
	return m, cmd
}

func (m *Model) inputActive() bool {
	return m.mode == modeAdd || m.mode == modeEdit || m.mode == modeFilter
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "ctrl+c" {
		return tea.Quit
	}
	switch m.mode {
	case modeAdd, modeEdit:
		return m.handleEditorKey(msg)
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeConfirmDelete:
		m.handleConfirmKey(msg)
		return nil
	case modeHelp:
		m.mode = modeBrowse
		return nil
	}
	return m.handleBrowseKey(msg)
}

func (m *Model) handleBrowseKey(msg tea.KeyPressMsg) tea.Cmd {
	m.setStatus("", false)
	switch {
	case key.Matches(msg, m.keys.Quit):
		return tea.Quit
	case key.Matches(msg, m.keys.Up):
		m.move(-1)
	case key.Matches(msg, m.keys.Down):
		m.move(1)
	case msg.String() == "left" || msg.String() == "h":
		m.navFocused = true
	case msg.String() == "right" || msg.String() == "l":
		m.navFocused = false
	case key.Matches(msg, m.keys.Switch):
		m.navFocused = !m.navFocused
	case key.Matches(msg, m.keys.Help):
		m.mode = modeHelp
	case key.Matches(msg, m.keys.Cancel):
		m.setFilter("")
	case key.Matches(msg, m.keys.Add):
		m.mode = modeAdd
		return m.openInput("添加 › ", "")
	case key.Matches(msg, m.keys.Filter):
		m.mode = modeFilter
		return m.openInput("筛选 › ", m.filter)
	case msg.String() == "enter" && m.navFocused:
		m.navFocused = false
	case m.navFocused:
		// The remaining keys act on a task.
	case key.Matches(msg, m.keys.Edit):
		if item, ok := m.selected(); ok {
			m.mode, m.editID = modeEdit, item.ID
			return m.openInput("编辑 › ", todo.FormatDraft(item))
		}
	case key.Matches(msg, m.keys.Toggle):
		m.toggleSelected()
	case key.Matches(msg, m.keys.Delete):
		if _, ok := m.selected(); ok {
			m.mode = modeConfirmDelete
		}
	}
	return nil
}

func (m *Model) handleEditorKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.closeInput()
		return nil
	case "enter":
		m.submitEditor()
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *Model) handleFilterKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.setFilter("")
		m.closeInput()
		return nil
	case "enter":
		m.closeInput()
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.setFilter(m.input.Value())
	return cmd
}

func (m *Model) handleConfirmKey(msg tea.KeyPressMsg) {
	m.mode = modeBrowse
	item, ok := m.selected()
	if msg.String() != "y" || !ok {
		m.setStatus("已取消删除", false)
		return
	}
	if err := m.store.Delete(item.ID); err != nil {
		m.setStatus(storeError(err), true)
	} else {
		m.setStatus(fmt.Sprintf("已删除 #%d", item.ID), false)
	}
	m.reload()
}

func (m *Model) openInput(prompt, value string) tea.Cmd {
	m.setStatus("", false)
	m.input.Prompt = prompt
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *Model) closeInput() {
	m.mode = modeBrowse
	m.input.Blur()
	m.input.Reset()
}

// submitEditor saves the add or edit input. On failure the input stays open so
// the text is not lost.
func (m *Model) submitEditor() {
	now := m.now()
	draft, err := todo.ParseDraft(m.input.Value(), now)
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	var item todo.Item
	if m.mode == modeEdit {
		if draft.Project == "" {
			draft.Project = m.defaultProject
		}
		item, err = m.store.Update(m.editID, draft)
	} else {
		m.applyViewDefaults(&draft, now)
		item, err = m.store.Add(draft)
	}
	if err != nil {
		m.setStatus(storeError(err), true)
		return
	}
	verb := "已添加"
	if m.mode == modeEdit {
		verb = "已保存"
	}
	m.closeInput()
	m.setStatus(fmt.Sprintf("%s #%d %s", verb, item.ID, item.Text), false)
	m.reload()
	m.selectID(item.ID)
}

// applyViewDefaults makes a new task appear in the view it was added from.
func (m *Model) applyViewDefaults(draft *todo.Draft, now time.Time) {
	entry := m.currentNav()
	if draft.Project == "" {
		if entry.kind == viewProject {
			draft.Project = entry.project
		} else {
			draft.Project = m.defaultProject
		}
	}
	if draft.Due == "" && entry.kind == viewToday {
		draft.Due = now.Format(todo.DueLayout)
	}
}

func (m *Model) toggleSelected() {
	item, ok := m.selected()
	if !ok {
		return
	}
	updated, err := m.store.SetDone(item.ID, !item.Done)
	if err != nil {
		m.setStatus(storeError(err), true)
		m.reload()
		return
	}
	if updated.Done {
		m.lingering[updated.ID] = true
		m.setStatus(fmt.Sprintf("已完成 #%d，按 space 可撤销", updated.ID), false)
	} else {
		delete(m.lingering, updated.ID)
		m.setStatus(fmt.Sprintf("已恢复 #%d", updated.ID), false)
	}
	m.reload()
	m.selectID(updated.ID)
}

func storeError(err error) string {
	if errors.Is(err, todo.ErrNotFound) {
		return "任务已不存在"
	}
	return err.Error()
}

func (m *Model) setStatus(text string, isError bool) {
	m.status, m.isError = text, isError
}

func (m *Model) setFilter(filter string) {
	m.filter = filter
	id, ok := m.selectedID()
	m.rebuild()
	if ok {
		m.selectID(id)
	}
}

func (m *Model) move(delta int) {
	if !m.navFocused {
		m.taskCursor = clamp(m.taskCursor+delta, 0, len(m.tasks)-1)
		return
	}
	next := clamp(m.navCursor+delta, 0, len(m.nav)-1)
	if next == m.navCursor {
		return
	}
	m.navCursor = next
	m.taskCursor, m.taskOffset = 0, 0
	clear(m.lingering)
	m.rebuild()
}

func (m *Model) selected() (todo.Item, bool) {
	if m.taskCursor < 0 || m.taskCursor >= len(m.tasks) {
		return todo.Item{}, false
	}
	return m.tasks[m.taskCursor], true
}

func (m *Model) selectedID() (int, bool) {
	item, ok := m.selected()
	return item.ID, ok
}

func (m *Model) selectID(id int) {
	if i := slices.IndexFunc(m.tasks, func(item todo.Item) bool { return item.ID == id }); i >= 0 {
		m.taskCursor = i
	}
}

func (m *Model) currentNav() navEntry {
	return m.nav[m.navCursor]
}

// reload rereads the store, keeping the selection on the same view and task.
func (m *Model) reload() {
	items, err := m.store.Load()
	if err != nil {
		m.setStatus(storeError(err), true)
		return
	}
	m.items = items
	id, hadSelection := m.selectedID()
	m.rebuild()
	if hadSelection {
		m.selectID(id)
	}
}

func (m *Model) rebuild() {
	var current navEntry
	if len(m.nav) > 0 {
		current = m.currentNav()
	} else {
		current = navEntry{kind: viewProject, project: todo.DefaultProject}
	}
	m.nav = []navEntry{
		{kind: viewProject, project: todo.DefaultProject, label: "Inbox"},
		{kind: viewToday, label: "Today"},
		{kind: viewUpcoming, label: "Upcoming"},
	}
	for _, project := range todo.Projects(m.items) {
		m.nav = append(m.nav, navEntry{kind: viewProject, project: project, label: project})
	}
	i := slices.IndexFunc(m.nav, func(e navEntry) bool {
		return e.kind == current.kind && e.project == current.project
	})
	if i < 0 {
		// The project lost its last task; fall back to the Inbox.
		i = 0
		m.taskCursor, m.taskOffset = 0, 0
	}
	m.navCursor = i
	m.tasks = m.matchFilter(m.viewItems(m.nav[i]))
	m.taskCursor = clamp(m.taskCursor, 0, len(m.tasks)-1)
}

func (m *Model) viewItems(entry navEntry) []todo.Item {
	now := m.now()
	switch entry.kind {
	case viewToday:
		return m.keepLingering(func(items []todo.Item) []todo.Item { return todo.Today(items, now) })
	case viewUpcoming:
		return m.keepLingering(func(items []todo.Item) []todo.Item { return todo.Upcoming(items, now) })
	}
	return todo.Select(m.items, todo.Filter{Project: entry.project})
}

// keepLingering runs a view that hides completed items as if lingering items
// were unfinished, so they stay in place, and then restores their state.
func (m *Model) keepLingering(view func([]todo.Item) []todo.Item) []todo.Item {
	items := slices.Clone(m.items)
	for i := range items {
		if m.lingering[items[i].ID] && items[i].Done {
			items[i].Done = false
		}
	}
	result := view(items)
	for i := range result {
		if m.lingering[result[i].ID] {
			result[i].Done = true
		}
	}
	return result
}

func (m *Model) matchFilter(items []todo.Item) []todo.Item {
	query := strings.ToLower(strings.TrimSpace(m.filter))
	if query == "" {
		return items
	}
	var result []todo.Item
	for _, item := range items {
		haystack := strings.ToLower(item.Text + " #" + item.Project + " @" + strings.Join(item.Labels, " @"))
		if strings.Contains(haystack, query) {
			result = append(result, item)
		}
	}
	return result
}

func clamp(value, low, high int) int {
	return max(low, min(value, high))
}
