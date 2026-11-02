package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chalk-tracker/internal/todo"
)

type runner struct {
	t    *testing.T
	data string
}

func newRunner(t *testing.T) *runner {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return &runner{t: t, data: filepath.Join(dir, "store.json")}
}

func (r *runner) run(args ...string) (string, error) {
	r.t.Helper()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--data-file", r.data}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func (r *runner) must(args ...string) string {
	r.t.Helper()
	out, err := r.run(args...)
	if err != nil {
		r.t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return out
}

func (r *runner) item(id int) todo.Item {
	r.t.Helper()
	items, err := todo.NewStore(r.data).Load()
	if err != nil {
		r.t.Fatal(err)
	}
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	r.t.Fatalf("no item %d", id)
	return todo.Item{}
}

func assertContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func assertOmits(t *testing.T, out string, unwanted ...string) {
	t.Helper()
	for _, s := range unwanted {
		if strings.Contains(out, s) {
			t.Errorf("output has %q:\n%s", s, out)
		}
	}
}

func TestListGroupsAndFilters(t *testing.T) {
	r := newRunner(t)
	r.must("add", "收件 @x p1")
	r.must("add", "工作任务 #Work @y p2")
	r.must("add", "生活任务 #Home @x")
	r.must("done", "3")

	out := r.must("list")
	assertContains(t, out, "#Inbox", "#Work", "收件", "工作任务", "#1", "#2")
	assertOmits(t, out, "生活任务")
	if strings.Index(out, "#Inbox") > strings.Index(out, "#Work") {
		t.Errorf("Inbox should come first:\n%s", out)
	}

	assertContains(t, r.must("list", "--all"), "生活任务")

	out = r.must("list", "--project", "Work")
	assertContains(t, out, "工作任务")
	assertOmits(t, out, "收件")

	assertContains(t, r.must("list", "-p", "#Work"), "工作任务")

	out = r.must("list", "--all", "--label", "x")
	assertContains(t, out, "收件", "生活任务")
	assertOmits(t, out, "工作任务")

	out = r.must("list", "--priority", "2")
	assertContains(t, out, "工作任务")
	assertOmits(t, out, "收件")

	if _, err := r.run("list", "--priority", "9"); err == nil {
		t.Error("expected an error for priority 9")
	}
}

func TestListEmpty(t *testing.T) {
	r := newRunner(t)
	assertContains(t, r.must("list"), "no tasks")
	assertContains(t, r.must("today"), "no tasks")
	assertContains(t, r.must("upcoming"), "no tasks")
}

func TestOutputToNonTerminalHasNoEscapesAndNoTruncation(t *testing.T) {
	r := newRunner(t)
	long := strings.Repeat("很长的任务", 40)
	r.must("add", long+" p1 today")
	for _, command := range []string{"list", "today"} {
		out := r.must(command)
		assertContains(t, out, long)
		assertOmits(t, out, "\x1b", "…")
	}
}

func TestTodayAndUpcoming(t *testing.T) {
	r := newRunner(t)
	now := time.Now()
	r.must("add", "今天的 today")
	r.must("add", "逾期的 "+now.AddDate(0, 0, -3).Format(todo.DueLayout))
	r.must("add", "明天的 tomorrow")
	r.must("add", "很久以后 "+now.AddDate(0, 0, 30).Format(todo.DueLayout))
	r.must("add", "没日期")

	out := r.must("today")
	assertContains(t, out, "今天的", "逾期的", "已逾期")
	assertOmits(t, out, "明天的", "很久以后", "没日期")
	if strings.Index(out, "逾期的") > strings.Index(out, "今天的") {
		t.Errorf("overdue should come first:\n%s", out)
	}

	out = r.must("upcoming")
	assertContains(t, out, "明天的")
	assertOmits(t, out, "今天的", "逾期的", "很久以后", "没日期")
}

func TestDoneAndUndo(t *testing.T) {
	r := newRunner(t)
	r.must("add", "a")

	assertContains(t, r.must("done", "1"), "completed #1")
	if !r.item(1).Done {
		t.Error("not completed")
	}
	assertContains(t, r.must("done", "--undo", "#1"), "reopened #1")
	if r.item(1).Done {
		t.Error("not reopened")
	}
}

func TestEditChangesOnlyGivenFields(t *testing.T) {
	r := newRunner(t)
	r.must("add", "原文 #Work @x p2 2026-12-25")

	r.must("edit", "1", "p1")
	got := r.item(1)
	if got.Priority != 1 || got.Text != "原文" || got.Project != "Work" || got.Due != "2026-12-25" || len(got.Labels) != 1 {
		t.Errorf("after p1: %+v", got)
	}

	r.must("edit", "1", "新文 #Home @y @z")
	got = r.item(1)
	if got.Text != "新文" || got.Project != "Home" || strings.Join(got.Labels, ",") != "y,z" || got.Priority != 1 {
		t.Errorf("after text: %+v", got)
	}

	r.must("edit", "1", "--no-due", "--no-labels")
	got = r.item(1)
	if got.Due != "" || got.Labels != nil || got.Text != "新文" {
		t.Errorf("after clearing: %+v", got)
	}
}

func TestEditErrors(t *testing.T) {
	r := newRunner(t)
	r.must("add", "a")
	for name, args := range map[string][]string{
		"nothing":       {"edit", "1"},
		"unknown":       {"edit", "9", "p1"},
		"bad date":      {"edit", "1", "2026-02-30"},
		"conflict due":  {"edit", "1", "today", "--no-due"},
		"conflict tags": {"edit", "1", "@x", "--no-labels"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.run(args...); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestDelete(t *testing.T) {
	r := newRunner(t)
	r.must("add", "a")
	r.must("add", "b")

	// The test's stdin is not a terminal, so confirmation cannot be given.
	if _, err := r.run("delete", "2"); err == nil {
		t.Error("expected refusal without --yes")
	}
	assertContains(t, r.must("list"), "b")

	assertContains(t, r.must("delete", "--yes", "2"), "deleted #2")
	assertOmits(t, r.must("list"), "  b")
	assertContains(t, r.must("add", "c"), "added #3")

	if _, err := r.run("delete", "--yes", "99"); err == nil {
		t.Error("expected an error for an unknown ID")
	}
}

func TestInvalidIDs(t *testing.T) {
	r := newRunner(t)
	for _, id := range []string{"abc", "0", "-1", "1.5"} {
		if _, err := r.run("done", "--", id); err == nil {
			t.Errorf("done %q: expected an error", id)
		}
	}
}
