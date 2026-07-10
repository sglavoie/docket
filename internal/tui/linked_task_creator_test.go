package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// TestLinkedTaskCreatorUsesCalmPlaceholder mirrors the Quick Capture polish
// from session 3: the prompt placeholder must read "Type a task title…"
// rather than the long sigil string that previously bled syntax noise into
// every empty form.
func TestLinkedTaskCreatorUsesCalmPlaceholder(t *testing.T) {
	project, err := models.NewProject("proj-1", "creator", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	sourceTask, err := models.NewTask("task-1", 1, models.TaskTypeTask, "source")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	m := NewLinkedTaskCreatorModel(sourceTask, project, LinkTypeBlocker)

	if got := m.input.Placeholder; got != "Type a task title…" {
		t.Fatalf("placeholder = %q, want calm 'Type a task title…' (session-3 Quick Capture parity)", got)
	}
}

// TestLinkedTaskCreatorViewHidesSigilHintByDefault ensures the always-visible
// `^type · !N · #tag · …` strip removed in a prior session does not re-render.
// The contextual app-shell footer instead advertises `? syntax` for users who
// want the full sigil reference, matching Quick Capture. The footer now lives
// in AppModel.View() (the inline child footer was removed this session), so the
// assertion is exercised through the composed app view, not the child View().
func TestLinkedTaskCreatorViewHidesSigilHintByDefault(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 80
	app.height = 24
	sourceTask := project.Tasks[0]

	handled, _ := app.handleTaskMessages(CreateLinkedTaskMsg{SourceTask: sourceTask, Project: project, LinkType: LinkTypeBlocker})
	if !handled {
		t.Fatal("CreateLinkedTaskMsg should be handled")
	}
	if app.currentView() != ViewLinkedTaskCreator {
		t.Fatalf("currentView should be LinkedTaskCreator, got %v", app.currentView())
	}

	plain := ansi.Strip(app.View().Content)

	// Legacy always-visible strip (a representative slice) must NOT appear.
	if strings.Contains(plain, "^type · !N · #tag") {
		t.Fatalf("expected always-visible sigil hint to be gone, but found it:\n%s", plain)
	}
	// The composed app-shell footer must advertise the gated syntax toggle.
	if !strings.Contains(plain, "? syntax") {
		t.Fatalf("expected '? syntax' in app-shell footer, got:\n%s", plain)
	}
	// Syntax guide content must be hidden by default.
	if strings.Contains(plain, "Capture Syntax") {
		t.Fatalf("expected syntax guide to be hidden until toggled, got:\n%s", plain)
	}
}

// TestLinkedTaskCreatorTogglesSyntaxGuideOnQuestionMark verifies the `?`
// toggle reveals the shared syntax guide. The guide is rendered via the
// `quickCaptureInput.renderSyntaxGuide` method lifted to the embedded type in
// this session so both Quick Capture and the linked-task creator share it.
func TestLinkedTaskCreatorTogglesSyntaxGuideOnQuestionMark(t *testing.T) {
	project, err := models.NewProject("proj-1", "creator", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	sourceTask, err := models.NewTask("task-1", 1, models.TaskTypeTask, "source")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	m := NewLinkedTaskCreatorModel(sourceTask, project, LinkTypeBlocker)
	m.SetSize(120, 30)

	updated, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = updated.(*LinkedTaskCreatorModel)
	if !m.helpExpanded {
		t.Fatalf("expected helpExpanded to flip true after '?' key")
	}

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Capture Syntax") {
		t.Fatalf("expected syntax guide header after toggle, got:\n%s", plain)
	}
	// Linked task creator skips %project sigil — the guide should not advertise it.
	if strings.Contains(plain, "%project") {
		t.Fatalf("linked task creator should not show %%project syntax (cross-project capture disabled), got:\n%s", plain)
	}

	// Toggle off should hide it again.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = updated.(*LinkedTaskCreatorModel)
	if m.helpExpanded {
		t.Fatalf("expected helpExpanded to flip false after second '?' key")
	}
	plain = ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Capture Syntax") {
		t.Fatalf("expected syntax guide to be hidden after second toggle, got:\n%s", plain)
	}
}

func TestLinkedTaskCreatorSubtaskAppendsAsLastChild(t *testing.T) {
	project, err := models.NewProject("proj-1", "creator", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	parent, err := models.NewTask("parent-1", 1, models.TaskTypeStory, "parent")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	childOne, err := models.NewTask("child-1", 2, models.TaskTypeTask, "child one")
	if err != nil {
		t.Fatalf("NewTask child one: %v", err)
	}
	childOne.ParentID = &parent.ID

	project.Tasks = append(project.Tasks, parent, childOne)
	project.BuildTaskTree()

	m := NewLinkedTaskCreatorModel(parent, project, LinkTypeSubtask)
	m.input.SetValue("child two")

	cmd := m.createTask(false, false)
	if cmd == nil {
		t.Fatal("expected createTask to return a command")
	}
	msg, ok := cmd().(LinkedTaskCreatedMsg)
	if !ok {
		t.Fatalf("expected LinkedTaskCreatedMsg, got %T", cmd())
	}
	if msg.NewTask == nil {
		t.Fatal("expected created subtask in message")
	}

	refreshedParent := project.GetTaskByID(parent.ID)
	if refreshedParent == nil {
		t.Fatal("expected parent task after save")
	}
	if len(refreshedParent.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(refreshedParent.Children))
	}
	if refreshedParent.Children[0].Title != "child one" {
		t.Fatalf("first child = %q, want %q", refreshedParent.Children[0].Title, "child one")
	}
	if refreshedParent.Children[1].Title != "child two" {
		t.Fatalf("last child = %q, want %q", refreshedParent.Children[1].Title, "child two")
	}
}
