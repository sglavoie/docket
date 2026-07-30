package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func findTaskRow(rows []outlinerRow, taskID string) int {
	for index, row := range rows {
		if row.task != nil && row.task.ID == taskID {
			return index
		}
	}
	return -1
}

func simplifiedTreeOutliner(t *testing.T) (*OutlinerModel, *models.Project, *models.Task, *models.Task) {
	t.Helper()
	parent := &models.Task{
		ID:            "parent",
		ShortID:       1,
		Title:         "Parent",
		WorkflowState: models.StateReady,
	}
	child := &models.Task{
		ID:            "child",
		ShortID:       2,
		Title:         "Child",
		WorkflowState: models.StateReady,
		ParentID:      &parent.ID,
	}
	project := &models.Project{
		ID:    "project",
		Name:  "Project",
		Tasks: []*models.Task{parent, child},
	}
	project.BuildTaskTree()
	cfg := newTestConfig()
	m := NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.selectOutlinerMode(ModeAll)
	return m, project, parent, child
}

func TestSimplifiedHorizontalNavigationCollapsesExpandsAndMovesToParent(t *testing.T) {
	m, _, parent, child := simplifiedTreeOutliner(t)
	parentRow := findTaskRow(m.rows, parent.ID)
	childRow := findTaskRow(m.rows, child.ID)
	if parentRow < 0 || childRow < 0 || !m.rows[parentRow].hasVisibleChildren {
		t.Fatalf("tree rows not rendered as parent/child: parent=%d child=%d rows=%+v", parentRow, childRow, m.rows)
	}

	m.cursor = parentRow
	if !m.collapseOrMoveToParent() {
		t.Fatal("h did not collapse the expanded parent")
	}
	if got := findTaskRow(m.rows, child.ID); got >= 0 {
		t.Fatalf("child remains visible at row %d after collapse", got)
	}

	if !m.expandAtCursor() {
		t.Fatal("l did not expand the collapsed parent")
	}
	childRow = findTaskRow(m.rows, child.ID)
	if childRow < 0 {
		t.Fatal("child did not become visible after expand")
	}
	m.cursor = childRow
	if !m.collapseOrMoveToParent() {
		t.Fatal("h on a leaf did not move to its parent")
	}
	if got := m.rows[m.cursor].task; got == nil || got.ID != parent.ID {
		t.Fatalf("cursor task after parent move = %v, want %s", got, parent.ID)
	}
}

func TestSimplifiedEnterUsesResponsiveInspector(t *testing.T) {
	app := newUITestApp()
	app.width = inspectorWideBreakpoint - 1
	project := scopeTestProject("project", "Project", "parent")
	task := project.Tasks[0]
	child := makeTask("child", "Child", "")
	child.ParentID = &task.ID
	project.Tasks = append(project.Tasks, child)
	project.BuildTaskTree()
	app.projects[project.ID] = project
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.rows = []outlinerRow{{
		task:               task,
		ProjectID:          project.ID,
		hasVisibleChildren: true,
	}}

	result := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationCoreDetail,
		Context: app.operationCommandContext(),
		Source:  OperationSourceDirectKey,
	})
	if result.Err != nil {
		t.Fatalf("execute detail: %v", result.Err)
	}
	detail, ok := result.Message.(ShowTaskInspectorMsg)
	if !ok {
		t.Fatalf("narrow Enter returned %T, want ShowTaskInspectorMsg", result.Message)
	}
	if detail.Task != task || detail.Project != project {
		t.Fatalf("detail target = (%v, %v), want (%v, %v)", detail.Task, detail.Project, task, project)
	}
	app.width = inspectorWideBreakpoint
	result = app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationCoreDetail,
		Context: app.operationCommandContext(),
		Source:  OperationSourceDirectKey,
	})
	if result.Err != nil || result.Message != nil {
		t.Fatalf("wide Enter result = message %T err %v, want no-op", result.Message, result.Err)
	}
	if len(app.crossOutliner.crossZoomStack) != 0 || len(app.crossOutliner.zoomStack) != 0 {
		t.Fatal("Enter unexpectedly changed task zoom")
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
	out.loadedProjects = []*models.Project{project}
	out.height = 11
	out.rows = make([]outlinerRow, 0, 12)
	out.rows = append(out.rows, outlinerRow{IsSeparator: true})
	for range 10 {
		out.rows = append(out.rows, outlinerRow{task: project.Tasks[0], ProjectID: project.ID})
	}
	out.rows = append(out.rows, outlinerRow{IsSeparator: true})
	return app
}

