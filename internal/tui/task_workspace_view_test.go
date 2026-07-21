package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

type recordingTaskWorkspaceMutations struct {
	action   string
	ids      []string
	pinned   bool
	edit     app.EditRequest
	state    models.WorkflowState
	priority int
}

func (m *recordingTaskWorkspaceMutations) record(action string, ids []string) tea.Cmd {
	m.action, m.ids = action, slices.Clone(ids)
	return func() tea.Msg { return StatusMsg(action) }
}

func (m *recordingTaskWorkspaceMutations) Transition(ids []string, state models.WorkflowState) tea.Cmd {
	m.state = state
	return m.record("transition", ids)
}
func (m *recordingTaskWorkspaceMutations) Edit(req app.EditRequest) tea.Cmd {
	m.edit = req
	return m.record("edit", []string{req.TaskID})
}
func (m *recordingTaskWorkspaceMutations) SetPriority(ids []string, priority int) tea.Cmd {
	m.priority = priority
	return m.record("priority", ids)
}
func (m *recordingTaskWorkspaceMutations) SetDueDate(ids []string, _ *time.Time, _ bool) tea.Cmd {
	return m.record("due-date", ids)
}
func (m *recordingTaskWorkspaceMutations) Snooze(ids []string, _ *time.Time) tea.Cmd {
	return m.record("snooze", ids)
}
func (m *recordingTaskWorkspaceMutations) SetPinned(id string, pinned bool) tea.Cmd {
	m.pinned = pinned
	return m.record("pin", []string{id})
}
func (m *recordingTaskWorkspaceMutations) MoveUp(id string) tea.Cmd {
	return m.record("move-up", []string{id})
}
func (m *recordingTaskWorkspaceMutations) MoveDown(id string) tea.Cmd {
	return m.record("move-down", []string{id})
}

func workspaceViewFixture(t *testing.T) (*TaskWorkspaceController, *TaskWorkspaceView, *recordingTaskWorkspaceMutations, *models.Project, *models.Project) {
	t.Helper()
	parentID := "parent"
	p1 := &models.Project{ID: "p1", Name: "Alpha", Tasks: []*models.Task{
		{ID: parentID, Title: "Parent task", WorkflowState: models.StateReady, Priority: 2},
		{ID: "child", Title: "Child needle", WorkflowState: models.StateReady, Priority: 3, ParentID: &parentID},
		{ID: "done", Title: "Finished", WorkflowState: models.StateDone, Priority: 4},
	}}
	p2 := &models.Project{ID: "p2", Name: "Beta", Tasks: []*models.Task{
		{ID: "other", Title: "Other task", WorkflowState: models.StateReady, Priority: 1},
	}}
	p1.BuildTaskTree()
	p2.BuildTaskTree()
	mutations := &recordingTaskWorkspaceMutations{}
	controller, err := NewTaskWorkspaceController(
		TaskWorkspaceParams{Scope: AppEntityScope(), Layout: WorkspaceOutline, Focus: WorkspaceAll},
		NewTaskWorkspaceQuery(staticTaskWorkspaceSource{data: TaskWorkspaceData{Projects: []*models.Project{p1, p2}}}),
		mutations,
	)
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewTaskWorkspaceView(controller)
	if err != nil {
		t.Fatal(err)
	}
	view.SetSize(80, 20)
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	return controller, view, mutations, p1, p2
}

func TestTaskWorkspaceOutlineHierarchyFoldZoomAndSelection(t *testing.T) {
	controller, view, _, _, _ := workspaceViewFixture(t)
	parent := TaskIdentity{ProjectID: "p1", TaskID: "parent"}
	child := TaskIdentity{ProjectID: "p1", TaskID: "child"}

	if !slices.ContainsFunc(view.rows, func(row taskWorkspaceRow) bool {
		return row.kind == workspaceProjectHeader && row.label == "Alpha"
	}) {
		t.Fatal("app-scoped Outline did not render project headers")
	}
	childRow := slices.IndexFunc(view.rows, func(row taskWorkspaceRow) bool {
		return row.kind == workspaceTaskRow && row.item.Identity == child
	})
	if childRow < 0 || view.rows[childRow].depth != 1 {
		t.Fatalf("child row was not hierarchical: index=%d rows=%+v", childRow, view.rows)
	}

	if !controller.State.SetCursor(child) || !controller.State.ToggleSelection(child) || !controller.State.SetCursor(parent) {
		t.Fatal("failed to seed Outline identity state")
	}
	view.collapseCurrent()
	if slices.ContainsFunc(view.rows, func(row taskWorkspaceRow) bool {
		return row.kind == workspaceTaskRow && row.item.Identity == child
	}) {
		t.Fatalf("collapsed parent retained a visible child: collapsed=%v rows=%+v cursor=%+v", view.collapsedTasks, view.rows, controller.State.cursor)
	}
	view.expandCurrent()
	if !slices.Equal(controller.State.Selected(), []TaskIdentity{child}) {
		t.Fatalf("unfold did not restore identity selection: %+v", controller.State.Selected())
	}
	if !view.ShouldCaptureEsc() {
		t.Fatal("workspace selection did not capture Esc")
	}
	_, _ = view.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(controller.State.Selected()) != 0 {
		t.Fatalf("Esc did not clear workspace selection: %+v", controller.State.Selected())
	}

	controller.State.SetCursor(parent)
	if cmd := view.handleAction(WorkspaceActionZoom); cmd != nil {
		_ = cmd()
	}
	if root, ok := controller.State.ZoomRoot(); !ok || root != parent {
		t.Fatalf("zoom root = %+v ok=%v", root, ok)
	}
	if breadcrumb := view.zoomBreadcrumb(); !strings.Contains(breadcrumb, "Alpha > Parent task") {
		t.Fatalf("zoom breadcrumb = %q", breadcrumb)
	}
	if cmd := view.handleAction(WorkspaceActionZoomOut); cmd != nil {
		_ = cmd()
	}
}

