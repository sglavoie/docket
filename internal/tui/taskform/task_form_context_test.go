package taskform

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

func TestTaskFormBasicsTabRendersContextRow(t *testing.T) {
	m := newTestTaskForm()
	m.activeTab = formTabBasics
	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Context:") {
		t.Fatalf("expanded Basics tab lost the Context row:\n%s", plain)
	}
	if !strings.Contains(plain, "(none)") {
		t.Fatalf("an unset context should read (none):\n%s", plain)
	}
}

func TestTaskFormContextFieldIsReachableAndTypable(t *testing.T) {
	m := newTestTaskForm()
	// Tab from the title through every Basics row; the Context row must be the
	// last one reached before the focus wraps.
	for range formBasicsFieldCount - 1 {
		m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	}
	if m.focusIndex != formBasicsContextField {
		t.Fatalf("focus after tabbing the Basics tab = %d, want %d", m.focusIndex, formBasicsContextField)
	}
	for _, r := range "Client-Acme" {
		m = sendFormKey(m, syntheticKey(r))
	}
	if !m.hasChanges {
		t.Error("typing a context should mark the form dirty")
	}
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	if m.draft.context != "Client-Acme" {
		t.Fatalf("draft context after commit = %q", m.draft.context)
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, "Client-Acme") {
		t.Fatalf("committed context is not visible on the unfocused row:\n%s", plain)
	}
}

// TestTaskFormContextRowStaysReachableAtMinimumSize keeps the last Basics row
// honest at the floor the form now renders at. The form's keys are the shell
// footer's, and at 60 columns that footer spends a second line the form is told
// about and pays for, so the body is windowed against the frame that is left and
// every row stays reachable: tabbing to Context brings it on screen.
func TestTaskFormContextRowStaysReachableAtMinimumSize(t *testing.T) {
	m := newTestTaskForm()
	m.SetSize(formMinWidth, formMinHeight)
	guidance := ansi.Strip(chrome.RenderGuidanceText(m.guidanceSpec(), formMinWidth))
	if !strings.Contains(guidance, "Ctrl+S save") || !strings.Contains(guidance, "Esc cancel") {
		t.Fatalf("save/cancel guidance is not published at the floor:\n%s", guidance)
	}
	m.SetShellFrameMetrics(chrome.ShellFrameMetrics{ControlRows: lipgloss.Height(guidance)})
	frameRows := formMinHeight - m.shellFrame.ExcessRows()
	if frameRows >= formMinHeight {
		t.Fatalf("the floor's footer fits one row, so the state under test was not reached:\n%s", guidance)
	}
	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "Terminal too small") {
		t.Fatalf("form should render at its floor:\n%s", plain)
	}
	if strings.Contains(plain, "Ctrl+S save") {
		t.Fatalf("the form drew its own footer under the shell's:\n%s", plain)
	}
	assertFitsWithin(t, m.View().Content, formMinWidth, frameRows)

	for range formBasicsFieldCount - 1 {
		m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
	}
	if m.focusIndex != formBasicsContextField {
		t.Fatalf("focus after tabbing the Basics tab = %d, want %d", m.focusIndex, formBasicsContextField)
	}
	focused := ansi.Strip(m.View().Content)
	if !strings.Contains(focused, "Context:") {
		t.Fatalf("Context row is unreachable at the minimum size:\n%s", focused)
	}
	assertFitsWithin(t, m.View().Content, formMinWidth, frameRows)
}

func TestTaskFormSaveNormalizesContext(t *testing.T) {
	env := newScenarioEnv(t, "create-context")
	m := scenarioCreateForm(t, env)
	m.titleInput.SetValue("Ship release")
	m.contextInput.SetValue("  Client-Acme  ")

	msg := runCreate(t, m)
	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if task.Context != "client-acme" {
		t.Fatalf("stored context = %q, want client-acme", task.Context)
	}
}

func TestTaskFormSaveClearsContextWhenFieldIsEmpty(t *testing.T) {
	env := newScenarioEnv(t, "create-context-clear")
	m := scenarioCreateForm(t, env)
	m.titleInput.SetValue("No context")
	m.contextInput.SetValue("client-acme")
	// Clearing the field before saving is the documented way to drop a context
	// carried in from a compact capture.
	m.contextInput.SetValue("")

	msg := runCreate(t, m)
	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if task.Context != "" {
		t.Fatalf("stored context = %q, want unset", task.Context)
	}
}

