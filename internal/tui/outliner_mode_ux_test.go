package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// newWorkModeOutliner builds a single-project outliner backed by a real runtime
// so mode selection (which rebuilds rows through the daily-plan/ranker inputs)
// runs end to end.
func newWorkModeOutliner(t *testing.T) *OutlinerModel {
	t.Helper()
	tasks := []*models.Task{
		makeTask("t-inbox", "Capture something", models.SectionInbox),
		makeTask("t-next", "Do the thing", models.SectionNext),
		makeTask("t-active", "In progress", ""),
	}
	project := renderFixtureProject(t, "proj-modes", "mod", "Modes Project", tasks)

	cfg := newTestConfig()
	m := NewOutlinerModel(project, nil, testRuntimeAt(cfg.DataDir(), cfg))
	m.SetSize(120, 24)
	m.rebuildRows()
	return m
}

func TestSelectOutlinerModeMapsToState(t *testing.T) {
	m := newWorkModeOutliner(t)

	cases := []struct {
		id     OutlinerModeID
		verify func(t *testing.T)
	}{
		{ModeToday, func(t *testing.T) {
			if !m.workTodayMode || m.workBacklogMode {
				t.Fatalf("Today: workTodayMode=%t workBacklogMode=%t", m.workTodayMode, m.workBacklogMode)
			}
		}},
		{ModeBacklog, func(t *testing.T) {
			if m.workTodayMode || !m.workBacklogMode {
				t.Fatalf("Backlog: workTodayMode=%t workBacklogMode=%t", m.workTodayMode, m.workBacklogMode)
			}
		}},
		{ModeActive, func(t *testing.T) {
			if m.workTodayMode || m.workBacklogMode {
				t.Fatal("Active must clear computed modes")
			}
			if m.activePerspective != PerspectiveActive || m.smartFocus != nil {
				t.Fatalf("Active: perspective=%d smartFocus=%v", m.activePerspective, m.smartFocus)
			}
		}},
		{ModeInbox, func(t *testing.T) {
			if m.activePerspective != PerspectiveInbox {
				t.Fatalf("Inbox: perspective=%d", m.activePerspective)
			}
		}},
		{ModeDueSoon, func(t *testing.T) {
			if m.smartFocus == nil || *m.smartFocus != OutlinerDueSoon {
				t.Fatalf("Due Soon: smartFocus=%v", m.smartFocus)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(string(tc.id), func(t *testing.T) {
			if _, cmd := m.selectOutlinerMode(tc.id); cmd != nil {
				_ = cmd()
			}
			tc.verify(t)
			if got := m.activeOutlinerMode(); got != tc.id {
				t.Fatalf("activeOutlinerMode() = %q, want %q", got, tc.id)
			}
		})
	}
}

func TestCycleWorkModeDeterministic(t *testing.T) {
	m := newWorkModeOutliner(t)

	// Forward from All wraps to Today, then walks the whole cycle.
	m.selectOutlinerMode(ModeAll)
	forward := []OutlinerModeID{ModeToday, ModeNext, ModeBacklog, ModeActive, ModeAll}
	for _, want := range forward {
		m.cycleWorkMode(1)
		if got := m.activeOutlinerMode(); got != want {
			t.Fatalf("forward cycle = %q, want %q", got, want)
		}
	}

	// Backward from Today wraps to All.
	m.selectOutlinerMode(ModeToday)
	m.cycleWorkMode(-1)
	if got := m.activeOutlinerMode(); got != ModeAll {
		t.Fatalf("backward from Today = %q, want All", got)
	}

	// From a mode outside the five-item cycle, forward enters at Today and
	// backward at All.
	m.selectOutlinerMode(ModeStandby)
	m.cycleWorkMode(1)
	if got := m.activeOutlinerMode(); got != ModeToday {
		t.Fatalf("forward from outside cycle = %q, want Today", got)
	}
	m.selectOutlinerMode(ModeStandby)
	m.cycleWorkMode(-1)
	if got := m.activeOutlinerMode(); got != ModeAll {
		t.Fatalf("backward from outside cycle = %q, want All", got)
	}
}

func TestIsComputedOrderMode(t *testing.T) {
	m := newWorkModeOutliner(t)
	cases := []struct {
		id   OutlinerModeID
		want bool
	}{
		{ModeToday, true},
		{ModeBacklog, true},
		{ModeDueDateLens, true},
		{ModeDailyReview, true},
		{ModeActivity, true},
		{ModeActive, false},
		{ModeAll, false},
		{ModeInbox, false},
		{ModeNext, false},
		{ModeDueSoon, false},
	}
	for _, tc := range cases {
		m.selectOutlinerMode(tc.id)
		if got := m.isComputedOrderMode(); got != tc.want {
			t.Errorf("isComputedOrderMode after %q = %t, want %t", tc.id, got, tc.want)
		}
	}
}

func TestModePickerFiltersByLabelAndAlias(t *testing.T) {
	// "back" matches Backlog by label.
	rows := selectableModes(modePickerRows("back"))
	if len(rows) != 1 || rows[0].mode.ID != ModeBacklog {
		t.Fatalf("query 'back' matched %v, want [Backlog]", modeIDs(rows))
	}
	// "done" includes Completed by its alias. Fuzzy matching may also include
	// later categories, but must preserve category/catalog order.
	rows = selectableModes(modePickerRows("done"))
	if len(rows) == 0 || rows[0].mode.ID != ModeComplete {
		t.Fatalf("query 'done' matched %v, want Completed first", modeIDs(rows))
	}
	// Review modes now render as Outliner modes and list as selectable.
	rows = selectableModes(modePickerRows("activity"))
	if len(rows) != 1 || rows[0].disabled {
		t.Fatalf("Activity should list as selectable, got %v", rows)
	}

	// Non-contiguous queries prove matching is fuzzy rather than substring-based.
	rows = selectableModes(modePickerRows("bklg"))
	if len(rows) != 1 || rows[0].mode.ID != ModeBacklog {
		t.Fatalf("fuzzy label query 'bklg' matched %v, want [Backlog]", modeIDs(rows))
	}
	rows = selectableModes(modePickerRows("shdl"))
	if len(rows) != 1 || rows[0].mode.ID != ModeDueDateLens {
		t.Fatalf("fuzzy alias query 'shdl' matched %v, want [Due Date]", modeIDs(rows))
	}
}

func TestModePickerUnicodeInputAndBackspace(t *testing.T) {
	m := newWorkModeOutliner(t)
	m.openModePicker()
	m.modePicker.cursor = 3
	m.modePicker.scroll = 4

	for _, r := range []rune{'é', '界'} {
		m.handleModePickerKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if got := m.modePicker.query; got != "é界" || !utf8.ValidString(got) {
		t.Fatalf("Unicode input query = %q (valid UTF-8: %t), want %q", got, utf8.ValidString(got), "é界")
	}
	if m.modePicker.cursor != 0 || m.modePicker.scroll != 0 {
		t.Fatalf("query change did not reset cursor/scroll: cursor=%d scroll=%d", m.modePicker.cursor, m.modePicker.scroll)
	}

	for _, want := range []string{"é", "", ""} {
		m.handleModePickerKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
		if got := m.modePicker.query; got != want || !utf8.ValidString(got) {
			t.Fatalf("Backspace query = %q (valid UTF-8: %t), want %q", got, utf8.ValidString(got), want)
		}
	}

	for _, msg := range []tea.KeyPressMsg{
		{Code: 'x', Mod: tea.ModCtrl},
		{Code: '\n', Text: "\n"},
	} {
		m.handleModePickerKey(msg)
		if got := m.modePicker.query; got != "" {
			t.Fatalf("non-printable or modified key changed query to %q", got)
		}
	}
}

// TestModePickerFitsViewport guards FR13-004: the categorized mode picker must
// budget its title, help, and list rows against the current Outliner height,
// keep the selected mode visible with continuation indicators when clipped, and
// leave every category keyboard-accessible instead of rendering ~31 fixed lines
// that overflow common terminals.
func TestModePickerFitsViewport(t *testing.T) {
	selectable := selectableModes(modePickerRows(""))
	if len(selectable) == 0 {
		t.Fatal("expected a non-empty mode catalog")
	}

	sizes := []struct{ w, h int }{
		{80, 24},
		{72, 20},
		{100, 32}, // roughly the picker's full unclipped height
	}

	for _, sz := range sizes {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			m := newWorkModeOutliner(t)
			m.SetSize(sz.w, sz.h)
			m.openModePicker()

			// Walk the cursor across every selectable mode. Each must stay
			// visible (proving every category is keyboard-reachable) and the
			// panel must never exceed the terminal height.
			for i := range selectable {
				m.modePicker.cursor = i
				rendered := m.renderModePicker(sz.w)
				if h := lipgloss.Height(rendered); h > sz.h {
					t.Fatalf("cursor %d (%s): picker height %d exceeds terminal height %d",
						i, selectable[i].mode.Label, h, sz.h)
				}
				plain := ansi.Strip(rendered)
				if label := selectable[i].mode.Label; !strings.Contains(plain, label) {
					t.Fatalf("cursor %d: selected mode %q not visible:\n%s", i, label, plain)
				}
			}
		})
	}

	// When the list is clipped, the picker renders continuation indicators that
	// track the cursor position.
	t.Run("ContinuationIndicators", func(t *testing.T) {
		m := newWorkModeOutliner(t)
		m.SetSize(80, 24)
		m.openModePicker()

		m.modePicker.cursor = 0
		top := ansi.Strip(m.renderModePicker(80))
		if strings.Contains(top, "more above") {
			t.Fatalf("no above indicator expected at the top of the list:\n%s", top)
		}
		if !strings.Contains(top, "more below") {
			t.Fatalf("expected a below indicator when the list is clipped:\n%s", top)
		}

		m.modePicker.cursor = len(selectable) - 1
		bottom := ansi.Strip(m.renderModePicker(80))
		if !strings.Contains(bottom, "more above") {
			t.Fatalf("expected an above indicator when scrolled to the bottom:\n%s", bottom)
		}
		if strings.Contains(bottom, "more below") {
			t.Fatalf("no below indicator expected at the bottom of the list:\n%s", bottom)
		}
	})
}

func TestModePickerEnterSelectsMode(t *testing.T) {
	m := newWorkModeOutliner(t)
	m.openModePicker()
	if !m.modePickerActive() {
		t.Fatal("picker should be active after openModePicker")
	}
	m.modePicker.query = "backlog"
	if _, cmd := m.handleModePickerKey(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		_ = cmd()
	}
	if m.modePickerActive() {
		t.Fatal("picker should close after selection")
	}
	if !m.workBacklogMode {
		t.Fatal("selecting Backlog from picker did not enter Backlog mode")
	}
}

func TestWorkModeStripShowsCountsAndFixedOrderIndicator(t *testing.T) {
	m := newWorkModeOutliner(t)
	m.selectOutlinerMode(ModeToday)
	strip := ansi.Strip(m.renderWorkModeStrip(120))
	for _, label := range []string{"Today", "Next", "Backlog", "Active", "All"} {
		if !strings.Contains(strip, label) {
			t.Errorf("work-mode strip missing %q: %q", label, strip)
		}
	}
	if !strings.Contains(strip, "auto-order") {
		t.Errorf("Today mode strip should show the fixed-order indicator: %q", strip)
	}
}

// buildFilterParityProject builds a project whose tasks populate every computed
// mode: a StateNext task (Next), an actionable Ready task (Backlog), and a
// due-today Ready task placed on the daily plan by the caller (Today + Daily
// Review). All three appear in Activity.
func buildFilterParityProject(id, name string) *models.Project {
	now := time.Now()
	due := now
	// Fresh UpdatedAt keeps the undated Next/Backlog tasks out of the Daily
	// Review "Old" bucket, so Daily Review classifies only the due-today task.
	next := &models.Task{ID: id + "-next", Title: name + " next", WorkflowState: models.StateNext, UpdatedAt: now}
	backlog := &models.Task{ID: id + "-backlog", Title: name + " backlog", WorkflowState: models.StateReady, UpdatedAt: now}
	today := &models.Task{ID: id + "-today", Title: name + " today", WorkflowState: models.StateReady, DueDate: &due, UpdatedAt: now}
	project := &models.Project{ID: id, Name: name, Tasks: []*models.Task{next, backlog, today}}
	project.BuildTaskTree()
	return project
}

// taskRowProjectIDs returns the owning project ID of every task row (excluding
// section headers and separators) in render order.
func taskRowProjectIDs(m *OutlinerModel) []string {
	var ids []string
	for _, r := range m.rows {
		if r.task != nil && !r.IsSectionHeader {
			ids = append(ids, r.ProjectID)
		}
	}
	return ids
}

// sectionCountSum totals the SectionCount of every section header row, the
// review-mode analog of the work-mode tab count.
func sectionCountSum(m *OutlinerModel) int {
	sum := 0
	for _, r := range m.rows {
		if r.IsSectionHeader {
			sum += r.SectionCount
		}
	}
	return sum
}

// TestComputedModesHonorProjectFilter asserts that when unscoped with a
// non-empty F P filter, every computed mode (Today, Next, Backlog, Daily
// Review, Activity) enumerates only the selected projects in both its rows and
// its counts, matching the tree/Priority rebuild paths.
func TestComputedModesHonorProjectFilter(t *testing.T) {
	alpha := buildFilterParityProject("alpha", "Alpha")
	beta := buildFilterParityProject("beta", "Beta")

	runtime := testConfigWithProjects(alpha, beta)
	runtime.Planning().SetDailyPlan([]config.DailyPlanItem{
		{TaskID: "alpha-today", ProjectID: "alpha", PlannedDate: "2026-07-23"},
		{TaskID: "beta-today", ProjectID: "beta", PlannedDate: "2026-07-23"},
	})

	m := NewOutlinerModel(nil, nil, runtime)
	m.SetSize(120, 40)
	m.loadedProjects = []*models.Project{alpha, beta}

	// Control: with no filter both projects are in scope and computed rows
	// include beta, so the filtered assertions below are meaningful regressions.
	if got := m.workModeScopeProjects(); len(got) != 2 {
		t.Fatalf("unfiltered scope = %d projects, want 2", len(got))
	}
	m.selectOutlinerMode(ModeBacklog)
	if !containsProject(taskRowProjectIDs(m), "beta") {
		t.Fatalf("unfiltered Backlog omitted beta rows: %v", taskRowProjectIDs(m))
	}

	// Apply the F P filter to Alpha only.
	m.filterProjects = map[string]bool{"alpha": true}
	if got := m.workModeScopeProjects(); len(got) != 1 || got[0].ID != "alpha" {
		t.Fatalf("filtered scope = %v, want [alpha]", projectIDsOf(got))
	}

	// Work modes: rows are Alpha-only and the tab count matches the row count.
	workCases := []struct {
		id   OutlinerModeID
		want int
	}{
		{ModeToday, 1},
		{ModeNext, 1},
		{ModeBacklog, 1},
	}
	for _, tc := range workCases {
		t.Run(string(tc.id), func(t *testing.T) {
			m.selectOutlinerMode(tc.id)
			ids := taskRowProjectIDs(m)
			assertAllProject(t, ids, "alpha")
			if len(ids) != tc.want {
				t.Fatalf("%s rows = %d, want %d (%v)", tc.id, len(ids), tc.want, ids)
			}
			if count := m.workModeItemCount(tc.id); count != len(ids) {
				t.Fatalf("%s count = %d, want row parity %d", tc.id, count, len(ids))
			}
		})
	}

	// Review modes: rows are Alpha-only and section header counts sum to the
	// number of task rows.
	for _, tc := range []struct {
		id   OutlinerModeID
		want int
	}{
		{ModeDailyReview, 1},
		{ModeActivity, 3},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			m.selectOutlinerMode(tc.id)
			ids := taskRowProjectIDs(m)
			assertAllProject(t, ids, "alpha")
			if len(ids) != tc.want {
				t.Fatalf("%s rows = %d, want %d (%v)", tc.id, len(ids), tc.want, ids)
			}
			if sum := sectionCountSum(m); sum != len(ids) {
				t.Fatalf("%s section-count sum = %d, want row parity %d", tc.id, sum, len(ids))
			}
		})
	}
}

