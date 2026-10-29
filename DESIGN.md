# System Architecture: `tmux-glance`

**Ambient Terminal Sentinels & Background Agent Orchestration for Tmux**  
*Status: Approved & Implemented (v1.0)*  
*Target: Standalone Open-Source Project (`github:mhutchinson/tmux-glance`)*

---

## 1. Executive Summary

As terminal workflows transition from synchronous shell commands to autonomous, long-running processes—such as background AI coding agents (Antigravity, Claude Code, Aider), compilation pipelines (`cargo build`, `nix build`), remote database syncs, and test suites—developers suffer from chronic **speculative window hopping**: cycling through tmux windows and panes simply to inspect whether a process has completed, failed, or blocked waiting for user input.

`tmux-glance` eliminates speculative hopping by establishing an **ambient status-right telemetry protocol**, an on-demand **Attention Hub** viewfinder (`prefix b`), a server-wide **Global Panes Go-To teleporter** (`prefix g`), and instant single-pane **Vigil watches** (`prefix v`).

---

## 2. Core Philosophy & Architectural Invariants

1. **Glanceable by Default (User-First Domain Clustering & Agent Enclosure):** Ambient status-right cues convey server-wide state at the edge of the developer's vision. To put the developer first, manual user vigils (`🚨 1`, `👁️ 2`) are clustered on the left. Autonomous background agents are enveloped in an elevated capsule with a deduplicated `🤖` domain icon (`🤖 ⏳ 1  ⚡ 2  ✓ 1`), preventing repetitive bot icons. If the status bar is quiet, zero action is required.
2. **Minimally Invasive by Default:** Designed to drop into mature, existing tmux setups with zero namespace collision.
   * Only registers non-disruptive core bindings by default (`prefix g`, `prefix b`, `prefix v`).
   * Never hijacks standard tmux defaults (especially `prefix [` copy-mode, or `c`, `z`, `n`, `p`).
   * Direct navigation chords, jumplist rewinds, and quickfix cycling are strictly Tier 2 opt-ins (`@glance_enable_*`).
   * Interactive search keys (`Ctrl-g`, `Ctrl-s`, `Ctrl-h`, `Enter`) stay strictly scoped inside the `fzf` popup subprocess.
3. **Sub-50ms Status Bar Execution Budget:** `tmux-glance status` runs synchronously inside `status-right` on tmux's `status-interval`. It must execute in `< 50ms`. Non-candidate panes (standard shells/compilations without active vigils) are bypassed immediately in O(1) time without capture-pane or hashing subprocesses. Candidate agent panes and vigils are evaluated concurrently across a bounded goroutine worker pool (`workerPoolSize = 8`), and background scans are debounced via a configurable cooldown threshold (`@glance_scan_cooldown`). All external telemetry (such as API quotas or saturation gauges) must read pre-warmed local cache files written asynchronously out-of-band.
4. **Pluggable Sentinels:** Agent TUIs and process monitors are decoupled behind a clean, pluggable Sentinel interface (`Matches`, `Classify`, `Fingerprint`), enabling modular support for diverse agents without core multiplexer entanglement.
5. **Intelligent Thought & Spinner Normalization:** Sentinels strip transient visual noise—such as animated braille spinners (`[⣟⣯⣷]`) and streaming thought indicators—before computing buffer fingerprints, eliminating false-positive unread alarms.
6. **Zero-Friction Auto-Acknowledgment with Dwell-Time Protection:** Merely switching focus into an alerted pane acknowledges and clears the alert after dwell time expires (configurable via `@glance_dwell_ms`, default 500ms). Dwell waiting runs outside the global state lock so rapid navigation whizzing past panes suppresses auto-acknowledgment without blocking concurrent navigation or status commands.
7. **Rock-Solid Concurrency & Atomicity:** Atomic directory locking (`.lock` via `os.Mkdir`) and temporary file renames (`os.CreateTemp` + `os.Rename`) protect state across concurrent tmux status redraws, window switches, and hook triggers.
8. **Cross-Platform POSIX Portability:** Scripts and tests run seamlessly on Darwin (macOS BSD coreutils, `md5`) and Linux (GNU coreutils, `md5sum`).


---

## 3. High-Level Architecture & Component Flow

As of v0.7, all stateful engine logic lives in **`glance-engine`**, a compiled Go binary. The bash script (`bin/tmux-glance`) is a thin dispatcher handling FZF popup assembly, preview loops, and key-read interactions.

