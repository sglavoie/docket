package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseTaskTypeFlagsConflicting(t *testing.T) {
	// Single flags should work
	tests := []struct {
		name     string
		epic     bool
		story    bool
		bug      bool
		idea     bool
		typeFlag string
		want     models.TaskType
	}{
		{"epic only", true, false, false, false, "", models.TaskTypeEpic},
		{"story only", false, true, false, false, "", models.TaskTypeStory},
		{"bug only", false, false, true, false, "", models.TaskTypeBug},
		{"idea only", false, false, false, true, "", models.TaskTypeIdea},
		{"type flag only", false, false, false, false, "epic", models.TaskTypeEpic},
		{"no flags", false, false, false, false, "", models.TaskTypeTask},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaskTypeFlags(tt.epic, tt.story, tt.bug, tt.idea, tt.typeFlag)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	// Multiple flags should error
	conflictTests := []struct {
		name     string
		epic     bool
		story    bool
		bug      bool
		idea     bool
		typeFlag string
	}{
		{"epic and story", true, true, false, false, ""},
		{"epic and bug", true, false, true, false, ""},
		{"story and idea", false, true, false, true, ""},
		{"bug and type flag", false, false, true, false, "epic"},
		{"all booleans", true, true, true, true, ""},
	}
	for _, tt := range conflictTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTaskTypeFlags(tt.epic, tt.story, tt.bug, tt.idea, tt.typeFlag)
			if err == nil {
				t.Error("expected error for conflicting type flags")
			}
			if !strings.Contains(err.Error(), "conflicting type flags") {
				t.Errorf("expected 'conflicting type flags' in error, got: %v", err)
			}
		})
	}
}
