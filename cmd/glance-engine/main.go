// Command glance-engine is the Go engine backing tmux-glance.
// It replaces the stateful logic of bin/tmux-glance (state management,
// scanning, jumplist, slots, formatting) while keeping the fzf popup
// assembly and preview loop in the thin bash wrapper.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mhutchinson/tmux-glance/internal/formatter"
	"github.com/mhutchinson/tmux-glance/internal/jumplist"
	"github.com/mhutchinson/tmux-glance/internal/quickfix"
	"github.com/mhutchinson/tmux-glance/internal/scanner"
	"github.com/mhutchinson/tmux-glance/internal/sentinel"
	"github.com/mhutchinson/tmux-glance/internal/slots"
	"github.com/mhutchinson/tmux-glance/internal/state"
	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "glance-engine: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: glance-engine <command> [args...]")
	}

	// Resolve state paths (mirrors the bash variable logic).
	stateFile := envOr("TMUX_GLANCE_STATE_FILE", filepath.Join(home(), ".tmux-glance"))
	jumpDir := envOr("TMUX_GLANCE_JUMP_DIR",
		filepath.Join(xdgState(), "tmux-glance"))
	backFile := envOr("TMUX_GLANCE_JUMP_BACK_FILE", filepath.Join(jumpDir, "jump_back"))
	fwdFile := envOr("TMUX_GLANCE_JUMP_FORWARD_FILE", filepath.Join(jumpDir, "jump_forward"))
	slotsFile := envOr("TMUX_GLANCE_SLOTS_FILE",
		filepath.Join(xdgState(), "tmux-glance", "slots"))

	// Initialise subsystems.
	store, err := state.NewFileStore(stateFile)
	if err != nil {
		return fmt.Errorf("state store: %w", err)
	}
	lock := state.NewLock(stateFile)

	slotStore, err := slots.NewStore(slotsFile)
	if err != nil {
		return fmt.Errorf("slot store: %w", err)
	}

	stack, err := jumplist.New(backFile, fwdFile, nil)
	if err != nil {
		return fmt.Errorf("jumplist: %w", err)
	}

	tmuxClient := tmux.Default

	// Discover sentinels.
	selfDir := selfDir()
	sentinelDirs := []string{
		os.Getenv("TMUX_GLANCE_SENTINEL_DIR"),
		filepath.Join(xdgConfig(), "tmux-glance", "sentinels"),
		filepath.Join(selfDir, "..", "sentinels"),
		filepath.Join(selfDir, "..", "share", "tmux-glance", "sentinels"),
	}
	disabledStr := envOrTmux(ctx, tmuxClient, "TMUX_GLANCE_DISABLED_SENTINELS", "@glance_disabled_sentinels", "")
	var disabled []string
	for _, d := range strings.Split(disabledStr, ",") {
		if d = strings.TrimSpace(d); d != "" {
			disabled = append(disabled, d)
		}
	}
	routesStr := envOrTmux(ctx, tmuxClient, "TMUX_GLANCE_ROUTES", "@glance_routes", "")
	reg, err := sentinel.New(sentinelDirs, disabled, sentinel.ParseRouteOverrides(routesStr))
	if err != nil {
		return fmt.Errorf("sentinel registry: %w", err)
	}

	cooldownSecs, _ := strconv.Atoi(envOrTmux(ctx, tmuxClient, "TMUX_GLANCE_SCAN_COOLDOWN", "@glance_scan_cooldown", "3"))
	sc := scanner.New(store, reg, tmuxClient, time.Duration(cooldownSecs)*time.Second)

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "status":
		return lock.WithLock(ctx, func() error {
			out, err := sc.StatusSummary(ctx)
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		})

	case "get-sentinel":
		// Prints the resolved sentinel name for a given command string and optional PID.
		// Used by tests instead of the old bash get_sentinel function.
		sentinelCmd := ""
		if len(rest) > 0 {
			sentinelCmd = rest[0]
		}
		var pid int
		if len(rest) > 1 {
			pid, _ = strconv.Atoi(rest[1])
		}
		fmt.Println(reg.ResolvePane(ctx, tmux.PaneInfo{Command: sentinelCmd, PID: pid}))
		return nil

	case "scan":
		force := len(rest) > 0 && (rest[0] == "force" || rest[0] == "--force")
		return lock.WithLock(ctx, func() error {
			return sc.Scan(ctx, force)
		})

	case "on-focus":
		paneID := ""
		if len(rest) > 0 {
			paneID = rest[0]
		}
		return lock.WithLock(ctx, func() error {
			return sc.OnFocus(ctx, paneID)
		})

	case "add", "toggle-vigil", "remove":
		return lock.WithLock(ctx, func() error {
			paneID, _ := tmuxClient.GetPaneField(ctx, "", "#{pane_id}")
			switch cmd {
			case "add":
				return addVigil(ctx, store, tmuxClient, reg, paneID)
			case "remove":
				return removeVigil(ctx, store, tmuxClient, paneID)
			case "toggle-vigil":
				_, found, _ := store.GetEntry(paneID)
				if found {
					return removeVigil(ctx, store, tmuxClient, paneID)
				}
				return addVigil(ctx, store, tmuxClient, reg, paneID)
			}
			return nil
		})

	case "is-watched", "is-pinned":
		paneID, _ := tmuxClient.GetPaneField(ctx, "", "#{pane_id}")
		_, found, err := store.GetEntry(paneID)
		if err != nil {
			return err
		}
		if !found {
			os.Exit(1)
		}
		return nil

	case "list-raw":
		mode := "toggle"
		if len(rest) > 0 {
			mode = rest[0]
		}
		return listRaw(ctx, store, reg, slotStore, tmuxClient, stack, mode)

	case "record-jump":
		if len(rest) < 2 {
			return fmt.Errorf("record-jump requires source and dest pane IDs")
		}
		return lock.WithLock(ctx, func() error {
			return stack.Record(ctx, rest[0], rest[1])
		})

	case "shift-to":
		if len(rest) < 2 {
			return fmt.Errorf("shift-to requires current and target pane IDs [direction]")
		}
		dir := ""
		if len(rest) > 2 {
			dir = rest[2]
		}
		return lock.WithLock(ctx, func() error {
			return stack.ShiftTo(ctx, rest[0], rest[1], dir)
		})

	case "jump-back", "undo":
		paneID := ""
		if len(rest) > 0 {
			paneID = rest[0]
		}
		return lock.WithLock(ctx, func() error {
			if paneID == "" {
				paneID, _ = tmuxClient.GetPaneField(ctx, "", "#{pane_id}")
			}
			target, err := stack.Back(ctx, paneID)
			if err != nil {
				return err
			}
			if target == "" {
				tmuxClient.DisplayMessage(ctx, "Jump list: at oldest location") //nolint:errcheck
				return nil
			}
			return selectAndFocus(ctx, target, sc, tmuxClient)
		})

	case "jump-forward", "redo":
		paneID := ""
		if len(rest) > 0 {
			paneID = rest[0]
		}
		return lock.WithLock(ctx, func() error {
			if paneID == "" {
				paneID, _ = tmuxClient.GetPaneField(ctx, "", "#{pane_id}")
			}
			target, err := stack.Forward(ctx, paneID)
			if err != nil {
				return err
			}
			if target == "" {
				tmuxClient.DisplayMessage(ctx, "Jump list: at newest location") //nolint:errcheck
				return nil
			}
			return selectAndFocus(ctx, target, sc, tmuxClient)
		})

	case "next-attention", "cnext":
		paneID := ""
		if len(rest) > 0 {
			paneID = rest[0]
		}
		return lock.WithLock(ctx, func() error {
			return quickfix.Cycle(ctx, "next", store, stack, tmuxClient, paneID)
		})

	case "prev-attention", "cprev":
		paneID := ""
		if len(rest) > 0 {
			paneID = rest[0]
		}
		return lock.WithLock(ctx, func() error {
			return quickfix.Cycle(ctx, "prev", store, stack, tmuxClient, paneID)
		})

	case "jump-slot":
		if len(rest) < 1 {
			return fmt.Errorf("jump-slot requires a slot identifier")
		}
		slotKey := rest[0]
		sourcePaneID := ""
		if len(rest) > 1 {
			sourcePaneID = rest[1]
		}
		return lock.WithLock(ctx, func() error {
			return jumpSlot(ctx, slotStore, stack, tmuxClient, slotKey, sourcePaneID)
		})

	case "assign-slot":
		if len(rest) < 1 {
			return fmt.Errorf("assign-slot requires slot [session]")
		}
		slotKey := rest[0]
		session := ""
		if len(rest) > 1 {
			session = rest[1]
		}
		if session == "" {
			session, _ = tmuxClient.GetSessionField(ctx, "", "#{session_name}")
		}
		if err := slotStore.Assign(slotKey, session); err != nil {
			return err
		}
		tmuxClient.DisplayMessage(ctx, fmt.Sprintf("📌 Pinned [%s] to slot [%s]", session, strings.ToUpper(slotKey))) //nolint:errcheck
		return nil

	case "unassign-slot":
		target := ""
		if len(rest) > 0 {
			target = rest[0]
		}
		if target == "" {
			target, _ = tmuxClient.GetSessionField(ctx, "", "#{session_name}")
		}
		if err := slotStore.Unassign(target); err != nil {
			return err
		}
		tmuxClient.DisplayMessage(ctx, fmt.Sprintf("Unpinned [%s]", target)) //nolint:errcheck
		return nil

	case "query-slot":
		// Returns the uppercase slot letter for the given session, or "" if unpinned.
		// Used by pin_interactive in bin/tmux-glance to show the current slot in the prompt.
		target := ""
		if len(rest) > 0 {
			target = rest[0]
		}
		if target == "" {
			target, _ = tmuxClient.GetSessionField(ctx, "", "#{session_name}")
		}
		slot, err := slotStore.LookupBySession(target)
		if err != nil {
			return err
		}
		if slot != "" {
			fmt.Println(strings.ToUpper(slot))
		}
		return nil

	case "clear-history", "clear-jump-history":
		return lock.WithLock(ctx, func() error {
			if err := stack.Clear(); err != nil {
				return err
			}
			tmuxClient.DisplayMessage(ctx, "Jump history cleared") //nolint:errcheck
			return nil
		})

	case "history-cursor-pos":
		curPane, _ := tmuxClient.GetPaneField(ctx, "", "#{pane_id}")
		pos := stack.CursorPos(ctx, curPane)
		fmt.Println(pos)
		return nil

	case "mode-header":
		target := "hub"
		if len(rest) > 0 {
			target = rest[0]
		}
		if target == "current" {
			target, _ = tmuxClient.GetGlobalOption(ctx, "@glance_mode")
		}
		fmt.Println(modeHeader(target))
		return nil

	case "eval-mode-action", "eval-history-action":
		targetReq := "history"
		if len(rest) > 0 {
			targetReq = rest[0]
		}
		curMode, _ := tmuxClient.GetGlobalOption(ctx, "@glance_mode")
		if curMode == "" {
			curMode = "hub"
		}
		targetMode := resolveMode(curMode, targetReq)
		tmuxClient.SetGlobalOption(ctx, "@glance_mode", targetMode) //nolint:errcheck

		selfBin := os.Getenv("TMUX_GLANCE_BIN")
		if selfBin == "" {
			selfBin = "tmux-glance"
		}
		header := modeHeader(targetMode)
		pos := 1
		if targetMode == "history" {
			curPane, _ := tmuxClient.GetPaneField(ctx, "", "#{pane_id}")
			pos = stack.CursorPos(ctx, curPane)
		}
		fmt.Printf("reload(%s list-raw %s)+change-header(%s)+wait+pos(%d)", selfBin, targetMode, header, pos)
		return nil

	case "help", "--help", "-h":
		fmt.Println("usage: glance-engine {status|scan|on-focus|add|remove|toggle-vigil|is-watched|list-raw|mode-header|record-jump|shift-to|jump-back|jump-forward|next-attention|prev-attention|jump-slot|assign-slot|unassign-slot|query-slot|clear-history|history-cursor-pos|eval-mode-action}")
		return nil

	case "version", "--version", "-v":
		fmt.Println("glance-engine v1.0.0")
		return nil

	default:
		return fmt.Errorf("unknown command: %s\nusage: glance-engine {status|scan|on-focus|add|remove|toggle-vigil|is-watched|list-raw|mode-header|record-jump|shift-to|jump-back|jump-forward|next-attention|prev-attention|jump-slot|assign-slot|unassign-slot|query-slot|clear-history|history-cursor-pos|eval-mode-action}", cmd)
	}
}

