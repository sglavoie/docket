package tui

import (
	"maps"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// visibleSectionHeaderRows is the header-row reading of the section axis these
// tests were written against. The Outliner itself no longer takes one: s reads
// the section keys off the rows' own groups through VisibleSections, so that a
// projection drawing no heading still folds. The scan survives here because a
// test that zooms or collapses one named section needs the row that names it.
func visibleSectionHeaderRows(m *outliner.OutlinerModel) []outliner.OutlinerRow {
	headers := make([]outliner.OutlinerRow, 0)
	for _, row := range m.Rows {
		if row.IsSectionHeader {
			headers = append(headers, row)
		}
	}
	return headers
}

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
	outlinerView, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    today.ID,
	}})
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	focus := taskref.TaskIdentity{ProjectID: project.ID, TaskID: doing.ID}
	if !outlinerView.MoveCursorToIdentity(focus) {
		t.Fatal("Doing task is missing before the bulk toggle")
	}
	outlinerView.MultiSelectMode = true
	outlinerView.SelectTaskIdentity(focus)

	parentIdentity := taskref.TaskIdentity{ProjectID: project.ID, TaskID: parent.ID}
	outlinerView.Folds.ExpandedNodes[parentIdentity] = false
	expandedBefore := maps.Clone(outlinerView.Folds.ExpandedNodes)
	// The project fold is named by the group the rows carry, since an expanded
	// group draws no header row to read a collapse key off.
	groups := outlinerView.VisibleProjectGroups()
	if len(groups) == 0 {
		t.Fatal("cross-project All mode is missing a project group")
	}
	projectHeaderKey := groups[0].ProjectKey

	headers := visibleSectionHeaderRows(outlinerView)
	if len(headers) < 4 {
		t.Fatalf("visible section count = %d, want Pinned, Today, and workflow sections", len(headers))
	}
	if !outlinerView.ToggleVisibleSections() {
		t.Fatal("all-expanded sections did not collapse")
	}
	for _, header := range headers {
		if !outlinerView.Folds.CollapsedSections[header.EffectiveCollapseKey()] {
			t.Errorf("section %q did not collapse", header.SectionName)
		}
	}
	for _, row := range outlinerView.Rows {
		if !row.IsSectionHeader && !row.IsSeparator {
			t.Errorf("collapsed view retained non-section row %+v", row)
		}
	}
	if outlinerView.Folds.CollapsedSections[projectHeaderKey] {
		t.Fatal("bulk section toggle collapsed a project subheader")
	}
	if !maps.Equal(outlinerView.Folds.ExpandedNodes, expandedBefore) {
		t.Fatalf("bulk section toggle changed task folds: got %v, want %v", outlinerView.Folds.ExpandedNodes, expandedBefore)
	}
	if outlinerView.SelectedCount() != 1 || !outlinerView.IsTaskIdentitySelected(focus) {
		t.Fatal("bulk section toggle changed the visual task selection")
	}
	if row := outlinerView.Rows[outlinerView.Cursor]; !row.IsSectionHeader || row.StateGroup != models.StateGroupDoing {
		t.Fatalf("collapsed cursor row = %+v, want Doing section header", row)
	}

	if !outlinerView.ToggleVisibleSections() {
		t.Fatal("collapsed sections did not expand")
	}
	for _, header := range visibleSectionHeaderRows(outlinerView) {
		if outlinerView.Folds.CollapsedSections[header.EffectiveCollapseKey()] {
			t.Errorf("section %q remained collapsed", header.SectionName)
		}
	}
	if task, _ := outlinerView.GetTaskRef(); task == nil || task.ID != doing.ID {
		t.Fatalf("expanded cursor task = %v, want %s", task, doing.ID)
	}
	if !maps.Equal(outlinerView.Folds.ExpandedNodes, expandedBefore) {
		t.Fatalf("expansion changed task folds: got %v, want %v", outlinerView.Folds.ExpandedNodes, expandedBefore)
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
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeDue)

	headers := visibleSectionHeaderRows(outlinerView)
	if len(headers) != 2 {
		t.Fatalf("Due section count = %d, want Today and Next Seven Days", len(headers))
	}
	outlinerView.SetSectionRowCollapsed(headers[0], true)
	outlinerView.RebuildRows()
	if !outlinerView.ToggleVisibleSections() {
		t.Fatal("mixed computed sections did not expand")
	}
	for _, header := range visibleSectionHeaderRows(outlinerView) {
		if outlinerView.Folds.CollapsedSections[header.EffectiveCollapseKey()] {
			t.Errorf("computed section %q remained collapsed", header.SectionName)
		}
	}

	plain := outliner.NewOutlinerModel(project, nil, nil)
	plain.SelectOutlinerMode(chrome.ModeInbox)
	if len(visibleSectionHeaderRows(plain)) != 0 {
		t.Fatal("un-pinned Inbox fixture unexpectedly has section headers")
	}
	if plain.ToggleVisibleSections() {
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
	outlinerView, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    regular.ID,
	}})
	outlinerView.SelectOutlinerMode(chrome.ModeToday)

	regularIdentity := taskref.TaskIdentity{ProjectID: project.ID, TaskID: regular.ID}
	if !outlinerView.MoveCursorToIdentity(regularIdentity) {
		t.Fatal("planned Today task is missing before bulk toggle")
	}
	if !outlinerView.ToggleVisibleSections() {
		t.Fatal("Pinned section did not collapse in Today mode")
	}
	if task, _ := outlinerView.GetTaskRef(); task == nil || task.ID != regular.ID {
		t.Fatalf("cursor task after Pinned collapse = %v, want %s", task, regular.ID)
	}
	if outlinerView.Folds.SectionToggleRestore != nil {
		t.Fatal("ungrouped Today task incorrectly received a hidden-task restore anchor")
	}
}

