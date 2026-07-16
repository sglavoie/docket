package tui

import (
	"testing"

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
		task.SetSection(tc.section)
		got := columnForTask(task, KanbanModeSection, nil, "")
		if got != tc.want {
			t.Errorf("section %q: want %q, got %q", tc.section, tc.want, got)
		}
	}
}

func TestColumnForTask_Section_CompletedStatusWins(t *testing.T) {
	tests := []struct {
		name    string
		section string
		status  models.TaskStatus
	}{
		{"done in inbox", models.SectionInbox, models.StatusDone},
		{"canceled in next", models.SectionNext, models.StatusCanceled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := testutil.NewTestTask("t", models.TaskTypeTask)
			task.WorkflowState = models.DeriveWorkflowState(tc.status, tc.section, "")
			got := columnForTask(task, KanbanModeSection, nil, "")
			if got != "completed" {
				t.Errorf("completed status in section %q: want completed, got %q", tc.section, got)
			}
		})
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

func TestColumnForTask_ScoreBuckets_NilProfile(t *testing.T) {
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	cfg := newTestConfig()
	got := columnForTask(task, KanbanModeScoreBuckets, cfg.Automation(), "no-such-project")
	if got != "0-25" {
		t.Errorf("nil profile: want 0-25, got %q", got)
	}
}

func TestColumnForTask_ScoreBuckets_WithProfile(t *testing.T) {
	profile := scoring.NewWSJFProfile()
	cfg := newTestConfig()
	cfg.Automation().ProjectScoringProfile = map[string]string{"proj1": profile.ID}
	cfg.Automation().ScoringProfiles = []scoring.ScoringProfile{profile}

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

	highCol := columnForTask(highTask, KanbanModeScoreBuckets, cfg.Automation(), "proj1")
	lowCol := columnForTask(lowTask, KanbanModeScoreBuckets, cfg.Automation(), "proj1")

	if highCol == "0-25" {
		t.Errorf("high-score task should not be in 0-25, got %q", highCol)
	}
	if highCol == lowCol {
		t.Errorf("high and low tasks should be in different buckets, both got %q", highCol)
	}
}

// ── rebuildColumns ────────────────────────────────────────────────────────────

func TestRebuildColumns_SectionMode_Distribution(t *testing.T) {
	cards := []KanbanCard{
		makeTestCard(models.SectionInbox, models.StatusTodo),
		makeTestCard(models.SectionInbox, models.StatusTodo),
		makeTestCard(models.SectionNext, models.StatusTodo),
	}
	cols := rebuildColumns(cards, KanbanModeSection, nil, nil, nil, nil)

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

func TestRebuildColumns_SortsColumnTasksByProjectName(t *testing.T) {
	alpha := testutil.NewTestProject("Alpha")
	beta := testutil.NewTestProject("Beta")

	card := func(project *models.Project, title string) KanbanCard {
		task := testutil.NewTestTask(title, models.TaskTypeTask)
		task.WorkflowState = models.StateInbox
		return KanbanCard{Task: task, Project: project}
	}

	cards := []KanbanCard{
		card(beta, "beta first"),
		card(alpha, "alpha first"),
		card(beta, "beta second"),
		card(alpha, "alpha second"),
	}

	cols := rebuildColumns(cards, KanbanModeSection, nil, nil, nil, nil)

	var inbox KanbanColumn
	found := false
	for _, col := range cols {
		if col.ID == "inbox" {
			inbox = col
			found = true
			break
		}
	}
	if !found {
		t.Fatal("inbox column not found")
	}

	got := make([]string, 0, len(inbox.Tasks))
	for _, task := range inbox.Tasks {
		got = append(got, task.Task.Title)
	}
	want := []string{"alpha first", "alpha second", "beta first", "beta second"}
	if len(got) != len(want) {
		t.Fatalf("task order length: want %d, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("task order[%d]: want %q, got %q; full order: %v", i, want[i], got[i], got)
		}
	}
}

func TestRebuildColumns_SectionMode_CompletedStatusOnlyInCompletedColumn(t *testing.T) {
	cards := []KanbanCard{
		makeTestCard(models.SectionInbox, models.StatusTodo),
		makeTestCard(models.SectionInbox, models.StatusDone),
		makeTestCard(models.SectionNext, models.StatusCanceled),
		makeTestCard(models.SectionNext, models.StatusTodo),
	}
	cols := rebuildColumns(
		cards,
		KanbanModeSection,
		nil,
		[]string{models.SectionInbox, models.SectionNext},
		nil,
		nil,
	)

	idx := map[string]int{}
	for i, c := range cols {
		idx[c.ID] = i
	}
	completedIdx, ok := idx["completed"]
	if !ok {
		t.Fatal("completed column not found")
	}
	completedCol := cols[completedIdx]
	if completedCol.Label != models.SectionCompleted {
		t.Errorf("completed label: want %q, got %q", models.SectionCompleted, completedCol.Label)
	}
	if n := len(cols[idx["inbox"]].Tasks); n != 1 {
		t.Errorf("inbox: want 1 non-completed task, got %d", n)
	}
	if n := len(cols[idx["next"]].Tasks); n != 1 {
		t.Errorf("next: want 1 non-completed task, got %d", n)
	}
	if n := len(completedCol.Tasks); n != 2 {
		t.Errorf("completed: want 2 tasks, got %d", n)
	}
	for _, id := range []string{"inbox", "next"} {
		for _, card := range cols[idx[id]].Tasks {
			if card.Task.IsComplete() {
				t.Errorf("%s column contains completed task with state %q", id, card.Task.State())
			}
		}
	}
}

func TestRebuildColumns_Empty(t *testing.T) {
	cols := rebuildColumns(nil, KanbanModeSection, nil, nil, nil, nil)
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
	cols := rebuildColumns(cards, KanbanModeSection, nil, nil, nil, nil)
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
	cols := rebuildColumns(cards, KanbanModeSection, prev, nil, nil, nil)
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
	cols := rebuildColumns(cards, KanbanModeSection, prev, nil, nil, nil)
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

func TestRebuildColumns_ClampsOutOfBoundsScrollOffset(t *testing.T) {
	cards := []KanbanCard{makeTestCard(models.SectionInbox, models.StatusTodo)}
	prev := []KanbanColumn{{ID: "inbox", Cursor: 0, ScrollOffset: 50}}
	cols := rebuildColumns(cards, KanbanModeSection, prev, nil, nil, nil)
	for _, c := range cols {
		if c.ID == "inbox" {
			if c.ScrollOffset >= len(c.Tasks) {
				t.Errorf("ScrollOffset not clamped: got %d for %d tasks", c.ScrollOffset, len(c.Tasks))
			}
			return
		}
	}
	t.Error("inbox column not found")
}

func TestRebuildColumns_ClampsNegativeCursor(t *testing.T) {
	cards := []KanbanCard{makeTestCard(models.SectionInbox, models.StatusTodo)}
	prev := []KanbanColumn{{ID: "inbox", Cursor: -5, ScrollOffset: -3}}
	cols := rebuildColumns(cards, KanbanModeSection, prev, nil, nil, nil)
	for _, c := range cols {
		if c.ID == "inbox" {
			if c.Cursor != 0 {
				t.Errorf("negative cursor not clamped: got %d", c.Cursor)
			}
			if c.ScrollOffset != 0 {
				t.Errorf("negative ScrollOffset not clamped: got %d", c.ScrollOffset)
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
	cols := rebuildColumns(cards, KanbanModeStatus, nil, nil, nil, nil)
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
	cols := rebuildColumns(cards, KanbanModePriority, nil, nil, nil, nil)
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

// ── filteredCards / tag filter ────────────────────────────────────────────────

func TestKanbanFilteredCards_TagFilter(t *testing.T) {
	proj := testutil.NewTestProject("p")
	tagged := testutil.NewTestTask("tagged", models.TaskTypeTask)
	tagged.Tags = []string{"urgent"}
	untagged := testutil.NewTestTask("untagged", models.TaskTypeTask)
	proj.Tasks = []*models.Task{tagged, untagged}

	cfg := newTestConfig()
	m := &KanbanModel{
		runtime:  testRuntimeAt(cfg.DataDir(), cfg),
		allCards: []KanbanCard{{Task: tagged, Project: proj}, {Task: untagged, Project: proj}},
		FilterMixin: FilterMixin{
			filter: newFilterState(),
		},
	}

	// No filter → all cards returned.
	if got := len(m.filteredCards()); got != 2 {
		t.Fatalf("no filter: want 2 cards, got %d", got)
	}

	// Set tag filter → only tagged card returned.
	m.filter.Tags = map[string]bool{"urgent": true}
	got := m.filteredCards()
	if len(got) != 1 {
		t.Fatalf("tag filter: want 1 card, got %d", len(got))
	}
	if got[0].Task.Title != "tagged" {
		t.Errorf("tag filter: want 'tagged', got %q", got[0].Task.Title)
	}
}

func TestKanbanFilteredCards_PerspectiveUsesWorkflowSection(t *testing.T) {
	project := testutil.NewTestProject("p")
	next := &models.Task{ID: "next", Title: "Next", WorkflowState: models.StateNext, Priority: 3}
	inbox := &models.Task{ID: "inbox", Title: "Inbox", WorkflowState: models.StateInbox, Priority: 1, Tags: []string{"tagged"}}
	done := &models.Task{ID: "done", Title: "Done", WorkflowState: models.StateDone}
	canceled := &models.Task{ID: "canceled", Title: "Canceled", WorkflowState: models.StateCanceled}
	m := &KanbanModel{
		crossProject:      true,
		activePerspective: 3,
		allCards: []KanbanCard{
			{Task: next, Project: project},
			{Task: inbox, Project: project},
			{Task: done, Project: project},
			{Task: canceled, Project: project},
		},
		FilterMixin: FilterMixin{filter: newFilterState()},
	}

	got := m.filteredCards()
	if len(got) != 1 || got[0].Task.ID != next.ID {
		t.Fatalf("Next perspective: got %#v, want only %q", got, next.ID)
	}

	m.activePerspective = 4
	got = m.filteredCards()
	if len(got) != 1 || got[0].Task.ID != inbox.ID {
		t.Fatalf("Inbox perspective: got %#v, want only %q", got, inbox.ID)
	}

	m.activePerspective = PerspectiveCompleted
	got = m.filteredCards()
	if len(got) != 2 || got[0].Task.ID != done.ID || got[1].Task.ID != canceled.ID {
		t.Fatalf("Completed perspective: got %#v, want done and canceled", got)
	}
}

func TestKanbanRefreshAndClamp_AppliesTagFilter(t *testing.T) {
	proj := testutil.NewTestProject("p")
	tagged := testutil.NewTestTask("tagged", models.TaskTypeTask)
	tagged.SetSection(models.SectionInbox)
	tagged.Tags = []string{"bug"}
	untagged := testutil.NewTestTask("untagged", models.TaskTypeTask)
	untagged.SetSection(models.SectionInbox)
	proj.Tasks = []*models.Task{tagged, untagged}

	cfg := newTestConfig()
	m := &KanbanModel{
		runtime:  testRuntimeAt(cfg.DataDir(), cfg),
		mode:     KanbanModeSection,
		allCards: []KanbanCard{{Task: tagged, Project: proj}, {Task: untagged, Project: proj}},
		FilterMixin: FilterMixin{
			filter: newFilterState(),
		},
	}
	m.rebuildCols()

	// Count inbox tasks before filter.
	inboxCount := 0
	for _, col := range m.columns {
		if col.ID == "inbox" {
			inboxCount = len(col.Tasks)
		}
	}
	if inboxCount != 2 {
		t.Fatalf("pre-filter inbox: want 2, got %d", inboxCount)
	}

	// Apply tag filter and rebuild.
	m.filter.Tags = map[string]bool{"bug": true}
	m.refreshAndClamp()

	for _, col := range m.columns {
		if col.ID == "inbox" {
			if len(col.Tasks) != 1 {
				t.Errorf("post-filter inbox: want 1, got %d", len(col.Tasks))
			}
			if len(col.Tasks) > 0 && col.Tasks[0].Task.Title != "tagged" {
				t.Errorf("post-filter: want 'tagged', got %q", col.Tasks[0].Task.Title)
			}
			return
		}
	}
	t.Error("inbox column not found")
}

func TestKanbanBuildTagCmd_CrossProjectSource(t *testing.T) {
	cfg := newTestConfig()

	// Single-project kanban should emit ViewKanban source.
	m := &KanbanModel{
		runtime:      testRuntimeAt(cfg.DataDir(), cfg),
		mode:         KanbanModeSection,
		crossProject: false,
		FilterMixin:  FilterMixin{filter: newFilterState()},
	}
	cmd := m.buildTagCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	tagMsg, ok := msg.(ShowTagFilterMsg)
	if !ok {
		t.Fatalf("expected ShowTagFilterMsg, got %T", msg)
	}
	if tagMsg.Source != ViewKanban {
		t.Errorf("single-project source = %v, want %v", tagMsg.Source, ViewKanban)
	}

	// Cross-project kanban should emit ViewCrossProjectKanban source.
	m2 := &KanbanModel{
		runtime:      testRuntimeAt(cfg.DataDir(), cfg),
		mode:         KanbanModeSection,
		crossProject: true,
		FilterMixin:  FilterMixin{filter: newFilterState()},
	}
	cmd2 := m2.buildTagCmd()
	msg2 := cmd2()
	tagMsg2, ok := msg2.(ShowTagFilterMsg)
	if !ok {
		t.Fatalf("expected ShowTagFilterMsg, got %T", msg2)
	}
	if tagMsg2.Source != ViewCrossProjectKanban {
		t.Errorf("cross-project source = %v, want %v", tagMsg2.Source, ViewCrossProjectKanban)
	}
}

// TestRestoreKanbanState_ClampsOutOfBoundsScrollOffset verifies that
// restoreKanbanState clamps a cached ScrollOffset that no longer fits within
// a column's task list. Without clamping, deleting tasks while away from the
// kanban view would leave the column scrolled past its last task on return.
