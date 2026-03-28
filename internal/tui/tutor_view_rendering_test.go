package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestTutorColorRulesIncludeAllStatuses(t *testing.T) {
	for _, status := range models.AllTaskStatuses() {
		statusStr := string(status)
		found := false
		for _, rule := range tutorColorRules {
			if rule.re.MatchString(statusStr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tutor color rules missing highlighting for status %q", statusStr)
		}
	}
}

func TestColorizeTutorLineHighlightsStandby(t *testing.T) {
	input := "Tasks on standby are paused."
	output := colorizeTutorLine(input)
	if input == output {
		t.Error("expected colorizeTutorLine to apply styling to 'standby', but output was unchanged")
	}
}
