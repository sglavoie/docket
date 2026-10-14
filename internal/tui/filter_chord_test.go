package tui

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
)

func TestFilterChordWhichKeyHints(t *testing.T) {
	app := newFilterChordTestApp(t)

	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'f', Text: "f"}); !handled || cmd != nil {
		t.Fatalf("f = handled %v cmd %T; want a pending prefix", handled, cmd)
	}
	row := pendingChordHintRowForTest(app)
	for _, want := range []string{
		"! priority threshold",
		"a completion age",
		"@ due",
		"w states",
		"# tags",
		"c contexts",
		"% projects",
		"= review",
		"h hierarchy",
		"s stale",
		"x clear all",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("f which-key row %q omits %q", row, want)
		}
	}

	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"}); !handled || cmd != nil {
		t.Fatalf("f! = handled %v cmd %T; want a pending prefix", handled, cmd)
	}
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{"0 clear", "1 high", "2 medium+", "3 low+"} {
		if !strings.Contains(row, want) {
			t.Errorf("f! which-key row %q omits %q", row, want)
		}
	}
}

func TestFilterChordWhichKeyHintsShowEffectiveValues(t *testing.T) {
	app := newFilterChordTestApp(t)
	taskFilter := &app.crossOutliner.Filter.CommonFilterState
	taskFilter.Priority = models.PriorityHigh
	taskFilter.Tags = map[string]bool{"triage": true, "deep": true}
	taskFilter.Deferred = true
	taskFilter.Review = true
	taskFilter.Hierarchy = query.HierarchyParent
	taskFilter.Stale = true
	terminal, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	taskFilter.SetStateFilter(terminal)
	taskFilter.SetCompletionAgeFilter(query.CompletionAge90Days)
	taskFilter.Contexts = map[string]bool{"errand": true}
	taskFilter.NoContext = true
	taskFilter.Projects = []string{"alpha", "beta"}

	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'f', Text: "f"}); !handled {
		t.Fatal("f did not open its pending chord")
	}
	row := pendingChordHintRowForTest(app)
	for _, want := range []string{
		"! priority:high+", "w states:2", "a completed:90d+", "@ ?deferred",
		"# tags:2", "c contexts:2", "% projects:2", "= review:on", "h hierarchy:parent", "s stale:on",
		"x clear:9",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("decorated f row %q omits %q", row, want)
		}
	}
	if got := app.crossOutliner.ActiveFilterCount(); got != 9 {
		t.Errorf("task filter count = %d, want 9 independently clearable facets", got)
	}

	// The f@ row now names two mutually exclusive facets, so the due-date branch
	// needs its own reading once the deferred one is cleared. Both readings are the
	// token the picker's row writes, not a label of the chord's own.
	taskFilter.Deferred = false
	taskFilter.DueDate = query.DueDateToday
	row = pendingChordHintRowForTest(app)
	if !strings.Contains(row, "@ @today") {
		t.Errorf("decorated f row %q omits %q", row, "@ @today")
	}

	app.operations.Chord = []string{"F"}
	projectFilter := &app.crossOutliner.ProjectFilter
	projectFilter.Tags = map[string]bool{"client": true}
	projectFilter.Untagged = true
	projectFilter.ParkedMode = query.ProjectPopulationInclude
	projectFilter.ArchivedMode = query.ProjectPopulationInclude
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{
		"# project tags:3", "p parked:on", "P only parked",
		"a archived:on", "A only archived", "x clear:3",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("decorated F row %q omits %q", row, want)
		}
	}

	projectFilter.ParkedMode = query.ProjectPopulationOnly
	projectFilter.ArchivedMode = query.ProjectPopulationOnly
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{
		"p include parked", "P parked:only", "a include archived", "A archived:only",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("isolated F row %q omits %q", row, want)
		}
	}
	if got := app.crossOutliner.ActiveFilterCount(); got != 12 {
		t.Errorf("combined filter count = %d, want 12 independent task/project facets", got)
	}
}

