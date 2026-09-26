// Package scanner implements the scan_agents logic: it evaluates all tmux panes,
// classifies them via sentinels, updates state, and computes the status summary.
// Candidate panes (agents + manual vigils) are evaluated concurrently using
// goroutines. Non-candidate generic panes are fast-pathed in O(1) time.
package scanner

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhutchinson/tmux-glance/internal/sentinel"
	"github.com/mhutchinson/tmux-glance/internal/state"
	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

const (
	defaultCooldown = 3 * time.Second
	workerPoolSize  = 8
)

// Scanner orchestrates pane scanning and state updates.
type Scanner struct {
	store    *state.FileStore
	sentinel *sentinel.Registry
	tmux     *tmux.Client
	cooldown time.Duration
}

// New returns a Scanner ready to scan.
func New(store *state.FileStore, reg *sentinel.Registry, client *tmux.Client, cooldown time.Duration) *Scanner {
	if cooldown == 0 {
		cooldown = defaultCooldown
	}
	return &Scanner{store: store, sentinel: reg, tmux: client, cooldown: cooldown}
}

// paneResult is the outcome of evaluating a single candidate pane.
type paneResult struct {
	paneID string
	action string // "set", "remove", "noop"
	entry  state.Entry
}

// Scan runs a full scan pass over all tmux panes, updates state, and records
// scan metadata for debouncing. force=true bypasses the cooldown check.
// Pruning of dead panes always runs (even within cooldown) so status output
// is never stale with dead entries.
func (sc *Scanner) Scan(ctx context.Context, force bool) error {
	// Pruning always runs — dead panes must be removed regardless of cooldown.
	allPanes, err := sc.tmux.ListAllPanes(ctx)
	if err != nil {
		return fmt.Errorf("listing panes: %w", err)
	}
	alivePaneIDs := make([]string, len(allPanes))
	for i, p := range allPanes {
		alivePaneIDs[i] = p.ID
	}
	if err := sc.store.Prune(alivePaneIDs); err != nil {
		return fmt.Errorf("pruning state: %w", err)
	}

	if !force {
		if sc.withinCooldown(ctx) {
			return nil
		}
	}

	// Load current state.
	entries, err := sc.store.ReadAll()
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	stateByID := make(map[string]state.Entry, len(entries))
	for _, e := range entries {
		stateByID[e.PaneID] = e
	}

	// Get focused panes.
	focusedIDs, _ := sc.tmux.FocusedPaneIDs(ctx)
	focused := make(map[string]bool, len(focusedIDs))
	for _, id := range focusedIDs {
		focused[id] = true
	}

	// Identify candidates (agent panes + manual vigils).
	var candidates []tmux.PaneInfo
	for _, p := range allPanes {
		sentName := sc.sentinel.ResolvePane(ctx, p)
		_, tracked := stateByID[p.ID]
		if sentName == "generic" && !tracked {
			continue // fast-path: untracked generic pane
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		return sc.recordScan(ctx, focusedIDs)
	}

	// Evaluate candidates concurrently with a worker pool.
	results := make([]paneResult, len(candidates))
	sem := make(chan struct{}, workerPoolSize)
	var wg sync.WaitGroup

	for i, p := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, pane tmux.PaneInfo) {
			defer wg.Done()
			defer func() { <-sem }()
			existing := stateByID[pane.ID]
			isFocused := focused[pane.ID]
			results[idx] = sc.evaluateCandidate(ctx, pane, existing, isFocused)
		}(i, p)
	}
	wg.Wait()

	// Apply results.
	updated := false
	for _, res := range results {
		switch res.action {
		case "set":
			if err := sc.store.UpsertEntry(res.entry); err != nil {
				return fmt.Errorf("upsert %s: %w", res.paneID, err)
			}
			updated = true
		case "remove":
			if err := sc.store.RemoveEntry(res.paneID); err != nil {
				return fmt.Errorf("remove %s: %w", res.paneID, err)
			}
			updated = true
		case "refresh":
			// Live metadata refresh (path/label update for Issue #5).
			p := candidateByID(candidates, res.paneID)
			if p != nil {
				sc.store.UpdateEntry(p.ID, p.Session, p.Window, p.Pane, p.Path, p.Command) //nolint:errcheck
			}
		}
	}
	_ = updated

	return sc.recordScan(ctx, focusedIDs)
}

