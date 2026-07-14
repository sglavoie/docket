package config

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

func TestResetChangedDefaultTemplates_EnableTemplatesTrue(t *testing.T) {
	cfg := NewAutomationConfig()

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
	cfg := NewAutomationConfig()

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
	cfg := NewAutomationConfig()

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

// TestResetChangedDefaultTemplates_TagOrderDetected pins the fix for the
// set-vs-list bug in the prior templatesAreDifferent helper: reordering Tags
// must now be treated as a content change so ResetChangedDefaultTemplates
// rewrites the user-edited template back to the shipped order.
func TestResetChangedDefaultTemplates_TagOrderDetected(t *testing.T) {
	cfg := NewAutomationConfig()

	// Find a shipped default with >1 tags so a reorder is meaningful.
	defaults := template.GetDefaultTemplates()
	var shipped *template.TaskTemplate
	for i := range defaults {
		if len(defaults[i].Tags) >= 2 {
			shipped = &defaults[i]
			break
		}
	}
	if shipped == nil {
		t.Skip("no default template with multiple tags available")
	}

	// Clone shipped values exactly, but reverse the tag order.
	modified := *shipped
	modified.Tags = append([]string(nil), shipped.Tags...)
	for i, j := 0, len(modified.Tags)-1; i < j; i, j = i+1, j-1 {
		modified.Tags[i], modified.Tags[j] = modified.Tags[j], modified.Tags[i]
	}
	// Sanity: tags must actually differ in order.
	if modified.Tags[0] == shipped.Tags[0] {
		t.Fatalf("test setup failed: tags not reordered (shipped=%v modified=%v)", shipped.Tags, modified.Tags)
	}
	modified.Enabled = false
	cfg.TaskTemplates = []template.TaskTemplate{modified}

	updated, _, unchanged, affectedIDs := cfg.ResetChangedDefaultTemplates(false)

	if updated == 0 {
		t.Fatalf("expected reordered-tag template to be detected as updated, got updated=%d unchanged=%d", updated, unchanged)
	}
	found := slices.Contains(affectedIDs, shipped.ID)
	if !found {
		t.Errorf("expected %s in affectedIDs, got %v", shipped.ID, affectedIDs)
	}

	got := cfg.GetTaskTemplate(shipped.ID)
	if got == nil {
		t.Fatalf("template %s missing after reset", shipped.ID)
	}
	for i, tag := range shipped.Tags {
		if got.Tags[i] != tag {
			t.Errorf("tag order not restored: got %v, want %v", got.Tags, shipped.Tags)
			break
		}
	}
}

// TestResetChangedDefaultTemplates_IgnoresMetadata verifies the
// instance-level fields (Enabled, UseCount) are not treated as content
// differences by the ContentEqual switch.
func TestResetChangedDefaultTemplates_IgnoresMetadata(t *testing.T) {
	cfg := NewAutomationConfig()

	defaults := template.GetDefaultTemplates()
	if len(defaults) == 0 {
		t.Skip("no default templates available")
	}
	shipped := defaults[0]

	// Exact content clone, but flip Enabled and bump UseCount.
	clone := shipped
	clone.Enabled = !shipped.Enabled
	clone.UseCount = 42
	cfg.TaskTemplates = []template.TaskTemplate{clone}

	updated, _, unchanged, _ := cfg.ResetChangedDefaultTemplates(false)
	if updated != 0 {
		t.Errorf("metadata-only differences should not count as updates, got updated=%d", updated)
	}
	if unchanged == 0 {
		t.Errorf("expected at least one unchanged template, got 0")
	}
	got := cfg.GetTaskTemplate(shipped.ID)
	if got == nil {
		t.Fatalf("template %s missing", shipped.ID)
	}
	if got.UseCount != 42 {
		t.Errorf("UseCount should be preserved across unchanged content, got %d want 42", got.UseCount)
	}
}

func TestResetChangedDefaultTemplates_ReaddedDisabled(t *testing.T) {
	cfg := NewAutomationConfig()

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
