package taskform

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

func TestTaskComposerMultilineValidationIsAtomicAndUsesSourceLines(t *testing.T) {
	env := newScenarioEnv(t, "composer-atomic")
	composer := NewTaskComposerModel(TaskComposerPreset{
		Kind:    ComposerProject,
		Project: env.project,
		Shared:  TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	input := "First task\n\n!1\nThird task"
	composer.Input.SetValue(input)

	if cmd := composer.CreateCompact(false); cmd != nil {
		t.Fatalf("invalid batch returned commit command %T", cmd())
	}
	if len(composer.Errors) != 1 || composer.Errors[0].Line != 3 {
		t.Fatalf("errors = %+v, want source line 3", composer.Errors)
	}
	if composer.Input.Value() != input {
		t.Fatalf("failed batch changed Input: %q", composer.Input.Value())
	}
	if composer.Input.Line() != 2 || composer.Input.Column() != 0 {
		t.Fatalf("cursor = %d:%d, want start of source line 3", composer.Input.Line(), composer.Input.Column())
	}
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("atomic validation created %d tasks", got)
	}
}

func TestTaskComposerFailedCaptureFocusesOffscreenWrappedLine(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Project: project}, nil)
	composer.SetSize(60, 18) // The shell reserves the other two terminal rows.
	lines := make([]string, 25)
	for i := range lines {
		lines[i] = fmt.Sprintf("Task %d %s", i, strings.Repeat("wrapped text ", 8))
	}
	lines[12] = "!1"
	draft := strings.Join(lines, "\n")
	composer.Update(tea.PasteMsg{Content: draft})
	if len(composer.Errors) != 0 {
		t.Fatal("line validation ran before submission")
	}
	composer.AC.Clear()
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if composer.Input.Line() != 12 || composer.Input.Column() != 0 {
		t.Fatalf("cursor = %d:%d, want start of line 13", composer.Input.Line(), composer.Input.Column())
	}
	if composer.Input.Value() != draft || composer.SubmissionInFlight {
		t.Fatal("refused submission changed the draft or started a commit")
	}
	if view := ansi.Strip(composer.Input.View()); !strings.Contains(view, "!1") {
		t.Fatalf("off-screen error was not revealed:\n%s", view)
	}
	composer.SetSize(100, 33)
	if composer.Input.Line() != 12 || !strings.Contains(ansi.Strip(composer.Input.View()), "!1") {
		t.Fatal("resize lost the error line")
	}
}

func TestTaskComposerRefreshesErrorsWithoutMovingCaretOrCommitting(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Project: project}, nil)
	composer.Input.SetValue("!1\n\n!2")
	composer.CreateCompact(false)
	composer.Update(tea.PasteMsg{Content: "First "})
	if len(composer.Errors) != 1 || composer.Errors[0].Line != 3 {
		t.Fatalf("corrected line retained stale errors: %+v", composer.Errors)
	}
	if composer.Input.Line() != 0 || composer.Input.Column() != 6 {
		t.Fatal("editing jumped to the next error")
	}
	composer.Input.MoveToEnd()
	composer.Input.CursorStart()
	composer.Update(tea.PasteMsg{Content: "Second "})
	if len(composer.Errors) != 0 || composer.SubmissionInFlight {
		t.Fatalf("corrected draft has errors or submitted: %+v", composer.Errors)
	}
	// Live validation remains active after a correction, including the newline
	// branch that bypasses the textarea's usual UpdateInput path.
	composer.Input.MoveToEnd()
	composer.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	composer.Update(tea.PasteMsg{Content: "!3"})
	if len(composer.Errors) != 1 || composer.Errors[0].Line != 4 || composer.Input.Column() != 2 {
		t.Fatalf("new error did not follow the edited draft: %+v, col=%d", composer.Errors, composer.Input.Column())
	}
}

