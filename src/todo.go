package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// Todo represents a single task.
type Todo struct {
	ID        int        `json:"id"`
	Title     string     `json:"title"`
	Done      bool       `json:"done"`
	Priority  string     `json:"priority"` // low, medium, high
	CreatedAt time.Time  `json:"created_at"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

// Store manages the collection of todos and handles persistence to disk.
type Store struct {
	Path   string  `json:"-"`
	Todos  []*Todo `json:"todos"`
	NextID int     `json:"next_id"`
}

// NewStore creates a store bound to the given file path, loading existing
// data if the file already exists.
func NewStore(path string) (*Store, error) {
	s := &Store{Path: path, NextID: 1}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return s, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading store file: %w", err)
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing store file: %w", err)
	}
	s.Path = path
	return s, nil
}

// Save writes the current state to disk as pretty-printed JSON.
func (s *Store) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding store: %w", err)
	}
	if err := os.WriteFile(s.Path, data, 0644); err != nil {
		return fmt.Errorf("writing store file: %w", err)
	}
	return nil
}

// Add creates a new todo and returns it.
func (s *Store) Add(title, priority string, due *time.Time) *Todo {
	t := &Todo{
		ID:        s.NextID,
		Title:     title,
		Done:      false,
		Priority:  normalizePriority(priority),
		CreatedAt: time.Now(),
		DueDate:   due,
	}
	s.Todos = append(s.Todos, t)
	s.NextID++
	return t
}

// Find returns the todo with the given ID, or nil if not found.
func (s *Store) Find(id int) *Todo {
	for _, t := range s.Todos {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// Remove deletes the todo with the given ID. Returns true if it existed.
func (s *Store) Remove(id int) bool {
	for i, t := range s.Todos {
		if t.ID == id {
			s.Todos = append(s.Todos[:i], s.Todos[i+1:]...)
			return true
		}
	}
	return false
}

// Toggle flips the done state of a todo, stamping DoneAt appropriately.
func (s *Store) Toggle(id int) bool {
	t := s.Find(id)
	if t == nil {
		return false
	}
	t.Done = !t.Done
	if t.Done {
		now := time.Now()
		t.DoneAt = &now
	} else {
		t.DoneAt = nil
	}
	return true
}

// ClearDone removes all completed todos and returns how many were removed.
func (s *Store) ClearDone() int {
	kept := s.Todos[:0]
	removed := 0
	for _, t := range s.Todos {
		if t.Done {
			removed++
			continue
		}
		kept = append(kept, t)
	}
	s.Todos = kept
	return removed
}

// Sorted returns todos ordered by: not-done first, then priority (high->low),
// then due date (soonest first, no-due-date last), then ID.
func (s *Store) Sorted() []*Todo {
	out := make([]*Todo, len(s.Todos))
	copy(out, s.Todos)

	priorityRank := map[string]int{"high": 0, "medium": 1, "low": 2}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Done != b.Done {
			return !a.Done // not-done sorts first
		}
		if priorityRank[a.Priority] != priorityRank[b.Priority] {
			return priorityRank[a.Priority] < priorityRank[b.Priority]
		}
		if (a.DueDate == nil) != (b.DueDate == nil) {
			return a.DueDate != nil // items with a due date come first
		}
		if a.DueDate != nil && b.DueDate != nil && !a.DueDate.Equal(*b.DueDate) {
			return a.DueDate.Before(*b.DueDate)
		}
		return a.ID < b.ID
	})
	return out
}

func normalizePriority(p string) string {
	switch p {
	case "high", "h", "H", "High", "HIGH":
		return "high"
	case "low", "l", "L", "Low", "LOW":
		return "low"
	default:
		return "medium"
	}
}
