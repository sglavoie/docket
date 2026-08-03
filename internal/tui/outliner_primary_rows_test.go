package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func primaryRowsTask(id string, state models.WorkflowState) *models.Task {
	return &models.Task{
		ID:            id,
		Title:         id,
		WorkflowState: state,
		UpdatedAt:     time.Now(),
	}
}

func primaryRowsProject(id string, tasks ...*models.Task) *models.Project {
	project := &models.Project{ID: id, Name: id, Slug: id, Tasks: tasks}
	project.BuildTaskTree()
	return project
}

func primaryRowsOutliner(projects ...*models.Project) (*OutlinerModel, *config.RuntimeState) {
	runtime := testConfigWithProjects(projects...)
	outliner := NewOutlinerModel(nil, nil, runtime)
	outliner.loadedProjects = projects
	return outliner, runtime
}

func TestPrimaryAllUsesCanonicalGroupsAndDeduplicatesToday(t *testing.T) {
	tasks := []*models.Task{
		primaryRowsTask("in-progress", models.StateDoing),
		primaryRowsTask("inbox", models.StateInbox),
		primaryRowsTask("next", models.StateBacklog),
		primaryRowsTask("today", models.StateBacklog),
		primaryRowsTask("backlog", models.StateBacklog),
		primaryRowsTask("waiting", models.StateWaiting),
		primaryRowsTask("paused", models.StatePaused),
		primaryRowsTask("someday", models.StateSomeday),
		primaryRowsTask("done", models.StateDone),
		primaryRowsTask("canceled", models.StateCanceled),
		primaryRowsTask("snoozed", models.StateBacklog),
	}
	future := time.Now().AddDate(0, 0, 2)
	tasks[len(tasks)-1].SnoozedUntil = &future
	project := primaryRowsProject("alpha", tasks...)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    "today",
	}})
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    "next",
	}})

	outliner.selectOutlinerMode(ModeAll)

	var groups []models.WorkflowStateGroup
	for _, row := range outliner.rows {
		if row.IsSectionHeader {
			groups = append(groups, row.StateGroup)
		}
	}
	wantGroups := models.AllModeWorkflowStateGroups()
	if !slices.Equal(groups, wantGroups) {
		t.Fatalf("All group order = %v, want %v", groups, wantGroups)
	}
	if got := countTaskRows(outliner, "today"); got != 1 {
		t.Fatalf("Today task row count = %d, want one deduplicated row", got)
	}
	if row, ok := findPlanningTaskRow(outliner, "today"); !ok || !row.IsTodaySection {
		t.Fatalf("Today task row = %+v found=%t, want synthetic Today membership", row, ok)
	}
	if row, ok := findPlanningTaskRow(outliner, "next"); !ok || !row.IsNextSection {
		t.Fatalf("Next task row = %+v found=%t, want synthetic Next membership", row, ok)
	}
	for _, hidden := range []string{"done", "canceled", "snoozed"} {
		if got := countTaskRows(outliner, hidden); got != 0 {
			t.Errorf("default All rendered %q %d time(s), want hidden", hidden, got)
		}
	}
	for _, row := range outliner.rows {
		if row.IsSectionHeader && row.SectionName != row.StateGroup.Label() {
			t.Errorf("group %q label = %q, want canonical %q", row.StateGroup, row.SectionName, row.StateGroup.Label())
		}
	}
}

func TestPrimaryAllExplicitFiltersOverrideTerminalAndSnoozedDefaults(t *testing.T) {
	done := primaryRowsTask("done", models.StateDone)
	canceled := primaryRowsTask("canceled", models.StateCanceled)
	snoozed := primaryRowsTask("snoozed", models.StateBacklog)
	future := time.Now().AddDate(0, 0, 2)
	snoozed.SnoozedUntil = &future
	project := primaryRowsProject("alpha", done, canceled, snoozed)
	outliner, runtime := primaryRowsOutliner(project)
	// Stale planning references must not make explicit terminal/snoozed facets
	// lose rows. The application invariant will normally clean these references,
	// but the projection remains defensive while a refresh is in flight.
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: done.ID}})
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: snoozed.ID}})
	outliner.selectOutlinerMode(ModeAll)

	outliner.filter.setStateFilter(testWorkflowStateSet(models.StateDone, models.StateCanceled), true)
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"done", "canceled"}) {
		t.Fatalf("terminal-filter rows = %v, want done/canceled", got)
	}
	var terminalGroups []models.WorkflowStateGroup
	for _, row := range outliner.rows {
		if row.IsSectionHeader {
			terminalGroups = append(terminalGroups, row.StateGroup)
		}
	}
	if !slices.Equal(terminalGroups, []models.WorkflowStateGroup{models.StateGroupDone, models.StateGroupCanceled}) {
		t.Fatalf("terminal groups = %v, want Done then Canceled", terminalGroups)
	}

	clearCommonFilters(&outliner.filter.CommonFilterState)
	outliner.filter.Snoozed = true
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"snoozed"}) {
		t.Fatalf("snoozed-filter rows = %v, want explicit snoozed Today task", got)
	}
	if row, ok := findPlanningTaskRow(outliner, snoozed.ID); !ok || !row.IsTodaySection {
		t.Fatalf("snoozed Today row = %+v found=%t, want explicit planning projection", row, ok)
	}
}