func TestTaskComposerLiveBatchDiagnosticsPreserveDraftAndCaret(t *testing.T) {
	project := &models.Project{ID: "project", Slug: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{
		Project: project, AllowProjectSwitch: true,
		InitialText: "First task\n\n!1\nLast task",
	}, nil)
	composer.SetSize(60, 18)
	if got := ansi.Strip(composer.renderComposerPanelPreview(2)); !strings.Contains(got, "3 tasks · Line 4 · 1 line needs attention") {
		t.Fatalf("missing live diagnosis: %s", got)
	}
	if len(composer.Errors) != 0 || composer.validateOnEdit || composer.SubmissionInFlight {
		t.Fatal("live diagnosis published a submit refusal or submitted the draft")
	}
	before, line, column := composer.Input.Value(), composer.Input.Line(), composer.Input.Column()
	composer.View()
	if composer.Input.Value() != before || composer.Input.Line() != line || composer.Input.Column() != column {
		t.Fatal("rendering live diagnostics changed the draft or caret")
	}
	// Editing another line adds a project-resolution error before any submit.
	for _, r := range " %missing" {
		composer.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if composer.batchInvalidLines != 2 {
		t.Fatalf("live count = %d, want 2", composer.batchInvalidLines)
	}
	_, errs := composer.parseCompactLines()
	if len(errs) != 2 || errs[0].Line != 3 || errs[1].Line != 4 {
		t.Fatalf("submit diagnostics differ: %+v", errs)
	}
	for range len(" %missing") {
		composer.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	if composer.batchInvalidLines != 1 {
		t.Fatalf("correcting the draft left %d invalid lines", composer.batchInvalidLines)
	}
	composer.Input.MoveToBegin()
	composer.Input.CursorDown()
	composer.Input.CursorDown()
	composer.Input.CursorEnd()
	for _, r := range " Valid title" {
		composer.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if composer.batchInvalidLines != 0 || strings.Contains(ansi.Strip(composer.renderComposerPanelPreview(2)), "needs attention") {
		t.Fatal("fixed batch retained its warning")
	}
}

func TestTaskComposerNextProblemVisitsReasonsAndWraps(t *testing.T) {
	project := &models.Project{ID: "test", Slug: "test", Name: "Test"}
	m := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	m.SetSize(60, 18)
	draft := "Valid\nBad !4\n\nAlso bad !5\nLast bad !4"
	m.Input.SetValue(draft)
	if cmd := m.CreateCompact(false); cmd != nil {
		t.Fatal("invalid batch committed")
	}
	for i, line := range []int{2, 4, 5, 2} {
		if i > 0 {
			m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt})
		}
		if m.Input.Line()+1 != line {
			t.Fatalf("visit %d: line %d want %d", i, m.Input.Line()+1, line)
		}
		expected := fmt.Sprintf("Problem %d of 3 · Line %d", i%3+1, line)
		if !strings.Contains(ansi.Strip(m.View().Content), expected) {
			t.Fatalf("missing %q:\n%s", expected, ansi.Strip(m.View().Content))
		}
		if m.Input.Value() != draft || m.SubmissionInFlight {
			t.Fatal("navigation altered the draft or submitted")
		}
	}
	m.Input.SetValue("Valid\nFixed\n\nAlso bad !5\nLast bad !4")
	m.refreshCaptureErrorsAfterEdit(draft, project)
	m.focusNextCaptureError()
	if m.Input.Line() != 3 || len(m.Errors) != 2 {
		t.Fatal("next problem retained corrected line")
	}
}

func TestTaskComposerPreviousProblemPreservesDraftAndSkipsCorrectedLines(t *testing.T) {
	project := &models.Project{ID: "test", Slug: "test", Name: "Test"}
	m := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
	m.SetSize(60, 18)
	draft := "Valid\nBad !4\n\nAlso bad !5\nLast bad !4"
	m.Input.SetValue(draft)
	previous := tea.KeyPressMsg{Code: 'p', Mod: tea.ModAlt}
	line, column := m.Input.Line(), m.Input.Column()
	m.Update(previous)
	if m.Input.Line() != line || m.Input.Column() != column || m.Input.Value() != draft {
		t.Fatal("previous problem changed the draft before a refused submission")
	}
	if cmd := m.CreateCompact(false); cmd != nil {
		t.Fatal("invalid batch committed")
	}
	for i, line := range []int{5, 4, 2, 5} {
		if _, cmd := m.Update(previous); cmd != nil {
			t.Fatal("problem navigation emitted a command")
		}
		wantColumn := strings.Index(strings.Split(draft, "\n")[line-1], "!")
		if m.Input.Line()+1 != line || m.Input.Column() != wantColumn {
			t.Fatalf("visit %d: line %d column %d, want line %d column %d", i, m.Input.Line()+1, m.Input.Column(), line, wantColumn)
		}
		problem := []int{3, 2, 1, 3}[i]
		want := fmt.Sprintf("Problem %d of 3 · Line %d", problem, line)
		if !strings.Contains(ansi.Strip(m.View().Content), want) {
			t.Fatalf("missing %q in narrow view: %s", want, ansi.Strip(m.View().Content))
		}
		if m.Input.Value() != draft || m.SubmissionInFlight {
			t.Fatal("navigation altered the draft or submitted")
		}
	}
	m.HelpExpanded = true
	m.Update(previous)
	if m.Input.Line() != 4 {
		t.Fatal("previous problem navigated underneath syntax help")
	}
	m.HelpExpanded = false
	fixed := "Valid\nBad !4\n\nFixed\nLast bad !4"
	m.Input.SetValue(fixed)
	m.refreshCaptureErrorsAfterEdit(draft, project)
	m.focusCaptureErrorLine(5)
	m.Update(previous)
	if m.Input.Line() != 1 || len(m.Errors) != 2 {
		t.Fatal("previous problem retained the corrected line")
	}
	m.Input.SetValue("Valid\nFixed\n\nFixed\nLast bad !4")
	m.refreshCaptureErrorsAfterEdit(fixed, project)
	m.Update(previous)
	m.Update(previous)
	if m.Input.Line() != 4 || len(m.Errors) != 1 {
		t.Fatal("previous problem did not wrap to the sole remaining error")
	}
}
