package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func stripFixtureTask(t *testing.T, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask("task-id-"+title, 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", title, err)
	}
	return task
}

func stripFixtureProject(t *testing.T, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject("proj-id-"+name, name, "")
	if err != nil {
		t.Fatalf("NewProject(%q): %v", name, err)
	}
	return project
}

func renderStripPlain(t *testing.T, opts NextActionStripOpts) string {
	t.Helper()
	return ansi.Strip(renderNextActionStrip(opts))
}

// TestNextActionStripWorkOnTaskShowsTitleProjectAndReasons covers the most
// common cockpit state: an actionable task surfaced as WorkOnTask with reason
// badges. The strip must include the kind prefix ("Next:"), the task title,
// a project badge, and the human-readable reason copy in that order.
func TestNextActionStripWorkOnTaskShowsTitleProjectAndReasons(t *testing.T) {
	task := stripFixtureTask(t, "Ship overdue invoice")
	project := stripFixtureProject(t, "Billing")

	plain := renderStripPlain(t, NextActionStripOpts{
		Action: RecommendedAction{
			Kind:    RecommendationKindWorkOnTask,
			Task:    task,
			Project: project,
			Reasons: []RecommendationReason{ReasonOverdue, ReasonHighPriority},
		},
		Width:       120,
		HasProjects: true,
		OpenLoops:   OpenLoops{HasTasks: true, Overdue: 1},
	})

	wants := []string{"Next:", "Ship overdue invoice", "[Billing]", "overdue", "high priority"}
	for _, want := range wants {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected substring %q in strip, got %q", want, plain)
		}
	}

	// Ordering invariant: reasons must appear in the engine's emission order
	// (overdue before high priority). The renderer must not re-sort.
	overduePos := strings.Index(plain, "overdue")
	priorityPos := strings.Index(plain, "high priority")
	if overduePos == -1 || priorityPos == -1 || overduePos > priorityPos {
		t.Fatalf("reasons out of order in %q (overdue=%d, high priority=%d)", plain, overduePos, priorityPos)
	}
}

// TestNextActionStripReviewBlockedSurfacesUnblockHint covers the kind that
// rotates blocked tasks. The strip must announce "Review blocked" and offer
// the unblock affordance in the hint cluster.
func TestNextActionStripReviewBlockedSurfacesUnblockHint(t *testing.T) {
	task := stripFixtureTask(t, "Wire SSO callback")
	project := stripFixtureProject(t, "Auth")

	plain := renderStripPlain(t, NextActionStripOpts{
		Action: RecommendedAction{
			Kind:    RecommendationKindReviewBlocked,
			Task:    task,
			Project: project,
			Reasons: []RecommendationReason{ReasonBlocked},
		},
		Width:       120,
		HasProjects: true,
		OpenLoops:   OpenLoops{HasTasks: true},
	})

	for _, want := range []string{"Review blocked", "Wire SSO callback", "[Auth]", "blocked", "u unblock"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected substring %q in strip, got %q", want, plain)
		}
	}
}

// TestNextActionStripTriageInboxCountsItems covers the inbox-triage kind: the
// strip must render the inbox count using OpenLoops.Inbox and offer the
// triage shortcut.
func TestNextActionStripTriageInboxCountsItems(t *testing.T) {
	plain := renderStripPlain(t, NextActionStripOpts{
		Action: RecommendedAction{
			Kind:    RecommendationKindTriageInbox,
			Reasons: []RecommendationReason{ReasonInboxNeedsTriage},
		},
		Width:       120,
		HasProjects: true,
		OpenLoops:   OpenLoops{HasTasks: true, Inbox: 4},
	})

	for _, want := range []string{"Triage inbox", "(4 items)", "inbox needs triage", ";i triage"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected substring %q in strip, got %q", want, plain)
		}
	}
}

// TestNextActionStripTriageInboxFallsBackToOpenLoops verifies that when the
// engine returns AllDone (because no inbox tasks made it into Projects) but
// OpenLoops.Inbox is non-zero, the renderer overlays the triage copy. This
// is the documented cockpit fallback for partial engine inputs.
func TestNextActionStripTriageInboxFallsBackToOpenLoops(t *testing.T) {
	plain := renderStripPlain(t, NextActionStripOpts{
		Action:      RecommendedAction{Kind: RecommendationKindAllDone},
		Width:       120,
		HasProjects: false,
		OpenLoops:   OpenLoops{HasTasks: true, Inbox: 1},
	})

	for _, want := range []string{"Triage inbox", "(1 item)", ";i triage"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected substring %q in strip, got %q", want, plain)
		}
	}
	if strings.Contains(plain, "items)") {
		t.Fatalf("singular item suffix expected, got plural in %q", plain)
	}
}

