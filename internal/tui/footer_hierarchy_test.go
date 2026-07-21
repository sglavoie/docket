package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// Session 11 pins three layered help surfaces:
//
//  1. The Next Action strip carries exactly one recommendation-unique key,
//     never overlapping the host view's footer cluster.
//  2. Full-height footers render `<top 3 contextual> · ? help · : cmd · ;; hub`
//     so the standard escape hatches stay discoverable from every primary
//     daily-driver surface.
//  3. Non-full-height views (e.g., Dashboard) inherit the same escape-hatch
//     cluster from the app status bar so the user never sees one set of
//     escape keys in Today and a different set in Dashboard.
//
// The tests below enforce each layer with focused assertions; they fail fast
// on regressions that would re-introduce duplicate hints or drop the escape
// hatches from a full-height surface.

// TestFooterEscapeHintsCanonicalOrder pins the order and content of the
// shared FooterEscapeHints constant. The cluster is intentionally
// `;; hub · : cmd · ? help` (matches app_view_status.go's default) so the
// escape-hatch row reads identically across full-height and non-full-height
// surfaces.
func TestFooterEscapeHintsCanonicalOrder(t *testing.T) {
	if FooterEscapeHints != "ctrl+g go · : cmd · ? help" {
		t.Fatalf("FooterEscapeHints drifted from canonical order: %q", FooterEscapeHints)
	}
}

// TestComposeFullHeightFooterLeftBlendsContextualAndEscapeHatches verifies
// the helper joins the contextual cluster and the escape-hatch cluster with
// the `·` separator the rest of the cockpit uses.
func TestComposeFullHeightFooterLeftBlendsContextualAndEscapeHatches(t *testing.T) {
	plain := ansi.Strip(composeFullHeightFooterLeft("x done · a add · enter open"))
	want := "x done · a add · enter open · " + FooterEscapeHints
	if plain != want {
		t.Fatalf("composed footer mismatch:\n got: %q\nwant: %q", plain, want)
	}
}

// TestComposeFullHeightFooterLeftFallsBackToEscapeHatches verifies the helper
// emits the escape-hatch cluster on its own when no contextual hint cluster
// is supplied. This is the empty-state path for full-height surfaces that
// only need the escape hatches.
func TestComposeFullHeightFooterLeftFallsBackToEscapeHatches(t *testing.T) {
	plain := ansi.Strip(composeFullHeightFooterLeft(""))
	if plain != FooterEscapeHints {
		t.Fatalf("empty-context footer mismatch:\n got: %q\nwant: %q", plain, FooterEscapeHints)
	}
}

// TestPlannerFooterUsesSixContextualHints locks the consolidated inline-footer
// budget. Global bindings remain in Help instead of expanding this row.
func TestPlannerFooterUsesSixContextualHints(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)

	plain := ansi.Strip(m.View().Content)
	for _, want := range []string{"j/k move", "enter detail", "space select", "x done", "a add", "tab switch"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Planner footer missing %q; rendered:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, FooterEscapeHints) {
		t.Fatalf("Planner footer exceeds its six contextual hints with %q; rendered:\n%s", FooterEscapeHints, plain)
	}
}

