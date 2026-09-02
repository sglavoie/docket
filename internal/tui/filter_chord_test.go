package tui

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func newFilterChordTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	project := &models.Project{
		ID:   "project",
		Name: "Project",
		Slug: "project",
		Tasks: []*models.Task{
			{ID: "critical", ShortID: 1, Title: "Critical", WorkflowState: models.StateBacklog, Priority: 1},
			{ID: "medium", ShortID: 2, Title: "Medium", WorkflowState: models.StateBacklog, Priority: 2, Review: true},
			{ID: "low", ShortID: 3, Title: "Low", WorkflowState: models.StateWaiting, Priority: 3, Tags: []string{"deep"}},
		},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

func pressFilterChord(t *testing.T, app *AppModel, keys string) tea.Cmd {
	t.Helper()
	var cmd tea.Cmd
	var handled bool
	for _, key := range keys {
		handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: key, Text: string(key)})
		if !handled {
			t.Fatalf("key %q of chord %q was not handled", key, keys)
		}
	}
	if len(app.pendingChord) != 0 {
		t.Fatalf("chord %q left pending prefix %v", keys, app.pendingChord)
	}
	return cmd
}

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
		"$ states",
		"# tags",
		"c contexts",
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
	taskFilter := &app.crossOutliner.filter.CommonFilterState
	taskFilter.Priority = models.PriorityHigh
	taskFilter.Tags = map[string]bool{"triage": true, "deep": true}
	taskFilter.Deferred = true
	taskFilter.Review = true
	taskFilter.Hierarchy = HierarchyParent
	taskFilter.Stale = true
	terminal, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	taskFilter.setStateFilter(terminal)
	taskFilter.setCompletionAgeFilter(CompletionAge90Days)
	taskFilter.Contexts = map[string]bool{"errand": true}
	taskFilter.NoContext = true

	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'f', Text: "f"}); !handled {
		t.Fatal("f did not open its pending chord")
	}
	row := pendingChordHintRowForTest(app)
	for _, want := range []string{
		"! priority:high+", "$ states:2", "a completed:90d+", "@ ?deferred",
		"# tags:2", "c contexts:2", "= review:on", "h hierarchy:parent", "s stale:on",
		"x clear:8",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("decorated f row %q omits %q", row, want)
		}
	}
	if got := app.crossOutliner.ActiveFilterCount(); got != 8 {
		t.Errorf("task filter count = %d, want 8 independently clearable facets", got)
	}

	// The f@ row now names two mutually exclusive facets, so the due-date branch
	// needs its own reading once the deferred one is cleared. Both readings are the
	// token the picker's row writes, not a label of the chord's own.
	taskFilter.Deferred = false
	taskFilter.DueDate = DueDateToday
	row = pendingChordHintRowForTest(app)
	if !strings.Contains(row, "@ @today") {
		t.Errorf("decorated f row %q omits %q", row, "@ @today")
	}

	app.pendingChord = []string{"F"}
	projectFilter := &app.crossOutliner.projectFilter
	projectFilter.Tags = map[string]bool{"client": true}
	projectFilter.Untagged = true
	projectFilter.ParkedMode = projectPopulationInclude
	projectFilter.ArchivedMode = projectPopulationInclude
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{
		"# project tags:3", "p parked:on", "P only parked",
		"a archived:on", "A only archived", "x clear:3",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("decorated F row %q omits %q", row, want)
		}
	}

	projectFilter.ParkedMode = projectPopulationOnly
	projectFilter.ArchivedMode = projectPopulationOnly
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{
		"p include parked", "P parked:only", "a include archived", "A archived:only",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("isolated F row %q omits %q", row, want)
		}
	}
	if got := app.crossOutliner.ActiveFilterCount(); got != 11 {
		t.Errorf("combined filter count = %d, want 11 independent task/project facets", got)
	}
}

func TestReviewFilterChordTogglesTheStoredReviewFacet(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.filter.CommonFilterState
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
	filter := &app.crossOutliner.filter.CommonFilterState
	now := time.Now()
	app.crossOutliner.loadedProjects[0].Tasks[0].UpdatedAt = now.AddDate(0, 0, -models.StaleTaskThresholdDays)
	for _, task := range app.crossOutliner.loadedProjects[0].Tasks[1:] {
		task.UpdatedAt = now
	}
	app.crossOutliner.rebuildRows()

	pressFilterChord(t, app, "fs")
	if !filter.Stale {
		t.Fatal("fs did not activate the Stale facet")
	}
	if visible := visibleOutlinerTaskIDs(app.crossOutliner); !maps.Equal(visible, map[string]bool{"critical": true}) {
		t.Fatalf("Stale filter visible = %v, want only critical", visible)
	}
	if got := commonFilterParts(filter); !slices.Equal(got, []string{"?stale"}) {
		t.Fatalf("Stale badge parts = %v, want ?stale", got)
	}

	pressFilterChord(t, app, "fs")
	if filter.Stale {
		t.Fatal("second fs did not clear the Stale facet")
	}
}