// addVigil adds a manual vigil on the given pane.
func addVigil(ctx context.Context, store *state.FileStore, tc *tmux.Client, reg *sentinel.Registry, paneID string) error {
	info, err := tc.GetPaneField(ctx, paneID, "#{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_current_path}|#{pane_current_command}")
	if err != nil {
		return err
	}
	parts := strings.SplitN(info, "|", 6)
	if len(parts) < 6 {
		return fmt.Errorf("unexpected pane info: %q", info)
	}
	win, _ := strconv.Atoi(parts[2])
	pane, _ := strconv.Atoi(parts[3])
	path := parts[4]
	command := parts[5]
	label := command + " in " + filepath.Base(path)
	hash, _ := reg.Fingerprint(ctx, "generic", paneID)
	tc.SetPaneOption(ctx, paneID, "@glance_snapshot", hash) //nolint:errcheck

	e := state.Entry{
		PaneID:  paneID,
		Session: parts[1],
		Window:  win,
		Pane:    pane,
		Path:    path,
		Command: command,
		Label:   label,
		Kind:    state.KindManual,
		State:   state.StateWatching,
	}
	if err := store.UpsertEntry(e); err != nil {
		return err
	}
	tc.SetGlobalOption(ctx, "@glance_last_scan_time", "0") //nolint:errcheck
	tc.RefreshClients(ctx)                                  //nolint:errcheck
	tc.DisplayMessage(ctx, "👁️ Vigil active: "+label)       //nolint:errcheck
	return nil
}

