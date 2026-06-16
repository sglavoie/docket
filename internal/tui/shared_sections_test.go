package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCustomSectionTabColorsForProjectsUsesProjectColors(t *testing.T) {
	project := &models.Project{
		ID:            "alpha",
		Name:          "Alpha",
		Sections:      []string{models.SectionNext, "Sprint-1"},
		SectionColors: map[string]string{"Sprint-1": "green"},
	}

	got := CustomSectionTabColorsForProjects([]*models.Project{project}, nil)

	if got["Sprint-1"] != "green" {
		t.Fatalf("Sprint-1 color = %q, want green", got["Sprint-1"])
	}
}

func TestCustomSectionTabColorsForProjectsPrefersGlobalColors(t *testing.T) {
	project := &models.Project{
		ID:            "alpha",
		Name:          "Alpha",
		Sections:      []string{models.SectionNext, "Sprint-1"},
		SectionColors: map[string]string{"Sprint-1": "green"},
	}

	got := CustomSectionTabColorsForProjects([]*models.Project{project}, map[string]string{"Sprint-1": "purple"})

	if got["Sprint-1"] != "purple" {
		t.Fatalf("Sprint-1 color = %q, want purple", got["Sprint-1"])
	}
}
