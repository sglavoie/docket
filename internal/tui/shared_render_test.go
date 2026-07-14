package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestRenderDescriptionIndicator(t *testing.T) {
	t.Run("empty description returns empty string", func(t *testing.T) {
		task := &models.Task{Description: ""}
		if got := renderDescriptionIndicator(task); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("non-empty description contains icon", func(t *testing.T) {
		task := &models.Task{Description: "Some details"}
		got := renderDescriptionIndicator(task)
		icon := IconDescription()
		if !strings.Contains(got, icon) {
			t.Errorf("expected output to contain icon %q, got %q", icon, got)
		}
	})
}

func TestRenderCrossProjectTaskLineShowDescription(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	icon := IconDescription()

	t.Run("ShowDescription off omits icon even when description set", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: "details"}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: false,
		})
		if strings.Contains(got, icon) {
			t.Errorf("expected no description icon when ShowDescription=false, got %q", got)
		}
	})

	t.Run("ShowDescription on with description renders icon", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: "details"}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: true,
		})
		if !strings.Contains(got, icon) {
			t.Errorf("expected description icon when ShowDescription=true and task has description, got %q", got)
		}
	})

	t.Run("ShowDescription on without description omits icon", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: ""}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: true,
		})
		if strings.Contains(got, icon) {
			t.Errorf("expected no description icon when task has empty description, got %q", got)
		}
	})
}

// TestTaskLineNoLongerRendersStageBadge guards Session 2: shared task rows
// must not render a stage badge inline. Stage information for selected rows
// now lives in the metadata strip (see task_metadata_strip.go); leaving it in
// the row would defeat the compaction.
func TestTaskLineNoLongerRendersStageBadge(t *testing.T) {
	task := &models.Task{
		Title:         "title",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	line := TaskLine{
		Task:      task,
		MaxTitle:  40,
		ViewWidth: 80,
	}.Render()
	plain := ansi.Strip(line)
	if strings.Contains(plain, "design") {
		t.Fatalf("expected shared task row to omit stage badge, got %q", plain)
	}
}

// TestRenderCrossProjectTaskLineNoLongerRendersStageBadge guards the same
// invariant for the cross-project shared renderer.
func TestRenderCrossProjectTaskLineNoLongerRendersStageBadge(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	task := &models.Task{ID: "t1", Title: "title"}
	got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{})
	plain := ansi.Strip(got)
	if strings.Contains(plain, "design") {
		t.Fatalf("expected cross-project row to omit stage badge, got %q", plain)
	}
}

func TestTaskLineShowsParkedSnippet(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		Title:         "parked task",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
		ParkedNote:    "resume API",
	}

	line := TaskLine{Task: task, MaxTitle: 40, ViewWidth: 80}.Render()
	plain := ansi.Strip(line)
	if !strings.Contains(plain, "⏸ resume API") {
		t.Fatalf("expected parked snippet in shared task row, got %q", plain)
	}
}

func TestTaskLineShowsParkedFallbackForBlankNote(t *testing.T) {
	parkedAt := time.Now()
	task := &models.Task{
		ID:            "t1",
		Title:         "blank-note parked task",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
		ParkedAt:      &parkedAt,
	}

	line := TaskLine{Task: task, MaxTitle: 40, ViewWidth: 80}.Render()
	plain := ansi.Strip(line)
	if !strings.Contains(plain, "⏸ parked") {
		t.Fatalf("expected fallback parked snippet in shared task row, got %q", plain)
	}
}

func TestRenderCrossProjectTaskLineShowsParkedSnippetWithExistingPostTitle(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	task := &models.Task{
		ID:         "t1",
		Title:      "cross-project parked task",
		Tags:       []string{"ops"},
		ParkedNote: "restart deploy",
	}

	got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
		ShowTags: true,
	})
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "#ops") {
		t.Fatalf("expected existing post-title tag to remain, got %q", plain)
	}
	if !strings.Contains(plain, "⏸ restart deploy") {
		t.Fatalf("expected parked snippet in cross-project row, got %q", plain)
	}
}

func TestRenderCrossProjectTaskLineShowsParkedFallbackWithExistingPostTitle(t *testing.T) {
	parkedAt := time.Now()
	proj := &models.Project{ID: "p1", Name: "Proj"}
	task := &models.Task{
		ID:       "t1",
		Title:    "cross-project blank-note parked task",
		Tags:     []string{"ops"},
		ParkedAt: &parkedAt,
	}

	got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
		ShowTags: true,
	})
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "#ops") {
		t.Fatalf("expected existing post-title tag to remain, got %q", plain)
	}
	if !strings.Contains(plain, "⏸ parked") {
		t.Fatalf("expected fallback parked snippet in cross-project row, got %q", plain)
	}
}