// TestOutlinerModeTransitionsAreExclusive asserts the Due Date lens and every
// other mode discriminator stay mutually exclusive: selecting a perspective,
// rotating with h/l, or entering Priority always leaves the Due Date lens, and
// selecting Due Date always clears a prior smart focus and priority mode. Guards
// against the FR13-002 regression where activeOutlinerMode's lens precedence let
// a stale Due Date lens or smart predicate silently override the picker.
func TestOutlinerModeTransitionsAreExclusive(t *testing.T) {
	alpha := buildFilterParityProject("alpha", "Alpha")
	beta := buildFilterParityProject("beta", "Beta")

	runtime := testConfigWithProjects(alpha, beta)
	m := NewOutlinerModel(nil, nil, runtime)
	m.SetSize(120, 40)
	m.loadedProjects = []*models.Project{alpha, beta}

	enterDueDate := func(t *testing.T) {
		t.Helper()
		m.selectOutlinerMode(ModeDueDateLens)
		if m.lens != OutlinerLensDueDate {
			t.Fatalf("setup: expected Due Date lens, got lens=%d", m.lens)
		}
		if got := m.activeOutlinerMode(); got != ModeDueDateLens {
			t.Fatalf("setup: activeOutlinerMode = %q, want Due Date", got)
		}
	}

	t.Run("DueDateToAll", func(t *testing.T) {
		enterDueDate(t)
		m.selectOutlinerMode(ModeAll)
		if m.lens != OutlinerLensTree {
			t.Fatalf("All must restore the tree lens, got lens=%d", m.lens)
		}
		if got := m.activeOutlinerMode(); got != ModeAll {
			t.Fatalf("activeOutlinerMode = %q, want All", got)
		}
	})

	t.Run("DueDateToWorkflowPerspective", func(t *testing.T) {
		enterDueDate(t)
		m.selectOutlinerMode(ModeInbox)
		if m.lens != OutlinerLensTree {
			t.Fatalf("Inbox must restore the tree lens, got lens=%d", m.lens)
		}
		if m.activePerspective != PerspectiveInbox {
			t.Fatalf("perspective = %d, want Inbox", m.activePerspective)
		}
		if got := m.activeOutlinerMode(); got != ModeInbox {
			t.Fatalf("activeOutlinerMode = %q, want Inbox", got)
		}
	})

	t.Run("DueDateThroughRotation", func(t *testing.T) {
		enterDueDate(t)
		// l leaves Due Date and rotates onto a workflow perspective.
		m.crossHandlePerspectiveCycle("l")
		if m.lens != OutlinerLensTree {
			t.Fatalf("l rotation must leave the Due Date lens, got lens=%d", m.lens)
		}
		if got := m.activeOutlinerMode(); got == ModeDueDateLens {
			t.Fatal("l rotation still reports Due Date mode")
		}
		if _, ok := OutlinerModeForPerspective(m.activePerspective); !ok {
			t.Fatalf("l rotation landed on non-perspective state (perspective=%d)", m.activePerspective)
		}
		// h continues rotating perspectives with the tree lens intact.
		m.crossHandlePerspectiveCycle("h")
		if m.lens != OutlinerLensTree {
			t.Fatalf("h rotation must keep the tree lens, got lens=%d", m.lens)
		}
		if got := m.activeOutlinerMode(); got == ModeDueDateLens {
			t.Fatal("h rotation still reports Due Date mode")
		}
	})

	t.Run("SmartFocusToDueDate", func(t *testing.T) {
		m.selectOutlinerMode(ModeDueSoon)
		if m.smartFocus == nil || *m.smartFocus != OutlinerDueSoon {
			t.Fatalf("setup: expected Due Soon smart focus, got %v", m.smartFocus)
		}
		m.selectOutlinerMode(ModeDueDateLens)
		if m.smartFocus != nil {
			t.Fatalf("Due Date must clear the smart focus, got %v", *m.smartFocus)
		}
		if m.lens != OutlinerLensDueDate {
			t.Fatalf("expected Due Date lens, got lens=%d", m.lens)
		}
		if got := m.activeOutlinerMode(); got != ModeDueDateLens {
			t.Fatalf("activeOutlinerMode = %q, want Due Date", got)
		}
	})

	t.Run("DueDateToPriorityAndBack", func(t *testing.T) {
		enterDueDate(t)
		m.selectOutlinerMode(ModePriorityLens)
		if !m.crossPriorityMode {
			t.Fatal("selecting Priority did not enter cross priority mode")
		}
		if m.lens != OutlinerLensTree {
			t.Fatalf("Priority must clear the Due Date lens, got lens=%d", m.lens)
		}
		if got := m.activeOutlinerMode(); got != ModePriorityLens {
			t.Fatalf("activeOutlinerMode = %q, want Priority", got)
		}
		// Back to Due Date clears the priority discriminator and restores the lens.
		m.selectOutlinerMode(ModeDueDateLens)
		if m.crossPriorityMode {
			t.Fatal("Due Date must clear cross priority mode")
		}
		if m.lens != OutlinerLensDueDate {
			t.Fatalf("expected Due Date lens on return, got lens=%d", m.lens)
		}
		if got := m.activeOutlinerMode(); got != ModeDueDateLens {
			t.Fatalf("activeOutlinerMode = %q, want Due Date", got)
		}
	})
}

