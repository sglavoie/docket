package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

func TestQuickCaptureParsesContextAndRefusesClearBySurface(t *testing.T) {
	project := &models.Project{ID: "project"}
	result, err := grammar.ParseQuickCapture("Ship it ##Client-Acme #bug", project, nil, false)
	if err != nil {
		t.Fatalf("parse capture: %v", err)
	}
	if result.Title != "Ship it" || result.Context != "client-acme" {
		t.Fatalf("capture result = title %q context %q", result.Title, result.Context)
	}

	captureDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldContext)
	if _, err := grammar.ParseQuickCapture("Ship it "+captureDefinition.Trigger+"0", project, nil, false); err == nil || !strings.Contains(err.Error(), captureDefinition.Trigger+"0") {
		t.Fatalf("capture clear error = %v, want named refusal", err)
	}

	outlineDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldContext)
	if _, err := grammar.ParseQuickCaptureSurface("Ship it "+outlineDefinition.Trigger+"0", project, nil, true, textgrammar.SurfaceOutline); err == nil || !strings.Contains(err.Error(), outlineDefinition.Forms[0]) {
		t.Fatalf("outline clear error = %v, want omission guidance", err)
	}
}

func TestQuickEditSetsAndClearsContext(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Before", Context: "legacy", WorkflowState: models.StateInbox}
	project := &models.Project{ID: "project", Tasks: []*models.Task{task}}

	result, err := grammar.ParseQuickEdit("After ##Client-Acme", task, project)
	if err != nil {
		t.Fatalf("parse set: %v", err)
	}
	if !result.ContextSet || result.ContextClear || result.ContextValue != "client-acme" || result.Preview.Context != "client-acme" {
		t.Fatalf("set patch = %+v preview=%q", result, result.Preview.Context)
	}

	result, err = grammar.ParseQuickEdit("After ##0", task, project)
	if err != nil {
		t.Fatalf("parse clear: %v", err)
	}
	if !result.ContextSet || !result.ContextClear || result.Preview.Context != "" {
		t.Fatalf("clear patch = set %v clear %v preview %q", result.ContextSet, result.ContextClear, result.Preview.Context)
	}
}

func TestContextAutocompleteUsesProjectVocabulary(t *testing.T) {
	project := &models.Project{Tasks: []*models.Task{
		{Context: "platform"},
		{Context: "client-acme"},
		{Context: "platform"},
	}}
	q := grammar.NewQuickCaptureInput(project, nil, "", false)
	items := q.GetContextSuggestions("plat")
	definition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldContext)
	if len(items) != 1 || items[0].Display != definition.Trigger+"platform" || items[0].Value != "platform" {
		t.Fatalf("context suggestions = %+v", items)
	}
	mode, _, partial := grammar.DetectTrigger("Ship "+definition.Trigger+"plat", q.Triggers)
	if mode != grammar.ACModeContext || partial != "plat" {
		t.Fatalf("context trigger = mode %d partial %q", mode, partial)
	}
}

func TestTaskComposerCarriesContextIntoExpandedMode(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:        taskform.ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
	}, appModel.domain.runtime)
	composer.Input.SetValue("Ship release ##Client-Acme")

	reqs, ok := composer.ParseCompactLines()
	if !ok || len(reqs) != 1 || reqs[0].Context != "client-acme" {
		t.Fatalf("context requests = %+v ok=%v", reqs, ok)
	}
	composer.ExpandSingle()
	if len(composer.Errors) != 0 {
		t.Fatalf("expand refused a line carrying a context: %+v", composer.Errors)
	}
	if composer.Expanded == nil {
		t.Fatal("expand did not open the form")
	}
	// The seeded draft value itself is asserted inside the taskform package,
	// which can read the unexported draft directly.
	if !strings.Contains(composer.View().Content, "client-acme") {
		t.Fatalf("expanded form did not show the captured context:\n%s", composer.View().Content)
	}
}
