package state

import (
	"fmt"
	"os"
	"sync"
)

type undoEntry struct {
	Path    string
	Content []byte
	Existed bool
}

type UndoStore struct {
	mu      sync.Mutex
	entries map[string]undoEntry
}

func NewUndoStore() *UndoStore {
	return &UndoStore{
		entries: make(map[string]undoEntry),
	}
}

func (u *UndoStore) Save(path string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	entry := undoEntry{Path: path}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			entry.Existed = false
		} else {
			return fmt.Errorf("failed to save undo state for %q: %s", path, err.Error())
		}
	} else {
		entry.Existed = true
		entry.Content = content
	}

	u.entries[path] = entry
	return nil
}

func (u *UndoStore) Restore(path string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	entry, ok := u.entries[path]
	if !ok {
		return fmt.Errorf("no undo history for %q", path)
	}

	if !entry.Existed {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to undo (remove) %q: %s", path, err.Error())
		}
	} else {
		err := os.WriteFile(path, entry.Content, 0644)
		if err != nil {
			return fmt.Errorf("failed to undo (restore) %q: %s", path, err.Error())
		}
	}

	delete(u.entries, path)
	return nil
}