### Go/Bash Boundary

```
┌─────────────────────────────────────────────────────────────────┐
│  bin/tmux-glance  (bash dispatcher)                             │
│  • fzf popup launcher (list / list-all / list-sessions / history)│
│  • preview_pane loop (background tail via tmux capture-pane)     │
│  • pin_interactive (raw key read for Harpoon slot assignment)    │
│  • cheat_sheet (static printf)                                   │
│  Everything else: exec glance-engine "$@"                        │
└──────────────────────────┬──────────────────────────────────────┘
                           │ exec / subprocess
┌──────────────────────────▼──────────────────────────────────────┐
│  glance-engine  (Go binary)                                     │
│                                                                 │
│  cmd/glance-engine/main.go   ← thin arg dispatch               │
│  internal/                                                      │
│    tmux/       typed pane/session queries; process tree inspect │
│    state/      FileStore (atomic writes), bash-compat Lock      │
│    sentinel/   native Go sentinels (antigravity, generic)       │
│    scanner/    goroutine fan-out (pool=8), Scan/OnFocus/Status  │
│    jumplist/   back/forward MRU stacks; ShiftTo() frame shift   │
│    quickfix/   AttentionQueue severity sorting & cyclic jump    │
│    slots/      Harpoon slot store                               │
│    formatter/  FZF line renderers (attention/all/sessions/hist) │
└─────────────────────────────────────────────────────────────────┘
```

### Data Flow Diagram

```mermaid
flowchart TD
    subgraph Tmux Multiplexer Server
        P1["Pane %1 (agy: thinking)"]
        P2["Pane %2 (cargo test)"]
        P3["Pane %3 (agy: waiting confirmation)"]
        P4["Pane %4 (zsh prompt)"]
    end

    subgraph glance-engine ["glance-engine (Go binary)"]
        Scan["scanner.Scan()"]
        Router["sentinel.Registry.ResolvePane()"]
        S_AGY["sentinel.Antigravity\n(pure Go in-memory)"]
        S_GEN["sentinel.Generic\n(pure Go in-memory)"]
        Lock["state.Lock\n(mkdir — bash compat)"]
        State["state.FileStore\n(~/.tmux-glance)"]
        JL["jumplist.Stack"]
        QF["quickfix.Cycle()"]
        Slots["slots.Store"]
        Fmt["formatter.*List()"]
    end

    subgraph Presentation
        Status["Tmux status-right\n'🚨 1  👁️ 1  🤖 ⏳ 1  ⚡ 2'"]
        Hub["Attention Hub (prefix b)\nAgent tasks & vigils sorted by severity"]
        Fleet["Global Panes (prefix g)\n100% of panes teleporter"]
        Sessions["Sessionizer (Ctrl-s in modal)\nWorkspace switcher + slot badges"]
        Preview["fzf ANSI Live Preview\n(tmux capture-pane)"]
    end

    P1 & P2 & P3 & P4 --> Scan
    Scan --> Router
    Router --> S_AGY
    Router --> S_GEN
    S_AGY & S_GEN --> Lock
    Lock --> State
    State --> Fmt
    State --> QF
    Fmt --> Status
    Fmt --> Hub
    Fmt --> Fleet
    Fmt --> Sessions
    JL --> Fmt
    Slots --> Fmt
    Hub & Fleet & Sessions --> Preview
```


## 4. The Sentinel Architecture

Pane monitoring logic is implemented via modular `sentinel.Sentinel` implementations in `internal/sentinel/`. First-class sentinels (`antigravity`, `generic`) run in pure Go in-memory with zero subprocess forks.

### Sentinel Contract

Every Sentinel implements the `sentinel.Sentinel` interface:

```go
type Sentinel interface {
	Name() string
	Matches(pane tmux.PaneInfo) bool
	Classify(ctx context.Context, pane tmux.PaneInfo, buffer string) (Classification, error)
	Fingerprint(ctx context.Context, pane tmux.PaneInfo, buffer string) (string, error)
}
```

#### Process Tree Inspection (Issue #3)
Sentinels receive the full `tmux.PaneInfo`, including `pane.PID` (retrieved via `#{pane_pid}` in a single batched `list-panes` call).
When command names are generic wrappers (e.g. `cli`, `agent`, `run`), `Matches(pane)` invokes `tmux.InspectProcessTree(ctx, pane.PID)` to inspect process command lines (reading `/proc/<pid>/cmdline` on Linux or `ps` on macOS), disambiguating agent processes from unrelated CLIs without requiring manual `@glance_routes` overrides.

