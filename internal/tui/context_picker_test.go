package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestContextPickerShowsArchivedHiddenCountForCustomContext(t *testing.T) {
	cfg := newTestConfig()
	active := &models.Project{ID: "p1", Slug: "active", Name: "Active Project"}
	archived := &models.Project{ID: "p2", Slug: "archived", Name: "Archived Project", Archived: true}
	setTestProjects(cfg, []*models.Project{active, archived})
	cfg.Machine().SetCustomContext("Work", []string{"p1", "p2", "missing"})

	contexts := buildContextListWithCustom(cfg.ActiveProjects(), testRuntimeAt(cfg.DataDir(), cfg))
	var work ContextInfo
	found := false
	for _, ctx := range contexts {
		if ctx.IsCustom && ctx.Name == "Work" {
			work = ctx
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Work custom context")
	}
	if work.ProjectCount != 1 {
		t.Fatalf("active project count = %d, want 1", work.ProjectCount)
	}
	if work.ArchivedProjectCount != 1 {
		t.Fatalf("archived hidden project count = %d, want 1", work.ArchivedProjectCount)
	}

	picker := NewContextPickerModelFromRuntime(testRuntimeAt(cfg.DataDir(), cfg))
	picker.SetSize(100, 24)
	rendered := ansi.Strip(picker.renderContextPicker(picker.renderCurrentFilterHeader()))
	normalized := strings.Join(strings.Fields(rendered), " ")
	for _, want := range []string{
		"+ Work",
		"(1 active, 1 archived project hidden)",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("rendered context picker missing %q\n\n%s", want, rendered)
		}
	}
}