func TestSingleOutlinerNavigationAliasesMatchTheirPairedKeys(t *testing.T) {
	tests := []struct {
		name  string
		keys  []string
		start int
		want  int
	}{
		{name: "half page down", keys: []string{"ctrl+d", "pgdown"}, start: 1, want: 6},
		{name: "half page up", keys: []string{"ctrl+u", "pgup"}, start: 10, want: 5},
		{name: "first navigable row", keys: []string{"g", "home"}, start: 6, want: 1},
		{name: "last navigable row", keys: []string{"G", "end"}, start: 1, want: 10},
	}
	for _, test := range tests {
		for _, key := range test.keys {
			t.Run(test.name+"/"+key, func(t *testing.T) {
				app := navigationAliasTestApp(t)
				app.crossOutliner.cursor = test.start
				msg, err := parseUITestKey(key)
				if err != nil {
					t.Fatalf("parse %q: %v", key, err)
				}
				if handled, _ := app.handleGlobalKeyPress(msg); !handled {
					t.Fatalf("key %q was not handled", key)
				}
				if got := app.crossOutliner.cursor; got != test.want {
					t.Fatalf("cursor after %q = %d, want %d", key, got, test.want)
				}
			})
		}
	}
}

func TestSingleOutlinerNavigationJumpsFlashTheCursor(t *testing.T) {
	app := navigationAliasTestApp(t)
	app.crossOutliner.cursor = 1
	msg, err := parseUITestKey("ctrl+d")
	if err != nil {
		t.Fatalf("parse ctrl+d: %v", err)
	}
	if handled, cmd := app.handleGlobalKeyPress(msg); !handled || cmd == nil {
		t.Fatalf("ctrl+d = handled %v cmd %v; want a cursor flash command", handled, cmd)
	}
	if !app.crossOutliner.cursorFlash {
		t.Fatal("half-page jump did not flash the cursor")
	}
}

func TestSimplifiedReorderEligibility(t *testing.T) {
	task := makeTask("task", "Task", "")
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	project.BuildTaskTree()

	m := &OutlinerModel{
		project: project,
		rows:    []outlinerRow{{task: task}},
	}
	for _, mode := range []OutlinerModeID{ModeAll, ModeInbox, ModeNext, ModeBacklog} {
		m.primaryMode = mode
		if !m.simplifiedReorderEligible() {
			t.Errorf("%s should allow project-focused reorder", mode)
		}
	}
	m.primaryMode = ModeDue
	if m.simplifiedReorderEligible() {
		t.Fatal("computed Due mode allowed manual reorder")
	}

	m.project = nil
	m.primaryMode = ModeAll
	m.loadedProjects = []*models.Project{project}
	m.rows[0].ProjectID = project.ID
	if !m.simplifiedReorderEligible() {
		t.Fatal("base all-project hierarchy did not allow same-project sibling reorder")
	}
	m.rows[0].IsPinnedSection = true
	if m.simplifiedReorderEligible() {
		t.Fatal("automatic Pinned section allowed manual reorder")
	}
	m.rows[0].IsPinnedSection = false
	m.multiSelectMode = true
	if m.simplifiedReorderEligible() {
		t.Fatal("visual selection allowed cursor-only reorder")
	}
}

