package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// -----------------------------------------------------------------------------
// Cockpit scenario fixtures
// -----------------------------------------------------------------------------
//
// Sessions 2-6 of the UX redesign land a Today cockpit that surfaces an
// explainable next action above the planned-task list. Session 1 locks down
// the seven scenarios that the cockpit must handle so later sessions inherit
// stable baseline coverage:
//
//   1. empty app          - no projects, no tasks
//   2. inbox only         - tasks exist but none are planned/active
//   3. overdue            - one or more due-date misses
//   4. planned today      - daily-plan rows visible on Today tab
//   5. blocked            - planned-today task with StatusBlocked
//   6. today complete     - every planned-today task is done
//   7. many projects      - cross-project aggregation at scale
//
// Every cockpit fixture renders at each `baselineSizes` cell and, when the
// content fits, also at `narrowBaseline`. The render output is asserted for
// (a) visual line width within budget, (b) primary chrome visibility, and
// (c) no footer/content overlap. The chrome assertions intentionally target
// the *current* Today surface; sessions 4-5 extend them when the Next Action
// strip lands. See `assertBaselineLineWidth`, `assertPrimaryActionsVisible`,
// and `assertNoFooterContentOverlap` in baseline_render_test.go for the
// shared assertion helpers.

// cockpitProject creates a bare in-memory project with a stable slug. We avoid
// `testutil.NewTestProject` because it expects a non-empty path environment for
// some callers; the cockpit fixtures only need the Name field populated for
// per-project labels and the ID for dailyPlanIDs membership.
func cockpitProject(t *testing.T, name string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		t.Fatalf("cockpitProject(%q): %v", name, err)
	}
	p.Slug = models.Slugify(name)
	return p
}

// cockpitTask creates a task with the given title, status, and section. The
// task is appended to project.Tasks so subsequent boardRow{} values can
// reference both Task and Project safely.
func cockpitTask(t *testing.T, project *models.Project, title string, status models.TaskStatus, section string) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), len(project.Tasks)+1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("cockpitTask(%q): %v", title, err)
	}
	task.WorkflowState = legacyStateForStatusSection(status, section)
	if status == models.StatusDone {
		now := time.Now()
		task.CompletedAt = &now
	}
	project.Tasks = append(project.Tasks, task)
	return task
}

// cockpitBoardRow wraps a task in a boardRow with Source set to project.Name
// so the renderer can group/render cross-project context.
func cockpitBoardRow(task *models.Task, project *models.Project) boardRow {
	return boardRow{
		Task:     task,
		Project:  project,
		Priority: task.Priority,
		Source:   project.Name,
	}
}

// cockpitModel constructs a minimal FocusBoardModel with planted rows. It
// bypasses NewFocusBoardModel/loadItems so tests do not need a temp DataDir
// and so each scenario controls its own data shape. The active tab and the
// openLoops counter must be set explicitly by the caller because they drive
// the title-row content the cockpit cares about.
func cockpitModel(today, next, _ []boardRow, dailyPlanIDs map[string]bool, openLoops OpenLoops, activeTab int) *FocusBoardModel {
	m := newTestFocusBoard()
	if dailyPlanIDs == nil {
		dailyPlanIDs = map[string]bool{}
	}
	m.dailyPlanIDs = dailyPlanIDs
	// Compose a shared dailyPlan slice mirroring dailyPlanIDs so the focus
	// board's posMap lookups in sorting do not panic on nil indirection.
	plan := make([]FocusTask, 0, len(today))
	for _, r := range today {
		if r.Task == nil || !dailyPlanIDs[r.Task.ID] {
			continue
		}
		plan = append(plan, FocusTask{Task: r.Task, Project: r.Project})
	}
	*m.dailyPlan = plan
	m.todayRows = today
	m.nextRows = next
	// allRows mirrors loadItems(): the union of non-complete, non-snoozed rows
	// across active surfaces. Tests use it only as a tag pool, so the union of
	// today+next is sufficient for chrome rendering.
	m.allRows = make([]boardRow, 0, len(today)+len(next))
	m.allRows = append(m.allRows, today...)
	m.allRows = append(m.allRows, next...)
	m.tabs.Active = activeTab
	m.applyFilter()
	m.setOpenLoops(openLoops)
	return m
}

