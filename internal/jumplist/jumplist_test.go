package jumplist

import (
	"context"
	"path/filepath"
	"testing"
)

// alwaysAlive returns a VerifyFn that considers all pane IDs alive.
func alwaysAlive() VerifyFn {
	return func(_ context.Context, _ string) bool { return true }
}

// aliveSet returns a VerifyFn that only considers the given IDs alive.
func aliveSet(ids ...string) VerifyFn {
	m := make(map[string]bool)
	for _, id := range ids {
		m[id] = true
	}
	return func(_ context.Context, id string) bool { return m[id] }
}

func newTestStack(t *testing.T, verify VerifyFn) *Stack {
	t.Helper()
	dir := t.TempDir()
	s, err := New(
		filepath.Join(dir, "jump_back"),
		filepath.Join(dir, "jump_forward"),
		verify,
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestRecord_PushesSourceToBack(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	ctx := context.Background()

	if err := s.Record(ctx, "%1", "%2"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	back, _ := readStack(s.backFile)
	if len(back) != 1 || back[0] != "%1" {
		t.Errorf("want back=[%%1], got %v", back)
	}
}

func TestRecord_ClearsForwardStack(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	ctx := context.Background()
	// Manually write a forward entry.
	writeStack(s.forwardFile, []string{"%3"}) //nolint:errcheck

	s.Record(ctx, "%1", "%2") //nolint:errcheck
	fwd, _ := readStack(s.forwardFile)
	if len(fwd) != 0 {
		t.Errorf("forward stack should be empty after Record, got %v", fwd)
	}
}

func TestRecord_NopWhenSourceEqualsDestination(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	s.Record(context.Background(), "%1", "%1") //nolint:errcheck
	back, _ := readStack(s.backFile)
	if len(back) != 0 {
		t.Errorf("want empty back stack for same source/dest, got %v", back)
	}
}

func TestRecord_NopWhenSourceDead(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, aliveSet()) // nothing is alive
	s.Record(context.Background(), "%1", "%2") //nolint:errcheck
	back, _ := readStack(s.backFile)
	if len(back) != 0 {
		t.Errorf("dead source should not push to back, got %v", back)
	}
}

func TestBack_MovesCurrentToForward(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	ctx := context.Background()
	writeStack(s.backFile, []string{"%2", "%1"}) //nolint:errcheck

	target, err := s.Back(ctx, "%3")
	if err != nil {
		t.Fatalf("Back: %v", err)
	}
	if target != "%2" {
		t.Errorf("want target %%2, got %q", target)
	}
	fwd, _ := readStack(s.forwardFile)
	if len(fwd) == 0 || fwd[0] != "%3" {
		t.Errorf("current pane %%3 should be on forward stack, got %v", fwd)
	}
	back, _ := readStack(s.backFile)
	if len(back) != 1 || back[0] != "%1" {
		t.Errorf("remaining back should be [%%1], got %v", back)
	}
}

func TestBack_EmptyStackReturnsEmpty(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	target, err := s.Back(context.Background(), "%1")
	if err != nil {
		t.Fatalf("Back: %v", err)
	}
	if target != "" {
		t.Errorf("want empty target on empty stack, got %q", target)
	}
}

func TestForward_MovesCurrentToBack(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	ctx := context.Background()
	writeStack(s.forwardFile, []string{"%4", "%5"}) //nolint:errcheck

	target, err := s.Forward(ctx, "%3")
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if target != "%4" {
		t.Errorf("want target %%4, got %q", target)
	}
	back, _ := readStack(s.backFile)
	if len(back) == 0 || back[0] != "%3" {
		t.Errorf("current %%3 should be on back stack, got %v", back)
	}
}

// TestShiftTo_BackwardInHistory tests Issue #2: selecting a history item
// should shift CUR, not create a new branch.
func TestShiftTo_BackwardInHistory(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	ctx := context.Background()
	// Back stack: [%2, %1] (BACK #1=%2, BACK #2=%1).
	// Forward stack: [].
	// CUR = %3.
	writeStack(s.backFile, []string{"%2", "%1"}) //nolint:errcheck

	// Shift from %3 to %1 (BACK #2).
	if err := s.ShiftTo(ctx, "%3", "%1"); err != nil {
		t.Fatalf("ShiftTo: %v", err)
	}

	// After ShiftTo to %1:
	// - %3 and %2 (the items between CUR and %1) should be on forward stack.
	// - back stack should not contain %1.
	back, _ := readStack(s.backFile)
	fwd, _ := readStack(s.forwardFile)

	for _, id := range back {
		if id == "%1" {
			t.Errorf("target %%1 should not remain on back stack after shift")
		}
	}
	if len(fwd) < 2 {
		t.Errorf("want fwd to contain %%3 and %%2, got %v", fwd)
	}
	if fwd[0] != "%3" {
		t.Errorf("fwd[0] should be %%3 (old CUR), got %q", fwd[0])
	}
	if fwd[1] != "%2" {
		t.Errorf("fwd[1] should be %%2 (intermediate), got %q", fwd[1])
	}
}

func TestShiftTo_ForwardInHistory(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	ctx := context.Background()
	// Forward stack: [%4, %5] (FWD #1=%4, FWD #2=%5).
	// CUR = %3.
	writeStack(s.forwardFile, []string{"%4", "%5"}) //nolint:errcheck

	// Shift from %3 to %5 (FWD #2).
	if err := s.ShiftTo(ctx, "%3", "%5"); err != nil {
		t.Fatalf("ShiftTo: %v", err)
	}

	back, _ := readStack(s.backFile)
	fwd, _ := readStack(s.forwardFile)

	// %5 should not remain on forward stack.
	for _, id := range fwd {
		if id == "%5" {
			t.Errorf("target %%5 should not remain on forward stack")
		}
	}
	// %3 and %4 should be on back stack.
	if len(back) < 2 {
		t.Errorf("want back to contain %%3 and %%4, got %v", back)
	}
	if back[0] != "%3" {
		t.Errorf("back[0] should be %%3 (old CUR), got %q", back[0])
	}
	if back[1] != "%4" {
		t.Errorf("back[1] should be %%4 (intermediate), got %q", back[1])
	}
}

func TestClear(t *testing.T) {
	t.Parallel()
	s := newTestStack(t, alwaysAlive())
	writeStack(s.backFile, []string{"%1", "%2"})     //nolint:errcheck
	writeStack(s.forwardFile, []string{"%3", "%4"})  //nolint:errcheck
	if err := s.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	back, _ := readStack(s.backFile)
	fwd, _ := readStack(s.forwardFile)
	if len(back) != 0 || len(fwd) != 0 {
		t.Errorf("after Clear: back=%v fwd=%v", back, fwd)
	}
}

func TestHistoryList_FiltersDead(t *testing.T) {
	t.Parallel()
	// Only %2 is alive.
	s := newTestStack(t, aliveSet("%2", "%4"))
	writeStack(s.backFile, []string{"%1", "%2", "%3"})   //nolint:errcheck
	writeStack(s.forwardFile, []string{"%4", "%5"})      //nolint:errcheck

	fwd, cur, back := s.HistoryList(context.Background(), "%cur")
	if len(fwd) != 1 || fwd[0] != "%4" {
		t.Errorf("fwd: want [%%4], got %v", fwd)
	}
	if cur != "%cur" {
		t.Errorf("cur: want %%cur, got %q", cur)
	}
	if len(back) != 1 || back[0] != "%2" {
		t.Errorf("back: want [%%2], got %v", back)
	}
}

func TestCursorPos(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		back []string
		fwd  []string
		want int
	}{
		{"empty", nil, nil, 1},
		{"only fwd", []string{}, []string{"%f1", "%f2"}, 2},
		{"back exists", []string{"%b1"}, []string{"%f1"}, 3},
		{"only back", []string{"%b1", "%b2"}, nil, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := newTestStack(t, alwaysAlive())
			writeStack(s.backFile, tt.back)   //nolint:errcheck
			writeStack(s.forwardFile, tt.fwd) //nolint:errcheck
			got := s.CursorPos(context.Background(), "%cur")
			if got != tt.want {
				t.Errorf("CursorPos = %d, want %d", got, tt.want)
			}
		})
	}
}
