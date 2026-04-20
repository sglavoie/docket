package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/testutil"
)

// ── columnForTask ─────────────────────────────────────────────────────────────

func TestColumnForTask_Section(t *testing.T) {
	tests := []struct {
		section string
		want    string
	}{
		{models.SectionInbox, "inbox"},
		{models.SectionNext, "next"},
		{models.SectionWaiting, "waiting"},
		{models.SectionSomeday, "someday"},
		{models.SectionReview, "review"},
		{models.SectionStandby, "standby"},
		{models.SectionCompleted, "completed"},
		{"", "next"}, // empty section → Next bucket
	}
	for _, tc := range tests {
		task := testutil.NewTestTask("t", models.TaskTypeTask)
		task.Section = tc.section
		got := columnForTask(task, KanbanModeSection, nil, "")
		if got != tc.want {
			t.Errorf("section %q: want %q, got %q", tc.section, tc.want, got)
		}
	}
}

func TestColumnForTask_Status(t *testing.T) {
	tests := []struct {
		status models.TaskStatus
		want   string
	}{
		{models.StatusTodo, "todo"},
		{models.StatusInProgress, "in_progress"},
		{models.StatusDone, "done"},
		{models.StatusCanceled, "canceled"},
	}
	for _, tc := range tests {
		task := testutil.NewTestTaskWithStatus("t", tc.status)
		got := columnForTask(task, KanbanModeStatus, nil, "")
		if got != tc.want {
			t.Errorf("status %q: want %q, got %q", tc.status, tc.want, got)
		}
	}
}

func TestColumnForTask_Priority(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{1, "p1"},
		{2, "p2"},
		{3, "p3"},
		{4, "p4"},
		{5, "p5"},
		{0, "unset"},
	}
	for _, tc := range tests {
		task := testutil.NewTestTaskWithPriority("t", tc.priority)
		got := columnForTask(task, KanbanModePriority, nil, "")
		if got != tc.want {
			t.Errorf("priority %d: want %q, got %q", tc.priority, tc.want, got)
		}
	}
}

func TestColumnForTask_Stages(t *testing.T) {
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	if got := columnForTask(task, KanbanModeStages, nil, ""); got != "unset" {
		t.Errorf("empty stage: want unset, got %q", got)
	}
	task.Stage = "design"
	if got := columnForTask(task, KanbanModeStages, nil, ""); got != "design" {
		t.Errorf("stage=design: want design, got %q", got)
	}
}

func TestColumnForTask_CustomSections(t *testing.T) {
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	if got := columnForTask(task, KanbanModeCustomSections, nil, ""); got != "unset" {
		t.Errorf("empty custom section: want unset, got %q", got)
	}
	task.CustomSection = "sprint-1"
	if got := columnForTask(task, KanbanModeCustomSections, nil, ""); got != "sprint-1" {
		t.Errorf("CustomSection=sprint-1: want sprint-1, got %q", got)
	}
}

func TestColumnForTask_ScoreBuckets_NilProfile(t *testing.T) {
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	cfg := &config.Config{}
	got := columnForTask(task, KanbanModeScoreBuckets, cfg, "no-such-project")
	if got != "0-25" {
		t.Errorf("nil profile: want 0-25, got %q", got)
	}
}

func TestColumnForTask_ScoreBuckets_WithProfile(t *testing.T) {
	profile := scoring.NewWSJFProfile()
	cfg := &config.Config{
		ProjectScoringProfile: map[string]string{"proj1": profile.ID},
		ScoringProfiles:       []scoring.ScoringProfile{profile},
	}

	// Task with high business value and low job size → high normalized score.
	highTask := testutil.NewTestTask("high", models.TaskTypeTask)
	highTask.ScoreDimensions = map[string]int{
		"business_value":   10,
		"time_criticality": 10,
		"risk_reduction":   10,
		"job_size":         1,
	}

	// Task with low values → low normalized score.
	lowTask := testutil.NewTestTask("low", models.TaskTypeTask)
	lowTask.ScoreDimensions = map[string]int{
		"business_value":   1,
		"time_criticality": 1,
		"risk_reduction":   1,
		"job_size":         5,
	}

	highCol := columnForTask(highTask, KanbanModeScoreBuckets, cfg, "proj1")
	lowCol := columnForTask(lowTask, KanbanModeScoreBuckets, cfg, "proj1")

	if highCol == "0-25" {
		t.Errorf("high-score task should not be in 0-25, got %q", highCol)
	}
	if highCol == lowCol {
		t.Errorf("high and low tasks should be in different buckets, both got %q", highCol)
	}
}

