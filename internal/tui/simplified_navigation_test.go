package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	appservice "github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func findTaskRow(rows []outliner.OutlinerRow, taskID string) int {
	for index, row := range rows {
		if row.Task != nil && row.Task.ID == taskID {
			return index
		}
	}
	return -1
}

func simplifiedTreeOutliner(t *testing.T) (*outliner.OutlinerModel, *models.Project, *models.Task, *models.Task) {
	t.Helper()
	parent := &models.Task{
		ID:            "parent",
		ShortID:       1,
		Title:         "Parent",
		WorkflowState: models.StateBacklog,
	}
	child := &models.Task{
		ID:            "child",
		ShortID:       2,
		Title:         "Child",
		WorkflowState: models.StateBacklog,
		ParentID:      &parent.ID,
	}
	project := &models.Project{
		ID:    "project",
		Name:  "Project",
		Tasks: []*models.Task{parent, child},
	}
	project.BuildTaskTree()
	cfg := newTestConfig()
	m := outliner.NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SelectOutlinerMode(chrome.ModeAll)
	return m, project, parent, child
}

func TestSimplifiedHorizontalNavigationCollapsesExpandsAndMovesToParent(t *testing.T) {
	m, _, parent, child := simplifiedTreeOutliner(t)
	parentRow := findTaskRow(m.Rows, parent.ID)
	childRow := findTaskRow(m.Rows, child.ID)
	if parentRow < 0 || childRow < 0 || !m.Rows[parentRow].HasVisibleChildren {
		t.Fatalf("tree rows not rendered as parent/child: parent=%d child=%d rows=%+v", parentRow, childRow, m.Rows)
	}

	m.Cursor = parentRow
	if !m.CollapseOrMoveToParent() {
		t.Fatal("h did not collapse the expanded parent")
	}
	if got := findTaskRow(m.Rows, child.ID); got >= 0 {
		t.Fatalf("child remains visible at row %d after collapse", got)
	}

	if !m.ExpandAtCursor() {
		t.Fatal("l did not expand the collapsed parent")
	}
	childRow = findTaskRow(m.Rows, child.ID)
	if childRow < 0 {
		t.Fatal("child did not become visible after expand")
	}
	m.Cursor = childRow
	if !m.CollapseOrMoveToParent() {
		t.Fatal("h on a leaf did not move to its parent")
	}
	if got := m.Rows[m.Cursor].Task; got == nil || got.ID != parent.ID {
		t.Fatalf("cursor task after parent move = %v, want %s", got, parent.ID)
	}
}

func TestProjectHeadersCollapseAndExpandWithHorizontalNavigation(t *testing.T) {
	for _, keys := range []struct {
		name     string
		collapse string
		expand   string
	}{
		{name: "vim aliases", collapse: "h", expand: "l"},
		{name: "arrow aliases", collapse: "left", expand: "right"},
	} {
		t.Run(keys.name, func(t *testing.T) {
			app := newUITestApp()
			alpha := scopeTestProject("alpha", "Alpha", "alpha-task")
			beta := scopeTestProject("beta", "Beta", "beta-task")
			app.projects[alpha.ID] = alpha
			app.projects[beta.ID] = beta

			outlinerView := app.crossOutliner
			outlinerView.Population.Projects = []*models.Project{alpha, beta}
			outlinerView.Population.Invalidate()
			outlinerView.CrossProjectRebuildRows()

			projectHeader := -1
			for index, row := range outlinerView.Rows {
				if row.IsProjectHeader && row.ProjectID == alpha.ID {
					projectHeader = index
					break
				}
			}
			if projectHeader < 0 {
				t.Fatalf("Alpha project header is missing from rows: %+v", outlinerView.Rows)
			}
			outlinerView.Cursor = projectHeader
			collapseKey := outlinerView.Rows[projectHeader].EffectiveCollapseKey()

			collapseMsg, err := parseUITestKey(keys.collapse)
			if err != nil {
				t.Fatalf("parse collapse key %q: %v", keys.collapse, err)
			}
			if handled, _ := app.handleGlobalKeyPress(collapseMsg); !handled {
				t.Fatalf("collapse key %q was not handled", keys.collapse)
			}
			if !outlinerView.Folds.CollapsedSections[collapseKey] {
				t.Fatalf("collapse key %q was not recorded", collapseKey)
			}
			if findTaskRow(outlinerView.Rows, "alpha-task") >= 0 {
				t.Fatal("Alpha task remains visible after collapsing its project header")
			}
			if findTaskRow(outlinerView.Rows, "beta-task") < 0 {
				t.Fatal("collapsing Alpha also hid Beta's task")
			}

			expandMsg, err := parseUITestKey(keys.expand)
			if err != nil {
				t.Fatalf("parse expand key %q: %v", keys.expand, err)
			}
			if handled, _ := app.handleGlobalKeyPress(expandMsg); !handled {
				t.Fatalf("expand key %q was not handled", keys.expand)
			}
			if outlinerView.Folds.CollapsedSections[collapseKey] {
				t.Fatalf("expand key %q did not clear collapse key %q", keys.expand, collapseKey)
			}
			if findTaskRow(outlinerView.Rows, "alpha-task") < 0 {
				t.Fatal("Alpha task did not reappear after expanding its project header")
			}
		})
	}
}

