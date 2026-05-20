package external

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildProjectFzfInput(t *testing.T) {
	projects := []*models.Project{
		{Name: "project-a", Slug: "project-a", Context: "work"},
		{Name: "project-b", Slug: "project-b", Context: "personal"},
	}

	// Simulate the input building logic from FzfSelectProject.
	// Format: slug<TAB>display-name (slug)<TAB>context<TAB>N tasks
	var lines []string
	for _, p := range projects {
		line := fmt.Sprintf("%s\t%s (%s)\t%s\t%d tasks", p.Slug, p.Name, p.Slug, p.Context, p.CountTasks())
		lines = append(lines, line)
	}
	input := strings.Join(lines, "\n")

	t.Run("formats project lines correctly", func(t *testing.T) {
		if !strings.Contains(input, "project-a\tproject-a (project-a)\twork\t0 tasks") {
			t.Error("expected first project line with slug, display name, context, and task count")
		}
		if !strings.Contains(input, "project-b\tproject-b (project-b)\tpersonal\t0 tasks") {
			t.Error("expected second project line with slug, display name, context, and task count")
		}
	})

	t.Run("uses tab as delimiter with 4 parts", func(t *testing.T) {
		firstLine := strings.Split(input, "\n")[0]
		parts := strings.Split(firstLine, "\t")
		if len(parts) != 4 {
			t.Errorf("expected 4 tab-separated parts, got %d", len(parts))
		}
	})

	t.Run("slug is first field", func(t *testing.T) {
		firstLine := strings.Split(input, "\n")[0]
		parts := strings.Split(firstLine, "\t")
		if parts[0] != "project-a" {
			t.Errorf("expected slug 'project-a' as first field, got %q", parts[0])
		}
	})
}

func TestParseProjectFzfOutput(t *testing.T) {
	projectMap := map[string]*models.Project{
		"my-project-v2": {Name: "My Project (v2)", Slug: "my-project-v2"},
		"simple":        {Name: "Simple", Slug: "simple"},
	}

	t.Run("parses slug from first field", func(t *testing.T) {
		// Simulated fzf output: the full line including hidden slug column
		selected := "simple\tSimple (simple)\twork\t3 tasks"
		parts := strings.SplitN(selected, "\t", 2)
		if len(parts) == 0 {
			t.Fatal("expected parts from split")
		}
		project, ok := projectMap[parts[0]]
		if !ok {
			t.Fatalf("expected to find project with slug %q", parts[0])
		}
		if project.Name != "Simple" {
			t.Errorf("expected 'Simple', got %q", project.Name)
		}
	})

	t.Run("correctly handles project name with parentheses", func(t *testing.T) {
		// Project name contains parens — old LastIndex approach would break
		selected := "my-project-v2\tMy Project (v2) (my-project-v2)\twork\t0 tasks"
		parts := strings.SplitN(selected, "\t", 2)
		if len(parts) == 0 {
			t.Fatal("expected parts from split")
		}
		project, ok := projectMap[parts[0]]
		if !ok {
			t.Fatalf("expected to find project with slug %q", parts[0])
		}
		if project.Slug != "my-project-v2" {
			t.Errorf("expected slug 'my-project-v2', got %q", project.Slug)
		}
	})
}

func TestBuildTaskFzfInput(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Type: models.TaskTypeTask, Status: models.StatusTodo, Title: "First task"},
		{ShortID: 2, Type: models.TaskTypeBug, Status: models.StatusDone, Title: "Bug fix"},
	}

	// Simulate the input building logic from FzfSelectTask
	var lines []string
	for _, task := range tasks {
		line := fmt.Sprintf("#%d\t%s\t%s\t%s",
			task.ShortID,
			task.Type.Icon(),
			task.Status.Icon(),
			task.Title)
		lines = append(lines, line)
	}
	input := strings.Join(lines, "\n")

	t.Run("formats task lines correctly", func(t *testing.T) {
		if !strings.Contains(input, "#1\t") {
			t.Error("expected task line to start with #1")
		}
		if !strings.Contains(input, "#2\t") {
			t.Error("expected task line to start with #2")
		}
		if !strings.Contains(input, "First task") {
			t.Error("expected first task title in output")
		}
		if !strings.Contains(input, "Bug fix") {
			t.Error("expected second task title in output")
		}
	})

	t.Run("uses tab as delimiter", func(t *testing.T) {
		firstLine := strings.Split(input, "\n")[0]
		parts := strings.Split(firstLine, "\t")
		if len(parts) != 4 {
			t.Errorf("expected 4 tab-separated parts, got %d", len(parts))
		}
	})

	t.Run("short ID prefixed with hash", func(t *testing.T) {
		firstLine := strings.Split(input, "\n")[0]
		if !strings.HasPrefix(firstLine, "#") {
			t.Error("expected line to start with # for short ID")
		}
	})
}