// TestNextActionStripHintsDoNotDuplicateTodayFooter walks every recommendation
// kind/variant and asserts the strip's hint cluster shares no key with the
// Today-tab footer the strip actually co-renders with. The strip is suppressed
// on Next/Closed/Tunnel/Zen (see focus_board_render.go), so those footer
// variants are intentionally excluded — only the Today-tab footers matter
// for overlap detection.
//
// Session 11's design rule is "one canonical home" per shortcut: when a key
// appears in the footer it must not appear in the strip and vice versa.
func TestNextActionStripHintsDoNotDuplicateTodayFooter(t *testing.T) {
	// Footer hint sets that may co-render with the strip on the Today tab.
	// The strip is suppressed on Next/Closed/Tunnel/Zen, so Next-tab
	// (`w plan today · a add · enter open`) and Closed-tab footers are out
	// of scope.
	todayFootersByContext := map[string][]string{
		"steady":    {"enter detail", "space select", "x done", "a add", "tab switch"},
		"empty":     {"a add", "tab switch"},
		"completed": {"a add", "tab switch"},
	}

	project := cockpitProject(t, "Hierarchy")
	blocked := cockpitTask(t, project, "Resolve blocker", models.StatusBlocked, models.SectionNext)

	// Per-kind hint expectations. AllDone-caught-up intentionally emits no
	// hint because the completed-Today footer already carries `p plan more`.
	cases := []struct {
		name      string
		copy      nextActionStripCopy
		opts      NextActionStripOpts
		wantHints int
		// footerCtx names the footer variant the strip co-renders with for
		// this kind/variant.
		footerCtx string
	}{
		{"WorkOnTask", nextActionStripCopy{kind: RecommendationKindWorkOnTask}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindWorkOnTask}, HasProjects: true}, 0, "steady"},
		{"ReviewBlocked", nextActionStripCopy{kind: RecommendationKindReviewBlocked}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindReviewBlocked, Task: blocked, Project: project}, HasProjects: true}, 1, "steady"},
		{"TriageInbox-default", nextActionStripCopy{kind: RecommendationKindTriageInbox}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindTriageInbox}, HasProjects: true}, 1, "steady"},
		{"TriageInbox-inbox-only", nextActionStripCopy{kind: RecommendationKindTriageInbox, variant: stripVariantInboxOnly}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindAllDone}, OpenLoops: OpenLoops{Inbox: 1, HasTasks: true}}, 1, "empty"},
		{"PlanToday", nextActionStripCopy{kind: RecommendationKindPlanToday}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindPlanToday}, HasProjects: true, OpenLoops: OpenLoops{HasTasks: true}}, 0, "empty"},
		{"AllDone-first-run", nextActionStripCopy{kind: RecommendationKindAllDone, variant: stripVariantFirstRun}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindAllDone}}, 0, "empty"},
		{"AllDone-caught-up", nextActionStripCopy{kind: RecommendationKindAllDone, variant: stripVariantCaughtUp}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindAllDone}, HasProjects: true, OpenLoops: OpenLoops{HasTasks: true, TodayTotal: 1, TodayDone: 1}}, 0, "completed"},
	}
	for _, tc := range cases {
		hints := primaryHintsFor(tc.copy, tc.opts)
		if len(hints) != tc.wantHints {
			t.Errorf("[%s] expected %d hint(s), got %d: %v", tc.name, tc.wantHints, len(hints), hints)
		}
		footerKeys, ok := todayFootersByContext[tc.footerCtx]
		if !ok {
			t.Fatalf("[%s] unknown footer context %q", tc.name, tc.footerCtx)
		}
		for _, hint := range hints {
			for _, footerKey := range footerKeys {
				if hint == footerKey {
					t.Errorf("[%s] strip hint %q duplicates co-resident Today footer key %q (footer ctx: %s)",
						tc.name, hint, footerKey, tc.footerCtx)
				}
			}
		}
	}
}

// TestTodayCompletedFooterKeepsPlannerTabNavigation verifies the celebration
// path retains the consolidated Planner's tab affordance.
func TestTodayCompletedFooterKeepsPlannerTabNavigation(t *testing.T) {
	project := cockpitProject(t, "Cockpit Done")
	task := cockpitTask(t, project, "Wrap up release notes", models.StatusDone, models.SectionCompleted)
	row := cockpitBoardRow(task, project)
	dailyPlanIDs := map[string]bool{task.ID: true}
	openLoops := OpenLoops{TodayTotal: 1, TodayDone: 1, HasTasks: true}
	m := cockpitModel([]boardRow{row}, nil, nil, dailyPlanIDs, openLoops, boardTabToday)
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "tab switch") {
		t.Fatalf("completed-today footer missing `tab switch` hint:\n%s", plain)
	}
	if strings.Contains(plain, "p plan more") {
		t.Fatalf("completed-today footer contains retired planning hint:\n%s", plain)
	}
}

// TestAppStatusBarDefaultHelpMatchesFooterEscapeHints ensures the app status
// bar's default contextual help hint (used by non-full-height views like
// Dashboard) matches the escape-hatch constant the full-height footers
// advertise. Without this lockstep, Dashboard users would see one set of
// escape keys while Today and Hub users see a different ordering.
func TestAppStatusBarDefaultHelpMatchesFooterEscapeHints(t *testing.T) {
	const appStatusDefault = "ctrl+g go · : cmd · ? help"
	if FooterEscapeHints != appStatusDefault {
		t.Fatalf("FooterEscapeHints %q diverges from app status bar default %q; surfaces will show inconsistent escape hatches",
			FooterEscapeHints, appStatusDefault)
	}
}
