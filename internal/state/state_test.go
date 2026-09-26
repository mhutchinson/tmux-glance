package state

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func tmpStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	s, err := NewFileStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return s
}

func TestReadAll_Empty(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	entries, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	want := []Entry{
		{PaneID: "%1", Session: "main", Window: 0, Pane: 0, Path: "/repo", Command: "zsh", Label: "zsh in repo", Kind: KindManual, State: StateWatching},
		{PaneID: "%2", Session: "work", Window: 1, Pane: 2, Path: "/tmp", Command: "agy", Label: "agy in tmp", Kind: KindAuto, State: StateRunning},
	}
	if err := s.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseLine_MalformedSkipped(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"empty", "", false},
		{"too few fields", "%1\tmain\t0", false},
		{"whitespace only", "   ", false},
		{"valid", "%1\tmain\t0\t0\t/repo\tzsh\tlabel\tmanual\twatching", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, ok := parseLine(tt.input)
			if ok != tt.valid {
				t.Errorf("parseLine(%q): got valid=%v, want %v", tt.input, ok, tt.valid)
			}
		})
	}
}

func TestPrune(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	entries := []Entry{
		{PaneID: "%1", Session: "s", Window: 0, Pane: 0, Path: "/a", Command: "zsh", Label: "l", Kind: KindManual, State: StateWatching},
		{PaneID: "%2", Session: "s", Window: 0, Pane: 1, Path: "/b", Command: "zsh", Label: "l", Kind: KindManual, State: StateWatching},
		{PaneID: "%3", Session: "s", Window: 0, Pane: 2, Path: "/c", Command: "zsh", Label: "l", Kind: KindAuto, State: StateRunning},
	}
	if err := s.Write(entries); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Only %1 and %3 are alive.
	if err := s.Prune([]string{"%1", "%3"}); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	got, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries after prune, got %d", len(got))
	}
	if got[0].PaneID != "%1" || got[1].PaneID != "%3" {
		t.Errorf("wrong entries remaining: %v", got)
	}
}

func TestUpsertEntry(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	e1 := Entry{PaneID: "%1", Session: "s", Window: 0, Pane: 0, Path: "/a", Command: "zsh", Label: "l", Kind: KindManual, State: StateWatching}
	if err := s.UpsertEntry(e1); err != nil {
		t.Fatalf("UpsertEntry (insert): %v", err)
	}
	// Upsert existing: change state.
	e1.State = StateAlert
	if err := s.UpsertEntry(e1); err != nil {
		t.Fatalf("UpsertEntry (update): %v", err)
	}
	got, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].State != StateAlert {
		t.Errorf("want state alert, got %s", got[0].State)
	}
}

func TestRemoveEntry(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	entries := []Entry{
		{PaneID: "%1", Session: "s", Window: 0, Pane: 0, Path: "/a", Command: "zsh", Label: "l", Kind: KindManual, State: StateWatching},
		{PaneID: "%2", Session: "s", Window: 0, Pane: 1, Path: "/b", Command: "zsh", Label: "l", Kind: KindManual, State: StateWatching},
	}
	s.Write(entries) //nolint:errcheck
	if err := s.RemoveEntry("%1"); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}
	got, _ := s.ReadAll()
	if len(got) != 1 || got[0].PaneID != "%2" {
		t.Errorf("unexpected entries: %v", got)
	}
}

func TestUpdateState(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	e := Entry{PaneID: "%1", Session: "s", Window: 0, Pane: 0, Path: "/a", Command: "zsh", Label: "l", Kind: KindManual, State: StateWatching}
	s.Write([]Entry{e}) //nolint:errcheck
	if err := s.UpdateState("%1", StateAlert); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	got, _ := s.ReadAll()
	if got[0].State != StateAlert {
		t.Errorf("want alert, got %s", got[0].State)
	}
}

func TestUpdateEntry_RefreshesLabel(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	e := Entry{PaneID: "%1", Session: "s", Window: 0, Pane: 0, Path: "/old", Command: "zsh", Label: "zsh in old", Kind: KindManual, State: StateWatching}
	s.Write([]Entry{e}) //nolint:errcheck
	// Change directory and command — label should update deterministically.
	if err := s.UpdateEntry("%1", "s", 0, 0, "/new", "bash"); err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}
	got, _ := s.ReadAll()
	if got[0].Path != "/new" {
		t.Errorf("want path /new, got %q", got[0].Path)
	}
	if got[0].Command != "bash" {
		t.Errorf("want command 'bash', got %q", got[0].Command)
	}
	if got[0].Label != "bash in new" {
		t.Errorf("want label 'bash in new', got %q", got[0].Label)
	}
}

func TestLock_ConcurrentAcquire(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "state")
	os.WriteFile(lockPath, nil, 0o644) //nolint:errcheck
	l := NewLock(lockPath)

	// The filesystem mkdir lock serialises disk I/O across goroutines,
	// but goroutines still run concurrently in-process. Use atomic counter
	// to avoid a data race on the counter variable itself.
	const goroutines = 10
	var wg sync.WaitGroup
	var counter sync.Mutex
	increments := 0
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			ctx := context.Background()
			l.WithLock(ctx, func() error { //nolint:errcheck
				counter.Lock()
				increments++
				counter.Unlock()
				return nil
			})
		}()
	}
	wg.Wait()
	if increments != goroutines {
		t.Errorf("increments = %d, want %d", increments, goroutines)
	}
}

// TestWrite_ConcurrentCallsNoCollision verifies that concurrent Write calls
// from the same process don't clobber each other's temp files. With the old
// PID-based naming scheme (state.tmp.<pid>), two goroutines in the same process
// would use the identical temp path and could corrupt each other's writes.
// os.CreateTemp uses a random suffix, eliminating this collision.
func TestWrite_ConcurrentCallsNoCollision(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		i := i
		go func() {
			defer wg.Done()
			e := Entry{
				PaneID: "%1", Session: "s", Window: 0, Pane: i,
				Path: "/a", Command: "zsh", Label: "l",
				Kind: KindManual, State: StateWatching,
			}
			s.Write([]Entry{e}) //nolint:errcheck
		}()
	}
	wg.Wait()

	// After all goroutines finish, the file should be parseable (not corrupted).
	entries, err := s.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll after concurrent writes: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry after concurrent writes, got %d", len(entries))
	}
}

