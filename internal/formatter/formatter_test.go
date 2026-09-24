package formatter

import (
	"strings"
	"testing"

	"github.com/mhutchinson/tmux-glance/internal/slots"
	"github.com/mhutchinson/tmux-glance/internal/state"
	"github.com/mhutchinson/tmux-glance/internal/tmux"
	"path/filepath"
)

// nopResolver always returns "generic".
type nopResolver struct{}

func (nopResolver) Resolve(cmd string) string { return "generic" }

// agentResolver returns "antigravity" for "agy".
type agentResolver struct{}

func (agentResolver) Resolve(cmd string) string {
	if cmd == "agy" {
		return "antigravity"
	}
	return "generic"
}

func makeEntry(paneID string, kind state.Kind, s state.PaneState) state.Entry {
	return state.Entry{
		PaneID:  paneID,
		Session: "main",
		Window:  0,
		Pane:    0,
		Path:    "/home/user/repo",
		Command: "zsh",
		Label:   "zsh in repo",
		Kind:    kind,
		State:   s,
	}
}

func TestAttentionList_Empty(t *testing.T) {
	t.Parallel()
	out := AttentionList(nil)
	if !strings.Contains(out, "Empty") {
		t.Errorf("empty attention list should contain 'Empty', got:\n%s", out)
	}
}

func TestAttentionList_AlertFirst(t *testing.T) {
	t.Parallel()
	entries := []state.Entry{
		makeEntry("%1", state.KindManual, state.StateWatching),
		makeEntry("%2", state.KindManual, state.StateAlert),
		makeEntry("%3", state.KindAuto, state.StateRunning),
	}
	out := AttentionList(entries)

	// Alert should appear before Vigil.
	alertIdx := strings.Index(out, "🚨")
	vigilIdx := strings.Index(out, "👁️")
	if alertIdx < 0 || vigilIdx < 0 {
		t.Fatalf("expected both 🚨 and 👁️ in output:\n%s", out)
	}
	if alertIdx > vigilIdx {
		t.Errorf("alert badge should appear before vigil badge (alert at %d, vigil at %d)", alertIdx, vigilIdx)
	}
}

func TestAttentionList_TabDelimiter(t *testing.T) {
	t.Parallel()
	entries := []state.Entry{makeEntry("%5", state.KindManual, state.StateAlert)}
	out := AttentionList(entries)
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		t.Fatal("expected at least one output line")
	}
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			t.Errorf("expected exactly 2 tab-delimited fields, got %d: %q", len(parts), line)
		}
		if parts[1] != "%5" {
			t.Errorf("second field should be pane ID %%5, got %q", parts[1])
		}
	}
}

func TestAttentionList_AgentBadges(t *testing.T) {
	t.Parallel()
	entries := []state.Entry{
		makeEntry("%1", state.KindAuto, state.StateWaiting),
		makeEntry("%2", state.KindAuto, state.StateDone),
		makeEntry("%3", state.KindAuto, state.StateRunning),
	}
	out := AttentionList(entries)
	for _, badge := range []string{"⏳", "✓", "⚡"} {
		if !strings.Contains(out, badge) {
			t.Errorf("expected badge %q in output:\n%s", badge, out)
		}
	}
}

func TestAllPanesList_Empty(t *testing.T) {
	t.Parallel()
	out := AllPanesList(nil, nil, nopResolver{})
	if !strings.Contains(out, "Empty") {
		t.Errorf("no panes → should show Empty row, got:\n%s", out)
	}
}

func TestAllPanesList_GlobalIncludesAllPanes(t *testing.T) {
	t.Parallel()
	panes := []tmux.PaneInfo{
		{ID: "%1", Session: "s", Window: 0, Pane: 0, Path: "/repo", Command: "agy"},
		{ID: "%2", Session: "s", Window: 0, Pane: 1, Path: "/other", Command: "zsh"},
	}
	out := AllPanesList(nil, panes, agentResolver{})
	if !strings.Contains(out, "%1") {
		t.Errorf("agent pane %%1 should appear in all-panes list:\n%s", out)
	}
	if !strings.Contains(out, "%2") {
		t.Errorf("generic pane %%2 should ALSO appear in global all-panes list:\n%s", out)
	}
	if !strings.Contains(out, "💻 Pane") {
		t.Errorf("generic pane should have '💻 Pane' badge:\n%s", out)
	}

	idx1 := strings.Index(out, "%1")
	idx2 := strings.Index(out, "%2")
	if idx1 > idx2 {
		t.Errorf("agent pane %%1 should sort before generic pane %%2:\n%s", out)
	}
}

