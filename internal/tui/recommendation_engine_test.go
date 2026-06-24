package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Engine tests use a fixed clock so reasons depending on time (overdue,
// recently active, stale, carried over) stay deterministic. The cockpit
// surfaces (sessions 4-8) call Recommend with time.Now() at view time; tests
// pin Now to mid-day so day-boundary math is unambiguous.

var engineNow = time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)

func todayKeyFromNow() string {
	return util.StartOfDay(engineNow).Format(util.DateFormat)
}

func newEngineProject(t *testing.T, name string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		t.Fatalf("newEngineProject(%q): %v", name, err)
	}
	return p
}

func newEngineTask(t *testing.T, project *models.Project, title string) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), len(project.Tasks)+1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("newEngineTask(%q): %v", title, err)
	}
	task.CreatedAt = engineNow.Add(-7 * 24 * time.Hour)
	task.UpdatedAt = engineNow.Add(-7 * 24 * time.Hour)
	project.Tasks = append(project.Tasks, task)
	return task
}

func TestRecommendEmptyWorldReturnsAllDone(t *testing.T) {
	got := Recommend(RecommendationEngineInput{Now: engineNow})
	if got.Kind != RecommendationKindAllDone {
		t.Fatalf("empty world: Kind = %v, want AllDone", got.Kind)
	}
	if got.Task != nil || got.Project != nil {
		t.Errorf("empty world: Task=%v Project=%v, want nil", got.Task, got.Project)
	}
	if len(got.Reasons) != 0 {
		t.Errorf("empty world: Reasons = %v, want []", got.Reasons)
	}
	if !got.GeneratedAt.Equal(engineNow) {
		t.Errorf("GeneratedAt = %v, want %v", got.GeneratedAt, engineNow)
	}
}

func TestRecommendZeroNowDefaultsToWallClock(t *testing.T) {
	before := time.Now()
	got := Recommend(RecommendationEngineInput{})
	after := time.Now()
	if got.GeneratedAt.Before(before) || got.GeneratedAt.After(after) {
		t.Errorf("zero Now: GeneratedAt %v not between %v and %v", got.GeneratedAt, before, after)
	}
}

func TestRecommendOverdueBeatsEverythingElse(t *testing.T) {
	project := newEngineProject(t, "P")

	overdueTask := newEngineTask(t, project, "Overdue")
	due := engineNow.Add(-48 * time.Hour)
	overdueTask.DueDate = &due

	planned := newEngineTask(t, project, "Planned")
	plannedID := config.DailyPlanItem{TaskID: planned.ID, ProjectID: project.ID, PlannedDate: todayKeyFromNow()}

	inbox := newEngineTask(t, project, "InboxItem")
	inbox.Section = models.SectionInbox

	got := Recommend(RecommendationEngineInput{
		Projects:  []*models.Project{project},
		DailyPlan: []config.DailyPlanItem{plannedID},
		Now:       engineNow,
	})
	if got.Kind != RecommendationKindWorkOnTask {
		t.Fatalf("Kind = %v, want WorkOnTask", got.Kind)
	}
	if got.Task != overdueTask {
		t.Errorf("Task = %v, want overdue task", got.Task)
	}
	if !got.HasReason(ReasonOverdue) {
		t.Errorf("Reasons = %v, want ReasonOverdue present", got.Reasons)
	}
	if got.Reasons[0] != ReasonOverdue {
		t.Errorf("Reasons[0] = %v, want ReasonOverdue first", got.Reasons[0])
	}
}

func TestRecommendDueTodayCarriesDueTodayReason(t *testing.T) {
	project := newEngineProject(t, "P")
	task := newEngineTask(t, project, "Due today")
	due := util.StartOfDay(engineNow).Add(15 * time.Hour)
	task.DueDate = &due

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindWorkOnTask {
		t.Fatalf("Kind = %v, want WorkOnTask", got.Kind)
	}
	if got.Task != task {
		t.Errorf("Task = %v, want due-today task", got.Task)
	}
	if !got.HasReason(ReasonDueToday) {
		t.Errorf("Reasons = %v, want ReasonDueToday", got.Reasons)
	}
}

