package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestSetSectionColorIsConfigOnly proves the migrated section-color action makes
// the global config the sole owner: the change is recorded and persisted in the
// config, open in-memory projects reflect it immediately, and no per-project
// tasks.md raw save happens (the on-disk project markdown keeps its old color).
func TestSetSectionColorIsConfigOnly(t *testing.T) {
	dir := t.TempDir()

	project, err := models.NewProject("colorproj", "Color Proj", filepath.Join(dir, "projects", "colorproj"))
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.EnsureDefaultSections()
	// Seed a distinct on-disk markdown color so we can detect any raw save.
	project.SetSectionColor(models.SectionNext, "blue")
	if err := storage.SaveProjectLocked(project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	mgr := config.NewManager(filepath.Join(dir, ".tt.json"))
	cfg := testutil.NewTestConfig()
	cfg.Projects = []*models.Project{project}
	if err := mgr.Save(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	m := NewSectionManagerModel(nil, project, cfg, mgr, false)

	cmd := m.setSectionColor(models.SectionNext, "green")
	if cmd == nil {
		t.Fatal("setSectionColor returned a nil command")
	}
	msg := cmd()
	changed, ok := msg.(SectionColorChangedMsg)
	if !ok {
		t.Fatalf("expected SectionColorChangedMsg, got %T (%v)", msg, msg)
	}
	if changed.SectionName != models.SectionNext || changed.ColorName != "green" {
		t.Fatalf("unexpected change message: %+v", changed)
	}

	// In-memory config + project reflect the change immediately.
	if got := cfg.GetGlobalSectionColor(models.SectionNext); got != "green" {
		t.Errorf("in-memory global color = %q, want green", got)
	}
	if got := project.GetSectionColor(models.SectionNext); got != "green" {
		t.Errorf("in-memory project color = %q, want green", got)
	}

	// The global config is persisted (config is the sole durable owner).
	reloadedCfg, err := mgr.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := reloadedCfg.GetGlobalSectionColor(models.SectionNext); got != "green" {
		t.Errorf("persisted global color = %q, want green", got)
	}

	// The project markdown was NOT rewritten: a fresh disk load keeps the old
	// color, proving no per-project raw save occurred.
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if got := fresh.GetSectionColor(models.SectionNext); got != "blue" {
		t.Errorf("on-disk project color = %q, want the untouched blue (no raw save expected)", got)
	}
}