// OnFocus handles a pane-focus-in event, auto-acknowledging completed tasks
// and syncing agent snapshots.
func (sc *Scanner) OnFocus(ctx context.Context, paneID string) error {
	if paneID == "" {
		paneID, _ = sc.tmux.GetPaneField(ctx, "", "#{pane_id}")
	}
	if paneID == "" {
		return nil
	}

	entry, found, err := sc.store.GetEntry(paneID)
	if err != nil {
		return err
	}

	if found {
		switch entry.Kind {
		case state.KindAuto:
			if entry.State == state.StateDone {
				sc.store.RemoveEntry(paneID) //nolint:errcheck
				sc.tmux.RefreshClients(ctx)  //nolint:errcheck
			} else {
				cmd, _ := sc.tmux.GetPaneField(ctx, paneID, "#{pane_current_command}")
				pidStr, _ := sc.tmux.GetPaneField(ctx, paneID, "#{pane_pid}")
				pid, _ := strconv.Atoi(pidStr)
				sentName := sc.sentinel.ResolvePane(ctx, tmux.PaneInfo{ID: paneID, Command: cmd, PID: pid})
				if sentName != "generic" {
					path, _ := sc.tmux.GetPaneField(ctx, paneID, "#{pane_current_path}")
					cls, err := sc.sentinel.Classify(ctx, sentName, paneID, path, cmd)
					if err == nil && cls.State == "idle" {
						sc.store.RemoveEntry(paneID) //nolint:errcheck
						sc.tmux.RefreshClients(ctx)  //nolint:errcheck
					}
				}
			}
		case state.KindManual:
			hash, _ := sc.sentinel.Fingerprint(ctx, "generic", paneID)
			sc.tmux.SetPaneOption(ctx, paneID, "@glance_snapshot", hash) //nolint:errcheck
			if entry.State == state.StateAlert {
				sc.store.UpdateState(paneID, state.StateWatching) //nolint:errcheck
				sc.tmux.RefreshClients(ctx)                       //nolint:errcheck
			}
		}
	}

	// Always sync agent snapshot on focus if the pane runs an agent.
	cmd, _ := sc.tmux.GetPaneField(ctx, paneID, "#{pane_current_command}")
	pidStr, _ := sc.tmux.GetPaneField(ctx, paneID, "#{pane_pid}")
	pid, _ := strconv.Atoi(pidStr)
	sentName := sc.sentinel.ResolvePane(ctx, tmux.PaneInfo{ID: paneID, Command: cmd, PID: pid})
	if sentName != "generic" {
		hash, _ := sc.sentinel.Fingerprint(ctx, sentName, paneID)
		sc.tmux.SetPaneOption(ctx, paneID, "@glance_agent_snapshot", hash) //nolint:errcheck
	}

	// Invalidate scan cooldown so the next status poll sees fresh state.
	// Skip for untracked generic panes (e.g. the glance popup pane running fzf):
	// those focus events carry no meaningful state change and resetting the
	// cooldown would force a spurious full rescan on every popup open/close.
	if found || sentName != "generic" {
		sc.tmux.SetGlobalOption(ctx, "@glance_last_scan_time", "0") //nolint:errcheck
	}
	return nil
}

