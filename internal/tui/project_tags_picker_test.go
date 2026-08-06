package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestBehavioralProjectTagExplainerFinishesEverySummaryItClipped(t *testing.T) {
	for _, width := range []int{40, 49, 60} {
		explainer := behavioralProjectTagExplainer(ValueProviderProjectTags, width)
		block := ansi.Strip(explainer.Body)
		if lipgloss.Width(block) > width {
			t.Fatalf("width %d block %q is %d columns wide", width, block, lipgloss.Width(block))
		}
		continuation := explainer.Continuation()
		if !strings.HasSuffix(continuation, "hides them from Active Projects.") {
			t.Fatalf("width %d continuation = %q, want the end of the parked sentence", width, continuation)
		}
		info := models.BehavioralProjectTags[models.ParkedProjectTag]
		shown := strings.TrimSuffix(strings.TrimPrefix(block, info.Name+" "), "…")
		if shown+" "+continuation != info.Summary {
			t.Fatalf("width %d block %q plus continuation %q is not the summary", width, shown, continuation)
		}
	}
}

func TestBehavioralProjectTagExplainerKeepsTheWholeSummaryWithoutAContinuation(t *testing.T) {
	explainer := behavioralProjectTagExplainer(ValueProviderProjectTags, 200)
	info := models.BehavioralProjectTags[models.ParkedProjectTag]
	if got := ansi.Strip(explainer.Body); got != info.Name+" "+info.Summary {
		t.Fatalf("wide explainer = %q, want the full summary", got)
	}
	if got := explainer.Continuation(); got != "" {
		t.Fatalf("wide continuation = %q, want none", got)
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
		if got := behavioralProjectTagExplainer(provider, 60); got.Body != "" || got.Continuation() != "" {
			t.Fatalf("provider %q explainer = %+v, want the zero value", provider, got)
		}
	}
}