// TestNextActionStripPlanTodaySuggestsPlanning covers the kind that nudges
// the user to put something on today's plan. No reason badges, no project
// badge — just the directive plus key hints.
func TestNextActionStripPlanTodaySuggestsPlanning(t *testing.T) {
	plain := renderStripPlain(t, NextActionStripOpts{
		Action:      RecommendedAction{Kind: RecommendationKindPlanToday},
		Width:       120,
		HasProjects: true,
		OpenLoops:   OpenLoops{HasTasks: true},
	})

	for _, want := range []string{"Plan today's work", "w plan"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected substring %q in strip, got %q", want, plain)
		}
	}
	// Session 11: strip emits one recommendation-unique hint only — keys the
	// host footer already advertises (`a add`, `enter open`, `x done`) must
	// not be duplicated here.
	for _, banned := range []string{"a add", "enter open", "x done"} {
		if strings.Contains(plain, banned) {
			t.Fatalf("strip should not duplicate footer-owned hint %q, got %q", banned, plain)
		}
	}
	if strings.Contains(plain, "·") && strings.Contains(plain, "overdue") {
		t.Fatalf("plan today must not surface reason badges, got %q", plain)
	}
}

// TestNextActionStripAllDoneFirstRunCopy covers the first-run state: no
// projects exist and no open loops are tracked. The renderer must surface
// project-creation copy rather than the steady-state "all caught up".
func TestNextActionStripAllDoneFirstRunCopy(t *testing.T) {
	plain := renderStripPlain(t, NextActionStripOpts{
		Action:      RecommendedAction{Kind: RecommendationKindAllDone},
		Width:       120,
		HasProjects: false,
		OpenLoops:   OpenLoops{},
	})

	for _, want := range []string{"Create your first project", "p new project"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected substring %q in strip, got %q", want, plain)
		}
	}
	if strings.Contains(plain, "All caught up") || strings.Contains(plain, "All done") {
		t.Fatalf("first-run strip should not surface caught-up copy, got %q", plain)
	}
	// Session 11: strip emits only the recommendation-unique key. `a quick
	// capture` belongs to the host footer's contextual cluster and must not
	// repeat inside the strip.
	if strings.Contains(plain, "a quick capture") {
		t.Fatalf("strip should not duplicate footer-owned hint \"a quick capture\", got %q", plain)
	}
}

// TestNextActionStripAllDoneCaughtUpCopy covers the steady-state celebration:
// projects exist, today is empty, no open loops. The renderer surfaces the
// "all caught up" copy plus the "plan more" affordance.
func TestNextActionStripAllDoneCaughtUpCopy(t *testing.T) {
	plain := renderStripPlain(t, NextActionStripOpts{
		Action:      RecommendedAction{Kind: RecommendationKindAllDone},
		Width:       120,
		HasProjects: true,
		OpenLoops:   OpenLoops{HasTasks: true, TodayTotal: 2, TodayDone: 2},
	})

	if !strings.Contains(plain, "All caught up for today") {
		t.Fatalf("expected celebration title in strip, got %q", plain)
	}
	// Session 11: caught-up strip emits no hint cluster because the
	// completed-Today footer already advertises `w plan more` and `a add` —
	// the strip is a banner, the footer is the canonical affordance.
	for _, banned := range []string{"w plan more", "a add", "enter open"} {
		if strings.Contains(plain, banned) {
			t.Fatalf("caught-up strip should not duplicate footer-owned hint %q, got %q", banned, plain)
		}
	}
}

// TestNextActionStripDegradesUnderNarrowWidth verifies the documented
// degrade-on-narrow priority: hints drop first, then the row truncates with
// an ellipsis once the title alone exceeds the budget.
func TestNextActionStripDegradesUnderNarrowWidth(t *testing.T) {
	task := stripFixtureTask(t, "Ship the truly long task title that exceeds the budget significantly")
	project := stripFixtureProject(t, "Billing")

	wide := renderStripPlain(t, NextActionStripOpts{
		Action: RecommendedAction{
			Kind:    RecommendationKindWorkOnTask,
			Task:    task,
			Project: project,
			Reasons: []RecommendationReason{ReasonOverdue},
		},
		Width:       140,
		HasProjects: true,
	})
	// Session 11: strip carries the recommendation-unique key only. For
	// WorkOnTask that is `w plan today` (so the hint cannot collide with the
	// host footer's `enter open` / `x done` / `a add` cluster).
	if !strings.Contains(wide, "w plan today") {
		t.Fatalf("hints should be visible at width 140, got %q", wide)
	}

	narrow := renderStripPlain(t, NextActionStripOpts{
		Action: RecommendedAction{
			Kind:    RecommendationKindWorkOnTask,
			Task:    task,
			Project: project,
			Reasons: []RecommendationReason{ReasonOverdue},
		},
		Width:       40,
		HasProjects: true,
	})
	if strings.Contains(narrow, "w plan today") {
		t.Fatalf("hints should be dropped at width 40, got %q", narrow)
	}
	if lipgloss.Width(narrow) > 40 {
		t.Fatalf("narrow strip exceeds width 40: %d cols in %q", lipgloss.Width(narrow), narrow)
	}
	if !strings.Contains(narrow, "…") {
		t.Fatalf("narrow strip should truncate the title with ellipsis, got %q", narrow)
	}
}

