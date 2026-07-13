package cmd

import (
	"sort"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/util"
)

func TestFormatRelativeDueCLI_NilDate(t *testing.T) {
	result := focus.FormatRelativeDue(nil)
	if result != "" {
		t.Errorf("expected empty string for nil date, got %q", result)
	}
}

func TestFormatRelativeDueCLI_Yesterday(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	result := focus.FormatRelativeDue(&yesterday)
	if result != "yesterday" {
		t.Errorf("expected 'yesterday', got %q", result)
	}
}

func TestFormatRelativeDueCLI_DaysAgo(t *testing.T) {
	tests := []struct {
		daysAgo  int
		expected string
	}{
		{2, "2 days ago"},
		{3, "3 days ago"},
		{7, "7 days ago"},
		{30, "30 days ago"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			date := time.Now().AddDate(0, 0, -tc.daysAgo)
			result := focus.FormatRelativeDue(&date)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestFormatRelativeDueCLI_Today(t *testing.T) {
	today := time.Now()
	result := focus.FormatRelativeDue(&today)
	if result != "today" {
		t.Errorf("expected 'today', got %q", result)
	}
}

func TestFormatRelativeDueCLI_Tomorrow(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	result := focus.FormatRelativeDue(&tomorrow)
	if result != "tomorrow" {
		t.Errorf("expected 'tomorrow', got %q", result)
	}
}

func TestFormatRelativeDueCLI_WeekdayName(t *testing.T) {
	// Test days 2-6 from now which should show weekday name
	for daysAhead := 2; daysAhead <= 6; daysAhead++ {
		t.Run(time.Now().AddDate(0, 0, daysAhead).Weekday().String(), func(t *testing.T) {
			futureDate := time.Now().AddDate(0, 0, daysAhead)
			result := focus.FormatRelativeDue(&futureDate)
			expectedWeekday := futureDate.Format("Mon")
			if result != expectedWeekday {
				t.Errorf("expected weekday %q, got %q", expectedWeekday, result)
			}
		})
	}
}

func TestFormatRelativeDueCLI_SameYear(t *testing.T) {
	// A date 10 days from now should show "Jan 2" format (same year)
	now := time.Now()
	futureDate := now.AddDate(0, 0, 10)

	// Only test if the future date is in the same year as now
	if futureDate.Year() == now.Year() {
		result := focus.FormatRelativeDue(&futureDate)
		expected := futureDate.Format("Jan 2")
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	}
}

func TestFormatRelativeDueCLI_DifferentYear(t *testing.T) {
	// Create a date in the next year (far enough to be >= 7 days away)
	now := time.Now()
	futureDate := time.Date(now.Year()+1, time.March, 15, 0, 0, 0, 0, now.Location())

	result := focus.FormatRelativeDue(&futureDate)
	expected := futureDate.Format("Jan 2 '06")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatRelativeDueCLI_PastDifferentYear(t *testing.T) {
	// Test a date in a past year
	now := time.Now()
	pastDate := time.Date(now.Year()-1, time.June, 10, 0, 0, 0, 0, now.Location())

	result := focus.FormatRelativeDue(&pastDate)
	// Past dates more than 1 day ago show "X days ago"
	// Calculate days since that past date
	today := util.StartOfDay(now)
	pastDay := util.StartOfDay(pastDate)
	daysAgo := int(today.Sub(pastDay).Hours() / 24)

	expectedPrefix := " days ago"
	if !containsSubstringFocus(result, expectedPrefix) {
		t.Errorf("expected result to end with %q for %d days ago, got %q", expectedPrefix, daysAgo, result)
	}
}

// FocusTask sorting tests

func TestFocusSorting_ByPriorityAscending(t *testing.T) {
	project := testutil.NewTestProject("Test")

	low := testutil.NewTestTask("Low priority", models.TaskTypeTask)
	low.Priority = 5

	high := testutil.NewTestTask("High priority", models.TaskTypeTask)
	high.Priority = 1

	medium := testutil.NewTestTask("Medium priority", models.TaskTypeTask)
	medium.Priority = 3

	tasks := []focus.FocusTask{
		{Task: low, Project: project},
		{Task: high, Project: project},
		{Task: medium, Project: project},
	}

	// Apply the same sorting logic as runFocus
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		return tasks[i].Task.UpdatedAt.After(tasks[j].Task.UpdatedAt)
	})

	if tasks[0].Task.Priority != 1 {
		t.Errorf("expected first task priority 1, got %d", tasks[0].Task.Priority)
	}
	if tasks[1].Task.Priority != 3 {
		t.Errorf("expected second task priority 3, got %d", tasks[1].Task.Priority)
	}
	if tasks[2].Task.Priority != 5 {
		t.Errorf("expected third task priority 5, got %d", tasks[2].Task.Priority)
	}
}

func TestFocusSorting_ByUpdatedAtWithinSamePriority(t *testing.T) {
	project := testutil.NewTestProject("Test")

	older := testutil.NewTestTask("Older task", models.TaskTypeTask)
	older.Priority = 3
	older.UpdatedAt = time.Now().Add(-2 * time.Hour)

	newer := testutil.NewTestTask("Newer task", models.TaskTypeTask)
	newer.Priority = 3
	newer.UpdatedAt = time.Now()

	tasks := []focus.FocusTask{
		{Task: older, Project: project},
		{Task: newer, Project: project},
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		return tasks[i].Task.UpdatedAt.After(tasks[j].Task.UpdatedAt)
	})

	if tasks[0].Task.Title != "Newer task" {
		t.Errorf("expected 'Newer task' first (most recent), got %q", tasks[0].Task.Title)
	}
	if tasks[1].Task.Title != "Older task" {
		t.Errorf("expected 'Older task' second, got %q", tasks[1].Task.Title)
	}
}

func TestFocusSorting_CombinedPriorityAndUpdatedAt(t *testing.T) {
	project := testutil.NewTestProject("Test")

	// High priority, older
	highOlder := testutil.NewTestTask("High priority older", models.TaskTypeTask)
	highOlder.Priority = 1
	highOlder.UpdatedAt = time.Now().Add(-1 * time.Hour)

	// High priority, newer
	highNewer := testutil.NewTestTask("High priority newer", models.TaskTypeTask)
	highNewer.Priority = 1
	highNewer.UpdatedAt = time.Now()

	// Low priority, very recent
	lowRecent := testutil.NewTestTask("Low priority recent", models.TaskTypeTask)
	lowRecent.Priority = 5
	lowRecent.UpdatedAt = time.Now().Add(1 * time.Minute)

	tasks := []focus.FocusTask{
		{Task: lowRecent, Project: project},
		{Task: highOlder, Project: project},
		{Task: highNewer, Project: project},
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		return tasks[i].Task.UpdatedAt.After(tasks[j].Task.UpdatedAt)
	})

	// High priority tasks should come first regardless of update time
	if tasks[0].Task.Priority != 1 {
		t.Errorf("expected first task to be priority 1, got %d", tasks[0].Task.Priority)
	}
	// Within same priority, newer should come first
	if tasks[0].Task.Title != "High priority newer" {
		t.Errorf("expected 'High priority newer' first, got %q", tasks[0].Task.Title)
	}
	if tasks[1].Task.Title != "High priority older" {
		t.Errorf("expected 'High priority older' second, got %q", tasks[1].Task.Title)
	}
	// Low priority last
	if tasks[2].Task.Title != "Low priority recent" {
		t.Errorf("expected 'Low priority recent' third, got %q", tasks[2].Task.Title)
	}
}

