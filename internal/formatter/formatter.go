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
	"strings"

	"github.com/mhutchinson/tmux-glance/internal/slots"
	"github.com/mhutchinson/tmux-glance/internal/state"
	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

// truncate returns s truncated to maxLen, with "..." if over.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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
			sentName := reg.Resolve(p.Command)
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
			sentName = reg.Resolve(p.Command)
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
			return "🤖 ⚡ Running ", 4
		default:
			return fmt.Sprintf("🤖 %s      ", e.State), 6
		}
	}
	return "", 0
}

// SentinelResolver is the subset of sentinel.Registry consumed by the formatter.
type SentinelResolver interface {
	Resolve(cmd string) string
}

// resolverAdapter adapts sentinel.Registry (which takes a context) for the formatter.
type resolverAdapter struct {
	resolve func(cmd string) string
}

func (r resolverAdapter) Resolve(cmd string) string { return r.resolve(cmd) }
