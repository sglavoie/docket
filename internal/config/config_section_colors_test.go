package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestConfig_GetGlobalSectionColor(t *testing.T) {
	cfg := NewSettings()

	if color := cfg.GetGlobalSectionColor("Unknown"); color != "gray" {
		t.Errorf("expected gray for unknown section, got %s", color)
	}

	cfg.SetGlobalSectionColor("Active", "green")
	if color := cfg.GetGlobalSectionColor("Active"); color != "green" {
		t.Errorf("expected green, got %s", color)
	}

	cfg.SectionColors["Empty"] = ""
	if color := cfg.GetGlobalSectionColor("Empty"); color != "gray" {
		t.Errorf("expected gray for empty string color, got %s", color)
	}
}

func TestConfig_SetGlobalSectionColor(t *testing.T) {
	cfg := NewSettings()

	cfg.SetGlobalSectionColor("Active", "green")
	cfg.SetGlobalSectionColor("Done", "blue")

	if cfg.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green")
	}
	if cfg.SectionColors["Done"] != "blue" {
		t.Errorf("expected Done to be blue")
	}

	cfg.SetGlobalSectionColor("Active", "red")
	if cfg.SectionColors["Active"] != "red" {
		t.Errorf("expected Active to be red after overwrite")
	}
}

func TestConfig_SetGlobalSectionColor_NilMap(t *testing.T) {
	cfg := &Settings{}
	cfg.SectionColors = nil

	cfg.SetGlobalSectionColor("Active", "green")

	if cfg.SectionColors == nil {
		t.Error("SectionColors should be initialized")
	}
	if cfg.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green")
	}
}

func TestConfig_MigrateProjectSectionColors(t *testing.T) {
	cfg := NewSettings()
	cfg.SetGlobalSectionColor("Active", "blue")

	projectColors := map[string]string{
		"Active": "green",
		"Review": "yellow",
		"Done":   "gray",
		"Empty":  "",
	}

	cfg.MigrateProjectSectionColors(projectColors)

	if color := cfg.GetGlobalSectionColor("Active"); color != "blue" {
		t.Errorf("expected Active to remain blue, got %s", color)
	}

	if color := cfg.GetGlobalSectionColor("Review"); color != "yellow" {
		t.Errorf("expected Review to be yellow, got %s", color)
	}

	if _, exists := cfg.SectionColors["Done"]; exists {
		t.Error("Done should not be added to global registry (gray is default)")
	}

	if _, exists := cfg.SectionColors["Empty"]; exists {
		t.Error("Empty should not be added to global registry")
	}
}

func TestConfig_MigrateProjectSectionColors_NilInput(t *testing.T) {
	cfg := NewSettings()
	cfg.SetGlobalSectionColor("Active", "green")

	cfg.MigrateProjectSectionColors(nil)

	if color := cfg.GetGlobalSectionColor("Active"); color != "green" {
		t.Errorf("expected Active to remain green, got %s", color)
	}
}

func TestConfig_ApplyGlobalColorsToProject(t *testing.T) {
	cfg := NewSettings()
	cfg.SetGlobalSectionColor("Active", "green")
	cfg.SetGlobalSectionColor("Done", "blue")

	project, err := models.NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.Sections = []string{"Backlog", "Active", "Done", "Review"}

	cfg.ApplyGlobalColorsToProject(project)

	if project.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green, got %s", project.SectionColors["Active"])
	}
	if project.SectionColors["Done"] != "blue" {
		t.Errorf("expected Done to be blue, got %s", project.SectionColors["Done"])
	}

	if _, exists := project.SectionColors["Backlog"]; exists {
		t.Error("Backlog should not have a color (no global color)")
	}
	if _, exists := project.SectionColors["Review"]; exists {
		t.Error("Review should not have a color (no global color)")
	}
}

func TestConfig_ApplyGlobalColorsToProject_NilProjectColors(t *testing.T) {
	cfg := NewSettings()
	cfg.SetGlobalSectionColor("Active", "green")

	project, err := models.NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	project.SectionColors = nil
	project.Sections = []string{"Active"}

	cfg.ApplyGlobalColorsToProject(project)

	if project.SectionColors == nil {
		t.Error("SectionColors should be initialized")
	}
	if project.SectionColors["Active"] != "green" {
		t.Errorf("expected Active to be green")
	}
}