func TestTaskFormInvalidContextRefusesSaveAndKeepsDraft(t *testing.T) {
	env := newScenarioEnv(t, "create-context-invalid")
	m := scenarioCreateForm(t, env)
	m.titleInput.SetValue("Ship release")
	// Punctuation alone normalizes to nothing, which is the one spelling the
	// canonical writer refuses outright rather than kebab-casing.
	m.contextInput.SetValue("!!!")

	if cmd := m.handleSave(); cmd != nil {
		t.Fatal("an invalid context must not reach the coordinator")
	}
	if m.err == nil {
		t.Fatal("an invalid context must report an error")
	}
	if !strings.Contains(m.err.Error(), "context") {
		t.Fatalf("error should name the context field: %v", m.err)
	}
	if m.activeTab != formTabBasics || m.focusIndex != formBasicsContextField {
		t.Fatalf("refusal did not land on the Context row: tab=%d focus=%d", m.activeTab, m.focusIndex)
	}
	if m.contextInput.Value() != "!!!" {
		t.Fatalf("refusal discarded the typed value: %q", m.contextInput.Value())
	}
	if m.submissionInFlight {
		t.Error("a refused save must not hold the submission guard")
	}
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("a refused save wrote %d tasks", got)
	}
}

func TestTaskFormContextAndChecklistCommitAsOneUndoEntry(t *testing.T) {
	env := newScenarioEnv(t, "create-context-checklist")
	m := scenarioCreateForm(t, env)
	m.titleInput.SetValue("Ship release")
	m.contextInput.SetValue("client-acme")
	m.draft.checklist = []models.ChecklistItem{
		{Text: "step one"},
		{Text: "step two", Done: true},
	}

	msg := runCreate(t, m)
	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if task.Context != "client-acme" || len(task.Checklist) != 2 {
		t.Fatalf("context=%q checklist=%d, want one task holding both", task.Context, len(task.Checklist))
	}

	env.undo(t)
	if env.reload(t).GetTaskByID(msg.Result.TaskID) != nil {
		t.Fatal("one undo did not take back the context and its checklist together")
	}

	env.redo(t)
	restored := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if restored == nil {
		t.Fatal("redo did not restore the created task")
	}
	if restored.Context != "client-acme" || len(restored.Checklist) != 2 {
		t.Fatalf("redo lost part of the pair: context=%q checklist=%d", restored.Context, len(restored.Checklist))
	}
}

func TestTaskComposerExpandCarriesCapturedContext(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
	}, runtime)
	composer.Input.SetValue("Ship it ##Client-Acme #bug")
	composer.ExpandSingle()

	if composer.Mode != ComposerExpanded || composer.Expanded == nil {
		t.Fatalf("a line carrying a context did not expand: errors=%+v", composer.Errors)
	}
	if composer.Expanded.draft.context != "client-acme" {
		t.Fatalf("expanded draft context = %q, want client-acme", composer.Expanded.draft.context)
	}
	// The input has to be seeded too: commitAllTextInputs reads the live input,
	// not the draft, so an unseeded input would erase the value at save time.
	if composer.Expanded.contextInput.Value() != "client-acme" {
		t.Fatalf("expanded context input = %q, want client-acme", composer.Expanded.contextInput.Value())
	}
	composer.Expanded.commitAllTextInputs()
	if composer.Expanded.draft.context != "client-acme" {
		t.Fatalf("committing text inputs dropped the context: %q", composer.Expanded.draft.context)
	}
}

func TestTaskComposerExpandAcceptsPinAndRefusesMultiline(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	newComposer := func(line string) *TaskComposerModel {
		composer := NewTaskComposerModel(TaskComposerPreset{
			Kind:        ComposerProject,
			Project:     project,
			AllProjects: []*models.Project{project},
		}, runtime)
		composer.Input.SetValue(line)
		return composer
	}

	pinned := newComposer("Ship it *pin")
	pinned.ExpandSingle()
	if pinned.Mode != ComposerExpanded || !pinned.Expanded.draft.pinned {
		t.Fatalf("pin did not expand: mode=%v errors=%+v", pinned.Mode, pinned.Errors)
	}

	multiline := newComposer("One\nTwo")
	multiline.ExpandSingle()
	if multiline.Mode != ComposerCompact || len(multiline.Errors) != 1 ||
		!strings.Contains(multiline.Errors[0].Message, "exactly one") {
		t.Fatalf("multiline refusal changed: mode=%v errors=%+v", multiline.Mode, multiline.Errors)
	}
}