### In-Scope Sentinels

#### 1. `sentinel-antigravity`
* **Matcher:** Direct match on `cmd =~ ^(agy|antigravity)$`, or process tree inspection (`tmux.InspectProcessTree`) when `cmd` is a generic wrapper (`cli`, `agent`, `run`).
* **Classification:**
  * `WAITING`: Detects semantic blocked/approval cues in the last 15 non-empty lines: subagent blocked lifecycle widgets (`● Agent(...) Blocked`), framed approval headers (`┃ ... needs approval for`), and interactive permission prompts (`Requesting permission for`, `Run this command?` when not followed by `Command finished`). Represents states awaiting user input.
  * `RUNNING`: Detects active foreground execution (`esc to cancel` on the last line) as well as active non-blocked background subagents (`● Agent(...)`).
    * *Nuance (Subagent in flight vs. Foreground execution):* Unlike foreground execution where terminal input is blocked until cancellation/completion, background subagent execution leaves the parent prompt at rest (`>`), meaning the pane can still accept user input. While this could eventually warrant a dedicated state and icon (e.g. `🤖 🔀` subagent-running), classifying it as `RUNNING` for now prevents premature `done` / unread activity alerts while work is actively in flight.
  * `IDLE`: Detects resting footer (`? for shortcuts`) or prompt (`>`). Note that `WAITING` and `RUNNING` (active subagents) cues take precedence over `IDLE`, as the resting prompt (`? for shortcuts`) remains visible while subagents are running or blocked.
* **Thinking Spinner Normalizer:**
  Antigravity streams thoughts and animates braille spinners (`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷]`) in-place during thinking blocks. The normalizer strips these lines before hashing:
  ```go
  func (a *Antigravity) Fingerprint(_ context.Context, _ tmux.PaneInfo, buffer string) (string, error) {
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
  ```

#### 2. `sentinel-generic`
* **Matcher:** Fallback for all other commands (`bash`, `zsh`, `cargo`, `git`, `python`, etc.).
* **Classification:** Returns `watching` or `alert` based on screen fingerprint divergence.
* **Fingerprint:** Raw visible buffer MD5 hash.

### Third-Party Sentinels (e.g. Claude Code, Aider)
* Pluggable into `internal/sentinel/` implementing the Go `Sentinel` interface, or mapped via user route overrides (`@glance_routes`).

---

## 5. State Management & Invariant Engine

### State File Schema (`~/.tmux-glance`)
Tab-delimited flat file:
```
pane_id	session_name	window_index	pane_index	current_path	current_command	label	kind	state
```
* **kind:** `auto` (ephemeral autonomous agent) or `manual` (explicit user Vigil).
* **state:** `waiting`, `running`, `done`, `watching`, `alert`.

### Asynchronous Idle Agent Hashing Contract
Autonomous agents frequently drop back to the prompt (`? for shortcuts`) while background subagents or commands run. Capturing the footer alone misses newly appended tool logs.
1. Idle agents have their screen fingerprinted into `@glance_agent_snapshot`.
2. While the agent pane is in the background, if `curr_hash != @glance_agent_snapshot`, it indicates unread terminal activity.
3. The agent immediately transitions to `done` ("updated in <repo>") and enters the Attention Hub queue.
4. Focusing the pane (`on_focus`) updates the snapshot, acknowledges the activity, and silences the alert.

### Atomic Directory Locking
To prevent corrupt writes or race conditions between tmux's periodic `status-right` scan and interactive commands, `internal/state.Lock` uses atomic directory creation (`os.Mkdir`) with automatic stale-lock recovery after ~1 second (50 attempts × 20ms) and temp-file rename (`os.CreateTemp` + `os.Rename`) for all state, jumplist, and slot file writes.

### Harpoon Session Slots (`~/.local/state/tmux-glance/slots`)
To enable instantaneous, 1-chord teleportation across developer projects without hardcoding cryptic session names (like `0`, `J`, `K`, `L`), `tmux-glance` provides a dynamic Harpoon slot engine.
* **Storage Schema:** Tab-delimited flat file: `<slot>\t<session_name>` (e.g. `j\tbackend-api`). Stored under `$XDG_STATE_HOME/tmux-glance/slots`.
* **Zero Config Drag:** Slots are assigned dynamically without editing Nix or shell environment variables. Survives reboots and works seamlessly with `tmux-resurrect`.
* **Modal Assignment:** Slots are managed directly inside the Sessionizer (`Ctrl-s`) by pressing `Ctrl-p` on any highlighted session, avoiding global key namespace clutter.

