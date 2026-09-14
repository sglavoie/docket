package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// UX-acceptance fixture identities. Scenarios assert on these exact strings, so
// the builder and the committed JSON agree through these constants. No title is
// a substring of another, so a `contains` check can only be answered by the row
// it names, and every title is long enough that a 60-column row has to decide
// what to keep.
const (
	uxAcceptancePlatformName = "UX Platform Delivery"
	uxAcceptanceClientName   = "UX Client Rollout"
	uxAcceptanceSupportName  = "UX Support Queue"

	// The attention row set. Each title carries exactly one of the four
	// precedence states — overdue, review, due today, P1 — so a styling
	// regression names the row it broke instead of a colour.
	uxAcceptanceOverdueTitle  = "Restore the overdue billing reconciliation export before the audit closes"
	uxAcceptanceReviewTitle   = "Reread the quarterly access list flagged for review"
	uxAcceptanceDueTodayTitle = "Confirm the release sign-off owed today"
	uxAcceptancePriorityTitle = "Escalate the priority incident bridge"
	// Waiting alone stays quiet, so this row is the control the other four are
	// read against: it carries a check-in schedule and no attention.
	uxAcceptanceWaitingTitle = "Await the vendor security questionnaire"

	// The hierarchy. Three generations, so an indent ladder, a relation marker,
	// and a subtree fold all have something to act on.
	uxAcceptanceParentTitle     = "Coordinate the platform migration window"
	uxAcceptanceChildTitle      = "Stage the migration rehearsal run"
	uxAcceptanceGrandchildTitle = "Capture the rehearsal findings"

	uxAcceptanceDeferredTitle = "Hold the deferred storage cleanup"
	uxAcceptancePinnedTitle   = "Publish the launch communications plan"
	uxAcceptanceDoneTitle     = "Close the retired staging cluster"
	uxAcceptanceInboxTitle    = "Sort the unlabeled support export"

	uxAcceptanceDependentTitle   = "Draft the client rollout timeline"
	uxAcceptanceClientLongTitle  = "Reconcile the client onboarding checklist against the revised milestones"
	uxAcceptanceClientInboxTitle = "Collect the client onboarding assets"

	uxAcceptanceSupportTriageTitle  = "Triage the duplicate support tickets"
	uxAcceptanceSupportWaitingTitle = "Answer the escalated support thread"

	// Metadata every row-to-Peek migration has to keep reachable somewhere.
	uxAcceptanceResumeNote  = "Stopped after the ledger export; the reconciliation diff is still open."
	uxAcceptanceDescription = "The export has to be replayed before the auditors open the window."
	uxAcceptanceComment     = "Finance confirmed the export window on Monday."
	uxAcceptanceAttachment  = "reconciliation-brief.md"
	uxAcceptanceContext     = "finance"
)