// buildTodayHierarchyProject builds a single planned root with a small subtree
// so Today expands descendants whose match state can differ from the root:
//
//	rootparent (planned)
//	├── bluewidget           tag: urgent
//	│   └── needlepin        tag: deep
//	└── sibling
//
// Titles and tags are chosen so a subsequence search or a tag/criteria filter
// matches exactly one node, letting a test assert ancestor retention and
// sibling/descendant pruning independently.
func buildTodayHierarchyProject(id string) *models.Project {
	now := time.Now()
	root := &models.Task{ID: id + "-root", Title: "Rootparent", WorkflowState: models.StateReady, UpdatedAt: now}
	childA := &models.Task{ID: id + "-childA", Title: "Bluewidget", WorkflowState: models.StateReady, Tags: []string{"urgent"}, UpdatedAt: now, ParentID: &root.ID}
	grand := &models.Task{ID: id + "-grand", Title: "Needlepin", WorkflowState: models.StateReady, Tags: []string{"deep"}, UpdatedAt: now, ParentID: &childA.ID}
	childB := &models.Task{ID: id + "-childB", Title: "Sibling", WorkflowState: models.StateReady, UpdatedAt: now, ParentID: &root.ID}
	project := &models.Project{ID: id, Name: id, Tasks: []*models.Task{root, childA, grand, childB}}
	project.BuildTaskTree()
	return project
}