---

## 6. Ergonomics & Tiered Keybinding Architecture

To respect established tmux environments and prevent keybinding namespace pollution, `tmux-glance` enforces a strict **Two-Tier Keybinding Policy**:

### Tier 1: Core Non-Contentious Bindings (Enabled by Default)
Only binds dedicated, non-disruptive keys. Preserves all standard tmux navigation, copy-mode, and window management. All complex or mode-specific interactions remain strictly isolated inside the viewfinder popup:

| Keybinding | Scope | Purpose |
| :--- | :--- | :--- |
| `prefix g` | Global | **Global Panes (Go-To):** Search across 100% of panes in every session and window to preview and jump straight there. Active tasks sort to top. |
| `prefix b` | Global | **Attention Hub:** Agent tasks and vigils sorted strictly by severity (`🚨` > `🤖 ⏳` > `🤖 ✓` > `🤖 ⚡` > `👁️` > `🤖 💤`). |
| `prefix v` | Global | **Vigil Toggle:** Instantly watch/unwatch current pane for output. |
| `Ctrl-g` | *Popup only* | **Toggle Panes:** Flip between Global Panes (all) and Attention Hub inside fzf. |
| `Ctrl-s` | *Popup only* | **Sessionizer:** Switch to active tmux sessions with ambient status badges. |
| `Ctrl-p` | *Popup only (Sessions)* | **Pin Slot:** Assign or clear Harpoon session slot (`h/j/k/l`). |
| `Ctrl-h` | *Popup only* | **History:** Switch to Jump History timeline (`FWD` / `CUR` / `BACK`). |
| `Ctrl-x` | *Popup only (History)* | **Clear History:** Wipe the jump history stack cleanly. |
| `Ctrl-/` / `F1` | *Popup only* | **Cheat Sheet:** Display modal-only navigation shortcuts overlay. |
| `Enter` | *Popup only* | **Jump:** Instantly switch client, window, and pane to target. |

