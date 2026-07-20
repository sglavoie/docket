package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestSetSectionColorIsSettingsDomainOnly proves the migrated section-color
// action makes the Settings domain the sole owner: the change is recorded and
// persisted in settings.json, every loaded project reflects it only after the
// success message reaches the update loop, and no per-project tasks.md raw save
// happens (the on-disk project markdown keeps its old color).
func TestSetSectionColorIsSettingsDomainOnly(t *testing.T) {
	dir := t.TempDir()

	project, err := models.NewProject("colorproj", "Color Proj", filepath.Join(dir, "projects", "colorproj"))
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.EnsureDefaultSections()
	// Seed a distinct on-disk markdown color so we can detect any raw save.
	project.SetSectionColor(models.SectionNext, "blue")
	if saveErr := storage.SaveProjectLocked(project); saveErr != nil {
		t.Fatalf("seed project: %v", saveErr)
	}
	other, err := models.NewProject("other-colorproj", "Other Color Proj", filepath.Join(dir, "projects", "other-colorproj"))
	if err != nil {
		t.Fatalf("NewProject(other): %v", err)
	}
	other.EnsureDefaultSections()
	other.SetSectionColor(models.SectionNext, "yellow")
	if saveErr := storage.SaveProjectLocked(other); saveErr != nil {
		t.Fatalf("seed other project: %v", saveErr)
	}

	mgr := config.NewManager(filepath.Join(dir, ".tt.json"))
	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project, other})
	if saveErr := saveTestRuntime(mgr, cfg); saveErr != nil {
		t.Fatalf("seed config: %v", saveErr)
	}
	rs := testRuntimeAt(dir, cfg)

	m := NewSectionManagerModel(project, rs, false)
	crossLoaded, err := models.NewProject("cross-loaded", "Cross Loaded", "")
	if err != nil {
		t.Fatalf("NewProject(cross-loaded): %v", err)
	}
	crossLoaded.SetSectionColor(models.SectionNext, "purple")
	app := &AppModel{
		runtime: rs,
		projects: map[string]*models.Project{
			project.ID: project,
			other.ID:   other,
		},
		crossOutliner: &OutlinerModel{loadedProjects: []*models.Project{crossLoaded}},
		viewStack:     []ViewType{ViewSectionManager},
	}

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

	// The checked commit refreshes Settings before emitting success, but project
	// caches remain untouched until the message reaches AppModel.Update.
	if got := rs.Settings().GetGlobalSectionColor(models.SectionNext); got != "green" {
		t.Errorf("in-memory global color = %q, want green", got)
	}
	if got := project.GetSectionColor(models.SectionNext); got != "blue" {
		t.Errorf("selected project color before success handling = %q, want blue", got)
	}
	if got := other.GetSectionColor(models.SectionNext); got != "yellow" {
		t.Errorf("other project color before success handling = %q, want yellow", got)
	}
	if got := crossLoaded.GetSectionColor(models.SectionNext); got != "purple" {
		t.Errorf("cross-project cache color before success handling = %q, want purple", got)
	}

	handled, updateCmd := app.handleSectionMessages(changed)
	if handled {
		t.Fatal("SectionColorChangedMsg should fall through to the open section manager")
	}
	if updateCmd != nil {
		t.Fatal("SectionColorChangedMsg returned an unexpected command")
	}
	if got := project.GetSectionColor(models.SectionNext); got != "green" {
		t.Errorf("selected project color after success handling = %q, want green", got)
	}
	if got := other.GetSectionColor(models.SectionNext); got != "green" {
		t.Errorf("other project color after success handling = %q, want green", got)
	}
	if got := crossLoaded.GetSectionColor(models.SectionNext); got != "green" {
		t.Errorf("cross-project cache color after success handling = %q, want green", got)
	}

	// The global section color is persisted to the Settings domain (the sole
	// durable owner) — reload it from disk to prove it.
	reloaded, err := config.LoadRuntimeState(dir)
	if err != nil {
		t.Fatalf("reload runtime: %v", err)
	}
	if got := reloaded.Settings().GetGlobalSectionColor(models.SectionNext); got != "green" {
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

func TestSetSectionColorFailedCommitLeavesSettingsAndProjectCachesUnchanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tt"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create blocking .tt file: %v", err)
	}

	runtime := newTestConfigAt(dir)
	runtime.Settings().SectionColors = map[string]string{models.SectionNext: "red"}
	selected, err := models.NewProject("selected", "Selected", "")
	if err != nil {
		t.Fatalf("NewProject(selected): %v", err)
	}
	selected.SetSectionColor(models.SectionNext, "blue")
	other, err := models.NewProject("other", "Other", "")
	if err != nil {
		t.Fatalf("NewProject(other): %v", err)
	}
	other.SetSectionColor(models.SectionNext, "yellow")

	m := NewSectionManagerModel(selected, runtime, false)
	msg := m.setSectionColor(models.SectionNext, "green")()
	if _, ok := msg.(ErrorMsg); !ok {
		t.Fatalf("failed commit resolved to %T, want ErrorMsg", msg)
	}
	if got := runtime.Settings().GetGlobalSectionColor(models.SectionNext); got != "red" {
		t.Errorf("settings cache after failure = %q, want red", got)
	}
	if got := selected.GetSectionColor(models.SectionNext); got != "blue" {
		t.Errorf("selected project cache after failure = %q, want blue", got)
	}
	if got := other.GetSectionColor(models.SectionNext); got != "yellow" {
		t.Errorf("other project cache after failure = %q, want yellow", got)
	}
}

