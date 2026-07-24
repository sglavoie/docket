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

func TestPrimaryViewKey_OpenOutliner_NoOutlinerBindsCtrlP(t *testing.T) {
	// ProjectTree and the Ctrl+P project drilldown were retired: neither the
	// single-project nor the cross-project outliner binds ctrl+p. Project scope is
	// entered through Ctrl+X and the project-header scope actions.
	proj := &models.Project{ID: "p", Name: "P"}
	single := minimalOutliner()
	single.project = proj
	single.rows = []outlinerRow{{IsSectionHeader: true, SectionName: models.SectionInbox}}
	single.cursor = 0

	cross := minimalOutliner()

	for name, m := range map[string]*OutlinerModel{"single-project": single, "cross-project": cross} {
		entries := newOutlinerKeymapBuilder(m).primaryViewEntries()
		for _, e := range entries {
			if e.Binding != nil && e.Binding.Key == "ctrl+p" {
				t.Fatalf("%s outliner should not bind ctrl+p, got %q", name, e.Binding.Description)
			}
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

func TestPrimaryViewKey_InboxProcessor_CrossOnSectionHeaderSetsStatus(t *testing.T) {
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

func TestPrimaryViewKey_EditLastModified_CrossOnSectionHeaderSetsStatus(t *testing.T) {
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

func TestGotoKey_ViewTaskActivity_CrossOnSectionHeaderSetsStatus(t *testing.T) {
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

type directProjectKeyCase struct {
	name    string
	invoke  func(t *testing.T, m *OutlinerModel) tea.Msg
	project func(t *testing.T, msg tea.Msg) *models.Project
}

func directProjectKeyCases() []directProjectKeyCase {
	return []directProjectKeyCase{
		{
			name:   "B (Project Blockers)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "B") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowBlockersMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowBlockersMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "W (Inbox Processor)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "W") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowInboxProcessorMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowInboxProcessorMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "i (Quick Capture)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "i") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowQuickCaptureMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowQuickCaptureMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "I (Bulk Capture)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "I") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowBulkCaptureMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowBulkCaptureMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "{ (Open Notes)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "{") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowNotesMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowNotesMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "} (Create Note)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "}") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowNoteCreatorMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowNoteCreatorMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "gl (Project Activity)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeGotoChild(t, m, "l") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowProjectRecentActivityMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowProjectRecentActivityMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "ctrl+l (Edit Last Modified)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokePrimaryViewKey(t, m, "ctrl+l") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				edit, ok := msg.(EditTaskMsg)
				if !ok {
					t.Fatalf("message = %T, want EditTaskMsg", msg)
				}
				return edit.Project
			},
		},
		{
			name:   "ctrl+a (Archive Project)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+a") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				request, ok := msg.(ProjectArchiveRequestMsg)
				if !ok {
					t.Fatalf("message = %T, want ProjectArchiveRequestMsg", msg)
				}
				return request.Project
			},
		},
		{
			name:   "ctrl+s (Section Manager)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+s") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				show, ok := msg.(ShowSectionManagerMsg)
				if !ok {
					t.Fatalf("message = %T, want ShowSectionManagerMsg", msg)
				}
				return show.Project
			},
		},
		{
			name:   "ctrl+e (Export Project)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeUtilityKey(t, m, "ctrl+e") },
			project: func(t *testing.T, msg tea.Msg) *models.Project {
				t.Helper()
				export, ok := msg.(ExportProjectMsg)
				if !ok {
					t.Fatalf("message = %T, want ExportProjectMsg", msg)
				}
				return export.Project
			},
		},
	}
}

func prepareDirectProjectKeyFixture(t *testing.T, scopedEmpty bool) (*OutlinerModel, *models.Project) {
	t.Helper()
	m, _, betaHeader, _, _ := crossProjectHeaderFixture(t)
	config := newTestConfig()
	setTestProjects(config, m.loadedProjects)
	m.runtime = testRuntimeAt(config.DataDir(), config)

	target := m.crossResolveProject(m.rows[betaHeader].ProjectID)
	if target == nil {
		t.Fatal("fixture beta project did not resolve")
	}
	if scopedEmpty {
		m.scopedProjectID = target.ID
		m.rows = nil
		m.cursor = 0
	} else {
		m.cursor = betaHeader
	}
	return m, target
}