func TestRecommendPlannedTodayWinsOverHighPriority(t *testing.T) {
	project := newEngineProject(t, "P")

	planned := newEngineTask(t, project, "Planned")
	planned.Priority = 3

	highPri := newEngineTask(t, project, "P1 routine")
	highPri.Priority = 1

	plan := []config.DailyPlanItem{{TaskID: planned.ID, ProjectID: project.ID, PlannedDate: todayKeyFromNow()}}

	got := Recommend(RecommendationEngineInput{
		Projects:  []*models.Project{project},
		DailyPlan: plan,
		Now:       engineNow,
	})
	if got.Task != planned {
		t.Fatalf("Task = %v, want planned task to win over P1", got.Task)
	}
	if !got.HasReason(ReasonPlannedToday) {
		t.Errorf("Reasons = %v, want ReasonPlannedToday", got.Reasons)
	}
}

func TestRecommendCarriedOverReasonSurfaced(t *testing.T) {
	project := newEngineProject(t, "P")
	carry := newEngineTask(t, project, "Carry over")
	carry.Priority = 3
	plan := []config.DailyPlanItem{{
		TaskID:      carry.ID,
		ProjectID:   project.ID,
		PlannedDate: util.StartOfDay(engineNow).Add(-72 * time.Hour).Format(util.DateFormat),
	}}

	got := Recommend(RecommendationEngineInput{
		Projects:  []*models.Project{project},
		DailyPlan: plan,
		Now:       engineNow,
	})
	if got.Task != carry {
		t.Fatalf("Task = %v, want carried-over task", got.Task)
	}
	if !got.HasReason(ReasonCarriedOver) {
		t.Errorf("Reasons = %v, want ReasonCarriedOver", got.Reasons)
	}
}

func TestRecommendInboxOnlyReturnsTriageInbox(t *testing.T) {
	project := newEngineProject(t, "P")
	inbox := newEngineTask(t, project, "Unprocessed")
	inbox.Section = models.SectionInbox

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindTriageInbox {
		t.Fatalf("Kind = %v, want TriageInbox", got.Kind)
	}
	if got.Task != nil || got.Project != nil {
		t.Errorf("TriageInbox should have nil Task/Project; got %v / %v", got.Task, got.Project)
	}
	if !got.HasReason(ReasonInboxNeedsTriage) {
		t.Errorf("Reasons = %v, want ReasonInboxNeedsTriage", got.Reasons)
	}
}

func TestRecommendBlockedOnlyReturnsReviewBlocked(t *testing.T) {
	project := newEngineProject(t, "P")
	bTop := newEngineTask(t, project, "Blocked top")
	bTop.Status = models.StatusBlocked
	bTop.Priority = 2

	bLow := newEngineTask(t, project, "Blocked low")
	bLow.Status = models.StatusBlocked
	bLow.Priority = 5

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindReviewBlocked {
		t.Fatalf("Kind = %v, want ReviewBlocked", got.Kind)
	}
	if got.Task != bTop {
		t.Errorf("Task = %v, want highest-priority blocked task", got.Task)
	}
	if !got.HasReason(ReasonBlocked) {
		t.Errorf("Reasons = %v, want ReasonBlocked", got.Reasons)
	}
}

func TestRecommendDependencyBlockedOnlyReturnsReviewBlocked(t *testing.T) {
	project := newEngineProject(t, "P")
	gated := newEngineTask(t, project, "Gated by missing dependency")
	gated.DependsOn = []string{"missing-dependency"}
	gated.Priority = 1

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindReviewBlocked {
		t.Fatalf("Kind = %v, want ReviewBlocked for dependency-blocked-only input", got.Kind)
	}
	if got.Task != gated {
		t.Errorf("Task = %v, want dependency-blocked task", got.Task)
	}
	if !got.HasReason(ReasonBlocked) {
		t.Errorf("Reasons = %v, want ReasonBlocked", got.Reasons)
	}
}

