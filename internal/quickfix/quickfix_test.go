package quickfix

import (
	"testing"

	"github.com/mhutchinson/tmux-glance/internal/state"
)

func TestAttentionQueue_Ordering(t *testing.T) {
	t.Parallel()

	entries := []state.Entry{
		{PaneID: "%1", Session: "s2", Window: 1, Pane: 1, Kind: state.KindAuto, State: state.StateDone},       // prio 3
		{PaneID: "%2", Session: "s1", Window: 0, Pane: 0, Kind: state.KindManual, State: state.StateAlert},    // prio 1
		{PaneID: "%3", Session: "s1", Window: 2, Pane: 0, Kind: state.KindAuto, State: state.StateWaiting},    // prio 2
		{PaneID: "%4", Session: "s1", Window: 1, Pane: 0, Kind: state.KindManual, State: state.StateWatching},  // ignored
		{PaneID: "%5", Session: "s0", Window: 0, Pane: 0, Kind: state.KindAuto, State: state.StateRunning},   // ignored
		{PaneID: "%6", Session: "s0", Window: 1, Pane: 0, Kind: state.KindAuto, State: state.StateWaiting},    // prio 2 (s0 before s1)
	}

	queue := AttentionQueue(entries)
	if len(queue) != 4 {
		t.Fatalf("expected 4 attention items, got %d", len(queue))
	}

	expectedIDs := []string{"%2", "%6", "%3", "%1"}
	for i, wantID := range expectedIDs {
		if queue[i].PaneID != wantID {
			t.Errorf("queue[%d] = %q, want %q", i, queue[i].PaneID, wantID)
		}
	}
}

func TestNextPrevIndex(t *testing.T) {
	t.Parallel()

	queue := []state.Entry{
		{PaneID: "%10"},
		{PaneID: "%20"},
		{PaneID: "%30"},
	}

	// Empty queue edge cases
	if NextIndex(nil, "%10") != -1 {
		t.Errorf("NextIndex on empty queue should return -1")
	}
	if PrevIndex(nil, "%10") != -1 {
		t.Errorf("PrevIndex on empty queue should return -1")
	}

	// Starting from outside the queue
	if got := NextIndex(queue, "%99"); got != 0 {
		t.Errorf("NextIndex from outside = %d, want 0", got)
	}
	if got := PrevIndex(queue, "%99"); got != 2 {
		t.Errorf("PrevIndex from outside = %d, want 2", got)
	}

	// Cycling forward
	if got := NextIndex(queue, "%10"); got != 1 {
		t.Errorf("NextIndex(%%10) = %d, want 1", got)
	}
	if got := NextIndex(queue, "%20"); got != 2 {
		t.Errorf("NextIndex(%%20) = %d, want 2", got)
	}
	if got := NextIndex(queue, "%30"); got != 0 { // wraps around
		t.Errorf("NextIndex(%%30) = %d, want 0", got)
	}

	// Cycling backward
	if got := PrevIndex(queue, "%30"); got != 1 {
		t.Errorf("PrevIndex(%%30) = %d, want 1", got)
	}
	if got := PrevIndex(queue, "%20"); got != 0 {
		t.Errorf("PrevIndex(%%20) = %d, want 0", got)
	}
	if got := PrevIndex(queue, "%10"); got != 2 { // wraps around
		t.Errorf("PrevIndex(%%10) = %d, want 2", got)
	}
}

// TestDeadPaneRemoval simulates the slice-mutation logic in Cycle directly:
// verifies that removing dead panes from the local queue slice doesn't corrupt
// index arithmetic for subsequent iterations.
func TestDeadPaneRemoval_IndexStability(t *testing.T) {
	t.Parallel()

	// Simulate a queue where the first two entries are dead (select would fail).
	// After removing them, we expect to land on the third entry.
	queue := []state.Entry{
		{PaneID: "%dead1", Session: "s", State: state.StateAlert},
		{PaneID: "%dead2", Session: "s", State: state.StateAlert},
		{PaneID: "%alive", Session: "s", State: state.StateAlert},
	}

	// Replicate the pruning loop from Cycle (next direction, starting at idx=0).
	idx := 0
	var landed string
	for range queue {
		if len(queue) == 0 {
			break
		}
		idx = idx % len(queue)
		target := queue[idx]

		if target.PaneID == "%alive" {
			landed = target.PaneID
			break
		}

		// Simulate dead-pane removal (next direction: don't advance idx).
		queue = append(queue[:idx], queue[idx+1:]...)
	}

	if landed != "%alive" {
		t.Errorf("expected to land on %%alive after pruning dead panes, got %q (queue len=%d)", landed, len(queue))
	}
}
