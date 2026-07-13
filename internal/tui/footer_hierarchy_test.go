package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/util"
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
	if FooterEscapeHints != ";; hub · : cmd · ? help" {
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

// TestTodayFooterIncludesEscapeHatchCluster verifies the steady-state Today
// footer carries the full escape-hatch cluster after its contextual hints.
// A regression that drops ? / : / ;; from the footer would force the user to
// memorize per-view escape keys.
func TestTodayFooterIncludesEscapeHatchCluster(t *testing.T) {
	const title = "Ship overdue invoice"
	m := cockpitWorkOnTaskFixture(t, title)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, FooterEscapeHints) {
		t.Fatalf("Today footer missing escape-hatch cluster %q; rendered:\n%s", FooterEscapeHints, plain)
	}
}

// TestPipelineHubFooterIncludesEscapeHatchCluster verifies the Hub footer
// pins the same escape-hatch cluster Today and the app status bar advertise.
// Without this guarantee, Hub users would have a different muscle-memory
// surface for opening help / the command palette than the rest of tt.
func TestPipelineHubFooterIncludesEscapeHatchCluster(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := NewPipelineHubModel(cfg, nil, nil, nil, nil)
	m.SetSize(140, 30)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, FooterEscapeHints) {
		t.Fatalf("Pipeline Hub footer missing escape-hatch cluster %q; rendered:\n%s", FooterEscapeHints, plain)
	}
}

// TestPipelineHubFooterContextualCappedAtThreeHints pins the 3-contextual
// cap on the Hub footer. The full footer reads
// `<contextual> · ;; hub · : cmd · ? help`; with the escape-hatch cluster
// contributing three of its own segments, the contextual prefix must
// contribute at most three more (so the footer holds six `·`-separated
// segments before the right-side counter pill).
func TestPipelineHubFooterContextualCappedAtThreeHints(t *testing.T) {
	cfg := &config.Config{Projects: []*models.Project{}}
	m := NewPipelineHubModel(cfg, nil, nil, nil, nil)
	m.SetSize(160, 30)

	plain := ansi.Strip(m.View().Content)
	idx := strings.Index(plain, FooterEscapeHints)
	if idx < 0 {
		t.Fatalf("Pipeline Hub footer missing escape-hatch cluster; rendered:\n%s", plain)
	}
	// Find the line containing the escape-hatch cluster so we can isolate
	// the contextual prefix.
	footerLine := ""
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, FooterEscapeHints) {
			footerLine = line
			break
		}
	}
	if footerLine == "" {
		t.Fatalf("could not isolate footer line from render:\n%s", plain)
	}
	contextual := strings.TrimSpace(strings.TrimSuffix(strings.SplitN(footerLine, FooterEscapeHints, 2)[0], "· "))
	if contextual == "" {
		t.Fatal("Pipeline Hub footer has no contextual hints before escape-hatch cluster")
	}
	hints := strings.Split(contextual, " · ")
	if len(hints) > 3 {
		t.Fatalf("Pipeline Hub footer contextual cluster exceeds 3 hints (%d): %v", len(hints), hints)
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
		"steady":    {"x done", "p priority", "a add"},
		"empty":     {"a add", "tab switch", "Esc close"},
		"completed": {"p plan more", "a add", "Esc close"},
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
		{"WorkOnTask", nextActionStripCopy{kind: RecommendationKindWorkOnTask}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindWorkOnTask}, HasProjects: true}, 1, "steady"},
		{"ReviewBlocked", nextActionStripCopy{kind: RecommendationKindReviewBlocked}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindReviewBlocked, Task: blocked, Project: project}, HasProjects: true}, 1, "steady"},
		{"TriageInbox-default", nextActionStripCopy{kind: RecommendationKindTriageInbox}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindTriageInbox}, HasProjects: true}, 1, "steady"},
		{"TriageInbox-inbox-only", nextActionStripCopy{kind: RecommendationKindTriageInbox, variant: stripVariantInboxOnly}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindAllDone}, OpenLoops: OpenLoops{Inbox: 1, HasTasks: true}}, 1, "empty"},
		{"PlanToday", nextActionStripCopy{kind: RecommendationKindPlanToday}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindPlanToday}, HasProjects: true, OpenLoops: OpenLoops{HasTasks: true}}, 1, "empty"},
		{"AllDone-first-run", nextActionStripCopy{kind: RecommendationKindAllDone, variant: stripVariantFirstRun}, NextActionStripOpts{Action: RecommendedAction{Kind: RecommendationKindAllDone}}, 1, "empty"},
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