func TestSimplifiedShiftEnterPeekRendersAtEveryWidth(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "parent")
	task := project.Tasks[0]
	task.Tags = []string{"backend", "security"}
	child := makeTask("child", "Child", "")
	child.ParentID = &task.ID
	project.Tasks = append(project.Tasks, child)
	project.BuildTaskTree()
	app.projects[project.ID] = project
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.CrossProjectRebuildRows()
	app.crossOutliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID})

	for _, width := range []int{60, 99, 100, 160} {
		app.crossOutliner.PeekMode = false
		app.viewNav.width = width
		app.crossOutliner.SetSize(width, 22)
		result := app.operations.executor.Execute(operation.OperationExecutionRequest{
			ID:      operation.OperationCorePeek,
			Context: app.operationCommandContext(),
		})
		if result.Err != nil || result.Message != nil {
			t.Fatalf("width %d Ctrl+Enter result = message %T err %v", width, result.Message, result.Err)
		}
		if !app.crossOutliner.PeekMode {
			t.Fatalf("width %d Ctrl+Enter did not enable Peek", width)
		}
		plain := ansi.Strip(app.crossOutliner.View().Content)
		if !strings.Contains(plain, "Tags: #backend #security") {
			t.Fatalf("width %d Ctrl+Enter Peek missing selected task tags:\n%s", width, plain)
		}
	}
	if len(app.crossOutliner.Focus.CrossStack) != 0 || len(app.crossOutliner.Focus.Stack) != 0 {
		t.Fatal("Ctrl+Enter unexpectedly changed task Focus")
	}
}

// navigationAliasTestApp renders twelve rows whose first and last entries are
// non-navigable separators, so every jump alias has to skip inward.
func navigationAliasTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "one")
	app.projects[project.ID] = project
	out := app.crossOutliner
	out.Population.Projects = []*models.Project{project}
	out.SetSize(out.Width(), 11)
	out.Rows = make([]outliner.OutlinerRow, 0, 12)
	out.Rows = append(out.Rows, outliner.OutlinerRow{IsSeparator: true})
	for range 10 {
		out.Rows = append(out.Rows, outliner.OutlinerRow{Task: project.Tasks[0], ProjectID: project.ID})
	}
	out.Rows = append(out.Rows, outliner.OutlinerRow{IsSeparator: true})
	return app
}

