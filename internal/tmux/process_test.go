package tmux

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestInspectProcessTree(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("ps"); err != nil {
		if _, errProc := os.Stat("/proc"); os.IsNotExist(errProc) {
			t.Skip("skipping test: neither ps nor /proc are available in sandboxed environment")
		}
	}
	lines, err := InspectProcessTree(context.Background(), os.Getpid())
	if err != nil {
		t.Fatalf("InspectProcessTree: %v", err)
	}
	if len(lines) == 0 {
		t.Skip("no process lines discovered in sandbox")
	}
	t.Logf("Process lines: %v", lines)

	foundTest := false
	for _, l := range lines {
		if strings.Contains(l, "test") || strings.Contains(l, "tmux") {
			foundTest = true
			break
		}
	}
	if !foundTest {
		t.Errorf("expected command line to contain 'test' or 'tmux', got %v", lines)
	}
}