// newTodayHierarchyOutliner returns a cross-project outliner in Today mode whose
// scope is a single project with a planned root subtree (buildTodayHierarchyProject).
func newTodayHierarchyOutliner(t *testing.T) *OutlinerModel {
	t.Helper()
	project := buildTodayHierarchyProject("alpha")
	runtime := testConfigWithProjects(project)
	runtime.Planning().SetDailyPlan([]config.DailyPlanItem{
		{TaskID: "alpha-root", ProjectID: "alpha", PlannedDate: "2026-07-23"},
	})
	m := NewOutlinerModel(nil, nil, runtime)
	m.SetSize(120, 40)
	m.loadedProjects = []*models.Project{project}
	m.selectOutlinerMode(ModeToday)
	return m
}

// taskRowIDs returns the task ID of every task row (excluding headers) in order.
func taskRowIDs(m *OutlinerModel) []string {
	var ids []string
	for _, r := range m.rows {
		if r.task != nil && !r.IsSectionHeader {
			ids = append(ids, r.task.ID)
		}
	}
	return ids
}

// TestTodayModeFiltersDescendants guards FR13-003: Today must apply the composite
// task predicate to every planned root AND its expanded descendants, retaining a
// matching descendant with its minimal ancestor context while pruning
// non-matching sibling/descendant subtrees — not the old all-or-nothing behavior
// that keyed only off the depth-zero root.
func TestTodayModeFiltersDescendants(t *testing.T) {
	deepCriteria := &config.SavedFilter{
		Mode: "criteria",
		Criteria: []config.SmartListCriterion{
			{Field: "tags", Operation: "contains", Value: "deep"},
		},
	}

	cases := []struct {
		name  string
		apply func(m *OutlinerModel)
		want  []string
	}{
		{
			// Baseline: no filter expands the whole planned subtree.
			name:  "Unfiltered",
			apply: func(m *OutlinerModel) {},
			want:  []string{"alpha-root", "alpha-childA", "alpha-grand", "alpha-childB"},
		},
		{
			// Search matches only the deep grandchild; the root and middle child
			// are retained purely as ancestor context, the sibling is pruned.
			name:  "SearchMatchesDescendant",
			apply: func(m *OutlinerModel) { m.SetSearchQuery("needlepin") },
			want:  []string{"alpha-root", "alpha-childA", "alpha-grand"},
		},
		{
			// A common tag filter matches only the middle child. Its parent is kept
			// as context; its own non-matching descendant does not leak.
			name: "TagFilterMatchesDescendant",
			apply: func(m *OutlinerModel) {
				m.filter.Tags = map[string]bool{"urgent": true}
			},
			want: []string{"alpha-root", "alpha-childA"},
		},
		{
			// A saved criteria filter matches only the deep grandchild.
			name:  "SavedCriteriaMatchesDescendant",
			apply: func(m *OutlinerModel) { m.activeCriteriaFilter = deepCriteria },
			want:  []string{"alpha-root", "alpha-childA", "alpha-grand"},
		},
		{
			// Inverse: the search matches only the planned root, so every
			// non-matching descendant subtree is pruned.
			name:  "SearchMatchesRootOnly",
			apply: func(m *OutlinerModel) { m.SetSearchQuery("rootparent") },
			want:  []string{"alpha-root"},
		},
		{
			// A predicate that matches nothing drops the whole planned root.
			name:  "NoMatchDropsRoot",
			apply: func(m *OutlinerModel) { m.SetSearchQuery("zzzznomatch") },
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTodayHierarchyOutliner(t)
			tc.apply(m)
			m.rebuildRows()
			got := taskRowIDs(m)
			if len(got) != len(tc.want) {
				t.Fatalf("Today rows = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("Today rows = %v, want %v", got, tc.want)
				}
			}
		})
	}

	// Connector coherence: with the sibling pruned, the surviving middle child
	// must render as the last child (└───), not a stale ├─── implying a following
	// sibling that no longer exists.
	t.Run("PrunedSiblingRecomputesConnectors", func(t *testing.T) {
		m := newTodayHierarchyOutliner(t)
		m.SetSearchQuery("needlepin")
		m.rebuildRows()
		var childRow *outlinerRow
		for i := range m.rows {
			if m.rows[i].task != nil && m.rows[i].task.ID == "alpha-childA" {
				childRow = &m.rows[i]
				break
			}
		}
		if childRow == nil {
			t.Fatal("expected surviving middle child row alpha-childA")
		}
		if !strings.Contains(childRow.prefix, "└───") {
			t.Fatalf("pruned-sibling middle child prefix = %q, want a └─── last-child connector", childRow.prefix)
		}
	})
}

