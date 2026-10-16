package sentinel

import (
	"testing"

	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

func TestGeneric(t *testing.T) {
	t.Parallel()

	g := NewGeneric()

	if g == nil {
		t.Fatal("NewGeneric() returned nil")
	}

	if name := g.Name(); name != "generic" {
		t.Errorf("Name() = %q, want %q", name, "generic")
	}

	pane := tmux.PaneInfo{
		ID:      "%1",
		Command: "zsh",
		Path:    "/tmp",
	}

	if matches := g.Matches(pane); !matches {
		t.Errorf("Matches() = %v, want %v", matches, true)
	}
}
