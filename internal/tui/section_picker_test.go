package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildSectionPickerList_NonGTDSectionsCountAsActive(t *testing.T) {
	project := &models.Project{
		ID:            "test",
		Name:          "Test",
		Slug:          "test",
		Sections:      []string{"Next"},
		SectionColors: map[string]string{},
		Tasks: []*models.Task{
			{ID: "1", Section: "Next"},
			{ID: "2", Section: "Next"},
			{ID: "3", Section: "Sprint-1"}, // legacy non-GTD section
			{ID: "4", Section: "Sprint-1"}, // legacy non-GTD section
			{ID: "5", Section: ""},         // Active (no section)
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

	if counts["Sprint-1"] != 0 {
		t.Errorf("expected Sprint-1 count=0, got %d", counts["Sprint-1"])
	}

	if counts[""] != 3 {
		t.Errorf("expected Active (empty) count=3, got %d", counts[""])
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
