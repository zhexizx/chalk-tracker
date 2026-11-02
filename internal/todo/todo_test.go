package todo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func newTestStore(t *testing.T, content string) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.json")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return NewStore(path), path
}

const legacyStore = `[
  {"id": 1, "text": "hello", "done": true, "created_at": "2026-10-16T20:15:29+11:00"},
  {"id": 2, "text": "second", "done": false, "created_at": "2026-10-16T20:15:30+11:00"}
]`

func TestAddAppliesDefaults(t *testing.T) {
	store, _ := newTestStore(t, "")

	item, err := store.Add(Draft{Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 1 || item.Project != DefaultProject || item.Priority != DefaultPriority {
		t.Errorf("unexpected item: %+v", item)
	}
	if item.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
}

func TestAddRoundTripsAllFields(t *testing.T) {
	store, _ := newTestStore(t, "")

	added, err := store.Add(Draft{
		Text:     "a",
		Project:  "Work",
		Labels:   []string{"x", "y"},
		Priority: 1,
		Due:      "2026-12-31",
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
	got := items[0]
	if got.Project != "Work" || got.Priority != 1 || got.Due != "2026-12-31" ||
		!reflect.DeepEqual(got.Labels, []string{"x", "y"}) || got.ID != added.ID {
		t.Errorf("unexpected item: %+v", got)
	}
}

func TestAddRejectsInvalidInputWithoutWriting(t *testing.T) {
	cases := map[string]Draft{
		"blank text":        {Text: "  "},
		"priority too high": {Text: "a", Priority: 5},
		"negative priority": {Text: "a", Priority: -1},
		"malformed due":     {Text: "a", Due: "tomorrow"},
		"impossible due":    {Text: "a", Due: "2026-02-30"},
	}
	for name, draft := range cases {
		t.Run(name, func(t *testing.T) {
			store, path := newTestStore(t, "")
			if _, err := store.Add(draft); err == nil {
				t.Error("expected an error")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("store file should not exist: %v", err)
			}
		})
	}
}

func TestLoadLegacyFormatAppliesDefaults(t *testing.T) {
	store, _ := newTestStore(t, legacyStore)

	items, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items", len(items))
	}
	if items[0].Text != "hello" || !items[0].Done {
		t.Errorf("existing fields lost: %+v", items[0])
	}
	if items[0].Project != DefaultProject || items[0].Priority != DefaultPriority {
		t.Errorf("defaults not applied: %+v", items[0])
	}
}

func TestLoadDoesNotRewriteLegacyFile(t *testing.T) {
	store, path := newTestStore(t, legacyStore)

	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != legacyStore {
		t.Error("Load changed the file")
	}
}

func TestAddMigratesLegacyFormat(t *testing.T) {
	store, path := newTestStore(t, legacyStore)

	item, err := store.Add(Draft{Text: "third"})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 3 {
		t.Errorf("ID = %d, want 3", item.ID)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if data[0] != '{' {
		t.Errorf("file not migrated: %s", data)
	}
	items, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || !items[0].Done {
		t.Errorf("items after migration: %+v", items)
	}
}

func TestNextIDIsNotDerivedFromRemainingItems(t *testing.T) {
	store, _ := newTestStore(t, `{"next_id": 9, "items": [{"id": 2, "text": "a"}]}`)

	item, err := store.Add(Draft{Text: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 9 {
		t.Errorf("ID = %d, want 9", item.ID)
	}
}

func TestStaleNextIDDoesNotReuseExistingID(t *testing.T) {
	store, _ := newTestStore(t, `{"next_id": 1, "items": [{"id": 5, "text": "a"}]}`)

	item, err := store.Add(Draft{Text: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 6 {
		t.Errorf("ID = %d, want 6", item.ID)
	}
}

func TestEmptyAndMissingFilesLoadAsEmpty(t *testing.T) {
	for name, content := range map[string]string{"missing": "", "blank": "  \n"} {
		t.Run(name, func(t *testing.T) {
			store, _ := newTestStore(t, content)
			items, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 0 {
				t.Errorf("got %d items", len(items))
			}
		})
	}
}

func TestCorruptFileIsErrorAndIsNotOverwritten(t *testing.T) {
	const corrupt = `{"next_id": `
	store, path := newTestStore(t, corrupt)

	if _, err := store.Add(Draft{Text: "a"}); err == nil {
		t.Error("expected an error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != corrupt {
		t.Error("corrupt file was overwritten")
	}
}

func TestWriteLeavesNoTempFiles(t *testing.T) {
	store, path := newTestStore(t, "")

	if _, err := store.Add(Draft{Text: "a"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	want := []string{"store.json", "store.json.lock"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestUpdateReplacesFieldsAndPreservesIdentity(t *testing.T) {
	store, _ := newTestStore(t, "")
	original, err := store.Add(Draft{Text: "a", Project: "Work", Labels: []string{"x"}, Priority: 1, Due: "2026-12-31"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetDone(original.ID, true); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Update(original.ID, Draft{Text: "b"}); err != nil {
		t.Fatal(err)
	}
	items, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := items[0]
	if got.Text != "b" || got.Project != DefaultProject || got.Priority != DefaultPriority || got.Due != "" || got.Labels != nil {
		t.Errorf("fields not replaced: %+v", got)
	}
	if got.ID != original.ID || !got.Done || !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("identity not preserved: %+v", got)
	}
}

func TestUpdateRejectsInvalidDraft(t *testing.T) {
	store, _ := newTestStore(t, "")
	item, err := store.Add(Draft{Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(item.ID, Draft{Text: "b", Priority: 9}); err == nil {
		t.Error("expected an error")
	}
	items, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Text != "a" {
		t.Errorf("item changed: %+v", items[0])
	}
}

func TestSetDoneTogglesBothWays(t *testing.T) {
	store, _ := newTestStore(t, "")
	item, err := store.Add(Draft{Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []bool{true, false} {
		got, err := store.SetDone(item.ID, want)
		if err != nil {
			t.Fatal(err)
		}
		if got.Done != want {
			t.Errorf("Done = %v, want %v", got.Done, want)
		}
	}
}

func TestDeletedIDIsNeverReused(t *testing.T) {
	store, _ := newTestStore(t, "")
	if _, err := store.Add(Draft{Text: "a"}); err != nil {
		t.Fatal(err)
	}
	last, err := store.Add(Draft{Text: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(last.ID); err != nil {
		t.Fatal(err)
	}
	next, err := store.Add(Draft{Text: "c"})
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != last.ID+1 {
		t.Errorf("ID = %d, want %d", next.ID, last.ID+1)
	}
}

func TestUnknownIDReturnsErrNotFound(t *testing.T) {
	store, _ := newTestStore(t, "")
	if _, err := store.Add(Draft{Text: "a"}); err != nil {
		t.Fatal(err)
	}
	_, updateErr := store.Update(99, Draft{Text: "b"})
	_, doneErr := store.SetDone(99, true)
	deleteErr := store.Delete(99)
	for name, err := range map[string]error{"Update": updateErr, "SetDone": doneErr, "Delete": deleteErr} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

// A long-lived Store must see changes made through another one, as when the TUI
// is open while `todo add` runs in another terminal.
func TestMutationsStartFromCurrentFile(t *testing.T) {
	tui, path := newTestStore(t, "")
	other := NewStore(path)

	first, err := tui.Add(Draft{Text: "from tui"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Add(Draft{Text: "from cli"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tui.SetDone(first.ID, true); err != nil {
		t.Fatal(err)
	}
	items, err := other.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("lost an update: %+v", items)
	}
}

func TestConcurrentAddsLoseNothing(t *testing.T) {
	_, path := newTestStore(t, "")
	const n = 50

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := NewStore(path).Add(Draft{Text: fmt.Sprint(i)})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	items, err := NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, item := range items {
		seen[item.ID] = true
	}
	if len(items) != n || len(seen) != n {
		t.Errorf("got %d items with %d unique IDs, want %d", len(items), len(seen), n)
	}
}

func TestEditChangesOnlyWhatTheCallbackChanges(t *testing.T) {
	store, _ := newTestStore(t, "")
	item, err := store.Add(Draft{Text: "a", Project: "Work", Labels: []string{"x"}, Priority: 2, Due: "2026-12-31"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Edit(item.ID, func(d Draft) Draft {
		d.Priority = 1
		return d
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Priority != 1 || got.Text != "a" || got.Project != "Work" || got.Due != "2026-12-31" || len(got.Labels) != 1 {
		t.Errorf("unexpected item: %+v", got)
	}
}

func TestEditRejectsInvalidResultWithoutWriting(t *testing.T) {
	store, _ := newTestStore(t, "")
	item, err := store.Add(Draft{Text: "a"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Edit(item.ID, func(d Draft) Draft {
		d.Text = " "
		return d
	})
	if err == nil {
		t.Error("expected an error")
	}
	items, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Text != "a" {
		t.Errorf("item changed: %+v", items[0])
	}
}