// ── rebuildColumns ────────────────────────────────────────────────────────────

func makeTestCard(section string, status models.TaskStatus) KanbanCard {
	task := testutil.NewTestTask("task", models.TaskTypeTask)
	task.Section = section
	task.Status = status
	proj := testutil.NewTestProject("p")
	return KanbanCard{Task: task, Project: proj}
}

func TestRebuildColumns_SectionMode_Distribution(t *testing.T) {
	cards := []KanbanCard{
		makeTestCard(models.SectionInbox, models.StatusTodo),
		makeTestCard(models.SectionInbox, models.StatusTodo),
		makeTestCard(models.SectionNext, models.StatusTodo),
	}
	cols := rebuildColumns(cards, KanbanModeSection, nil, nil, nil, nil, nil, nil)

	idx := map[string]int{}
	for i, c := range cols {
		idx[c.ID] = i
	}

	if n := len(cols[idx["inbox"]].Tasks); n != 2 {
		t.Errorf("inbox: want 2 tasks, got %d", n)
	}
	if n := len(cols[idx["next"]].Tasks); n != 1 {
		t.Errorf("next: want 1 task, got %d", n)
	}
}

func TestRebuildColumns_Empty(t *testing.T) {
	cols := rebuildColumns(nil, KanbanModeSection, nil, nil, nil, nil, nil, nil)
	if len(cols) == 0 {
		t.Fatal("expected non-empty column list even with no cards")
	}
	for _, c := range cols {
		if len(c.Tasks) != 0 {
			t.Errorf("column %q: want 0 tasks, got %d", c.ID, len(c.Tasks))
		}
	}
}

func TestRebuildColumns_AllSameColumn(t *testing.T) {
	cards := make([]KanbanCard, 5)
	for i := range cards {
		cards[i] = makeTestCard(models.SectionInbox, models.StatusTodo)
	}
	cols := rebuildColumns(cards, KanbanModeSection, nil, nil, nil, nil, nil, nil)
	total := 0
	for _, c := range cols {
		total += len(c.Tasks)
	}
	if total != 5 {
		t.Errorf("want 5 total tasks, got %d", total)
	}
}

func TestRebuildColumns_PreservesCursorFromPrev(t *testing.T) {
	cards := []KanbanCard{
		makeTestCard(models.SectionInbox, models.StatusTodo),
		makeTestCard(models.SectionInbox, models.StatusTodo),
	}
	prev := []KanbanColumn{{ID: "inbox", Cursor: 1, ScrollOffset: 0}}
	cols := rebuildColumns(cards, KanbanModeSection, prev, nil, nil, nil, nil, nil)
	for _, c := range cols {
		if c.ID == "inbox" {
			if c.Cursor != 1 {
				t.Errorf("cursor: want 1, got %d", c.Cursor)
			}
			return
		}
	}
	t.Error("inbox column not found")
}

func TestRebuildColumns_ClampsOutOfBoundsCursor(t *testing.T) {
	// Previous state had cursor=3, but now only 1 task remains.
	cards := []KanbanCard{makeTestCard(models.SectionInbox, models.StatusTodo)}
	prev := []KanbanColumn{{ID: "inbox", Cursor: 3}}
	cols := rebuildColumns(cards, KanbanModeSection, prev, nil, nil, nil, nil, nil)
	for _, c := range cols {
		if c.ID == "inbox" {
			if c.Cursor != 0 {
				t.Errorf("clamped cursor: want 0, got %d", c.Cursor)
			}
			return
		}
	}
	t.Error("inbox column not found")
}

