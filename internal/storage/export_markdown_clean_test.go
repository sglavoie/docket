package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestWriteProjectMarkdownClean_EmptyProject(t *testing.T) {
	p := newProject("Empty")
	out := WriteProjectMarkdownClean(p)

	if !strings.HasPrefix(out, "# Project: Empty\n") {
		t.Errorf("expected project header, got: %q", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line for empty project, got %d: %q", len(lines), out)
	}
}

func TestWriteProjectMarkdownClean_SingleTask(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Do the thing")
	task.Priority = 0
	p := newProject("My Project", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Task: (#1) Do the thing") {
		t.Errorf("expected task heading, got: %q", out)
	}
	if strings.Contains(out, "[") {
		t.Errorf("expected no badge for todo/zero-priority task, got: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NestedHierarchy(t *testing.T) {
	epic := mustTask("e1", 1, models.TaskTypeEpic, "Big Epic")
	story := mustTask("s1", 2, models.TaskTypeStory, "A Story")
	story.ParentID = &epic.ID
	task := mustTask("t1", 3, models.TaskTypeTask, "Leaf Task")
	task.ParentID = &story.ID

	p := newProject("Nested", epic, story, task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Epic: (#1) Big Epic") {
		t.Errorf("missing epic heading: %q", out)
	}
	if !strings.Contains(out, "### Story: (#2) A Story") {
		t.Errorf("missing story heading: %q", out)
	}
	if !strings.Contains(out, "#### Task: (#3) Leaf Task") {
		t.Errorf("missing task heading: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Orphan(t *testing.T) {
	parentID := "missing-parent"
	orphan := mustTask("o1", 5, models.TaskTypeTask, "Orphan Task")
	orphan.ParentID = &parentID

	p := newProject("WithOrphan", orphan)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Task: (#5) Orphan Task") {
		t.Errorf("expected orphan to appear at root level: %q", out)
	}
}

func TestWriteProjectMarkdownClean_DeepNestingClamped(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	tasks := make([]*models.Task, len(ids))
	for i, id := range ids {
		tasks[i] = mustTask(id, i+1, models.TaskTypeTask, "Level "+id)
		if i > 0 {
			parentID := ids[i-1]
			tasks[i].ParentID = &parentID
		}
	}

	p := newProject("Deep", tasks...)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "#######") {
		t.Errorf("headings were not clamped at ######: %q", out)
	}
	if !strings.Contains(out, "###### Task: (#7)") {
		t.Errorf("expected level-7+ task at ###### heading: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Tags(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Tagged")
	task.Tags = []string{"backend", "auth"}
	p := newProject("Tags", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "tags: backend, auth") {
		t.Errorf("expected tags line: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NoTagsWhenEmpty(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "No Tags")
	task.Tags = []string{}
	p := newProject("NoTags", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "tags:") {
		t.Errorf("expected no tags line for empty tags: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Priority(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "High Priority")
	task.Priority = 1
	p := newProject("Prio", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "!1") {
		t.Errorf("expected priority in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_ZeroPriorityNoBadge(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Zero Prio")
	task.Priority = 0
	p := newProject("ZeroPrio", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "!0") {
		t.Errorf("zero priority should not appear in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_DueDate(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Has Due")
	due := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	task.DueDate = &due
	task.Priority = 0
	p := newProject("Due", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "due 2026-04-20") {
		t.Errorf("expected due date in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_SnoozedUntil(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Snoozed")
	snooze := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	task.SnoozedUntil = &snooze
	task.Priority = 0
	p := newProject("Snooze", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "snoozed until 2026-05-01") {
		t.Errorf("expected snooze date in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NonTodoStatus(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "In Progress")
	task.Status = models.StatusInProgress
	task.Priority = 0
	p := newProject("Status", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "in_progress") {
		t.Errorf("expected non-todo status in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_TodoStatusNoBadge(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Todo")
	task.Status = models.StatusTodo
	task.Priority = 0
	p := newProject("TodoStatus", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "[") {
		t.Errorf("todo status should not produce a badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_EmptyDescription(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "No Desc")
	task.Description = ""
	p := newProject("NoDesc", task)
	out := WriteProjectMarkdownClean(p)

	lines := strings.Split(out, "\n")
	taskHeadingFound := false
	for i, line := range lines {
		if strings.Contains(line, "## Task: (#1) No Desc") {
			taskHeadingFound = true
			if i+1 < len(lines) && lines[i+1] == "" {
				if i+2 < len(lines) && lines[i+2] != "" && !strings.HasPrefix(lines[i+2], "#") {
					t.Errorf("unexpected description content after empty description task: %v", lines[i+2])
				}
			}
			break
		}
	}
	if !taskHeadingFound {
		t.Errorf("task heading not found in output: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NoHTMLComments(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "Clean")
	task.Status = models.StatusInProgress
	task.Priority = 2
	task.Tags = []string{"x"}
	p := newProject("Clean", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "<!--") || strings.Contains(out, "-->") {
		t.Errorf("clean export must not contain HTML comments: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Description(t *testing.T) {
	task := mustTask("t1", 1, models.TaskTypeTask, "With Desc")
	task.Description = "This is the description.\nWith multiple lines."
	task.Priority = 0
	p := newProject("Desc", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "This is the description.\nWith multiple lines.") {
		t.Errorf("expected verbatim description in output: %q", out)
	}
}