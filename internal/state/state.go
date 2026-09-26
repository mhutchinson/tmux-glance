// Package state manages the tmux-glance state file: a tab-delimited flat
// file of tracked pane entries, with atomic writes and directory-based locking
// compatible with the existing bash mkdir-lock protocol.
package state

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Kind distinguishes manually-watched vigils from auto-detected agents.
type Kind string

const (
	KindManual Kind = "manual"
	KindAuto   Kind = "auto"
)

// PaneState is the current activity state of a tracked pane.
type PaneState string

const (
	StateWaiting  PaneState = "waiting"
	StateRunning  PaneState = "running"
	StateDone     PaneState = "done"
	StateWatching PaneState = "watching"
	StateAlert    PaneState = "alert"
	StateIdle     PaneState = "idle"
	StateUnknown  PaneState = "unknown"
)

// Entry represents a single tracked pane in the state file.
type Entry struct {
	PaneID  string
	Session string
	Window  int
	Pane    int
	Path    string
	Command string
	Label   string
	Kind    Kind
	State   PaneState
}

// format serialises an Entry to the tab-delimited wire format.
func (e Entry) format() string {
	return strings.Join([]string{
		e.PaneID,
		e.Session,
		strconv.Itoa(e.Window),
		strconv.Itoa(e.Pane),
		e.Path,
		e.Command,
		e.Label,
		string(e.Kind),
		string(e.State),
	}, "\t")
}

// parseLine parses one tab-delimited line into an Entry.
// Returns false if the line is malformed or empty.
func parseLine(line string) (Entry, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Entry{}, false
	}
	parts := strings.Split(line, "\t")
	if len(parts) < 9 {
		return Entry{}, false
	}
	win, _ := strconv.Atoi(parts[2])
	pane, _ := strconv.Atoi(parts[3])
	return Entry{
		PaneID:  parts[0],
		Session: parts[1],
		Window:  win,
		Pane:    pane,
		Path:    parts[4],
		Command: parts[5],
		Label:   parts[6],
		Kind:    Kind(parts[7]),
		State:   PaneState(parts[8]),
	}, true
}

// FileStore implements the state store using the tab-delimited flat file.
type FileStore struct {
	path string
	mu   sync.Mutex // guards all in-process reads/writes
}

// NewFileStore returns a FileStore backed by the given file path.
// The file and its parent directories are created if absent.
func NewFileStore(path string) (*FileStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating state dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("touching state file: %w", err)
	}
	f.Close()
	return &FileStore{path: path}, nil
}

// ReadAll returns all entries in the state file.
func (s *FileStore) ReadAll() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAll()
}

// readAll is the unlocked internal reader.
func (s *FileStore) readAll() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading state file: %w", err)
	}
	var entries []Entry
	for _, line := range strings.Split(string(data), "\n") {
		if e, ok := parseLine(line); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// Write atomically replaces the state file with the given entries.
func (s *FileStore) Write(entries []Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(entries)
}

// write is the unlocked internal writer.
func (s *FileStore) write(entries []Entry) error {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.format())
		sb.WriteByte('\n')
	}
	// Use os.CreateTemp for a unique random suffix rather than a PID-based
	// name, eliminating the theoretical collision if the same process issues
	// two concurrent writes (e.g. under test with -race).
	f, err := os.CreateTemp(filepath.Dir(s.path), filepath.Base(s.path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("creating state tmp: %w", err)
	}
	tmp := f.Name()
	if _, err := f.WriteString(sb.String()); err != nil {
		f.Close()
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("writing state tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("closing state tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("atomic rename state: %w", err)
	}
	return nil
}

// Prune removes entries for dead panes (not in alivePaneIDs).
func (s *FileStore) Prune(alivePaneIDs []string) error {
	alive := make(map[string]bool, len(alivePaneIDs))
	for _, id := range alivePaneIDs {
		alive[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return err
	}
	filtered := entries[:0]
	for _, e := range entries {
		if alive[e.PaneID] {
			filtered = append(filtered, e)
		}
	}
	return s.write(filtered)
}

// UpsertEntry adds a new entry or replaces the existing entry for its PaneID.
func (s *FileStore) UpsertEntry(e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return err
	}
	replaced := false
	for i, existing := range entries {
		if existing.PaneID == e.PaneID {
			entries[i] = e
			replaced = true
			break
		}
	}
	if !replaced {
		entries = append(entries, e)
	}
	return s.write(entries)
}

// RemoveEntry deletes the entry for the given pane ID if present.
func (s *FileStore) RemoveEntry(paneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return err
	}
	filtered := entries[:0]
	for _, e := range entries {
		if e.PaneID != paneID {
			filtered = append(filtered, e)
		}
	}
	return s.write(filtered)
}

// UpdateState changes only the State field of an existing entry.
func (s *FileStore) UpdateState(paneID string, st PaneState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.PaneID == paneID {
			entries[i].State = st
			break
		}
	}
	return s.write(entries)
}

// UpdateEntry applies live tmux metadata (session, window, pane, path, command)
// to an existing entry and refreshes its label ("<cmd> in <dir>").
// This is called on each scan pass to fix Issue #5.
func (s *FileStore) UpdateEntry(paneID, session string, window, pane int, path, command string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.PaneID != paneID {
			continue
		}
		entries[i].Session = session
		entries[i].Window = window
		entries[i].Pane = pane
		entries[i].Path = path
		entries[i].Command = command
		entries[i].Label = command + " in " + filepath.Base(path)
		break
	}
	return s.write(entries)
}

// GetEntry returns the Entry for the given pane ID, or false if not found.
func (s *FileStore) GetEntry(paneID string) (Entry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return Entry{}, false, err
	}
	for _, e := range entries {
		if e.PaneID == paneID {
			return e, true, nil
		}
	}
	return Entry{}, false, nil
}

// Lock implements directory-based locking compatible with the existing bash
// mkdir-lock protocol used by the shell script.
type Lock struct {
	dir string
}

// NewLock returns a Lock whose lock directory is stateFilePath + ".lock".
func NewLock(stateFilePath string) *Lock {
	return &Lock{dir: stateFilePath + ".lock"}
}

// Acquire spins until the lock directory can be created.
// Breaks stale locks after ~1 second (50 attempts × 20ms).
// Honors ctx cancellation.
func (l *Lock) Acquire(ctx context.Context) error {
	const maxAttempts = 50
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquiring lock %s: %w", l.dir, ctx.Err())
		default:
		}
		err := os.Mkdir(l.dir, 0o755)
		if err == nil {
			return nil // acquired
		}
		if !os.IsExist(err) {
			return fmt.Errorf("mkdir lock %s: %w", l.dir, err)
		}
		attempt++
		if attempt >= maxAttempts {
			// Stale lock: remove and retry.
			os.Remove(l.dir) //nolint:errcheck
			attempt = 0
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquiring lock %s: %w", l.dir, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// Release removes the lock directory.
func (l *Lock) Release() error {
	if err := os.Remove(l.dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("releasing lock %s: %w", l.dir, err)
	}
	return nil
}

// WithLock acquires the lock, runs fn, then releases it.
func (l *Lock) WithLock(ctx context.Context, fn func() error) error {
	if err := l.Acquire(ctx); err != nil {
		return err
	}
	defer l.Release() //nolint:errcheck
	return fn()
}
