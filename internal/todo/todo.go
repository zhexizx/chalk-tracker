package todo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const (
	DefaultProject  = "Inbox"
	DefaultPriority = 4

	// DueLayout is the time layout of Item.Due.
	DueLayout = "2006-01-02"

	lockTimeout    = 5 * time.Second
	lockRetryDelay = 10 * time.Millisecond
)

var ErrNotFound = errors.New("not found")

type Item struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	Project   string    `json:"project"`
	Labels    []string  `json:"labels,omitempty"`
	Priority  int       `json:"priority"`
	Due       string    `json:"due,omitempty"` // YYYY-MM-DD, empty when unset
	CreatedAt time.Time `json:"created_at"`
}

// Draft holds the user-supplied fields of a new item. Zero values fall back to
// DefaultProject and DefaultPriority.
type Draft struct {
	Text     string
	Project  string
	Labels   []string
	Priority int
	Due      string
}

// storeFile is the on-disk format. NextID is stored rather than derived from
// the items so that IDs of deleted items are never reused.
type storeFile struct {
	NextID int    `json:"next_id"`
	Items  []Item `json:"items"`
}

type Store struct {
	path string
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Load() ([]Item, error) {
	file, err := s.read()
	if err != nil {
		return nil, err
	}
	return file.Items, nil
}

func (s *Store) Add(draft Draft) (Item, error) {
	draft, err := normalize(draft)
	if err != nil {
		return Item{}, err
	}
	var item Item
	err = s.mutate(func(file *storeFile) error {
		item = Item{
			ID:        file.NextID,
			Text:      draft.Text,
			Project:   draft.Project,
			Labels:    draft.Labels,
			Priority:  draft.Priority,
			Due:       draft.Due,
			CreatedAt: time.Now(),
		}
		file.Items = append(file.Items, item)
		file.NextID++
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

// Update replaces every user-editable field of an item with the draft's values,
// so callers must pass the fields they want to keep. ID, Done and CreatedAt are
// preserved.
func (s *Store) Update(id int, draft Draft) (Item, error) {
	draft, err := normalize(draft)
	if err != nil {
		return Item{}, err
	}
	return s.modify(id, func(item *Item) {
		item.applyDraft(draft)
	})
}

func (item *Item) applyDraft(draft Draft) {
	item.Text = draft.Text
	item.Project = draft.Project
	item.Labels = draft.Labels
	item.Priority = draft.Priority
	item.Due = draft.Due
}

// Edit changes an item's user-editable fields based on their current values,
// as one locked operation. ID, Done and CreatedAt are preserved.
func (s *Store) Edit(id int, change func(Draft) Draft) (Item, error) {
	var item Item
	err := s.mutate(func(file *storeFile) error {
		i, err := file.index(id)
		if err != nil {
			return err
		}
		current := &file.Items[i]
		draft, err := normalize(change(Draft{
			Text:     current.Text,
			Project:  current.Project,
			Labels:   current.Labels,
			Priority: current.Priority,
			Due:      current.Due,
		}))
		if err != nil {
			return err
		}
		current.applyDraft(draft)
		item = *current
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func (s *Store) SetDone(id int, done bool) (Item, error) {
	return s.modify(id, func(item *Item) { item.Done = done })
}

func (s *Store) Delete(id int) error {
	return s.mutate(func(file *storeFile) error {
		i, err := file.index(id)
		if err != nil {
			return err
		}
		file.Items = slices.Delete(file.Items, i, i+1)
		return nil
	})
}

func (s *Store) modify(id int, change func(*Item)) (Item, error) {
	var item Item
	err := s.mutate(func(file *storeFile) error {
		i, err := file.index(id)
		if err != nil {
			return err
		}
		change(&file.Items[i])
		item = file.Items[i]
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func (f *storeFile) index(id int) (int, error) {
	i := slices.IndexFunc(f.Items, func(item Item) bool { return item.ID == id })
	if i < 0 {
		return 0, fmt.Errorf("task #%d: %w", id, ErrNotFound)
	}
	return i, nil
}

func normalize(draft Draft) (Draft, error) {
	draft.Text = strings.TrimSpace(draft.Text)
	if draft.Text == "" {
		return Draft{}, errors.New("text is required")
	}
	if draft.Project == "" {
		draft.Project = DefaultProject
	}
	if draft.Priority == 0 {
		draft.Priority = DefaultPriority
	}
	if draft.Priority < 1 || draft.Priority > 4 {
		return Draft{}, fmt.Errorf("invalid priority %d: must be 1 to 4", draft.Priority)
	}
	if draft.Due != "" {
		if _, err := time.Parse(DueLayout, draft.Due); err != nil {
			return Draft{}, fmt.Errorf("invalid due date %q: must be YYYY-MM-DD", draft.Due)
		}
	}
	return draft, nil
}

// mutate runs one read-change-write cycle under an exclusive lock, always
// starting from the file's current contents, so a long-running process such as
// the TUI cannot overwrite changes made by another process. The lock lives in a
// separate file because write replaces the store file itself.
func (s *Store) mutate(change func(*storeFile) error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	lock := flock.New(s.path + ".lock")
	locked, err := lock.TryLockContext(ctx, lockRetryDelay)
	if err != nil {
		return fmt.Errorf("lock %s: %w", s.path, err)
	}
	if !locked {
		return fmt.Errorf("lock %s: timed out", s.path)
	}
	defer lock.Unlock()

	file, err := s.read()
	if err != nil {
		return err
	}
	if err := change(&file); err != nil {
		return err
	}
	return s.write(file)
}

// read loads the store, accepting both the current object format and the
// legacy format, a bare array of items. A legacy file is rewritten in the
// current format on the next write.
func (s *Store) read() (storeFile, error) {
	var file storeFile
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return storeFile{}, err
	}
	data = bytes.TrimSpace(data)
	switch {
	case len(data) == 0:
	case data[0] == '[':
		err = json.Unmarshal(data, &file.Items)
	default:
		err = json.Unmarshal(data, &file)
	}
	if err != nil {
		return storeFile{}, fmt.Errorf("parse %s: %w", s.path, err)
	}

	// Never hand out an ID already in use, even if next_id is missing or stale.
	for i := range file.Items {
		item := &file.Items[i]
		if item.Project == "" {
			item.Project = DefaultProject
		}
		if item.Priority == 0 {
			item.Priority = DefaultPriority
		}
		if item.ID >= file.NextID {
			file.NextID = item.ID + 1
		}
	}
	if file.NextID < 1 {
		file.NextID = 1
	}
	return file, nil
}

// write replaces the store atomically, so an interrupted write cannot corrupt
// existing data.
func (s *Store) write(file storeFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