// removeVigil removes the vigil on the given pane.
func removeVigil(ctx context.Context, store *state.FileStore, tc *tmux.Client, paneID string) error {
	e, found, err := store.GetEntry(paneID)
	if err != nil {
		return err
	}
	if !found {
		tc.DisplayMessage(ctx, "No vigil active on this pane.") //nolint:errcheck
		return nil
	}
	if err := store.RemoveEntry(paneID); err != nil {
		return err
	}
	tc.UnsetPaneOption(ctx, paneID, "@glance_snapshot")         //nolint:errcheck
	tc.SetGlobalOption(ctx, "@glance_last_scan_time", "0")      //nolint:errcheck
	tc.RefreshClients(ctx)                                       //nolint:errcheck
	tc.DisplayMessage(ctx, "👁️ Vigil released: "+e.Label)        //nolint:errcheck
	return nil
}

// listRaw renders the appropriate mode and prints to stdout.
func listRaw(ctx context.Context, store *state.FileStore, reg *sentinel.Registry, slotStore *slots.Store, tc *tmux.Client, stack *jumplist.Stack, mode string) error {
	curMode, _ := tc.GetGlobalOption(ctx, "@glance_mode")
	if curMode == "" {
		curMode = "hub"
	}

	targetMode := resolveMode(curMode, mode)
	tc.SetGlobalOption(ctx, "@glance_mode", targetMode) //nolint:errcheck

	entries, _ := store.ReadAll()

	switch targetMode {
	case "all", "global":
		panes, _ := tc.ListAllPanes(ctx)
		resolver := funcResolver{fn: func(cmd string) string { return reg.Resolve(ctx, cmd) }}
		fmt.Print(formatter.AllPanesList(entries, panes, resolver))

	case "sessions":
		sessions, _ := tc.ListSessions(ctx)
		fmt.Print(formatter.SessionsList(entries, sessions, slotStore))

	case "history":
		curPane, _ := tc.GetPaneField(ctx, "", "#{pane_id}")
		fwd, cur, back := stack.HistoryList(ctx, curPane)
		paneInfoFn := func(id string) tmux.PaneInfo {
			info, _ := tc.GetPaneField(ctx, id, "#{pane_id}|#{session_name}|#{window_index}|#{pane_index}|#{pane_current_path}|#{pane_current_command}")
			parts := strings.SplitN(info, "|", 6)
			if len(parts) < 6 {
				return tmux.PaneInfo{}
			}
			win, _ := strconv.Atoi(parts[2])
			pane, _ := strconv.Atoi(parts[3])
			return tmux.PaneInfo{ID: parts[0], Session: parts[1], Window: win, Pane: pane, Path: parts[4], Command: parts[5]}
		}
		fmt.Print(formatter.HistoryList(fwd, back, cur, paneInfoFn))

	default: // "hub", "bots", "attention"
		panes, _ := tc.ListAllPanes(ctx)
		resolver := funcResolver{fn: func(cmd string) string { return reg.Resolve(ctx, cmd) }}
		fmt.Print(formatter.BotsList(entries, panes, resolver))
	}
	return nil
}

