package sentinel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_BuiltinCommands(t *testing.T) {
	t.Parallel()
	// Build a registry with the real sentinels dir so antigravity is loaded.
	r, err := New([]string{sentinelsDir(t)}, nil, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tests := []struct {
		cmd  string
		want string
	}{
		{"agy", "antigravity"},
		{"antigravity", "antigravity"},
		{"zsh", "generic"},
		{"bash", "generic"},
		{"", "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			t.Parallel()
			got := r.Resolve(context.Background(), tt.cmd)
			if got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestResolve_Caching(t *testing.T) {
	t.Parallel()
	r, _ := New([]string{sentinelsDir(t)}, nil, nil)
	// First call.
	got1 := r.Resolve(context.Background(), "agy")
	// Second call — should hit commandMap without re-execing bash.
	got2 := r.Resolve(context.Background(), "agy")
	if got1 != "antigravity" || got2 != "antigravity" {
		t.Errorf("caching broken: got %q / %q", got1, got2)
	}
}

func TestResolve_DisabledSentinel(t *testing.T) {
	t.Parallel()
	r, _ := New([]string{sentinelsDir(t)}, []string{"antigravity"}, nil)
	got := r.Resolve(context.Background(), "agy")
	if got != "generic" {
		t.Errorf("disabled sentinel should fall through to generic, got %q", got)
	}
}

func TestResolve_RouteOverrides(t *testing.T) {
	t.Parallel()
	r, _ := New([]string{sentinelsDir(t)}, nil, map[string]string{"mycli": "antigravity"})
	got := r.Resolve(context.Background(), "mycli")
	if got != "antigravity" {
		t.Errorf("route override: got %q, want antigravity", got)
	}
}

func TestClassify_Generic(t *testing.T) {
	t.Parallel()
	r, _ := New(nil, nil, nil)
	c, err := r.Classify(context.Background(), "generic", "%1", "/home/user/repo", "zsh")
	if err != nil {
		t.Fatalf("Classify generic: %v", err)
	}
	if c.State != "watching" {
		t.Errorf("want state watching, got %q", c.State)
	}
	if c.Label != "zsh in repo" {
		t.Errorf("want label 'zsh in repo', got %q", c.Label)
	}
}

func TestFingerprint_Generic(t *testing.T) {
	t.Parallel()
	r, _ := New(nil, nil, nil)
	// Inject a fake capture-pane function.
	r.capturePaneFn = func(_ context.Context, _ string) (string, error) {
		return "hello world\nsome output", nil
	}
	hash, err := r.Fingerprint(context.Background(), "generic", "%1")
	if err != nil {
		t.Fatalf("Fingerprint generic: %v", err)
	}
	if len(hash) != 32 {
		t.Errorf("expected 32-char MD5 hex, got %q (len %d)", hash, len(hash))
	}
	// Same content → same hash.
	hash2, _ := r.Fingerprint(context.Background(), "generic", "%1")
	if hash != hash2 {
		t.Errorf("fingerprint not deterministic: %q vs %q", hash, hash2)
	}
}

func TestFingerprint_DifferentContent(t *testing.T) {
	t.Parallel()
	r, _ := New(nil, nil, nil)
	calls := 0
	contents := []string{"output A", "output B"}
	r.capturePaneFn = func(_ context.Context, _ string) (string, error) {
		c := contents[calls%2]
		calls++
		return c, nil
	}
	h1, _ := r.Fingerprint(context.Background(), "generic", "%1")
	h2, _ := r.Fingerprint(context.Background(), "generic", "%1")
	if h1 == h2 {
		t.Errorf("different content should yield different fingerprints")
	}
}

func TestDiscovery_UserDirOverridesBuiltin(t *testing.T) {
	t.Parallel()
	// Create a fake user sentinel dir with an antigravity.sh.
	userDir := t.TempDir()
	fakeScript := filepath.Join(userDir, "antigravity.sh")
	os.WriteFile(fakeScript, []byte("#!/bin/bash\nsentinel_antigravity_matches() { return 0; }"), 0o755) //nolint:errcheck

	r, err := New([]string{userDir, sentinelsDir(t)}, nil, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s := r.find("antigravity")
	if s == nil {
		t.Fatal("antigravity sentinel not found")
	}
	// The user dir version should win.
	if s.ScriptPath != fakeScript {
		t.Errorf("expected user sentinel at %q, got %q", fakeScript, s.ScriptPath)
	}
}

func TestParseRouteOverrides(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  map[string]string
	}{
		{"", map[string]string{}},
		{"agy=antigravity", map[string]string{"agy": "antigravity"}},
		{"agy=antigravity, chat=generic", map[string]string{"agy": "antigravity", "chat": "generic"}},
		{"badpair", map[string]string{}},
		{"a=b,=empty,novalue=", map[string]string{"a": "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := ParseRouteOverrides(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseRouteOverrides(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for k, wantV := range tt.want {
				if got[k] != wantV {
					t.Errorf("key %q: got %q, want %q", k, got[k], wantV)
				}
			}
		})
	}
}

// sentinelsDir returns the repo's bundled sentinels directory.
func sentinelsDir(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find the repo root.
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("finding repo root: %v", err)
	}
	return filepath.Join(dir, "sentinels")
}
