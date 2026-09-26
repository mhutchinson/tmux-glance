# 👁️ tmux-glance

<p align="center">
  <img src="assets/tmux-glance-sticker.jpg" alt="tmux-glance: Stay in flow" width="640" />
</p>

<p align="center">
  <strong>Ambient activity indicators, crystal-ball pane peering, and instant teleportation for tmux.</strong><br>
  <em>Stop speculative window hopping. Stay in flow.</em>
</p>

<p align="center">
  <img src="assets/demo.gif" alt="tmux-glance in action: crystal-ball peering and instant teleportation" width="760" />
</p>
<p align="center">
  <sub><em>💡 <strong>In Action:</strong> Status bar alerts trigger while in flow (<code>main.go</code>) → <code>prefix b</code> opens the Attention Hub viewfinder with live ANSI preview → <code>Enter</code> teleports into the agent prompt to approve.<br>(Note: The bottom <code>💡 DEMO</code> subtitle bar illustrates keypresses and narration for this demo; <code>tmux-glance</code> itself lives ambiently in your status bar).</em></sub>
</p>

---

## What is tmux-glance?

`tmux-glance` turns tmux into an ambient heads-up display for changes in background terminals, such as long-running builds, test suites, and autonomous AI agents:

- 🔮 **Crystal Ball Peering (`prefix b` / `prefix g`):** Pop open a floating viewfinder to peer into any pane across any session on your tmux server. Watch compiler output, log streams, or agent progress in real-time ANSI preview without leaving your current workspace.
- ⚡ **Universal Go-To & Instant Teleportation (`prefix g` + `Enter`):** Type any directory name, repository, or command to fuzzy-match across all panes in every session and window, preview live output, and jump straight there.
- 👁️ **Glanceable Status Bar Icons:** Ambient indicators (`🚨 1`, `👁️ 2`, `🤖 ⏳ 1`, `🤖 ⚡ 2`) sit quietly in your status bar. If it's quiet, you stay in flow; if a build fails or an agent is blocked waiting for you, you know immediately.
- 🤖 **Automatic Agent Monitoring (Zero Setup):** You don't need to remember to watch your AI coding assistants. `tmux-glance` automatically discovers background agent sessions (such as Antigravity, Claude Code, and Aider) across all windows and sessions, categorizing their live states (`🤖 ⚡ running`, `🤖 ⏳ waiting for input`, `🤖 ✓ finished`) while intelligently normalizing animated braille spinners and streaming thoughts.
- 🎯 **One-Key Vigil Watches (`prefix v`):** For everything else—long builds (`cargo build`, `nix build`), database migrations, or test suites—slap a watchful sentinel on any pane with `prefix v`. Switch away, and your status bar will flash `🚨 1` the exact moment new output appears or the process exits. Merely focusing the pane auto-acknowledges the alert.
  - _Want custom trigger logic or noise filtering for a specific application?_ Write your own [**Sentinel plugin**](#pluggable-sentinels) in a few lines of bash! Sentinels can classify custom states (`waiting`, `running`, `idle`) and normalize spinners or streaming thought lines so you only get alerted when it truly matters.

---

## The Problem: Speculative Window Hopping

In modern development workflows, we juggle multiple long-running terminal tasks:

- Autonomous AI coding agents (Antigravity, Claude Code, Aider) executing multi-step refactors.
- Compilation and testing (`cargo build`, `nix build`, `bazel`, `go test`).
- Tail logs, database migrations, and remote test runners.

Without ambient awareness, developers suffer from **speculative window hopping**—repeatedly cycling `prefix n`, `prefix p`, or jumping between sessions just to check:

> _"Did my build finish? Is the agent waiting for confirmation? Did that background test fail?"_

Every speculative jump breaks concentration.

## The Solution: Ambient Glanceability

`tmux-glance` embeds minimal, ambient telemetry directly into your tmux status bar:

```
[10:42]  🚨 1  👁️ 1  🤖 ⏳ 1  🤖 ⚡ 2
```

If the status bar is quiet, you stay focused on your active code. Badges follow a **User-First Domain Clustering** order (manual user vigils lead, followed by autonomous background agents):

- `🚨 1` — A pane under **Vigil** just printed new terminal output!
- `👁️ 1` — Panes actively being watched under Vigil in the background.
- `🤖 ⏳ 1` — A background agent is blocked **waiting for confirmation**.
- `🤖 ⚡ 2` — Background agents actively running.
- `🤖 ✓ 1` — An agent completed its task or updated its output.

With one keystroke (`prefix b`), open the **Bots & Views** popup to see agent tasks and vigils (with urgent alerts and blocked prompts prioritized to the top), inspect their output with live ANSI preview, and press `Enter` to jump straight to the pane.

---

## Default Keybindings

`tmux-glance` registers only three non-disruptive Tier 1 keychords by default, never hijacking built-in tmux keys:

| Key | Action | Description |
| :--- | :--- | :--- |
| `prefix g` | **Global Panes (Go-To)** | Server-wide teleporter across 100% of panes in every session and window. Active tasks sort to top; preview live output and jump straight there. |
| `prefix b` | **Bots & Views** | Sentinels & watches filtered to agent tasks and manual vigils, sorted strictly by priority (`🚨`, `🤖 ⏳`, `🤖 ✓`, `🤖 ⚡`, `👁️`, `🤖 💤`). |
| `prefix v` | **Toggle Vigil** | Slap a watchful sentinel on the current pane (or release it). |

> **Inside the Viewfinder Popup:**
> * `Ctrl-g` — Switch directly to Global Panes (All Panes).
> * `Ctrl-b` — Toggle between Bots & Views and Global Panes.
> * `Ctrl-s` — Switch to the Sessionizer to search and jump between active tmux sessions with ambient status badges.
> * `Ctrl-h` — Switch to Jump History timeline (`FWD` / `CUR` / `BACK`) with live previews.
> * `Ctrl-x` — Clear the jump history stack (useful for context switches or privacy).
> * `Ctrl-p` — Pin / unpin highlighted session to a Harpoon slot (`h`, `j`, `k`, `l`). *(Note: badge renders on next cursor movement `Up`/`Down`)*.
> * `Ctrl-/` / `F1` — Open the in-modal cheat sheet overlay.
> * `Enter` — Teleport straight into the selected pane or session.
> * `Esc` — Close the popup without jumping.

---

## Key Features

- **Universal Go-To Directory Teleporter (`prefix g`):** `prefix g` isn't just an agent dashboard—it's the fastest teleporter across your entire tmux server. Every pane is indexed by its active directory, repository name, session, window, and foreground process. Press `prefix g`, type a project or directory name (e.g. `nix-home`, `tmux-glance`, `backend`), preview its live state in the viewfinder, and press `Enter` to switch client, window, and pane in a single stroke.
- **Intelligent Thought Normalizer:** AI agents animate braille spinners (`[⠋⠙⠹...][⣾⣽⣻⢿]`) and stream thoughts in-place. `tmux-glance` normalizes and filters out transient thought streams, ensuring unread alerts only trigger on real actions or tool completions.
- **Auto-Acknowledgment on Focus:** No tedious alert dismissal. Simply switching focus into a pane (`pane-focus-in` hook) acknowledges and clears its notification.
- **Zero Polling Lag & Atomic Locking:** State updates use file locking (`.lock`) with sub-millisecond execution, avoiding status bar micro-stutters or race conditions.
- **Active Real-Time Tailing:** The interactive viewer actively streams and updates the focused pane's live buffer in real-time, letting you watch agent output, compiler logs, and thinking progress without needing to navigate or reload.
- **Pluggable Sentinels:** Clean provider interface decouples the core multiplexer from specific agent TUIs.

---

## Prerequisites

- **tmux ≥ 3.2**: Required for floating popup windows (`display-popup`).
- **fzf**: Required for interactive picker and terminal live previews.
- Standard POSIX utilities: `bash`, `coreutils` (`awk`, `grep`, `sed`, `md5` or `md5sum`).

---

## Installation

### Option 1: Nix Flake + Home Manager (Recommended)

Add `tmux-glance` to your `flake.nix` inputs:

```nix
inputs.tmux-glance.url = "github:mhutchinson/tmux-glance";
```

Enable the module in your Home Manager configuration:

```nix
imports = [ inputs.tmux-glance.homeManagerModules.default ];

programs.tmux-glance.enable = true;
```

Then add ambient telemetry to your status bar in your tmux config:

```tmux
set -g status-right '#(tmux-glance status) %H:%M '
```

---

### Option 2: Tmux Plugin Manager (TPM)

Add `tmux-glance` to your plugins in `~/.tmux.conf`:

```tmux
set -g @plugin 'mhutchinson/tmux-glance'
set -g status-right '#(tmux-glance status) %H:%M '
```

Press `prefix + I` to fetch the plugin and activate.

---

### Option 3: Manual Git Clone

1. Clone the repository:

   ```bash
   git clone https://github.com/mhutchinson/tmux-glance ~/.tmux-glance
   ```

2. Add the automated loader to `~/.tmux.conf`:

   ```tmux
   run-shell ~/.tmux-glance/glance.tmux
   set -g status-right '#(~/.tmux-glance/bin/tmux-glance status) %H:%M '
   ```

3. Reload your tmux configuration:

   ```bash
   tmux source-file ~/.tmux.conf
   ```

---

### Option 4: Standalone Nix Profile (Without Home Manager)

```bash
nix profile install github:mhutchinson/tmux-glance
```

Add to `~/.tmux.conf`:

```tmux
bind-key g display-popup -E -w 85% -h 75% "tmux-glance list-all"
bind-key b display-popup -E -w 85% -h 75% "tmux-glance list"
bind-key v run-shell "tmux-glance toggle-vigil"
set-hook -g pane-focus-in "run-shell 'tmux-glance on-focus #{pane_id}'"
set -g status-right '#(tmux-glance status) %H:%M '
```

---

## Verifying Your Installation

1. **Check the CLI:**

   ```bash
   tmux-glance status
   ```

   _(Outputs nothing if all background panes are quiet, or formatted badges if agents are active)._

2. **Test Keybindings:**
   - Press `prefix + g`: The **Global Panes (Go-To)** popup should appear.
   - Press `prefix + b`: The **Bots & Views** popup should appear.
   - Press `prefix + v`: You should see a status message: `👁️ Vigil active: ...` (press again to release).

---

## Pluggable Sentinels

`tmux-glance` uses modular Sentinels to inspect different agent TUIs and processes. Sentinels live in `sentinels/` or your personal config directory `~/.config/tmux-glance/sentinels/`.

### The Sentinel Contract

To add support for a new tool (e.g. Claude Code, Aider, or a custom build tool), create a shell script:

```bash
#!/usr/bin/env bash

# 1. Suggested default commands for the routing table
sentinel_myagent_default_commands=("myagent" "myagent-cli")

# 2. Classifier: return "state\tlabel"
#    Available states: waiting | running | done | idle
sentinel_myagent_classify() {
    local pane_id="$1" path="$2" cmd="$3"
    local tail_text
    tail_text=$(tmux capture-pane -p -t "$pane_id" 2>/dev/null | tail -n 4)

    if [[ "$tail_text" =~ "Permission required" ]]; then
        printf "waiting\tpermission required in %s\n" "$(basename "$path")"
    elif [[ "$tail_text" =~ "Working..." ]]; then
        printf "running\trunning in %s\n" "$(basename "$path")"
    else
        printf "idle\tidle in %s\n" "$(basename "$path")"
    fi
}

# 3. Fingerprint: return normalized hash of screen buffer
sentinel_myagent_fingerprint() {
    local pane_id="$1"
    tmux capture-pane -p -t "$pane_id" 2>/dev/null \
        | grep -vE '^[[:space:]]*Progress: [0-9]+%' \
        | md5sum | cut -d' ' -f1
}
```

Drop it in `~/.config/tmux-glance/sentinels/myagent.sh` and it will be loaded automatically!

---

## Advanced Configuration

### 1. Custom Keybindings & Popup Dimensions

#### In Home Manager (`home.nix`):

```nix
programs.tmux-glance = {
  enable = true;

  # Custom keybindings (defaults: g, b, v; Tier 2: Tab)
  keybindings = {
    glance = "g";  # prefix + g: Global Panes (Go-To Teleport)
    hub = "b";     # prefix + b: Bots & Views
    vigil = "v";   # prefix + v: Toggle Vigil on current pane
    history = "Tab"; # prefix + Tab: Jump History Timeline (Tier 2)
  };

  # Custom popup window dimensions (defaults: 85% / 75%)
  popup = {
    width = "85%";
    height = "75%";
  };

  # Enable Tier 2 Harpoon fast-jump chords (prefix C-h, C-j, C-k, C-l):
  enableHarpoon = true;

  # Enable Tier 2 Jumplist backtrack chords & history modal (prefix Tab, C-z, C-y, repeatable prefix -r <, >, u, U):
  enableJumplist = true;

  # Enable Tier 2 Quickfix attention queue cycling (repeatable prefix -r ], [):
  enableQuickfix = true;

  # Cooldown threshold in seconds to debounce background scans when focus is unchanged (default: 3)
  scanCooldown = 3;
};
```

#### In `~/.tmux.conf` (TPM / Manual):

```tmux
# Custom keybindings (defaults: g, b, v)
set -g @glance_key 'g'
set -g @glance_hub_key 'b'
set -g @glance_vigil_key 'v'

# Custom popup window dimensions (defaults: 85% / 75%)
set -g @glance_popup_width '85%'
set -g @glance_popup_height '75%'

# Enable Tier 2 Harpoon fast-jump chords (prefix C-h, C-j, C-k, C-l):
set -g @glance_enable_harpoon 'on'

# Enable Tier 2 Jumplist backtrack chords & history modal (prefix Tab, C-z, C-y, -r <, -r >, -r u, -r U):
set -g @glance_enable_jumplist 'on'

# Enable Tier 2 Quickfix attention queue cycling (repeatable prefix -r ], -r [):
set -g @glance_enable_quickfix 'on'

# Optional: customize history viewfinder key (default: Tab)
# set -g @glance_history_key 'Tab'

# Cooldown threshold in seconds to debounce background scans when focus is unchanged (default: 3)
set -g @glance_scan_cooldown 3
```

---

### 2. Harpoon Session Slots
Pin your top 4 projects to instant 1-chord shortcuts:
1. Open the Sessionizer (`prefix g` or `prefix b` then `Ctrl-s`).
2. Highlight a session and press `Ctrl-p`.
3. Tap `h`, `j`, `k`, or `l` (or Space/Del to unpin).
4. Jump straight to it at any time using `prefix C-h`, `prefix C-j`, `prefix C-k`, or `prefix C-l`!

---

### 3. Jump History & Jumplist Backtracking
Traverse seamlessly back and forth between recent panes across all sessions without losing context:
* **In-Modal History Timeline (`Ctrl-h`):** From inside the Glance popup, press `Ctrl-h` to open the unified jump timeline. Panes are ordered vertically through time: forward/redo destinations (`⏭️ FWD`) sit above the current active pane (`📍 CUR`), which sits above past undo destinations (`⏮️ BACK`). By default, the immediate undo target (`BACK #1`) is highlighted so pressing `Enter` instantly jumps back. Pressing `Up` moves to `CUR` (where `Enter` is a safe no-op), and pressing `Up` again walks into future redo jumps (`FWD #1`). Press `Ctrl-x` inside the modal (or run `tmux-glance clear-history` via CLI) to wipe the jump history stack cleanly when switching contexts.
* **Instant Keyboard Undo/Redo & Viewfinder (Tier 2 Opt-in):** Enable `enableJumplist = true;` (`@glance_enable_jumplist 'on'`) to get direct chord navigation:
  * `prefix Tab` — Instant Jump History timeline viewfinder (configurable via `@glance_history_key`).
  * `prefix C-z` — Instant single-chord jump back (undo last jump; safely replaces tmux's default `suspend-client`).
  * `prefix C-y` — Instant single-chord jump forward (redo).
  * `prefix -r <` / `prefix -r >` — Repeatable history traversal (tap `prefix < <` or `prefix > >` to flip back and forth between panes with 2 keystrokes).
  * `prefix -r u` / `prefix -r U` — Repeatable backward and forward walk.

---

### 4. Quickfix Attention Queue Cycling (Tier 2 Opt-in)
Cycle through active attention items (`🚨 Alert`, `🤖 ⏳ Waiting`, `🤖 ✓ Finished`) just like Vim quickfix (`:cnext` / `:cprev`):
* Enable `enableQuickfix = true;` in Home Manager or `set -g @glance_enable_quickfix 'on'` in `~/.tmux.conf`.
* `prefix -r ]` — Jump to next attention item (repeatable: tap `prefix ] ] ]` to triage successive items).
* `prefix -r [` — Jump to previous attention item (repeatable).
* Circular navigation wraps around the queue and displays progress: `Glance [1/3] 🚨 Alert: session (label)`.

---

### 5. Command Routing & Disabling Sentinels

`tmux-glance` decouples sentinel implementations from command names through an **$O(1)$ Command Routing Table**.

If you have a binary with a colliding name (a doppelgänger CLI) or want to ignore an upstream sentinel, you can configure routes and disables with zero code changes:

#### In Home Manager (`home.nix`):

```nix
programs.tmux-glance = {
  enable = true;

  # Completely disable specific sentinels:
  disabledSentinels = [ "claude" "aider" ];

  # Command routing table overrides:
  routes = {
    # Resolve doppelgänger: force 'chat' to be treated as a normal shell
    "chat" = "generic";

    # Route custom wrappers or binary names to a sentinel:
    "my-internal-agent" = "antigravity";
  };
};
```

#### In `~/.tmux.conf` (TPM / Manual):

```tmux
# Disable specific sentinels:
set -g @glance_disabled_sentinels 'claude,aider'

# Command route overrides: <command>=<sentinel>
set -g @glance_routes 'chat=generic,my-internal-agent=antigravity'
```

---

## Architecture

For deep dives into data flow, screen fingerprinting, state files, and race condition prevention, refer to [DESIGN.md](DESIGN.md).

---

## Development & Contributing

The repository includes a convenient `Justfile` for local development, instant iteration, and CI:

### ⚡ Zero-Rebuild Live Development (`just live`)

When developing `tmux-glance`, you do not need to push to git or rebuild Nix packages to test changes. You can bind your running tmux session directly to your working tree:

```bash
# Wire active tmux keybindings directly to this local checkout
just live
```

* **Instant Feedback**: Edit scripts in `bin/` or `sentinels/` $\rightarrow$ Save $\rightarrow$ Press `prefix + s` (or `b` / `g`) in tmux. Changes take effect on the very next keystroke.
* **Reverting to Installed Package**: Press `prefix + r` (or run `tmux source-file ~/.config/tmux/tmux.conf`) to reset bindings back to your standard installed package.

### 🧪 Automated Testing & CI

```bash
# Run unit, integration, and headless fzf syntax tests
just test

# Run Nix flake checks (builds package + runs sandbox tests)
just check

# Run ShellCheck across all scripts
just lint

# Run all local CI verification checks (lint, test, check)
just ci

# Check GitHub Actions, PRs, and Issues
just actions
just gh-prs
just gh-issues
```

---

## 🗺️ Milestones & Roadmap

### Completed Milestones

- [x] **v0.1: Pinned Tmux Bookmarks** — Basic pane pinning, persistent state, and fuzzy jumping via floating popup.
- [x] **v0.2: Autonomous Agent Detection** — Background scraping of agent TUIs, heuristic state classification (`waiting`, `running`, `idle`), and ANSI preview.
- [x] **v0.3: Dynamic Vigil Watches** — Output diffing and hashing for arbitrary shell commands; automatic promotion of background watches (`👁️`) to alerts (`🚨`) with focus auto-acknowledgment (`pane-focus-in`).
- [x] **v0.4: Standalone Flake & Pluggable Sentinels** — Standalone flake with Apache 2.0 license, modular sentinels (`antigravity`, `generic`), thinking spinner normalizer, $O(1)$ command routing table (`routes`), Home Manager module, and cross-platform GitHub Actions CI.
- [x] **v0.5: Active Real-Time Tailing & Viewfinder Pinning** — Real-time 500ms diff-hashing preview tailing and bottom viewport locking (`:follow`) for live prompt and build tracking.
- [x] **v0.6: In-Dashboard Sessionizer (`Ctrl-s`) & Jump History** — Search and switch tmux sessions directly within the Glance dashboard with ambient status badges. Dual Back/Forward jump stack (`Ctrl-h`, `jump-back`, `jump-forward`) with Harpoon slot assignment.
- [x] **v0.7: Go Engine Rewrite** — Replaced the 1,816-line bash monolith with a typed Go binary (`glance-engine`) and a 140-line thin bash dispatcher (93% bash reduction). Goroutine fan-out for concurrent pane evaluation. Fixes Issues [#2](https://github.com/mhutchinson/tmux-glance/issues/2) (history frame-shift), [#3](https://github.com/mhutchinson/tmux-glance/issues/3) (sentinel `pane_id` missing), and [#5](https://github.com/mhutchinson/tmux-glance/issues/5) (stale vigil labels) structurally.
- [x] **v0.8: Quickfix Attention Cycling & Repeatable Navigation (`prefix -r ]` / `prefix -r [`)** — Vim quickfix-style cycling directly through active attention panes (`🚨 Alert` > `🤖 ⏳ Waiting` > `🤖 ✓ Finished`) with cyclic progress status feedback. Repeatable 2-keystroke jump history flipping (`prefix -r <` / `prefix -r >`) and whiz-past dwell-time protection to prevent premature alert auto-acknowledgment.

### Upcoming Roadmap

- [ ] **v1.0: Production Hardening, Dogfooding & Polish** — End-to-end edge-case hardening across diverse terminal dimensions and nested tmux workflows, full dogfooding cycle, documentation polish, and config contract freeze.

### Potential Features

Ideas that may graduate to a versioned milestone depending on whether real-world usage reveals a compelling need. The first three are the most likely candidates.

- 📢 **External Notification Hooks** — A `@glance_alert_cmd` hook invoked (with state + label as arguments) whenever an alert fires or an agent becomes `🤖 ⏳ waiting`. Enables desktop notifications (`osascript`, `notify-send`), webhook pings, or any shell command — critical when you step away from the terminal entirely.
- 🔕 **Alert Snooze / Dismiss-Without-Visit** — Acknowledge or snooze a `🚨` alert directly from the viewfinder (e.g. `Ctrl-d` in the modal, or a key in quickfix cycling) without teleporting into the pane. Useful when you’ve seen the alert but aren’t ready to context-switch yet.
- 🕐 **Alert Age / Elapsed Time** — Show how long a pane has been in its current state inline in the viewfinder (e.g. `🤖 ⏳ 12m`). Lets you triage at a glance whether an agent has been blocked for 2 minutes or 45, without visiting it.
- 📊 **Global Agent Quota Gauges** — Ambient capacity indicators for LLM API quotas (e.g. `🪫 18%`) in the status bar and viewfinder header, backed by an asynchronous local cache. Harder than it sounds: Antigravity exposes 4 independent quota bars (5-hour and weekly, across 2 model types) that don’t reduce cleanly to a single number.
- 🏷️ **Pane Labels / Aliases** — Attach a human-readable name to any pane (e.g. `tmux-glance label "prod migration"`), shown in the viewfinder and status badge instead of the raw command/directory. Sharpens scannability when many panes share the same working directory or shell.
- 🛑 **Kill / Interrupt From Viewfinder** — Send `SIGINT` (or `SIGKILL`) to a selected pane’s foreground process directly from the modal without teleporting there. A natural escape hatch when an agent or build is clearly stuck in a loop.
- 📋 **Per-Pane Scratch Notes** — Attach a short freeform note to any vigil watch (e.g. `"running prod DB migration — don’t kill"`), displayed alongside the live preview in the viewfinder.

---

## License

[Apache 2.0](LICENSE) © 2026 Martin Hutchinson