func TestReviewFilterChordTogglesTheStoredReviewFacet(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.Filter.CommonFilterState
	pressFilterChord(t, app, "f=")
	if !filter.Review {
		t.Fatalf("f= filter = %v", filter.Review)
	}
	if visible := visibleOutlinerTaskIDs(app.crossOutliner); !visible["medium"] || visible["critical"] || visible["low"] {
		t.Fatalf("Review filter visible = %v", visible)
	}
	pressFilterChord(t, app, "f=")
	if filter.Review {
		t.Fatalf("second f= filter = %v", filter.Review)
	}
}

func TestStaleFilterChordTogglesTouchAgeFacet(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.Filter.CommonFilterState
	now := time.Now()
	app.crossOutliner.Population.Projects[0].Tasks[0].UpdatedAt = now.AddDate(0, 0, -models.StaleTaskThresholdDays)
	for _, task := range app.crossOutliner.Population.Projects[0].Tasks[1:] {
		task.UpdatedAt = now
	}
	app.crossOutliner.RebuildRows()

	pressFilterChord(t, app, "fs")
	if !filter.Stale {
		t.Fatal("fs did not activate the Stale facet")
	}
	if visible := visibleOutlinerTaskIDs(app.crossOutliner); !maps.Equal(visible, map[string]bool{"critical": true}) {
		t.Fatalf("Stale filter visible = %v, want only critical", visible)
	}
	if got := query.CommonFilterParts(filter); !slices.Equal(got, []string{"?stale"}) {
		t.Fatalf("Stale badge parts = %v, want ?stale", got)
	}

	pressFilterChord(t, app, "fs")
	if filter.Stale {
		t.Fatal("second fs did not clear the Stale facet")
	}
}

// chordFooterForTest is the pending chord's packed footer as the shell would
// draw it at width, for the prefix just pressed over the cursor's own row.
func chordFooterForTest(t *testing.T, app *AppModel, prefix rune, width int) string {
	t.Helper()
	app.operations.Chord = nil
	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: prefix, Text: string(prefix)}); !handled {
		t.Fatalf("%c did not open its pending chord", prefix)
	}
	spec, ok := app.pendingChordGuidance()
	if !ok {
		t.Fatalf("%c published no chord guidance", prefix)
	}
	return strings.Join(chrome.GuidanceLines(spec, width), "\n")
}

// seatChordTestCursorOnTask moves the cursor onto a task row, where the go-to
// chord's three task entries apply.
func seatChordTestCursorOnTask(t *testing.T, app *AppModel) {
	t.Helper()
	for index, row := range app.crossOutliner.Rows {
		if row.IsTaskRow() {
			app.crossOutliner.Cursor = index
			return
		}
	}
	t.Fatal("the fixture drew no task row")
}