// Filtering tests

// collectFilteredFocusTasks replicates the focus filter the CLI applies to a
// queue's tasks: skip tasks in a terminal (done/canceled) state. The retired
// per-task Active flag is now modeled as queue membership and covered directly
// by the collectFocusQueueTasks tests below, so callers pass only the tasks
// already known to be queued.
func collectFilteredFocusTasks(project *models.Project) []focus.FocusTask {
	var tasks []focus.FocusTask
	for _, task := range project.Tasks {
		if task.State().IsTerminal() {
			continue
		}
		tasks = append(tasks, focus.FocusTask{Task: task, Project: project})
	}
	return tasks
}

func TestFocusFiltering_SkipsCompletedTasks(t *testing.T) {
	project := testutil.NewTestProject("Test")

	done := testutil.NewTestTask("Done task", models.TaskTypeTask)
	done.WorkflowState = models.StateDone

	canceled := testutil.NewTestTask("Canceled task", models.TaskTypeTask)
	canceled.WorkflowState = models.StateCanceled

	active := testutil.NewTestTask("Active task", models.TaskTypeTask)
	active.WorkflowState = models.StateReady

	project.Tasks = []*models.Task{done, canceled, active}

	tasks := collectFilteredFocusTasks(project)

	if len(tasks) != 1 {
		t.Errorf("expected 1 non-completed task, got %d", len(tasks))
	}
	if tasks[0].Task.Title != "Active task" {
		t.Errorf("expected 'Active task', got %q", tasks[0].Task.Title)
	}
}

