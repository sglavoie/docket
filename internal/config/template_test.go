package config

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

func TestResetChangedDefaultTemplates_EnableTemplatesTrue(t *testing.T) {
	cfg := NewConfig()

	// Add a default template that has been "changed" (different name from shipped)
	cfg.TaskTemplates = []template.TaskTemplate{
		{
			ID:       "def-bug-report",
			Name:     "Bug Report MODIFIED",
			Enabled:  false,
			Source:   "default",
			Category: "bug",
			Type:     models.TaskTypeBug,
		},
	}

	updated, readded, _, _ := cfg.ResetChangedDefaultTemplates(true)

	// The modified template should be reset and enabled
	if updated == 0 {
		t.Fatal("expected at least 1 updated template")
	}

	tmpl := cfg.GetTaskTemplate("def-bug-report")
	if tmpl == nil {
		t.Fatal("expected to find template def-bug-report")
	}
	if !tmpl.Enabled {
		t.Error("expected template to be enabled when enableTemplates=true, got disabled")
	}
	if tmpl.Name == "Bug Report MODIFIED" {
		t.Error("expected template name to be reset to shipped value")
	}

	// Re-added templates should also be enabled
	if readded > 0 {
		for _, tmpl := range cfg.TaskTemplates {
			if tmpl.Source == "default" && !tmpl.Enabled {
				t.Errorf("re-added default template %q should be enabled when enableTemplates=true", tmpl.ID)
			}
		}
	}
}

func TestResetChangedDefaultTemplates_EnableTemplatesFalse(t *testing.T) {
	cfg := NewConfig()

	// Add a default template that was enabled and has been changed
	cfg.TaskTemplates = []template.TaskTemplate{
		{
			ID:       "def-bug-report",
			Name:     "Bug Report MODIFIED",
			Enabled:  true,
			Source:   "default",
			Category: "bug",
			Type:     models.TaskTypeBug,
		},
	}

	updated, _, _, _ := cfg.ResetChangedDefaultTemplates(false)

	if updated == 0 {
		t.Fatal("expected at least 1 updated template")
	}

	tmpl := cfg.GetTaskTemplate("def-bug-report")
	if tmpl == nil {
		t.Fatal("expected to find template def-bug-report")
	}
	// Should preserve the original enabled state (true) when enableTemplates=false
	if !tmpl.Enabled {
		t.Error("expected template to preserve its enabled state when enableTemplates=false")
	}
}

func TestResetChangedDefaultTemplates_ReaddedEnabled(t *testing.T) {
	cfg := NewConfig()

	// Start with no templates — all defaults were "deleted"
	cfg.TaskTemplates = []template.TaskTemplate{}

	_, readded, _, _ := cfg.ResetChangedDefaultTemplates(true)

	if readded == 0 {
		t.Fatal("expected at least 1 re-added template")
	}

	for _, tmpl := range cfg.TaskTemplates {
		if tmpl.Source == "default" && !tmpl.Enabled {
			t.Errorf("re-added template %q should be enabled when enableTemplates=true", tmpl.ID)
		}
	}
}

func TestResetChangedDefaultTemplates_ReaddedDisabled(t *testing.T) {
	cfg := NewConfig()

	// Start with no templates — all defaults were "deleted"
	cfg.TaskTemplates = []template.TaskTemplate{}

	_, readded, _, _ := cfg.ResetChangedDefaultTemplates(false)

	if readded == 0 {
		t.Fatal("expected at least 1 re-added template")
	}

	for _, tmpl := range cfg.TaskTemplates {
		if tmpl.Source == "default" && tmpl.Enabled {
			t.Errorf("re-added template %q should be disabled when enableTemplates=false", tmpl.ID)
		}
	}
}
