package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestLifecycleDestinationsMapToCanonicalCreationStates(t *testing.T) {
	project := &models.Project{Name: "Grammar", ID: "grammar"}
	for _, state := range models.AllWorkflowStates() {
		t.Run(state.String(), func(t *testing.T) {
			input := "Task $" + state.String()

			captured, err := parseQuickCapture(input, project, nil, true)
			if err != nil {
				t.Fatalf("capture parse: %v", err)
			}
			if !captured.destinationSet || captured.destination.WorkflowState() != state {
				t.Fatalf("capture destination = %q (set=%v), want state %q", captured.destination, captured.destinationSet, state)
			}

			edited, err := parseQuickEdit(input, &models.Task{Title: "Old"}, project)
			if err != nil {
				t.Fatalf("edit parse: %v", err)
			}
			if !edited.stateSet || edited.state != state {
				t.Fatalf("edit state = %q (set=%v), want %q", edited.state, edited.stateSet, state)
			}

			outliner := scopeTestOutliner(project)
			outliner.SetSearchQuery(input)
			gate := outliner.searchGate
			if gate == nil || gate.destination == nil || gate.destination.destination.WorkflowState() != state {
				t.Fatalf("search destination = %+v, want state %q", gate, state)
			}
			if want := state.IsTerminal(); gate.admitsTerminal != want {
				t.Fatalf("search $%s admitsTerminal = %v, want %v", state, gate.admitsTerminal, want)
			}
		})
	}
}

func TestSearchPlanningDestinationsAreMembershipRatherThanState(t *testing.T) {
	project := &models.Project{Name: "Grammar", ID: "grammar", Slug: "grammar"}
	for _, name := range []string{"today", "next"} {
		outliner := scopeTestOutliner(project)
		outliner.SetSearchQuery("Task $" + name)
		gate := outliner.searchGate
		if gate == nil || gate.destination == nil {
			t.Fatalf("$%s produced no destination gate", name)
		}
		if gate.destination.destination.WorkflowState() != models.StateBacklog {
			t.Fatalf("$%s state = %q, want backlog", name, gate.destination.destination.WorkflowState())
		}
		if gate.admitsTerminal {
			t.Errorf("$%s relaxed the nonterminal rule", name)
		}
		if name == "today" && gate.destination.todayIDs == nil {
			t.Error("$today did not resolve a planning membership set")
		}
		if name == "next" && gate.destination.nextIDs == nil {
			t.Error("$next did not resolve a planning membership set")
		}
	}
}

func TestDestinationGrammarDrivesCompletionHelpAndGuidance(t *testing.T) {
	q := &quickCaptureInput{width: 120}
	suggestions := q.getDestinationSuggestions("")
	if len(suggestions) != len(captureDestinationNamesForTest()) {
		t.Fatalf("destination suggestions = %d, want %d", len(suggestions), len(captureDestinationNamesForTest()))
	}

	guide := q.renderSyntaxGuide()
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
		result, err := parseQuickEdit("Task "+token, task, project)
		if err != nil {
			t.Fatalf("quick-edit parse %q: %v", token, err)
		}
		req := taskLineEditRequest(TaskIdentity{ProjectID: project.ID, TaskID: task.ID}, result)
		if req.Destination == nil || string(*req.Destination) != name {
			t.Errorf("quick-edit request destination = %v, want %q", req.Destination, name)
		}
	}

	project := &models.Project{Name: "Grammar"}
	for raw, replacement := range map[string]string{"ready": "$backlog", "in_progress": "$doing"} {
		_, captureErr := parseQuickCapture("Task $"+raw, project, nil, true)
		if captureErr == nil || !strings.Contains(captureErr.Error(), replacement) {
			t.Errorf("capture error for %q = %v, want replacement %s", raw, captureErr, replacement)
		}
	}
	if parsed, err := parseQuickCapture("Task $today", project, nil, true); err != nil || parsed.destination != app.CaptureDestinationToday {
		t.Fatalf("Today destination = %+v, %v", parsed, err)
	}
}
