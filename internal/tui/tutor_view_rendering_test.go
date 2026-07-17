package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestTutorColorRulesIncludeAllWorkflowStates(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		stateStr := state.String()
		found := false
		for _, rule := range tutorColorRules {
			if rule.re.MatchString(stateStr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tutor color rules missing highlighting for workflow state %q", stateStr)
		}
	}
}

func TestColorizeTutorLineHighlightsPaused(t *testing.T) {
	input := "Tasks in paused stay out of the Focus Queue."
	output := colorizeTutorLine(input)
	if input == output {
		t.Error("expected colorizeTutorLine to apply styling to 'paused', but output was unchanged")
	}
}

func TestTutorColorRulesDoNotTreatRetiredStageKeysAsBindings(t *testing.T) {
	input := ".1 Set first stage"
	if output := colorizeTutorLine(input); output != input {
		t.Fatalf("retired stage binding was colorized: %q", output)
	}
}
