# Agent Guidelines & Documentation Map: tmux-glance

> **Operating Contract for AI Agents**: This document serves as a high-level distillation and navigation index for autonomous coding agents contributing to `tmux-glance`.
> Canonical architecture, philosophy, and specifications live in [DESIGN.md](DESIGN.md). Always consult canonical docs before proposing or implementing changes.

---

## 1. Documentation Sitemap & Reading Order

| Document | Purpose | What to Look For |
| :--- | :--- | :--- |
| **[README.md](README.md)** | User-facing documentation | Problem statement, feature demos, installation methods (Nix Flake, Home Manager, TPM, manual), and the [Milestones & Roadmap](README.md#%EF%B8%8F-milestones--roadmap). |
| **[DESIGN.md](DESIGN.md)** | Canonical architecture & design | Component data flow, [Core Philosophy & Invariants](DESIGN.md#2-core-philosophy--architectural-invariants), [Tiered Keybindings](DESIGN.md#6-ergonomics--tiered-keybinding-architecture), state locking, and deep-dive technical specs. |
| **[Justfile](Justfile)** | Local workflow automation | Standard recipes for testing (`just test`), linting (`just lint`), building (`just build`), full verification (`just ci`), and GitHub management. |
| **[`sentinels/`](sentinels/)** | Modular agent classifiers | The pluggable sentinel contract: `sentinel_<name>_classify` and `sentinel_<name>_fingerprint`. |

---

## 2. Fast Invariant Checklist

Before editing code or writing tests, verify your plan against these hard architectural invariants (detailed in [DESIGN.md §2](DESIGN.md#2-core-philosophy--architectural-invariants)):

* [ ] **Minimally Invasive by Default:** Only Tier 1 non-disruptive keys (`prefix g`, `prefix b`, `prefix v`) are registered by default. Any direct-jump chords, jumplist rewinds, or quickfix cycling MUST be Tier 2 opt-ins (`@glance_enable_*` / HM options). Never hijack built-in tmux keys (`prefix [` copy-mode, `c`, `z`, `n`, `p`).
* [ ] **Sub-50ms Status Budget:** `tmux-glance status` runs on `status-interval` (<50ms budget). Zero network requests or heavy subshell pipelines. External metrics (e.g. quota gauges) must strictly read pre-warmed local cache files written out-of-band.
* [ ] **Thinking & Spinner Normalization:** Terminal buffers must pass through `sentinel_<name>_fingerprint` to strip animated braille spinners (`[⠋⠙⠹...]`) and streaming thinking lines before computing hashes. Thinking is not an unread alert.
* [ ] **Dwell-Time Auto-Ack Protection:** Rapid navigation across panes must suppress `pane-focus-in` auto-acknowledgment until dwell time expires, avoiding accidental alert dismissal when whizzing past.
* [ ] **Atomic Concurrency:** All state modifications must use directory locking (`acquire_lock` via `mkdir`) and atomic file moves (`mv`).
* [ ] **Viewfinder Pinning:** Live preview windows must specify `--preview-window='down:60%:wrap:follow'` to remain locked to the bottom line (active prompt / compiler output).
* [ ] **Strictly GitHub-Only:** Hosted exclusively at `github.com/mhutchinson/tmux-glance`. Never configure or push to internal Gitea (`nase.aquarium`) remotes.
* [ ] **POSIX & Dual-OS Portability:** Scripts must run portably on macOS (BSD `md5`) and Linux (GNU `md5sum`).

---

## 3. Pre-Commit Quality Gate

Always run the full test and verification suite before staging or committing changes:

```bash
# Run ShellCheck, integration test suite, and Nix sandbox flake check:
just ci
```

Never bypass failing tests or commit without running `just ci`.
