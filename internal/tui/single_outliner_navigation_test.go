package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// cursorOnFirstTaskRow parks the Outliner cursor on the first visible task row
// and returns that task plus its owning project ID, so a palette-resolution
// assertion can compare against exactly what the visible model shows.
func cursorOnFirstTaskRow(t *testing.T, m *OutlinerModel) (*models.Task, string) {
	t.Helper()
	for i, row := range m.rows {
		if row.task != nil {
			m.cursor = i
			return row.task, row.ProjectID
		}
	}
	t.Fatal("no task row visible in the scoped cross Outliner")
	return nil, ""
}

// TestLinkedTaskAndBlockerNavigationUseCanonicalOutliner locks FR14-001:
// linked-task navigation and Blocker "go to project" navigation must operate on
// the single authoritative cross Outliner (project selection is a scope frame)
// rather than constructing or targeting a separate AppModel.outliner root behind
// a project-scoped TaskWorkspace route. Because the visible model and the
// palette's target are the same instance, a palette task action after either
// navigation can never mutate a hidden canonical cursor, and Esc restores the
// previous Outliner scope.
//
//nolint:gocyclo // Both navigation paths share one canonical-Outliner invariant matrix.
func TestLinkedTaskAndBlockerNavigationUseCanonicalOutliner(t *testing.T) {
	t.Run("linked-task navigation scopes the cross Outliner and reveals the task", func(t *testing.T) {
		owner, ownerTasks := planningOrderProject(t, t.TempDir(), "p-own", "own", "Alpha", "Bravo")
		other, _ := planningOrderProject(t, t.TempDir(), "p-oth", "oth", "Charlie", "Delta")
		app, cross, _ := newPlanningOrderApp(t, owner, other)

		handled, _ := app.handleLinkedTaskNavigation(ownerTasks[1], owner)
		if !handled {
			t.Fatal("handleLinkedTaskNavigation should report handled")
		}
		if app.outliner != nil {
			t.Fatal("linked-task navigation must not construct a separate AppModel.outliner root")
		}
		if app.views.Get(ViewOutliner) != nil {
			t.Fatal("linked-task navigation must not register a ViewOutliner route")
		}
		if app.baseProjectView() != ViewCrossProjectOutliner {
			t.Fatalf("base view = %s, want the single %s root", app.baseProjectView(), ViewCrossProjectOutliner)
		}
		if cross.scopedProjectID != owner.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, owner.ID)
		}
		if len(cross.scopeStack) == 0 {
			t.Fatal("expected a saved scope frame so Esc restores the previous scope")
		}
		if cross.cursor < 0 || cross.cursor >= len(cross.rows) {
			t.Fatalf("cursor %d out of range for %d rows", cross.cursor, len(cross.rows))
		}
		if row := cross.rows[cross.cursor]; row.task == nil || row.task.ID != ownerTasks[1].ID {
			t.Fatalf("cursor did not land on the linked task; got row %+v", row)
		}

		// A palette task action after linked-task navigation resolves the same
		// project-qualified task the scoped Outliner shows.
		task, proj := app.paletteTaskTarget()
		if task == nil || proj == nil {
			t.Fatal("palette task action resolved no task from the scoped cross Outliner")
		}
		if task.ID != ownerTasks[1].ID || proj.ID != owner.ID {
			t.Fatalf("palette resolved %q in %q, want visible scoped task %q in %q",
				task.ID, proj.ID, ownerTasks[1].ID, owner.ID)
		}

		// Esc semantics: the pushed scope pops back to the clean global scope.
		if !cross.popProjectScope() {
			t.Fatal("expected the pushed project scope to pop")
		}
		if cross.scopedProjectID != "" {
			t.Fatalf("after pop scopedProjectID = %q, want global (empty)", cross.scopedProjectID)
		}
	})

	t.Run("blocker navigation returns to the app-scoped root and pushes a scope frame", func(t *testing.T) {
		owner, _ := planningOrderProject(t, t.TempDir(), "p-own2", "own2", "Alpha", "Bravo")
		other, _ := planningOrderProject(t, t.TempDir(), "p-oth2", "oth2", "Charlie", "Delta")
		app, cross, _ := newPlanningOrderApp(t, owner, other)
		// Enter the blockers hub, then jump to a project from it.
		if cmd := app.activateManageHub(ManageParams{Module: ManageBlockers, Scope: AppEntityScope()}); cmd != nil {
			cmd()
		}

		handled, cmd := app.handleViewSupportMessages(BlockerGoToProjectMsg{Project: other})
		if !handled {
			t.Fatal("BlockerGoToProjectMsg should be handled")
		}
		if cmd != nil {
			cmd()
		}
		if app.outliner != nil {
			t.Fatal("blocker navigation must not construct a separate AppModel.outliner root")
		}
		if app.views.Get(ViewOutliner) != nil {
			t.Fatal("blocker navigation must not register a ViewOutliner route")
		}
		if app.baseProjectView() != ViewCrossProjectOutliner {
			t.Fatalf("base view = %s, want the single %s root", app.baseProjectView(), ViewCrossProjectOutliner)
		}
		if cross.scopedProjectID != other.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, other.ID)
		}
		if len(cross.scopeStack) == 0 {
			t.Fatal("expected a saved scope frame so Esc restores the previous scope")
		}

		// A palette task action after blocker navigation resolves the same
		// project-qualified task the scoped Outliner shows under its cursor.
		visibleTask, visibleProject := cursorOnFirstTaskRow(t, cross)
		task, proj := app.paletteTaskTarget()
		if task == nil || proj == nil {
			t.Fatal("palette task action resolved no task from the scoped cross Outliner")
		}
		if task.ID != visibleTask.ID || proj.ID != visibleProject {
			t.Fatalf("palette resolved %q in %q, want visible cursor task %q in %q",
				task.ID, proj.ID, visibleTask.ID, visibleProject)
		}
		if visibleProject != other.ID {
			t.Fatalf("scoped Outliner shows project %q, want scoped project %q", visibleProject, other.ID)
		}

		if !cross.popProjectScope() {
			t.Fatal("expected the pushed project scope to pop")
		}
		if cross.scopedProjectID != "" {
			t.Fatalf("after pop scopedProjectID = %q, want global (empty)", cross.scopedProjectID)
		}
	})

	t.Run("typed linked-task navigation keeps the root TaskWorkspace app-scoped", func(t *testing.T) {
		owner, ownerTasks := planningOrderProject(t, t.TempDir(), "p-typed", "typed", "Alpha", "Bravo")
		app, cross, _ := newPlanningOrderApp(t, owner)
		app.enableTypedNavigation()

		if _, cmd := app.handleLinkedTaskNavigation(ownerTasks[1], owner); cmd != nil {
			cmd()
		}
		if app.outliner != nil {
			t.Fatal("typed linked-task navigation must not construct AppModel.outliner")
		}
		route := app.navigation.Primary.Current()
		if route.ID() != TaskWorkspace {
			t.Fatalf("root route = %v, want TaskWorkspace", route.ID())
		}
		params, ok := route.Params().(TaskWorkspaceParams)
		if !ok || params.Scope.Kind != ScopeApp {
			t.Fatalf("root scope = %v, want app-scoped (project selection is a scope frame)", params.Scope)
		}
		if cross.scopedProjectID != owner.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, owner.ID)
		}
	})
}

