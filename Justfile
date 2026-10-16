set shell := ["bash", "-uc"]

# Show available recipes
default:
    @just --list

# Run unit and integration test suite
test: go-build
    env -u TMUX -u TMUX_PANE bash tests/run_tests.sh

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
    @tmux set-environment -g PATH "$PWD/bin:$(tmux show-environment -g PATH 2>/dev/null | sed 's/^PATH=//' || echo "$$PATH")"
    @tmux bind-key s display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list-sessions"
    @tmux bind-key b display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list hub"
    @tmux bind-key g display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list-all"
    @tmux bind-key v run-shell "$PWD/bin/tmux-glance toggle-vigil"
    @tmux bind-key Tab display-popup -E -w 85% -h 75% "$PWD/bin/tmux-glance list-history"
    @tmux bind-key -r C-h run-shell "$PWD/bin/tmux-glance jump-slot h"
    @tmux bind-key -r C-j run-shell "$PWD/bin/tmux-glance jump-slot j"
    @tmux bind-key -r C-k run-shell "$PWD/bin/tmux-glance jump-slot k"
    @tmux bind-key -r C-l run-shell "$PWD/bin/tmux-glance jump-slot l"
    @tmux bind-key -r '<' run-shell "$PWD/bin/tmux-glance jump-back"
    @tmux bind-key -r '>' run-shell "$PWD/bin/tmux-glance jump-forward"
    @tmux bind-key -r '}' run-shell "$PWD/bin/tmux-glance next-attention"
    @tmux bind-key -r '{' run-shell "$PWD/bin/tmux-glance prev-attention"
    @tmux set-hook -g pane-focus-in "run-shell -b '$PWD/bin/tmux-glance on-focus #{pane_id}'"
    @tmux refresh-client -S
    @echo "⚡ Live dev mode active: running tmux bindings and status now point directly to $PWD/bin/tmux-glance"


# Run linters (ShellCheck and golangci-lint)
lint: lint-sh lint-go

# Run ShellCheck across shell scripts
lint-sh:
    @if command -v shellcheck >/dev/null 2>&1; then \
        shellcheck bin/tmux-glance glance.tmux tests/*.sh; \
    else \
        nix run nixpkgs#shellcheck -- bin/tmux-glance glance.tmux tests/*.sh; \
    fi

# Run golangci-lint across Go packages
lint-go:
    @if command -v golangci-lint >/dev/null 2>&1; then \
        golangci-lint run; \
    else \
        nix run nixpkgs#golangci-lint -- run; \
    fi

# Run Go vulnerability check
vulncheck:
    @if command -v govulncheck >/dev/null 2>&1; then \
        govulncheck ./...; \
    else \
        nix run nixpkgs#govulncheck -- ./...; \
    fi

# Run all local CI verification steps (lint, vulncheck, go-test, test, check)
ci: lint vulncheck go-test test check

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

# Bump tmux-glance input in ~/nix-home and deploy the updated configuration via home-manager
deploy-nix-home:
    @if [ ! -d "$HOME/nix-home" ]; then \
        echo "Error: ~/nix-home directory not found"; \
        exit 1; \
    fi
    @echo "📦 Updating tmux-glance input in ~/nix-home..."
    @nix flake update tmux-glance --flake "$HOME/nix-home"
    @echo "🚀 Deploying home-manager configuration on macOS..."
    @nix run github:nix-community/home-manager -- switch --flake "$HOME/nix-home#mhutchinson-mac" -b hm-backup
    @echo "✅ Successfully deployed updated tmux-glance to nix-home!"


