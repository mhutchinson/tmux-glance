// Package sentinel manages the routing table (command → sentinel name) and
// dispatches classify/fingerprint calls to bash sentinel scripts via os/exec.
// The bash sentinel API is unchanged — existing sentinels/*.sh files work
// without modification.
package sentinel

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Classification is the result of calling sentinel_<name>_classify.
type Classification struct {
	State string // e.g. "waiting", "running", "done", "idle", "unknown"
	Label string
}

// Sentinel represents a discovered sentinel plugin backed by a shell script.
type Sentinel struct {
	Name       string   // e.g. "antigravity"
	ScriptPath string   // absolute path to the .sh file (empty for built-ins)
	Commands   []string // default command names that map directly to this sentinel
}

// Registry holds the loaded sentinels and their command routing table.
type Registry struct {
	sentinels      []Sentinel
	mu             sync.RWMutex
	commandMap     map[string]string // cmd → sentinel name (cached)
	disabled       map[string]bool
	routeOverrides map[string]string
	// capturePaneFn allows tests to mock tmux capture-pane calls.
	capturePaneFn func(ctx context.Context, paneID string) (string, error)
}

// builtinSentinels defines the known default command lists for bundled sentinels.
// These are consulted before any dynamic script-based resolution.
var builtinCommands = map[string][]string{
	"antigravity": {"agy", "antigravity"},
	"generic":     {},
}

// New discovers sentinels from searchDirs (first found per name wins) and
// returns a Registry ready to route commands.
func New(searchDirs []string, disabled []string, routeOverrides map[string]string) (*Registry, error) {
	r := &Registry{
		commandMap:     make(map[string]string),
		disabled:       make(map[string]bool),
		routeOverrides: routeOverrides,
	}
	for _, d := range disabled {
		r.disabled[strings.TrimSpace(d)] = true
	}

	// Discover sentinel scripts; first-found per name wins.
	seen := make(map[string]bool)
	for _, dir := range searchDirs {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // non-existent or unreadable directories are silently skipped
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sh") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".sh")
			if name == "generic" || seen[name] {
				continue // generic is always last; skip duplicates
			}
			seen[name] = true
			cmds, _ := builtinCommands[name] // use known list; empty for unknown scripts
			r.sentinels = append(r.sentinels, Sentinel{
				Name:       name,
				ScriptPath: filepath.Join(dir, entry.Name()),
				Commands:   cmds,
			})
		}
	}

	// Always append generic as the final fallback (no script path needed).
	r.sentinels = append(r.sentinels, Sentinel{Name: "generic"})

	// Pre-populate the routing table from sentinel default commands.
	for _, s := range r.sentinels {
		if r.disabled[s.Name] {
			continue
		}
		for _, cmd := range s.Commands {
			if cmd != "" {
				r.commandMap[cmd] = s.Name
			}
		}
	}

	// Apply user route overrides (e.g. "chat=generic,my-cli=antigravity").
	for cmd, target := range routeOverrides {
		r.commandMap[strings.TrimSpace(cmd)] = strings.TrimSpace(target)
	}

	return r, nil
}

// Resolve returns the sentinel name for a given command string.
// Returns "generic" for empty commands or if no specific sentinel matches.
// Results are cached in commandMap for O(1) repeat lookups.
func (r *Registry) Resolve(ctx context.Context, cmd string) string {
	if cmd == "" {
		return "generic"
	}

	r.mu.RLock()
	if name, ok := r.commandMap[cmd]; ok {
		r.mu.RUnlock()
		if r.disabled[name] {
			return "generic"
		}
		return name
	}
	r.mu.RUnlock()

	// Dynamic fallback: call sentinel_<name>_matches via bash subprocess.
	for _, s := range r.sentinels {
		if s.Name == "generic" || r.disabled[s.Name] || s.ScriptPath == "" {
			continue
		}
		if r.bashMatches(ctx, s, cmd, "") {
			r.mu.Lock()
			r.commandMap[cmd] = s.Name
			r.mu.Unlock()
			return s.Name
		}
	}

	r.mu.Lock()
	r.commandMap[cmd] = "generic"
	r.mu.Unlock()
	return "generic"
}

