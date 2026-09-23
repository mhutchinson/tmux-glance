package sentinel

import (
	"context"
	"crypto/md5"
	"fmt"
	"path/filepath"

	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

// Generic is the fallback sentinel for standard shells, compilation jobs, tests, and watches.
type Generic struct{}

// NewGeneric returns a new Generic fallback sentinel.
func NewGeneric() *Generic {
	return &Generic{}
}

func (g *Generic) Name() string {
	return "generic"
}

func (g *Generic) Matches(_ tmux.PaneInfo) bool {
	return true
}

func (g *Generic) Classify(_ context.Context, pane tmux.PaneInfo, _ string) (Classification, error) {
	return Classification{
		State: "watching",
		Label: fmt.Sprintf("%s in %s", pane.Command, filepath.Base(pane.Path)),
	}, nil
}

func (g *Generic) Fingerprint(_ context.Context, _ tmux.PaneInfo, buffer string) (string, error) {
	sum := md5.Sum([]byte(buffer))
	return fmt.Sprintf("%x", sum), nil
}