// StatusSummary returns the tmux status-right formatted string.
// It triggers a Scan first to ensure data is fresh within cooldown.
func (sc *Scanner) StatusSummary(ctx context.Context) (string, error) {
	if err := sc.Scan(ctx, false); err != nil {
		return "", err
	}
	entries, err := sc.store.ReadAll()
	if err != nil {
		return "", err
	}
	var alerts, watches, waiting, running, done int
	for _, e := range entries {
		switch e.Kind {
		case state.KindManual:
			if e.State == state.StateAlert {
				alerts++
			} else {
				watches++
			}
		case state.KindAuto:
			switch e.State {
			case state.StateWaiting:
				waiting++
			case state.StateRunning:
				running++
			case state.StateDone:
				done++
			}
		}
	}

	var sb strings.Builder
	if alerts > 0 {
		fmt.Fprintf(&sb, "#[fg=#f38ba8,bold]🚨 %d#[default] ", alerts)
	}
	if watches > 0 {
		fmt.Fprintf(&sb, "#[fg=#b4befe]👁️ %d#[default] ", watches)
	}
	if waiting > 0 {
		fmt.Fprintf(&sb, "#[fg=#fab387,bold]🤖 ⏳ %d#[default] ", waiting)
	}
	if running > 0 {
		fmt.Fprintf(&sb, "#[fg=#a6e3a1]🤖 ⚡ %d#[default] ", running)
	}
	if done > 0 {
		fmt.Fprintf(&sb, "#[fg=#89b4fa,bold]🤖 ✓ %d#[default] ", done)
	}
	return strings.TrimRight(sb.String(), " "), nil
}

// evaluateCandidate classifies a single candidate pane and returns the action to take.
func (sc *Scanner) evaluateCandidate(ctx context.Context, p tmux.PaneInfo, existing state.Entry, isFocused bool) paneResult {
	sentName := sc.sentinel.ResolvePane(ctx, p)
	hasExisting := existing.PaneID != ""

	// Refresh live metadata for Issue #5 (stale vigil labels).
	refreshResult := paneResult{paneID: p.ID, action: "refresh"}

	// Manual vigil.
	if hasExisting && existing.Kind == state.KindManual {
		currHash, _ := sc.sentinel.Fingerprint(ctx, "generic", p.ID)
		prevHash, _ := sc.tmux.GetPaneOption(ctx, p.ID, "@glance_snapshot")

		if isFocused {
			sc.tmux.SetPaneOption(ctx, p.ID, "@glance_snapshot", currHash) //nolint:errcheck
			if existing.State == state.StateAlert {
				updated := existing
				updated.State = state.StateWatching
				return paneResult{paneID: p.ID, action: "set", entry: updated}
			}
			return refreshResult
		}
		if prevHash == "" {
			sc.tmux.SetPaneOption(ctx, p.ID, "@glance_snapshot", currHash) //nolint:errcheck
			return refreshResult
		}
		if currHash != prevHash && existing.State != state.StateAlert {
			updated := existing
			updated.State = state.StateAlert
			return paneResult{paneID: p.ID, action: "set", entry: updated}
		}
		return refreshResult
	}

	// Autonomous agent (non-generic sentinel).
	if sentName != "generic" {
		cls, err := sc.sentinel.Classify(ctx, sentName, p.ID, p.Path, p.Command)
		if err != nil {
			return paneResult{paneID: p.ID, action: "noop"}
		}

		if cls.State == "idle" {
			currHash, _ := sc.sentinel.Fingerprint(ctx, sentName, p.ID)
			prevHash, _ := sc.tmux.GetPaneOption(ctx, p.ID, "@glance_agent_snapshot")

			if isFocused {
				sc.tmux.SetPaneOption(ctx, p.ID, "@glance_agent_snapshot", currHash) //nolint:errcheck
				if hasExisting {
					return paneResult{paneID: p.ID, action: "remove"}
				}
				return paneResult{paneID: p.ID, action: "noop"}
			}
			if prevHash != "" && currHash != prevHash {
				// Unread activity while idle.
				updated := existing
				updated.PaneID = p.ID
				updated.Session = p.Session
				updated.Window = p.Window
				updated.Pane = p.Pane
				updated.Path = p.Path
				updated.Command = p.Command
				updated.Kind = state.KindAuto
				updated.State = state.StateDone
				updated.Label = "updated in " + filepath.Base(p.Path)
				return paneResult{paneID: p.ID, action: "set", entry: updated}
			}
			if prevHash == "" {
				sc.tmux.SetPaneOption(ctx, p.ID, "@glance_agent_snapshot", currHash) //nolint:errcheck
			}
			if hasExisting && (existing.State == state.StateWaiting || existing.State == state.StateRunning) {
				return paneResult{paneID: p.ID, action: "remove"}
			}
			return paneResult{paneID: p.ID, action: "noop"}
		}

		// Agent is actively running/waiting.
		if isFocused {
			if !hasExisting {
				return paneResult{paneID: p.ID, action: "noop"}
			}
			if cls.State == "done" || existing.State == state.StateDone || cls.State == "idle" {
				return paneResult{paneID: p.ID, action: "remove"}
			}
			newState := state.PaneState(cls.State)
			if existing.State != newState {
				entry := existing
				entry.State = newState
				entry.Label = cls.Label
				return paneResult{paneID: p.ID, action: "set", entry: entry}
			}
			return paneResult{paneID: p.ID, action: "noop"}
		}

		newState := state.PaneState(cls.State)
		if !hasExisting || existing.State != newState {
			entry := state.Entry{
				PaneID:  p.ID,
				Session: p.Session,
				Window:  p.Window,
				Pane:    p.Pane,
				Path:    p.Path,
				Command: p.Command,
				Label:   cls.Label,
				Kind:    state.KindAuto,
				State:   newState,
			}
			return paneResult{paneID: p.ID, action: "set", entry: entry}
		}
		return paneResult{paneID: p.ID, action: "noop"}
	}

	// Generic pane that was previously tracked (command changed away from agent).
	if hasExisting && (existing.State == state.StateRunning || existing.State == state.StateWaiting) {
		return paneResult{paneID: p.ID, action: "set", entry: state.Entry{
			PaneID:  p.ID,
			Session: p.Session,
			Window:  p.Window,
			Pane:    p.Pane,
			Path:    p.Path,
			Command: p.Command,
			Label:   "exited in " + filepath.Base(p.Path),
			Kind:    state.KindAuto,
			State:   state.StateDone,
		}}
	}

	return paneResult{paneID: p.ID, action: "noop"}
}