func TestParseTaskFzfOutput(t *testing.T) {
	taskMap := map[int]*models.Task{
		1: {ShortID: 1, Title: "First task"},
		2: {ShortID: 2, Title: "Second task"},
	}

	t.Run("parses valid selection", func(t *testing.T) {
		selected := "#1\t📋\t⬜\tFirst task"
		parts := strings.Split(selected, "\t")
		if len(parts) == 0 {
			t.Fatal("expected parts from split")
		}
		idStr := strings.TrimPrefix(parts[0], "#")
		var shortID int
		_, err := fmt.Sscanf(idStr, "%d", &shortID)
		if err != nil {
			t.Fatalf("failed to parse short ID: %v", err)
		}
		task, ok := taskMap[shortID]
		if !ok {
			t.Fatal("expected to find task in map")
		}
		if task.Title != "First task" {
			t.Errorf("expected 'First task', got %q", task.Title)
		}
	})

	t.Run("handles whitespace in selection", func(t *testing.T) {
		selected := strings.TrimSpace("  #2\t📋\t⬜\tSecond task  \n")
		parts := strings.Split(selected, "\t")
		idStr := strings.TrimPrefix(parts[0], "#")
		var shortID int
		_, err := fmt.Sscanf(idStr, "%d", &shortID)
		if err != nil {
			t.Fatalf("failed to parse short ID: %v", err)
		}
		if shortID != 2 {
			t.Errorf("expected short ID 2, got %d", shortID)
		}
	})
}

func TestBuildMultiSelectProjectFzfInput_PreselectedMarker(t *testing.T) {
	projects := []*models.Project{
		{ID: "id-a", Name: "project-a", Slug: "project-a", Context: "work"},
		{ID: "id-b", Name: "project-b", Slug: "project-b", Context: "personal"},
		{ID: "id-c", Name: "project-c", Slug: "project-c", Context: "work"},
	}

	preselectedIDs := map[string]bool{
		"id-a": true,
		"id-c": true,
	}

	// Simulate the input building logic from FzfMultiSelectProjects
	var lines []string
	for _, p := range projects {
		marker := " "
		if preselectedIDs[p.ID] {
			marker = "*"
		}
		line := fmt.Sprintf("%s\t%s %s (%s)\t%s\t%d tasks", p.Slug, marker, p.Name, p.Slug, p.Context, p.CountTasks())
		lines = append(lines, line)
	}
	input := strings.Join(lines, "\n")

	t.Run("marks pre-selected projects with asterisk", func(t *testing.T) {
		inputLines := strings.Split(input, "\n")
		// project-a is pre-selected
		if !strings.Contains(inputLines[0], "\t* project-a") {
			t.Errorf("expected project-a to be marked with *, got %q", inputLines[0])
		}
		// project-b is NOT pre-selected
		if !strings.Contains(inputLines[1], "\t  project-b") {
			t.Errorf("expected project-b to not be marked with *, got %q", inputLines[1])
		}
		// project-c is pre-selected
		if !strings.Contains(inputLines[2], "\t* project-c") {
			t.Errorf("expected project-c to be marked with *, got %q", inputLines[2])
		}
	})

	t.Run("nil preselectedIDs marks nothing", func(t *testing.T) {
		var noPreselect map[string]bool
		var lines2 []string
		for _, p := range projects {
			marker := " "
			if noPreselect[p.ID] {
				marker = "*"
			}
			line := fmt.Sprintf("%s\t%s %s (%s)\t%s\t%d tasks", p.Slug, marker, p.Name, p.Slug, p.Context, p.CountTasks())
			lines2 = append(lines2, line)
		}
		for _, line := range lines2 {
			if strings.Contains(line, "\t*") {
				t.Errorf("expected no markers with nil preselectedIDs, got %q", line)
			}
		}
	})
}

