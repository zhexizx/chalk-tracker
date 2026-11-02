package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chalk-tracker/internal/todo"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var testNow = time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)

type harness struct {
	t     *testing.T
	m     *Model
	store *todo.Store
}

func newHarness(t *testing.T, drafts ...string) *harness {
	t.Helper()
	store := todo.NewStore(filepath.Join(t.TempDir(), "store.json"))
	for _, input := range drafts {
		draft, err := todo.ParseDraft(input, testNow)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Add(draft); err != nil {
			t.Fatal(err)
		}
	}
	m := New(store, "Inbox")
	m.now = func() time.Time { return testNow }
	m.reload()
	h := &harness{t: t, m: m, store: store}
	h.send(tea.WindowSizeMsg{Width: 100, Height: 30})
	return h
}

func (h *harness) send(msg tea.Msg) tea.Cmd {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	return cmd
}

var specialKeys = map[string]tea.KeyPressMsg{
	"enter": {Code: tea.KeyEnter},
	"esc":   {Code: tea.KeyEscape},
	"tab":   {Code: tea.KeyTab},
	"space": {Code: tea.KeySpace, Text: " "},
	"down":  {Code: tea.KeyDown},
	"up":    {Code: tea.KeyUp},
}

// press sends named keys, or types each rune of anything else.
func (h *harness) press(keys ...string) {
	h.t.Helper()
	for _, name := range keys {
		if msg, ok := specialKeys[name]; ok {
			h.send(msg)
			continue
		}
		for _, r := range name {
			h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
	}
}

func (h *harness) texts() []string {
	var result []string
	for _, item := range h.m.tasks {
		result = append(result, item.Text)
	}
	return result
}

func (h *harness) stored() []todo.Item {
	h.t.Helper()
	items, err := h.store.Load()
	if err != nil {
		h.t.Fatal(err)
	}
	return items
}

func (h *harness) selectNav(label string) {
	h.t.Helper()
	h.m.navFocused = true
	for i := 0; i < 20 && h.m.currentNav().label != label; i++ {
		h.press("down")
	}
	if h.m.currentNav().label != label {
		h.t.Fatalf("nav entry %q not found", label)
	}
	h.m.navFocused = false
}

func TestAddUsesCurrentProjectView(t *testing.T) {
	h := newHarness(t, "existing #Work")
	h.selectNav("Work")

	h.press("a", "新任务 @x p1", "enter")

	items := h.stored()
	got := items[len(items)-1]
	if got.Text != "新任务" || got.Project != "Work" || got.Priority != 1 {
		t.Errorf("unexpected item: %+v", got)
	}
	if h.m.mode != modeBrowse {
		t.Error("input should close after saving")
	}
	if selected, _ := h.m.selected(); selected.ID != got.ID {
		t.Errorf("new task not selected: %+v", selected)
	}
}

func TestAddInTodayDefaultsDueToToday(t *testing.T) {
	h := newHarness(t)
	h.selectNav("Today")

	h.press("a", "今天做", "enter")

	if got := h.stored()[0]; got.Due != "2026-11-02" || got.Project != "Inbox" {
		t.Errorf("unexpected item: %+v", got)
	}
	if len(h.m.tasks) != 1 {
		t.Errorf("new task not visible: %v", h.texts())
	}
}

func TestInvalidInputKeepsEditorOpen(t *testing.T) {
	h := newHarness(t)

	h.press("a", "x 2026-02-30", "enter")

	if h.m.mode != modeAdd || !h.m.isError || h.m.input.Value() != "x 2026-02-30" {
		t.Errorf("mode=%v isError=%v value=%q", h.m.mode, h.m.isError, h.m.input.Value())
	}
	if len(h.stored()) != 0 {
		t.Error("nothing should be saved")
	}
}

func TestLettersInEditorAreNotCommands(t *testing.T) {
	h := newHarness(t)

	h.press("a", "qdej k/?", "enter")

	if got := h.stored(); len(got) != 1 || got[0].Text != "qdej k/?" {
		t.Errorf("unexpected items: %+v", got)
	}
}

func TestEscCancelsAdd(t *testing.T) {
	h := newHarness(t)
	h.press("a", "abc", "esc")
	if h.m.mode != modeBrowse || len(h.stored()) != 0 {
		t.Errorf("mode=%v items=%v", h.m.mode, h.stored())
	}
}

func TestEditPrefillsAndKeepsDoneState(t *testing.T) {
	h := newHarness(t, `修复 \p1 #Work @x p2 2026-12-25`)
	h.selectNav("Work")
	h.press("space", "e")

	if got, want := h.m.input.Value(), `修复 \p1 #Work @x p2 2026-12-25`; got != want {
		t.Fatalf("prefill = %q, want %q", got, want)
	}
	h.press(" 更多", "enter")

	got := h.stored()[0]
	if got.Text != "修复 p1 更多" || got.Project != "Work" || got.Priority != 2 || got.Due != "2026-12-25" || !got.Done {
		t.Errorf("unexpected item: %+v", got)
	}
}

func TestToggleLingersInTodayUntilViewChanges(t *testing.T) {
	h := newHarness(t, "a today", "b today")
	h.selectNav("Today")

	h.press("space")
	if len(h.m.tasks) != 2 || !h.m.tasks[0].Done {
		t.Fatalf("completed task should linger: %+v", h.m.tasks)
	}
	if !h.stored()[0].Done {
		t.Error("completion not saved")
	}

	h.m.reload()
	if len(h.m.tasks) != 2 {
		t.Error("reload should keep lingering task")
	}

	h.press("space")
	if h.stored()[0].Done || h.m.tasks[0].Done {
		t.Error("undo failed")
	}

	h.press("space")
	h.m.navFocused = true
	h.press("up", "down")
	if got := h.texts(); len(got) != 1 || got[0] != "b" {
		t.Errorf("after leaving the view: %v", got)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	h := newHarness(t, "a", "b")

	h.press("d", "n")
	if len(h.stored()) != 2 {
		t.Fatal("deleted without confirmation")
	}

	h.press("d", "y")
	if got := h.stored(); len(got) != 1 || got[0].Text != "b" {
		t.Errorf("unexpected items: %+v", got)
	}
}

func TestFilterMatchesTextProjectAndLabelAndClearsOnEsc(t *testing.T) {
	h := newHarness(t, "apple @fruit", "BANANA", "carrot @veg")

	h.press("/", "ban")
	if got := h.texts(); len(got) != 1 || got[0] != "BANANA" {
		t.Errorf("live filter: %v", got)
	}
	h.press("enter")
	if h.m.mode != modeBrowse || len(h.m.tasks) != 1 {
		t.Error("enter should keep the filter")
	}
	h.press("/", "esc")
	if len(h.m.tasks) != 3 {
		t.Errorf("esc should clear the filter: %v", h.texts())
	}
	h.press("/", "@veg", "enter")
	if got := h.texts(); len(got) != 1 || got[0] != "carrot" {
		t.Errorf("label filter: %v", got)
	}
}

func TestReloadShowsExternalChangesAndKeepsSelection(t *testing.T) {
	h := newHarness(t, "a p2", "b p3")
	h.press("down")

	if _, err := h.store.Add(todo.Draft{Text: "external", Priority: 1}); err != nil {
		t.Fatal(err)
	}
	h.send(reloadMsg{})

	if len(h.m.tasks) != 3 {
		t.Fatalf("tasks: %v", h.texts())
	}
	if selected, _ := h.m.selected(); selected.Text != "b" {
		t.Errorf("selection moved to %q", selected.Text)
	}
}

func TestProjectLosingLastTaskFallsBackToInbox(t *testing.T) {
	h := newHarness(t, "keep", "only #Work")
	h.selectNav("Work")

	h.press("d", "y")

	if entry := h.m.currentNav(); entry.label != "Inbox" {
		t.Errorf("view = %q", entry.label)
	}
	if got := h.texts(); len(got) != 1 || got[0] != "keep" {
		t.Errorf("tasks: %v", got)
	}
}

func TestTaskKeysDoNothingWhileNavFocused(t *testing.T) {
	h := newHarness(t, "a")
	h.press("tab", "space", "d", "e")
	if h.stored()[0].Done || h.m.mode != modeBrowse {
		t.Errorf("mode=%v item=%+v", h.m.mode, h.stored()[0])
	}
}

func TestQuit(t *testing.T) {
	for name, msg := range map[string]tea.KeyPressMsg{
		"q":      {Code: 'q', Text: "q"},
		"ctrl+c": {Code: 'c', Mod: tea.ModCtrl},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			cmd := h.send(msg)
			if cmd == nil {
				t.Fatal("no command")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Error("expected quit")
			}
		})
	}
}