// withinCooldown returns true if a scan completed recently and focus hasn't changed.
func (sc *Scanner) withinCooldown(ctx context.Context) bool {
	if sc.cooldown <= 0 {
		return false
	}
	lastStr, _ := sc.tmux.GetGlobalOption(ctx, "@glance_last_scan_time")
	lastTs, err := strconv.ParseInt(strings.TrimSpace(lastStr), 10, 64)
	if err != nil || lastTs == 0 {
		return false
	}
	elapsed := time.Since(time.Unix(lastTs, 0))
	if elapsed >= sc.cooldown {
		return false
	}
	// Also check if focus has changed.
	lastFocus, _ := sc.tmux.GetGlobalOption(ctx, "@glance_last_scan_focus")
	currFocused, _ := sc.tmux.FocusedPaneIDs(ctx)
	currFocusStr := strings.Join(currFocused, " ")
	return strings.TrimSpace(lastFocus) == strings.TrimSpace(currFocusStr)
}

// recordScan stamps the scan timestamp and focus state into tmux options.
func (sc *Scanner) recordScan(ctx context.Context, focusedIDs []string) error {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sc.tmux.SetGlobalOption(ctx, "@glance_last_scan_time", ts)                   //nolint:errcheck
	sc.tmux.SetGlobalOption(ctx, "@glance_last_scan_focus", strings.Join(focusedIDs, " ")) //nolint:errcheck
	return nil
}

// candidateByID finds a PaneInfo in a slice by ID.
func candidateByID(panes []tmux.PaneInfo, id string) *tmux.PaneInfo {
	for i := range panes {
		if panes[i].ID == id {
			return &panes[i]
		}
	}
	return nil
}
