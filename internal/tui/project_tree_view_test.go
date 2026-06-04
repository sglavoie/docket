package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func newTestProjectTreeConfig() *config.Config {
	return &config.Config{
		Projects:          []*models.Project{},
		ProjectViewStates: make(map[string]config.ProjectViewState),
		SectionColors:     make(map[string]string),
	}
}

func newTestProjectTreeProject(t *testing.T, name string, tasks []*models.Task) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		t.Fatal(err)
	}
	p.Tasks = tasks
	return p
}

func newTestProjectTreeTask(t *testing.T, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestProjectTreeView_BasicRender(t *testing.T) {
	task1 := newTestProjectTreeTask(t, "First task")
	task2 := newTestProjectTreeTask(t, "Second task")
	project := newTestProjectTreeProject(t, "Test Project", []*models.Task{task1, task2})
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)
	m.SetSize(80, 24)

	output := m.View().Content
	if !strings.Contains(output, "First task") {
		t.Error("expected output to contain 'First task'")
	}
	if !strings.Contains(output, "Second task") {
		t.Error("expected output to contain 'Second task'")
	}
}

func TestProjectTreeView_EmptyProject(t *testing.T) {
	project := newTestProjectTreeProject(t, "Empty Project", nil)
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)
	m.SetSize(80, 24)

	output := m.View().Content
	if output == "" {
		t.Error("expected non-empty output even for empty project")
	}
}

func TestProjectTreeView_CursorBounds(t *testing.T) {
	project := newTestProjectTreeProject(t, "Empty", nil)
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)
	m.cursor = 100
	m.clampCursor()

	if m.cursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.cursor)
	}
}

func TestViewModeCycleIncludesAll(t *testing.T) {
	// Verify that cycling view modes wraps correctly through all modes,
	// including ViewModeAll (the last one).
	seen := make(map[ViewMode]bool)
	mode := ViewModeActive
	for range int(ViewModeAll) + 2 { // One full cycle + 1 to verify wrap
		seen[mode] = true
		mode = (mode + 1) % (ViewModeAll + 1)
	}
	for vm := ViewModeActive; vm <= ViewModeAll; vm++ {
		if !seen[vm] {
			t.Errorf("view mode %d never reached during cycling", vm)
		}
	}
	// After a full cycle, we should be back at Active
	mode = ViewModeAll
	mode = (mode + 1) % (ViewModeAll + 1)
	if mode != ViewModeActive {
		t.Errorf("expected wrap to ViewModeActive (0), got %d", mode)
	}
}

func TestProjectTreeCyclableViewModesInboxFirst(t *testing.T) {
	project := newTestProjectTreeProject(t, "Test", nil)
	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)

	got := m.cyclableViewModes()
	want := []ViewMode{ViewModeInbox, ViewModeActive, ViewModeNext, ViewModeWaiting, ViewModeSomeday}
	if len(got) != len(want) {
		t.Fatalf("expected modes %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mode[%d]: want %v, got %v", i, want[i], got[i])
		}
	}
}

func TestProjectTreeViewModeIndicatorRendersInboxFirst(t *testing.T) {
	active := newTestProjectTreeTask(t, "Active task")
	inbox := newTestProjectTreeTask(t, "Inbox task")
	inbox.Section = models.SectionInbox
	next := newTestProjectTreeTask(t, "Next task")
	next.Section = models.SectionNext

	project := newTestProjectTreeProject(t, "Test", []*models.Task{active, inbox, next})
	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.SetSize(120, 24)

	got := ansi.Strip(m.renderViewModeIndicator())
	inboxIndex := strings.Index(got, "Inbox[1]")
	activeIndex := strings.Index(got, "Active[1]")
	nextIndex := strings.Index(got, "Next[1]")
	if inboxIndex < 0 || activeIndex < 0 || nextIndex < 0 {
		t.Fatalf("expected Inbox, Active, and Next counts in %q", got)
	}
	if !(inboxIndex < activeIndex && activeIndex < nextIndex) {
		t.Fatalf("expected Inbox before Active before Next, got %q", got)
	}
}

func TestProjectTreeView_FilterState(t *testing.T) {
	project := newTestProjectTreeProject(t, "Test", nil)
	cfg := newTestProjectTreeConfig()

	m := NewProjectTreeModel(project, cfg, nil)

	if m.filter.Type != "" {
		t.Errorf("expected empty type filter, got %q", m.filter.Type)
	}
	if m.filter.Status != "" {
		t.Errorf("expected empty status filter, got %q", m.filter.Status)
	}
	if m.filter.Priority != 0 {
		t.Errorf("expected 0 priority filter, got %d", m.filter.Priority)
	}
}

