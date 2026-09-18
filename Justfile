set shell := ["bash", "-uc"]

# Show available recipes
default:
    @just --list

# Run unit and integration test suite
test:
    bash tests/run_tests.sh

# Run Nix flake checks (builds package and runs sandboxed tests)
check:
    nix flake check

# Build the tmux-glance package via Nix
build:
    nix build .#tmux-glance

# Run ShellCheck across all scripts
lint:
    @if command -v shellcheck >/dev/null 2>&1; then \
        shellcheck bin/tmux-glance sentinels/*.sh glance.tmux tests/*.sh; \
    else \
        nix run nixpkgs#shellcheck -- bin/tmux-glance sentinels/*.sh glance.tmux tests/*.sh; \
    fi

# Run all local CI verification steps (lint, test, check)
ci: lint test check

# List open GitHub pull requests
gh-prs:
    gh pr list

# List open GitHub issues
gh-issues:
    gh issue list

# View latest GitHub Actions runs
actions:
    gh run list
