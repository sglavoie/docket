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

// TestZoomTitleNamesTheSectionAndLosesItFirstAtNarrowWidth is the context row's
// overflow contract read through the section axis. Section is the last entry, so
// it is the first segment the row gives up; what the row must never do is end
// where it ran out of room, and what it gives up must still be readable whole
// under the key its suffix names.
func TestZoomTitleNamesTheSectionAndLosesItFirstAtNarrowWidth(t *testing.T) {
	app := zoomChromeApp(t)
	out := app.crossOutliner
	out.Filter.Priority = models.PriorityHigh
	adoptFilterCarriers(out)
	out.Population.Invalidate()
	out.RebuildRows()
	zoomIntoSection(t, out, models.StateGroupBacklog.Label())
	// The query is installed after the zoom, because zooming resolves a section
	// header the ranked projection does not draw.
	out.SetSearchQuery("authentication")
	out.RebuildRows()

	context := out.BuildViewContextEntries(160)
	if len(context) < 2 {
		t.Fatalf("the zoomed view carries %d context entries, want the axes this test narrows", len(context))
	}
	if last := context[len(context)-1]; last.Label != "section" || last.Value != models.StateGroupBacklog.Label() {
		t.Fatalf("last context entry = %+v, want the section entry", last)
	}

	wide := ansi.Strip(out.BuildOutlinerContextRow(160))
	if !strings.Contains(wide, "Filters:") || !strings.Contains(wide, "Section: "+models.StateGroupBacklog.Label()) {
		t.Fatalf("wide context row = %q, want the filter and section segments whole", wide)
	}

	// Segments go whole and from the right, and every step past the first reserves
	// the overflow suffix, so the narrowed row still names how much it shed and
	// where the whole of it is.
	const narrow = 48
	row := out.BuildOutlinerContextRow(narrow)
	if lipgloss.Width(row) > narrow {
		t.Fatalf("narrow context row rendered %d cells: %q", lipgloss.Width(row), ansi.Strip(row))
	}
	plain := ansi.Strip(row)
	if strings.Contains(plain, "Section: "+models.StateGroupBacklog.Label()) {
		t.Fatalf("narrow context row = %q, want the Section segment dropped first", plain)
	}
	if !strings.Contains(plain, "more") || !strings.Contains(plain, "Q details") {
		t.Fatalf("narrow context row = %q, want it to name what it dropped and where to read it", plain)
	}

	// The shed value is not lost, only moved: the readout the suffix names spells
	// every axis the row could not.
	out.ToggleQueryReadout()
	readout := ansi.Strip(out.QueryReadoutBody(160))
	if !strings.Contains(readout, models.StateGroupBacklog.Label()) {
		t.Fatalf("readout body = %q, want the section the row shed", readout)
	}
}