func TestPrimaryModeMembershipHierarchyOrderingAndQualifiedRows(t *testing.T) {
	alphaNext := primaryRowsTask("alpha-next", models.StateBacklog)
	alphaNextChild := primaryRowsTask("alpha-next-child", models.StateBacklog)
	alphaNextChild.ParentID = &alphaNext.ID
	alphaInbox := primaryRowsTask("alpha-inbox", models.StateInbox)
	alphaReady := primaryRowsTask("alpha-ready", models.StateBacklog)
	alphaToday := primaryRowsTask("alpha-today", models.StateBacklog)
	alphaProgress := primaryRowsTask("alpha-progress", models.StateDoing)
	alpha := primaryRowsProject(
		"alpha",
		alphaNext,
		alphaNextChild,
		alphaInbox,
		alphaReady,
		alphaToday,
		alphaProgress,
	)
	betaNext := primaryRowsTask("beta-next", models.StateBacklog)
	betaInbox := primaryRowsTask("beta-inbox", models.StateInbox)
	beta := primaryRowsProject("beta", betaNext, betaInbox)
	outliner, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: beta.ID, TaskID: betaNext.ID},
		{ProjectID: alpha.ID, TaskID: alphaNext.ID},
		{ProjectID: alpha.ID, TaskID: alphaNextChild.ID},
	})
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: alpha.ID,
		TaskID:    alphaToday.ID,
	}})

	outliner.selectOutlinerMode(ModeNext)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		betaNext.ID,
		alphaNext.ID,
		alphaNextChild.ID,
	}) {
		t.Fatalf("Next rows = %v, want NextOrder order with hierarchy intact", got)
	}
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.ProjectID == "" {
			t.Fatalf("cross-project Next row %q lost qualified project identity", row.task.ID)
		}
	}

	outliner.expandedNodes[TaskIdentity{ProjectID: alpha.ID, TaskID: alphaNext.ID}] = false
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaNext.ID, alphaNext.ID, alphaNextChild.ID}) {
		t.Fatalf("collapsed Next rows = %v, want explicit members unaffected by hierarchy folds", got)
	}
	if row, ok := findPlanningTaskRow(outliner, alphaNextChild.ID); !ok || row.Breadcrumb != alphaNext.Title+" ›" {
		t.Fatalf("promoted Next child = %+v found=%t, want ancestor breadcrumb", row, ok)
	}

	outliner.selectOutlinerMode(ModeToday)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaToday.ID}) {
		t.Fatalf("Today rows = %v, want explicit durable membership only", got)
	}

	outliner.selectOutlinerMode(ModeInbox)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaInbox.ID, betaInbox.ID}) {
		t.Fatalf("Inbox rows = %v, want exact StateInbox membership", got)
	}

	outliner.selectOutlinerMode(ModeBacklog)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		alphaNext.ID,
		alphaReady.ID,
		betaNext.ID,
	}) {
		t.Fatalf("Backlog rows = %v, want Backlog lifecycle tasks excluding Today with folded descendants hidden", got)
	}

	outliner.scopedProjectID = beta.ID
	outliner.selectOutlinerMode(ModeInbox)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaInbox.ID}) {
		t.Fatalf("project-scoped Inbox rows = %v, want beta only", got)
	}
}

func TestPrimaryAllFocusedProjectOmitsRedundantProjectHeaders(t *testing.T) {
	alpha := primaryRowsProject(
		"alpha",
		primaryRowsTask("alpha-progress", models.StateDoing),
		primaryRowsTask("alpha-ready", models.StateBacklog),
	)
	outliner, _ := primaryRowsOutliner(alpha)
	outliner.selectOutlinerMode(ModeAll)

	baseProjectHeaders := 0
	for _, row := range outliner.rows {
		if row.IsProjectHeader {
			baseProjectHeaders++
		}
	}
	if baseProjectHeaders == 0 {
		t.Fatal("single-project base scope omitted cross-project sub-headers")
	}

	outliner.pushProjectScope(alpha.ID)

	sectionCount := 0
	taskCount := 0
	for _, row := range outliner.rows {
		switch {
		case row.IsProjectHeader:
			t.Fatalf("focused project rendered redundant project header: %+v", row)
		case row.IsSectionHeader:
			sectionCount++
		case row.isTaskRow():
			taskCount++
			if row.ProjectID != alpha.ID {
				t.Errorf("focused task %q project ID = %q, want %q", row.task.ID, row.ProjectID, alpha.ID)
			}
		}
	}
	if sectionCount != 2 || taskCount != 2 {
		t.Fatalf("focused rows contain %d sections and %d tasks, want 2 and 2", sectionCount, taskCount)
	}

	if !outliner.popProjectScope() {
		t.Fatal("focused project scope did not pop")
	}
	projectHeaders := 0
	for _, row := range outliner.rows {
		if row.IsProjectHeader {
			projectHeaders++
		}
	}
	if projectHeaders == 0 {
		t.Fatal("base scope did not restore cross-project sub-headers")
	}
}
