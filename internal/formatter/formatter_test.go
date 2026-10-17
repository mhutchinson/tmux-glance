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
	if fwdIdx >= curIdx || curIdx >= bkIdx {
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

func TestTruncate_RuneWidth(t *testing.T) {
	t.Parallel()
	// Each emoji is 1 rune but 3-4 bytes. Byte-based truncation would
	// cut mid-codepoint and produce a garbled or too-short result.
	s := "🚨 waiting for confirmation in repo"
	got := truncate(s, 20)
	runes := []rune(got)
	if len(runes) > 20 {
		t.Errorf("truncate produced %d runes, want ≤ 20: %q", len(runes), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncate should end with '...', got %q", got)
	}
	// Verify the result is valid UTF-8 (byte slicing would break this).
	if got != string([]rune(got)) {
		t.Errorf("truncate result is not valid UTF-8: %q", got)
	}
}

func terminalCellWidth(s string) int {
	w := 0
	for _, r := range s {
		switch r {
		case '🤖', '⏳', '💤', '🚨', '👁':
			w += 2
		case '\ufe0f': // variation selector
			// 0 width
		default:
			w += 1
		}
	}
	return w
}

func TestAttentionList_UnknownStateBadgeAlignment(t *testing.T) {
	t.Parallel()
	entries := []state.Entry{
		makeEntry("%1", state.KindAuto, state.StateWaiting),
		makeEntry("%2", state.KindAuto, state.StateUnknown),
		makeEntry("%3", state.KindAuto, state.StateRunning),
		makeEntry("%4", state.KindAuto, state.StateDone),
	}
	out := AttentionList(entries)
	lines := nonEmptyLines(out)
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got:\n%s", out)
	}

	// Verify that the badge prefix before '│' occupies exactly 14 terminal cells across all states,
	// preventing 'whitespace out of whack' layout distortion when unknown states are encountered.
	const wantWidth = 14
	for i, line := range lines {
		sepIdx := strings.Index(line, "│")
		if sepIdx < 0 {
			t.Fatalf("line %d missing '│' column separator: %s", i, line)
		}
		badge := line[:sepIdx]
		gotWidth := terminalCellWidth(badge)
		if gotWidth != wantWidth {
			t.Errorf("line %d badge %q cell width = %d, want %d", i, badge, gotWidth, wantWidth)
		}
	}
}

func TestFormatStatus(t *testing.T) {
	t.Parallel()

	t.Run("EmptyAllZero", func(t *testing.T) {
		got := FormatStatus(0, 0, 0, 0, 0, "", "")
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("UserAlertAndWatchOnly", func(t *testing.T) {
		got := FormatStatus(1, 2, 0, 0, 0, "", "")
		want := "#[fg=#f38ba8,bold]🚨 1#[default] #[fg=#b4befe]👁️ 2#[default]"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("SingleBotStateRounded", func(t *testing.T) {
		got := FormatStatus(0, 0, 1, 0, 0, "", "rounded")
		if !strings.Contains(got, "") || !strings.Contains(got, "") {
			t.Errorf("expected rounded capsule delimiters, got %q", got)
		}
		if !strings.Contains(got, "🤖 #[fg=#fab387,bold]⏳ 1") {
			t.Errorf("expected waiting badge, got %q", got)
		}
		if strings.Count(got, "🤖") != 1 {
			t.Errorf("expected exactly 1 bot icon, got %d in %q", strings.Count(got, "🤖"), got)
		}
	})

	t.Run("MultipleBotStatesDeduplicated", func(t *testing.T) {
		// Waiting + Running + Done
		got := FormatStatus(0, 0, 1, 2, 3, "#313244", "rounded")
		// Verify single '🤖' glyph
		if strings.Count(got, "🤖") != 1 {
			t.Errorf("expected bot icon deduplicated to 1 instance, got %d in %q", strings.Count(got, "🤖"), got)
		}
		for _, badge := range []string{"⏳ 1", "⚡ 2", "✓ 3"} {
			if !strings.Contains(got, badge) {
				t.Errorf("expected badge %q in envelope, got %q", badge, got)
			}
		}
	})

	t.Run("CompositeUserAndBot", func(t *testing.T) {
		got := FormatStatus(1, 1, 1, 1, 0, "#313244", "rounded")
		if !strings.HasPrefix(got, "#[fg=#f38ba8,bold]🚨 1#[default] #[fg=#b4befe]👁️ 1#[default] ") {
			t.Errorf("expected user badges on the left, got %q", got)
		}
		if !strings.Contains(got, "🤖") || !strings.Contains(got, "⏳ 1") || !strings.Contains(got, "⚡ 1") {
			t.Errorf("missing bot components in %q", got)
		}
	})

	t.Run("StyleBlock", func(t *testing.T) {
		got := FormatStatus(0, 0, 1, 1, 0, "#313244", "block")
		if strings.Contains(got, "") || strings.Contains(got, "") {
			t.Errorf("block style should not contain nerd font curves, got %q", got)
		}
		if !strings.HasPrefix(got, "#[fg=#cdd6f4,bg=#313244] 🤖 ") {
			t.Errorf("expected block style prefix with space, got %q", got)
		}
		if !strings.HasSuffix(got, " #[default]") {
			t.Errorf("expected block style suffix with space, got %q", got)
		}
	})

	t.Run("StyleBracket", func(t *testing.T) {
		got := FormatStatus(0, 0, 1, 0, 0, "#313244", "bracket")
		if !strings.Contains(got, "[") || !strings.Contains(got, "]") {
			t.Errorf("expected bracket delimiters in %q", got)
		}
	})

	t.Run("StylePlain", func(t *testing.T) {
		got := FormatStatus(0, 0, 1, 0, 0, "", "plain")
		if strings.Contains(got, "bg=") {
			t.Errorf("plain style should have no background, got %q", got)
		}
		if !strings.Contains(got, "🤖 #[fg=#fab387,bold]⏳ 1#[default]") {
			t.Errorf("expected plain bot badge, got %q", got)
		}
	})

	t.Run("CustomBg", func(t *testing.T) {
		got := FormatStatus(0, 0, 1, 0, 0, "#45475a", "rounded")
		if !strings.Contains(got, "#45475a") {
			t.Errorf("expected custom bg color in %q", got)
		}
	})
}

func TestRenderANSI(t *testing.T) {
	t.Parallel()

	t.Run("Empty", func(t *testing.T) {
		if got := RenderANSI(""); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("PlainPassthrough", func(t *testing.T) {
		got := RenderANSI("hello world")
		if !strings.HasPrefix(got, "hello world") {
			t.Errorf("expected plain text passthrough, got %q", got)
		}
	})

	t.Run("ConvertsHexColorsAndStyles", func(t *testing.T) {
		input := "#[fg=#fab387,bold]⏳ 1#[default] #[fg=#a6e3a1,nobold]⚡ 2#[default]"
		got := RenderANSI(input)
		// #fab387 -> 250, 179, 135
		if !strings.Contains(got, "\x1b[38;2;250;179;135m") {
			t.Errorf("missing fg color for peach in %q", got)
		}
		if !strings.Contains(got, "\x1b[1m") {
			t.Errorf("missing bold in %q", got)
		}
		// #a6e3a1 -> 166, 227, 161
		if !strings.Contains(got, "\x1b[38;2;166;227;161m") {
			t.Errorf("missing fg color for green in %q", got)
		}
		if !strings.Contains(got, "\x1b[22m") {
			t.Errorf("missing nobold in %q", got)
		}
		if !strings.HasSuffix(got, "\x1b[0m") {
			t.Errorf("missing trailing reset in %q", got)
		}
	})

	t.Run("ConvertsBackgroundAndDelimiters", func(t *testing.T) {
		input := "#[fg=#313244,bg=default]#[fg=#cdd6f4,bg=#313244]🤖 ⏳ 1#[fg=#313244,bg=default]#[default]"
		got := RenderANSI(input)
		// bg=#313244 -> 49, 50, 68
		if !strings.Contains(got, "\x1b[48;2;49;50;68m") {
			t.Errorf("missing bg color in %q", got)
		}
		if !strings.Contains(got, "\x1b[49m") {
			t.Errorf("missing bg default reset in %q", got)
		}
		if !strings.Contains(got, "") || !strings.Contains(got, "") {
			t.Errorf("missing capsule delimiters in %q", got)
		}
	})
}



func TestHistoryLine(t *testing.T) {
	t.Parallel()
	t.Run("DeadPane", func(t *testing.T) {
		paneInfo := func(id string) tmux.PaneInfo {
			return tmux.PaneInfo{}
		}
		got := historyLine("BADGE ", "dead_id", paneInfo)
		if !strings.Contains(got, "[Dead Pane]") {
			t.Errorf("expected [Dead Pane] for dead pane, got %q", got)
		}
		if !strings.Contains(got, "dead_id") {
			t.Errorf("expected dead_id in output for dead pane, got %q", got)
		}
	})

	t.Run("ValidPane", func(t *testing.T) {
		paneInfo := func(id string) tmux.PaneInfo {
			return tmux.PaneInfo{
				ID:      id,
				Session: "mysess",
				Window:  1,
				Pane:    2,
				Path:    "/path/to/myrepo",
				Command: "nvim",
			}
		}
		got := historyLine("BADGE ", "alive_id", paneInfo)
		if !strings.Contains(got, "BADGE") {
			t.Errorf("expected badge in output, got %q", got)
		}
		if !strings.Contains(got, "[mysess:1.2]") {
			t.Errorf("expected context in output, got %q", got)
		}
		if !strings.Contains(got, "myrepo") {
			t.Errorf("expected repo in output, got %q", got)
		}
		if !strings.Contains(got, "alive_id") {
			t.Errorf("expected pane ID in output, got %q", got)
		}
	})
}

func TestEmptyRow(t *testing.T) {
	t.Parallel()
	got := emptyRow("BADGE ", "Nothing to see here")
	if !strings.Contains(got, "BADGE") {
		t.Errorf("expected badge in output, got %q", got)
	}
	if !strings.Contains(got, "[0:0.0]") {
		t.Errorf("expected context in output, got %q", got)
	}
	if !strings.Contains(got, "none") {
		t.Errorf("expected repo 'none' in output, got %q", got)
	}
	if !strings.Contains(got, "Nothing to see here") {
		t.Errorf("expected message in output, got %q", got)
	}
}

func TestBadgeAndPrio(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		entry     state.Entry
		wantBadge string
		wantPrio  int
	}{
		{"Manual Alert", state.Entry{Kind: state.KindManual, State: state.StateAlert}, "🚨 Alert      ", 1},
		{"Manual Vigil", state.Entry{Kind: state.KindManual, State: state.StateWatching}, "👁️ Vigil      ", 5},
		{"Auto Waiting", state.Entry{Kind: state.KindAuto, State: state.StateWaiting}, "🤖 ⏳ Waiting ", 2},
		{"Auto Done", state.Entry{Kind: state.KindAuto, State: state.StateDone}, "🤖 ✓ Finished ", 3},
		{"Auto Running", state.Entry{Kind: state.KindAuto, State: state.StateRunning}, "🤖 ⚡ Running  ", 4},
		{"Auto Idle", state.Entry{Kind: state.KindAuto, State: state.StateIdle}, "🤖 💤 Idle    ", 6},
		{"Auto Unknown", state.Entry{Kind: state.KindAuto, State: state.StateUnknown}, "🤖 ? Unknown  ", 6},
		{"Auto Custom", state.Entry{Kind: state.KindAuto, State: state.PaneState("custom")}, "🤖 custom    ", 6},
		{"Unknown Kind", state.Entry{Kind: state.Kind("unknown")}, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBadge, gotPrio := badgeAndPrio(tt.entry)
			if gotBadge != tt.wantBadge {
				t.Errorf("badgeAndPrio() gotBadge = %q, want %q", gotBadge, tt.wantBadge)
			}
			if gotPrio != tt.wantPrio {
				t.Errorf("badgeAndPrio() gotPrio = %v, want %v", gotPrio, tt.wantPrio)
			}
		})
	}
}
