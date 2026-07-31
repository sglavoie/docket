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
		primaryRowsTask("in-progress", models.StateInProgress),
		primaryRowsTask("inbox", models.StateInbox),
		primaryRowsTask("next", models.StateNext),
		primaryRowsTask("today", models.StateReady),
		primaryRowsTask("backlog", models.StateReady),
		primaryRowsTask("waiting", models.StateWaiting),
		primaryRowsTask("paused", models.StatePaused),
		primaryRowsTask("someday", models.StateSomeday),
		primaryRowsTask("done", models.StateDone),
		primaryRowsTask("canceled", models.StateCanceled),
		primaryRowsTask("snoozed", models.StateReady),
	}
	future := time.Now().AddDate(0, 0, 2)
	tasks[len(tasks)-1].SnoozedUntil = &future
	project := primaryRowsProject("alpha", tasks...)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    "today",
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
	snoozed := primaryRowsTask("snoozed", models.StateReady)
	future := time.Now().AddDate(0, 0, 2)
	snoozed.SnoozedUntil = &future
	project := primaryRowsProject("alpha", done, canceled, snoozed)
	outliner, _ := primaryRowsOutliner(project)
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
		t.Fatalf("snoozed-filter rows = %v, want snoozed Ready task", got)
	}
}

func TestPrimaryModeMembershipHierarchyOrderingAndQualifiedRows(t *testing.T) {
	alphaNext := primaryRowsTask("alpha-next", models.StateNext)
	alphaNextChild := primaryRowsTask("alpha-next-child", models.StateNext)
	alphaNextChild.ParentID = &alphaNext.ID
	alphaInbox := primaryRowsTask("alpha-inbox", models.StateInbox)
	alphaReady := primaryRowsTask("alpha-ready", models.StateReady)
	alphaToday := primaryRowsTask("alpha-today", models.StateReady)
	alphaProgress := primaryRowsTask("alpha-progress", models.StateInProgress)
	alpha := primaryRowsProject(
		"alpha",
		alphaNext,
		alphaNextChild,
		alphaInbox,
		alphaReady,
		alphaToday,
		alphaProgress,
	)
	betaNext := primaryRowsTask("beta-next", models.StateNext)
	betaInbox := primaryRowsTask("beta-inbox", models.StateInbox)
	beta := primaryRowsProject("beta", betaNext, betaInbox)
	outliner, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetNextOrder([]config.ProjectTaskLocator{
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
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaNext.ID, alphaNext.ID}) {
		t.Fatalf("collapsed Next rows = %v, want child hidden by shared fold state", got)
	}

	outliner.selectOutlinerMode(ModeInbox)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaInbox.ID, betaInbox.ID}) {
		t.Fatalf("Inbox rows = %v, want exact StateInbox membership", got)
	}

	outliner.selectOutlinerMode(ModeBacklog)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaReady.ID}) {
		t.Fatalf("Backlog rows = %v, want Ready excluding Today only", got)
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
		primaryRowsTask("alpha-progress", models.StateInProgress),
		primaryRowsTask("alpha-ready", models.StateReady),
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
