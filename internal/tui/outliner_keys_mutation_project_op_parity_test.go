package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// invokeUtilityKey resolves and executes the top-level utility-entry binding
// for `key` against the given outliner model, returning the emitted message.
func invokeUtilityKey(t *testing.T, m *OutlinerModel, key string) tea.Msg {
	t.Helper()
	b := newOutlinerKeymapBuilder(m)
	entries := b.utilityEntries()
	return runEntryAction(t, findTopLevelBinding(t, entries, key))
}

// invokeCommonTaskActionKey resolves and executes the top-level
// commonTaskAction-entry binding for `key`, returning the emitted message.
func invokeCommonTaskActionKey(t *testing.T, m *OutlinerModel, key string) tea.Msg {
	t.Helper()
	b := newOutlinerKeymapBuilder(m)
	entries := b.commonTaskActionEntries()
	return runEntryAction(t, findTopLevelBinding(t, entries, key))
}

func TestUtilityKey_AllTasks_CrossNonCompletedEmitsShowAllTasks(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	// activePerspective defaults to zero (not PerspectiveCompleted).
	m.cursor = 1 // task row
	msg := invokeUtilityKey(t, m, "A")
	if _, ok := msg.(ShowAllTasksMsg); !ok {
		t.Fatalf("expected ShowAllTasksMsg, got %T", msg)
	}
}

func TestUtilityKey_AllTasks_CrossNonCompletedHeaderStillEmits(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0 // header row - non-Completed perspective still emits ShowAllTasksMsg
	msg := invokeUtilityKey(t, m, "A")
	if _, ok := msg.(ShowAllTasksMsg); !ok {
		t.Fatalf("expected ShowAllTasksMsg, got %T", msg)
	}
}

func TestUtilityKey_ArchiveCompleted_CrossUsesCursorProject(t *testing.T) {
	m, projA, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.activePerspective = PerspectiveCompleted
	// Give projA at least one completed task so the prompt shows.
	completed := makeTask("done-a", "DoneA", models.SectionCompleted)
	completed.Status = models.StatusDone
	projA.Tasks = append(projA.Tasks, completed)
	m.cursor = 1 // task in projA

	if msg := invokeUtilityKey(t, m, "A"); msg != nil {
		t.Fatalf("expected nil cmd on archive prompt, got %T", msg)
	}
	if !m.confirm.IsActive() {
		t.Fatal("expected confirm dialog to be active after archive-tasks prompt")
	}
	if m.confirm.Action() != "archive-tasks" {
		t.Fatalf("confirm.Action = %q, want %q", m.confirm.Action(), "archive-tasks")
	}
	if m.archiveTargetProject != projA {
		t.Fatalf("archiveTargetProject = %v, want %v", m.archiveTargetProject, projA)
	}
}