// funcResolver adapts a plain function to the formatter.SentinelResolver interface.
type funcResolver struct{ fn func(string) string }

func (f funcResolver) Resolve(cmd string) string { return f.fn(cmd) }

// modeHeader returns the dynamic header string for the given viewfinder mode.
func modeHeader(mode string) string {
	switch mode {
	case "all", "global":
		return "👁️ Global (Ctrl-g: Hub | Ctrl-s: Sessions | Ctrl-h: History | Ctrl-/: Help)"
	case "sessions":
		return "👁️ Sessions (Ctrl-p: Pin | Ctrl-g: Global | Ctrl-h: History | Ctrl-/: Help)"
	case "history":
		return "👁️ History (Ctrl-x: Clear | Ctrl-g: Global | Ctrl-s: Sessions | Ctrl-/: Help)"
	default: // "hub", "bots", "attention"
		return "👁️ Attention Hub (Ctrl-g: Global | Ctrl-s: Sessions | Ctrl-h: History | Ctrl-/: Help)"
	}
}

// resolveMode computes the target mode from the request and current mode.
func resolveMode(curMode, req string) string {
	switch req {
	case "current":
		return curMode
	case "toggle", "toggle-global", "global-toggle":
		if curMode == "all" {
			return "hub"
		}
		return "all"
	case "hub", "bots", "attention":
		return "hub"
	case "all", "global":
		return "all"
	case "sessions", "toggle-sessions", "sessions-toggle":
		return "sessions"
	case "history", "toggle-history", "history-toggle":
		return "history"
	default:
		return req
	}
}

