package tui

import (
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// --- the narrowing templates' own semantics ----------------------------------

func lensSemanticsTask(id string, state models.WorkflowState) *models.Task {
	task := primaryRowsTask(id, state)
	task.CreatedAt = time.Now()
	return task
}

// daysFromNow is a calendar date, midnight local: a due date carrying a
// time-of-day is compared at its exact instant instead of at day granularity,
// and "due today" would then read as already overdue by however long today has
// been running.
func daysFromNow(days int) *time.Time {
	when := util.StartOfDay(time.Now().AddDate(0, 0, days))
	return &when
}

// lensSemanticsOutliner holds the one population the narrowing templates are read
// against: dated work either side of today, tasks carrying the stored review flag
// beside tasks that merely spell the word on the tag and context axes, and
// terminal work either side of the ninety-day completion threshold.
func lensSemanticsOutliner() *OutlinerModel {
	overdueNear := lensSemanticsTask("overdue-near", models.StateBacklog)
	overdueNear.DueDate = daysFromNow(-1)
	overdueFar := lensSemanticsTask("overdue-far", models.StateBacklog)
	overdueFar.DueDate = daysFromNow(-9)
	dueToday := lensSemanticsTask("due-today", models.StateBacklog)
	dueToday.DueDate = daysFromNow(0)
	dueLater := lensSemanticsTask("due-later", models.StateBacklog)
	dueLater.DueDate = daysFromNow(3)
	overdueDone := lensSemanticsTask("overdue-done", models.StateDone)
	overdueDone.DueDate = daysFromNow(-4)
	overdueDone.CompletedAt = daysFromNow(-1)

	reviewFlagged := lensSemanticsTask("review-flagged", models.StateBacklog)
	reviewFlagged.Review = true
	// Neither of the next two is in the queue: the flag is its own axis, so a task
	// that only spells the word as a tag or a context is not flagged.
	reviewTagged := lensSemanticsTask("review-tagged", models.StateBacklog)
	reviewTagged.Tags = []string{"review"}
	reviewContext := lensSemanticsTask("review-context", models.StateBacklog)
	reviewContext.Context = "review"
	reviewUrgent := lensSemanticsTask("review-urgent", models.StateBacklog)
	reviewUrgent.Review = true
	reviewUrgent.Tags = []string{"urgent"}
	urgentOnly := lensSemanticsTask("urgent-only", models.StateBacklog)
	urgentOnly.Tags = []string{"urgent"}

	doneOld := lensSemanticsTask("done-old", models.StateDone)
	doneOld.CompletedAt = daysFromNow(-120)
	canceledOld := lensSemanticsTask("canceled-old", models.StateCanceled)
	canceledOld.CompletedAt = daysFromNow(-200)
	doneRecent := lensSemanticsTask("done-recent", models.StateDone)
	doneRecent.CompletedAt = daysFromNow(-10)
	// A nonterminal task wearing an old completion stamp is deliberately
	// impossible-looking: it is the one shape that isolates Cleanup 90d's state
	// predicate from its age predicate, because only the state predicate can
	// reject it.
	waitingOld := lensSemanticsTask("waiting-old", models.StateWaiting)
	waitingOld.CompletedAt = daysFromNow(-200)

	return scopeTestOutliner(primaryRowsProject(
		"alpha",
		overdueNear, overdueFar, dueToday, dueLater, overdueDone,
		reviewFlagged, reviewTagged, reviewContext, reviewUrgent, urgentOnly,
		doneOld, canceledOld, doneRecent, waitingOld,
	))
}

func sortedVisibleTaskIDs(outliner *OutlinerModel) []string {
	ids := slices.Clone(searchVisibleTaskIDs(outliner))
	sort.Strings(ids)
	return ids
}

// TestOverdueTemplateUsesDueModeFixedOrdering pins Overdue to the ordering its
// mode owns. The template narrows with the overdue bucket and nothing else; the
// order the rows arrive in is Due mode's calendar order, not the manual sort the
// definition still carries for whenever a tree-order mode reads it.
func TestOverdueTemplateUsesDueModeFixedOrdering(t *testing.T) {
	outliner := lensSemanticsOutliner()
	template := lensTemplateByName(t, "Overdue")
	applyLensOrFatal(t, outliner, template)

	mode, ok := OutlinerModeByID(outliner.primaryMode)
	if !ok || outliner.primaryMode != ModeDue {
		t.Fatalf("mode = %v, want Due", outliner.primaryMode)
	}
	if !mode.FixedOrder() {
		t.Fatal("Due must own a fixed order for the template to inherit one")
	}
	if outliner.sort != NewSortState() {
		t.Fatalf("active sort = %+v, want a fixed-order mode's inactive default", outliner.sort)
	}

	// Manual order in the project puts overdue-near first; the calendar order the
	// mode imposes puts the earlier due date first instead. Terminal work is not
	// overdue however far past its date it sits.
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"overdue-far", "overdue-near"}) {
		t.Fatalf("rows = %v, want the overdue bucket in due-date order", got)
	}
}

// TestReviewQueueTemplateIntersectsItsOtherFacets pins Review Queue to the
// stored review flag and to the intersection semantics every facet has: adding
// a tag narrows rather than broadens. The flag is its own axis, so a task whose
// tag or context merely spells review is not in the queue.
func TestReviewQueueTemplateIntersectsItsOtherFacets(t *testing.T) {
	outliner := lensSemanticsOutliner()
	applyLensOrFatal(t, outliner, lensTemplateByName(t, "Review Queue"))

	want := []string{"review-flagged", "review-urgent"}
	if got := sortedVisibleTaskIDs(outliner); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want every task carrying the review flag %v", got, want)
	}

	// A tag intersects with the flag rather than joining it.
	outliner.filter.Tags["urgent"] = true
	outliner.adoptFilterCarriers()
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if got := sortedVisibleTaskIDs(outliner); !slices.Equal(got, []string{"review-urgent"}) {
		t.Fatalf("rows under flag and tag = %v, want only the task carrying both", got)
	}
}

// TestCleanup90dTemplateCombinesItsTwoPredicates pins Cleanup 90d to the
// conjunction its two facets make. Terminal state and completion age are separate
// predicates that both have to hold; neither is widened to carry the other.
func TestCleanup90dTemplateCombinesItsTwoPredicates(t *testing.T) {
	outliner := lensSemanticsOutliner()
	applyLensOrFatal(t, outliner, lensTemplateByName(t, "Cleanup 90d"))

	want := []string{"canceled-old", "done-old"}
	if got := sortedVisibleTaskIDs(outliner); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want the terminal work older than the threshold %v", got, want)
	}
	// done-recent is terminal but young, and waiting-old is old but not terminal.
	// Their absence above is the whole assertion; naming them here keeps the
	// failure message honest about which predicate did the rejecting.
	for _, rejected := range []string{"done-recent", "waiting-old", "overdue-done"} {
		if slices.Contains(searchVisibleTaskIDs(outliner), rejected) {
			t.Errorf("Cleanup 90d admitted %q, so one of its two predicates was broadened", rejected)
		}
	}
}
