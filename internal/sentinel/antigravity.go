package sentinel

import (
	"context"
	"crypto/md5"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

var (
	// Blocked / Waiting cues:
	reSubagentBlocked = regexp.MustCompile(`(?m)^\s*●\s+Agent\(.*Blocked`)
	reApprovalHeader  = regexp.MustCompile(`(?m)^\s*[┃|]\s*.*needs\s+approval\s+for`)
	rePermPrompt      = regexp.MustCompile(`(Requesting\s+permission\s+for|Run\s+this\s+command\?)`)
	reCmdFinished     = regexp.MustCompile(`Command\s+finished`)

	// Running cues:
	reEscCancel       = regexp.MustCompile(`esc\s+to\s+cancel`)
	reSubagentRunning = regexp.MustCompile(`(?m)^\s*●\s+Agent\(.*Running`)

	// Idle cues:
	reShortcuts = regexp.MustCompile(`\?\s+for\s+shortcuts`)
	rePromptGtr = regexp.MustCompile(`^\s*>\s*$`)

	// Normalizer cues:
	reSpinnerLine = regexp.MustCompile(`^\s*[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷]\s`)
	reThinking    = regexp.MustCompile(`Thinking\.\.\..*`)

	// Process disambiguation (Issue #3)
	reProcessMatch = regexp.MustCompile(`(^|[/\s])(agy|antigravity)(\s|$)`)
)

// Antigravity monitors Google DeepMind Antigravity sessions.
type Antigravity struct {
	processInspector tmux.ProcessInspector
}

// NewAntigravity returns a new native Antigravity sentinel.
func NewAntigravity(pi tmux.ProcessInspector) *Antigravity {
	if pi == nil {
		pi = tmux.DefaultProcessInspector
	}
	return &Antigravity{processInspector: pi}
}

func (a *Antigravity) Name() string {
	return "antigravity"
}

func (a *Antigravity) Matches(pane tmux.PaneInfo) bool {
	// 1. Direct command match
	if pane.Command == "agy" || pane.Command == "antigravity" {
		return true
	}

	// 2. Generic command disambiguation (Issue #3)
	// If the command is a generic wrapper (e.g. cli, agent, run, node),
	// inspect process tree cmdlines.
	if pane.PID > 1 && (pane.Command == "cli" || pane.Command == "agent" || pane.Command == "run") {
		cmdlines, err := a.processInspector(context.Background(), pane.PID)
		if err == nil {
			for _, line := range cmdlines {
				if reProcessMatch.MatchString(line) {
					return true
				}
			}
		}
	}

	return false
}

func (a *Antigravity) Classify(_ context.Context, pane tmux.PaneInfo, buffer string) (Classification, error) {
	lines := strings.Split(buffer, "\n")
	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}

	if len(nonEmpty) == 0 {
		return Classification{State: "unknown", Label: filepath.Base(pane.Path)}, nil
	}

	lastLine := nonEmpty[len(nonEmpty)-1]
	startTail := 0
	if len(nonEmpty) > 15 {
		startTail = len(nonEmpty) - 15
	}
	tailText := strings.Join(nonEmpty[startTail:], "\n")

	// 1. Blocked / Waiting cues:
	// A) Subagent explicitly reported Blocked in lifecycle widget: e.g. "● Agent(...) Blocked · ..."
	// B) Approval card header: e.g. "┃ self needs approval for Bash"
	// C) Interactive permission prompt: "Requesting permission for" or "Run this command?" (not followed by Command finished)
	isWaiting := reSubagentBlocked.MatchString(tailText) ||
		reApprovalHeader.MatchString(tailText) ||
		(rePermPrompt.MatchString(tailText) && !reCmdFinished.MatchString(tailText))

	if isWaiting {
		return Classification{
			State: "waiting",
			Label: fmt.Sprintf("waiting for confirmation in %s", filepath.Base(pane.Path)),
		}, nil
	}

	// 2. Running cues:
	if reEscCancel.MatchString(lastLine) || reSubagentRunning.MatchString(tailText) {
		return Classification{
			State: "running",
			Label: fmt.Sprintf("running in %s", filepath.Base(pane.Path)),
		}, nil
	}

	// 3. Idle cues:
	if reShortcuts.MatchString(lastLine) || rePromptGtr.MatchString(lastLine) {
		return Classification{
			State: "idle",
			Label: fmt.Sprintf("idle in %s", filepath.Base(pane.Path)),
		}, nil
	}

	return Classification{
		State: "unknown",
		Label: filepath.Base(pane.Path),
	}, nil
}

func (a *Antigravity) Fingerprint(_ context.Context, _ tmux.PaneInfo, buffer string) (string, error) {
	// Intelligent Screen Normalizer:
	// Filters braille thinking spinners ([⠋⠙⠹...]) and in-place 'Thinking...' text
	// before computing hash, ensuring thought stream updates don't trigger unread alarms.
	lines := strings.Split(buffer, "\n")
	var normalized []string
	for _, l := range lines {
		if reSpinnerLine.MatchString(l) {
			continue
		}
		clean := reThinking.ReplaceAllString(l, "")
		normalized = append(normalized, clean)
	}
	content := strings.Join(normalized, "\n")
	sum := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", sum), nil
}