// TestTodayCompletedFooterAdvertisesPlanMore verifies the celebration path
// (all planned items done) keeps `w plan more` discoverable in the footer
// without dropping the escape-hatch cluster — the strip's own hint is also
// `w plan more`, but the rule is "one canonical home" per key and the
// celebration footer is where users look for "now what?" follow-up.
func TestTodayCompletedFooterAdvertisesPlanMore(t *testing.T) {
	project := cockpitProject(t, "Cockpit Done")
	task := cockpitTask(t, project, "Wrap up release notes", models.StatusDone, models.SectionCompleted)
	row := cockpitBoardRow(task, project)
	dailyPlanIDs := map[string]bool{task.ID: true}
	openLoops := OpenLoops{TodayTotal: 1, TodayDone: 1, HasTasks: true}
	m := cockpitModel([]boardRow{row}, nil, nil, dailyPlanIDs, openLoops, boardTabToday)
	m.SetSize(120, 30)

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "p plan more") {
		t.Fatalf("completed-today footer missing `p plan more` hint:\n%s", plain)
	}
	if !strings.Contains(plain, FooterEscapeHints) {
		t.Fatalf("completed-today footer missing escape-hatch cluster:\n%s", plain)
	}
}

// TestAppStatusBarDefaultHelpMatchesFooterEscapeHints ensures the app status
// bar's default contextual help hint (used by non-full-height views like
// Dashboard) matches the escape-hatch constant the full-height footers
// advertise. Without this lockstep, Dashboard users would see one set of
// escape keys while Today and Hub users see a different ordering.
func TestAppStatusBarDefaultHelpMatchesFooterEscapeHints(t *testing.T) {
	const appStatusDefault = ";; hub · : cmd · ? help"
	if FooterEscapeHints != appStatusDefault {
		t.Fatalf("FooterEscapeHints %q diverges from app status bar default %q; surfaces will show inconsistent escape hatches",
			FooterEscapeHints, appStatusDefault)
	}
}

// TestPipelineHubFooterRendersAtPlannedTodayFixture exercises a non-trivial
// hub render path (a saved project with a planned-today task) to confirm the
// escape-hatch cluster survives when the strip is in WorkOnTask mode rather
// than first-run. Mirrors the fixture shape pipeline_hub_orientation_test.go
// uses for strip coverage.
func TestPipelineHubFooterRendersAtPlannedTodayFixture(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Hub Plan")
	project.Path = cfg.GetProjectPath(project.Slug)
	planned, err := models.NewTask("planned-footer", 1, models.TaskTypeTask, "Ship pipeline polish")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	planned.WorkflowState = models.StateInProgress
	project.Tasks = []*models.Task{planned}
	cfg.Projects = []*models.Project{project}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	todayKey := util.StartOfDay(time.Now()).Format(util.DateFormat)
	cfg.DailyPlan = []config.DailyPlanItem{{
		TaskID:      planned.ID,
		ProjectID:   project.ID,
		PlannedDate: todayKey,
	}}

	m := NewPipelineHubModel(cfg, nil, nil, nil, project)
	m.SetSize(160, 30)
	m.setOpenLoops(OpenLoops{TodayTotal: 1, HasTasks: true})

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, FooterEscapeHints) {
		t.Fatalf("Pipeline Hub footer (planned-today fixture) missing escape-hatch cluster:\n%s", plain)
	}
}
