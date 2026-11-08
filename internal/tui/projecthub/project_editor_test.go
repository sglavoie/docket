package projecthub

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
)

const (
	projectEditorMinimumWidth = 72
	projectEditorWideWidth    = 140
)

func TestProjectEditorWrapsCompleteTagsExplainerAtMinimumWidth(t *testing.T) {
	width := projectEditorExplainerWidth(projectEditorMinimumWidth)
	body := ansi.Strip(BehavioralProjectTagExplainer(
		operation.ValueProviderProjectTags,
		width,
	))
	for _, line := range strings.Split(body, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %q is %d columns wide, want <= %d", line, got, width)
		}
	}
	info := models.BehavioralProjectTags[models.ParkedProjectTag]
	if got, want := strings.Join(strings.Fields(body), " "), info.Name+" "+info.Summary; got != want {
		t.Fatalf("wrapped explainer = %q, want %q", got, want)
	}
}

func TestProjectEditorKeepsWideTagsExplainerOnOneLine(t *testing.T) {
	body := ansi.Strip(BehavioralProjectTagExplainer(
		operation.ValueProviderProjectTags,
		projectEditorExplainerWidth(projectEditorWideWidth),
	))
	if strings.Contains(body, "\n") {
		t.Fatalf("wide explainer wrapped unexpectedly: %q", body)
	}
}

// The alias field reads the stored alias, counts as an edit, and refuses an
// alias the catalog gate would refuse before anything is written.
func TestProjectEditorAliasFieldValidatesBeforeSaving(t *testing.T) {
	project := &models.Project{ID: "p", Slug: "internal-archive", Name: "Internal Archive", Alias: "IA"}
	editor := NewProjectEditorModel(project, nil, nil, nil, false)
	if editor.aliasInput.Value() != "IA" || editor.checkChanges() {
		t.Fatalf("alias field = %q, changed %v", editor.aliasInput.Value(), editor.checkChanges())
	}
	editor.aliasInput.SetValue("I A")
	if !editor.checkChanges() {
		t.Fatal("an edited alias is not a change")
	}
	if _, err := editor.resolveSave(); err == nil || !strings.Contains(err.Error(), "project alias") {
		t.Fatalf("an alias with a space saved: %v", err)
	}
	if !strings.Contains(ansi.Strip(editor.View().Content), "Alias:") {
		t.Fatal("the editor draws no Alias field")
	}
}
