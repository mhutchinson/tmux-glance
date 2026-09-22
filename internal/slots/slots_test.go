package slots

import (
	"path/filepath"
	"testing"
)

func tmpStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "slots"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestAll_Empty(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	m, err := s.All()
	if err != nil || len(m) != 0 {
		t.Errorf("want empty map, got %v %v", m, err)
	}
}

func TestAssignAndLookup(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	if err := s.Assign("h", "nix-home"); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	got, err := s.LookupBySlot("h")
	if err != nil || got != "nix-home" {
		t.Errorf("LookupBySlot(h) = %q %v, want nix-home", got, err)
	}
	got, err = s.LookupBySession("nix-home")
	if err != nil || got != "h" {
		t.Errorf("LookupBySession(nix-home) = %q %v, want h", got, err)
	}
}

func TestAssign_CaseInsensitiveSlot(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	s.Assign("H", "main") //nolint:errcheck
	got, _ := s.LookupBySlot("h")
	if got != "main" {
		t.Errorf("want main, got %q", got)
	}
}

func TestAssign_ReplacesOldSlotForSameSession(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	s.Assign("h", "main") //nolint:errcheck
	s.Assign("j", "main") //nolint:errcheck  // same session, new slot
	m, _ := s.All()
	if _, ok := m["h"]; ok {
		t.Errorf("old slot h should have been removed when session moved to j")
	}
	if m["j"] != "main" {
		t.Errorf("want j=main, got %v", m)
	}
}

func TestAssign_ReplacesOldSessionForSameSlot(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	s.Assign("h", "old-session") //nolint:errcheck
	s.Assign("h", "new-session") //nolint:errcheck
	got, _ := s.LookupBySlot("h")
	if got != "new-session" {
		t.Errorf("want new-session, got %q", got)
	}
}

func TestUnassign_BySlot(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	s.Assign("h", "main") //nolint:errcheck
	s.Assign("j", "work") //nolint:errcheck
	s.Unassign("h")       //nolint:errcheck
	m, _ := s.All()
	if _, ok := m["h"]; ok {
		t.Errorf("h should have been removed")
	}
	if m["j"] != "work" {
		t.Errorf("j should still be work")
	}
}

func TestUnassign_BySession(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	s.Assign("h", "nix-home") //nolint:errcheck
	s.Unassign("nix-home")    //nolint:errcheck
	got, _ := s.LookupBySlot("h")
	if got != "" {
		t.Errorf("slot h should be empty after unassign, got %q", got)
	}
}

func TestLookupBySlot_Missing(t *testing.T) {
	t.Parallel()
	s := tmpStore(t)
	got, err := s.LookupBySlot("z")
	if err != nil || got != "" {
		t.Errorf("want empty, got %q %v", got, err)
	}
}