func TestToggleVisibleSectionsDispatchesLowercaseS(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "task")
	app.domain.projects[project.ID] = project
	outlinerView := app.crossOutliner
	outlinerView.Population.Projects = []*models.Project{project}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	if !outlinerView.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: "task"}) {
		t.Fatal("task is missing before s dispatch")
	}

	// Both presses are productive, so both stay silent: the refusal status the
	// handler carries is paid only when the toggle reports no change.
	handled, cmd := app.handleGlobalKeyPress(syntheticKey('s'))
	if !handled || cmd != nil {
		t.Fatalf("s = handled %v, cmd %v; want handled with no command", handled, cmd)
	}
	headers := visibleSectionHeaderRows(outlinerView)
	if len(headers) != 1 || !outlinerView.Folds.CollapsedSections[headers[0].EffectiveCollapseKey()] {
		t.Fatalf("s did not collapse the visible section: headers=%+v collapsed=%v", headers, outlinerView.Folds.CollapsedSections)
	}
	if row := outlinerView.Rows[outlinerView.Cursor]; !row.IsSectionHeader {
		t.Fatalf("cursor after s = %+v, want section header", row)
	}

	handled, cmd = app.handleGlobalKeyPress(syntheticKey('s'))
	if !handled || cmd != nil {
		t.Fatalf("second s = handled %v, cmd %v; want handled with no command", handled, cmd)
	}
	if task, _ := outlinerView.GetTaskRef(); task == nil || task.ID != "task" {
		t.Fatalf("task after second s = %v, want task", task)
	}
}

// A zoom overrides the one section it leaves visible, so s has no fold to write
// and says so. This is the divergence p deliberately does not share: an admitted
// section still emits its project headers, so p there folds rows on screen.
func TestToggleVisibleSectionsReportsInsideASectionZoom(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "task")
	app.domain.projects[project.ID] = project
	outlinerView := app.crossOutliner
	outlinerView.Population.Projects = []*models.Project{project}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()

	headers := visibleSectionHeaderRows(outlinerView)
	if len(headers) != 1 {
		t.Fatalf("section header count = %d, want exactly one to zoom into", len(headers))
	}
	if !outlinerView.EnterSectionZoom(headers[0]) {
		t.Fatal("the section header refused a zoom")
	}
	foldsBefore := maps.Clone(outlinerView.Folds.CollapsedSections)

	handled, cmd := app.handleGlobalKeyPress(syntheticKey('s'))
	if !handled || cmd == nil {
		t.Fatalf("zoomed s = handled %v, cmd %v; want handled with a status", handled, cmd)
	}
	if status, ok := cmd().(status.StatusMsg); !ok || string(status) != "No foldable sections here" {
		t.Fatalf("zoomed s message = %#v, want the refusal status", cmd())
	}
	if !maps.Equal(outlinerView.Folds.CollapsedSections, foldsBefore) {
		t.Fatalf("zoomed s wrote a fold: got %v, want %v", outlinerView.Folds.CollapsedSections, foldsBefore)
	}
}
