package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

func TestBehavioralProjectTagRowIsShortButPreviewStaysComplete(t *testing.T) {
	source := (&AppModel{}).projectTagListSource(&models.Project{})
	if len(source.Items) == 0 {
		t.Fatal("project tag source has no behavioral row")
	}
	row := source.Items[0]
	info := models.BehavioralProjectTags[models.ParkedProjectTag]
	if row.PrimaryText != models.ParkedProjectTag || row.SecondaryText != info.RowSummary {
		t.Fatalf("behavioral row = %q / %q, want %q / %q", row.PrimaryText, row.SecondaryText, models.ParkedProjectTag, info.RowSummary)
	}
	if row.SecondaryText != "Parks this project" {
		t.Fatalf("parked row summary = %q", row.SecondaryText)
	}
	if len(row.Preview.Fields) == 0 || row.Preview.Fields[0].Value != info.Summary {
		t.Fatalf("parked preview does not retain full summary %q", info.Summary)
	}
}

// TestProjectTagFilterPickerRowsWearTheSameColorsAsTheEditor covers the one
// project-tag surface that builds its own rows instead of sharing
// projectTagListSource, so parked cannot render in warning at # and plain at F#.
// Untagged is exempt by name: it stands for the empty tag set, so there is no
// tag whose color it could honestly wear.
func TestProjectTagFilterPickerRowsWearTheSameColorsAsTheEditor(t *testing.T) {
	app := newUITestApp()
	setTestAppProjects(app,
		&models.Project{ID: "held", Name: "Held", Slug: "held", Tags: []string{models.ParkedProjectTag}},
		&models.Project{ID: "platform", Name: "Platform", Slug: "platform", Tags: []string{"infra"}},
		&models.Project{ID: "bare", Name: "Bare", Slug: "bare"},
	)
	picker, err := app.newFilterProjectTagsPicker(
		operation.OperationExecutionRequest{ID: operation.OperationFilterProjectTags},
	)
	if err != nil {
		t.Fatalf("newFilterProjectTagsPicker: %v", err)
	}

	colored := 0
	for _, item := range picker.surface.Items() {
		if item.Data == query.UntaggedProjectFilterKey {
			if item.PrimaryStyle != nil {
				t.Fatalf("the untagged row wears %v, want no tag color", item.PrimaryStyle.GetForeground())
			}
			continue
		}
		assertRowWearsTheColorItApplies(t, item)
		colored++
	}
	if colored != 2 {
		t.Fatalf("colored %d facet rows, want the parked and infra rows", colored)
	}
}

func TestBehavioralProjectTagExplainerWrapsEverySummaryInline(t *testing.T) {
	for _, width := range []int{40, 49, 60} {
		explainer := behavioralProjectTagExplainer(operation.ValueProviderProjectTags, width)
		plain := ansi.Strip(explainer)
		for _, line := range strings.Split(plain, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %q is %d columns wide", width, line, got)
			}
		}
		info := models.BehavioralProjectTags[models.ParkedProjectTag]
		if got, want := strings.Join(strings.Fields(plain), " "), info.Name+" "+info.Summary; got != want {
			t.Fatalf("width %d wrapped explainer = %q, want %q", width, got, want)
		}
	}
}

func TestBehavioralProjectTagExplainerKeepsWideSummaryOnOneLine(t *testing.T) {
	explainer := behavioralProjectTagExplainer(operation.ValueProviderProjectTags, 200)
	info := models.BehavioralProjectTags[models.ParkedProjectTag]
	if got := ansi.Strip(explainer); got != info.Name+" "+info.Summary {
		t.Fatalf("wide explainer = %q, want the full summary", got)
	}
}

func TestBehavioralProjectTagExplainerIsSilentForOtherProviders(t *testing.T) {
	for _, provider := range []operation.ValueProviderID{
		operation.ValueProviderTags,
		operation.ValueProviderFilterTags,
		operation.ValueProviderFilterProjectTags,
		operation.ValueProviderPriority,
		operation.ValueProviderState,
	} {
		if got := behavioralProjectTagExplainer(provider, 60); got != "" {
			t.Fatalf("provider %q explainer = %q, want empty", provider, got)
		}
	}
}
