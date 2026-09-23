// Package quickfix implements Vim-quickfix-style cyclic navigation
// across active attention items (alerts, waiting prompts, and finished tasks).
package quickfix

import (
	"context"
	"fmt"
	"sort"

	"github.com/mhutchinson/tmux-glance/internal/jumplist"
	"github.com/mhutchinson/tmux-glance/internal/state"
	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

// AttentionQueue returns a slice of pane entries that currently need attention,
// sorted by severity rank:
// 1. Alerts (🚨)
// 2. Waiting for confirmation (🤖 ⏳)
// 3. Finished tasks (🤖 ✓)
// Secondary tie-breaker: Session, Window, Pane index.
func AttentionQueue(entries []state.Entry) []state.Entry {
	type item struct {
		entry state.Entry
		prio  int
	}
	var items []item
	for _, e := range entries {
		prio := 0
		switch e.Kind {
		case state.KindManual:
			if e.State == state.StateAlert {
				prio = 1
			}
		case state.KindAuto:
			switch e.State {
			case state.StateWaiting:
				prio = 2
			case state.StateDone:
				prio = 3
			}
		}
		if prio > 0 {
			items = append(items, item{entry: e, prio: prio})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].prio != items[j].prio {
			return items[i].prio < items[j].prio
		}
		if items[i].entry.Session != items[j].entry.Session {
			return items[i].entry.Session < items[j].entry.Session
		}
		if items[i].entry.Window != items[j].entry.Window {
			return items[i].entry.Window < items[j].entry.Window
		}
		return items[i].entry.Pane < items[j].entry.Pane
	})

	res := make([]state.Entry, len(items))
	for i, it := range items {
		res[i] = it.entry
	}
	return res
}

// NextIndex returns the next index in a circular queue given the current pane ID.
func NextIndex(queue []state.Entry, curPaneID string) int {
	if len(queue) == 0 {
		return -1
	}
	for i, e := range queue {
		if e.PaneID == curPaneID {
			return (i + 1) % len(queue)
		}
	}
	return 0
}

// PrevIndex returns the previous index in a circular queue given the current pane ID.
func PrevIndex(queue []state.Entry, curPaneID string) int {
	if len(queue) == 0 {
		return -1
	}
	for i, e := range queue {
		if e.PaneID == curPaneID {
			return (i - 1 + len(queue)) % len(queue)
		}
	}
	return len(queue) - 1
}

// Cycle jumps to the next or previous attention pane in the queue.
func Cycle(ctx context.Context, dir string, store *state.FileStore, stack *jumplist.Stack, tc *tmux.Client, curPaneID string) error {
	if curPaneID == "" {
		curPaneID, _ = tc.GetPaneField(ctx, "", "#{pane_id}")
	}

	entries, err := store.ReadAll()
	if err != nil {
		return err
	}

	queue := AttentionQueue(entries)
	if len(queue) == 0 {
		tc.DisplayMessage(ctx, "Glance: no attention items (all quiet)") //nolint:errcheck
		return nil
	}

	var idx int
	if dir == "prev" || dir == "backward" || dir == "cprev" {
		idx = PrevIndex(queue, curPaneID)
	} else {
		idx = NextIndex(queue, curPaneID)
	}

	for attempts := 0; attempts < len(queue); attempts++ {
		target := queue[idx]
		if target.PaneID == curPaneID && len(queue) == 1 {
			tc.DisplayMessage(ctx, fmt.Sprintf("Glance [1/1]: already on %s (%s)", target.Session, target.State)) //nolint:errcheck
			return nil
		}

		if err := tc.SelectPane(ctx, target.PaneID); err == nil {
			if curPaneID != "" && target.PaneID != curPaneID {
				stack.Record(ctx, curPaneID, target.PaneID) //nolint:errcheck
			}

			badge := "🚨 Alert"
			switch target.State {
			case state.StateWaiting:
				badge = "🤖 ⏳ Waiting"
			case state.StateDone:
				badge = "🤖 ✓ Finished"
			}
			msg := fmt.Sprintf("Glance [%d/%d] %s: %s (%s)", idx+1, len(queue), badge, target.Session, target.Label)
			tc.DisplayMessage(ctx, msg) //nolint:errcheck
			return nil
		}

		// Prune dead pane if select fails and advance index
		store.RemoveEntry(target.PaneID) //nolint:errcheck
		if dir == "prev" || dir == "backward" || dir == "cprev" {
			idx = (idx - 1 + len(queue)) % len(queue)
		} else {
			idx = (idx + 1) % len(queue)
		}
	}

	tc.DisplayMessage(ctx, "Glance: no reachable attention items") //nolint:errcheck
	return nil
}