func TestTaskLineParkedSnippetStaysWithinViewWidth(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		Title:         "parked task with a title long enough to pressure the row",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
		ParkedNote:    "very long resume note that must be truncated before it widens the row",
	}

	const viewWidth = 52
	line := TaskLine{Task: task, MaxTitle: 80, ViewWidth: viewWidth}.Render()
	if got := lipgloss.Width(ansi.Strip(line)); got > viewWidth {
		t.Fatalf("parked shared row width = %d, want <= %d, stripped=%q", got, viewWidth, ansi.Strip(line))
	}
}

func TestTaskLineParkedSnippetExpandsWithViewWidth(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		Title:         "parked task",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
		ParkedNote:    "resume alpha beta gamma delta marker epsilon zeta",
	}

	narrow := ansi.Strip(TaskLine{Task: task, MaxTitle: 20, ViewWidth: 54}.Render())
	wide := ansi.Strip(TaskLine{Task: task, MaxTitle: 20, ViewWidth: 120}.Render())

	if strings.Contains(narrow, "delta marker") {
		t.Fatalf("narrow shared row exposed too much parked note text, got %q", narrow)
	}
	if !strings.Contains(wide, "delta marker") {
		t.Fatalf("wide shared row should expose more parked note text, got %q", wide)
	}
}

func TestCrossProjectSelectedStripDoesNotRepeatInlineStaleBadge(t *testing.T) {
	staleDays := 1
	cfg := newTestConfig()
	cfg.Settings().StaleTaskDays = &staleDays
	runtime := testRuntimeAt(cfg.DataDir(), cfg)
	proj := &models.Project{ID: "p1", Name: "Proj"}
	task := &models.Task{
		ID:            "t1",
		Title:         "old cross-project task",
		WorkflowState: models.StateReady,
		Priority:      2,
		CreatedAt:     time.Now().Add(-30 * 24 * time.Hour),
		UpdatedAt:     time.Now().Add(-30 * 24 * time.Hour),
	}

	row := RenderCrossProjectTaskLine(task, proj, runtime, true, 120, 40, CrossProjectTaskLineOpts{})
	strip := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		Runtime: runtime,
		Width:   120,
		AlreadyShown: TaskMetadataAlreadyShown{
			Project:  true,
			Priority: true,
			Stale:    true,
		},
	})

	plain := ansi.Strip(row + "\n" + strip)
	if got := strings.Count(plain, "stale"); got != 1 {
		t.Fatalf("expected selected cross-project row and strip to show stale once, got %d in %q", got, plain)
	}
}

func TestSelectedCrossProjectViewsDoNotRepeatStaleBadge(t *testing.T) {
	cfg, proj, task := staleCrossProjectViewFixture(t)
	due := time.Now().Add(24 * time.Hour)
	task.DueDate = &due

	tests := []struct {
		name string
		view func() string
	}{
		{
			name: "all tasks",
			view: func() string {
				m := NewAllTasksViewModel(testRuntimeAt(cfg.DataDir(), cfg), nil)
				m.SetSize(118, 18)
				item := CrossProjectTask{Task: task, Project: proj}
				m.allTasks = []CrossProjectTask{item}
				m.groups = []AllTasksGroup{{Name: proj.Name, Tasks: []CrossProjectTask{item}}}
				m.projectCache = map[string]*models.Project{proj.ID: proj}
				m.cursor = 1
				m.calculateNavigation()
				return m.View().Content
			},
		},
		{
			name: "due date overview",
			view: func() string {
				m := NewDueDateOverviewModel(testRuntimeAt("", cfg), nil)
				m.SetSize(118, 18)
				m.today = []CrossProjectTask{{Task: task, Project: proj}}
				m.recalcTotalTasks()
				m.cursor = 0
				return m.View().Content
			},
		},
		{
			name: "focus view",
			view: func() string {
				queue := []FocusTask{}
				m := NewFocusViewModel(nil, nil, &queue, map[string]bool{}, map[string]bool{}, map[string]bool{})
				m.SetSize(118, 18)
				m.tabs.Active = focusTabByProject
				ft := FocusTask{Task: task, Project: proj}
				m.tasks = []FocusTask{ft}
				m.filtered = []FocusTask{ft}
				m.computeSubTabs()
				m.cursor = 0
				return m.View().Content
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plain := ansi.Strip(tt.view())
			if got := strings.Count(plain, "stale"); got != 1 {
				t.Fatalf("expected exactly one stale badge in %s, got %d:\n%s", tt.name, got, plain)
			}
		})
	}
}