func TestSingleOutlinerNavigationAliasesMatchTheirPairedKeys(t *testing.T) {
	tests := []struct {
		name      string
		spellings [][]string
		start     int
		want      int
	}{
		{name: "half page down", spellings: [][]string{{"ctrl+d"}, {"pgdown"}}, start: 1, want: 6},
		{name: "half page up", spellings: [][]string{{"ctrl+u"}, {"pgup"}}, start: 10, want: 5},
		{name: "first navigable row", spellings: [][]string{{"g", "g"}, {"home"}}, start: 6, want: 1},
		{name: "last navigable row", spellings: [][]string{{"G"}, {"end"}}, start: 1, want: 10},
	}
	for _, test := range tests {
		for _, spelling := range test.spellings {
			t.Run(test.name+"/"+strings.Join(spelling, ""), func(t *testing.T) {
				app := navigationAliasTestApp(t)
				app.crossOutliner.Cursor = test.start
				for _, key := range spelling {
					msg, err := parseUITestKey(key)
					if err != nil {
						t.Fatalf("parse %q: %v", key, err)
					}
					if handled, _ := app.handleGlobalKeyPress(msg); !handled {
						t.Fatalf("key %q was not handled", key)
					}
				}
				if got := app.crossOutliner.Cursor; got != test.want {
					t.Fatalf("cursor after %v = %d, want %d", spelling, got, test.want)
				}
			})
		}
	}
}

func TestSingleOutlinerNavigationJumpsFlashTheCursor(t *testing.T) {
	app := navigationAliasTestApp(t)
	app.crossOutliner.Cursor = 1
	msg, err := parseUITestKey("ctrl+d")
	if err != nil {
		t.Fatalf("parse ctrl+d: %v", err)
	}
	if handled, cmd := app.handleGlobalKeyPress(msg); !handled || cmd == nil {
		t.Fatalf("ctrl+d = handled %v cmd %v; want a cursor flash command", handled, cmd)
	}
	if !app.crossOutliner.CursorFlash {
		t.Fatal("half-page jump did not flash the cursor")
	}
}

func TestSimplifiedReorderEligibility(t *testing.T) {
	task := makeTask("task", "Task", "")
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()

	m := &outliner.OutlinerModel{
		Project: project,
		Rows:    []outliner.OutlinerRow{{Task: task}},
	}
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeInbox, chrome.ModeNext, chrome.ModeBacklog} {
		m.PrimaryMode = mode
		if !m.SimplifiedReorderEligible() {
			t.Errorf("%s should allow project-focused reorder", mode)
		}
	}
	m.PrimaryMode = chrome.ModeDue
	if m.SimplifiedReorderEligible() {
		t.Fatal("computed Due mode allowed manual reorder")
	}
	// A pinned row carries its own manual order ahead of the mode's ordering.
	m.PrimaryMode = chrome.ModeToday
	m.Rows[0].IsPinnedSection = true
	if !m.SimplifiedReorderEligible() {
		t.Fatal("Pinned row in Today mode refused manual reorder")
	}
	m.Rows[0].IsPinnedSection = false

	m.Project = nil
	m.PrimaryMode = chrome.ModeAll
	m.Population.Projects = []*models.Project{project}
	m.Rows[0].ProjectID = project.ID
	if !m.SimplifiedReorderEligible() {
		t.Fatal("base all-project hierarchy did not allow same-project sibling reorder")
	}
	m.Rows[0].IsPinnedSection = true
	if !m.SimplifiedReorderEligible() {
		t.Fatal("cross-project Pinned row refused manual reorder")
	}
	m.Rows[0].IsPinnedSection = false
	m.MultiSelectMode = true
	if m.SimplifiedReorderEligible() {
		t.Fatal("visual selection allowed cursor-only reorder")
	}
}

func TestMutationRebuildPreservesCursorIdentityAndVisibleSelection(t *testing.T) {
	first := makeTask("first", "First", "")
	second := makeTask("second", "Second", "")
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{first, second}}
	project.BuildTaskTree()
	m := outliner.NewOutlinerModel(project, nil, nil)
	m.SelectOutlinerMode(chrome.ModeAll)
	m.Cursor = findTaskRow(m.Rows, second.ID)
	if m.Cursor < 0 {
		t.Fatal("second task is missing before rebuild")
	}
	m.MultiSelectMode = true
	m.SelectTaskIdentity(taskref.For(first, project))
	m.SelectTaskIdentity(taskref.For(second, project))

	project.Tasks[0], project.Tasks[1] = project.Tasks[1], project.Tasks[0]
	project.BuildTaskTree()
	outliner.RebuildOutlinerPreservingCursor(m)

	task, _ := m.GetTaskRef()
	if task == nil || task.ID != second.ID {
		t.Fatalf("cursor target after rebuild = %v, want %s", task, second.ID)
	}
	if !m.MultiSelectMode || m.SelectedCount() != 2 {
		t.Fatalf("visible selection after rebuild = active %v count %d, want active/2", m.MultiSelectMode, m.SelectedCount())
	}
}