func TestMutationRebuildPreservesCursorIdentityAndVisibleSelection(t *testing.T) {
	first := makeTask("first", "First", "")
	second := makeTask("second", "Second", "")
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{first, second}}
	project.BuildTaskTree()
	m := NewOutlinerModel(project, nil, nil)
	m.selectOutlinerMode(ModeAll)
	m.cursor = findTaskRow(m.rows, second.ID)
	if m.cursor < 0 {
		t.Fatal("second task is missing before rebuild")
	}
	m.multiSelectMode = true
	m.SelectTaskIdentity(taskIdentityFor(first, project))
	m.SelectTaskIdentity(taskIdentityFor(second, project))

	project.Tasks[0], project.Tasks[1] = project.Tasks[1], project.Tasks[0]
	project.BuildTaskTree()
	rebuildOutlinerPreservingCursor(m)

	task, _ := m.getTaskRef()
	if task == nil || task.ID != second.ID {
		t.Fatalf("cursor target after rebuild = %v, want %s", task, second.ID)
	}
	if !m.multiSelectMode || m.SelectedCount() != 2 {
		t.Fatalf("visible selection after rebuild = active %v count %d, want active/2", m.multiSelectMode, m.SelectedCount())
	}
}

func TestSimplifiedCaptureUsesSystemInboxAtBaseAndFocusedProjectInScope(t *testing.T) {
	app := newUITestApp()
	inbox := scopeTestProject("inbox", "Inbox", "one")
	project := scopeTestProject("project", "Project", "one")
	app.runtime.Catalog().SystemInboxProjectID = inbox.ID
	app.projects[inbox.ID] = inbox
	app.projects[project.ID] = project
	app.crossOutliner.loadedProjects = []*models.Project{inbox, project}
	app.crossOutliner.rows = []outlinerRow{{
		task:      project.Tasks[0],
		ProjectID: project.ID,
	}}

	base, ok := app.coreCaptureMessage().(ShowTaskComposerMsg)
	if !ok {
		t.Fatalf("base capture returned %T", app.coreCaptureMessage())
	}
	if base.Preset.Project != inbox {
		t.Fatalf("base capture project = %v, want system Inbox %v", base.Preset.Project, inbox)
	}

	app.crossOutliner.scopedProjectID = project.ID
	focused, ok := app.coreCaptureMessage().(ShowTaskComposerMsg)
	if !ok || focused.Preset.Project != project {
		t.Fatalf("focused capture = %#v, want project %v", focused, project)
	}
}

func TestResetToGlobalAllClearsInlineControls(t *testing.T) {
	project := scopeTestProject("project", "Project", "task")
	m := scopeTestOutliner(project)
	tasks := project.Tasks
	if len(tasks) == 0 {
		t.Fatal("fixture has no tasks")
	}
	m.confirm.Show("delete", "Delete?")
	m.quickEdit.Enter(tasks[0], false)
	m.crossQuickEditProject = m.loadedProjects[0]

	m.resetToGlobalAll()

	if m.confirm.IsActive() ||
		m.quickEdit.IsActive() ||
		m.crossQuickEditProject != nil {
		t.Fatalf(
			"reset left inline state: confirm=%v edit=%v editProject=%v",
			m.confirm.IsActive(),
			m.quickEdit.IsActive(),
			m.crossQuickEditProject,
		)
	}
}

func TestCtrlEscCancelsShellOwnedConfirmations(t *testing.T) {
	app := newUITestApp()
	project := scopeTestProject("project", "Project", "one")
	app.pendingProjectArchive = project
	app.pendingProjectArchiveArchive = true
	app.projectArchiveConfirm.Show(projectArchiveConfirmAction, "Archive?")
	app.pendingTaskDelete = taskDeletePending{
		roots: []TaskProjectRef{{Task: project.Tasks[0], Project: project}},
	}
	app.taskDeleteConfirm.Show(taskDeleteConfirmAction, "Delete?")

	app.jumpToOutlinerRoot()

	if app.projectArchiveConfirm.IsActive() ||
		app.pendingProjectArchive != nil ||
		app.pendingProjectArchiveArchive {
		t.Fatal("Ctrl+Esc left a project archive confirmation pending")
	}
	if app.taskDeleteConfirm.IsActive() || len(app.pendingTaskDelete.roots) != 0 {
		t.Fatal("Ctrl+Esc left a task delete confirmation pending")
	}
}
