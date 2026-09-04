package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

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
		primaryRowsTask("deferred", models.StateBacklog),
	}
	future := time.Now().AddDate(0, 0, 2)
	tasks[len(tasks)-1].DeferredUntil = &future
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

	outliner.selectOutlinerMode(chrome.ModeAll)

	var groups []models.WorkflowStateGroup
	for _, row := range outliner.rows {
		if row.IsSectionHeader && row.StateGroup.IsValid() {
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
	for _, hidden := range []string{"done", "canceled", "deferred"} {
		if got := countTaskRows(outliner, hidden); got != 0 {
			t.Errorf("default All rendered %q %d time(s), want hidden", hidden, got)
		}
	}
	for _, row := range outliner.rows {
		if row.IsSectionHeader && row.StateGroup.IsValid() && row.SectionName != row.StateGroup.Label() {
			t.Errorf("group %q label = %q, want canonical %q", row.StateGroup, row.SectionName, row.StateGroup.Label())
		}
	}
}

// TestPrimaryAllPromotesReviewAfterDoingWithoutMutatingMembership pins the
// Review lane to the stored review flag alone. Every other axis stays its own,
// so a task whose context or tag list happens to spell review stays in its
// lifecycle group: the lane reads task.Review and nothing else.
func TestPrimaryAllPromotesReviewAfterDoingWithoutMutatingMembership(t *testing.T) {
	reviewed := primaryRowsTask("reviewed", models.StateBacklog)
	reviewed.Review = true
	reviewedToday := primaryRowsTask("reviewed-today", models.StateBacklog)
	reviewedToday.Review = true
	reviewedNext := primaryRowsTask("reviewed-next", models.StateBacklog)
	reviewedNext.Review = true
	contextReviewed := primaryRowsTask("context-reviewed", models.StateBacklog)
	contextReviewed.Context = reviewLaneKey
	contextReviewed.AddTag(reviewLaneKey)
	doing := primaryRowsTask("doing", models.StateDoing)
	pinned := primaryRowsTask("pinned-review", models.StateDoing)
	pinned.Review = true
	pinned.Pinned = true
	now := time.Now()
	pinned.PinnedAt = &now
	project := primaryRowsProject("alpha", reviewed, reviewedToday, reviewedNext, contextReviewed, doing, pinned)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: reviewedToday.ID}})
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: reviewedNext.ID}})
	outliner.selectOutlinerMode(chrome.ModeAll)

	lanes := []struct {
		id          string
		want        string
		promotedTag string
		today       bool
		next        bool
		pinned      bool
	}{
		{id: reviewed.ID, want: "Review lane", promotedTag: reviewLaneKey},
		{id: contextReviewed.ID, want: "unpromoted backlog row"},
		{id: reviewedToday.ID, want: "Today precedence", today: true},
		{id: reviewedNext.ID, want: "Next precedence", next: true},
		{id: pinned.ID, want: "Pinned precedence", pinned: true},
	}
	for _, lane := range lanes {
		if countTaskRows(outliner, lane.id) != 1 {
			t.Fatalf("task %q was not deduplicated: %v", lane.id, visibleTaskIDs(outliner))
		}
		row, ok := findPlanningTaskRow(outliner, lane.id)
		if !ok || row.PromotedTag != lane.promotedTag || row.IsTodaySection != lane.today ||
			row.IsNextSection != lane.next || row.IsPinnedSection != lane.pinned {
			t.Fatalf("task %q row = %+v found=%t, want %s", lane.id, row, ok, lane.want)
		}
	}
	doingIndex, reviewIndex, reviewCount := -1, -1, 0
	for index, row := range outliner.rows {
		if row.IsSectionHeader && row.StateGroup == models.StateGroupDoing {
			doingIndex = index
		}
		if row.IsSectionHeader && row.PromotedTag == reviewLaneKey {
			reviewIndex = index
			reviewCount = row.SectionCount
		}
	}
	if doingIndex < 0 || reviewIndex <= doingIndex {
		t.Fatalf("Doing index=%d Review index=%d, want Review after Doing", doingIndex, reviewIndex)
	}
	if reviewCount != 1 {
		t.Fatalf("Review section count = %d, want the one unplanned flagged task", reviewCount)
	}
	if reviewed.State() != models.StateBacklog || !reviewed.Review {
		t.Fatalf("Review projection mutated durable task: state=%s review=%v", reviewed.State(), reviewed.Review)
	}
	if !reviewedToday.Review || !reviewedNext.Review {
		t.Fatalf("planning precedence cleared the review flag: today=%v next=%v", reviewedToday.Review, reviewedNext.Review)
	}
	today := runtime.Planning().GetToday()
	if len(today) != 1 || today[0].TaskID != reviewedToday.ID {
		t.Fatalf("Today membership = %+v, want only %q", today, reviewedToday.ID)
	}
	next := runtime.Planning().GetNext()
	if len(next) != 1 || next[0].TaskID != reviewedNext.ID {
		t.Fatalf("Next membership = %+v, want only %q", next, reviewedNext.ID)
	}
	if contextReviewed.Context != reviewLaneKey || contextReviewed.Review {
		t.Fatalf("Review projection mutated context task: context=%q review=%v", contextReviewed.Context, contextReviewed.Review)
	}
}

