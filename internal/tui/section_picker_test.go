package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildSectionPickerList_CustomSectionNotDoubleCounted(t *testing.T) {
	project := &models.Project{
		ID:            "test",
		Name:          "Test",
		Slug:          "test",
		Sections:      []string{"Next"},
		SectionColors: map[string]string{},
		Tasks: []*models.Task{
			{ID: "1", Section: "Next"},
			{ID: "2", Section: "Next"},
			{ID: "3", Section: "", CustomSection: "Sprint-1"}, // custom section
			{ID: "4", Section: "", CustomSection: "Sprint-1"}, // custom section
			{ID: "5", Section: ""},                            // Active (no section)
		},
	}

	sections := buildSectionPickerList(project, nil)

	counts := make(map[string]int)
	for _, s := range sections {
		counts[s.Name] = s.TaskCount
	}

	// "Next" should have 2 tasks
	if counts["Next"] != 2 {
		t.Errorf("expected Next count=2, got %d", counts["Next"])
	}

	// "Sprint-1" should have 2 tasks
	if counts["Sprint-1"] != 2 {
		t.Errorf("expected Sprint-1 count=2, got %d", counts["Sprint-1"])
	}

	// "" (Active) should have 1 task, NOT 3 (which would include the 2 custom-section tasks)
	if counts[""] != 1 {
		t.Errorf("expected Active (empty) count=1, got %d", counts[""])
	}
}

func TestBuildSectionPickerList_OnlyGTDSections(t *testing.T) {
	project := &models.Project{
		ID:            "test",
		Name:          "Test",
		Slug:          "test",
		Sections:      []string{"Next"},
		SectionColors: map[string]string{},
		Tasks: []*models.Task{
			{ID: "1", Section: "Next"},
			{ID: "2", Section: "Waiting"},
			{ID: "3", Section: ""},
		},
	}

	sections := buildSectionPickerList(project, nil)

	counts := make(map[string]int)
	for _, s := range sections {
		counts[s.Name] = s.TaskCount
	}

	if counts["Next"] != 1 {
		t.Errorf("expected Next count=1, got %d", counts["Next"])
	}
	if counts["Waiting"] != 1 {
		t.Errorf("expected Waiting count=1, got %d", counts["Waiting"])
	}
	if counts[""] != 1 {
		t.Errorf("expected Active count=1, got %d", counts[""])
	}
}