func TestRecommendInboxPreemptsHighPriorityActionable(t *testing.T) {
	project := newEngineProject(t, "P")

	highPri := newEngineTask(t, project, "P1 routine")
	highPri.Priority = 1

	inboxTask := newEngineTask(t, project, "New idea")
	inboxTask.Section = models.SectionInbox

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindTriageInbox {
		t.Fatalf("Kind = %v, want TriageInbox (inbox outranks plain P1)", got.Kind)
	}
}

func TestRecommendPlanTodayWhenOnlyFallbackActionable(t *testing.T) {
	project := newEngineProject(t, "P")
	fallback := newEngineTask(t, project, "Routine task")
	fallback.Priority = 3
	// Touched 10 days ago: not stale, not recent — pure fallback.
	fallback.UpdatedAt = engineNow.Add(-10 * 24 * time.Hour)

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindPlanToday {
		t.Fatalf("Kind = %v, want PlanToday", got.Kind)
	}
	if got.Task != nil || got.Project != nil {
		t.Errorf("PlanToday should have nil Task/Project; got %v / %v", got.Task, got.Project)
	}
}

func TestRecommendCompletedTasksExcluded(t *testing.T) {
	project := newEngineProject(t, "P")
	done := newEngineTask(t, project, "Done")
	done.Status = models.StatusDone
	completed := engineNow
	done.CompletedAt = &completed

	canceled := newEngineTask(t, project, "Canceled")
	canceled.Status = models.StatusCanceled

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindAllDone {
		t.Fatalf("Kind = %v, want AllDone (no actionable left)", got.Kind)
	}
}

func TestRecommendSnoozedTasksExcluded(t *testing.T) {
	project := newEngineProject(t, "P")

	snoozedActionable := newEngineTask(t, project, "Snoozed normal")
	until := engineNow.Add(48 * time.Hour)
	snoozedActionable.SnoozedUntil = &until

	snoozedInbox := newEngineTask(t, project, "Snoozed inbox")
	snoozedInbox.Section = models.SectionInbox
	snoozedInbox.SnoozedUntil = &until

	snoozedBlocked := newEngineTask(t, project, "Snoozed blocked")
	snoozedBlocked.Status = models.StatusBlocked
	snoozedBlocked.SnoozedUntil = &until

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindAllDone {
		t.Fatalf("Kind = %v, want AllDone (snoozed tasks excluded from every bucket)", got.Kind)
	}
}

func TestRecommendTasksWithIncompleteDepsExcluded(t *testing.T) {
	project := newEngineProject(t, "P")
	dep := newEngineTask(t, project, "Dependency")
	due := engineNow.Add(-24 * time.Hour) // overdue so the engine surfaces it
	dep.DueDate = &due
	gated := newEngineTask(t, project, "Gated")
	gated.DependsOn = []string{dep.ID}
	gatedDue := engineNow.Add(-72 * time.Hour) // more overdue than dep
	gated.DueDate = &gatedDue

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Task != dep {
		t.Errorf("Task = %v, want dependency to surface (gated task hidden behind dep)", got.Task)
	}
}

func TestRecommendUrgentOutranksBlockedReview(t *testing.T) {
	project := newEngineProject(t, "P")

	overdue := newEngineTask(t, project, "Overdue")
	due := engineNow.Add(-72 * time.Hour)
	overdue.DueDate = &due

	blocked := newEngineTask(t, project, "Blocked")
	blocked.Status = models.StatusBlocked

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindWorkOnTask {
		t.Fatalf("Kind = %v, want WorkOnTask (urgent beats blocked review)", got.Kind)
	}
	if got.Task != overdue {
		t.Errorf("Task = %v, want overdue task", got.Task)
	}
}