func TestPrimaryAllStaleLanePrecedenceAndOrder(t *testing.T) {
	old := time.Now().AddDate(0, 0, -(models.StaleTaskThresholdDays + 10))
	staleTask := func(id string, state models.WorkflowState) *models.Task {
		task := primaryRowsTask(id, state)
		task.UpdatedAt = old
		return task
	}

	pinned := staleTask("pinned-stale", models.StateBacklog)
	pinned.Pinned = true
	pinned.PinnedAt = &old
	today := staleTask("today-stale", models.StateBacklog)
	next := staleTask("next-stale", models.StateBacklog)
	review := staleTask("review-stale", models.StateBacklog)
	review.Review = true
	review.UpdatedAt = old
	doing := staleTask("doing-stale", models.StateDoing)
	stale := staleTask("plain-stale", models.StateBacklog)
	inbox := primaryRowsTask("fresh-inbox", models.StateInbox)
	project := primaryRowsProject("alpha", pinned, today, next, review, doing, stale, inbox)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: today.ID}})
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: next.ID}})
	outliner.selectOutlinerMode(chrome.ModeAll)

	wantLane := map[string]string{
		pinned.ID: "Pinned",
		today.ID:  "Today",
		next.ID:   "Next",
		review.ID: "Review",
		doing.ID:  "Doing",
		stale.ID:  "Stale",
	}
	for id, want := range wantLane {
		if got := countTaskRows(outliner, id); got != 1 {
			t.Fatalf("task %q row count = %d, want one: %v", id, got, visibleTaskIDs(outliner))
		}
		if got := primarySectionForTask(outliner, id); got != want {
			t.Errorf("task %q section = %q, want %q", id, got, want)
		}
	}

	sectionIndex := make(map[string]int)
	for index, row := range outliner.rows {
		if row.IsSectionHeader {
			sectionIndex[row.SectionName] = index
		}
	}
	if sectionIndex["Doing"] >= sectionIndex["Review"] ||
		sectionIndex["Review"] >= sectionIndex["Stale"] ||
		sectionIndex["Stale"] >= sectionIndex["Inbox"] {
		t.Fatalf("section indexes = %v, want Doing < Review < Stale < Inbox", sectionIndex)
	}
	if staleSectionCollapseKey == reviewSectionCollapseKey ||
		staleProjectSectionCollapsePrefix == reviewProjectSectionCollapsePrefix {
		t.Fatal("Stale and Review collapse namespaces overlap")
	}
}

func TestBuildStaleSectionUsesUpdatedCalendarDayAndVisibilityGuards(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 2, 0, 1, 0, 0, location)
	taskAt := func(id string, updated time.Time) *models.Task {
		task := primaryRowsTask(id, models.StateBacklog)
		task.UpdatedAt = updated
		return task
	}
	day90 := now.AddDate(0, 0, -models.StaleTaskThresholdDays)
	boundary := taskAt("boundary", time.Date(
		day90.Year(), day90.Month(), day90.Day(), 23, 59, 0, 0, location,
	))
	boundary.StateChangedAt = now
	day89 := now.AddDate(0, 0, -(models.StaleTaskThresholdDays - 1))
	fresh := taskAt("fresh", util.StartOfDay(day89))
	fresh.StateChangedAt = now.AddDate(0, 0, -200)
	terminal := taskAt("terminal", boundary.UpdatedAt)
	terminal.WorkflowState = models.StateDone
	deferred := taskAt("deferred", boundary.UpdatedAt)
	deferredUntil := now.Add(time.Hour)
	deferred.DeferredUntil = &deferredUntil
	excluded := taskAt("excluded", boundary.UpdatedAt)
	filtered := taskAt("filtered", boundary.UpdatedAt)
	project := primaryRowsProject("alpha", boundary, fresh, terminal, deferred, excluded, filtered)
	outliner, _ := primaryRowsOutliner(project)
	input := OutlinerModeInput{
		Projects: []*models.Project{project},
		Now:      now,
		TaskExclude: func(task *models.Task, _ *models.Project) bool {
			return task.ID == excluded.ID
		},
		TaskFilter: func(task *models.Task, _ *models.Project) bool {
			return task.ID != filtered.ID
		},
	}
	empty := map[taskref.TaskIdentity]bool{}
	section := outliner.buildStaleSection(input, empty, empty, empty, empty, empty)

	want := taskref.TaskIdentity{ProjectID: project.ID, TaskID: boundary.ID}
	if section.count != 1 || len(section.projected) != 1 || !section.projected[want] {
		t.Fatalf("Stale projection = count %d identities %v, want only %v", section.count, section.projected, want)
	}
}

