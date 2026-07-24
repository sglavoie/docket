package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// visibleOutlinerTaskIDs collects the task IDs currently rendered as task rows,
// so a filter/scope assertion can confirm exactly which tasks survived.
func visibleOutlinerTaskIDs(m *OutlinerModel) map[string]bool {
	ids := make(map[string]bool)
	for _, row := range m.rows {
		if row.task != nil {
			ids[row.task.ID] = true
		}
	}
	return ids
}

// TestTagAndSectionWorkflowsUseCanonicalOutliner locks FR13-001: global-tag
// navigation, palette section navigation, section-filter completion, and palette
// task actions must all operate on the single authoritative cross Outliner
// (project selection is a scope frame) rather than constructing or targeting a
// separate AppModel.outliner root. The visible model and the palette's target can
// therefore never diverge.
func TestTagAndSectionWorkflowsUseCanonicalOutliner(t *testing.T) {
	t.Run("tag navigation scopes the cross Outliner and filters", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-tag", "tag", "Alpha", "Bravo", "Charlie")
		tasks[0].Tags = []string{"focus"}
		tasks[1].Tags = []string{"later"}
		tasks[2].Tags = []string{"focus"}
		app, cross, _ := newPlanningOrderApp(t, project)
		app.pushView(ViewGlobalTagManager)

		handled, _ := app.handleTagMessages(GlobalTagNavigateMsg{Project: project, Tag: "focus"})
		if !handled {
			t.Fatal("GlobalTagNavigateMsg should be handled")
		}
		if app.outliner != nil {
			t.Fatal("tag navigation must not construct a separate AppModel.outliner root")
		}
		if app.baseProjectView() != ViewCrossProjectOutliner {
			t.Fatalf("base view = %s, want the single %s root", app.baseProjectView(), ViewCrossProjectOutliner)
		}
		if cross.scopedProjectID != project.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, project.ID)
		}
		if !cross.filter.Tags["focus"] {
			t.Fatalf("cross filter tags = %v, want focus", cross.filter.Tags)
		}
		visible := visibleOutlinerTaskIDs(cross)
		if !visible[tasks[0].ID] || !visible[tasks[2].ID] {
			t.Fatalf("expected focus-tagged tasks visible, got %v", visible)
		}
		if visible[tasks[1].ID] {
			t.Fatalf("expected non-matching task hidden, got %v", visible)
		}
	})

	t.Run("section navigation scopes the cross Outliner and filters", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-sec", "sec", "Alpha", "Bravo", "Charlie")
		tasks[1].SetSection(models.SectionInbox)
		app, cross, _ := newPlanningOrderApp(t, project)
		app.pushView(ViewCommandPalette)

		handled, _ := app.handleSectionMessages(CommandPaletteSectionSelectMsg{Project: project, Section: models.SectionInbox})
		if !handled {
			t.Fatal("CommandPaletteSectionSelectMsg should be handled")
		}
		if app.outliner != nil {
			t.Fatal("section navigation must not construct a separate AppModel.outliner root")
		}
		if app.baseProjectView() != ViewCrossProjectOutliner {
			t.Fatalf("base view = %s, want the single %s root", app.baseProjectView(), ViewCrossProjectOutliner)
		}
		if cross.scopedProjectID != project.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, project.ID)
		}
		if !cross.filter.Sections[models.SectionInbox] {
			t.Fatalf("cross filter sections = %v, want inbox", cross.filter.Sections)
		}
		visible := visibleOutlinerTaskIDs(cross)
		if !visible[tasks[1].ID] {
			t.Fatalf("expected inbox task visible, got %v", visible)
		}
		if visible[tasks[0].ID] || visible[tasks[2].ID] {
			t.Fatalf("expected non-inbox tasks hidden, got %v", visible)
		}
	})

	t.Run("section filter completion narrows the scoped cross Outliner", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-done", "done", "Alpha", "Bravo", "Charlie")
		tasks[2].SetSection(models.SectionInbox)
		app, cross, _ := newPlanningOrderApp(t, project)

		// Enter the project scope on the canonical cross Outliner, then open and
		// complete a section filter from it.
		app.enterProjectScopeOutliner(project, nil, OutlinerOpenOptions{})
		app.pushView(ViewSectionFilterSelector)

		handled, _ := app.handleSectionMessages(SectionFilterSelectorDoneMsg{SelectedSections: []string{models.SectionInbox}})
		if !handled {
			t.Fatal("SectionFilterSelectorDoneMsg should be handled")
		}
		if app.outliner != nil {
			t.Fatal("section filter completion must not construct a separate AppModel.outliner root")
		}
		if !cross.filter.Sections[models.SectionInbox] {
			t.Fatalf("cross filter sections = %v, want inbox applied to the visible model", cross.filter.Sections)
		}
		visible := visibleOutlinerTaskIDs(cross)
		if !visible[tasks[2].ID] {
			t.Fatalf("expected inbox task visible after completion, got %v", visible)
		}
		if visible[tasks[0].ID] || visible[tasks[1].ID] {
			t.Fatalf("expected non-inbox tasks hidden after completion, got %v", visible)
		}
	})

	t.Run("palette task action resolves the scoped cross Outliner task", func(t *testing.T) {
		project, tasks := planningOrderProject(t, t.TempDir(), "p-act", "act", "Alpha", "Bravo", "Charlie")
		app, _, _ := newPlanningOrderApp(t, project)

		app.enterProjectScopeOutliner(project, tasks[1], OutlinerOpenOptions{})
		if app.outliner != nil {
			t.Fatal("scope navigation must not construct a separate AppModel.outliner root")
		}

		task, proj := app.paletteTaskTarget()
		if task == nil || proj == nil {
			t.Fatal("palette task action resolved no task from the scoped cross Outliner")
		}
		if task.ID != tasks[1].ID {
			t.Fatalf("palette resolved task %q, want the visible scoped task %q", task.ID, tasks[1].ID)
		}
		if proj.ID != project.ID {
			t.Fatalf("palette resolved project %q, want %q", proj.ID, project.ID)
		}
	})
}