func TestTaskWorkspaceSearchLayoutSwitchGroupingAndCompleted(t *testing.T) {
	controller, view, _, _, _ := workspaceViewFixture(t)
	child := TaskIdentity{ProjectID: "p1", TaskID: "child"}
	other := TaskIdentity{ProjectID: "p2", TaskID: "other"}
	if !controller.State.SetCursor(other) || !controller.State.ToggleSelection(child) {
		t.Fatal("failed to seed layout-preservation state")
	}

	_, _ = view.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range "needle" {
		_, _ = view.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if got := workspaceSnapshotTaskIDs(view.snapshot); !slices.Equal(got, []string{"child"}) {
		t.Fatalf("search results = %v", got)
	}
	_, _ = view.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(view.snapshot.Items) != 3 {
		t.Fatalf("clearing search restored %d tasks, want 3", len(view.snapshot.Items))
	}
	if !controller.State.SetCursor(other) {
		t.Fatal("failed to restore the cross-project cursor before switching layout")
	}

	if err := controller.State.SetGrouping(WorkspaceGroupProject); err != nil {
		t.Fatal(err)
	}
	if err := controller.State.SetLayout(WorkspaceList); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	if cursor, _ := controller.State.Cursor(); cursor != other {
		t.Fatalf("List switch changed cursor identity: %+v", cursor)
	}
	if !slices.Equal(controller.State.Selected(), []TaskIdentity{child}) {
		t.Fatalf("List switch changed selection identity: %+v", controller.State.Selected())
	}
	groups := 0
	for _, row := range view.rows {
		if row.kind == workspaceGroupHeader {
			groups++
		}
	}
	if groups != 2 {
		t.Fatalf("List did not render both project groups: %+v", view.rows)
	}

	if err := controller.State.SetFocus(WorkspaceCompleted, ""); err != nil {
		t.Fatal(err)
	}
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := workspaceSnapshotTaskIDs(view.snapshot); !slices.Equal(got, []string{"done"}) {
		t.Fatalf("Completed focus = %v", got)
	}
}

func TestTaskWorkspaceViewMenuIncludesBoardAndUsesOneOrganizationSurface(t *testing.T) {
	controller, view, _, _, _ := workspaceViewFixture(t)
	items := view.workspaceMenuItems()
	if !slices.ContainsFunc(items, func(item TaskWorkspaceMenuItem) bool { return item.ID == "layout:board" }) {
		t.Fatal("registered Board mode is missing from the View menu")
	}
	for _, id := range []string{"layout:outline", "layout:list", "organize:group:status", "organize:sort:priority", "organize:column:project"} {
		if !slices.ContainsFunc(items, func(item TaskWorkspaceMenuItem) bool { return item.ID == id }) {
			t.Fatalf("single View menu missing %q", id)
		}
	}
	idx := slices.IndexFunc(items, func(item TaskWorkspaceMenuItem) bool { return item.ID == "layout:list" })
	view.menuOpen, view.menuCursor = true, idx
	cmd := view.handleMenuKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if controller.State.Layout != WorkspaceList || view.menuOpen {
		t.Fatalf("View menu did not switch to List: layout=%v open=%v", controller.State.Layout, view.menuOpen)
	}
	if cmd == nil {
		t.Fatal("layout change did not emit typed route synchronization messages")
	}
}

func TestTaskWorkspaceCreationMutationMoveAndBulkSelection(t *testing.T) {
	controller, view, mutations, _, _ := workspaceViewFixture(t)
	parent := TaskIdentity{ProjectID: "p1", TaskID: "parent"}
	child := TaskIdentity{ProjectID: "p1", TaskID: "child"}
	controller.State.SetCursor(parent)

	cmd := view.handleAction(WorkspaceActionAdd)
	if msg, ok := cmd().(CreateTaskMsg); !ok || msg.Project == nil || msg.Project.ID != "p1" {
		t.Fatalf("workspace add emitted %#v", msg)
	}
	cmd = view.handleAction(WorkspaceActionEdit)
	editMsg, ok := cmd().(EditTaskMsg)
	if !ok || editMsg.Task == nil || editMsg.Task.ID != "parent" || editMsg.Project == nil || editMsg.Project.ID != "p1" {
		t.Fatalf("workspace edit did not route through Task Composer preset: %#v", editMsg)
	}

	controller.State.ToggleSelection(parent)
	controller.State.ToggleSelection(child)
	cmd = view.handleAction(WorkspaceActionComplete)
	if cmd == nil || mutations.action != "transition" || !slices.Equal(mutations.ids, []string{"parent", "child"}) {
		t.Fatalf("bulk completion did not preserve one target set: action=%q ids=%v", mutations.action, mutations.ids)
	}
	controller.State.ClearSelection()
	controller.State.SetCursor(child)
	cmd = view.handleAction(WorkspaceActionMoveUp)
	if cmd == nil || mutations.action != "move-up" || !slices.Equal(mutations.ids, []string{"child"}) {
		t.Fatalf("reorder did not use the workspace mutation adapter: action=%q ids=%v", mutations.action, mutations.ids)
	}
}

func TestTaskWorkspaceNarrowRenderAndRefreshAfterRemoval(t *testing.T) {
	controller, view, _, p1, _ := workspaceViewFixture(t)
	view.SetSize(32, 8)
	for _, line := range strings.Split(view.View().Content, "\n") {
		if width := lipgloss.Width(ansi.Strip(line)); width > 32 {
			t.Fatalf("narrow workspace line width = %d, want <= 32: %q", width, ansi.Strip(line))
		}
	}

	removed := TaskIdentity{ProjectID: "p1", TaskID: "parent"}
	controller.State.SetCursor(removed)
	p1.Tasks = slices.DeleteFunc(p1.Tasks, func(task *models.Task) bool { return task.ID == "parent" || task.ID == "child" })
	p1.BuildTaskTree()
	if err := controller.Refresh(); err != nil {
		t.Fatal(err)
	}
	if cursor, ok := controller.State.Cursor(); !ok || cursor == removed {
		t.Fatalf("refresh retained removed cursor identity: %+v ok=%v", cursor, ok)
	}
}

func TestTypedWorkspacePresentationsDoNotRemainLiveLegacyRoutes(t *testing.T) {
	controller, view, _, p1, _ := workspaceViewFixture(t)
	app := &AppModel{
		typedNavigation:   true,
		navigation:        NewNavigationState(),
		viewStack:         []PresentationID{ViewDashboard},
		views:             NewViewRegistry(),
		taskWorkspace:     controller,
		taskWorkspaceView: view,
		currentProject:    p1,
	}
	cases := []struct {
		legacy PresentationID
		layout TaskWorkspaceLayout
		focus  TaskWorkspaceFocus
	}{
		{ViewOutliner, WorkspaceOutline, WorkspaceActive},
		{ViewCrossProjectOutliner, WorkspaceOutline, WorkspaceActive},
		{ViewAllTasks, WorkspaceList, WorkspaceActive},
		{ViewKanban, WorkspaceBoard, WorkspaceActive},
		{ViewCrossProjectKanban, WorkspaceBoard, WorkspaceActive},
		{ViewPriorityView, WorkspaceList, WorkspaceHighPriority},
		{ViewProjectFocusView, WorkspaceList, WorkspaceNext},
	}
	for _, tc := range cases {
		legacy := tc.legacy
		if !app.installTypedPresentation(legacy) {
			t.Fatalf("failed to adapt %s", legacy)
		}
		route := app.navigation.Primary.Current()
		if route.ID() != TaskWorkspace {
			t.Fatalf("%s route = %s", legacy, route.ID())
		}
		if presentation, live := route.Presentation(); live {
			t.Fatalf("%s remained a live legacy route as %s", legacy, presentation)
		}
		if !app.activeTaskWorkspace() {
			t.Fatalf("%s did not activate the consolidated workspace", legacy)
		}
		params := route.Params().(TaskWorkspaceParams)
		if params.Layout != tc.layout || params.Focus != tc.focus {
			t.Fatalf("%s params = %+v, want layout=%v focus=%v", legacy, params, tc.layout, tc.focus)
		}
	}
}
