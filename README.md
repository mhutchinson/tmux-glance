# 👁️ tmux-glance

**Glanceable terminal sentinels and background agent orchestration for tmux.**  
*Stop speculative window hopping. Stay in flow.*

---

## The Problem: Speculative Window Hopping

In modern development workflows, we juggle multiple long-running terminal tasks:
* Autonomous AI coding agents ([Antigravity](https://github.com), Claude Code, Aider) executing multi-step refactors.
* Compilation pipelines (`cargo build`, `nix build`, `bazel`, `go test`).
* Tail logs, database migrations, and remote test runners.

Without ambient awareness, developers suffer from **speculative window hopping**—repeatedly cycling `prefix n`, `prefix p`, or jumping between sessions just to check:
> *"Did my build finish? Is the agent waiting for confirmation? Did that background test fail?"*

Every speculative jump breaks concentration. 

## The Solution: Ambient Glanceability

`tmux-glance` embeds minimal, ambient telemetry directly into your tmux status bar:

```
[10:42]  🚨 1  👁️ 1  🤖 ⏳ 1  🤖 ⚡ 2
```

If the status bar is quiet, you stay focused on your active code. Badges follow a **User-First Domain Clustering** order (manual user vigils lead, followed by autonomous background agents):
* `🚨 1` — A pane under **Vigil** just printed new terminal output!
* `👁️ 1` — Panes actively being watched under Vigil in the background.
* `🤖 ⏳ 1` — A background agent is blocked **waiting for confirmation**.
* `🤖 ⚡ 2` — Background agents actively running.
* `🤖 ✓ 1` — An agent completed its task or updated its output.

With one keystroke (`prefix b`), open the **Attention Hub** popup to see only the tasks that need you, inspect their output with live ANSI preview, and press `Enter` to jump straight to the pane.

---

## Demos & Workflow

### 1. Ambient Status Bar Telemetry
Ambient icons stay out of your way until state changes (manual vigils grouped first, then background agents):

```tmux
#[fg=#f38ba8,bold]🚨 1#[default]  #[fg=#b4befe]👁️ 2#[default]  #[fg=#fab387,bold]🤖 ⏳ 1#[default]
```

### 2. Attention Hub (`prefix b`)
Opens a floating `fzf` popup sorted strictly by urgency. Puts blocked agents and firing alerts at the top with a live 30-line terminal preview:

```text
┌── 👁️ tmux-glance (Ctrl-b: toggle Fleet / Attention | Enter: jump | Esc: cancel) ──┐
│🚨 Alert       │ [0:3.1] cargo    │ api-server     │ test failed in api-server     │
│🤖 ⏳ Waiting   │ [0:2.1] agy      │ nix-home       │ waiting for confirmation      │
│🤖 ✓ Finished  │ [J:1.2] agy      │ backend-auth   │ updated in backend-auth       │
│👁️ Vigil       │ [0:4.1] tail     │ production-log │ tail -f /var/log/syslog       │
│                                                                                   │
│───────────────────────────────────────────────────────────────────────────────────│
│  [Preview: Pane %2 (api-server)]                                                  │
│  running 14 tests                                                                 │
│  test tests::test_session_expiry ... ok                                           │
│  test tests::test_token_refresh ... FAILED                                        │
│                                                                                   │
│  failures:                                                                        │
│      tests::test_token_refresh                                                    │
│                                                                                   │
│  test result: FAILED. 13 passed; 1 failed; finished in 1.42s                      │
└───────────────────────────────────────────────────────────────────────────────────┘
```

### 3. Glance Mode / Fleet View (`prefix g`)
Inspect every active agent session and watched process across all sessions on the tmux server:

```text
┌── 👁️ tmux-glance (Ctrl-b: toggle Fleet / Attention | Enter: jump | Esc: cancel) ──┐
│🤖 ⚡ Running   │ [0:2.1] agy      │ nix-home       │ running in nix-home           │
│🤖 💤 Idle     │ [0:3.1] agy      │ mobile-app     │ idle in mobile-app            │
│🤖 💤 Idle     │ [J:1.1] agy      │ cloud-infra    │ idle in cloud-infra           │
│👁️ Vigil       │ [K:2.1] docker   │ monitoring     │ docker compose up             │
└───────────────────────────────────────────────────────────────────────────────────┘
```
> **Tip:** Press `Ctrl-b` inside the popup at any time to toggle between the Attention Hub and the full Fleet View!

### 4. Vigil Watch (`prefix v`)
Place a vigil on *any* shell, build, or long-running command.
* Press `prefix v` in the pane.
* Switch away and keep working.
* As soon as output changes, `status-right` lights up with `🚨 1`.
* Jumping to the pane automatically acknowledges the alert and resets the baseline.

---

## Key Features

* **Intelligent Thought Normalizer:** AI agents animate braille spinners (`[⠋⠙⠹...][⣾⣽⣻⢿]`) and stream thoughts in-place. `tmux-glance` normalizes and filters out transient thought streams, ensuring unread alerts only trigger on real actions or tool completions.
* **Auto-Acknowledgment on Focus:** No tedious alert dismissal. Simply switching focus into a pane (`pane-focus-in` hook) acknowledges and clears its notification.
* **Zero Polling Lag & Atomic Locking:** State updates use file locking (`.lock`) with sub-millisecond execution, avoiding status bar micro-stutters or race conditions.
* **Active Real-Time Tailing:** The interactive viewer actively streams and updates the focused pane's live buffer in real-time, letting you watch agent output, compiler logs, and thinking progress without needing to navigate or reload.
* **Pluggable Sentinels:** Clean provider interface decouples the core multiplexer from specific agent TUIs.

---

## Prerequisites

* **tmux ≥ 3.2**: Required for floating popup windows (`display-popup`).
* **fzf**: Required for interactive picker and terminal live previews.
* Standard POSIX utilities: `bash`, `coreutils` (`awk`, `grep`, `sed`, `md5` or `md5sum`).

---

## Installation

### Option 1: Nix Flake + Home Manager (Recommended)

Add `tmux-glance` to your `flake.nix`:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    tmux-glance = {
      url = "github:mhutchinson/tmux-glance";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, tmux-glance, ... }: {
    homeConfigurations."myuser" = home-manager.lib.homeManagerConfiguration {
      modules = [
        tmux-glance.homeManagerModules.default
        {
          programs.tmux-glance = {
            enable = true;

            # Optional keybinding customization (defaults: g, b, v)
            keybindings = {
              glance = "g"; # prefix + g: Server-wide Fleet View
              hub = "b";    # prefix + b: Attention Hub
              vigil = "v";  # prefix + v: Toggle Vigil on current pane
            };

            # Optional popup window dimensions (defaults: 85% / 75%)
            popup = {
              width = "85%";
              height = "75%";
            };
          };
        }
      ];
    };
  };
}
```

The Home Manager module automatically installs the wrapped package with all runtime dependencies, sets up the keybindings, and registers the `pane-focus-in` auto-acknowledgment hook.

Then add ambient telemetry to your tmux status bar in your tmux configuration:
```tmux
set -g status-right '#(tmux-glance status) %H:%M '
```

---

### Option 2: Standalone Nix Profile (Without Home Manager)

If you use Nix but do not use Home Manager:

```bash
nix profile install github:mhutchinson/tmux-glance
```

Then add the following to your `~/.tmux.conf`:

```tmux
# tmux-glance bindings
bind-key g display-popup -E -w 85% -h 75% "tmux-glance list-all"
bind-key b display-popup -E -w 85% -h 75% "tmux-glance list"
bind-key v run-shell "tmux-glance toggle-vigil"

# Auto-acknowledge alerts when focusing a pane
set-hook -g pane-focus-in "run-shell 'tmux-glance on-focus #{pane_id}'"

# Ambient status telemetry
set -g status-right '#(tmux-glance status) %H:%M '
```

---

### Option 3: Tmux Plugin Manager (TPM)

Add `tmux-glance` to your TPM plugins in `~/.tmux.conf`:

```tmux
set -g @plugin 'mhutchinson/tmux-glance'

# Optional keybinding customization (defaults: g, b, v)
# set -g @glance_key 'g'
# set -g @glance_hub_key 'b'
# set -g @glance_vigil_key 'v'

# Ambient status telemetry
set -g status-right '#(tmux-glance status) %H:%M '
```

Press `prefix + I` to fetch the plugin and activate.

---

### Option 4: Manual Git Installation (No Nix or TPM required)

1. Ensure `tmux` and `fzf` are installed:
   ```bash
   # macOS
   brew install tmux fzf

   # Debian / Ubuntu
   sudo apt install tmux fzf

   # Fedora / RHEL
   sudo dnf install tmux fzf
   ```

2. Clone the repository into your preferred location:
   ```bash
   git clone https://github.com/mhutchinson/tmux-glance ~/.tmux-glance
   ```

3. Add either the automated loader or explicit bindings to your `~/.tmux.conf`:

   **Automated loader (`glance.tmux`):**
   ```tmux
   run-shell ~/.tmux-glance/glance.tmux
   set -g status-right '#(~/.tmux-glance/bin/tmux-glance status) %H:%M '
   ```

   **Or explicit manual bindings:**
   ```tmux
   bind-key g display-popup -E -w 85% -h 75% "~/.tmux-glance/bin/tmux-glance list-all"
   bind-key b display-popup -E -w 85% -h 75% "~/.tmux-glance/bin/tmux-glance list"
   bind-key v run-shell "~/.tmux-glance/bin/tmux-glance toggle-vigil"

   set-hook -g pane-focus-in "run-shell '~/.tmux-glance/bin/tmux-glance on-focus #{pane_id}'"
   set -g status-right '#(~/.tmux-glance/bin/tmux-glance status) %H:%M '
   ```

4. Reload your tmux configuration:
   ```bash
   tmux source-file ~/.tmux.conf
   ```

---

## Verifying Your Installation

1. **Check the CLI:**
   ```bash
   tmux-glance status
   ```
   *(Outputs nothing if all background panes are quiet, or formatted badges if agents are active).*

2. **Test Keybindings:**
   * Press `prefix + g`: The **Glance Fleet View** popup should appear.
   * Press `prefix + b`: The **Attention Hub** popup should appear.
   * Press `prefix + v`: You should see a status message: `👁️ Vigil active: ...` (press again to release).

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

## Configuring Routes & Disabling Sentinels

`tmux-glance` decouples sentinel implementations from command names through an **$O(1)$ Command Routing Table**.

If you have a binary with a colliding name (a doppelgänger CLI) or want to ignore an upstream sentinel, you can configure routes and disables with zero code changes:

### In Nix / Home Manager (`home.nix`):

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

### In `~/.tmux.conf` (TPM / Manual):

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

The repository includes a convenient `Justfile` for local development and CI:

```bash
# List all recipes
just

# Run the unit and integration test suite
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
* [x] **v0.1: Pinned Tmux Bookmarks** — Basic pane pinning, persistent state, and fuzzy jumping via floating popup.
* [x] **v0.2: Autonomous Agent Detection** — Background scraping of agent TUIs, heuristic state classification (`waiting`, `running`, `idle`), and ANSI preview.
* [x] **v0.3: Dynamic Vigil Watches** — Output diffing and hashing for arbitrary shell commands; automatic promotion of background watches (`👁️`) to alerts (`🚨`) with focus auto-acknowledgment (`pane-focus-in`).
* [x] **v0.4: Standalone Flake & Pluggable Sentinels** — Standalone flake with Apache 2.0 license, modular sentinels (`antigravity`, `generic`), thinking spinner normalizer, $O(1)$ command routing table (`routes`), Home Manager module, and cross-platform GitHub Actions CI.
* [x] **v0.5: Active Real-Time Tailing & Viewfinder Pinning** — Real-time 500ms diff-hashing preview tailing and bottom viewport locking (`:follow`) for live prompt and build tracking.

### Upcoming Roadmap
* [ ] **v0.6: In-Dashboard Sessionizer (`Ctrl-s`)** — Search and switch tmux sessions directly within the Glance dashboard, complete with ambient status badge summaries (`🚨 1`, `🤖 ⏳ 1`, `👁️ 2`) representing the state of each workspace.
* [ ] **v0.7: Hierarchy Telescoping (`Ctrl-w` / `Ctrl-p`)** — Expand picker scope to search across all open windows (`Ctrl-w`) or all active panes (`Ctrl-p`) across the server, creating a unified navigation hub.
* [ ] **v0.8: Jump History & Jumplist Backtracking (`Ctrl-h` / Undo-Redo)** — Dual Back/Forward jump stack for seamless pane navigation. Full in-viewer history stack (`Ctrl-h`), with opt-in instant undo/redo chords (`prefix C-z` / `prefix C-y` or repeatable `prefix -r u` / `U`) to jump straight back without opening a menu.
* [ ] **v0.9: Global Agent Quotas & Saturation Gauges** — Sentinel capacity/quota hook (`sentinel_<name>_gauge`). Kept quiet in `status-right` until capacity drops below 20% (escalating to warning colors), always visible in the dashboard header, strictly backed by asynchronous local cache.
* [ ] **v0.10: Quickfix Attention Cycling & Queue HUD (`prefix -r ]` / `prefix -r [`)** — Vim quickfix-style cycling directly through panes contributing active status icons. Opt-in repeatable `[` and `]` navigation with a docked mini-queue HUD / status overlay and dwell-time auto-ack suppression to avoid dismissing alerts while whizzing past.



---

## License

[Apache 2.0](LICENSE) © 2026 Martin Hutchinson
