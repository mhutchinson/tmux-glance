// Package jumplist manages the jump-back and jump-forward stacks used by
// tmux-glance's history navigation (Ctrl-h, jump-back, jump-forward).
//
// The stacks are stored as one-pane-ID-per-line flat files. All mutations
// are atomic (write tmp → rename).
//
// This package fixes Issue #2: history-modal selection now correctly
// shifts the CUR frame pointer rather than recording a new branch.
package jumplist

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxStack = 50

// VerifyFn checks whether a pane ID is still alive on the tmux server.
type VerifyFn func(ctx context.Context, paneID string) bool

// Stack holds the paths to the back and forward stack files.
type Stack struct {
	backFile    string
	forwardFile string
	verifyFn    VerifyFn
}

// New returns a Stack using the given file paths.
// verifyFn is called to determine whether a pane ID is still alive;
// pass nil to use the default (real tmux subprocess call).
func New(backFile, forwardFile string, verifyFn VerifyFn) (*Stack, error) {
	for _, dir := range []string{filepath.Dir(backFile), filepath.Dir(forwardFile)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating jumplist dir: %w", err)
		}
	}
	for _, f := range []string{backFile, forwardFile} {
		fh, err := os.OpenFile(f, os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("touching jumplist file %s: %w", f, err)
		}
		fh.Close()
	}
	if verifyFn == nil {
		verifyFn = DefaultVerify
	}
	return &Stack{backFile: backFile, forwardFile: forwardFile, verifyFn: verifyFn}, nil
}

// Record pushes source onto the back stack and clears the forward stack.
// Called on every new forward jump (Go-To, Attention Hub, Harpoon slot).
// source == dest is a no-op; dead source panes are also no-ops.
func (s *Stack) Record(ctx context.Context, source, dest string) error {
	if source == "" || dest == "" || source == dest {
		return nil
	}
	if !s.verifyFn(ctx, source) {
		return nil
	}
	back, err := readStack(s.backFile)
	if err != nil {
		return err
	}
	if len(back) == 0 || back[0] != source {
		back = prepend(back, source)
	}
	if err := writeStack(s.backFile, back); err != nil {
		return err
	}
	return clearFile(s.forwardFile)
}

// Back moves one step backward in history.
// The current pane is pushed onto the forward stack.
// Returns the target pane ID, or "" if already at the oldest location.
func (s *Stack) Back(ctx context.Context, currentPane string) (string, error) {
	back, err := readStack(s.backFile)
	if err != nil {
		return "", err
	}
	target, idx := findLive(ctx, back, currentPane, s.verifyFn)
	if target == "" {
		writeStack(s.backFile, nil) //nolint:errcheck
		return "", nil
	}
	remaining := back[idx+1:]
	if err := writeStack(s.backFile, remaining); err != nil {
		return "", err
	}
	if currentPane != "" {
		fwd, _ := readStack(s.forwardFile)
		if len(fwd) == 0 || fwd[0] != currentPane {
			fwd = prepend(fwd, currentPane)
		}
		writeStack(s.forwardFile, fwd) //nolint:errcheck
	}
	return target, nil
}

// Forward moves one step forward in history.
// The current pane is pushed onto the back stack.
// Returns the target pane ID, or "" if already at the newest location.
func (s *Stack) Forward(ctx context.Context, currentPane string) (string, error) {
	fwd, err := readStack(s.forwardFile)
	if err != nil {
		return "", err
	}
	target, idx := findLive(ctx, fwd, currentPane, s.verifyFn)
	if target == "" {
		writeStack(s.forwardFile, nil) //nolint:errcheck
		return "", nil
	}
	remaining := fwd[idx+1:]
	if err := writeStack(s.forwardFile, remaining); err != nil {
		return "", err
	}
	if currentPane != "" {
		back, _ := readStack(s.backFile)
		if len(back) == 0 || back[0] != currentPane {
			back = prepend(back, currentPane)
		}
		writeStack(s.backFile, back) //nolint:errcheck
	}
	return target, nil
}

