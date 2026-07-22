package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// These tests document and lock in the finalized retired-section decision for the
// single-project outliner: sendTaskToSection routes through the canonical
// Service.SetSection seam (via the single-task coordinator), which writes ONLY
// the GTD section field. The legacy overlay composition — RelocateTaskToEndOfSection
// cosmetic reorder and the Activate-if-empty retired-flag revival — is intentionally
// dropped and must not reappear.

// sendSectionFixture builds a disk-backed single-project outliner wired to the
// single-task coordinator, seeded with three ordered tasks.
func sendSectionFixture(t *testing.T) (*OutlinerModel, *models.Project) {
	t.Helper()
	dir := t.TempDir()
	a, _ := models.NewTask("sec-a", 1, models.TaskTypeTask, "A")
	a.SetSection(models.SectionInbox)
	b, _ := models.NewTask("sec-b", 2, models.TaskTypeTask, "B")
	b.SetSection(models.SectionNext)
	c, _ := models.NewTask("sec-c", 3, models.TaskTypeTask, "C")
	c.SetSection(models.SectionNext)

	proj := &models.Project{ID: "sec-proj", Name: "Sec", Slug: "sec-proj", NextShortID: 4, Path: filepath.Join(dir, "projects", "sec-proj")}
	proj.Tasks = []*models.Task{a, b, c}
	proj.BuildTaskTree()

	m := minimalOutliner()
	m.project = proj
	m.MultiSelectState = MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)}
	m.SetTaskItemCoordinator(newTestSecondaryCoordinators(proj).TaskItem)
	return m, proj
}

func reloadSendSection(t *testing.T, proj *models.Project) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: proj.ID, Name: proj.Name, Slug: proj.Slug, Path: proj.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return fresh
}

// TestOutlinerSendTaskToSection_CanonicalNoRelocate: moving A (first task, Inbox)
// to the Next section changes only its section field and does NOT relocate it to
// the end of the section's tasks — the storage order stays [A, B, C].
func TestOutlinerSendTaskToSection_CanonicalNoRelocate(t *testing.T) {
	m, proj := sendSectionFixture(t)

	_, cmd := m.sendTaskToSection(proj.Tasks[0], models.SectionNext)
	if cmd == nil {
		t.Fatal("expected a coordinator command")
	}
	cmd()

	fresh := reloadSendSection(t, proj)
	got := projectTaskIDs(fresh)
	assertIDsEqual(t, "p.Tasks order preserved (no relocate)", got, []string{"sec-a", "sec-b", "sec-c"})
	if a := fresh.GetTaskByID("sec-a"); a == nil || a.State() != models.StateNext {
		t.Errorf("expected A section=Next, got %+v", a)
	}
}

// TestOutlinerSendTaskToSection_EmptyDoesNotActivate: moving a task to the empty
// (Ready) section must NOT revive the retired Active flag.
func TestOutlinerSendTaskToSection_EmptyDoesNotActivate(t *testing.T) {
	m, proj := sendSectionFixture(t)

	_, cmd := m.sendTaskToSection(proj.Tasks[0], "")
	if cmd == nil {
		t.Fatal("expected a coordinator command")
	}
	cmd()

	fresh := reloadSendSection(t, proj)
	a := fresh.GetTaskByID("sec-a")
	if a == nil {
		t.Fatal("task A missing after section move")
	}
	if models.SectionForState(a.State()) != "" {
		t.Errorf("expected A section cleared, got %q", models.SectionForState(a.State()))
	}
}