func TestFocusFiltering_IncludesBlockedTasks(t *testing.T) {
	project := testutil.NewTestProject("Test")

	blocked := testutil.NewTestTask("Blocked task", models.TaskTypeTask)
	blocked.WorkflowState = models.StateWaiting

	project.Tasks = []*models.Task{blocked}

	tasks := collectFilteredFocusTasks(project)

	if len(tasks) != 1 {
		t.Errorf("expected 1 blocked task, got %d", len(tasks))
	}
}

func TestFocusFiltering_MultipleProjects(t *testing.T) {
	project1 := testutil.NewTestProject("Project 1")
	task1 := testutil.NewTestTask("Task from P1", models.TaskTypeTask)
	task1.Priority = 1
	project1.Tasks = []*models.Task{task1}

	project2 := testutil.NewTestProject("Project 2")
	task2 := testutil.NewTestTask("Task from P2", models.TaskTypeTask)
	task2.Priority = 2
	project2.Tasks = []*models.Task{task2}

	var tasks []focus.FocusTask
	for _, project := range []*models.Project{project1, project2} {
		tasks = append(tasks, collectFilteredFocusTasks(project)...)
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Task.Priority != tasks[j].Task.Priority {
			return tasks[i].Task.Priority < tasks[j].Task.Priority
		}
		return tasks[i].Task.UpdatedAt.After(tasks[j].Task.UpdatedAt)
	})

	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks from 2 projects, got %d", len(tasks))
	}

	// Higher priority (1) should come first
	if tasks[0].Task.Title != "Task from P1" {
		t.Errorf("expected 'Task from P1' first (priority 1), got %q", tasks[0].Task.Title)
	}
	if tasks[0].Project.Name != "Project 1" {
		t.Errorf("expected project name 'Project 1', got %q", tasks[0].Project.Name)
	}
}