func TestUtilityKey_ArchiveCompleted_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.activePerspective = PerspectiveCompleted
	m.cursor = 0 // header

	if msg := invokeUtilityKey(t, m, "A"); msg != nil {
		t.Fatalf("expected nil cmd on header, got %T", msg)
	}
	if m.confirm.IsActive() {
		t.Fatal("confirm should not be active on header row")
	}
	if m.archiveTargetProject != nil {
		t.Fatalf("archiveTargetProject = %v, want nil", m.archiveTargetProject)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestUtilityKey_ArchiveCompleted_CrossWithNoCompletedTasksDoesNotPrompt(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.activePerspective = PerspectiveCompleted
	m.cursor = 1 // task row, but projA has zero completed tasks

	if msg := invokeUtilityKey(t, m, "A"); msg != nil {
		t.Fatalf("expected nil cmd, got %T", msg)
	}
	if m.confirm.IsActive() {
		t.Fatal("confirm should not be active when project has no completed tasks")
	}
}

func TestUtilityKey_ArchiveCompleted_SingleProjectUnchanged(t *testing.T) {
	proj := &models.Project{ID: "p", Name: "P"}
	completed := makeTask("done", "Done", models.SectionCompleted)
	completed.Status = models.StatusDone
	proj.Tasks = []*models.Task{completed}
	m := minimalOutliner()
	m.project = proj
	m.activePerspective = PerspectiveCompleted
	m.rows = []outlinerRow{{IsSectionHeader: true, SectionName: models.SectionCompleted}}
	m.cursor = 0 // header in single mode still routes via m.project

	if msg := invokeUtilityKey(t, m, "A"); msg != nil {
		t.Fatalf("expected nil cmd, got %T", msg)
	}
	if !m.confirm.IsActive() {
		t.Fatal("expected confirm dialog active in single mode")
	}
	if m.archiveTargetProject != proj {
		t.Fatalf("archiveTargetProject = %v, want %v", m.archiveTargetProject, proj)
	}
}

func TestUtilityKey_ArchiveProject_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokeUtilityKey(t, m, "ctrl+a")
	req, ok := msg.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("expected ProjectArchiveRequestMsg, got %T", msg)
	}
	if req.Project != projA {
		t.Fatalf("Project = %v, want %v", req.Project, projA)
	}
	if req.Archive != true {
		t.Fatalf("Archive = %v, want true (project starts unarchived)", req.Archive)
	}

	m.cursor = 4
	msg = invokeUtilityKey(t, m, "ctrl+a")
	req, _ = msg.(ProjectArchiveRequestMsg)
	if req.Project != projB {
		t.Fatalf("Project = %v, want %v", req.Project, projB)
	}
}

func TestUtilityKey_ArchiveProject_CrossUnarchives(t *testing.T) {
	m, projA, _, _, _ := crossOutlinerWithTwoProjects(t)
	projA.Archived = true
	m.cursor = 1

	msg := invokeUtilityKey(t, m, "ctrl+a")
	req, ok := msg.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("expected ProjectArchiveRequestMsg, got %T", msg)
	}
	if req.Archive != false {
		t.Fatalf("Archive = %v, want false (project starts archived)", req.Archive)
	}
}