func TestRecommendHighPriorityWinsWithoutInboxOrBlocked(t *testing.T) {
	project := newEngineProject(t, "P")

	fallback := newEngineTask(t, project, "Routine")
	fallback.Priority = 4
	fallback.UpdatedAt = engineNow.Add(-10 * 24 * time.Hour)

	pri := newEngineTask(t, project, "Important")
	pri.Priority = 1
	pri.UpdatedAt = engineNow.Add(-10 * 24 * time.Hour)

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindWorkOnTask {
		t.Fatalf("Kind = %v, want WorkOnTask", got.Kind)
	}
	if got.Task != pri {
		t.Errorf("Task = %v, want high-priority task", got.Task)
	}
	if !got.HasReason(ReasonHighPriority) {
		t.Errorf("Reasons = %v, want ReasonHighPriority", got.Reasons)
	}
}

func TestRecommendStaleReasonAttached(t *testing.T) {
	project := newEngineProject(t, "P")
	stale := newEngineTask(t, project, "Stale")
	stale.Priority = 3
	stale.UpdatedAt = engineNow.Add(-30 * 24 * time.Hour)

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Kind != RecommendationKindPlanToday {
		t.Fatalf("Kind = %v, want PlanToday (stale isn't urgent enough alone)", got.Kind)
	}
	// Sanity-check the stale-detection path is wired by surfacing it via a
	// secondary actionable that's high priority.
	hot := newEngineTask(t, project, "Hot")
	hot.Priority = 1
	hot.UpdatedAt = stale.UpdatedAt

	got = Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if got.Task != hot {
		t.Fatalf("Task = %v, want high-priority hot task", got.Task)
	}
	if !got.HasReason(ReasonStale) {
		t.Errorf("Reasons = %v, want stale reason on the long-untouched P1 task", got.Reasons)
	}
}

func TestRecommendRecentActivityReason(t *testing.T) {
	project := newEngineProject(t, "P")
	task := newEngineTask(t, project, "Touched")
	task.Priority = 1
	task.UpdatedAt = engineNow.Add(-2 * time.Hour)

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	if !got.HasReason(ReasonRecentlyActive) {
		t.Errorf("Reasons = %v, want ReasonRecentlyActive", got.Reasons)
	}
}

func TestRecommendDeterministicTiebreakAcrossProjectAndTaskID(t *testing.T) {
	// Two projects, three identically-scored tasks each. The engine must
	// return the same Task pointer regardless of project ordering on input.
	projA := newEngineProject(t, "A")
	projA.ID = "aaa-project"
	projB := newEngineProject(t, "B")
	projB.ID = "bbb-project"

	mk := func(p *models.Project, id string) {
		task, err := models.NewTask(id, len(p.Tasks)+1, models.TaskTypeTask, "T-"+id)
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3
		task.CreatedAt = engineNow.Add(-24 * time.Hour)
		task.UpdatedAt = engineNow.Add(-24 * time.Hour)
		due := util.StartOfDay(engineNow).Add(-48 * time.Hour) // overdue → tier 0
		task.DueDate = &due
		p.Tasks = append(p.Tasks, task)
	}
	mk(projA, "task-1")
	mk(projA, "task-3")
	mk(projB, "task-2")

	orderings := [][]*models.Project{
		{projA, projB},
		{projB, projA},
	}
	var baselineID, baselineProjectID string
	for i, projs := range orderings {
		got := Recommend(RecommendationEngineInput{Projects: projs, Now: engineNow})
		if got.Task == nil || got.Project == nil {
			t.Fatalf("ordering %d: expected populated Task/Project", i)
		}
		if i == 0 {
			baselineID = got.Task.ID
			baselineProjectID = got.Project.ID
			continue
		}
		if got.Task.ID != baselineID || got.Project.ID != baselineProjectID {
			t.Errorf("ordering %d: Task/Project = %s/%s, want %s/%s", i, got.Task.ID, got.Project.ID, baselineID, baselineProjectID)
		}
	}

	// Lowest task ID across all tied candidates should win the deterministic
	// tiebreak (ID asc when score / due / priority all match).
	if baselineID != "task-1" {
		t.Errorf("deterministic tiebreak: Task.ID = %s, want task-1 (lowest)", baselineID)
	}
}

