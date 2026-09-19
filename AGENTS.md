# Agent Guidelines & Invariants: tmux-glance

> **Operating Contract for AI Agents**: This document specifies non-negotiable architectural invariants, development conventions, and pitfalls for AI agents contributing to `tmux-glance`. Read this before proposing or implementing changes.

---

## 1. Core Philosophy: Minimally Invasive by Default

`tmux-glance` is designed to be added to mature, heavily customized tmux configurations with zero friction or unintended side effects.

* **Never Pollute the Global Keybinding Namespace:**
  * **Tier 1 (Core Defaults):** Only register dedicated, non-disruptive bindings by default (`prefix g` for Glance Fleet, `prefix b` for Attention Hub, `prefix v` for Vigil).
  * **Tier 2 (Power-User Navigation):** All direct-jump chords, jumplist rewinds (`C-z`/`C-y`, repeatable `u`/`U`), and quickfix alert cycling (`[`/`]`) **MUST be strictly opt-in** via configuration options (`@glance_enable_*` or Home Manager options).
  * **Forbidden Key Hijacking:** Never hijack standard tmux defaults (especially `prefix [` which is tmux's default copy-mode, or `prefix c`, `z`, `n`, `p`) in the default configuration.
  * **Subprocess Scoping:** Interactive navigation keys (`Ctrl-b`, `Ctrl-s`, `Ctrl-h`, `Enter`) must stay strictly confined within the `fzf` popup subprocess.

---

## 2. Hard Invariants & Performance Gotchas

### A. Sub-50ms Status Bar Execution Budget
* `tmux-glance status` runs synchronously inside tmux's `status-right` on every `status-interval` (default 2–5 seconds).
* **Hard Rule:** Execution must complete in **< 50ms**.
* **Zero Network Requests:** Never make HTTP, curl/xh, or CLI API queries inside `status` or sentinels.
* **Asynchronous Local Cache Invariant:** Any external metrics (such as agent harness quotas, saturation gauges, or remote alerts) must strictly read from pre-warmed local cache files written out-of-band by background jobs or hooks.

### B. Thinking & TUI Spinner Normalization
* AI coding agents stream tokens and rotate braille spinners (`[⠋⠙⠹...][⣾⣽⣻⢿]`).
* **Hard Rule:** Never hash or fingerprint raw terminal buffers without passing them through the sentinel normalizer (`sentinel_<name>_fingerprint`).
* Transient thoughts, elapsed timers, and spinner rotations must be stripped; only meaningful state transitions (e.g. tool execution, prompt waiting, completion) should change the hash.

### C. Dwell-Time Auto-Ack Protection ("The Whiz-Past Invariant")
* Switching into an alerted pane acknowledges and clears its notification (`pane-focus-in` hook).
* When implementing rapid cycling or jump navigation, auto-acknowledgment **must be deferred** (suppressed while cycling until dwell time expires or user types in the pane) so users don't accidentally silence alerts while quickly whizzing past.

### D. Atomic State & Concurrency Locking
* Multiple tmux hooks, status polling cycles, and popup viewers run concurrently.
* Always wrap state modifications (`~/.tmux-glance-*`, `/tmp/tmux-glance-*`) with atomic file locking (`acquire_lock` via `mkdir`) and write to temporary files before atomically moving (`mv`) them into place.

### E. Viewfinder Bottom Pinning (`:follow`)
* When configuring fzf preview windows for terminal outputs, `--preview-window='down:60%:wrap:follow'` is mandatory. Without `:follow`, the viewport snaps to the top or middle of the scrollback rather than locking to the active prompt and build output at the bottom.

---

## 3. Environment & Remote Topology

* **Strictly GitHub-Only:**
  * Unlike Martin's internal dotfiles repository (`nix-home`), `tmux-glance` is a public, standalone open-source project hosted exclusively on GitHub (`git@github.com:mhutchinson/tmux-glance.git`).
  * **Never attempt to push to or configure private Gitea (`nase.aquarium`) remotes for this repository.**
* **Dual OS Support (macOS + Linux):**
  * All scripts and test suites must run seamlessly on Darwin (BSD coreutils, `md5`) and Linux (GNU coreutils, `md5sum`).
  * Use the POSIX-compatible fallback wrappers defined in `bin/tmux-glance` rather than GNU-specific flags (e.g. `sed -i` vs `sed -i ''`).

---

## 4. Quality Gate: Always Run Verification Before Committing

Always execute the local verification suite prior to committing:

```bash
# Run full CI suite: lint (ShellCheck), tests, and Nix flake check
just ci
```

Or execute granularly:
* `just test` — Runs headless tmux server integration, routing, and normalizer test suites.
* `just lint` — Runs `shellcheck` across all shell scripts.
* `just check` — Runs `nix flake check` verifying builds and sandbox tests.
* `just actions` — Verifies GitHub Actions CI workflow runs.

Do not commit or push if any check in `just ci` fails.
