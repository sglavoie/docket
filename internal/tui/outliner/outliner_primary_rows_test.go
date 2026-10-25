package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/util"
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

	outliner.SelectOutlinerMode(chrome.ModeAll)

	var groups []models.WorkflowStateGroup
	for _, row := range outliner.Rows {
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
	for _, row := range outliner.Rows {
		if row.IsSectionHeader && row.StateGroup.IsValid() && row.SectionName != row.StateGroup.Label() {
			t.Errorf("group %q label = %q, want canonical %q", row.StateGroup, row.SectionName, row.StateGroup.Label())
		}
	}
}

// TestPrimaryAllPromotesAttentionWithManualAndScheduledPrecedence pins the
// shared attention predicate and the distinction between its two branches.
// Manual flags keep their established claim over Doing; scheduled-only
// reminders stay in Pinned, Today, Next, or Doing and render their indicator in
// place rather than duplicating the task under Review. Deferred work remains
// outside the default population until an explicit query opens it.
func TestPrimaryAllPromotesAttentionWithManualAndScheduledPrecedence(t *testing.T) {
	now := time.Now()
	dueCheckIn := util.StartOfDay(now.AddDate(0, 0, -1))
	futureCheckIn := util.StartOfDay(now.AddDate(0, 0, 2))
	reviewed := primaryRowsTask("reviewed", models.StateBacklog)
	reviewed.Review = true
	manualDoing := primaryRowsTask("manual-doing", models.StateDoing)
	manualDoing.Review = true
	scheduled := primaryRowsTask("scheduled", models.StateBacklog)
	scheduled.NextCheckIn = &dueCheckIn
	scheduledDoing := primaryRowsTask("scheduled-doing", models.StateDoing)
	scheduledDoing.NextCheckIn = &dueCheckIn
	future := primaryRowsTask("future", models.StateBacklog)
	future.NextCheckIn = &futureCheckIn
	deferredFlagged := primaryRowsTask("deferred-flagged", models.StateBacklog)
	deferredFlagged.Review = true
	deferredUntil := now.AddDate(0, 0, 3)
	deferredFlagged.DeferredUntil = &deferredUntil
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
	pinned.PinnedAt = &now
	project := primaryRowsProject(
		"alpha", reviewed, manualDoing, scheduled, scheduledDoing, future, deferredFlagged,
		reviewedToday, reviewedNext, contextReviewed, doing, pinned,
	)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: reviewedToday.ID}})
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: reviewedNext.ID}})
	outliner.SelectOutlinerMode(chrome.ModeAll)

	lanes := []struct {
		id          string
		want        string
		promotedTag string
		today       bool
		next        bool
		pinned      bool
	}{
		{id: reviewed.ID, want: "Review lane", promotedTag: reviewLaneKey},
		{id: manualDoing.ID, want: "manual Review precedence", promotedTag: reviewLaneKey},
		{id: scheduled.ID, want: "scheduled Review lane", promotedTag: reviewLaneKey},
		{id: scheduledDoing.ID, want: "Doing precedence"},
		{id: future.ID, want: "future schedule stays in Backlog"},
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
	if countTaskRows(outliner, deferredFlagged.ID) != 0 {
		t.Fatalf("deferred flagged task escaped the default population: %v", visibleTaskIDs(outliner))
	}
	doingIndex, reviewIndex, reviewCount := -1, -1, 0
	for index, row := range outliner.Rows {
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
	if reviewCount != 3 {
		t.Fatalf("Review section count = %d, want manual backlog/Doing plus scheduled backlog", reviewCount)
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

func TestPrimaryAllLeavesStaleWorkInItsLifecycleLane(t *testing.T) {
	old := time.Now().AddDate(0, 0, -(models.StaleTaskThresholdDays + 10))
	stale := primaryRowsTask("plain-stale", models.StateBacklog)
	stale.UpdatedAt = old
	inbox := primaryRowsTask("fresh-inbox", models.StateInbox)
	project := primaryRowsProject("alpha", stale, inbox)
	outliner, _ := primaryRowsOutliner(project)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	if got := countTaskRows(outliner, stale.ID); got != 1 {
		t.Fatalf("stale task row count = %d, want one: %v", got, visibleTaskIDs(outliner))
	}
	if got := primarySectionForTask(outliner, stale.ID); got != models.StateGroupBacklog.Label() {
		t.Fatalf("stale task section = %q, want ordinary Backlog", got)
	}
	for _, row := range outliner.Rows {
		if row.IsSectionHeader && row.SectionName == "Stale" {
			t.Fatal("All mode still constructed a Stale lane")
		}
	}
}

func primarySectionForTask(outliner *OutlinerModel, taskID string) string {
	section := ""
	for _, row := range outliner.Rows {
		if row.IsSectionHeader {
			section = row.SectionName
			continue
		}
		if row.IsTaskRow() && row.Task.ID == taskID {
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
	outliner.SelectOutlinerMode(chrome.ModeAll)

	outliner.Filter.SetStateFilter(testWorkflowStateSet(models.StateDone, models.StateCanceled))
	adoptFilterCarriers(outliner)
	outliner.RebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{"done", "canceled"}) {
		t.Fatalf("terminal-filter rows = %v, want done/canceled", got)
	}
	var terminalGroups []models.WorkflowStateGroup
	for _, row := range outliner.Rows {
		if row.IsSectionHeader {
			terminalGroups = append(terminalGroups, row.StateGroup)
		}
	}
	if !slices.Equal(terminalGroups, []models.WorkflowStateGroup{models.StateGroupDone, models.StateGroupCanceled}) {
		t.Fatalf("terminal groups = %v, want Done then Canceled", terminalGroups)
	}

	clearCommonFilters(&outliner.Filter.CommonFilterState)
	outliner.Filter.Deferred = true
	adoptFilterCarriers(outliner)
	outliner.RebuildRows()
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

	outliner.SelectOutlinerMode(chrome.ModeNext)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		alphaNext.ID,
		alphaNextChild.ID,
		betaNext.ID,
	}) {
		t.Fatalf("Next rows = %v, want pinned-first/alphabetical project grouping with hierarchy intact", got)
	}
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.ProjectID == "" {
			t.Fatalf("cross-project Next row %q lost qualified project identity", row.Task.ID)
		}
	}
	parentRow, parentFound := findPlanningTaskRow(outliner, alphaNext.ID)
	childRow, childFound := findPlanningTaskRow(outliner, alphaNextChild.ID)
	if !parentFound || !childFound || !parentRow.HasVisibleChildren || childRow.Breadcrumb != "" ||
		len(childRow.Prefix) <= len(parentRow.Prefix) {
		t.Fatalf("nested Next rows = parent %+v found=%t child %+v found=%t", parentRow, parentFound, childRow, childFound)
	}

	outliner.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: alphaNext.ID}] = false
	outliner.RebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaNext.ID, betaNext.ID}) {
		t.Fatalf("collapsed Next rows = %v, want nested child hidden", got)
	}
	if row, ok := findPlanningTaskRow(outliner, alphaNextChild.ID); ok {
		t.Fatalf("collapsed Next child remained visible: %+v", row)
	}

	outliner.SelectOutlinerMode(chrome.ModeToday)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaToday.ID}) {
		t.Fatalf("Today rows = %v, want explicit durable membership only", got)
	}

	outliner.SelectOutlinerMode(chrome.ModeInbox)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaInbox.ID, betaInbox.ID}) {
		t.Fatalf("Inbox rows = %v, want exact StateInbox membership", got)
	}

	outliner.SelectOutlinerMode(chrome.ModeBacklog)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
		alphaNext.ID,
		alphaReady.ID,
		betaNext.ID,
	}) {
		t.Fatalf("Backlog rows = %v, want Backlog lifecycle tasks excluding Today with folded descendants hidden", got)
	}

	outliner.Scope.ProjectID = beta.ID
	outliner.SelectOutlinerMode(chrome.ModeInbox)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaInbox.ID}) {
		t.Fatalf("project-scoped Inbox rows = %v, want beta only", got)
	}
}