### Tier 2: Power-User & Direct Navigation (Strictly Opt-In)
Global navigation chords, jumplist rewinds, and queue cycling can collide with personal shortcuts (e.g. tmux's default `[` for copy-mode or custom window switchers). These are **disabled by default** and require explicit opt-in:

| Keybinding | Scope | Feature | Config Option |
| :--- | :--- | :--- | :--- |
| `prefix Tab` | Global | **Jump History Timeline** (v0.7) | `@glance_enable_jumplist 'on'` (key: `@glance_history_key`) |
| `prefix -r C-h/j/k/l` | Global | **Harpoon Session Jump** | `@glance_enable_harpoon 'on'` (`enableHarpoon = true`) |
| `prefix -r <` / `prefix -r >` | Global | **Repeatable History Jump** (v0.7/v0.8) | `@glance_enable_jumplist 'on'` |
| `prefix -r }` / `prefix -r {` | Global | **Quickfix Alert Cycling** (v0.8) | `@glance_enable_quickfix 'on'` (`enableQuickfix = true`) |

*Note: Users who do not opt into Tier 2 bindings still have 100% access to history, sessions, and navigation features from inside the popup dashboard (`Ctrl-h`, `Ctrl-s`, `Ctrl-p`, etc.) without polluting their global prefix table.*

### Known Ergonomic Limitations & Quirks

* **In-Modal Harpoon Assignment Visual Refresh:**
  - **Behavior:** When pinning or unpinning a Harpoon slot (`Ctrl-p`) from within the sessionizer popup, the slot assignment is recorded immediately to `$SLOTS_FILE`. However, because `fzf`'s curses engine does not dirty stationary lines across background `reload()` when total item count remains unchanged, the new slot badge (`[H]`, `[J]`, etc.) is repainted on screen upon the next cursor movement (pressing `Up` or `Down`).
  - **Status:** Tracked as [Issue #1](https://github.com/mhutchinson/tmux-glance/issues/1) for a future native in-view selection or non-subshell transition.

---

## 7. Design Evolution Log

* **v1.0 Production Hardening, Dogfooding & Polish:**
  - Hardened concurrent state/jumplist/slot writes with `os.CreateTemp`, UTF-8 rune-safe truncation, Vim MRU jumplist deduplication, and 500ms dwell-time auto-acknowledgment (`@glance_dwell_ms`) executed outside the global state lock.
  - Standardized Tier 2 navigation on repeatable `prefix -r <` / `prefix -r >` (history) and `prefix -r }` / `prefix -r {` (quickfix cycling, protecting stock tmux `prefix [` copy-mode).
* **v0.8 Attention Quickfix Cycling & Repeatable Navigation:**
  - Implemented cyclic, Vim-quickfix-style navigation (`next-attention` / `prev-attention`, aliases `cnext` / `cprev`) cycling across active attention panes via Tier 2 opt-in repeatable chords (`prefix -r }` / `prefix -r {`).
  - Strict deterministic queue ordering sorted by severity rank (`🚨 Alert` > `🤖 ⏳ Waiting` > `🤖 ✓ Finished`).
  - Preserved the "whiz-past" invariant: rapid traversal suppresses instantaneous `pane-focus-in` auto-acknowledgment so alerts remain queued during navigation until dwell time expires.
  - Implemented 2-keystroke repeatable jump history flipping (`prefix -r <` / `prefix -r >`) and client repeat mode preservation across jumps.
  - Interactive status feedback (`Glance [1/3] 🚨 Alert: session (label)`).
* **v0.7 Go Engine Rewrite, Jump History & Backtracking Jumplist:**
  - Replaced the bash monolith with the compiled `glance-engine` Go binary and native in-memory Go sentinels.
  - Implemented dual back/forward traversal stack (`jump-back` / `jump-forward`) with liveness verification and `ShiftTo()` frame-shifting (resolving Issue #2).
  - In-modal history inspector (`Ctrl-h`) and Tier 2 `prefix Tab` viewfinder with unified vertical timeline (`FWD` above `CUR` above `BACK`), dynamic FZF cursor positioning on `BACK #1`, and safe no-op on `CUR`.
  - Tier 2 opt-in direct keyboard backtracking chords (repeatable `prefix -r <` / `prefix -r >`).
* **v0.6 In-Dashboard Sessionizer & Harpoon Slots:**
  - Integrated tmux session navigation directly into the Glance popup (`Ctrl-s`).
  - Ambient badge aggregation computes per-workspace telemetry (`🚨 1`, `🤖 ⏳ 1`, `👁️ 2`).
  - Interactive modal Harpoon slot pinning (`Ctrl-p` in Sessionizer $\rightarrow$ `h/j/k/l`).
  - In-modal help cheat sheet (`Ctrl-/` / `F1`) scoped strictly to viewfinder controls.
  - Opt-in Tier 2 fast session jumping via `prefix -r C-h/j/k/l` (`jump-slot`).
* **v0.4–v0.5 Standalone Project Extraction (`tmux-glance`):**
  - Formalized as standalone flake and TPM package with active 500ms real-time preview tailing (`:follow`).
  - Pluggable Sentinel architecture introduced with thinking spinner normalizer.
  - Migrated `prefix B` to `prefix v` (Vigil) for an all-unshifted left-hand navigation cluster (`g`, `b`, `v`).
* **v0.1–v0.3 Early Prototypes:**
  - Initial pinned pane bookmarks, background agent TUI scraping, and output-hashing Vigil alerts.

---

## 8. Future Explorations (Post-v1.0 Candidates)

Potential features evaluated on real-world need (see [README.md § Potential Features](README.md#potential-features) for the full list):

* **External Notification Hooks (`@glance_alert_cmd`):** Invoke a user command (with state and label arguments) whenever a Vigil alert fires or an agent transitions to `🤖 ⏳ waiting`, enabling desktop notifications (`osascript`, `notify-send`) or webhook pings when away from the terminal.
* **Alert Snooze & Age Indicators:** Dismiss or snooze an alert directly from inside the viewfinder without teleporting into the pane, and display inline elapsed time in state (e.g. `🤖 ⏳ 12m`).
* **Global Agent Quotas & Saturation Gauges:**
  - Allow sentinels to optionally contribute a single global capacity gauge (e.g. `#[fg=#fab387]🪫 18%#[default]` when below 20% in `status-right`, always visible in the viewfinder header).
  - **Asynchronous Local Cache Invariant:** Gauges must **never** make synchronous HTTP or CLI queries inside `status-right` polling (must read a local cache file written out-of-band to preserve the `<50ms` status budget).