func TestSimplifiedCaptureUsesSystemInboxAtBaseAndFocusedProjectInScope(t *testing.T) {
	app := newUITestApp()
	inbox := scopeTestProject("inbox", "Inbox", "one")
	project := scopeTestProject("project", "Project", "one")
	app.runtime.Catalog().SystemUnfiledProjectID = inbox.ID
	app.projects[inbox.ID] = inbox
	app.projects[project.ID] = project
	app.crossOutliner.Population.Projects = []*models.Project{inbox, project}
	app.crossOutliner.Rows = []outliner.OutlinerRow{{
		Task:      project.Tasks[0],
		ProjectID: project.ID,
	}}

	base, ok := app.coreCaptureMessage().(taskform.ShowTaskComposerMsg)
	if !ok {
		t.Fatalf("base capture returned %T", app.coreCaptureMessage())
	}
	if base.Preset.Project != inbox {
		t.Fatalf("base capture project = %v, want system Inbox %v", base.Preset.Project, inbox)
	}

	app.crossOutliner.Scope.ProjectID = project.ID
	focused, ok := app.coreCaptureMessage().(taskform.ShowTaskComposerMsg)
	if !ok || focused.Preset.Project != project {
		t.Fatalf("focused capture = %#v, want project %v", focused, project)
	}
}

// TestSimplifiedCaptureFollowsAProjectHeaderCursor covers the one cursor row
// that names a project explicitly. Every other row keeps base capture in the
// system Inbox, and both destinations are named in the composer header.
func TestSimplifiedCaptureFollowsAProjectHeaderCursor(t *testing.T) {
	app := newUITestApp()
	inbox := scopeTestProject("inbox", "Unfiled", "one")
	project := scopeTestProject("project", "Project", "one")
	app.runtime.Catalog().SystemUnfiledProjectID = inbox.ID
	app.projects[inbox.ID] = inbox
	app.projects[project.ID] = project
	app.crossOutliner.Population.Projects = []*models.Project{inbox, project}
	app.crossOutliner.Rows = []outliner.OutlinerRow{
		{IsProjectHeader: true, ProjectID: project.ID},
		{Task: project.Tasks[0], ProjectID: project.ID},
	}

	app.crossOutliner.Cursor = 0
	header, ok := app.coreCaptureMessage().(taskform.ShowTaskComposerMsg)
	if !ok || header.Preset.Project != project {
		t.Fatalf("header capture = %#v, want project %v", header, project)
	}
	if header.Preset.HeaderLabel != "Capture → Project" {
		t.Fatalf("header capture label = %q", header.Preset.HeaderLabel)
	}

	app.crossOutliner.Cursor = 1
	task, ok := app.coreCaptureMessage().(taskform.ShowTaskComposerMsg)
	if !ok || task.Preset.Project != inbox {
		t.Fatalf("task-row capture = %#v, want system Inbox %v", task, inbox)
	}
	if task.Preset.HeaderLabel != "Capture → Unfiled" {
		t.Fatalf("task-row capture label = %q", task.Preset.HeaderLabel)
	}
}