func primarySectionForTask(outliner *OutlinerModel, taskID string) string {
	section := ""
	for _, row := range outliner.rows {
		if row.IsSectionHeader {
			section = row.SectionName
			continue
		}
		if row.isTaskRow() && row.task.ID == taskID {
			return section
		}
	}
	return ""
}

func TestPrimaryAllExplicitFiltersOverrideTerminalAndDeferredDefaults(t *testing.T) {
	done := primaryRowsTask("done", models.StateDone)
	canceled := primaryRowsTask("canceled", models.StateCanceled)
	deferred := primaryRowsTask("deferred", models.StateBacklog)
	future := time.Now().AddDate(0, 0, 2)
	deferred.DeferredUntil = &future
	project := primaryRowsProject("alpha", done, canceled, deferred)
	outliner, runtime := primaryRowsOutliner(project)
	// Stale planning references must not make explicit terminal/deferred facets
	// lose rows. The application invariant will normally clean these references,
	// but the projection remains defensive while a refresh is in flight.
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: done.ID}})
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: deferred.ID}})
	outliner.selectOutlinerMode(chrome.ModeAll)

	outliner.filter.SetStateFilter(testWorkflowStateSet(models.StateDone, models.StateCanceled))
	outliner.adoptFilterCarriers()
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
	outliner.filter.Deferred = true
	outliner.adoptFilterCarriers()
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"deferred"}) {
		t.Fatalf("deferred-filter rows = %v, want explicit deferred Today task", got)
	}
	if row, ok := findPlanningTaskRow(outliner, deferred.ID); !ok || !row.IsTodaySection {
		t.Fatalf("deferred Today row = %+v found=%t, want explicit planning projection", row, ok)
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

	outliner.selectOutlinerMode(chrome.ModeNext)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		alphaNext.ID,
		alphaNextChild.ID,
		betaNext.ID,
	}) {
		t.Fatalf("Next rows = %v, want pinned-first/alphabetical project grouping with hierarchy intact", got)
	}
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.ProjectID == "" {
			t.Fatalf("cross-project Next row %q lost qualified project identity", row.task.ID)
		}
	}
	parentRow, parentFound := findPlanningTaskRow(outliner, alphaNext.ID)
	childRow, childFound := findPlanningTaskRow(outliner, alphaNextChild.ID)
	if !parentFound || !childFound || !parentRow.hasVisibleChildren || childRow.Breadcrumb != "" ||
		len(childRow.prefix) <= len(parentRow.prefix) {
		t.Fatalf("nested Next rows = parent %+v found=%t child %+v found=%t", parentRow, parentFound, childRow, childFound)
	}

	outliner.expandedNodes[taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alphaNext.ID}] = false
	outliner.rebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaNext.ID, betaNext.ID}) {
		t.Fatalf("collapsed Next rows = %v, want nested child hidden", got)
	}
	if row, ok := findPlanningTaskRow(outliner, alphaNextChild.ID); ok {
		t.Fatalf("collapsed Next child remained visible: %+v", row)
	}

	outliner.selectOutlinerMode(chrome.ModeToday)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaToday.ID}) {
		t.Fatalf("Today rows = %v, want explicit durable membership only", got)
	}

	outliner.selectOutlinerMode(chrome.ModeInbox)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaInbox.ID, betaInbox.ID}) {
		t.Fatalf("Inbox rows = %v, want exact StateInbox membership", got)
	}

	outliner.selectOutlinerMode(chrome.ModeBacklog)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		alphaNext.ID,
		alphaReady.ID,
		betaNext.ID,
	}) {
		t.Fatalf("Backlog rows = %v, want Backlog lifecycle tasks excluding Today with folded descendants hidden", got)
	}

	outliner.scopedProjectID = beta.ID
	outliner.selectOutlinerMode(chrome.ModeInbox)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaInbox.ID}) {
		t.Fatalf("project-scoped Inbox rows = %v, want beta only", got)
	}
}
