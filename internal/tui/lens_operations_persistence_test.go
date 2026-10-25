package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/lens"
	"github.com/sglavoie/docket/internal/tui/operation"
)

// With no writable workspace rooted, the in-memory seam is the whole commit and
// the runtime accessors must observe it exactly as they would after a write.
func TestLensOperationsUseTheInMemorySeamWithoutADataDir(t *testing.T) {
	model := lensOperationApp(t)
	if model.domain.runtime.DataDir() != "" {
		t.Fatalf("fixture rooted a data dir at %q", model.domain.runtime.DataDir())
	}
	before := model.domain.runtime.Machine()

	if result := executeLens(model, lens.OperationLensCreate, operation.EntityIdentity{}, "In memory", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}
	after := model.domain.runtime.Machine()
	if after == before {
		t.Fatal("the in-memory seam must publish the clone, not mutate the cached snapshot")
	}
	if before.NamedLensByID(lensNamed(t, model, "In memory").ID) != nil {
		t.Fatal("the pre-commit snapshot was mutated in place")
	}
	if after.Theme != before.Theme {
		t.Fatalf("the clone dropped the theme: %q vs %q", after.Theme, before.Theme)
	}
}

// The disk path is the production one: the record must survive a fresh load of
// the same workspace's machine-local file.
func TestLensCreatePersistsToTheMachineLocalFile(t *testing.T) {
	model := lensOperationApp(t)
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.domain.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	// testRuntimeAt seeds the catalog alone; a reload reads the split-domain set
	// whole, so the planning file has to exist beside it.
	if err := manager.CommitPlanning(config.NewPlanningState()); err != nil {
		t.Fatalf("seed planning: %v", err)
	}

	if result := executeLens(model, lens.OperationLensCreate, operation.EntityIdentity{}, "Persisted", false); result.Err != nil {
		t.Fatalf("create lens: %v", result.Err)
	}

	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatalf("reload domains: %v", err)
	}
	if domains.Machine == nil {
		t.Fatal("reloaded domains carry no machine state")
	}
	if lensNames(domains.Machine.NamedLenses) == nil ||
		!strings.Contains(strings.Join(lensNames(domains.Machine.NamedLenses), ","), "Persisted") {
		t.Fatalf("reloaded lenses = %v, want the saved record", lensNames(domains.Machine.NamedLenses))
	}
}

// A commit that fails on disk must leave the cached snapshot and the active
// lens exactly as they were: the effect is published only after success.
func TestAFailedLensCommitChangesNeitherMachineStateNorTheActiveLens(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.domain.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))

	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	// A read-only runtime root is the one failure the commit cannot route around.
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Skipf("cannot make the runtime root read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	before := model.domain.runtime.Machine()
	result := executeLens(model, lens.OperationLensRename, operation.LensEntity(starter.ID), "Renamed", false)
	if result.Err == nil {
		t.Skip("the runtime root stayed writable; the failure path is untested here")
	}
	if model.domain.runtime.Machine() != before {
		t.Fatal("a failed commit replaced the cached machine snapshot")
	}
	if before.NamedLensByID(starter.ID).Name != starter.Name {
		t.Fatal("a failed commit rewrote the cached definition in place")
	}
	if out.Lens.Active == nil || out.Lens.Active.Name != starter.Name {
		t.Fatal("a failed commit relabeled the active lens")
	}
}

// executeLensValue is the typed-value form of executeLens: the template picker
