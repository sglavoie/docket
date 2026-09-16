package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
)

// zoomChromeApp is the filter-chord fixture with explicit fresh touch times, so
// its age remains irrelevant to the workflow-section chrome under test.
func zoomChromeApp(t *testing.T) *AppModel {
	t.Helper()
	app := newFilterChordTestApp(t)
	for _, project := range app.crossOutliner.Population.Projects {
		for _, task := range project.Tasks {
			task.UpdatedAt = time.Now()
		}
	}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return app
}

func TestZoomEmptiedUnderAFilterKeepsTheSectionCause(t *testing.T) {
	app := zoomChromeApp(t)
	out := app.crossOutliner
	zoomIntoSection(t, out, models.StateGroupBacklog.Label())

	out.Filter.Tags = map[string]bool{"missing": true}
	adoptFilterCarriers(out)
	out.Population.Invalidate()
	out.RebuildRows()
	out.ClampCursor()

	if len(out.Rows) != 0 {
		t.Fatalf("the filtered zoom kept %d rows, want an empty projection", len(out.Rows))
	}
	if !out.Focus.Zoom.Active() {
		t.Fatal("the emptied zoom unwound itself instead of staying put")
	}
	if out.Cursor != 0 {
		t.Fatalf("clampCursor left the zero-row list at cursor %d", out.Cursor)
	}
	if !query.HasActiveCommonFilters(&out.Filter.CommonFilterState) {
		t.Fatal("the fixture lost the filter this ordering is about")
	}

	if got := out.ClassifyEmptyCause(); got != outliner.OutlinerEmptySection {
		t.Fatalf("empty cause = %d, want the section cause to outrank the filter cause", got)
	}
	const want = "No tasks are left in this section. Press Esc to unzoom."
	if got := out.OutlinerEmptyMessage(); got != want {
		t.Fatalf("empty message = %q, want %q", got, want)
	}
	if !strings.Contains(ansi.Strip(out.RenderOutlinerTreeContent(80)), want) {
		t.Fatal("the zero-row zoom did not render the empty state")
	}

	spec, ok := out.ContextualGuidance()
	if !ok || len(spec.Hints) == 0 || !spec.Hints[0].Required {
		t.Fatalf("empty zoom guidance = (%+v, %v), want a required recovery act", spec, ok)
	}
	got := guidanceEntryText(spec)
	if !strings.Contains(got, "Esc unzoom") || strings.Contains(got, "clear filters") {
		t.Fatalf("empty zoom guidance = %q, want the unzoom act rather than the filter chord", got)
	}
}

func TestZoomedHeaderGuidanceOffersUnzoomInsteadOfDeadKeys(t *testing.T) {
	app := zoomChromeApp(t)
	out := app.crossOutliner
	backlog := models.StateGroupBacklog.Label()

	out.Cursor = sectionHeaderIndex(t, out, backlog)
	spec, ok := out.ContextualGuidance()
	if !ok {
		t.Fatal("the unzoomed section header produced no guidance")
	}
	if before := guidanceEntryText(spec); !strings.Contains(before, "Enter") ||
		!strings.Contains(before, "collapse") || !strings.Contains(before, "s sections") {
		t.Fatalf("unzoomed header guidance = %q, want the acts the zoom later removes", before)
	}

	zoomIntoSection(t, out, backlog)
	if !out.IsZoomedSectionRow(out.Rows[out.Cursor]) {
		t.Fatalf("cursor row %d is not the zoomed header", out.Cursor)
	}
	spec, ok = out.ContextualGuidance()
	if !ok || spec.Rows != 1 {
		t.Fatalf("zoomed header guidance = (%+v, %v), want a one-row profile", spec, ok)
	}
	got := guidanceEntryText(spec)
	if !strings.Contains(got, "Esc unzoom") {
		t.Fatalf("zoomed header guidance omits the unzoom act: %q", got)
	}
	for _, dead := range []string{"Enter", "collapse", "expand", "s sections"} {
		if strings.Contains(got, dead) {
			t.Errorf("zoomed header guidance names %q, which cannot act there: %q", dead, got)
		}
	}
	index := slices.IndexFunc(spec.Hints, func(hint chrome.GuidanceHint) bool { return hint.Key == "Esc" })
	if index < 0 || !spec.Hints[index].Required {
		t.Fatalf("Esc unzoom is not a required hint: %+v", spec.Hints)
	}
}

func TestZoomTitleNamesTheSectionAndLosesItFirstAtNarrowWidth(t *testing.T) {
	app := zoomChromeApp(t)
	out := app.crossOutliner
	out.Filter.Priority = models.PriorityHigh
	adoptFilterCarriers(out)
	out.Population.Invalidate()
	out.RebuildRows()
	zoomIntoSection(t, out, models.StateGroupBacklog.Label())

	context := out.BuildOutlinerInteractionContext(160)
	if len(context) == 0 {
		t.Fatal("the zoomed title carries no interaction context")
	}
	if last := ansi.Strip(context[len(context)-1]); last != "Section: "+models.StateGroupBacklog.Label() {
		t.Fatalf("last context segment = %q, want the Section segment", last)
	}

	wide := ansi.Strip(out.BuildOutlinerTitleLine(160))
	if !strings.Contains(wide, "Filters:") || !strings.Contains(wide, "Section: "+models.StateGroupBacklog.Label()) {
		t.Fatalf("wide title = %q, want the filter and section segments whole", wide)
	}

	// The Context region is joined before it is fitted, so the trailing Section
	// segment is cut rather than dropped: the filter segment ahead of it survives.
	const narrow = 42
	title := out.BuildOutlinerTitleLine(narrow)
	if lipgloss.Width(title) > narrow {
		t.Fatalf("narrow title rendered %d cells: %q", lipgloss.Width(title), ansi.Strip(title))
	}
	plain := ansi.Strip(title)
	if !strings.Contains(plain, "Filters:") {
		t.Fatalf("narrow title = %q, want the earlier filter segment kept", plain)
	}
	if strings.Contains(plain, "Section: "+models.StateGroupBacklog.Label()) {
		t.Fatalf("narrow title = %q, want the Section segment to be the first content cut", plain)
	}
}
