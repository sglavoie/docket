package projecthub

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
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