func TestSanitizeFzfField(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"plain", "plain"},
		{"with\ttab", "with tab"},
		{"with\nnewline", "with newline"},
		{"with\r\ncrlf", "with  crlf"},
		{"\t\nmixed\t\n", "  mixed  "},
		{"", ""},
	}
	for _, tc := range cases {
		got := sanitizeFzfField(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeFzfField(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTaskTitleWithTabDoesNotBreakFzfInput(t *testing.T) {
	// A task title containing a tab character would corrupt fzf's tab-delimited
	// input — the column count would no longer be 4 and parsing would break.
	tasks := []*models.Task{
		{ShortID: 7, Type: models.TaskTypeTask, Status: models.StatusTodo, Title: "Title\twith\ttabs"},
	}

	var lines []string
	for _, task := range tasks {
		line := fmt.Sprintf("#%d\t%s\t%s\t%s",
			task.ShortID,
			task.Type.Icon(),
			task.Status.Icon(),
			sanitizeFzfField(task.Title))
		lines = append(lines, line)
	}
	parts := strings.Split(lines[0], "\t")
	if len(parts) != 4 {
		t.Errorf("expected 4 tab-separated parts after sanitization, got %d (line=%q)", len(parts), lines[0])
	}
	if strings.Contains(parts[3], "\t") {
		t.Errorf("expected no tabs in title field after sanitization, got %q", parts[3])
	}
}

func TestProjectFieldsWithNewlinesDoNotBreakFzfInput(t *testing.T) {
	// Newlines in a project Name would split a single project across multiple
	// fzf lines and break selection.
	projects := []*models.Project{
		{Slug: "p1", Name: "name\nwith\nnewlines", Context: "ctx\twith\ttab"},
	}

	var lines []string
	for _, p := range projects {
		line := fmt.Sprintf("%s\t%s (%s)\t%s\t%d tasks",
			sanitizeFzfField(p.Slug),
			sanitizeFzfField(p.Name),
			sanitizeFzfField(p.Slug),
			sanitizeFzfField(p.Context),
			p.CountTasks())
		lines = append(lines, line)
	}
	if strings.Count(lines[0], "\n") != 0 {
		t.Errorf("expected no newlines in sanitized line, got %q", lines[0])
	}
	parts := strings.Split(lines[0], "\t")
	if len(parts) != 4 {
		t.Errorf("expected 4 tab-separated parts after sanitization, got %d (line=%q)", len(parts), lines[0])
	}
}

func TestParseMultiSelectOutput(t *testing.T) {
	taskMap := map[int]*models.Task{
		1: {ShortID: 1, Title: "First task"},
		2: {ShortID: 2, Title: "Second task"},
		3: {ShortID: 3, Title: "Third task"},
	}

	output := "#1\t📋\t⬜\tFirst task\n#3\t📋\t⬜\tThird task"

	var selectedTasks []*models.Task
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) > 0 {
			idStr := strings.TrimPrefix(parts[0], "#")
			var shortID int
			if _, err := fmt.Sscanf(idStr, "%d", &shortID); err == nil {
				if task, ok := taskMap[shortID]; ok {
					selectedTasks = append(selectedTasks, task)
				}
			}
		}
	}

	if len(selectedTasks) != 2 {
		t.Errorf("expected 2 selected tasks, got %d", len(selectedTasks))
	}
	if selectedTasks[0].ShortID != 1 {
		t.Errorf("expected first selected task to have ShortID 1, got %d", selectedTasks[0].ShortID)
	}
	if selectedTasks[1].ShortID != 3 {
		t.Errorf("expected second selected task to have ShortID 3, got %d", selectedTasks[1].ShortID)
	}
}
