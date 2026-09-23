package tmux

import (
	"context"
	"strings"
	"testing"
)

// fakeExec implements Executor for tests, returning scripted responses.
type fakeExec struct {
	responses map[string]string // key: first arg (subcommand), value: output
}

func (f *fakeExec) Run(_ context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if out, ok := f.responses[strings.Join(args, " ")]; ok {
		return out, nil
	}
	// Fallback: match on just the subcommand.
	if out, ok := f.responses[args[0]]; ok {
		return out, nil
	}
	return "", nil
}

func TestListAllPanes(t *testing.T) {
	t.Parallel()
	fake := &fakeExec{responses: map[string]string{
		"list-panes -a -F #{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_current_path}|#{pane_current_command}|#{pane_tty}|#{pane_pid}": "%1|main|0|0|/home/user/repo|zsh|/dev/pts/0|1234\n%2|work|1|1|/tmp|vim|/dev/pts/1|5678",
	}}
	c := New(fake)
	panes, err := c.ListAllPanes(context.Background())
	if err != nil {
		t.Fatalf("ListAllPanes: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("want 2 panes, got %d", len(panes))
	}
	tests := []struct {
		idx  int
		want PaneInfo
	}{
		{0, PaneInfo{ID: "%1", Session: "main", Window: 0, Pane: 0, Path: "/home/user/repo", Command: "zsh", TTY: "/dev/pts/0", PID: 1234}},
		{1, PaneInfo{ID: "%2", Session: "work", Window: 1, Pane: 1, Path: "/tmp", Command: "vim", TTY: "/dev/pts/1", PID: 5678}},
	}
	for _, tt := range tests {
		got := panes[tt.idx]
		if got != tt.want {
			t.Errorf("pane[%d]: got %+v, want %+v", tt.idx, got, tt.want)
		}
	}
}

func TestListAllPanes_Empty(t *testing.T) {
	t.Parallel()
	c := New(&fakeExec{responses: map[string]string{}})
	panes, err := c.ListAllPanes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(panes) != 0 {
		t.Fatalf("want 0 panes, got %d", len(panes))
	}
}

func TestListAllPanes_MalformedLines(t *testing.T) {
	t.Parallel()
	fake := &fakeExec{responses: map[string]string{
		"list-panes -a -F #{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_current_path}|#{pane_current_command}|#{pane_tty}|#{pane_pid}": "%1|main|0|0|/repo|zsh|/dev/pts/0|999\nbad-line\n",
	}}
	c := New(fake)
	panes, err := c.ListAllPanes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(panes) != 1 {
		t.Fatalf("want 1 pane (malformed skipped), got %d", len(panes))
	}
}

func TestFocusedPaneIDs_FromClients(t *testing.T) {
	t.Parallel()
	fake := &fakeExec{responses: map[string]string{
		"list-clients -F #{pane_id}": "%3\n%7",
	}}
	c := New(fake)
	ids, err := c.FocusedPaneIDs(context.Background())
	if err != nil {
		t.Fatalf("FocusedPaneIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != "%3" || ids[1] != "%7" {
		t.Errorf("unexpected ids: %v", ids)
	}
}

func TestPaneExists(t *testing.T) {
	t.Parallel()
	fake := &fakeExec{responses: map[string]string{
		"display-message -p -t %5 #{pane_id}": "%5",
	}}
	c := New(fake)
	exists, err := c.PaneExists(context.Background(), "%5")
	if err != nil {
		t.Fatalf("PaneExists: %v", err)
	}
	if !exists {
		t.Errorf("expected pane %%5 to exist")
	}
}

func TestPaneExists_Missing(t *testing.T) {
	t.Parallel()
	c := New(&fakeExec{responses: map[string]string{}})
	exists, err := c.PaneExists(context.Background(), "%99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Errorf("expected pane %%99 to not exist")
	}
}

func TestListSessions(t *testing.T) {
	t.Parallel()
	fake := &fakeExec{responses: map[string]string{
		"list-sessions -F #{session_name}|#{session_windows}|#{session_attached}|#{pane_current_path}|#{pane_current_command}|#{window_name}": "main|3|1|/home/user|zsh|editor\nwork|1|0|/tmp|bash|shell",
	}}
	c := New(fake)
	sessions, err := c.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(sessions))
	}
	if sessions[0].Name != "main" || sessions[0].Windows != 3 || !sessions[0].Attached {
		t.Errorf("session[0] mismatch: %+v", sessions[0])
	}
	if sessions[1].Name != "work" || sessions[1].Attached {
		t.Errorf("session[1] mismatch: %+v", sessions[1])
	}
}

func TestSplitLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  []string
	}{
		{"a\nb\nc", []string{"a", "b", "c"}},
		{"a\n\nb\n", []string{"a", "b"}},
		{"", nil},
		{"  \n  ", nil},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitLines(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitLines(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
