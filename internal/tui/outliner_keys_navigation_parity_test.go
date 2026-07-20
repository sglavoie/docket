package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// crossOutlinerWithTwoProjects builds a minimal cross-project OutlinerModel
// whose rows alternate between two projects (separated by a row separator
// and bracketed by section headers). Sessions 2-4 reuse this scaffold for
// cursor-project routing tests.
func crossOutlinerWithTwoProjects(t *testing.T) (*OutlinerModel, *models.Project, *models.Project, *models.Task, *models.Task) {
	t.Helper()

	projA := &models.Project{ID: "proj-A", Name: "A"}
	projB := &models.Project{ID: "proj-B", Name: "B"}
	taskA := makeTask("ta", "TA", models.SectionInbox)
	taskA.UpdatedAt = time.Now().Add(-1 * time.Hour)
	taskB := makeTask("tb", "TB", models.SectionInbox)
	taskB.UpdatedAt = time.Now()
	projA.Tasks = []*models.Task{taskA}
	projB.Tasks = []*models.Task{taskB}

	m := minimalOutliner()
	m.loadedProjects = []*models.Project{projA, projB}
	m.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projA.ID},
		{task: taskA, ProjectID: projA.ID},
		{IsSeparator: true},
		{IsSectionHeader: true, SectionName: models.SectionInbox, ProjectID: projB.ID},
		{task: taskB, ProjectID: projB.ID},
	}
	return m, projA, projB, taskA, taskB
}

// findTopLevelBinding returns the action for a top-level (non-prefix) entry
// keyed by `key`. Fails the test when the entry is missing or is a prefix.
func findTopLevelBinding(t *testing.T, entries []keymap.Entry, key string) keymap.Action {
	t.Helper()
	for _, e := range entries {
		if e.Binding != nil && e.Binding.Key == key {
			return e.Binding.Action
		}
	}
	t.Fatalf("entry %q not found among %d top-level entries", key, len(entries))
	return nil
}

// findPrefixChildBinding returns the action for a child entry of the prefix
// keyed by `prefix`. Fails the test when either the prefix or child is missing.
func findPrefixChildBinding(t *testing.T, entries []keymap.Entry, prefix, childKey string) keymap.Action {
	t.Helper()
	for _, e := range entries {
		if e.PrefixNode != nil && e.PrefixNode.Key == prefix {
			for _, c := range e.PrefixNode.Children {
				if c.Binding != nil && c.Binding.Key == childKey {
					return c.Binding.Action
				}
			}
			t.Fatalf("child %q not found under prefix %q", childKey, prefix)
		}
	}
	t.Fatalf("prefix %q not found", prefix)
	return nil
}

func runEntryAction(t *testing.T, action keymap.Action) tea.Msg {
	t.Helper()
	if action == nil {
		t.Fatal("action is nil")
	}
	_, cmd := action()
	if cmd == nil {
		return nil
	}
	return cmd()
}

// invokePrimaryViewKey resolves and executes the top-level primary-view
// binding for `key` against the given outliner model, returning the emitted
// message (nil when the action emitted no command).
func invokePrimaryViewKey(t *testing.T, m *OutlinerModel, key string) tea.Msg {
	t.Helper()
	b := newOutlinerKeymapBuilder(m)
	entries := b.primaryViewEntries()
	return runEntryAction(t, findTopLevelBinding(t, entries, key))
}

// invokeGotoChild resolves and executes the `g`-prefix child binding for
// `childKey`, returning the emitted message.
func invokeGotoChild(t *testing.T, m *OutlinerModel, childKey string) tea.Msg {
	t.Helper()
	b := newOutlinerKeymapBuilder(m)
	entries := b.gotoEntries()
	return runEntryAction(t, findPrefixChildBinding(t, entries, "g", childKey))
}

