// Package tmux provides a typed, context-aware wrapper around tmux CLI commands.
// All tmux interactions in the engine use this package; no other package
// shells tmux directly.
package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PaneInfo holds live metadata for a single tmux pane.
type PaneInfo struct {
	ID      string // e.g. "%5"
	Session string
	Window  int
	Pane    int
	Path    string
	Command string
	TTY     string
}

// SessionInfo holds live metadata for a tmux session.
type SessionInfo struct {
	Name     string
	Windows  int
	Attached bool
	Path     string  // active pane current path
	Command  string  // active pane current command
	WinName  string  // active window name
}

// Executor is the interface for running tmux subcommands.
// The default implementation uses exec.CommandContext; tests inject fakes.
type Executor interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// realExec is the production Executor backed by the real tmux binary.
type realExec struct{}

func (realExec) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Client wraps an Executor and exposes typed tmux operations.
type Client struct {
	exec Executor
}

// New returns a Client using the provided Executor.
func New(e Executor) *Client {
	return &Client{exec: e}
}

// Default is a package-level Client backed by the real tmux binary.
var Default = New(realExec{})

// run is a convenience wrapper that swallows "command not found / tmux not running"
// style errors into empty string + nil when ignoreMissing is true.
func (c *Client) run(ctx context.Context, ignoreMissing bool, args ...string) (string, error) {
	out, err := c.exec.Run(ctx, args...)
	if err != nil {
		if ignoreMissing {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

// ListAllPanes returns PaneInfo for every pane on the tmux server.
func (c *Client) ListAllPanes(ctx context.Context) ([]PaneInfo, error) {
	format := "#{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_current_path}|#{pane_current_command}|#{pane_tty}"
	out, err := c.run(ctx, true, "list-panes", "-a", "-F", format)
	if err != nil || out == "" {
		return nil, err
	}
	var panes []PaneInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 7)
		if len(parts) < 7 {
			continue
		}
		win, _ := strconv.Atoi(parts[2])
		pane, _ := strconv.Atoi(parts[3])
		panes = append(panes, PaneInfo{
			ID:      parts[0],
			Session: parts[1],
			Window:  win,
			Pane:    pane,
			Path:    parts[4],
			Command: parts[5],
			TTY:     parts[6],
		})
	}
	return panes, nil
}

// ListSessions returns metadata for all active tmux sessions.
func (c *Client) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	format := "#{session_name}|#{session_windows}|#{session_attached}|#{pane_current_path}|#{pane_current_command}|#{window_name}"
	out, err := c.run(ctx, true, "list-sessions", "-F", format)
	if err != nil || out == "" {
		return nil, err
	}
	var sessions []SessionInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}
		wins, _ := strconv.Atoi(parts[1])
		attached, _ := strconv.Atoi(parts[2])
		sessions = append(sessions, SessionInfo{
			Name:     parts[0],
			Windows:  wins,
			Attached: attached > 0,
			Path:     parts[3],
			Command:  parts[4],
			WinName:  parts[5],
		})
	}
	return sessions, nil
}

// FocusedPaneIDs returns the pane IDs currently focused by attached clients.
// Falls back to active session panes when no clients are attached.
func (c *Client) FocusedPaneIDs(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, true, "list-clients", "-F", "#{pane_id}")
	if err == nil && out != "" {
		return splitLines(out), nil
	}
	// Fallback: use session active panes.
	out, err = c.run(ctx, true, "list-sessions", "-F", "#{pane_id}")
	if err != nil || out == "" {
		return nil, err
	}
	return splitLines(out), nil
}

// PaneExists returns true if the given pane ID is alive on the server.
func (c *Client) PaneExists(ctx context.Context, paneID string) (bool, error) {
	got, err := c.GetPaneField(ctx, paneID, "#{pane_id}")
	if err != nil {
		return false, nil //nolint:nilerr // tmux error = pane doesn't exist
	}
	return got == paneID, nil
}