// jumpSlot switches to the session assigned to the given slot.
func jumpSlot(ctx context.Context, slotStore *slots.Store, stack *jumplist.Stack, tc *tmux.Client, slotKey, sourcePaneID string) error {
	session, err := slotStore.LookupBySlot(slotKey)
	if err != nil {
		return err
	}
	upper := strings.ToUpper(slotKey)
	if session == "" {
		tc.DisplayMessage(ctx, fmt.Sprintf("Slot [%s] is unassigned (open Glance → Ctrl-s → Ctrl-p to pin)", upper)) //nolint:errcheck
		return nil
	}
	ok, _ := tc.HasSession(ctx, session)
	if !ok {
		tc.DisplayMessage(ctx, fmt.Sprintf("Slot [%s] session [%s] is not running", upper, session)) //nolint:errcheck
		return nil
	}
	if sourcePaneID == "" {
		sourcePaneID, _ = tc.GetPaneField(ctx, "", "#{pane_id}")
	}
	targPane, _ := tc.SessionActivePaneID(ctx, session)
	if sourcePaneID != "" && targPane != "" {
		stack.Record(ctx, sourcePaneID, targPane) //nolint:errcheck
	}
	return tc.SwitchClient(ctx, session)
}

// selectAndFocus selects a pane and triggers on-focus logic.
func selectAndFocus(ctx context.Context, paneID string, sc *scanner.Scanner, tc *tmux.Client) error {
	tc.SelectPane(ctx, paneID) //nolint:errcheck
	if err := sc.OnFocus(ctx, paneID); err != nil {
		return err
	}
	loc, _ := tc.GetPaneField(ctx, paneID, "#{session_name}:#{window_index}.#{pane_index}")
	tc.DisplayMessage(ctx, fmt.Sprintf("Jumped to [%s]", loc)) //nolint:errcheck
	return nil
}

// --- path helpers ---

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

func xdgState() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return d
	}
	return filepath.Join(home(), ".local", "state")
}

func xdgConfig() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(home(), ".config")
}

func selfDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return filepath.Dir(exe)
	}
	return filepath.Dir(resolved)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrTmux(ctx context.Context, tc *tmux.Client, envKey, tmuxKey, def string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	v, _ := tc.GetGlobalOption(ctx, tmuxKey)
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return def
}