func TestRebuildColumns_StatusMode(t *testing.T) {
	todo := testutil.NewTestTaskWithStatus("t1", models.StatusTodo)
	done := testutil.NewTestTaskWithStatus("t2", models.StatusDone)
	proj := testutil.NewTestProject("p")
	cards := []KanbanCard{
		{Task: todo, Project: proj},
		{Task: done, Project: proj},
	}
	cols := rebuildColumns(cards, KanbanModeStatus, nil, nil, nil, nil, nil, nil)
	idx := map[string]int{}
	for i, c := range cols {
		idx[c.ID] = i
	}
	if n := len(cols[idx["todo"]].Tasks); n != 1 {
		t.Errorf("todo: want 1, got %d", n)
	}
	if n := len(cols[idx["done"]].Tasks); n != 1 {
		t.Errorf("done: want 1, got %d", n)
	}
}

func TestRebuildColumns_PriorityMode(t *testing.T) {
	proj := testutil.NewTestProject("p")
	t1 := testutil.NewTestTaskWithPriority("high", 1)
	t2 := testutil.NewTestTaskWithPriority("unset", 0)
	cards := []KanbanCard{{Task: t1, Project: proj}, {Task: t2, Project: proj}}
	cols := rebuildColumns(cards, KanbanModePriority, nil, nil, nil, nil, nil, nil)
	idx := map[string]int{}
	for i, c := range cols {
		idx[c.ID] = i
	}
	if n := len(cols[idx["p1"]].Tasks); n != 1 {
		t.Errorf("p1: want 1, got %d", n)
	}
	if n := len(cols[idx["unset"]].Tasks); n != 1 {
		t.Errorf("unset: want 1, got %d", n)
	}
}

func TestRebuildColumns_StagesMode_DynamicColumn(t *testing.T) {
	proj := testutil.NewTestProject("p")
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	task.Stage = "alpha"
	cards := []KanbanCard{{Task: task, Project: proj}}
	// stages list doesn't include "alpha" → dynamic column created
	cols := rebuildColumns(cards, KanbanModeStages, nil, nil, nil, []string{"beta"}, nil, nil)
	found := false
	for _, c := range cols {
		if c.ID == "alpha" {
			found = true
			if len(c.Tasks) != 1 {
				t.Errorf("alpha: want 1 task, got %d", len(c.Tasks))
			}
		}
	}
	if !found {
		t.Error("dynamic 'alpha' column not created")
	}
}

// ── collectCustomSections ─────────────────────────────────────────────────────

func TestCollectCustomSections(t *testing.T) {
	proj := testutil.NewTestProject("p")
	t1 := testutil.NewTestTask("a", models.TaskTypeTask)
	t1.CustomSection = "alpha"
	t2 := testutil.NewTestTask("b", models.TaskTypeTask)
	t2.CustomSection = "beta"
	t3 := testutil.NewTestTask("c", models.TaskTypeTask)
	t3.CustomSection = "alpha" // duplicate

	cards := []KanbanCard{{Task: t1, Project: proj}, {Task: t2, Project: proj}, {Task: t3, Project: proj}}
	got := collectCustomSections(cards)
	if len(got) != 2 {
		t.Errorf("want 2 unique sections, got %d: %v", len(got), got)
	}
}

func TestCollectCustomSections_Empty(t *testing.T) {
	got := collectCustomSections(nil)
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

// ── collectStages ─────────────────────────────────────────────────────────────

func TestCollectStages_MergesGlobalAndProjectStages(t *testing.T) {
	proj := testutil.NewTestProject("p")
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	task.Stage = "project-stage"
	proj.Tasks = []*models.Task{task}

	got := collectStages([]string{"global-stage"}, []*models.Project{proj})
	if len(got) != 2 {
		t.Errorf("want 2 stages, got %d: %v", len(got), got)
	}
}

func TestCollectStages_Deduplication(t *testing.T) {
	proj := testutil.NewTestProject("p")
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	task.Stage = "s1"
	proj.Tasks = []*models.Task{task}

	got := collectStages([]string{"s1"}, []*models.Project{proj})
	if len(got) != 1 {
		t.Errorf("want 1 unique stage, got %d: %v", len(got), got)
	}
}
