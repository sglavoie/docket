package cmd

import (
	"fmt"
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

func TestValidateParentTask_NestingDepthExceeded(t *testing.T) {
	// Create a project with a task at MaxNestingDepth.
	project, err := models.NewProject("proj-1", "Test Project", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// Create a chain of tasks from level 0 up to MaxNestingDepth.
	parent, err := models.NewTask("parent-id", 1, models.TaskTypeEpic, "Deep Parent")
	if err != nil {
		t.Fatal(err)
	}
	parent.Level = models.MaxNestingDepth
	project.Tasks = append(project.Tasks, parent)

	// Trying to add a child under a task at MaxNestingDepth should fail.
	_, err = ValidateParentTask(project, "1", models.TaskTypeTask)
	if err == nil {
		t.Fatal("expected error when parent is at MaxNestingDepth, got nil")
	}
	if !strings.Contains(err.Error(), "nesting") {
		t.Errorf("expected nesting depth error, got: %v", err)
	}
}

func TestValidateParentTask_NestingDepthAllowed(t *testing.T) {
	project, err := models.NewProject("proj-1", "Test Project", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	parent, err := models.NewTask("parent-id", 1, models.TaskTypeEpic, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	parent.Level = models.MaxNestingDepth - 1
	project.Tasks = append(project.Tasks, parent)

	// Adding a child one level below MaxNestingDepth should succeed.
	result, err := ValidateParentTask(project, fmt.Sprintf("%d", parent.ShortID), models.TaskTypeTask)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ID != parent.ID {
		t.Errorf("expected parent task returned, got %s", result.ID)
	}
}
