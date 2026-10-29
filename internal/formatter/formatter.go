// Package formatter renders the list-raw output for the fzf viewfinder.
// It produces the same tab-delimited format the bash script used:
//
//	<display_col>\t<pane_or_session_id>
//
// where display_col is formatted for fzf's --with-nth=1 rendering.
package formatter

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mhutchinson/tmux-glance/internal/slots"
	"github.com/mhutchinson/tmux-glance/internal/state"
	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

// truncate returns s truncated to maxLen runes, with "..." if over.
// Uses rune count (not byte length) so multi-byte characters (emojis, CJK)
// are measured correctly.
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-3]) + "..."
}

// BotsList renders the Bots & Views view: all active agent alerts,
// blocked waiting states, finished states, running states, manual vigils,
// and idle agent sentinels. Sorted strictly by severity/priority:
// 1. Alert (🚨)
// 2. Waiting (🤖 ⏳)
// 3. Finished (🤖 ✓)
// 4. Running (🤖 ⚡)
// 5. Vigil (👁️)
// 6. Idle (🤖 💤)
func BotsList(entries []state.Entry, panes []tmux.PaneInfo, reg SentinelResolver) string {
	stateByID := make(map[string]state.Entry, len(entries))
	for _, e := range entries {
		stateByID[e.PaneID] = e
	}

	type row struct {
		prio    int
		display string
	}
	var rows []row

	for _, e := range entries {
		badge, prio := badgeAndPrio(e)
		if badge == "" {
			continue
		}
		ctx := truncate(fmt.Sprintf("[%s:%d.%d] %s", e.Session, e.Window, e.Pane, e.Command), 16)
		repo := truncate(filepath.Base(e.Path), 24)
		label := truncate(e.Label, 32)
		display := fmt.Sprintf("%s│ %-16s │ %-24s │ %s\t%s", badge, ctx, repo, label, e.PaneID)
		rows = append(rows, row{prio: prio, display: display})
	}

	if reg != nil {
		for _, p := range panes {
			if _, tracked := stateByID[p.ID]; tracked {
				continue
			}
			sentName := resolvePaneSentinel(reg, p)
			if sentName != "generic" {
				badge := "🤖 💤 Idle    "
				ctx := truncate(fmt.Sprintf("[%s:%d.%d] %s", p.Session, p.Window, p.Pane, p.Command), 16)
				repo := truncate(filepath.Base(p.Path), 24)
				label := truncate("idle in "+filepath.Base(p.Path), 32)
				display := fmt.Sprintf("%s│ %-16s │ %-24s │ %s\t%s", badge, ctx, repo, label, p.ID)
				rows = append(rows, row{prio: 6, display: display})
			}
		}
	}

	if len(rows) == 0 {
		return emptyRow("🤖 Empty      ", "No active agents or vigils found (Ctrl-g for all panes)")
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].prio < rows[j].prio })
	var sb strings.Builder
	for _, r := range rows {
		sb.WriteString(r.display)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// AttentionList renders the legacy Attention Hub (alerts + vigils + agent states).
// Preserved for backward compatibility.
func AttentionList(entries []state.Entry) string {
	return BotsList(entries, nil, nil)
}

// AllPanesList renders the Global (All Panes) view: every single pane across
// all tmux sessions and windows. Badged and tracked panes (alerts, waiting,
// running, vigils, idle agents) sort to the top, followed by standard/generic panes.
func AllPanesList(entries []state.Entry, panes []tmux.PaneInfo, reg SentinelResolver) string {
	if len(panes) == 0 {
		return emptyRow("💻 Empty      ", "No active tmux panes found")
	}

	stateByID := make(map[string]state.Entry, len(entries))
	for _, e := range entries {
		stateByID[e.PaneID] = e
	}

	type row struct {
		prio    int
		display string
	}
	var rows []row

	for _, p := range panes {
		e, tracked := stateByID[p.ID]
		sentName := "generic"
		if reg != nil {
			sentName = resolvePaneSentinel(reg, p)
		}

		var badge, label string
		var prio int
		if tracked {
			badge, prio = badgeAndPrio(e)
			label = e.Label
		}
		if badge == "" {
			if sentName != "generic" {
				badge = "🤖 💤 Idle    "
				prio = 6
				label = "idle in " + filepath.Base(p.Path)
			} else {
				badge = "💻 Pane       "
				prio = 7
				label = p.Path
			}
		}

		ctx := truncate(fmt.Sprintf("[%s:%d.%d] %s", p.Session, p.Window, p.Pane, p.Command), 16)
		repo := truncate(filepath.Base(p.Path), 24)
		displayLabel := truncate(label, 32)
		display := fmt.Sprintf("%s│ %-16s │ %-24s │ %s\t%s", badge, ctx, repo, displayLabel, p.ID)
		rows = append(rows, row{prio: prio, display: display})
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].prio < rows[j].prio })
	var sb strings.Builder
	for _, r := range rows {
		sb.WriteString(r.display)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// SessionsList renders the Sessionizer view with ambient badges per session.
func SessionsList(entries []state.Entry, sessions []tmux.SessionInfo, slotStore *slots.Store) string {
	if len(sessions) == 0 {
		return emptyRow("🪟 Empty      ", "No active tmux sessions found")
	}

	// Aggregate counts per session.
	type counts struct{ alerts, watches, waiting, running, done int }
	bySess := make(map[string]*counts)
	for _, e := range entries {
		c := bySess[e.Session]
		if c == nil {
			c = &counts{}
			bySess[e.Session] = c
		}
		switch e.Kind {
		case state.KindManual:
			if e.State == state.StateAlert {
				c.alerts++
			} else {
				c.watches++
			}
		case state.KindAuto:
			switch e.State {
			case state.StateWaiting:
				c.waiting++
			case state.StateRunning:
				c.running++
			case state.StateDone:
				c.done++
			}
		}
	}

	// Load slot assignments.
	slotMap, _ := slotStore.All()
	// Invert: session → slot.
	sessToSlot := make(map[string]string)
	for sl, sess := range slotMap {
		sessToSlot[sess] = strings.ToUpper(sl)
	}

	type row struct {
		prio    int
		display string
	}
	var rows []row

	for _, sess := range sessions {
		c := bySess[sess.Name]
		if c == nil {
			c = &counts{}
		}
		var badgeParts []string
		prio := 6
		if c.alerts > 0 {
			badgeParts = append(badgeParts, fmt.Sprintf("🚨 %d", c.alerts))
			prio = 1
		}
		if c.waiting > 0 {
			badgeParts = append(badgeParts, fmt.Sprintf("🤖 ⏳ %d", c.waiting))
			if prio > 2 {
				prio = 2
			}
		}
		if c.done > 0 {
			badgeParts = append(badgeParts, fmt.Sprintf("🤖 ✓ %d", c.done))
			if prio > 3 {
				prio = 3
			}
		}
		if c.running > 0 {
			badgeParts = append(badgeParts, fmt.Sprintf("🤖 ⚡ %d", c.running))
			if prio > 4 {
				prio = 4
			}
		}
		if c.watches > 0 {
			badgeParts = append(badgeParts, fmt.Sprintf("👁️ %d", c.watches))
			if prio > 5 {
				prio = 5
			}
		}

		var badges string
		if len(badgeParts) == 0 {
			badges = "🌱 Clean      "
		} else {
			badges = strings.Join(badgeParts, " ")
			// Pad to consistent width (~14 visible chars).
			if len([]rune(badges)) < 14 {
				badges += strings.Repeat(" ", 14-len([]rune(badges)))
			}
		}

		slotBadge := "   "
		if sl, ok := sessToSlot[sess.Name]; ok {
			slotBadge = fmt.Sprintf("[%s]", sl)
		}
		attached := ""
		if sess.Attached {
			attached = "*"
		}
		displaySess := truncate(fmt.Sprintf("%s [%s]%s", slotBadge, sess.Name, attached), 20)
		repo := truncate(filepath.Base(sess.Path), 24)
		noun := "windows"
		if sess.Windows == 1 {
			noun = "window"
		}
		details := truncate(fmt.Sprintf("%d %s (active: %s in %s)", sess.Windows, noun, sess.Command, sess.WinName), 38)

		display := fmt.Sprintf("%s│ %-20s │ %-24s │ %s\t%s", badges, displaySess, repo, details, sess.Name)
		rows = append(rows, row{prio: prio, display: display})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].prio < rows[j].prio })
	var sb strings.Builder
	for _, r := range rows {
		sb.WriteString(r.display)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// HistoryList renders the Jump History timeline: FWD entries (reversed),
// CUR, then BACK entries.
func HistoryList(fwdPanes, backPanes []string, curPane string, paneInfo func(id string) tmux.PaneInfo) string {
	var sb strings.Builder
	numFwd := len(fwdPanes)

	// Print FWD entries in reverse order so FWD #1 is nearest CUR.
	for i := numFwd - 1; i >= 0; i-- {
		badge := fmt.Sprintf("⏭️  FWD #%-2d   ", i+1)
		line := historyLine(badge, fwdPanes[i], paneInfo)
		if line != "" {
			sb.WriteString(line)
		}
	}

	// CUR.
	if curPane != "" {
		line := historyLine("📍 CUR       ", curPane, paneInfo)
		if line != "" {
			sb.WriteString(line)
		}
	}

	// BACK entries.
	for i, id := range backPanes {
		badge := fmt.Sprintf("⏮️  BACK #%-2d  ", i+1)
		line := historyLine(badge, id, paneInfo)
		if line != "" {
			sb.WriteString(line)
		}
	}

	if sb.Len() == 0 {
		return emptyRow("🕒 Empty      ", "No jump history recorded yet")
	}
	return sb.String()
}

func historyLine(badge, paneID string, paneInfo func(string) tmux.PaneInfo) string {
	p := paneInfo(paneID)
	if p.ID == "" {
		return ""
	}
	ctx := truncate(fmt.Sprintf("[%s:%d.%d] %s", p.Session, p.Window, p.Pane, p.Command), 16)
	repo := truncate(filepath.Base(p.Path), 24)
	label := truncate(p.Path, 32)
	return fmt.Sprintf("%s│ %-16s │ %-24s │ %s\t%s\n", badge, ctx, repo, label, p.ID)
}

func emptyRow(badge, msg string) string {
	return fmt.Sprintf("%s│ %-16s │ %-24s │ %s\t%s\n", badge, "[0:0.0]", "none", msg, "")
}

func badgeAndPrio(e state.Entry) (badge string, prio int) {
	switch e.Kind {
	case state.KindManual:
		if e.State == state.StateAlert {
			return "🚨 Alert      ", 1
		}
		return "👁️ Vigil      ", 5
	case state.KindAuto:
		switch e.State {
		case state.StateWaiting:
			return "🤖 ⏳ Waiting ", 2
		case state.StateDone:
			return "🤖 ✓ Finished ", 3
		case state.StateRunning:
			return "🤖 ⚡ Running  ", 4
		case state.StateIdle:
			return "🤖 💤 Idle    ", 6
		case state.StateUnknown:
			return "🤖 ? Unknown  ", 6
		default:
			return fmt.Sprintf("🤖 %-9s ", truncate(string(e.State), 9)), 6
		}
	}
	return "", 0
}

// SentinelResolver is the subset of sentinel.Registry consumed by the formatter.
type SentinelResolver interface {
	Resolve(cmd string) string
}

// PaneSentinelResolver is an optional extension of SentinelResolver that resolves
// using full PaneInfo (including PID for process tree disambiguation).
type PaneSentinelResolver interface {
	SentinelResolver
	ResolvePane(p tmux.PaneInfo) string
}

func resolvePaneSentinel(reg SentinelResolver, p tmux.PaneInfo) string {
	if pr, ok := reg.(PaneSentinelResolver); ok {
		return pr.ResolvePane(p)
	}
	return reg.Resolve(p.Command)
}

// FormatStatus formats the ambient status-right badges.
// Manual vigils/alerts are placed on the left in their standard style.
// Background agent states are enclosed in an envelope with a subtle elevated
// background and deduplicated '🤖' container glyph.
//
// Supported styles:
//   - "rounded" (default): Nerd Font half-circle capsule ( ... )
//   - "block": Standard space-padded background block
//   - "bracket": Delimited with brackets
//   - "plain": Unboxed text without background
func FormatStatus(alerts, watches, waiting, running, done int, bg, style string) string {
	var userParts []string
	if alerts > 0 {
		userParts = append(userParts, fmt.Sprintf("#[fg=#f38ba8,bold]🚨 %d#[default]", alerts))
	}
	if watches > 0 {
		userParts = append(userParts, fmt.Sprintf("#[fg=#b4befe]👁️ %d#[default]", watches))
	}

	var botParts []string
	if waiting > 0 {
		botParts = append(botParts, fmt.Sprintf("#[fg=#fab387,bold]⏳ %d", waiting))
	}
	if running > 0 {
		botParts = append(botParts, fmt.Sprintf("#[fg=#a6e3a1,nobold]⚡ %d", running))
	}
	if done > 0 {
		botParts = append(botParts, fmt.Sprintf("#[fg=#89b4fa,bold]✓ %d", done))
	}

	var botPill string
	if len(botParts) > 0 {
		if bg == "" {
			bg = "#313244"
		}
		if style == "" {
			style = "rounded"
		}
		statesStr := strings.Join(botParts, " ")

		switch style {
		case "block":
			botPill = fmt.Sprintf("#[fg=#cdd6f4,bg=%s] 🤖 %s #[default]", bg, statesStr)
		case "bracket":
			botPill = fmt.Sprintf("#[fg=#585b70][#[default]#[fg=#cdd6f4,bg=%s] 🤖 %s #[default]#[fg=#585b70]]#[default]", bg, statesStr)
		case "plain":
			botPill = fmt.Sprintf("#[fg=#cdd6f4]🤖 %s#[default]", statesStr)
		default: // "rounded"
			botPill = fmt.Sprintf("#[fg=%s,bg=default]#[fg=#cdd6f4,bg=%s]🤖 %s#[fg=%s,bg=default,nobold]#[default]", bg, bg, statesStr, bg)
		}
	}

	if len(userParts) > 0 && botPill != "" {
		return strings.Join(userParts, " ") + " " + botPill
	}
	if len(userParts) > 0 {
		return strings.Join(userParts, " ")
	}
	return botPill
}

// RenderANSI converts tmux format directives (e.g. #[fg=#RRGGBB,bg=...,bold])
// into standard 24-bit ANSI color escape sequences suitable for direct display
// in terminal emulators.
func RenderANSI(s string) string {
	if s == "" {
		return ""
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], "#[") {
			end := strings.IndexByte(s[i:], ']')
			if end != -1 {
				tagContent := s[i+2 : i+end]
				sb.WriteString(parseTmuxTagToANSI(tagContent))
				i += end + 1
				continue
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	// Always reset terminal styling at the end
	sb.WriteString("\x1b[0m")
	return sb.String()
}

func parseTmuxTagToANSI(tag string) string {
	var sb strings.Builder
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		switch {
		case part == "default":
			sb.WriteString("\x1b[0m")
		case part == "bold":
			sb.WriteString("\x1b[1m")
		case part == "nobold":
			sb.WriteString("\x1b[22m")
		case part == "dim":
			sb.WriteString("\x1b[2m")
		case part == "nodim":
			sb.WriteString("\x1b[22m")
		case part == "fg=default":
			sb.WriteString("\x1b[39m")
		case part == "bg=default":
			sb.WriteString("\x1b[49m")
		case strings.HasPrefix(part, "fg=#") && len(part) == 10:
			if r, g, b, ok := parseHexRGB(part[4:]); ok {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", r, g, b)
			}
		case strings.HasPrefix(part, "bg=#") && len(part) == 10:
			if r, g, b, ok := parseHexRGB(part[4:]); ok {
				fmt.Fprintf(&sb, "\x1b[48;2;%d;%d;%dm", r, g, b)
			}
		case strings.HasPrefix(part, "fg=colour"):
			if n, err := strconv.Atoi(part[9:]); err == nil {
				fmt.Fprintf(&sb, "\x1b[38;5;%dm", n)
			}
		case strings.HasPrefix(part, "bg=colour"):
			if n, err := strconv.Atoi(part[9:]); err == nil {
				fmt.Fprintf(&sb, "\x1b[48;5;%dm", n)
			}
		}
	}
	return sb.String()
}

func parseHexRGB(hex string) (uint8, uint8, uint8, bool) {
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8((v >> 16) & 0xff), uint8((v >> 8) & 0xff), uint8(v & 0xff), true
}


