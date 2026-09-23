package sentinel

import (
	"context"
	"testing"

	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

func TestAntigravity_Normalizer(t *testing.T) {
	t.Parallel()
	a := NewAntigravity(nil)

	buf1 := `Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
⠋ Thinking... inspecting repository dependencies
? for shortcuts
`
	buf2 := `Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
⣾ Thinking... synthesizing response from neural model
? for shortcuts
`
	buf3 := `Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
⣽ Thinking... checking git status
? for shortcuts
`
	buf4 := `Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
Requesting permission for run_command: git status
? for shortcuts
`

	pane := tmux.PaneInfo{ID: "%1", Path: "/Users/mhutchinson/project"}

	h1, err := a.Fingerprint(context.Background(), pane, buf1)
	if err != nil {
		t.Fatalf("Fingerprint buf1: %v", err)
	}
	h2, err := a.Fingerprint(context.Background(), pane, buf2)
	if err != nil {
		t.Fatalf("Fingerprint buf2: %v", err)
	}
	h3, err := a.Fingerprint(context.Background(), pane, buf3)
	if err != nil {
		t.Fatalf("Fingerprint buf3: %v", err)
	}
	h4, err := a.Fingerprint(context.Background(), pane, buf4)
	if err != nil {
		t.Fatalf("Fingerprint buf4: %v", err)
	}

	if h1 != h2 {
		t.Errorf("expected h1 == h2, got %q vs %q", h1, h2)
	}
	if h2 != h3 {
		t.Errorf("expected h2 == h3, got %q vs %q", h2, h3)
	}
	if h1 == h4 {
		t.Errorf("expected h1 != h4, got identical hash %q", h1)
	}
}

func TestAntigravity_Classify(t *testing.T) {
	t.Parallel()
	a := NewAntigravity(nil)
	pane := tmux.PaneInfo{ID: "%1", Path: "/Users/mhutchinson/project"}

	tests := []struct {
		name      string
		buffer    string
		wantState string
	}{
		{
			name: "Subagent blocked cue",
			buffer: `● Agent(self: System Inspector)(Please run a non-interactive top comma...)
  I've spawned a subagent.
───────────────────────────────────────────────────────────────────────────────────────────
  ● Agent(self)  Blocked · Running command · 6s
───────────────────────────────────────────────────────────────────────────────────────────
? for shortcuts                       accept-edits · Gemini 3.8 Flash · low`,
			wantState: "waiting",
		},
		{
			name: "Approval card cue",
			buffer: ` ┃ self needs approval for Bash
 ┃ ─────────────────────────────────────────────────────────────────────────────────────
 ┃ ● Bash(top -l 1 -n 5 -o cpu)
 ┃ ctrl+y approve · alt+j manage
───────────────────────────────────────────────────────────────────────────────────────────
? for shortcuts`,
			wantState: "waiting",
		},
		{
			name: "Permission prompt cue",
			buffer: `Requesting permission for run_command: git commit
1. Yes, run command
2. No, cancel
? for shortcuts`,
			wantState: "waiting",
		},
		{
			name: "Permission prompt finished does not block",
			buffer: `Requesting permission for run_command: git commit
Command finished with exit code 0
? for shortcuts`,
			wantState: "idle",
		},
		{
			name: "Esc to cancel is running",
			buffer: `Thinking... analyzing code
esc to cancel`,
			wantState: "running",
		},
		{
			name: "Subagent running cue",
			buffer: `● Agent(self)  Running · Processing steps · 12s
esc to cancel`,
			wantState: "running",
		},
		{
			name: "Shortcuts footer is idle",
			buffer: `Ready for next task.
? for shortcuts`,
			wantState: "idle",
		},
		{
			name: "Bare prompt is idle",
			buffer: `Ready.
> `,
			wantState: "idle",
		},
		{
			name:      "Unknown state",
			buffer:    `Compiling package main...`,
			wantState: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cls, err := a.Classify(context.Background(), pane, tt.buffer)
			if err != nil {
				t.Fatalf("Classify failed: %v", err)
			}
			if cls.State != tt.wantState {
				t.Errorf("Classify(%s) = %q, want %q", tt.name, cls.State, tt.wantState)
			}
		})
	}
}

func TestAntigravity_Matches_Issue3(t *testing.T) {
	t.Parallel()

	// Inspector returning an agy child process
	agyInspector := func(_ context.Context, _ int) ([]string, error) {
		return []string{"-zsh", "/usr/local/bin/agy -c"}, nil
	}
	// Inspector returning non-agy process (e.g. AWS CLI)
	awsInspector := func(_ context.Context, _ int) ([]string, error) {
		return []string{"-zsh", "aws ec2 describe-instances"}, nil
	}

	aAgy := NewAntigravity(agyInspector)
	aAws := NewAntigravity(awsInspector)

	// Direct command matches without PID inspection
	if !aAgy.Matches(tmux.PaneInfo{Command: "agy"}) {
		t.Errorf("expected 'agy' to match")
	}
	if !aAgy.Matches(tmux.PaneInfo{Command: "antigravity"}) {
		t.Errorf("expected 'antigravity' to match")
	}

	// Ambiguous command "cli" with agy process tree matches
	if !aAgy.Matches(tmux.PaneInfo{Command: "cli", PID: 1234}) {
		t.Errorf("expected 'cli' with agy process tree to match antigravity (Issue #3)")
	}

	// Ambiguous command "cli" with aws process tree does NOT match
	if aAws.Matches(tmux.PaneInfo{Command: "cli", PID: 5678}) {
		t.Errorf("expected 'cli' with aws process tree NOT to match antigravity (Issue #3)")
	}

	// Unrelated command does not match
	if aAgy.Matches(tmux.PaneInfo{Command: "vim", PID: 1234}) {
		t.Errorf("expected 'vim' NOT to match antigravity")
	}
}