// TestFilterChordFooterIsGroupedAndKeepsClearAll holds the f chord's division:
// two headed runs, each heading riding its first entry, then clear all and the
// project facet outside both. Clear all is the kept entry, so every width down
// to the floor names it, and the project facet declared after it is shed first.
func TestFilterChordFooterIsGroupedAndKeepsClearAll(t *testing.T) {
	app := newFilterChordTestApp(t)

	wide := chordFooterForTest(t, app, 'f', 400)
	last := -1
	for _, entry := range []string{
		chrome.FilterFieldHintGroup + " ! priority threshold",
		"@ due/deferred", "w states", "# tags", "c contexts", "= review", "* pinned",
		chrome.FilterDerivedHintGroup + " s stale",
		"h hierarchy", "~ references", "a completion age",
		"x clear all", "% projects", "Esc cancel",
	} {
		index := strings.Index(wide, entry)
		if index <= last {
			t.Fatalf("f footer %q does not hold %q in declaration order", wide, entry)
		}
		last = index
	}
	for _, heading := range []string{chrome.FilterFieldHintGroup, chrome.FilterDerivedHintGroup} {
		if got := strings.Count(wide, heading); got != 1 {
			t.Fatalf("heading %q drawn %d times, want one contiguous run: %q", heading, got, wide)
		}
	}

	for width := chrome.MinTerminalWidth; width <= 220; width++ {
		footer := chordFooterForTest(t, app, 'f', width)
		for _, line := range strings.Split(footer, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d: line %q is %d columns", width, line, got)
			}
		}
		if !strings.Contains(footer, "x clear all") {
			t.Fatalf("width %d shed clear all:\n%s", width, footer)
		}
		if strings.Contains(footer, chrome.FilterDerivedHintGroup) && !strings.Contains(footer, chrome.FilterDerivedHintGroup+" s stale") {
			t.Fatalf("width %d strands the Derived heading:\n%s", width, footer)
		}
		if !strings.Contains(footer, chrome.FilterFieldHintGroup+" ! priority threshold") {
			t.Fatalf("width %d lost the Field run's head:\n%s", width, footer)
		}
	}

	// The floor is where the row is fullest, and what it gives up is pinned: the
	// completion window, which only a terminal population reads, and the project
	// facet declared behind clear all for exactly this.
	floor := chordFooterForTest(t, app, 'f', chrome.MinTerminalWidth)
	for _, entry := range []string{"@ due/deferred", "w states", "# tags", "c contexts", "= review", "* pinned", "s stale", "h hierarchy", "~ references", "x clear all"} {
		if !strings.Contains(floor, entry) {
			t.Errorf("the floor shed %q:\n%s", entry, floor)
		}
	}
	for _, shed := range []string{"a completion age", "% projects"} {
		if strings.Contains(floor, shed) {
			t.Errorf("the floor kept %q, so something else paid for it:\n%s", shed, floor)
		}
	}
}

// TestGotoChordFooterIsGrouped holds the g chord's three runs behind cursor-top,
// which stands ahead of them unheaded. A task row adds the Task run, and at the
// floor that row's footer gives up the Sync pair whole rather than half of it.
func TestGotoChordFooterIsGrouped(t *testing.T) {
	app := newFilterChordTestApp(t)

	header := chordFooterForTest(t, app, 'g', chrome.MinTerminalWidth)
	for _, entry := range []string{
		"g top", chrome.GotoModeHintGroup + " a all", "i inbox", "t today", "n next", "b backlog", "@ due", "r recent",
		chrome.GotoSyncHintGroup + " s sync now", "c conflicts", "Esc cancel",
	} {
		if !strings.Contains(header, entry) {
			t.Errorf("a header row's g footer shed %q at the floor:\n%s", entry, header)
		}
	}
	if strings.Contains(header, chrome.GotoTaskHintGroup) {
		t.Errorf("a header row drew the Task run:\n%s", header)
	}

	seatChordTestCursorOnTask(t, app)
	wide := chordFooterForTest(t, app, 'g', 400)
	last := -1
	for _, entry := range []string{
		"g top", chrome.GotoModeHintGroup + " a all", "r recent",
		chrome.GotoTaskHintGroup + " p task project", "P parent task", "~ related task",
		chrome.GotoSyncHintGroup + " s sync now", "c conflicts", "Esc cancel",
	} {
		index := strings.Index(wide, entry)
		if index <= last {
			t.Fatalf("g footer %q does not hold %q in declaration order", wide, entry)
		}
		last = index
	}
	for width := chrome.MinTerminalWidth; width <= 220; width++ {
		footer := chordFooterForTest(t, app, 'g', width)
		for _, group := range []struct{ heading, first string }{
			{chrome.GotoModeHintGroup, "a all"},
			{chrome.GotoTaskHintGroup, "p task project"},
			{chrome.GotoSyncHintGroup, "s sync now"},
		} {
			if strings.Contains(footer, group.heading) && !strings.Contains(footer, group.heading+" "+group.first) {
				t.Fatalf("width %d strands the %q heading:\n%s", width, group.heading, footer)
			}
		}
		if !strings.Contains(footer, "~ related task") {
			t.Fatalf("width %d shed a task entry ahead of the Sync pair:\n%s", width, footer)
		}
	}
}
