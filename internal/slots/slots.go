// Package slots manages Harpoon-style session slot assignments.
// Slots are a tab-delimited flat file: slot_letter<TAB>session_name.
// All writes are atomic.
package slots

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Slot is a single letter key used to identify a pinned session (h/j/k/l or 1-9).
type Slot = string

// Store manages the slot file.
type Store struct {
	path string
}

// NewStore returns a Store backed by the given file path.
func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating slots dir: %w", err)
	}
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("touching slots file: %w", err)
	}
	fh.Close()
	return &Store{path: path}, nil
}

// All returns the full slot map: slot → session name.
func (s *Store) All() (map[Slot]string, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return map[Slot]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading slots: %w", err)
	}
	m := make(map[Slot]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		m[strings.ToLower(parts[0])] = parts[1]
	}
	return m, nil
}

// Assign pins a session to a slot (case-insensitive slot key).
// Any previous assignment for that slot OR that session is replaced.
func (s *Store) Assign(slot, session string) error {
	slot = strings.ToLower(strings.TrimSpace(slot))
	session = strings.TrimSpace(session)
	if slot == "" || session == "" {
		return fmt.Errorf("slot and session must be non-empty")
	}
	m, err := s.All()
	if err != nil {
		return err
	}
	// Remove previous assignment for the same slot or same session.
	for k, v := range m {
		if k == slot || v == session {
			delete(m, k)
		}
	}
	m[slot] = session
	return s.write(m)
}

// Unassign removes the slot or session from the store.
// target can be a slot letter or a session name.
func (s *Store) Unassign(target string) error {
	target = strings.TrimSpace(target)
	m, err := s.All()
	if err != nil {
		return err
	}
	lower := strings.ToLower(target)
	for k, v := range m {
		if k == lower || v == target {
			delete(m, k)
		}
	}
	return s.write(m)
}

// LookupBySlot returns the session name for the given slot, or "" if unassigned.
func (s *Store) LookupBySlot(slot string) (string, error) {
	m, err := s.All()
	if err != nil {
		return "", err
	}
	return m[strings.ToLower(slot)], nil
}

// LookupBySession returns the slot for the given session name, or "" if unpinned.
func (s *Store) LookupBySession(session string) (string, error) {
	m, err := s.All()
	if err != nil {
		return "", err
	}
	for k, v := range m {
		if v == session {
			return k, nil
		}
	}
	return "", nil
}

// write atomically replaces the slot file.
func (s *Store) write(m map[Slot]string) error {
	tmp := fmt.Sprintf("%s.tmp.%d", s.path, os.Getpid())
	var sb strings.Builder
	for slot, session := range m {
		sb.WriteString(slot)
		sb.WriteByte('\t')
		sb.WriteString(session)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("writing slots tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("renaming slots: %w", err)
	}
	return nil
}
