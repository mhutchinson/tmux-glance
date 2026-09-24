set shell := ["bash", "-uc"]

# Show available recipes
default:
    @just --list

# Run unit and integration test suite
test: go-build
    bash tests/run_tests.sh

# Build the Go engine binary into bin/
go-build:
    go build -o bin/glance-engine ./cmd/glance-engine

# Run Go unit tests with race detector
go-test:
    go test -race ./...

# Run Nix flake checks (builds package and runs sandboxed tests)
check:
    nix flake check

# Build the tmux-glance package via Nix
build:
    nix build .#tmux-glance


# Wire running tmux directly to this working tree for instant zero-rebuild live development
live: go-build
    @if ! tmux info >/dev/null 2>&1; then \
        echo "Error: tmux server is not running"; \
        exit 1; \
    fi
    @tmux bind-key s display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list-sessions"
    @tmux bind-key b display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list bots"
    @tmux bind-key g display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list-all"
    @tmux bind-key v run-shell "$PWD/bin/tmux-glance toggle-vigil"
    @tmux bind-key Tab display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list-history"
    @tmux bind-key -r C-h run-shell "$PWD/bin/tmux-glance jump-slot h"
    @tmux bind-key -r C-j run-shell "$PWD/bin/tmux-glance jump-slot j"
    @tmux bind-key -r C-k run-shell "$PWD/bin/tmux-glance jump-slot k"
    @tmux bind-key -r C-l run-shell "$PWD/bin/tmux-glance jump-slot l"
    @tmux bind-key -r '<' run-shell "$PWD/bin/tmux-glance jump-back"
    @tmux bind-key -r '>' run-shell "$PWD/bin/tmux-glance jump-forward"
    @tmux bind-key -r ']' run-shell "$PWD/bin/tmux-glance next-attention"
    @tmux bind-key -r '[' run-shell "$PWD/bin/tmux-glance prev-attention"
    @echo "⚡ Live dev mode active: running tmux bindings now point directly to $PWD/bin/tmux-glance"


# Run ShellCheck across all scripts
lint:
    @if command -v shellcheck >/dev/null 2>&1; then \
        shellcheck bin/tmux-glance glance.tmux tests/*.sh; \
    else \
        nix run nixpkgs#shellcheck -- bin/tmux-glance glance.tmux tests/*.sh; \
    fi

# Run all local CI verification steps (lint, go-test, test, check)
ci: lint go-test test check

# List open GitHub pull requests
gh-prs:
    gh pr list

# List open GitHub issues
gh-issues:
    gh issue list

# View latest GitHub Actions runs
actions:
    gh run list

# Record the animated demo GIF using VHS
record-demo:
    vhs scripts/demo.tape