// GetPaneField queries a single tmux format field for a specific pane.
func (c *Client) GetPaneField(ctx context.Context, paneID, format string) (string, error) {
	return c.run(ctx, true, "display-message", "-p", "-t", paneID, format)
}

// GetSessionField queries a single tmux format field for a session target.
func (c *Client) GetSessionField(ctx context.Context, target, format string) (string, error) {
	return c.run(ctx, true, "display-message", "-p", "-t", target, format)
}

// CapturePaneRaw returns the visible text of a pane buffer.
func (c *Client) CapturePaneRaw(ctx context.Context, paneID string) (string, error) {
	return c.run(ctx, true, "capture-pane", "-p", "-t", paneID)
}

// CapturePaneANSI returns the visible text of a pane buffer with ANSI escape codes.
func (c *Client) CapturePaneANSI(ctx context.Context, paneID string) (string, error) {
	return c.run(ctx, true, "capture-pane", "-e", "-p", "-t", paneID)
}

// SetPaneOption sets a pane-local tmux option.
func (c *Client) SetPaneOption(ctx context.Context, paneID, option, value string) error {
	_, err := c.run(ctx, true, "set-option", "-p", "-t", paneID, option, value)
	return err
}

// GetPaneOption gets a pane-local tmux option. Returns "" if unset.
func (c *Client) GetPaneOption(ctx context.Context, paneID, option string) (string, error) {
	return c.run(ctx, true, "show-options", "-pv", "-t", paneID, option)
}

// UnsetPaneOption removes a pane-local tmux option.
func (c *Client) UnsetPaneOption(ctx context.Context, paneID, option string) error {
	_, err := c.run(ctx, true, "set-option", "-u", "-p", "-t", paneID, option)
	return err
}

// SetGlobalOption sets a global tmux server option.
func (c *Client) SetGlobalOption(ctx context.Context, option, value string) error {
	_, err := c.run(ctx, true, "set-option", "-g", option, value)
	return err
}

// GetGlobalOption gets a global tmux option. Returns "" if unset.
func (c *Client) GetGlobalOption(ctx context.Context, option string) (string, error) {
	return c.run(ctx, true, "show-options", "-gv", option)
}

// RefreshClients refreshes the tmux status bar for all attached clients.
func (c *Client) RefreshClients(ctx context.Context) error {
	_, err := c.run(ctx, true, "refresh-client", "-S")
	return err
}

// DisplayMessage shows a notification via tmux display-message.
func (c *Client) DisplayMessage(ctx context.Context, msg string) error {
	_, err := c.run(ctx, true, "display-message", msg)
	return err
}

// SelectPane switches focus to a pane (select-pane + select-window + switch-client).
func (c *Client) SelectPane(ctx context.Context, paneID string) error {
	c.run(ctx, true, "select-pane", "-t", paneID)   //nolint:errcheck
	c.run(ctx, true, "select-window", "-t", paneID) //nolint:errcheck
	_, err := c.run(ctx, true, "switch-client", "-t", paneID)
	return err
}

// SwitchClient switches the tmux client to a target session or pane.
func (c *Client) SwitchClient(ctx context.Context, target string) error {
	_, err := c.run(ctx, true, "switch-client", "-t", target)
	return err
}

// HasSession returns true if a named tmux session exists.
func (c *Client) HasSession(ctx context.Context, session string) (bool, error) {
	_, err := c.exec.Run(ctx, "has-session", "-t", session)
	if err != nil {
		return false, nil //nolint:nilerr // non-zero exit = session doesn't exist
	}
	return true, nil
}

// SessionActivePaneID returns the currently active pane ID in a named session.
func (c *Client) SessionActivePaneID(ctx context.Context, session string) (string, error) {
	return c.GetSessionField(ctx, session, "#{pane_id}")
}

// splitLines splits newline-delimited output into non-empty trimmed strings.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
