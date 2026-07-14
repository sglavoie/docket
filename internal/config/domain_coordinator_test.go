package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// seededManager builds a manager over a fresh temp workspace and persists a
// default config into the four synced domain files, returning the manager and its
// workspace data directory.
func seededManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	// Keep machine.json local to the temp workspace so the test leaves no trace in
	// the real home runtime directory.
	t.Setenv("HOME", tmpDir)
	manager := NewManager(filepath.Join(tmpDir, ".tt.json"))
	cfg, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("initial Load: %v", err)
	}
	if err := saveDomainBundle(manager.domainStore(), cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return manager, tmpDir
}

// TestManagerLoad_PartialDomainStateIsRecoverableError proves that a split-domain
// configuration missing one synced domain fails Load with a recoverable
// PartialDomainStateError naming the gap, rather than silently defaulting the
// vanished domain.
func TestManagerLoad_PartialDomainStateIsRecoverableError(t *testing.T) {
	manager, tmpDir := seededManager(t)

	// A complete set loads cleanly.
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("complete domain set should load: %v", err)
	}

	// Remove one synced domain to create a partial set.
	if err := os.Remove(PlanningStatePath(tmpDir)); err != nil {
		t.Fatalf("remove planning: %v", err)
	}

	_, err := manager.LoadDomains()
	if !errors.Is(err, ErrPartialDomainState) {
		t.Fatalf("expected ErrPartialDomainState, got %v", err)
	}
	var partial *PartialDomainStateError
	if !errors.As(err, &partial) {
		t.Fatalf("expected *PartialDomainStateError, got %T", err)
	}
	if !sliceHasString(partial.Missing, "planning") {
		t.Fatalf("missing domains = %v, want to include planning", partial.Missing)
	}
	if !sliceHasString(partial.Present, "workspace") {
		t.Fatalf("present domains = %v, want to include workspace", partial.Present)
	}
}

// TestManagerLoad_AbsentDomainsFallBackToLegacy confirms that a workspace with no
// synced domain files (and no legacy .tt.json) still loads a fresh default config
// rather than reporting a partial-state error.
func TestManagerLoad_AbsentDomainsFallBackToLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	manager := NewManager(filepath.Join(tmpDir, ".tt.json"))
	cfg, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("absent domains should fall back to a fresh config, got %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected a fresh config")
	}
}

// TestDomainCoordinator_CheckedArtifactsCaptureLoadRevision confirms the
// coordinator stamps each synced artifact with the load-time raw revision, which
// matches the current on-disk hash when the config is unchanged.
func TestDomainCoordinator_CheckedArtifactsCaptureLoadRevision(t *testing.T) {
	manager, _ := seededManager(t)

	coord := manager.DomainCoordinator()
	loaded, err := coord.LoadDomains()
	if err != nil {
		t.Fatalf("coordinator Load: %v", err)
	}

	arts, err := coord.CheckedDomainArtifacts(loaded.Catalog, loaded.Planning, loaded.Settings, loaded.Automation)
	if err != nil {
		t.Fatalf("CheckedArtifacts: %v", err)
	}
	if len(arts) != len(syncedDomainKinds) {
		t.Fatalf("got %d artifacts, want %d", len(arts), len(syncedDomainKinds))
	}
	for _, a := range arts {
		if a.ExpectedRevision == nil {
			t.Fatalf("artifact %s has nil ExpectedRevision; a coordinator must check every synced domain", filepath.Base(a.Path))
		}
		raw, err := os.ReadFile(a.Path)
		if err != nil {
			t.Fatalf("read on-disk artifact %s: %v", filepath.Base(a.Path), err)
		}
		if want := revisionOf(raw); *a.ExpectedRevision != want {
			t.Fatalf("artifact %s ExpectedRevision = %s, want load-time on-disk revision %s", filepath.Base(a.Path), *a.ExpectedRevision, want)
		}
	}
}

// TestDomainCoordinator_AbsentDomainRequiresAbsence confirms that when a synced
// domain does not exist at load time, the coordinator emits a require-absent
// contract (a pointer to the empty string) so a create races safely.
func TestDomainCoordinator_AbsentDomainRequiresAbsence(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	coord := NewDomainCoordinator(tmpDir)
	loaded, err := coord.LoadDomains()
	if err != nil {
		t.Fatalf("coordinator Load on empty workspace: %v", err)
	}
	arts, err := coord.CheckedDomainArtifacts(loaded.Catalog, loaded.Planning, loaded.Settings, loaded.Automation)
	if err != nil {
		t.Fatalf("CheckedArtifacts: %v", err)
	}
	for _, a := range arts {
		if a.ExpectedRevision == nil || *a.ExpectedRevision != "" {
			t.Fatalf("artifact %s ExpectedRevision = %v, want a require-absent pointer to \"\"", filepath.Base(a.Path), a.ExpectedRevision)
		}
	}
}

// TestDomainCoordinator_AdoptArtifactsAdvancesBaseline confirms the coordinator
// advances its optimistic baseline to a committed artifact's new revision so the
// next CheckedArtifacts is validated against the freshly written state.
func TestDomainCoordinator_AdoptArtifactsAdvancesBaseline(t *testing.T) {
	manager, _ := seededManager(t)
	coord := manager.DomainCoordinator()
	loaded, err := coord.LoadDomains()
	if err != nil {
		t.Fatalf("coordinator Load: %v", err)
	}

	first, err := coord.CheckedDomainArtifacts(loaded.Catalog, loaded.Planning, loaded.Settings, loaded.Automation)
	if err != nil {
		t.Fatalf("CheckedArtifacts: %v", err)
	}
	// Simulate committing those bytes and adopting them as the new baseline.
	coord.AdoptArtifacts(first)

	second, err := coord.CheckedDomainArtifacts(loaded.Catalog, loaded.Planning, loaded.Settings, loaded.Automation)
	if err != nil {
		t.Fatalf("CheckedArtifacts second: %v", err)
	}
	byPath := make(map[string][]byte, len(first))
	for _, a := range first {
		byPath[a.Path] = a.Data
	}
	for _, a := range second {
		if a.ExpectedRevision == nil {
			t.Fatalf("artifact %s lost its ExpectedRevision after AdoptArtifacts", filepath.Base(a.Path))
		}
		if want := revisionOf(byPath[a.Path]); *a.ExpectedRevision != want {
			t.Fatalf("artifact %s baseline = %s after adopt, want %s (hash of committed bytes)", filepath.Base(a.Path), *a.ExpectedRevision, want)
		}
	}
}

func sliceHasString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