// TestNoProductionSingleOutlinerIdentity locks FR16-003's structural half:
// production (typed) project navigation must not construct, register, present,
// or target a separate AppModel.outliner or ViewOutliner. Every project
// selection becomes a scope frame on the single authoritative cross-project
// Outliner, so switchToProject no longer builds a second single-project instance
// on the typed path. This is what keeps the retained project commands exposed
// from the sole root operating on the model the user actually sees.
func TestNoProductionSingleOutlinerIdentity(t *testing.T) {
	assertSoleRoot := func(t *testing.T, app *AppModel, cross *OutlinerModel, projectID string) {
		t.Helper()
		if app.outliner != nil {
			t.Fatal("production navigation must not construct a separate AppModel.outliner")
		}
		if app.views.Get(ViewOutliner) != nil {
			t.Fatal("production navigation must not register a ViewOutliner route")
		}
		if app.baseProjectView() != ViewCrossProjectOutliner {
			t.Fatalf("base view = %s, want the single %s root", app.baseProjectView(), ViewCrossProjectOutliner)
		}
		route := app.navigation.Primary.Current()
		if route.ID() != TaskWorkspace {
			t.Fatalf("root route = %v, want TaskWorkspace", route.ID())
		}
		params, ok := route.Params().(TaskWorkspaceParams)
		if !ok || params.Scope.Kind != ScopeApp {
			t.Fatalf("root scope = %+v, want app-scoped (project selection is a scope frame)", route.Params())
		}
		if cross.scopedProjectID != projectID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, projectID)
		}
	}

	t.Run("selectProjectNow scopes the cross Outliner without a second instance", func(t *testing.T) {
		owner, _ := planningOrderProject(t, t.TempDir(), "p-sel", "sel", "Alpha", "Bravo")
		other, _ := planningOrderProject(t, t.TempDir(), "p-sel2", "sel2", "Charlie", "Delta")
		app, cross, _ := newPlanningOrderApp(t, owner, other)
		app.enableTypedNavigation()

		if cmd := app.selectProjectNow(other); cmd != nil {
			cmd()
		}
		assertSoleRoot(t, app, cross, other.ID)
	})

	t.Run("task navigation scopes the cross Outliner without a second instance", func(t *testing.T) {
		owner, ownerTasks := planningOrderProject(t, t.TempDir(), "p-nav", "nav", "Alpha", "Bravo")
		app, cross, _ := newPlanningOrderApp(t, owner)
		app.enableTypedNavigation()

		if cmd := app.navigateToTaskInProjectNow(ownerTasks[1], owner); cmd != nil {
			cmd()
		}
		assertSoleRoot(t, app, cross, owner.ID)
	})

	t.Run("switchToProject builds no single-project instance on the typed path", func(t *testing.T) {
		owner, _ := planningOrderProject(t, t.TempDir(), "p-sw", "sw", "Alpha", "Bravo")
		app, _, _ := newPlanningOrderApp(t, owner)
		app.enableTypedNavigation()

		app.switchToProject(owner, true)
		if app.outliner != nil {
			t.Fatal("switchToProject must not construct AppModel.outliner on the typed navigation path")
		}
		if app.views.Get(ViewOutliner) != nil {
			t.Fatal("switchToProject must not register ViewOutliner on the typed navigation path")
		}
	})
}