func TestOutlinerProjectHeaderDirectKeysTargetHeaderProject(t *testing.T) {
	for _, tc := range directProjectKeyCases() {
		t.Run(tc.name, func(t *testing.T) {
			m, target := prepareDirectProjectKeyFixture(t, false)
			if got := tc.project(t, tc.invoke(t, m)); got != target {
				t.Fatalf("project = %v, want highlighted header project %v", got, target)
			}
		})
	}
}

func TestOutlinerProjectContextScopedEmptyDirectKeysTargetScope(t *testing.T) {
	for _, tc := range directProjectKeyCases() {
		t.Run(tc.name, func(t *testing.T) {
			m, target := prepareDirectProjectKeyFixture(t, true)
			if got := tc.project(t, tc.invoke(t, m)); got != target {
				t.Fatalf("project = %v, want scoped project %v", got, target)
			}
		})
	}
}

func TestOutlinerProjectContextCompletedArchiveAndPasteAcceptProjectContext(t *testing.T) {
	for _, scopedEmpty := range []bool{false, true} {
		name := "project header"
		if scopedEmpty {
			name = "scoped empty view"
		}
		t.Run(name, func(t *testing.T) {
			t.Run("completed archive", func(t *testing.T) {
				m, target := prepareDirectProjectKeyFixture(t, scopedEmpty)
				target.Tasks[0].WorkflowState = models.StateDone
				m.activePerspective = PerspectiveCompleted

				if msg := invokeUtilityKey(t, m, "A"); msg != nil {
					t.Fatalf("archive completed message = %T, want nil", msg)
				}
				if m.archiveTargetProject != target {
					t.Fatalf("archive target = %v, want %v", m.archiveTargetProject, target)
				}
				if !m.confirm.IsActive() || m.confirm.Action() != "archive-tasks" {
					t.Fatal("archive completed did not open the expected confirmation")
				}
			})

			t.Run("clipboard paste", func(t *testing.T) {
				m, _ := prepareDirectProjectKeyFixture(t, scopedEmpty)
				action := findTopLevelBinding(t, newOutlinerKeymapBuilder(m).utilityEntries(), "ctrl+v")
				_, cmd := action()
				if cmd == nil {
					t.Fatal("clipboard paste rejected valid project context")
				}
				if m.statusMessage == statusNoProjectUnderCursor {
					t.Fatalf("status = %q for valid project context", m.statusMessage)
				}
			})
		})
	}
}

func TestOutlinerProjectHeaderTaskOnlyBindingsRejectHeader(t *testing.T) {
	type taskOnlyCase struct {
		name   string
		invoke func(t *testing.T, m *OutlinerModel) tea.Msg
	}
	cases := []taskOnlyCase{
		{
			name:   "gh (Task Activity)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeGotoChild(t, m, "h") },
		},
		{
			name:   "o (Task Detail)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeCommonTaskActionKey(t, m, "o") },
		},
		{
			name:   "ctrl+b (Move Task to Project)",
			invoke: func(t *testing.T, m *OutlinerModel) tea.Msg { return invokeCommonTaskActionKey(t, m, "ctrl+b") },
		},
		{
			name: "tn (Create Child Task)",
			invoke: func(_ *testing.T, m *OutlinerModel) tea.Msg {
				_, cmd := m.handleTaskPrefix("n", nil)
				if cmd == nil {
					return nil
				}
				return cmd()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _, betaHeader, _, _ := crossProjectHeaderFixture(t)
			m.cursor = betaHeader
			if msg := tc.invoke(t, m); msg != nil {
				t.Fatalf("task-only binding emitted %T for project header", msg)
			}
			if task, project := m.getTaskRef(); task != nil || project != nil {
				t.Fatalf("getTaskRef() = (%v, %v), want (nil, nil)", task, project)
			}
		})
	}
}
