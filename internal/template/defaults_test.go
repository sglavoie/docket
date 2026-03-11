package template

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetDefaultTemplates_NotEmpty(t *testing.T) {
	templates := GetDefaultTemplates()
	if len(templates) == 0 {
		t.Fatal("expected at least one default template")
	}
}

func TestGetDefaultTemplates_AllHaveSource(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if tmpl.Source != "default" {
			t.Errorf("template %q: Source = %q, want %q", tmpl.Name, tmpl.Source, "default")
		}
	}
}

func TestGetDefaultTemplates_AllDisabled(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if tmpl.Enabled {
			t.Errorf("template %q: Enabled = true, want false", tmpl.Name)
		}
	}
}

func TestGetDefaultTemplates_AllHaveID(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if tmpl.ID == "" {
			t.Errorf("template %q: ID should not be empty", tmpl.Name)
		}
	}
}

func TestGetDefaultTemplates_UniqueIDs(t *testing.T) {
	seen := make(map[string]string)
	for _, tmpl := range GetDefaultTemplates() {
		if prev, ok := seen[tmpl.ID]; ok {
			t.Errorf("duplicate ID %q: used by %q and %q", tmpl.ID, prev, tmpl.Name)
		}
		seen[tmpl.ID] = tmpl.Name
	}
}

func TestGetDefaultTemplates_AllHaveName(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if tmpl.Name == "" {
			t.Errorf("template with ID %q: Name should not be empty", tmpl.ID)
		}
	}
}

func TestGetDefaultTemplates_AllHaveCategory(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if tmpl.Category == "" {
			t.Errorf("template %q: Category should not be empty", tmpl.Name)
		}
	}
}

func TestGetDefaultTemplates_AllHaveValidType(t *testing.T) {
	validTypes := map[models.TaskType]bool{
		models.TaskTypeEpic:  true,
		models.TaskTypeStory: true,
		models.TaskTypeTask:  true,
		models.TaskTypeBug:   true,
		models.TaskTypeIdea:  true,
	}

	for _, tmpl := range GetDefaultTemplates() {
		if !validTypes[tmpl.Type] {
			t.Errorf("template %q: invalid Type %q", tmpl.Name, tmpl.Type)
		}
	}
}

func TestGetDefaultTemplates_AllPassValidation(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if err := tmpl.Validate(); err != nil {
			t.Errorf("template %q: Validate() failed: %v", tmpl.Name, err)
		}
	}
}

func TestGetDefaultTemplates_PriorityInRange(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if tmpl.Priority < 0 || tmpl.Priority > models.PriorityMax {
			t.Errorf("template %q: Priority %d out of range 0-%d", tmpl.Name, tmpl.Priority, models.PriorityMax)
		}
	}
}

func TestGetDefaultTemplates_ChildrenHaveTitles(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		for i, child := range tmpl.Children {
			if child.Title == "" {
				t.Errorf("template %q: child %d has empty title", tmpl.Name, i+1)
			}
		}
	}
}

func TestGetDefaultTemplates_AllHaveTagsOrChildren(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		hasTags := len(tmpl.Tags) > 0
		hasChildren := len(tmpl.Children) > 0
		hasChecklist := len(tmpl.Checklist) > 0
		if !hasTags && !hasChildren && !hasChecklist {
			t.Errorf("template %q: should have at least tags, children, or checklist", tmpl.Name)
		}
	}
}

func TestGetDefaultTemplates_IDsStartWithDefPrefix(t *testing.T) {
	for _, tmpl := range GetDefaultTemplates() {
		if len(tmpl.ID) < 4 || tmpl.ID[:4] != "def-" {
			t.Errorf("template %q: ID %q should start with 'def-'", tmpl.Name, tmpl.ID)
		}
	}
}

func TestGetDefaultTemplates_ReturnsCopy(t *testing.T) {
	first := GetDefaultTemplates()
	second := GetDefaultTemplates()

	if len(first) != len(second) {
		t.Fatal("consecutive calls should return same number of templates")
	}

	// Mutating one should not affect the other
	first[0].Name = "MUTATED"
	fresh := GetDefaultTemplates()
	if fresh[0].Name == "MUTATED" {
		t.Error("GetDefaultTemplates should return a fresh slice each time")
	}
}