// TestTaskDeepLinkUsesCanonicalOutlinerScope locks FR15-001 for task deep links.
// The typed resolver's parent is the app-scoped Outliner root (never a
// project-scoped TaskWorkspace), and the live deep-link entry point reveals the
// task by pushing a project scope frame on the single cross Outliner while the
// root route stays app-scoped and no separate AppModel.outliner root survives.
func TestTaskDeepLinkUsesCanonicalOutlinerScope(t *testing.T) {
	t.Run("resolver parent is the app-scoped Outliner root plus a Task Detail child", func(t *testing.T) {
		lookup := fakeRouteLookup{
			projects: map[string]bool{"p1": true},
			tasks:    map[string]bool{"p1/t1": true},
		}
		resolution, err := ResolveTaskDetailDeepLink(TaskDetailParams{ProjectID: "p1", TaskID: "t1", Tab: TaskComments}, lookup)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.Parent.ID() != TaskWorkspace {
			t.Fatalf("parent route = %s, want TaskWorkspace", resolution.Parent.ID())
		}
		params, ok := resolution.Parent.Params().(TaskWorkspaceParams)
		if !ok || params.Scope != AppEntityScope() {
			t.Fatalf("parent scope = %+v, want app-scoped root (never a project root)", resolution.Parent.Params())
		}
		if resolution.Detail == nil || resolution.Detail.ID() != TaskDetail {
			t.Fatalf("detail child = %+v, want a Task Detail child above the app-scoped root", resolution.Detail)
		}
	})

	t.Run("live deep link keeps the root app-scoped and scopes the cross Outliner", func(t *testing.T) {
		owner, ownerTasks := planningOrderProject(t, t.TempDir(), "p-deep", "deep", "Alpha", "Bravo")
		other, _ := planningOrderProject(t, t.TempDir(), "p-deep2", "deep2", "Charlie", "Delta")
		app, cross, _ := newPlanningOrderApp(t, owner, other)
		app.enableTypedNavigation()

		if cmd := app.navigateToTaskInProjectNow(ownerTasks[1], owner); cmd != nil {
			cmd()
		}
		if app.outliner != nil {
			t.Fatal("task deep link must not construct a separate AppModel.outliner root")
		}
		if app.views.Get(ViewOutliner) != nil {
			t.Fatal("task deep link must not register a ViewOutliner route")
		}
		route := app.navigation.Primary.Current()
		if route.ID() != TaskWorkspace {
			t.Fatalf("root route = %s, want TaskWorkspace", route.ID())
		}
		params, ok := route.Params().(TaskWorkspaceParams)
		if !ok || params.Scope.Kind != ScopeApp {
			t.Fatalf("root scope = %+v, want app-scoped root (project selection is a scope frame)", params.Scope)
		}
		if cross.scopedProjectID != owner.ID {
			t.Fatalf("scopedProjectID = %q, want %q", cross.scopedProjectID, owner.ID)
		}
		if len(cross.scopeStack) == 0 {
			t.Fatal("expected a saved scope frame so Esc restores the previous scope")
		}
		if cross.cursor < 0 || cross.cursor >= len(cross.rows) {
			t.Fatalf("cursor %d out of range for %d rows", cross.cursor, len(cross.rows))
		}
		if row := cross.rows[cross.cursor]; row.task == nil || row.task.ID != ownerTasks[1].ID {
			t.Fatalf("cursor did not land on the deep-linked task; got row %+v", row)
		}
	})
}