func TestFocusFiltering_EmptyResult(t *testing.T) {
	project := testutil.NewTestProject("Test")

	done := testutil.NewTestTask("Done task", models.TaskTypeTask)
	done.WorkflowState = models.StateDone

	project.Tasks = []*models.Task{done}

	tasks := collectFilteredFocusTasks(project)

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// containsSubstringFocus is a helper to check if a string contains a substring.
func containsSubstringFocus(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCollectFocusQueueTasks_SkipsNilTasks guards the legacy --active path
// (now backed by Focus Queue membership): when a project's Tasks slice contains
// a stray nil entry, the loop must skip it instead of panicking on field access.
func TestCollectFocusQueueTasks_SkipsNilTasks(t *testing.T) {
	project := testutil.NewTestProject("Test")

	queued := testutil.NewTestTask("Queued task", models.TaskTypeTask)

	// Inject a nil between real tasks to simulate corrupted in-memory state.
	project.Tasks = []*models.Task{queued, nil}

	queue := []config.FocusQueueItem{{TaskID: queued.ID, ProjectID: project.ID}}

	tasks := collectFocusQueueTasks(queue, []*models.Project{project, nil})

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task (nil and nil project skipped), got %d", len(tasks))
	}
	if tasks[0].Task.Title != "Queued task" {
		t.Errorf("unexpected task: %q", tasks[0].Task.Title)
	}
}

// TestCollectFocusQueueTasks_ExcludesTerminalAndUnqueued asserts the queue
// membership + nonterminal-state filter that replaced the Active-flag read.
func TestCollectFocusQueueTasks_ExcludesTerminalAndUnqueued(t *testing.T) {
	project := testutil.NewTestProject("Test")

	queued := testutil.NewTestTask("Queued task", models.TaskTypeTask)
	unqueued := testutil.NewTestTask("Unqueued task", models.TaskTypeTask)
	done := testutil.NewTestTask("Done task", models.TaskTypeTask)
	done.Complete()
	project.Tasks = []*models.Task{queued, unqueued, done}

	queue := []config.FocusQueueItem{
		{TaskID: queued.ID, ProjectID: project.ID},
		{TaskID: done.ID, ProjectID: project.ID},
	}

	tasks := collectFocusQueueTasks(queue, []*models.Project{project})

	if len(tasks) != 1 {
		t.Fatalf("expected only the queued nonterminal task, got %d", len(tasks))
	}
	if tasks[0].Task.Title != "Queued task" {
		t.Errorf("unexpected task: %q", tasks[0].Task.Title)
	}
}

// TestBuildProjectAndTaskMaps_SkipsNilTasks guards `tt focus today` and
// `tt focus queue`: building lookup maps must not panic when a project's
// Tasks slice contains nil entries.
func TestBuildProjectAndTaskMaps_SkipsNilTasks(t *testing.T) {
	project := testutil.NewTestProject("Test")
	task := testutil.NewTestTask("Real", models.TaskTypeTask)
	project.Tasks = []*models.Task{nil, task, nil}

	projectMap, taskMap := buildProjectAndTaskMaps([]*models.Project{nil, project})

	if _, ok := projectMap[project.ID]; !ok {
		t.Errorf("expected project %q in projectMap", project.ID)
	}
	if len(taskMap) != 1 {
		t.Fatalf("expected 1 task in taskMap, got %d", len(taskMap))
	}
	if _, ok := taskMap[task.ID]; !ok {
		t.Errorf("expected task %q in taskMap", task.ID)
	}
}

// TestSortFocusTasks_DeterministicTiebreak ensures that when two tasks share
// both priority and UpdatedAt, the order is stable across invocations rather
// than depending on input order. Without an explicit Task.ID tiebreak the row
// order would flicker between runs.
func TestSortFocusTasks_DeterministicTiebreak(t *testing.T) {
	project := testutil.NewTestProject("Test")
	now := time.Now()

	mk := func(id, title string) focus.FocusTask {
		task := testutil.NewTestTask(title, models.TaskTypeTask)
		task.ID = id
		task.Priority = 3
		task.UpdatedAt = now
		return focus.FocusTask{Task: task, Project: project}
	}

	a := mk("aaa", "A")
	b := mk("bbb", "B")
	c := mk("ccc", "C")

	cases := [][]focus.FocusTask{
		{a, b, c},
		{c, b, a},
		{b, a, c},
		{c, a, b},
	}

	for i, input := range cases {
		sortFocusTasks(input)
		if input[0].Task.ID != "aaa" || input[1].Task.ID != "bbb" || input[2].Task.ID != "ccc" {
			t.Errorf("case %d: expected aaa, bbb, ccc; got %s, %s, %s",
				i, input[0].Task.ID, input[1].Task.ID, input[2].Task.ID)
		}
	}
}

// TestSortFocusTasks_EmptySlice ensures the sort helper does not panic on
// an empty or nil slice (which `tt focus` may legitimately produce when no
// active projects yield tasks).
func TestSortFocusTasks_EmptySlice(t *testing.T) {
	sortFocusTasks(nil)
	sortFocusTasks([]focus.FocusTask{})
}