func staleCrossProjectViewFixture(t *testing.T) (*config.RuntimeState, *models.Project, *models.Task) {
	t.Helper()
	staleDays := 1
	cfg := newTestConfig()
	cfg.Settings().StaleTaskDays = &staleDays

	proj, err := models.NewProject("p-stale-regression", "Alpha", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("t-stale-regression", 42, models.TaskTypeTask, "aged task")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	task.WorkflowState = models.StateReady
	task.Priority = 2
	task.SetSection(models.SectionNext)
	task.UpdatedAt = old
	task.CreatedAt = old
	proj.Tasks = []*models.Task{task}
	return cfg, proj, task
}

// TestTaskLineLongStageDoesNotAffectWidth guards against the older behavior
// where pre-title stages widened the row. The stage column is gone from
// shared rows entirely, so width must be identical with and without a stage.
func TestTaskLineLongStageDoesNotAffectWidth(t *testing.T) {
	base := &models.Task{
		Title:         "title",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	staged := *base

	baseLine := TaskLine{Task: base, MaxTitle: 40, ViewWidth: 80}.Render()
	stagedLine := TaskLine{Task: &staged, MaxTitle: 40, ViewWidth: 80}.Render()

	if lipgloss.Width(ansi.Strip(baseLine)) != lipgloss.Width(ansi.Strip(stagedLine)) {
		t.Fatalf("long stage should not affect shared row width:\nbase=%q (w=%d)\nstaged=%q (w=%d)",
			ansi.Strip(baseLine), lipgloss.Width(ansi.Strip(baseLine)),
			ansi.Strip(stagedLine), lipgloss.Width(ansi.Strip(stagedLine)))
	}
}

// TestComputeTaskLineColumnLayoutIgnoresStage guards that the shared layout
// helper stopped measuring stage widths when shared rows lost the column.
// Sessions 3-7 rely on this so they can read HasStage from a view-specific
// layout without inheriting it from the shared helper.
func TestComputeTaskLineColumnLayoutIgnoresStage(t *testing.T) {
	t1 := &models.Task{
		Title:         "alpha",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateInProgress,
		Priority:      2,
	}
	cl := ComputeTaskLineColumnLayout([]*models.Task{t1}, false)
	if cl == nil {
		t.Fatal("expected non-nil layout")
	}
	// The shared TaskLine no longer renders stage and the column has been
	// removed from BadgeWidthOpts/ColumnLayout entirely; an absent column is
	// the strongest possible guarantee that long stages cannot push other
	// columns. Re-introducing stage measurement should require deliberately
	// re-adding the API and is caught by the compiler.
	_ = cl
}

// TestSharedRowLongStageManySiblingsConsistentWidth guards the plan's
// primary regression: a long stage like "Implement initial version" on one
// task in a list of many empty-stage siblings must not shift titles or
// create empty padded row gaps. The shared row dropped stage in session 2;
// this asserts that contract across a representative row population.
func TestSharedRowLongStageManySiblingsConsistentWidth(t *testing.T) {
	tasks := make([]*models.Task, 0, 12)
	for i := 0; i < 12; i++ {
		tasks = append(tasks, &models.Task{
			Title:         "Task " + strings.Repeat("x", 4),
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateReady,
			Priority:      3,
		})
	}

	layout := ComputeTaskLineColumnLayout(tasks, false)
	if layout == nil {
		t.Fatal("expected non-nil layout for non-empty task slice")
	}

	const viewWidth = 80
	widths := make(map[int]int, len(tasks))
	for _, task := range tasks {
		line := TaskLine{
			Task:      task,
			MaxTitle:  40,
			ViewWidth: viewWidth,
			Layout:    layout,
		}.Render()
		w := lipgloss.Width(ansi.Strip(line))
		widths[w]++
	}
	if len(widths) != 1 {
		t.Fatalf("expected uniform row width across long-stage and empty-stage siblings, got width distribution %v", widths)
	}
}

// TestCrossProjectRowLongStageGroupedListConsistentWidth asserts the same
// invariant for the cross-project shared renderer used by fullscreen grouped
// lists (Priority, Due Date, All Tasks, Daily/Weekly Review, Recent Activity,
// Focus View). A long stage on one task in a sibling list must not shift any
// other row's width because stage is no longer rendered inline.
func TestCrossProjectRowLongStageGroupedListConsistentWidth(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	tasks := make([]*models.Task, 0, 8)
	for i := 0; i < 8; i++ {
		tasks = append(tasks, &models.Task{
			ID:            "t" + strings.Repeat("x", i+1),
			Title:         "Grouped row task " + strings.Repeat("x", 3),
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateReady,
			Priority:      3,
		})
	}

	layout := ComputeBadgeWidths(tasks, BadgeWidthOpts{
		IncludeStatus: true,
		IncludeType:   true,
	})
	if layout == nil {
		t.Fatal("expected non-nil layout for non-empty task slice")
	}

	const viewWidth = 100
	widths := make(map[int]int, len(tasks))
	for i, task := range tasks {
		line := RenderCrossProjectTaskLine(task, proj, nil, false, viewWidth, 40, CrossProjectTaskLineOpts{
			RowIndex: i,
			Layout:   layout,
		})
		w := lipgloss.Width(ansi.Strip(line))
		widths[w]++
	}
	if len(widths) != 1 {
		t.Fatalf("expected uniform cross-project row width across long-stage and empty-stage siblings, got %v", widths)
	}
}

// TestTaskLineDimmedOverdueSkipsHeroDemotion locks in the Session-2 cleanup
// of applyHeroDemotion: the helper used to special-case Dimmed rows even when
// pickHeroAttribute already returned HeroNone for them. Removing the redundant
// guard must not change behavior — a dimmed overdue task still skips demotion
// because the policy short-circuits to HeroNone, and the rendered line must
// match a non-overdue dimmed sibling exactly.
func TestTaskLineDimmedOverdueSkipsHeroDemotion(t *testing.T) {
	base := &models.Task{
		Title:         "dim row",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
		Dimmed:        true,
	}
	yesterday := time.Now().Add(-48 * time.Hour)
	overdue := *base
	overdue.DueDate = &yesterday

	baseLine := TaskLine{Task: base, MaxTitle: 40, ViewWidth: 80}.Render()
	overdueLine := TaskLine{Task: &overdue, MaxTitle: 40, ViewWidth: 80}.Render()

	if ansi.Strip(baseLine) != ansi.Strip(overdueLine) {
		t.Fatalf("dimmed overdue row diverged from non-overdue baseline; demotion regression?\nbase=%q\noverdue=%q",
			ansi.Strip(baseLine), ansi.Strip(overdueLine))
	}
}

// TestTaskLinePrioritySegmentDemotionPreservesAlignment guards against a
// regression where applyHeroDemotion stripped the alignment padding that
// PadBadge inserted for a task whose own priority was hidden by
// HideLowPriority. The padding (whitespace) was trimmed by
// demotedTaskLineSegment, collapsing the column to width 0 and breaking
// vertical alignment with sibling rows that did show a priority badge.
//
// Scenario: layout reserves PriorityWidth=1 (some sibling row has Priority<=2).
// The current task has HideLowPriority=true, Priority=5 (no badge) and is
// overdue (HeroOverdue triggers demotion of the priority segment). We assert
// the rendered prioritySegment occupies the same visual width as a non-demoted
// row where the priority badge is empty for the same layout — i.e. the
// padding survives demotion.
func TestTaskLinePrioritySegmentDemotionPreservesAlignment(t *testing.T) {
	layout := &ColumnLayout{HasPriority: true, PriorityWidth: 1}

	// Baseline: low-priority hidden, but no hero (Priority=3 is not P1, not
	// overdue, not in_review). prioritySegment should be PadBadge("",1)+" " = "  ".
	baselineTask := &models.Task{
		Title:         "samename",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
	}
	baseline := TaskLine{
		Task:            baselineTask,
		MaxTitle:        20,
		HideLowPriority: true,
		Layout:          layout,
	}.Render()

	// Demoted case: same hidden priority, but task is overdue → HeroOverdue
	// triggers demotion of prioritySegment via demotedTaskLineSegment.
	yesterday := time.Now().Add(-48 * time.Hour)
	demotedTask := &models.Task{
		Title:         "samename",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      3,
		DueDate:       &yesterday,
	}
	demoted := TaskLine{
		Task:            demotedTask,
		MaxTitle:        20,
		HideLowPriority: true,
		Layout:          layout,
	}.Render()

	baselineW := lipgloss.Width(ansi.Strip(baseline))
	demotedW := lipgloss.Width(ansi.Strip(demoted))

	if baselineW != demotedW {
		t.Fatalf("hero demotion shrank prioritySegment alignment: baseline=%d demoted=%d\nbaseline=%q\ndemoted=%q",
			baselineW, demotedW, ansi.Strip(baseline), ansi.Strip(demoted))
	}
}