// ShiftTo moves the CUR frame to the given target pane without recording a
// new branch — fixes Issue #2. Items between CUR and target are moved between
// the stacks to keep the timeline consistent.
//
// If target is in the back stack, entries above it (between CUR and target) are
// moved to the forward stack. If target is in the forward stack, entries above
// it are moved to the back stack.
func (s *Stack) ShiftTo(ctx context.Context, currentPane, target string) error {
	if target == "" || target == currentPane {
		return nil
	}
	back, _ := readStack(s.backFile)
	fwd, _ := readStack(s.forwardFile)

	// Check back stack first.
	for i, id := range back {
		if id != target {
			continue
		}
		// Entries back[0..i-1] sit between CUR and target; move them to forward.
		movedToFwd := make([]string, i)
		copy(movedToFwd, back[:i])
		newBack := back[i+1:]
		newFwd := make([]string, 0, 1+len(movedToFwd)+len(fwd))
		if currentPane != "" {
			newFwd = append(newFwd, currentPane)
		}
		newFwd = append(newFwd, movedToFwd...)
		newFwd = append(newFwd, fwd...)
		writeStack(s.backFile, newBack)  //nolint:errcheck
		return writeStack(s.forwardFile, newFwd)
	}

	// Check forward stack.
	for i, id := range fwd {
		if id != target {
			continue
		}
		// Entries fwd[0..i-1] sit between target and CUR; move them to back.
		movedToBack := make([]string, i)
		copy(movedToBack, fwd[:i])
		newFwd := fwd[i+1:]
		newBack := make([]string, 0, 1+len(movedToBack)+len(back))
		if currentPane != "" {
			newBack = append(newBack, currentPane)
		}
		newBack = append(newBack, movedToBack...)
		newBack = append(newBack, back...)
		writeStack(s.forwardFile, newFwd) //nolint:errcheck
		return writeStack(s.backFile, newBack)
	}

	// Not in either stack: treat as a fresh branch record.
	return s.Record(ctx, currentPane, target)
}

// Clear wipes both stacks.
func (s *Stack) Clear() error {
	if err := clearFile(s.backFile); err != nil {
		return err
	}
	return clearFile(s.forwardFile)
}

// HistoryList returns the ordered timeline:
//   - fwd: forward entries (FWD #1 is nearest CUR, so fwd[0] = FWD #1)
//   - cur: the current pane ID
//   - back: back entries (BACK #1 is nearest CUR, so back[0] = BACK #1)
//
// Dead panes are filtered out.
func (s *Stack) HistoryList(ctx context.Context, currentPane string) (fwd []string, cur string, back []string) {
	rawFwd, _ := readStack(s.forwardFile)
	rawBack, _ := readStack(s.backFile)
	for _, id := range rawFwd {
		if id != currentPane && s.verifyFn(ctx, id) {
			fwd = append(fwd, id)
		}
	}
	for _, id := range rawBack {
		if id != currentPane && s.verifyFn(ctx, id) {
			back = append(back, id)
		}
	}
	return fwd, currentPane, back
}

// CursorPos returns the 1-based FZF row position for the history list.
// Positions on BACK #1 when available; FWD top otherwise.
func (s *Stack) CursorPos(ctx context.Context, currentPane string) int {
	fwd, _, back := s.HistoryList(ctx, currentPane)
	if len(back) > 0 {
		return len(fwd) + 2 // past all FWD entries and CUR
	}
	if len(fwd) > 0 {
		return len(fwd)
	}
	return 1
}

// --- file helpers ---

func readStack(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading stack %s: %w", path, err)
	}
	var entries []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			entries = append(entries, line)
		}
	}
	return entries, nil
}

func writeStack(path string, entries []string) error {
	if len(entries) > maxStack {
		entries = entries[:maxStack]
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("writing stack tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("renaming stack: %w", err)
	}
	return nil
}

func clearFile(path string) error { return writeStack(path, nil) }

func prepend(slice []string, item string) []string {
	result := make([]string, 0, len(slice)+1)
	result = append(result, item)
	result = append(result, slice...)
	if len(result) > maxStack {
		result = result[:maxStack]
	}
	return result
}

func findLive(ctx context.Context, entries []string, currentPane string, verify VerifyFn) (string, int) {
	for i, id := range entries {
		if id == "" || id == currentPane {
			continue
		}
		if verify(ctx, id) {
			return id, i
		}
	}
	return "", -1
}

// DefaultVerify checks pane liveness via a real tmux subprocess call.
func DefaultVerify(ctx context.Context, paneID string) bool {
	out, err := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", paneID, "#{pane_id}").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == paneID
}