func TestPrimaryViewKey_OpenOutliner_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithTwoProjects(t)

	m.cursor = 1 // task in projA
	msg := invokePrimaryViewKey(t, m, "ctrl+p")
	show, ok := msg.(ShowOutlinerMsg)
	if !ok {
		t.Fatalf("expected ShowOutlinerMsg, got %T", msg)
	}
	if show.Project != projA {
		t.Fatalf("Project = %v, want %v", show.Project, projA)
	}

	m.cursor = 4 // task in projB
	msg = invokePrimaryViewKey(t, m, "ctrl+p")
	show, ok = msg.(ShowOutlinerMsg)
	if !ok {
		t.Fatalf("expected ShowOutlinerMsg, got %T", msg)
	}
	if show.Project != projB {
		t.Fatalf("Project = %v, want %v", show.Project, projB)
	}
}

func TestPrimaryViewKey_OpenOutliner_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0 // header row

	if msg := invokePrimaryViewKey(t, m, "ctrl+p"); msg != nil {
		t.Fatalf("expected nil message on header, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestPrimaryViewKey_OpenOutliner_SingleProjectHasNoCtrlP(t *testing.T) {
	// ProjectTree was retired: the single-project outliner is the only per-project
	// view, so it no longer binds a no-op ctrl+p "toggle". The binding survives
	// only on the cross-project outliner as a meaningful drill-down.
	proj := &models.Project{ID: "p", Name: "P"}
	m := minimalOutliner()
	m.project = proj
	m.rows = []outlinerRow{{IsSectionHeader: true, SectionName: models.SectionInbox}}
	m.cursor = 0

	entries := newOutlinerKeymapBuilder(m).primaryViewEntries()
	for _, e := range entries {
		if e.Binding != nil && e.Binding.Key == "ctrl+p" {
			t.Fatalf("single-project outliner should not bind ctrl+p, got %q", e.Binding.Description)
		}
	}
}

func TestPrimaryViewKey_ProjectBlockers_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokePrimaryViewKey(t, m, "B")
	show, ok := msg.(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", msg)
	}
	if show.Project != projA {
		t.Fatalf("Project = %v, want %v", show.Project, projA)
	}

	m.cursor = 4
	msg = invokePrimaryViewKey(t, m, "B")
	show, _ = msg.(ShowBlockersMsg)
	if show.Project != projB {
		t.Fatalf("Project = %v, want %v", show.Project, projB)
	}
}