func TestSectionColorSettingsCommitDeepClonesMutableFieldsOnFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tt"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create blocking .tt file: %v", err)
	}

	runtime := newTestConfigAt(dir)
	timeout := 5
	showEmpty := true
	grantedAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	settings := runtime.Settings()
	settings.FzfTimeout = &timeout
	settings.Onboarding = &config.OnboardingProgress{}
	settings.GlobalPinnedTags = []string{"home"}
	settings.SectionColors = map[string]string{models.SectionNext: "red"}
	settings.KanbanSettings.EnabledModes = []string{"section"}
	settings.KanbanSettings.SectionColumnOrder = []string{"next"}
	settings.KanbanSettings.StatusColumnOrder = []string{"ready"}
	settings.KanbanSettings.ShowEmptyColumns = &showEmpty
	settings.AIConsent.GrantedAt = &grantedAt

	err := commitSettingsToRuntime(runtime, func(candidate *config.Settings) {
		*candidate.FzfTimeout = 99
		candidate.Onboarding.ProjectCreated = true
		candidate.GlobalPinnedTags[0] = "mutated"
		candidate.SectionColors[models.SectionNext] = "green"
		candidate.KanbanSettings.EnabledModes[0] = "mutated"
		candidate.KanbanSettings.SectionColumnOrder[0] = "mutated"
		candidate.KanbanSettings.StatusColumnOrder[0] = "mutated"
		*candidate.KanbanSettings.ShowEmptyColumns = false
		mutatedGrantedAt := grantedAt.Add(time.Hour)
		candidate.AIConsent.GrantedAt = &mutatedGrantedAt
	})
	if err == nil {
		t.Fatal("commitSettingsToRuntime succeeded with an invalid domain directory")
	}

	if got := *settings.FzfTimeout; got != 5 {
		t.Errorf("FzfTimeout after failure = %d, want 5", got)
	}
	if settings.Onboarding.ProjectCreated {
		t.Error("Onboarding was mutated before the failed commit")
	}
	if got := settings.GlobalPinnedTags[0]; got != "home" {
		t.Errorf("GlobalPinnedTags after failure = %q, want home", got)
	}
	if got := settings.SectionColors[models.SectionNext]; got != "red" {
		t.Errorf("SectionColors after failure = %q, want red", got)
	}
	if got := settings.KanbanSettings.EnabledModes[0]; got != "section" {
		t.Errorf("EnabledModes after failure = %q, want section", got)
	}
	if got := settings.KanbanSettings.SectionColumnOrder[0]; got != "next" {
		t.Errorf("SectionColumnOrder after failure = %q, want next", got)
	}
	if got := settings.KanbanSettings.StatusColumnOrder[0]; got != "ready" {
		t.Errorf("StatusColumnOrder after failure = %q, want ready", got)
	}
	if got := *settings.KanbanSettings.ShowEmptyColumns; !got {
		t.Error("ShowEmptyColumns was mutated before the failed commit")
	}
	if got := *settings.AIConsent.GrantedAt; !got.Equal(grantedAt) {
		t.Errorf("AIConsent.GrantedAt after failure = %s, want %s", got, grantedAt)
	}
}