func TestUtilityKey_ArchiveProject_CrossOnSeparatorSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 2 // separator

	if msg := invokeUtilityKey(t, m, "ctrl+a"); msg != nil {
		t.Fatalf("expected nil msg on separator, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestUtilityKey_ArchiveProject_CrossSandboxEmitsError(t *testing.T) {
	m, projA, _, _, _ := crossOutlinerWithTwoProjects(t)
	projA.Sandbox = true
	m.cursor = 1

	msg := invokeUtilityKey(t, m, "ctrl+a")
	if _, ok := msg.(ErrorMsg); !ok {
		t.Fatalf("expected ErrorMsg for sandbox archive, got %T", msg)
	}
}

func TestUtilityKey_PasteClipboard_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0 // header

	if msg := invokeUtilityKey(t, m, "ctrl+v"); msg != nil {
		// Header path must not emit any message.
		t.Fatalf("expected nil msg on header, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestUtilityKey_PasteClipboard_CrossOnSeparatorSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 2 // separator

	if msg := invokeUtilityKey(t, m, "ctrl+v"); msg != nil {
		t.Fatalf("expected nil msg on separator, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestCommonTaskAction_MoveToProject_CrossUsesCursorTaskAndProject(t *testing.T) {
	m, projA, projB, taskA, taskB := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokeCommonTaskActionKey(t, m, "ctrl+b")
	show, ok := msg.(ShowProjectSelectorForMoveMsg)
	if !ok {
		t.Fatalf("expected ShowProjectSelectorForMoveMsg, got %T", msg)
	}
	if show.Task != taskA || show.SourceProject != projA {
		t.Fatalf("got (%v, %v), want (%v, %v)", show.Task, show.SourceProject, taskA, projA)
	}

	m.cursor = 4
	msg = invokeCommonTaskActionKey(t, m, "ctrl+b")
	show, _ = msg.(ShowProjectSelectorForMoveMsg)
	if show.Task != taskB || show.SourceProject != projB {
		t.Fatalf("got (%v, %v), want (%v, %v)", show.Task, show.SourceProject, taskB, projB)
	}
}

func TestCommonTaskAction_MoveToProject_CrossOnHeaderEmitsNothing(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0 // header

	if msg := invokeCommonTaskActionKey(t, m, "ctrl+b"); msg != nil {
		t.Fatalf("expected nil msg on header, got %T", msg)
	}
}

func TestCommonTaskAction_MoveToProject_SingleProjectUnchanged(t *testing.T) {
	proj := &models.Project{ID: "p", Name: "P"}
	task := makeTask("t1", "T1", models.SectionInbox)
	proj.Tasks = []*models.Task{task}
	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{{task: task}}
	m.cursor = 0

	msg := invokeCommonTaskActionKey(t, m, "ctrl+b")
	show, ok := msg.(ShowProjectSelectorForMoveMsg)
	if !ok {
		t.Fatalf("expected ShowProjectSelectorForMoveMsg, got %T", msg)
	}
	if show.Task != task || show.SourceProject != proj {
		t.Fatalf("got (%v, %v), want (%v, %v)", show.Task, show.SourceProject, task, proj)
	}
}

func TestCommonTaskAction_QuickNote_CrossUsesCursorTaskAndProject(t *testing.T) {
	m, projA, projB, taskA, taskB := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokeCommonTaskActionKey(t, m, "ctrl+.")
	open, ok := msg.(OpenQuickNoteMsg)
	if !ok {
		t.Fatalf("expected OpenQuickNoteMsg, got %T", msg)
	}
	if open.Task != taskA || open.Project != projA {
		t.Fatalf("got (%v, %v), want (%v, %v)", open.Task, open.Project, taskA, projA)
	}

	m.cursor = 4
	msg = invokeCommonTaskActionKey(t, m, "ctrl+.")
	open, _ = msg.(OpenQuickNoteMsg)
	if open.Task != taskB || open.Project != projB {
		t.Fatalf("got (%v, %v), want (%v, %v)", open.Task, open.Project, taskB, projB)
	}
}

func TestCommonTaskAction_QuickNote_CrossOnSeparatorEmitsNothing(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 2 // separator

	if msg := invokeCommonTaskActionKey(t, m, "ctrl+."); msg != nil {
		t.Fatalf("expected nil msg on separator, got %T", msg)
	}
}

func TestCommonTaskAction_QuickNote_SingleProjectUnchanged(t *testing.T) {
	proj := &models.Project{ID: "p", Name: "P"}
	task := makeTask("t1", "T1", models.SectionInbox)
	proj.Tasks = []*models.Task{task}
	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{{task: task}}
	m.cursor = 0

	msg := invokeCommonTaskActionKey(t, m, "ctrl+.")
	open, ok := msg.(OpenQuickNoteMsg)
	if !ok {
		t.Fatalf("expected OpenQuickNoteMsg, got %T", msg)
	}
	if open.Task != task || open.Project != proj {
		t.Fatalf("got (%v, %v), want (%v, %v)", open.Task, open.Project, task, proj)
	}
}

// TestArchiveCompleted_ConfirmCancelClearsArchiveTarget verifies that
// declining the archive-tasks confirmation drops m.archiveTargetProject so
// a later confirm from a different cursor row cannot reuse the stale target.
func TestArchiveCompleted_ConfirmCancelClearsArchiveTarget(t *testing.T) {
	m, projA, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.PrimaryViewBundle = NewPrimaryViewBundle(nil, nil)
	m.activePerspective = PerspectiveCompleted
	completed := makeTask("done-a", "DoneA", models.SectionCompleted)
	completed.Status = models.StatusDone
	projA.Tasks = append(projA.Tasks, completed)
	m.cursor = 1

	_ = invokeUtilityKey(t, m, "A")
	if m.archiveTargetProject != projA {
		t.Fatalf("archiveTargetProject = %v, want %v", m.archiveTargetProject, projA)
	}

	// Decline confirmation.
	if _, _ = m.handleConfirmDialog("n"); m.archiveTargetProject != nil {
		t.Fatalf("archiveTargetProject should clear after cancel, got %v", m.archiveTargetProject)
	}
}
