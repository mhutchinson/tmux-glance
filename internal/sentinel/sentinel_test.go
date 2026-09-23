package sentinel

import (
	"context"
	"testing"

	"github.com/mhutchinson/tmux-glance/internal/tmux"
)

func TestResolve_BuiltinCommands(t *testing.T) {
	t.Parallel()
	r, err := New(nil, nil, nil)
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
	r, _ := New(nil, nil, nil)
	// First call.
	got1 := r.Resolve(context.Background(), "agy")
	// Second call — hits commandMap.
	got2 := r.Resolve(context.Background(), "agy")
	if got1 != "antigravity" || got2 != "antigravity" {
		t.Errorf("caching broken: got %q / %q", got1, got2)
	}
}

func TestResolve_DisabledSentinel(t *testing.T) {
	t.Parallel()
	r, _ := New(nil, []string{"antigravity"}, nil)
	got := r.Resolve(context.Background(), "agy")
	if got != "generic" {
		t.Errorf("disabled sentinel should fall through to generic, got %q", got)
	}
}

func TestResolve_RouteOverrides(t *testing.T) {
	t.Parallel()
	r, _ := New(nil, nil, map[string]string{"mycli": "antigravity"})
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

type fakeCustomSentinel struct{}

func (f *fakeCustomSentinel) Name() string                                              { return "custom" }
func (f *fakeCustomSentinel) Matches(p tmux.PaneInfo) bool                              { return p.Command == "my-agent" }
func (f *fakeCustomSentinel) Classify(_ context.Context, _ tmux.PaneInfo, _ string) (Classification, error) {
	return Classification{State: "waiting", Label: "custom alert"}, nil
}
func (f *fakeCustomSentinel) Fingerprint(_ context.Context, _ tmux.PaneInfo, _ string) (string, error) {
	return "custom-hash", nil
}

func TestRegister_CustomSentinel(t *testing.T) {
	t.Parallel()
	r, err := New(nil, nil, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r.Register(&fakeCustomSentinel{})

	got := r.Resolve(context.Background(), "my-agent")
	if got != "custom" {
		t.Errorf("expected custom sentinel to resolve, got %q", got)
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
