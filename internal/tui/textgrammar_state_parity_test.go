package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

func TestCanonicalStateTokensHaveCaptureEditSearchParity(t *testing.T) {
	project := &models.Project{Name: "Grammar", ID: "grammar"}
	for _, state := range models.AllWorkflowStates() {
		t.Run(state.String(), func(t *testing.T) {
			input := "Task $" + state.String()

			captured, err := parseQuickCapture(input, project, nil, true)
			if err != nil {
				t.Fatalf("capture parse: %v", err)
			}
			if !captured.stateSet || captured.state != state {
				t.Fatalf("capture state = %q (set=%v), want %q", captured.state, captured.stateSet, state)
			}

			edited, err := parseQuickEdit(input, &models.Task{Title: "Old"}, project)
			if err != nil {
				t.Fatalf("edit parse: %v", err)
			}
			if !edited.stateSet || edited.state != state {
				t.Fatalf("edit state = %q (set=%v), want %q", edited.state, edited.stateSet, state)
			}

			searched := ParseSearchQuery("$" + state.String())
			if !slices.Equal(searched.States, []models.WorkflowState{state}) {
				t.Fatalf("search states = %v, want [%s]", searched.States, state)
			}
			if searched.TextQuery != "" {
				t.Fatalf("search text = %q, want empty", searched.TextQuery)
			}
		})
	}
}

func TestCanonicalStateGrammarDrivesCompletionPreviewHelpAndErrors(t *testing.T) {
	q := &quickCaptureInput{width: 120}
	suggestions := q.getStatusSuggestions("")
	if len(suggestions) != len(textgrammar.StateNames()) {
		t.Fatalf("state suggestions = %d, want %d", len(suggestions), len(textgrammar.StateNames()))
	}

	guide := q.renderSyntaxGuide()
	searchHelp := renderSearchHelpPane(100, 30)
	for _, name := range textgrammar.StateNames() {
		token := "$" + name
		if !strings.Contains(guide, token) {
			t.Errorf("capture help does not contain %q", token)
		}
		if !strings.Contains(searchHelp, name) {
			t.Errorf("search help does not contain %q", name)
		}
		preview := renderQuickEditPreview(
			&quickEditResult{title: "Task", titleSet: true, state: models.WorkflowState(name), stateSet: true},
			&quickEditResult{title: "Task", state: differentState(models.WorkflowState(name))},
		)
		if !strings.Contains(preview, token) {
			t.Errorf("quick-edit preview %q does not contain %q", preview, token)
		}
	}

	project := &models.Project{Name: "Grammar"}
	_, captureErr := parseQuickCapture("Task $unknown", project, nil, true)
	_, editErr := parseQuickEdit("Task $unknown", &models.Task{Title: "Old"}, project)
	if captureErr == nil || editErr == nil || captureErr.Error() != editErr.Error() {
		t.Fatalf("shared state errors differ: capture=%v edit=%v", captureErr, editErr)
	}
}

func differentState(state models.WorkflowState) models.WorkflowState {
	if state != models.StateReady {
		return models.StateReady
	}
	return models.StateInbox
}