func TestPrimaryViewKey_ProjectBlockers_CrossOnSeparatorSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 2 // separator row

	if msg := invokePrimaryViewKey(t, m, "B"); msg != nil {
		t.Fatalf("expected nil message on separator, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestPrimaryViewKey_InboxProcessor_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokePrimaryViewKey(t, m, "W")
	show, ok := msg.(ShowInboxProcessorMsg)
	if !ok {
		t.Fatalf("expected ShowInboxProcessorMsg, got %T", msg)
	}
	if show.Project != projA {
		t.Fatalf("Project = %v, want %v", show.Project, projA)
	}

	m.cursor = 4
	msg = invokePrimaryViewKey(t, m, "W")
	show, _ = msg.(ShowInboxProcessorMsg)
	if show.Project != projB {
		t.Fatalf("Project = %v, want %v", show.Project, projB)
	}
}

func TestPrimaryViewKey_InboxProcessor_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0

	if msg := invokePrimaryViewKey(t, m, "W"); msg != nil {
		t.Fatalf("expected nil message on header, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestPrimaryViewKey_CreateNote_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokePrimaryViewKey(t, m, "}")
	show, ok := msg.(ShowNoteCreatorMsg)
	if !ok {
		t.Fatalf("expected ShowNoteCreatorMsg, got %T", msg)
	}
	if show.Project != projA {
		t.Fatalf("Project = %v, want %v", show.Project, projA)
	}

	m.cursor = 4
	msg = invokePrimaryViewKey(t, m, "}")
	show, _ = msg.(ShowNoteCreatorMsg)
	if show.Project != projB {
		t.Fatalf("Project = %v, want %v", show.Project, projB)
	}
}

func TestPrimaryViewKey_CreateNote_CrossOnSeparatorSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 2

	if msg := invokePrimaryViewKey(t, m, "}"); msg != nil {
		t.Fatalf("expected nil message on separator, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestPrimaryViewKey_EditLastModified_CrossUsesCursorProjectTask(t *testing.T) {
	m, projA, projB, taskA, taskB := crossOutlinerWithTwoProjects(t)

	// Cursor on projA's task → edit projA's most recent task.
	m.cursor = 1
	msg := invokePrimaryViewKey(t, m, "ctrl+l")
	edit, ok := msg.(EditTaskMsg)
	if !ok {
		t.Fatalf("expected EditTaskMsg, got %T", msg)
	}
	if edit.Project != projA {
		t.Fatalf("Project = %v, want %v", edit.Project, projA)
	}
	if edit.Task != taskA {
		t.Fatalf("Task = %v, want %v", edit.Task, taskA)
	}

	// Cursor on projB's task → edit projB's most recent task.
	m.cursor = 4
	msg = invokePrimaryViewKey(t, m, "ctrl+l")
	edit, ok = msg.(EditTaskMsg)
	if !ok {
		t.Fatalf("expected EditTaskMsg, got %T", msg)
	}
	if edit.Project != projB {
		t.Fatalf("Project = %v, want %v", edit.Project, projB)
	}
	if edit.Task != taskB {
		t.Fatalf("Task = %v, want %v", edit.Task, taskB)
	}
}

func TestPrimaryViewKey_EditLastModified_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0

	if msg := invokePrimaryViewKey(t, m, "ctrl+l"); msg != nil {
		t.Fatalf("expected nil message on header, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}

func TestGotoKey_ViewTaskActivity_CrossUsesCursorTaskProject(t *testing.T) {
	m, projA, projB, taskA, taskB := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokeGotoChild(t, m, "h")
	act, ok := msg.(OpenTaskActivityMsg)
	if !ok {
		t.Fatalf("expected OpenTaskActivityMsg, got %T", msg)
	}
	if act.Project != projA || act.Task != taskA {
		t.Fatalf("got (%v, %v), want (%v, %v)", act.Task, act.Project, taskA, projA)
	}

	m.cursor = 4
	msg = invokeGotoChild(t, m, "h")
	act, _ = msg.(OpenTaskActivityMsg)
	if act.Project != projB || act.Task != taskB {
		t.Fatalf("got (%v, %v), want (%v, %v)", act.Task, act.Project, taskB, projB)
	}
}

func TestGotoKey_ViewTaskActivity_CrossOnHeaderSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 0

	if msg := invokeGotoChild(t, m, "h"); msg != nil {
		t.Fatalf("expected nil message on header, got %T", msg)
	}
	if m.statusMessage != statusNoTaskSelected {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoTaskSelected)
	}
}

func TestGotoKey_ProjectRecentActivity_CrossUsesCursorProject(t *testing.T) {
	m, projA, projB, _, _ := crossOutlinerWithTwoProjects(t)

	m.cursor = 1
	msg := invokeGotoChild(t, m, "l")
	show, ok := msg.(ShowProjectRecentActivityMsg)
	if !ok {
		t.Fatalf("expected ShowProjectRecentActivityMsg, got %T", msg)
	}
	if show.Project != projA {
		t.Fatalf("Project = %v, want %v", show.Project, projA)
	}

	m.cursor = 4
	msg = invokeGotoChild(t, m, "l")
	show, _ = msg.(ShowProjectRecentActivityMsg)
	if show.Project != projB {
		t.Fatalf("Project = %v, want %v", show.Project, projB)
	}
}

func TestGotoKey_ProjectRecentActivity_CrossOnSeparatorSetsStatus(t *testing.T) {
	m, _, _, _, _ := crossOutlinerWithTwoProjects(t)
	m.cursor = 2

	if msg := invokeGotoChild(t, m, "l"); msg != nil {
		t.Fatalf("expected nil message on separator, got %T", msg)
	}
	if m.statusMessage != statusNoProjectUnderCursor {
		t.Fatalf("status = %q, want %q", m.statusMessage, statusNoProjectUnderCursor)
	}
}