// assertCockpitContains fails the test when needle is missing from haystack
// after ANSI stripping. It is the counter-counterpart to
// assertPrimaryActionsVisible for non-action strings (counter pills, subtitle
// copy). Named with a `Cockpit` prefix to avoid colliding with the existing
// `assertContains` helper in cross_project_outliner_render_test.go, which has
// a different argument order.
func assertCockpitContains(t *testing.T, label, haystack, needle string) {
	t.Helper()
	plain := ansi.Strip(haystack)
	if !strings.Contains(plain, needle) {
		t.Fatalf("%s: missing %q\n\n%s", label, needle, plain)
	}
}

// -----------------------------------------------------------------------------
// 1. Empty app
// -----------------------------------------------------------------------------

// TestBaselineCockpitEmptyAppRendersAcrossSizes locks down the first-run
// Today cockpit when no projects and no tasks exist. The empty-state path
// must surface a discoverable hint at every baseline width including narrow.
func TestBaselineCockpitEmptyAppRendersAcrossSizes(t *testing.T) {
	sizes := append([]baselineSize{}, baselineSizes...)
	sizes = append(sizes, narrowBaseline)

	for _, sz := range sizes {
		t.Run(sz.label, func(t *testing.T) {
			m := cockpitModel(nil, nil, nil, nil, OpenLoops{}, boardTabToday)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "cockpit empty " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Today",
				"Capture your first task",
				"a add",
			)
			// Next Action strip first-run copy must surface a discoverable
			// onboarding cue when no projects and no tasks exist. Session 4
			// lands the strip; session 6 layers an action-oriented body
			// below it ("Capture your first task to get started.") whose
			// Planner add affordance states the
			// keybinding for the strip's CTA.
			assertCockpitContains(t, label, rendered, "Create your first project")
			assertCockpitContains(t, label, rendered, "Capture your first task to get started.")
			assertCockpitContains(t, label, rendered, "a add")
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// -----------------------------------------------------------------------------
// 2. Inbox only
// -----------------------------------------------------------------------------

// TestBaselineCockpitInboxOnlyRendersAcrossSizes covers the state where the
// user has captured tasks but none are scheduled or actively in flight. The
// Today cockpit must keep the inbox count visible in the openLoops pill at
// every desktop width; narrow drops the pill into the title row but still
// must not overflow.
func TestBaselineCockpitInboxOnlyRendersAcrossSizes(t *testing.T) {
	sizes := append([]baselineSize{}, baselineSizes...)
	sizes = append(sizes, narrowBaseline)

	for _, sz := range sizes {
		t.Run(sz.label, func(t *testing.T) {
			openLoops := OpenLoops{Inbox: 4, HasTasks: true}
			m := cockpitModel(nil, nil, nil, nil, openLoops, boardTabToday)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "cockpit inbox-only " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Today",
				"waiting in your inbox",
			)
			// The openLoops pill (`inbox 4`) is the cockpit's only signal that
			// triage work exists when nothing is planned. Validate at every
			// width — sessions 4-5 must keep this contract while landing the
			// Next Action strip on top.
			assertCockpitContains(t, label, rendered, "inbox 4")
			// Next Action strip overlay: when openLoops.Inbox > 0 and the
			// engine returns AllDone (no actionable tasks loaded), the strip
			// pulls the inbox count out of the pill and into a direct CTA.
			assertCockpitContains(t, label, rendered, "Triage inbox")
			assertCockpitContains(t, label, rendered, "4 items")
			// Session 6 empty-state body restates the inbox count below the
			// strip with the triage keybinding so the user sees both the
			// strip CTA and the long-form action hint.
			assertCockpitContains(t, label, rendered, "4 items waiting in your inbox.")
			assertCockpitContains(t, label, rendered, ": manage inbox")
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// -----------------------------------------------------------------------------
// 3. Overdue
// -----------------------------------------------------------------------------

// TestBaselineCockpitOverdueRendersAcrossSizes covers the warning-state
// subtitle that fires when the user has overdue tasks but no Today plan. The
// header switches accent color and the subtitle reads `0 planned · ⚠ N
// overdue`. Narrow is excluded because the warning subtitle plus the title
// counter cannot all share a 60-column row today; session 11 (Footer And
// Help Hierarchy) will tighten the narrow chrome.
func TestBaselineCockpitOverdueRendersAcrossSizes(t *testing.T) {
	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			project := cockpitProject(t, "Cockpit Overdue")
			past := time.Now().Add(-48 * time.Hour)
			overdue := cockpitTask(t, project, "Ship overdue invoice", models.StatusTodo, models.SectionNext)
			overdue.DueDate = &past
			row := cockpitBoardRow(overdue, project)

			openLoops := OpenLoops{Overdue: 1, HasTasks: true}
			m := cockpitModel(nil, []boardRow{row}, nil, nil, openLoops, boardTabToday)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "cockpit overdue " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Today",
				"0 planned",
				"overdue",
			)
			assertCockpitContains(t, label, rendered, "overdue 1")
			// Next Action strip surfaces the overdue task name and reason
			// badge above the (empty) Today list. The strip's WorkOnTask
			// branch wins because tier=tierOverdue (0) <= tierCarriedOver.
			assertCockpitContains(t, label, rendered, "Next: Ship overdue invoice")
			assertCockpitContains(t, label, rendered, "overdue")
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// -----------------------------------------------------------------------------
// 4. Planned today
// -----------------------------------------------------------------------------

// TestBaselineCockpitPlannedTodayRendersAcrossSizes covers the populated
// Today tab with a small daily plan. Subtitle reads `N tasks · 0/N done`,
// the openLoops pill shows `today 0/N`, and the footer offers the in-flight
// triage shortcuts (`x done`, `p priority`, `a add`). Narrow is excluded
// because the row metadata strip + footer + title overflows 20 rows once at
// least one row is present.
func TestBaselineCockpitPlannedTodayRendersAcrossSizes(t *testing.T) {
	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			project := cockpitProject(t, "Cockpit Plan")
			plan := []*models.Task{
				cockpitTask(t, project, "Write design doc draft", models.StatusInProgress, models.SectionNext),
				cockpitTask(t, project, "Review staging deploy", models.StatusTodo, models.SectionNext),
			}
			dailyPlanIDs := map[string]bool{}
			rows := make([]boardRow, 0, len(plan))
			for _, task := range plan {
				dailyPlanIDs[task.ID] = true
				rows = append(rows, cockpitBoardRow(task, project))
			}

			openLoops := OpenLoops{TodayTotal: len(plan), HasTasks: true}
			m := cockpitModel(rows, nil, nil, dailyPlanIDs, openLoops, boardTabToday)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "cockpit planned-today " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Today",
				"x done",
				"space",
				"a add",
			)
			assertCockpitContains(t, label, rendered, fmt.Sprintf("today 0/%d", len(plan)))
			// The Today subtitle ends in `· 0/N done` regardless of how many
			// rows are in m.rows (which is a separate "active/due-soon" axis).
			// Locking the done suffix keeps the planned-vs-done contract
			// stable through sessions 4-5.
			assertCockpitContains(t, label, rendered, fmt.Sprintf("0/%d done", len(plan)))
			assertCockpitContains(t, label, rendered, "Write design doc draft")
			// Next Action strip picks up the top planned-today task and the
			// "planned today" reason badge. The specific task title depends
			// on engine sort tiebreaks (UpdatedAt then task.ID), so this
			// asserts the structural pieces — the "Next:" prefix and the
			// reason badge — rather than locking the exact title.
			assertCockpitContains(t, label, rendered, "Next: ")
			assertCockpitContains(t, label, rendered, "planned today")
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// -----------------------------------------------------------------------------
// 5. Blocked
// -----------------------------------------------------------------------------

// TestBaselineCockpitBlockedRendersAcrossSizes covers a planned-today task
// that is currently blocked. The cockpit must still render the row and
// surface enough context that the user understands why the task is parked.
// Session 2 introduces an explicit "blocked" reason badge; this baseline
// only asserts the status word appears so that work remains backward
// compatible with the existing status-badge renderer.
func TestBaselineCockpitBlockedRendersAcrossSizes(t *testing.T) {
	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			project := cockpitProject(t, "Cockpit Blocked")
			blocker := cockpitTask(t, project, "Resolve auth handoff", models.StatusTodo, models.SectionNext)
			blocked := cockpitTask(t, project, "Wire SSO callback", models.StatusBlocked, models.SectionNext)
			_ = blocked.AddDependency(blocker.ID)

			dailyPlanIDs := map[string]bool{blocked.ID: true}
			rows := []boardRow{cockpitBoardRow(blocked, project)}

			openLoops := OpenLoops{TodayTotal: 1, HasTasks: true}
			m := cockpitModel(rows, nil, nil, dailyPlanIDs, openLoops, boardTabToday)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "cockpit blocked " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Today",
				"Wire SSO callback",
			)
			// The blocked status badge currently renders as the ⊘ glyph.
			// Sessions 2-3 add a richer "blocked" reason badge in the Next
			// Action strip; until then this baseline locks the glyph in
			// place so a regression that drops the visual cue fails fast.
			assertCockpitContains(t, label, rendered, "⊘")
			// Next Action strip routes the blocked task through the
			// ReviewBlocked kind because the only actionable task ("Resolve
			// auth handoff") has no urgency reason (tier=tierFallback),
			// failing the urgent-actionable gate. Strip renders "Review
			// blocked: …" with the unblock hint.
			assertCockpitContains(t, label, rendered, "Review blocked: Wire SSO callback")
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// -----------------------------------------------------------------------------
// 6. Today complete
// -----------------------------------------------------------------------------

// TestBaselineCockpitTodayCompleteRendersAcrossSizes covers the celebratory
// state when every planned-today task is done. The title becomes "Today ✓",
// the subtitle reads "All done!", the openLoops pill collapses to
// `today ✓`, and the footer offers `p plan more`. Session 6 (Today Empty
// And Completion States) tightens the surrounding momentum copy; this
// baseline locks the structural pieces that copy will sit inside.
func TestBaselineCockpitTodayCompleteRendersAcrossSizes(t *testing.T) {
	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			project := cockpitProject(t, "Cockpit Done")
			done1 := cockpitTask(t, project, "Ship demo recording", models.StatusDone, models.SectionCompleted)
			done2 := cockpitTask(t, project, "Email stakeholders", models.StatusDone, models.SectionCompleted)

			dailyPlanIDs := map[string]bool{done1.ID: true, done2.ID: true}
			rows := []boardRow{
				cockpitBoardRow(done1, project),
				cockpitBoardRow(done2, project),
			}

			openLoops := OpenLoops{TodayTotal: 2, TodayDone: 2, HasTasks: true}
			m := cockpitModel(rows, nil, nil, dailyPlanIDs, openLoops, boardTabToday)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "cockpit today-complete " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Today",
				"All done!",
				"tab",
			)
			assertCockpitContains(t, label, rendered, "today ✓")
			// Next Action strip caught-up variant — every actionable task is
			// done, openLoops reports tasks exist (HasTasks=true) and no
			// inbox triage is pending, so the AllDone overlay routes to
			// "All caught up for today" instead of the first-run cue.
			assertCockpitContains(t, label, rendered, "All caught up for today")
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}

// -----------------------------------------------------------------------------
// 7. Many projects
// -----------------------------------------------------------------------------

// TestBaselineCockpitManyProjectsRendersAcrossSizes covers the cross-project
// aggregation case the cockpit must not regress on. Eight projects each
// contribute one Next-section task; the Next tab must render every project's
// row source within budget at each desktop width. The Today tab stays empty
// here because the scenario is about "many projects, no plan yet" — the
// user's cue to triage rather than execute.
func TestBaselineCockpitManyProjectsRendersAcrossSizes(t *testing.T) {
	for _, sz := range baselineSizes {
		t.Run(sz.label, func(t *testing.T) {
			rows := make([]boardRow, 0, 8)
			for i := 0; i < 8; i++ {
				p := cockpitProject(t, fmt.Sprintf("Proj%02d", i+1))
				task := cockpitTask(t, p, fmt.Sprintf("Next step %d", i+1), models.StatusTodo, models.SectionNext)
				rows = append(rows, cockpitBoardRow(task, p))
			}

			openLoops := OpenLoops{HasTasks: true}
			m := cockpitModel(nil, rows, nil, nil, openLoops, boardTabNext)
			m.SetSize(sz.width, sz.height)

			rendered := m.View().Content
			label := "cockpit many-projects " + sz.label

			assertBaselineLineWidth(t, label, rendered, sz.width)
			assertPrimaryActionsVisible(t, label, rendered,
				"Next step 1",
				"Next step 8",
				"tab",
			)
			assertNoFooterContentOverlap(t, label, rendered, sz.height)
		})
	}
}
