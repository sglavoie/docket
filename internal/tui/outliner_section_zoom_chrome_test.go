package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// zoomChromeApp is the filter-chord fixture with fresh touch times, so its tasks
// reach their workflow-state groups instead of being swept into the Stale lane
// that a zero UpdatedAt puts them in.
func zoomChromeApp(t *testing.T) *AppModel {
	t.Helper()
	app := newFilterChordTestApp(t)
	for _, project := range app.crossOutliner.loadedProjects {
		for _, task := range project.Tasks {
			task.UpdatedAt = time.Now()
		}
	}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

func TestZoomEmptiedUnderAFilterKeepsTheSectionCause(t *testing.T) {
	app := zoomChromeApp(t)
	out := app.crossOutliner
	zoomIntoSection(t, out, models.StateGroupBacklog.Label())

	out.filter.Tags = map[string]bool{"missing": true}
	out.crossForceRebuild = true
	out.rebuildRows()
	out.clampCursor()

	if len(out.rows) != 0 {
		t.Fatalf("the filtered zoom kept %d rows, want an empty projection", len(out.rows))
	}
	if !out.zoom.active() {
		t.Fatal("the emptied zoom unwound itself instead of staying put")
	}
	if out.cursor != 0 {
		t.Fatalf("clampCursor left the zero-row list at cursor %d", out.cursor)
	}
	if !hasActiveCommonFilters(&out.filter.CommonFilterState) {
		t.Fatal("the fixture lost the filter this ordering is about")
	}

	if got := out.classifyEmptyCause(); got != outlinerEmptySection {
		t.Fatalf("empty cause = %d, want the section cause to outrank the filter cause", got)
	}
	const want = "No tasks are left in this section. Press Esc to return."
	if got := out.outlinerEmptyMessage(); got != want {
		t.Fatalf("empty message = %q, want %q", got, want)
	}
	if !strings.Contains(ansi.Strip(out.renderOutlinerTreeContent(80)), want) {
		t.Fatal("the zero-row zoom did not render the empty state")
	}

	spec, ok := out.ContextualGuidance()
	if !ok || len(spec.Hints) == 0 || !spec.Hints[0].Required {
		t.Fatalf("empty zoom guidance = (%+v, %v), want a required recovery act", spec, ok)
	}
	got := guidanceEntryText(spec)
	if !strings.Contains(got, "Esc return") || strings.Contains(got, "clear filters") {
		t.Fatalf("empty zoom guidance = %q, want Esc return rather than the filter chord", got)
	}
}

func TestZoomedHeaderGuidanceOffersUnzoomInsteadOfDeadKeys(t *testing.T) {
	app := zoomChromeApp(t)
	out := app.crossOutliner
	backlog := models.StateGroupBacklog.Label()

	out.cursor = sectionHeaderIndex(t, out, backlog)
	spec, ok := out.ContextualGuidance()
	if !ok {
		t.Fatal("the unzoomed section header produced no guidance")
	}
	if before := guidanceEntryText(spec); !strings.Contains(before, "Enter") ||
		!strings.Contains(before, "collapse") || !strings.Contains(before, "s sections") {
		t.Fatalf("unzoomed header guidance = %q, want the acts the zoom later removes", before)
	}

	zoomIntoSection(t, out, backlog)
	if !out.isZoomedSectionRow(out.rows[out.cursor]) {
		t.Fatalf("cursor row %d is not the zoomed header", out.cursor)
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
	index := slices.IndexFunc(spec.Hints, func(hint GuidanceHint) bool { return hint.Key == "Esc" })
	if index < 0 || !spec.Hints[index].Required {
		t.Fatalf("Esc unzoom is not a required hint: %+v", spec.Hints)
	}
}

func TestZoomTitleNamesTheSectionAndLosesItFirstAtNarrowWidth(t *testing.T) {
	app := zoomChromeApp(t)
	out := app.crossOutliner
	out.filter.Priority = models.PriorityHigh
	out.crossForceRebuild = true
	out.rebuildRows()
	zoomIntoSection(t, out, models.StateGroupBacklog.Label())

	context := out.buildOutlinerInteractionContext(160)
	if len(context) == 0 {
		t.Fatal("the zoomed title carries no interaction context")
	}
	if last := ansi.Strip(context[len(context)-1]); last != "Section: "+models.StateGroupBacklog.Label() {
		t.Fatalf("last context segment = %q, want the Section segment", last)
	}

	wide := ansi.Strip(out.buildOutlinerTitleLine(160))
	if !strings.Contains(wide, "Filters:") || !strings.Contains(wide, "Section: "+models.StateGroupBacklog.Label()) {
		t.Fatalf("wide title = %q, want the filter and section segments whole", wide)
	}

	// The Context region is joined before it is fitted, so the trailing Section
	// segment is cut rather than dropped: the filter segment ahead of it survives.
	const narrow = 56
	title := out.buildOutlinerTitleLine(narrow)
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