// TestNextActionStripSuppressedBelowMinWidth verifies the strip returns empty
// below NextActionStripMinWidth so the caller can reclaim the row instead of
// emitting a garbled fragment.
func TestNextActionStripSuppressedBelowMinWidth(t *testing.T) {
	got := renderNextActionStrip(NextActionStripOpts{
		Action: RecommendedAction{Kind: RecommendationKindPlanToday},
		Width:  NextActionStripMinWidth - 1,
	})
	if got != "" {
		t.Fatalf("expected empty strip below MinWidth, got %q", got)
	}
}

// TestNextActionStripNeverExceedsWidth pins the visual-width invariant across
// every kind × variant combination at all baseline widths plus narrow. A
// regression that lets reasons or hints overflow would corrupt the cockpit
// header layout, so the check is exhaustive.
func TestNextActionStripNeverExceedsWidth(t *testing.T) {
	task := stripFixtureTask(t, "Sample task with a moderately long title")
	project := stripFixtureProject(t, "Sample Project")
	cases := []struct {
		name string
		opts NextActionStripOpts
	}{
		{"work-on-task", NextActionStripOpts{
			Action: RecommendedAction{
				Kind: RecommendationKindWorkOnTask, Task: task, Project: project,
				Reasons: []RecommendationReason{ReasonOverdue, ReasonHighPriority, ReasonCarriedOver},
			},
			HasProjects: true,
			OpenLoops:   OpenLoops{HasTasks: true, Overdue: 1},
		}},
		{"review-blocked", NextActionStripOpts{
			Action: RecommendedAction{
				Kind: RecommendationKindReviewBlocked, Task: task, Project: project,
				Reasons: []RecommendationReason{ReasonBlocked, ReasonPlannedToday},
			},
			HasProjects: true,
			OpenLoops:   OpenLoops{HasTasks: true},
		}},
		{"triage-inbox", NextActionStripOpts{
			Action: RecommendedAction{
				Kind:    RecommendationKindTriageInbox,
				Reasons: []RecommendationReason{ReasonInboxNeedsTriage},
			},
			HasProjects: true,
			OpenLoops:   OpenLoops{HasTasks: true, Inbox: 42},
		}},
		{"plan-today", NextActionStripOpts{
			Action:      RecommendedAction{Kind: RecommendationKindPlanToday},
			HasProjects: true,
			OpenLoops:   OpenLoops{HasTasks: true},
		}},
		{"all-done-first-run", NextActionStripOpts{
			Action:      RecommendedAction{Kind: RecommendationKindAllDone},
			HasProjects: false,
			OpenLoops:   OpenLoops{},
		}},
		{"all-done-caught-up", NextActionStripOpts{
			Action:      RecommendedAction{Kind: RecommendationKindAllDone},
			HasProjects: true,
			OpenLoops:   OpenLoops{HasTasks: true, TodayTotal: 3, TodayDone: 3},
		}},
	}

	widths := []int{60, 80, 100, 140}
	for _, tc := range cases {
		for _, w := range widths {
			opts := tc.opts
			opts.Width = w
			rendered := renderNextActionStrip(opts)
			plain := ansi.Strip(rendered)
			if got := lipgloss.Width(plain); got > w {
				t.Fatalf("[%s @ width %d] strip width %d exceeds budget: %q", tc.name, w, got, plain)
			}
			if strings.Contains(plain, "\n") {
				t.Fatalf("[%s @ width %d] strip must be one row, found newline in %q", tc.name, w, plain)
			}
		}
	}
}

// TestNextActionStripNilTaskFallbackCopy guards the WorkOnTask / ReviewBlocked
// nil-task path: callers must never see "Next: " with an empty title.
func TestNextActionStripNilTaskFallbackCopy(t *testing.T) {
	plain := renderStripPlain(t, NextActionStripOpts{
		Action: RecommendedAction{
			Kind:    RecommendationKindWorkOnTask,
			Task:    nil,
			Reasons: []RecommendationReason{ReasonOverdue},
		},
		Width:       120,
		HasProjects: true,
		OpenLoops:   OpenLoops{HasTasks: true},
	})
	if !strings.Contains(plain, "Next:") {
		t.Fatalf("expected Next: prefix even with nil task, got %q", plain)
	}
	if strings.Contains(plain, "Next: \n") || strings.HasSuffix(strings.TrimRight(plain, " "), "Next:") {
		t.Fatalf("nil-task fallback should provide a non-empty title, got %q", plain)
	}
}

// TestNextActionStripPreservesReasonOrder uses the documented "duplicates are
// preserved for emphasis" contract from session 2 to verify the renderer
// does not dedupe a repeated reason.
func TestNextActionStripPreservesReasonOrder(t *testing.T) {
	task := stripFixtureTask(t, "Ship release notes")
	project := stripFixtureProject(t, "Docs")
	plain := renderStripPlain(t, NextActionStripOpts{
		Action: RecommendedAction{
			Kind:    RecommendationKindWorkOnTask,
			Task:    task,
			Project: project,
			Reasons: []RecommendationReason{ReasonOverdue, ReasonOverdue, ReasonHighPriority},
		},
		Width:       140,
		HasProjects: true,
	})

	if got := strings.Count(plain, "overdue"); got != 2 {
		t.Fatalf("expected two overdue badges (engine emphasis), got %d in %q", got, plain)
	}
}
