package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// The parked summary is 93 columns and the form fits it to
// width-FormLabelWidth-len("parked")-1, so 72 (the committed minimum layout)
// clips it while 140 shows the whole sentence under the field.
const (
	projectEditorClippedWidth = 72
	projectEditorWholeWidth   = 140
)

func TestProjectEditorFinishesTheClippedTagsExplainer(t *testing.T) {
	editor := &ProjectEditorModel{}
	continuation := editor.FooterContinuation(projectEditorClippedWidth)
	if continuation == "" {
		t.Fatalf("expected a continuation at width %d, where the explainer clips", projectEditorClippedWidth)
	}

	summary := models.BehavioralProjectTags[models.ParkedProjectTag].Summary
	if !strings.HasSuffix(summary, continuation) {
		t.Fatalf("continuation %q does not finish %q", continuation, summary)
	}

	body := ansi.Strip(behavioralProjectTagExplainer(
		ValueProviderProjectTags,
		projectEditorExplainerWidth(projectEditorClippedWidth),
	).Body)
	if strings.Contains(body, continuation) {
		t.Fatalf("the form already shows %q, so the status bar row would only repeat it:\n%s", continuation, body)
	}
}

func TestProjectEditorHasNothingToContinueWhenTheExplainerFits(t *testing.T) {
	editor := &ProjectEditorModel{}
	if continuation := editor.FooterContinuation(projectEditorWholeWidth); continuation != "" {
		t.Fatalf("expected no continuation at width %d, got %q", projectEditorWholeWidth, continuation)
	}
}
