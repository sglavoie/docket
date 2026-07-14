package tui

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// TestInboxProcessors_HandledKeysParity is the canonical "one interaction model"
// guard for session 6. Both InboxProcessorModel and GlobalInboxProcessorModel
// embed inboxCore, so the keymap derived from buildKeyMap() must be byte-identical
// between the two — there is no permitted key-level divergence. Any future
// triage affordance must extend the shared core, not a per-backend keymap.
func TestInboxProcessors_HandledKeysParity(t *testing.T) {
	local := (&InboxProcessorModel{}).HandledKeys()
	global := (&GlobalInboxProcessorModel{}).HandledKeys()

	sortedLocal := append([]string{}, local...)
	sortedGlobal := append([]string{}, global...)
	sort.Strings(sortedLocal)
	sort.Strings(sortedGlobal)

	if !slices.Equal(sortedLocal, sortedGlobal) {
		t.Fatalf("inbox processors must share an identical key set; got:\n"+
			" local : %v\n"+
			" global: %v",
			sortedLocal, sortedGlobal)
	}
}

// TestRenderInboxTaskDisplay_ProjectNameVisibility locks in the only render-time
// difference between local and global inbox: the global processor surfaces
// "Project: <name>" while the local processor omits the line entirely. inboxCore
// routes the choice through backend.showProjectInHeader() — see
// inbox_core_render.go — so any future redesign of the task card must keep the
// projectName-controlled branch in renderInboxTaskDisplay.
func TestRenderInboxTaskDisplay_ProjectNameVisibility(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		ShortID:       42,
		Title:         "Sample task",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateInbox,
		Tags:          []string{},
		DependsOn:     []string{},
	}
	cfg := newTestConfig()
	cfg.Settings().SectionColors = map[string]string{}

	t.Run("global mode shows project line", func(t *testing.T) {
		out := ansi.Strip(renderInboxTaskDisplay(task, 100, false, "Backend API", cfg.Settings()))
		if !strings.Contains(out, "Project: Backend API") {
			t.Fatalf("expected 'Project: Backend API' line in global render, got:\n%s", out)
		}
	})

	t.Run("local mode omits project line", func(t *testing.T) {
		out := ansi.Strip(renderInboxTaskDisplay(task, 100, false, "", cfg.Settings()))
		if strings.Contains(out, "Project:") {
			t.Fatalf("expected no 'Project:' line in local render, got:\n%s", out)
		}
	})
}

// TestInboxProcessorBackendStrings locks the user-facing copy that legitimately
// differs between the two processors so a regression that swaps or unifies it
// gets caught here. Anything not listed must stay shared via inboxCore.
func TestInboxProcessorBackendStrings(t *testing.T) {
	local := &InboxProcessorModel{}
	global := &GlobalInboxProcessorModel{projectCache: map[string]*models.Project{}}

	if local.showProjectInHeader() {
		t.Error("local processor must not show project header")
	}
	if !global.showProjectInHeader() {
		t.Error("global processor must show project header")
	}

	if !strings.Contains(local.headerTitle(), "Inbox") {
		t.Errorf("local header title should mention Inbox, got %q", local.headerTitle())
	}
	if !strings.Contains(global.headerTitle(), "Global Inbox") {
		t.Errorf("global header title should mention Global Inbox, got %q", global.headerTitle())
	}

	if local.headerSubtitleExtra() != "" {
		t.Errorf("local header subtitle extra should be empty, got %q", local.headerSubtitleExtra())
	}
	if !strings.Contains(global.headerSubtitleExtra(), "across 0") {
		t.Errorf("global header subtitle extra should reflect project count, got %q", global.headerSubtitleExtra())
	}

	global.projectCache["p1"] = &models.Project{ID: "p1", Name: "P1"}
	global.projectCache["p2"] = &models.Project{ID: "p2", Name: "P2"}
	if !strings.Contains(global.headerSubtitleExtra(), "across 2") {
		t.Errorf("global subtitle should track project cache size, got %q", global.headerSubtitleExtra())
	}

	if local.returnHint() == global.returnHint() {
		t.Errorf("local and global return hints should differ; both are %q", local.returnHint())
	}

	if local.viewType() == global.viewType() {
		t.Error("local and global view types must differ so the view registry routes them separately")
	}

	if local.StatusBarLabel() == global.StatusBarLabel() {
		t.Errorf("local and global status-bar labels should differ; both are %q", local.StatusBarLabel())
	}
}

// TestInboxProcessorAutoProcessHooksAreUnified verifies the session-4 contract
// that local and global inbox processors treat "processed/triaged" the same
// way: deleting, sectioning a task out of Inbox, or completing it must mark
// the task processed in both backends. The previous divergence (local kept
// transitioned tasks in the queue) confused power users because progress
// counts and undo behavior depended on whether they entered the loop via the
// project tree or the dashboard.
func TestInboxProcessorAutoProcessHooksAreUnified(t *testing.T) {
	local := &InboxProcessorModel{}
	global := &GlobalInboxProcessorModel{}

	if !local.markDeletedProcessed() {
		t.Error("local processor must auto-mark deleted tasks processed")
	}
	if !global.markDeletedProcessed() {
		t.Error("global processor must auto-mark deleted tasks processed")
	}

	if !local.shouldProcessOnStateChange(models.StateDone) {
		t.Error("local processor must auto-process on completion")
	}
	if !global.shouldProcessOnStateChange(models.StateDone) {
		t.Error("global processor must auto-process on completion")
	}
	if local.shouldProcessOnStateChange(models.StateReady) {
		t.Error("local processor must not auto-process on a non-terminal status change")
	}
	if global.shouldProcessOnStateChange(models.StateReady) {
		t.Error("global processor must not auto-process on a non-terminal status change")
	}

	project := &models.Project{ID: "p1", Name: "P1"}
	leavingTask := &models.Task{ID: "t-leaving", WorkflowState: models.StateNext}
	stayingTask := &models.Task{ID: "t-staying", WorkflowState: models.StateInbox}
	project.Tasks = []*models.Task{leavingTask, stayingTask}

	if !local.shouldProcessOnSectionChange(leavingTask, project) {
		t.Error("local processor must auto-process when a task leaves the inbox")
	}
	if !global.shouldProcessOnSectionChange(leavingTask, project) {
		t.Error("global processor must auto-process when a task leaves the inbox")
	}
	if local.shouldProcessOnSectionChange(stayingTask, project) {
		t.Error("local processor must keep tasks still in inbox in the queue")
	}
	if global.shouldProcessOnSectionChange(stayingTask, project) {
		t.Error("global processor must keep tasks still in inbox in the queue")
	}
}
