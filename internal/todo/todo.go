package todo

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Item struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	path string
}

func NewStore(path string) *Store { return &Store{path: path} }

func DefaultStore() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "chalk-tracker", "store.json"), nil
}

func (s *Store) Load() ([]Item, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) Save(item []Item) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) Add(text string) (Item, error) {
	items, err := s.Load()
	if err != nil {
		return Item{}, err
	}

	next := 1
	if len(items) > 0 {
		next = items[len(items)-1].ID + 1
	}
	item := Item{ID: next, Text: text, Done: false, CreatedAt: time.Now()}
	return item, s.Save(append(items, item))
}

// TODO: delete
