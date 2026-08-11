package tui

import (
	"maps"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestToggleVisibleSectionsCollapsesBuiltInsAndRestoresTaskFocus(t *testing.T) {
	now := time.Now()
	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Pinned = true
	pinned.PinnedAt = &now
	today := primaryRowsTask("today", models.StateBacklog)
	doing := primaryRowsTask("doing", models.StateDoing)
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	project := primaryRowsProject("alpha", pinned, today, doing, parent, child)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    today.ID,
	}})
	outliner.selectOutlinerMode(ModeAll)

	focus := TaskIdentity{ProjectID: project.ID, TaskID: doing.ID}
	if !outliner.moveCursorToIdentity(focus) {
		t.Fatal("Doing task is missing before the bulk toggle")
	}
	outliner.multiSelectMode = true
	outliner.SelectTaskIdentity(focus)

	parentIdentity := TaskIdentity{ProjectID: project.ID, TaskID: parent.ID}
	outliner.expandedNodes[parentIdentity] = false
	expandedBefore := maps.Clone(outliner.expandedNodes)
	projectHeaderKey := ""
	for _, row := range outliner.rows {
		if row.IsProjectHeader {
			projectHeaderKey = row.effectiveCollapseKey()
			break
		}
	}
	if projectHeaderKey == "" {
		t.Fatal("cross-project All mode is missing a project header")
	}

	headers := outliner.visibleSectionHeaders()
	if len(headers) < 4 {
		t.Fatalf("visible section count = %d, want Pinned, Today, and workflow sections", len(headers))
	}
	if !outliner.toggleVisibleSections() {
		t.Fatal("all-expanded sections did not collapse")
	}
	for _, header := range headers {
		if !outliner.collapsedSections[header.effectiveCollapseKey()] {
			t.Errorf("section %q did not collapse", header.SectionName)
		}
	}
	for _, row := range outliner.rows {
		if !row.IsSectionHeader && !row.IsSeparator {
			t.Errorf("collapsed view retained non-section row %+v", row)
		}
	}
	if outliner.collapsedSections[projectHeaderKey] {
		t.Fatal("bulk section toggle collapsed a project subheader")
	}
	if !maps.Equal(outliner.expandedNodes, expandedBefore) {
		t.Fatalf("bulk section toggle changed task folds: got %v, want %v", outliner.expandedNodes, expandedBefore)
	}
	if outliner.SelectedCount() != 1 || !outliner.IsTaskIdentitySelected(focus) {
		t.Fatal("bulk section toggle changed the visual task selection")
	}
	if row := outliner.rows[outliner.cursor]; !row.IsSectionHeader || row.StateGroup != models.StateGroupDoing {
		t.Fatalf("collapsed cursor row = %+v, want Doing section header", row)
	}

	if !outliner.toggleVisibleSections() {
		t.Fatal("collapsed sections did not expand")
	}
	for _, header := range outliner.visibleSectionHeaders() {
		if outliner.collapsedSections[header.effectiveCollapseKey()] {
			t.Errorf("section %q remained collapsed", header.SectionName)
		}
	}
	if task, _ := outliner.getTaskRef(); task == nil || task.ID != doing.ID {
		t.Fatalf("expanded cursor task = %v, want %s", task, doing.ID)
	}
	if !maps.Equal(outliner.expandedNodes, expandedBefore) {
		t.Fatalf("expansion changed task folds: got %v, want %v", outliner.expandedNodes, expandedBefore)
	}
}

func TestToggleVisibleSectionsExpandsMixedAndComputedModes(t *testing.T) {
	now := time.Now()
	today := primaryRowsTask("today", models.StateBacklog)
	today.DueDate = &now
	later := primaryRowsTask("later", models.StateBacklog)
	laterDue := now.AddDate(0, 0, 2)
	later.DueDate = &laterDue
	project := primaryRowsProject("alpha", today, later)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeDue)

	headers := outliner.visibleSectionHeaders()
	if len(headers) != 2 {
		t.Fatalf("Due section count = %d, want Today and Next Seven Days", len(headers))
	}
	outliner.setSectionRowCollapsed(headers[0], true)
	outliner.rebuildRows()
	if !outliner.toggleVisibleSections() {
		t.Fatal("mixed computed sections did not expand")
	}
	for _, header := range outliner.visibleSectionHeaders() {
		if outliner.collapsedSections[header.effectiveCollapseKey()] {
			t.Errorf("computed section %q remained collapsed", header.SectionName)
		}
	}

	plain := NewOutlinerModel(project, nil, nil)
	plain.selectOutlinerMode(ModeInbox)
	if len(plain.visibleSectionHeaders()) != 0 {
		t.Fatal("un-pinned Inbox fixture unexpectedly has section headers")
	}
	if plain.toggleVisibleSections() {
		t.Fatal("section-less view reported a bulk toggle")
	}
}

func TestToggleVisibleSectionsKeepsUnsectionedPrimaryTaskFocused(t *testing.T) {
	now := time.Now()
	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Pinned = true
	pinned.PinnedAt = &now
	regular := primaryRowsTask("regular", models.StateBacklog)
	project := primaryRowsProject("alpha", pinned, regular)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    regular.ID,
	}})
	outliner.selectOutlinerMode(ModeToday)

	regularIdentity := TaskIdentity{ProjectID: project.ID, TaskID: regular.ID}
	if !outliner.moveCursorToIdentity(regularIdentity) {
		t.Fatal("planned Today task is missing before bulk toggle")
	}
	if !outliner.toggleVisibleSections() {
		t.Fatal("Pinned section did not collapse in Today mode")
	}
	if task, _ := outliner.getTaskRef(); task == nil || task.ID != regular.ID {
		t.Fatalf("cursor task after Pinned collapse = %v, want %s", task, regular.ID)
	}
	if outliner.sectionToggleRestore != nil {
		t.Fatal("ungrouped Today task incorrectly received a hidden-task restore anchor")
	}
}

func TestToggleVisibleSectionsDispatchesUppercaseS(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "task")
	app.projects[project.ID] = project
	outliner := app.crossOutliner
	outliner.loadedProjects = []*models.Project{project}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if !outliner.moveCursorToIdentity(TaskIdentity{ProjectID: project.ID, TaskID: "task"}) {
		t.Fatal("task is missing before S dispatch")
	}

	handled, cmd := app.handleGlobalKeyPress(syntheticKey('S'))
	if !handled || cmd != nil {
		t.Fatalf("S = handled %v, cmd %v; want handled with no command", handled, cmd)
	}
	headers := outliner.visibleSectionHeaders()
	if len(headers) != 1 || !outliner.collapsedSections[headers[0].effectiveCollapseKey()] {
		t.Fatalf("S did not collapse the visible section: headers=%+v collapsed=%v", headers, outliner.collapsedSections)
	}
	if row := outliner.rows[outliner.cursor]; !row.IsSectionHeader {
		t.Fatalf("cursor after S = %+v, want section header", row)
	}

	handled, cmd = app.handleGlobalKeyPress(syntheticKey('S'))
	if !handled || cmd != nil {
		t.Fatalf("second S = handled %v, cmd %v; want handled with no command", handled, cmd)
	}
	if task, _ := outliner.getTaskRef(); task == nil || task.ID != "task" {
		t.Fatalf("task after second S = %v, want task", task)
	}
}