func containsProject(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

func projectIDsOf(projects []*models.Project) []string {
	ids := make([]string, 0, len(projects))
	for _, p := range projects {
		if p != nil {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func assertAllProject(t *testing.T, ids []string, want string) {
	t.Helper()
	if len(ids) == 0 {
		t.Fatalf("no task rows produced, want %s-only rows", want)
	}
	for _, got := range ids {
		if got != want {
			t.Fatalf("row project = %q, want only %q (%v)", got, want, ids)
		}
	}
}

// buildContinuityProject builds a project with many Ready tasks (Backlog- and
// All-eligible) plus a handful of Next tasks (Next-horizon only), enough rows
// that a deep selection forces a non-zero persistent scroll offset. Fresh
// UpdatedAt keeps the undated tasks out of the Daily Review "Old" bucket.
func buildContinuityProject(id, name string, ready, next int) *models.Project {
	now := time.Now()
	var tasks []*models.Task
	for i := 0; i < ready; i++ {
		tasks = append(tasks, &models.Task{
			ID:            fmt.Sprintf("%s-ready-%d", id, i),
			Title:         fmt.Sprintf("%s ready %d", name, i),
			WorkflowState: models.StateReady,
			UpdatedAt:     now,
		})
	}
	for i := 0; i < next; i++ {
		tasks = append(tasks, &models.Task{
			ID:            fmt.Sprintf("%s-next-%d", id, i),
			Title:         fmt.Sprintf("%s next %d", name, i),
			WorkflowState: models.StateNext,
			UpdatedAt:     now,
		})
	}
	project := &models.Project{ID: id, Name: name, Tasks: tasks}
	project.BuildTaskTree()
	return project
}

// newContinuityOutliner returns a cross-project outliner over two continuity
// projects (buildContinuityProject) sized to the given viewport.
func newContinuityOutliner(t *testing.T, w, h int) *OutlinerModel {
	t.Helper()
	alpha := buildContinuityProject("alpha", "Alpha", 12, 4)
	beta := buildContinuityProject("beta", "Beta", 12, 4)
	runtime := testConfigWithProjects(alpha, beta)
	m := NewOutlinerModel(nil, nil, runtime)
	m.SetSize(w, h)
	m.loadedProjects = []*models.Project{alpha, beta}
	return m
}

// rowIndexForTask returns the row index of the project-qualified task, or -1.
func rowIndexForTask(m *OutlinerModel, projectID, taskID string) int {
	for i, r := range m.rows {
		if r.isTaskRow() && r.task != nil && r.task.ID == taskID && r.ProjectID == projectID {
			return i
		}
	}
	return -1
}

// cursorTaskIdentity returns the project-qualified identity of the task under
// the cursor, and false when the cursor is not on a task row.
func cursorTaskIdentity(m *OutlinerModel) (projectID, taskID string, ok bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return "", "", false
	}
	r := m.rows[m.cursor]
	if !r.isTaskRow() || r.task == nil {
		return "", "", false
	}
	return r.ProjectID, r.task.ID, true
}

// TestModeSwitchPreservesCursorIdentityAndScroll guards FR14-002: a mode switch
// must capture the selected project-qualified task identity before rebuilding
// and restore it afterward, keeping an eligible task focused and its scroll
// position intact across a differently shaped rebuild, and falling back
// deterministically to the nearest navigable row when the task is absent.
func TestModeSwitchPreservesCursorIdentityAndScroll(t *testing.T) {
	// A task eligible in both a tree mode (All) and a computed mode (Backlog)
	// keeps focus across the reshape in either direction.
	t.Run("IdentityPreservedTreeToComputed", func(t *testing.T) {
		m := newContinuityOutliner(t, 120, 40)
		m.selectOutlinerMode(ModeAll)
		idx := rowIndexForTask(m, "alpha", "alpha-ready-8")
		if idx < 0 {
			t.Fatalf("setup: ready task missing from All rows: %v", taskRowIDs(m))
		}
		m.cursor = idx

		m.selectOutlinerMode(ModeBacklog)
		if rowIndexForTask(m, "alpha", "alpha-ready-8") < 0 {
			t.Fatalf("setup: ready task missing from Backlog rows: %v", taskRowIDs(m))
		}
		pid, tid, ok := cursorTaskIdentity(m)
		if !ok || pid != "alpha" || tid != "alpha-ready-8" {
			t.Fatalf("cursor after All→Backlog = (%q,%q ok=%t), want alpha/alpha-ready-8", pid, tid, ok)
		}

		// Returning to the tree mode keeps the same task selected.
		m.selectOutlinerMode(ModeAll)
		pid, tid, ok = cursorTaskIdentity(m)
		if !ok || pid != "alpha" || tid != "alpha-ready-8" {
			t.Fatalf("cursor after Backlog→All = (%q,%q ok=%t), want alpha/alpha-ready-8", pid, tid, ok)
		}
	})

	// A deep selection scrolls the viewport; switching modes must not reset the
	// scroll to the top while the selected task remains visible.
	t.Run("ScrollPreservedComputedToTree", func(t *testing.T) {
		m := newContinuityOutliner(t, 80, 14)
		m.selectOutlinerMode(ModeBacklog)

		last := -1
		for i, r := range m.rows {
			if r.isTaskRow() {
				last = i
			}
		}
		if last < 0 {
			t.Fatalf("setup: no task rows in Backlog: %v", taskRowIDs(m))
		}
		m.cursor = last
		// Render once so the persistent viewport scrolls to reveal the deep cursor.
		_ = m.renderCrossProjectContent()
		offset := m.viewScrollOffset
		if offset == 0 {
			t.Fatalf("setup: expected a non-zero scroll offset for a deep cursor over %d rows", len(m.rows))
		}
		pid, tid, ok := cursorTaskIdentity(m)
		if !ok {
			t.Fatal("setup: cursor not on a task row before the switch")
		}

		m.selectOutlinerMode(ModeAll)
		gp, gt, gok := cursorTaskIdentity(m)
		if !gok || gp != pid || gt != tid {
			t.Fatalf("identity after Backlog→All = (%q,%q ok=%t), want (%q,%q)", gp, gt, gok, pid, tid)
		}
		if m.viewScrollOffset != offset {
			t.Fatalf("scroll offset after switch = %d, want preserved %d (no unconditional top reset)",
				m.viewScrollOffset, offset)
		}
	})

	// A StateNext task selected in Next is absent from Backlog (which excludes
	// StateNext). The transition must fall back to a navigable row, and the
	// fallback must be deterministic across identical runs.
	t.Run("AbsentTaskUsesDeterministicFallback", func(t *testing.T) {
		run := func() (cursor, rows int, navigable bool) {
			m := newContinuityOutliner(t, 120, 40)
			m.selectOutlinerMode(ModeNext)
			idx := rowIndexForTask(m, "alpha", "alpha-next-2")
			if idx < 0 {
				t.Fatalf("setup: next task missing from Next rows: %v", taskRowIDs(m))
			}
			m.cursor = idx

			m.selectOutlinerMode(ModeBacklog)
			if rowIndexForTask(m, "alpha", "alpha-next-2") >= 0 {
				t.Fatal("setup: StateNext task unexpectedly present in Backlog")
			}
			nav := m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].isNavigable()
			return m.cursor, len(m.rows), nav
		}

		c1, r1, nav1 := run()
		c2, r2, nav2 := run()
		if !nav1 || !nav2 {
			t.Fatalf("fallback cursor not on a navigable row: nav1=%t nav2=%t", nav1, nav2)
		}
		if c1 != c2 || r1 != r2 {
			t.Fatalf("fallback not deterministic: (cursor=%d rows=%d) vs (cursor=%d rows=%d)", c1, r1, c2, r2)
		}
	})
}

func modeIDs(rows []modePickerRow) []OutlinerModeID {
	out := make([]OutlinerModeID, len(rows))
	for i, r := range rows {
		out[i] = r.mode.ID
	}
	return out
}