// TestSimplifiedCaptureFollowsTaskFocus covers the explicit projection anchor:
// inside a Focus, capture parents the new task under the focus root instead of
// landing a root task somewhere the focused projection cannot show it.
func TestSimplifiedCaptureFollowsTaskFocus(t *testing.T) {
	shell := newUITestApp()
	inbox := scopeTestProject("inbox", "Unfiled", "one")
	project := scopeTestProject("project", "Project", "story")
	story := project.Tasks[0]
	story.Title = "Launch platform"
	shell.runtime.Catalog().SystemUnfiledProjectID = inbox.ID
	shell.projects[inbox.ID] = inbox
	shell.projects[project.ID] = project
	shell.crossOutliner.Population.Projects = []*models.Project{inbox, project}
	shell.crossOutliner.Rows = []outliner.OutlinerRow{
		{IsProjectHeader: true, ProjectID: inbox.ID},
	}
	shell.crossOutliner.Cursor = 0
	shell.crossOutliner.Focus.CrossStack = []outliner.CrossProjectFocusEntry{
		{ProjectID: project.ID, TaskID: story.ID},
	}

	focused, ok := shell.coreCaptureMessage().(taskform.ShowTaskComposerMsg)
	if !ok {
		t.Fatalf("focus capture returned %T", shell.coreCaptureMessage())
	}
	if focused.Preset.Kind != taskform.ComposerChild {
		t.Fatalf("focus capture kind = %v, want ComposerChild", focused.Preset.Kind)
	}
	if focused.Preset.Shared.ParentID == nil || *focused.Preset.Shared.ParentID != story.ID {
		t.Fatalf("focus capture parent = %v, want %s", focused.Preset.Shared.ParentID, story.ID)
	}
	if focused.Preset.Project != project {
		t.Fatalf("focus capture project = %v, want %v", focused.Preset.Project, project)
	}
	if focused.Preset.Shared.Destination != appservice.CaptureDestinationBacklog {
		t.Fatalf("focus capture destination = %q, want backlog", focused.Preset.Shared.Destination)
	}
	if focused.Preset.HeaderLabel != "Capture → child of Launch platform" {
		t.Fatalf("focus capture label = %q", focused.Preset.HeaderLabel)
	}

	// At the nesting cap a child would be detached back to the root, so capture
	// degrades to the focus root's project rather than refusing.
	story.Level = models.MaxNestingDepth
	capped, ok := shell.coreCaptureMessage().(taskform.ShowTaskComposerMsg)
	if !ok || capped.Preset.Project != project {
		t.Fatalf("capped focus capture = %#v, want project %v", capped, project)
	}
	if capped.Preset.Shared.ParentID != nil {
		t.Fatalf("capped focus capture kept parent %v", capped.Preset.Shared.ParentID)
	}
	if capped.Preset.HeaderLabel != "Capture → Project" {
		t.Fatalf("capped focus capture label = %q", capped.Preset.HeaderLabel)
	}
}

func TestResetToGlobalAllClearsInlineControls(t *testing.T) {
	project := scopeTestProject("project", "Project", "task")
	m := scopeTestOutliner(project)
	tasks := project.Tasks
	if len(tasks) == 0 {
		t.Fatal("fixture has no tasks")
	}
	m.Confirm.Show("delete", "Delete?")
	m.QuickEdit.Enter(tasks[0], false)

	m.ResetToGlobalAll()

	if m.Confirm.IsActive() ||
		m.QuickEdit.IsActive() {
		t.Fatalf(
			"reset left inline state: confirm=%v edit=%v",
			m.Confirm.IsActive(),
			m.QuickEdit.IsActive(),
		)
	}
}

func TestCtrlEscCancelsShellOwnedConfirmations(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "one")
	app.project.pendingArchive = project
	app.project.archiving = true
	app.project.archiveConfirm.Show(projecthub.ProjectArchiveConfirmAction, "Archive?")
	app.taskMutation.pendingDelete = taskDeletePending{
		roots: []TaskProjectRef{{Task: project.Tasks[0], Project: project}},
	}
	app.taskMutation.deleteConfirm.Show(taskDeleteConfirmAction, "Delete?")

	jumpToOutlinerRootForTest(app)

	if app.project.archiveConfirm.IsActive() ||
		app.project.pendingArchive != nil ||
		app.project.archiving {
		t.Fatal("Ctrl+Esc left a project archive confirmation pending")
	}
	if app.taskMutation.deleteConfirm.IsActive() || len(app.taskMutation.pendingDelete.roots) != 0 {
		t.Fatal("Ctrl+Esc left a task delete confirmation pending")
	}
}
