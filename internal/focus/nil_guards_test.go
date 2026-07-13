package focus

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TestCollectActionableTasksSkipsNilProject guards against nil projects or
// projects containing nil tasks. Loosely-assembled slices fed in from CLI
// layers used to panic on the nil-deref.
func TestCollectActionableTasksSkipsNilProject(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil project/task: %v", r)
		}
	}()

	good, err := models.NewProject(uuid.New().String(), "Good", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	good.Tasks = []*models.Task{task, nil}

	out := CollectActionableTasks([]*models.Project{nil, good, nil})
	if len(out) != 1 {
		t.Fatalf("expected 1 actionable task, got %d", len(out))
	}
	if out[0].Task != task {
		t.Errorf("expected the one good task, got %+v", out[0])
	}
}

// TestCollectActionableTasksSkipsNilProjectTasksDuringDependencyChecks guards
// against nil entries in Project.Tasks while dependency checks are resolving
// blockers. Focus accepts loosely-assembled project slices and should skip nil
// task entries instead of panicking inside model lookup helpers.
func TestCollectActionableTasksSkipsNilProjectTasksDuringDependencyChecks(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil project task during dependency check: %v", r)
		}
	}()

	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Blocker")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Blocked")
	if err != nil {
		t.Fatal(err)
	}
	blocked.DependsOn = []string{blocker.ID}
	actionable, err := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "Actionable")
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = []*models.Task{nil, blocker, blocked, actionable}

	out := CollectActionableTasks([]*models.Project{project})
	if len(out) != 2 {
		t.Fatalf("expected blocker and actionable tasks, got %d: %+v", len(out), out)
	}
	for _, ft := range out {
		if ft.Task == blocked {
			t.Fatal("blocked task with incomplete dependency should not be actionable")
		}
	}
}

// TestRankCrossProjectTasksSkipsNilEntries guards against nil FocusTask
// members (nil Task or nil Project) — these previously crashed the score
// computation and the balance-penalty loop.
func TestRankCrossProjectTasksSkipsNilEntries(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	good, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}

	tasks := []FocusTask{
		{Task: nil, Project: project},
		{Task: good, Project: project},
		{Task: good, Project: nil},
	}

	ranked := RankCrossProjectTasks(tasks, nil, nil)
	if len(ranked) != 1 {
		t.Fatalf("expected 1 ranked task (others nil-skipped), got %d", len(ranked))
	}
	if ranked[0].Task != good {
		t.Errorf("expected the good task")
	}
}

// TestRankCrossProjectTasksSkipsNilProjectTasksDuringDependentScoring guards
// countIncompleteDependents against nil Project.Tasks entries. Project helpers
// assume task slices are clean, but focus may receive loosely assembled data.
func TestRankCrossProjectTasksSkipsNilProjectTasksDuringDependentScoring(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on nil project task during dependent scoring: %v", r)
		}
	}()

	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Blocker")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Dependent")
	if err != nil {
		t.Fatal(err)
	}
	dependent.DependsOn = []string{blocker.ID}
	project.Tasks = []*models.Task{nil, blocker, dependent}

	ranked := RankCrossProjectTasks([]FocusTask{{Task: blocker, Project: project}}, nil, nil)
	if len(ranked) != 1 {
		t.Fatalf("expected one ranked task, got %d", len(ranked))
	}
	if !containsSubstring(ranked[0].Reason, "nblocks 1 task") {
		t.Fatalf("expected dependent to be counted in reason, got %q", ranked[0].Reason)
	}
}

// TestComputeCrossProjectScoreNilInputs guards against nil task or nil
// project. The function previously dereferenced both unconditionally.
func TestComputeCrossProjectScoreNilInputs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	_ = computeCrossProjectScore(nil, project, nil, time.Now())
	_ = computeCrossProjectScore(task, nil, nil, time.Now())
	_ = getSuggestionReason(nil, project, nil, time.Now())
	_ = getSuggestionReason(task, nil, nil, time.Now())
}

// TestComputeScoreFutureCreatedNotNegative checks the age bonus does not
// produce a negative score when the task's CreatedAt is in the future
// (clock skew or imported data). Previously daysSinceCreated/7 could be
// negative, deducting from the score.
func TestComputeScoreFutureCreatedNotNegative(t *testing.T) {
	now := time.Now()
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}

	pastTask, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Past")
	if err != nil {
		t.Fatal(err)
	}
	pastTask.Priority = 3
	pastTask.CreatedAt = now
	pastTask.UpdatedAt = now

	futureTask, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Future")
	if err != nil {
		t.Fatal(err)
	}
	futureTask.Priority = 3
	futureTask.CreatedAt = now.Add(30 * 24 * time.Hour) // created in the future
	futureTask.UpdatedAt = now

	pastScore := computeCrossProjectScore(pastTask, project, nil, now)
	futureScore := computeCrossProjectScore(futureTask, project, nil, now)

	if futureScore < pastScore {
		t.Errorf("future-created task score (%d) should not be lower than same-day task (%d) due to negative age bonus",
			futureScore, pastScore)
	}
}

// TestAllChildrenCompleteNilTask returns true on nil receiver instead of
// panicking.
func TestAllChildrenCompleteNilTask(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	if !allChildrenComplete(nil) {
		t.Error("expected true for nil task")
	}
}

func TestComputeScoreNilChildrenCountAsLeaf(t *testing.T) {
	now := time.Now()
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}

	leaf, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Leaf")
	if err != nil {
		t.Fatal(err)
	}
	leaf.Priority = 3
	leaf.CreatedAt = now
	leaf.UpdatedAt = now

	withNilChild, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Nil child")
	if err != nil {
		t.Fatal(err)
	}
	withNilChild.Priority = leaf.Priority
	withNilChild.CreatedAt = leaf.CreatedAt
	withNilChild.UpdatedAt = leaf.UpdatedAt
	withNilChild.Children = []*models.Task{nil}

	leafScore := computeCrossProjectScore(leaf, project, nil, now)
	nilChildScore := computeCrossProjectScore(withNilChild, project, nil, now)
	if nilChildScore != leafScore {
		t.Errorf("task with only nil children should score as a leaf: got %d, want %d", nilChildScore, leafScore)
	}
}

// TestGetSuggestionReasonRecentActivityWithFutureUpdate regression-guards an
// inconsistency between computeCrossProjectScore (which boosts tasks with
// non-positive daysSinceUpdated as "recent") and getSuggestionReason (which
// only checked == 0 and so omitted the "recent activity" reason for
// future-dated UpdatedAt timestamps from clock skew or imported data).
func TestGetSuggestionReasonRecentActivityWithFutureUpdate(t *testing.T) {
	now := time.Now()
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Future")
	if err != nil {
		t.Fatal(err)
	}
	task.Priority = 3
	task.UpdatedAt = now.Add(48 * time.Hour) // future
	task.CreatedAt = now
	task.WorkflowState = models.StateReady

	reason := getSuggestionReason(task, project, nil, now)
	if !containsSubstring(reason, "ecent activity") {
		t.Errorf("expected 'recent activity' for future UpdatedAt, got %q", reason)
	}
}
