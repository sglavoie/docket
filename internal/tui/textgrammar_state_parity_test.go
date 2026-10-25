package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func TestLifecycleDestinationsMapToCanonicalCreationStates(t *testing.T) {
	project := &models.Project{Name: "Grammar", ID: "grammar"}
	for _, state := range models.AllWorkflowStates() {
		t.Run(state.String(), func(t *testing.T) {
			input := "Task $" + state.String()

			captured, err := capturetext.ParseQuickCapture(input, project, nil, true)
			if err != nil {
				t.Fatalf("capture parse: %v", err)
			}
			if !captured.DestinationSet || captured.Destination.WorkflowState() != state {
				t.Fatalf("capture destination = %q (set=%v), want state %q", captured.Destination, captured.DestinationSet, state)
			}

			edited, err := grammar.ParseQuickEdit(input, &models.Task{Title: "Old"}, project)
			if err != nil {
				t.Fatalf("edit parse: %v", err)
			}
			if !edited.StateSet || edited.State != state {
				t.Fatalf("edit state = %q (set=%v), want %q", edited.State, edited.StateSet, state)
			}

			outlinerView := scopeTestOutliner(project)
			outlinerView.SetSearchQuery(input)
			gate := outlinerView.Search.Gate
			searched, ok := gate.CompiledDestination()
			if !ok || searched.WorkflowState() != state {
				t.Fatalf("search destination = %q (set=%v), want state %q", searched, ok, state)
			}
			if want := state.IsTerminal(); gate.AllowsTerminal() != want {
				t.Fatalf("search $%s AllowsTerminal = %v, want %v", state, gate.AllowsTerminal(), want)
			}
		})
	}
}

func TestSearchPlanningDestinationsAreMembershipRatherThanState(t *testing.T) {
	project := &models.Project{Name: "Grammar", ID: "grammar", Slug: "grammar"}
	for _, name := range []string{"today", "next"} {
		outlinerView := scopeTestOutliner(project)
		outlinerView.SetSearchQuery("Task $" + name)
		gate := outlinerView.Search.Gate
		destination, ok := gate.CompiledDestination()
		if !ok {
			t.Fatalf("$%s produced no destination gate", name)
		}
		if destination.WorkflowState() != models.StateBacklog {
			t.Fatalf("$%s state = %q, want backlog", name, destination.WorkflowState())
		}
		if gate.AllowsTerminal() {
			t.Errorf("$%s relaxed the nonterminal rule", name)
		}
		if !gate.DestinationResolvedMembership() {
			t.Errorf("$%s did not resolve a planning membership set", name)
		}
	}
}

func TestDestinationGrammarDrivesCompletionHelpAndGuidance(t *testing.T) {
	q := &grammar.QuickCaptureInput{Width: 120}
	suggestions := q.GetDestinationSuggestions("")
	if len(suggestions) != len(captureDestinationNamesForTest()) {
		t.Fatalf("destination suggestions = %d, want %d", len(suggestions), len(captureDestinationNamesForTest()))
	}

	guide, _ := q.RenderSyntaxGuide()
	for _, name := range captureDestinationNamesForTest() {
		token := "$" + name
		if !strings.Contains(guide, token) {
			t.Errorf("capture help does not contain %q", token)
		}
	}
	for _, state := range models.AllWorkflowStates() {
		name := state.String()
		token := "$" + name
		task := &models.Task{ID: "task", Title: "Old", WorkflowState: models.StateInbox}
		project := &models.Project{ID: "grammar", Name: "Grammar", Tasks: []*models.Task{task}}
		result, err := grammar.ParseQuickEdit("Task "+token, task, project)
		if err != nil {
			t.Fatalf("quick-edit parse %q: %v", token, err)
		}
		req := outliner.TaskLineEditRequest(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID}, result)
		if req.Destination == nil || string(*req.Destination) != name {
			t.Errorf("quick-edit request destination = %v, want %q", req.Destination, name)
		}
	}

	project := &models.Project{Name: "Grammar"}
	for raw, replacement := range map[string]string{"ready": "$backlog", "in_progress": "$doing"} {
		_, captureErr := capturetext.ParseQuickCapture("Task $"+raw, project, nil, true)
		if captureErr == nil || !strings.Contains(captureErr.Error(), replacement) {
			t.Errorf("capture error for %q = %v, want replacement %s", raw, captureErr, replacement)
		}
	}
	if parsed, err := capturetext.ParseQuickCapture("Task $today", project, nil, true); err != nil || parsed.Destination != app.CaptureDestinationToday {
		t.Fatalf("Today destination = %+v, %v", parsed, err)
	}
}