// Classify calls the appropriate sentinel's classify function for the given pane.
// The generic sentinel is handled in pure Go; others delegate to a bash subprocess.
func (r *Registry) Classify(ctx context.Context, name, paneID, path, cmd string) (Classification, error) {
	if name == "generic" || name == "" {
		return Classification{
			State: "watching",
			Label: cmd + " in " + filepath.Base(path),
		}, nil
	}
	s := r.find(name)
	if s == nil || s.ScriptPath == "" {
		return Classification{State: "unknown", Label: name}, nil
	}
	out, err := r.bashCall(ctx, s.ScriptPath, name, "classify", paneID, path, cmd)
	if err != nil {
		return Classification{State: "unknown"}, fmt.Errorf("sentinel %s classify: %w", name, err)
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	c := Classification{State: strings.TrimSpace(parts[0])}
	if len(parts) > 1 {
		c.Label = strings.TrimSpace(parts[1])
	}
	return c, nil
}

// Fingerprint returns an MD5 hex digest of the (normalised) pane buffer.
// For the generic sentinel, it uses crypto/md5 directly — no external binary.
// For named sentinels, it delegates to sentinel_<name>_fingerprint via bash.
func (r *Registry) Fingerprint(ctx context.Context, name, paneID string) (string, error) {
	if name == "generic" || name == "" {
		return r.genericFingerprint(ctx, paneID)
	}
	s := r.find(name)
	if s == nil || s.ScriptPath == "" {
		return r.genericFingerprint(ctx, paneID)
	}
	out, err := r.bashCall(ctx, s.ScriptPath, name, "fingerprint", paneID)
	if err != nil {
		// Fall back to generic fingerprint on error.
		return r.genericFingerprint(ctx, paneID)
	}
	hash := strings.TrimSpace(out)
	if hash == "" {
		return r.genericFingerprint(ctx, paneID)
	}
	return hash, nil
}

// genericFingerprint captures a pane and returns the MD5 of its content.
func (r *Registry) genericFingerprint(ctx context.Context, paneID string) (string, error) {
	var content string
	var err error
	if r.capturePaneFn != nil {
		content, err = r.capturePaneFn(ctx, paneID)
	} else {
		content, err = capturePane(ctx, paneID)
	}
	if err != nil {
		return "", fmt.Errorf("capturing pane %s: %w", paneID, err)
	}
	sum := md5.Sum([]byte(content)) //nolint:gosec // MD5 used only for change detection, not security
	return fmt.Sprintf("%x", sum), nil
}

// capturePane runs tmux capture-pane -p -t <paneID> and returns the output.
func capturePane(ctx context.Context, paneID string) (string, error) {
	out, err := exec.CommandContext(ctx, "tmux", "capture-pane", "-p", "-t", paneID).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// bashMatches calls sentinel_<name>_matches via a bash subprocess.
// Returns true if exit code is 0.
func (r *Registry) bashMatches(ctx context.Context, s Sentinel, cmd, paneID string) bool {
	args := []string{"-c",
		fmt.Sprintf("source %q; sentinel_%s_matches \"$@\"", s.ScriptPath, s.Name),
		"_", cmd,
	}
	if paneID != "" {
		args = append(args, paneID)
	}
	err := exec.CommandContext(ctx, "bash", args...).Run()
	return err == nil
}

// bashCall invokes a sentinel function (classify or fingerprint) via bash.
func (r *Registry) bashCall(ctx context.Context, scriptPath, name, fn string, extraArgs ...string) (string, error) {
	bashScript := fmt.Sprintf("source %q; sentinel_%s_%s \"$@\"", scriptPath, name, fn)
	args := append([]string{"-c", bashScript, "_"}, extraArgs...)
	cmd := exec.CommandContext(ctx, "bash", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("bash sentinel_%s_%s: %w", name, fn, err)
	}
	return string(out), nil
}

// find returns the Sentinel with the given name, or nil.
func (r *Registry) find(name string) *Sentinel {
	for i := range r.sentinels {
		if r.sentinels[i].Name == name {
			return &r.sentinels[i]
		}
	}
	return nil
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
