// Package sentinel manages agent classification, command routing, and buffer
// normalization. Built-in sentinels are implemented natively in pure Go without
// subprocess overhead.
package sentinel

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

// Classification is the result of calling a sentinel's Classify.
type Classification struct {
	State string // e.g. "waiting", "running", "done", "idle", "unknown", "watching"
	Label string
}

// Sentinel represents an agent classifier and buffer normalizer.
type Sentinel interface {
	Name() string
	Matches(pane tmux.PaneInfo) bool
	Classify(ctx context.Context, pane tmux.PaneInfo, buffer string) (Classification, error)
	Fingerprint(ctx context.Context, pane tmux.PaneInfo, buffer string) (string, error)
}

// Registry holds registered sentinels and resolves commands to matching sentinels.
type Registry struct {
	sentinels      map[string]Sentinel
	order          []string // evaluation order (antigravity, ..., generic)
	mu             sync.RWMutex
	commandMap     map[string]string // fast command-name cache
	disabled       map[string]bool
	routeOverrides map[string]string
	capturePaneFn  func(ctx context.Context, paneID string) (string, error)
}

// New initializes the sentinel registry with built-in native sentinels.
// searchDirs is preserved for interface compatibility.
func New(_ []string, disabled []string, routeOverrides map[string]string) (*Registry, error) {
	r := &Registry{
		sentinels:      make(map[string]Sentinel),
		commandMap:     make(map[string]string),
		disabled:       make(map[string]bool),
		routeOverrides: routeOverrides,
	}

	for _, d := range disabled {
		if d = strings.TrimSpace(d); d != "" {
			r.disabled[d] = true
		}
	}

	// Register built-in native sentinels
	r.Register(NewAntigravity(nil))
	r.Register(NewGeneric())

	// Pre-populate commandMap for direct mappings
	r.commandMap["agy"] = "antigravity"
	r.commandMap["antigravity"] = "antigravity"

	// Apply user route overrides
	for cmd, target := range routeOverrides {
		r.commandMap[strings.TrimSpace(cmd)] = strings.TrimSpace(target)
	}

	return r, nil
}

// Register adds a sentinel to the registry. Generic is always evaluated last.
func (r *Registry) Register(s Sentinel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := s.Name()
	r.sentinels[name] = s

	// Maintain evaluation order with generic always at the end
	var newOrder []string
	for _, n := range r.order {
		if n != name && n != "generic" {
			newOrder = append(newOrder, n)
		}
	}
	if name != "generic" {
		newOrder = append(newOrder, name)
	}
	newOrder = append(newOrder, "generic")
	r.order = newOrder
}

// Resolve returns the sentinel name for a command string (for backward compatibility).
func (r *Registry) Resolve(ctx context.Context, cmd string) string {
	return r.ResolvePane(ctx, tmux.PaneInfo{Command: cmd})
}

// ResolvePane matches a pane against registered sentinels, taking into account
// command names, user route overrides, disabled sentinels, and process tree inspection.
func (r *Registry) ResolvePane(ctx context.Context, pane tmux.PaneInfo) string {
	cmd := strings.TrimSpace(pane.Command)
	if cmd == "" {
		return "generic"
	}

	// 1. Direct user route overrides (e.g. @glance_routes: "mycli=antigravity", "agy=generic")
	if target, ok := r.routeOverrides[cmd]; ok {
		if !r.disabled[target] {
			return target
		}
		return "generic"
	}

	// 2. Disabled check for known commands
	r.mu.RLock()
	if name, ok := r.commandMap[cmd]; ok {
		r.mu.RUnlock()
		if r.disabled[name] {
			return "generic"
		}
		return name
	}
	r.mu.RUnlock()

	// 3. Dynamic match across registered sentinels in order
	for _, name := range r.order {
		if name == "generic" || r.disabled[name] {
			continue
		}
		s := r.sentinels[name]
		if s.Matches(pane) {
			// Cache direct command matches that don't depend on dynamic PID
			if pane.PID == 0 || cmd == "agy" || cmd == "antigravity" {
				r.mu.Lock()
				r.commandMap[cmd] = name
				r.mu.Unlock()
			}
			return name
		}
	}

	// 4. Cache fallback to generic for static commands without PID
	if pane.PID == 0 {
		r.mu.Lock()
		r.commandMap[cmd] = "generic"
		r.mu.Unlock()
	}
	return "generic"
}

// Classify captures the pane buffer and delegates classification to the named sentinel.
func (r *Registry) Classify(ctx context.Context, name, paneID, path, cmd string) (Classification, error) {
	s := r.getSentinel(name)
	buf, err := r.capture(ctx, paneID)
	if err != nil && name != "generic" {
		return Classification{State: "unknown", Label: filepath.Base(path)}, err
	}
	pane := tmux.PaneInfo{
		ID:      paneID,
		Path:    path,
		Command: cmd,
	}
	return s.Classify(ctx, pane, buf)
}

// Fingerprint captures the pane buffer and computes the normalized fingerprint.
func (r *Registry) Fingerprint(ctx context.Context, name, paneID string) (string, error) {
	s := r.getSentinel(name)
	buf, err := r.capture(ctx, paneID)
	if err != nil {
		return "", fmt.Errorf("capturing pane %s: %w", paneID, err)
	}
	pane := tmux.PaneInfo{ID: paneID}
	return s.Fingerprint(ctx, pane, buf)
}

func (r *Registry) getSentinel(name string) Sentinel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.sentinels[name]; ok && !r.disabled[name] {
		return s
	}
	return r.sentinels["generic"]
}

func (r *Registry) capture(ctx context.Context, paneID string) (string, error) {
	if r.capturePaneFn != nil {
		return r.capturePaneFn(ctx, paneID)
	}
	return capturePane(ctx, paneID)
}

// capturePane runs tmux capture-pane -p -t <paneID> and returns the output.
func capturePane(ctx context.Context, paneID string) (string, error) {
	out, err := exec.CommandContext(ctx, "tmux", "capture-pane", "-p", "-t", paneID).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ParseRouteOverrides parses a comma-separated "cmd=sentinel,..." string
// into a map[string]string. Invalid pairs are silently skipped.
func ParseRouteOverrides(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k != "" && v != "" {
			m[k] = v
		}
	}
	return m
}
