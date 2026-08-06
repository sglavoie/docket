package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
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

func TestBehavioralProjectTagExplainerWrapsEverySummaryInline(t *testing.T) {
	for _, width := range []int{40, 49, 60} {
		explainer := behavioralProjectTagExplainer(ValueProviderProjectTags, width)
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
	explainer := behavioralProjectTagExplainer(ValueProviderProjectTags, 200)
	info := models.BehavioralProjectTags[models.ParkedProjectTag]
	if got := ansi.Strip(explainer); got != info.Name+" "+info.Summary {
		t.Fatalf("wide explainer = %q, want the full summary", got)
	}
}

func TestBehavioralProjectTagExplainerIsSilentForOtherProviders(t *testing.T) {
	for _, provider := range []ValueProviderID{
		ValueProviderTags,
		ValueProviderFilterTags,
		ValueProviderFilterProjectTags,
		ValueProviderPriority,
		ValueProviderState,
	} {
		if got := behavioralProjectTagExplainer(provider, 60); got != "" {
			t.Fatalf("provider %q explainer = %q, want empty", provider, got)
		}
	}
}