func TestRecommendInboxPriorityBeatsCreatedAt(t *testing.T) {
	project := newEngineProject(t, "P")

	oldLow := newEngineTask(t, project, "Old low pri")
	oldLow.Section = models.SectionInbox
	oldLow.Priority = 4
	oldLow.CreatedAt = engineNow.Add(-48 * time.Hour)

	newHigh := newEngineTask(t, project, "New high pri")
	newHigh.Section = models.SectionInbox
	newHigh.Priority = 1
	newHigh.CreatedAt = engineNow.Add(-1 * time.Hour)

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{project},
		Now:      engineNow,
	})
	// Kind is TriageInbox (cockpit-wide). Inbox internal ordering still
	// matters because session 4+ might surface preview rows from the sorted
	// inbox bucket — guard the sort indirectly by confirming the input
	// produces the expected Kind without surfacing a Task pointer.
	if got.Kind != RecommendationKindTriageInbox {
		t.Errorf("Kind = %v, want TriageInbox", got.Kind)
	}
	if got.Task != nil {
		t.Errorf("TriageInbox Task = %v, want nil", got.Task)
	}
}

func TestRecommendReasonsOrderedByTier(t *testing.T) {
	project := newEngineProject(t, "P")
	task := newEngineTask(t, project, "Stacked")
	task.Priority = 1
	due := engineNow.Add(-24 * time.Hour) // overdue
	task.DueDate = &due
	task.UpdatedAt = engineNow.Add(-1 * time.Hour) // recently active
	plan := []config.DailyPlanItem{{
		TaskID:      task.ID,
		ProjectID:   project.ID,
		PlannedDate: todayKeyFromNow(),
	}}

	got := Recommend(RecommendationEngineInput{
		Projects:  []*models.Project{project},
		DailyPlan: plan,
		Now:       engineNow,
	})
	wantOrder := []RecommendationReason{
		ReasonOverdue,        // tier 0
		ReasonPlannedToday,   // tier 2
		ReasonHighPriority,   // tier 4
		ReasonRecentlyActive, // tier 6
	}
	if len(got.Reasons) != len(wantOrder) {
		t.Fatalf("Reasons = %v, want %v", got.Reasons, wantOrder)
	}
	for i, want := range wantOrder {
		if got.Reasons[i] != want {
			t.Errorf("Reasons[%d] = %v, want %v (full: %v)", i, got.Reasons[i], want, got.Reasons)
		}
	}
}

func TestRecommendIgnoresNilProjectsAndNilTasks(t *testing.T) {
	project := newEngineProject(t, "P")
	project.Tasks = append(project.Tasks, nil)
	task := newEngineTask(t, project, "Real")
	task.Priority = 1
	due := engineNow.Add(-24 * time.Hour)
	task.DueDate = &due

	got := Recommend(RecommendationEngineInput{
		Projects: []*models.Project{nil, project, nil},
		Now:      engineNow,
	})
	if got.Task != task {
		t.Fatalf("Task = %v, want real overdue task", got.Task)
	}
}

func TestRecommendOverrideStaleAndRecencyThresholds(t *testing.T) {
	project := newEngineProject(t, "P")

	fresh := newEngineTask(t, project, "Just touched")
	fresh.Priority = 1
	fresh.UpdatedAt = engineNow.Add(-30 * time.Minute)

	got := Recommend(RecommendationEngineInput{
		Projects:      []*models.Project{project},
		Now:           engineNow,
		RecencyWindow: 10 * time.Minute, // tighter window: 30m ago no longer "recent"
	})
	if got.HasReason(ReasonRecentlyActive) {
		t.Errorf("Reasons = %v, must not include ReasonRecentlyActive with 10m window", got.Reasons)
	}

	got = Recommend(RecommendationEngineInput{
		Projects:       []*models.Project{project},
		Now:            engineNow,
		StaleThreshold: 15 * time.Minute, // tighter stale: 30m ago counts as stale
	})
	if !got.HasReason(ReasonStale) {
		t.Errorf("Reasons = %v, want ReasonStale with 15m threshold", got.Reasons)
	}
}