func TestNavigateToTaskClearsStaleCustomSectionTab(t *testing.T) {
	activeTask := newTestProjectTreeTask(t, "Active task")
	customTask := newTestProjectTreeTask(t, "Custom task")
	customTask.SetSection("Sprint-1")

	project := newTestProjectTreeProject(t, "Test Project", []*models.Task{activeTask, customTask})
	project.EnsureSection("Sprint-1")
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.customSectionTab = 1
	m.updateVisibility()

	cmd := m.NavigateToTask(activeTask)

	if m.customSectionTab != 0 {
		t.Fatalf("expected custom section tab reset to All, got %d", m.customSectionTab)
	}
	if m.viewMode != ViewModeActive {
		t.Fatalf("expected Active view, got %v", m.viewMode)
	}
	visible := m.visibleNodes()
	if len(visible) == 0 || visible[m.cursor].Task.ID != activeTask.ID {
		t.Fatalf("expected cursor on active task, got %#v", visible)
	}
	if cmd == nil {
		t.Fatal("expected status command when clearing a stale custom section tab")
	}
	if msg := cmd(); msg != nil {
		if _, ok := msg.(InfoStatusMsg); !ok {
			t.Fatalf("expected InfoStatusMsg, got %T", msg)
		}
	} else {
		t.Fatal("expected InfoStatusMsg, got nil")
	}
}

func TestNavigateToTaskCustomSectionUsesAllViewAndMatchingTab(t *testing.T) {
	activeTask := newTestProjectTreeTask(t, "Active task")
	sprintOneTask := newTestProjectTreeTask(t, "Sprint one")
	sprintOneTask.SetSection("Sprint-1")
	sprintTwoTask := newTestProjectTreeTask(t, "Sprint two")
	sprintTwoTask.SetSection("Sprint-2")

	project := newTestProjectTreeProject(t, "Test Project", []*models.Task{activeTask, sprintOneTask, sprintTwoTask})
	project.EnsureSection("Sprint-1")
	project.EnsureSection("Sprint-2")
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	m.viewMode = ViewModeInbox
	m.customSectionTab = 2
	m.updateVisibility()

	cmd := m.NavigateToTask(sprintOneTask)

	if m.viewMode != ViewModeAll {
		t.Fatalf("expected All view for custom-section navigation, got %v", m.viewMode)
	}
	if m.customSectionTab != 1 {
		t.Fatalf("expected Sprint-1 tab index 1, got %d", m.customSectionTab)
	}
	visible := m.visibleNodes()
	if len(visible) != 1 || visible[0].Task.ID != sprintOneTask.ID {
		t.Fatalf("expected only Sprint-1 task visible, got %#v", visible)
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor at first visible row, got %d", m.cursor)
	}
	if cmd == nil {
		t.Fatal("expected status command when changing custom section filters")
	}
	if msg := cmd(); msg != nil {
		if _, ok := msg.(InfoStatusMsg); !ok {
			t.Fatalf("expected InfoStatusMsg, got %T", msg)
		}
	} else {
		t.Fatal("expected InfoStatusMsg, got nil")
	}
}

func TestNavigateToTaskShowsSnoozedTasks(t *testing.T) {
	snoozedTask := newTestProjectTreeTask(t, "Snoozed task")
	snoozedUntil := time.Now().Add(24 * time.Hour)
	snoozedTask.SnoozedUntil = &snoozedUntil

	project := newTestProjectTreeProject(t, "Test Project", []*models.Task{snoozedTask})
	project.BuildTaskTree()

	m := NewProjectTreeModel(project, newTestProjectTreeConfig(), nil)
	if len(m.visibleNodes()) != 0 {
		t.Fatalf("expected snoozed task hidden from default view, got %#v", m.visibleNodes())
	}

	if cmd := m.NavigateToTask(snoozedTask); cmd != nil {
		t.Fatalf("expected no status command without active filters, got %T", cmd())
	}
	if m.viewMode != ViewModeSnoozed {
		t.Fatalf("expected Snoozed view, got %v", m.viewMode)
	}
	visible := m.visibleNodes()
	if len(visible) != 1 || visible[0].Task.ID != snoozedTask.ID {
		t.Fatalf("expected snoozed task visible, got %#v", visible)
	}
}