func TestLayoutFitsTerminal(t *testing.T) {
	var drafts []string
	for range 60 {
		drafts = append(drafts, "很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长很长的中文任务 #一个很长的项目名称用来测试截断 @标签一 @标签二 p1 today")
	}
	sizes := [][2]int{{60, 10}, {80, 24}, {120, 40}, {200, 60}}
	for _, size := range sizes {
		h := newHarness(t, drafts...)
		h.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, step := range []func(){
			func() {},
			func() { h.selectNav("Today") },
			func() {
				h.press("down", "down", "down", "down", "down", "down", "down", "down", "down", "down", "down", "down")
			},
			func() { h.press("a", "输入内容") },
			func() { h.press("esc", "?") },
		} {
			step()
			lines := strings.Split(h.m.render(), "\n")
			if len(lines) > size[1] {
				t.Errorf("%v: %d lines", size, len(lines))
			}
			for _, line := range lines {
				if w := lipgloss.Width(line); w > size[0] {
					t.Errorf("%v: line width %d: %q", size, w, line)
				}
			}
			if !strings.Contains(h.m.render(), "chalk-tracker") {
				t.Errorf("%v: title scrolled away", size)
			}
		}
	}
}

func TestSelectedTaskStaysVisibleWhenScrolling(t *testing.T) {
	var drafts []string
	for i := range 40 {
		drafts = append(drafts, "task"+string(rune('A'+i)))
	}
	h := newHarness(t, drafts...)
	h.send(tea.WindowSizeMsg{Width: 80, Height: 15})
	for i := range 39 {
		h.press("down")
		if selected, _ := h.m.selected(); !strings.Contains(h.m.render(), selected.Text) {
			t.Fatalf("step %d: %q not on screen", i, selected.Text)
		}
	}
	for i := range 39 {
		h.press("up")
		if selected, _ := h.m.selected(); !strings.Contains(h.m.render(), selected.Text) {
			t.Fatalf("up step %d: %q not on screen", i, selected.Text)
		}
	}
}

func TestSmallTerminalShowsMessage(t *testing.T) {
	h := newHarness(t)
	h.send(tea.WindowSizeMsg{Width: 50, Height: 8})
	if !strings.Contains(h.m.render(), "终端太小") {
		t.Error(h.m.render())
	}
}