func TestBotsList_FiltersGenericPanes(t *testing.T) {
	t.Parallel()
	panes := []tmux.PaneInfo{
		{ID: "%1", Session: "s", Window: 0, Pane: 0, Path: "/repo", Command: "agy"},
		{ID: "%2", Session: "s", Window: 0, Pane: 1, Path: "/other", Command: "zsh"},
	}
	out := BotsList(nil, panes, agentResolver{})
	if !strings.Contains(out, "%1") {
		t.Errorf("agent pane %%1 should appear in bots list:\n%s", out)
	}
	if strings.Contains(out, "%2") {
		t.Errorf("generic pane %%2 should NOT appear in bots list:\n%s", out)
	}
}

func TestSessionsList_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ss, _ := slots.NewStore(filepath.Join(dir, "slots"))
	out := SessionsList(nil, nil, ss)
	if !strings.Contains(out, "Empty") {
		t.Errorf("empty sessions list should contain 'Empty', got:\n%s", out)
	}
}

func TestSessionsList_SlotBadge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ss, _ := slots.NewStore(filepath.Join(dir, "slots"))
	ss.Assign("h", "work") //nolint:errcheck

	sessions := []tmux.SessionInfo{
		{Name: "main", Windows: 1, Path: "/home", Command: "zsh", WinName: "win"},
		{Name: "work", Windows: 2, Path: "/work", Command: "agy", WinName: "dev"},
	}
	out := SessionsList(nil, sessions, ss)
	if !strings.Contains(out, "[H]") {
		t.Errorf("expected slot badge [H] for session 'work':\n%s", out)
	}
}

func TestSessionsList_AlertPriority(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ss, _ := slots.NewStore(filepath.Join(dir, "slots"))

	entries := []state.Entry{
		{PaneID: "%1", Session: "quiet", Kind: state.KindManual, State: state.StateWatching},
		{PaneID: "%2", Session: "loud", Kind: state.KindManual, State: state.StateAlert},
	}
	sessions := []tmux.SessionInfo{
		{Name: "quiet", Windows: 1, Path: "/q", Command: "zsh", WinName: "w"},
		{Name: "loud", Windows: 1, Path: "/l", Command: "zsh", WinName: "w"},
	}
	out := SessionsList(entries, sessions, ss)
	loudIdx := strings.Index(out, "loud")
	quietIdx := strings.Index(out, "quiet")
	if loudIdx < 0 || quietIdx < 0 {
		t.Fatalf("both sessions should appear:\n%s", out)
	}
	if loudIdx > quietIdx {
		t.Errorf("alert session 'loud' should sort before 'quiet'")
	}
}

func TestHistoryList_Empty(t *testing.T) {
	t.Parallel()
	out := HistoryList(nil, nil, "", func(string) tmux.PaneInfo { return tmux.PaneInfo{} })
	if !strings.Contains(out, "Empty") {
		t.Errorf("empty history should contain 'Empty', got:\n%s", out)
	}
}

func TestHistoryList_Order(t *testing.T) {
	t.Parallel()
	paneMap := map[string]tmux.PaneInfo{
		"%fwd": {ID: "%fwd", Session: "s", Window: 0, Pane: 0, Path: "/fwd", Command: "zsh"},
		"%cur": {ID: "%cur", Session: "s", Window: 0, Pane: 1, Path: "/cur", Command: "zsh"},
		"%bk":  {ID: "%bk", Session: "s", Window: 0, Pane: 2, Path: "/bk", Command: "zsh"},
	}
	out := HistoryList(
		[]string{"%fwd"},
		[]string{"%bk"},
		"%cur",
		func(id string) tmux.PaneInfo { return paneMap[id] },
	)
	fwdIdx := strings.Index(out, "FWD")
	curIdx := strings.Index(out, "CUR")
	bkIdx := strings.Index(out, "BACK")
	if fwdIdx < 0 || curIdx < 0 || bkIdx < 0 {
		t.Fatalf("expected FWD, CUR, BACK badges:\n%s", out)
	}
	if !(fwdIdx < curIdx && curIdx < bkIdx) {
		t.Errorf("expected FWD < CUR < BACK in output order, got positions %d %d %d", fwdIdx, curIdx, bkIdx)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