// buildUXAcceptanceFixture is the one fixture the UX refinement is reviewed
// against. Every other fixture isolates a single axis on purpose — `narrow`
// holds long titles, `signals` holds deferrals, `checklist` holds items — which
// is what keeps their committed scenarios stable, and is also why none of them
// can answer "does a task row still read at 80 columns when a long title, a
// deadline, a priority, a parent, tags, a context, and a selection are all true
// at once". This fixture puts those together deliberately:
//
//   - three projects, so cross-project rows have to name their owner and a
//     single-project scope has to stop naming it;
//   - a three-generation subtree, so indentation, relation markers, and subtree
//     folding act on something deeper than one hop;
//   - titles that do not fit 60 columns, so clipping is observable rather than
//     hypothetical;
//   - one row per attention state — overdue, review, due today, P1 — plus a
//     quiet Waiting control carrying a check-in schedule;
//   - the full metadata set on one row (description, resume note, tags,
//     context, checklist, comment, attachment, cross-project dependency), so
//     anything moved off a row can be checked for a Peek home;
//   - enough adjacent sibling rows in one group for a visual range selection.
//
// It deliberately does not extend an existing fixture: `rich` alone is read by
// enough committed scenarios that changing its row population would rewrite
// unrelated contracts.
func buildUXAcceptanceFixture(w *uitestWorkspace) error {
	var overdue *models.Task
	platform, err := w.addProject(uxAcceptancePlatformName, func(p *models.Project) {
		p.Description = "The delivery project the UX refinement is reviewed against."

		overdue = newFixtureTask(p, uxAcceptanceOverdueTitle)
		overdue.Priority = 1
		overdue.ApplyWorkflowState(models.StateBacklog)
		setDue(overdue, -2*24*time.Hour)
		overdue.Description = uxAcceptanceDescription
		overdue.ResumeNote = uxAcceptanceResumeNote
		overdue.Tags = []string{"billing", "audit"}
		overdue.Context = uxAcceptanceContext
		overdue.Checklist = []models.ChecklistItem{
			{Text: "Pull the ledger export", Done: true},
			{Text: "Diff against the audit sheet"},
		}
		overdue.Comments = []*models.Comment{{Text: uxAcceptanceComment, CreatedAt: time.Now()}}
		overdue.Attachments = []models.Attachment{{
			Filename: uxAcceptanceAttachment,
			AddedAt:  time.Now(),
		}}

		parent := newFixtureTask(p, uxAcceptanceParentTitle)
		parent.Priority = 2
		parent.ApplyWorkflowState(models.StateBacklog)

		child := newFixtureTask(p, uxAcceptanceChildTitle)
		child.ParentID = &parent.ID
		child.ApplyWorkflowState(models.StateDoing)

		grandchild := newFixtureTask(p, uxAcceptanceGrandchildTitle)
		grandchild.ParentID = &child.ID
		grandchild.ApplyWorkflowState(models.StateBacklog)

		review := newFixtureTask(p, uxAcceptanceReviewTitle)
		review.ApplyWorkflowState(models.StateBacklog)
		review.Review = true

		dueToday := newFixtureTask(p, uxAcceptanceDueTodayTitle)
		dueToday.ApplyWorkflowState(models.StateBacklog)
		today := util.StartOfDay(time.Now())
		dueToday.DueDate = &today

		priority := newFixtureTask(p, uxAcceptancePriorityTitle)
		priority.Priority = 1
		priority.ApplyWorkflowState(models.StateDoing)

		// Waiting alone is quiet, so this row proves the absence of attention
		// rather than its presence. Its check-in is two days out, which is a
		// schedule that has not arrived: the row stays quiet and Peek still
		// names the date.
		waiting := newFixtureTask(p, uxAcceptanceWaitingTitle)
		waiting.ApplyWorkflowState(models.StateWaiting)
		waiting.StateChangedAt = time.Now().Add(-9*24*time.Hour - 2*time.Hour)
		checkIn := util.StartOfDay(time.Now()).AddDate(0, 0, 2)
		waiting.NextCheckIn = &checkIn
		waiting.CheckInRepeatDays = 14

		deferred := newFixtureTask(p, uxAcceptanceDeferredTitle)
		deferred.ApplyWorkflowState(models.StateBacklog)
		wake := util.StartOfDay(time.Now()).AddDate(0, 0, 3)
		deferred.DeferredUntil = &wake

		pinned := newFixtureTask(p, uxAcceptancePinnedTitle)
		pinned.ApplyWorkflowState(models.StateBacklog)
		pinned.Pinned = true
		pinnedAt := time.Now()
		pinned.PinnedAt = &pinnedAt

		markDone(newFixtureTask(p, uxAcceptanceDoneTitle))

		newFixtureTask(p, uxAcceptanceInboxTitle).ApplyWorkflowState(models.StateInbox)
	})
	if err != nil {
		return err
	}

	if _, err = w.addProject(uxAcceptanceClientName, func(p *models.Project) {
		p.Description = "The second project, so a cross-project row has an owner to name."

		dependent := newFixtureTask(p, uxAcceptanceDependentTitle)
		dependent.Priority = 2
		dependent.ApplyWorkflowState(models.StateBacklog)
		// The one reference crossing a project boundary, so Peek has both a
		// Dependencies line here and a Dependents line on the overdue row.
		dependent.Dependencies = []models.TaskReference{{
			ProjectID: platform.ID,
			TaskID:    overdue.ID,
		}}

		long := newFixtureTask(p, uxAcceptanceClientLongTitle)
		long.ApplyWorkflowState(models.StateBacklog)
		long.Tags = []string{"onboarding"}

		newFixtureTask(p, uxAcceptanceClientInboxTitle).ApplyWorkflowState(models.StateInbox)
	}); err != nil {
		return err
	}

	if _, err = w.addProject(uxAcceptanceSupportName, func(p *models.Project) {
		p.Description = "The third project, so a project label is never a two-way choice."

		newFixtureTask(p, uxAcceptanceSupportTriageTitle).ApplyWorkflowState(models.StateInbox)

		waiting := newFixtureTask(p, uxAcceptanceSupportWaitingTitle)
		waiting.ApplyWorkflowState(models.StateWaiting)
	}); err != nil {
		return err
	}

	w.setCurrentProject(platform.ID)
	return nil
}
