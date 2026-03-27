package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseQuickEdit_Priority(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Priority: 3}

	result, err := parseQuickEdit("Updated title !1", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.title != "Updated title" {
		t.Errorf("expected title 'Updated title', got '%s'", result.title)
	}
	if !result.prioritySet || result.priority != 1 {
		t.Errorf("expected priority 1, got %d (set=%v)", result.priority, result.prioritySet)
	}
}

func TestParseQuickEdit_Tags(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Tags: []string{"old"}}

	result, err := parseQuickEdit("Task #newtag -#old", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "newtag" {
		t.Errorf("expected tagsToAdd=['newtag'], got %v", result.tagsToAdd)
	}
	if len(result.tagsToRemove) != 1 || result.tagsToRemove[0] != "old" {
		t.Errorf("expected tagsToRemove=['old'], got %v", result.tagsToRemove)
	}
}

func TestParseQuickEdit_DueDate(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task @tomorrow", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.dueDateSet || result.dueDate == nil {
		t.Error("expected due date to be set")
	}
}

func TestParseQuickEdit_ClearDueDate(t *testing.T) {
	project := &models.Project{Name: "test"}
	now := time.Now()
	task := &models.Task{Title: "Original", DueDate: &now}

	result, err := parseQuickEdit("Task @0", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.dueDateSet || !result.dueDateClear {
		t.Error("expected due date clear to be set")
	}
}

func TestParseQuickEdit_Status(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Status: models.StatusTodo}

	result, err := parseQuickEdit("Task $start", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.statusSet || result.status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %s (set=%v)", result.status, result.statusSet)
	}
}

func TestParseQuickEdit_Section(t *testing.T) {
	project := &models.Project{
		Name:     "test",
		Sections: []string{"Inbox", "Backlog"},
	}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task /backlog", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.sectionSet || result.section != "Backlog" {
		t.Errorf("expected section 'Backlog', got '%s' (set=%v)", result.section, result.sectionSet)
	}
}

func TestParseQuickEdit_ClearSection(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Section: "Backlog"}

	result, err := parseQuickEdit("Task /active", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.sectionSet || result.section != "" {
		t.Errorf("expected empty section (Active), got '%s' (set=%v)", result.section, result.sectionSet)
	}
}

func TestParseQuickEdit_EmptyTitle(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original"}

	_, err := parseQuickEdit("!1 #tag", task, project)
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestParseQuickEdit_Combined(t *testing.T) {
	project := &models.Project{
		Name:     "test",
		Sections: []string{"Inbox"},
	}
	task := &models.Task{Title: "Original", Priority: 3}

	result, err := parseQuickEdit("New title !2 #urgent @tomorrow /inbox $start", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.title != "New title" {
		t.Errorf("expected title 'New title', got '%s'", result.title)
	}
	if result.priority != 2 {
		t.Errorf("expected priority 2, got %d", result.priority)
	}
	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "urgent" {
		t.Errorf("expected tag 'urgent', got %v", result.tagsToAdd)
	}
	if result.dueDate == nil {
		t.Error("expected due date to be set")
	}
	if result.section != "Inbox" {
		t.Errorf("expected section 'Inbox', got '%s'", result.section)
	}
	if result.status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", result.status)
	}
}

func TestParseQuickEdit_HyphenatedTags(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Tags: []string{"old-tag"}}

	result, err := parseQuickEdit("Task #new-tag -#old-tag", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "new-tag" {
		t.Errorf("expected tagsToAdd=['new-tag'], got %v", result.tagsToAdd)
	}
	if len(result.tagsToRemove) != 1 || result.tagsToRemove[0] != "old-tag" {
		t.Errorf("expected tagsToRemove=['old-tag'], got %v", result.tagsToRemove)
	}
	if result.title != "Task" {
		t.Errorf("expected title 'Task', got '%s'", result.title)
	}
}

func TestParseQuickEdit_DottedTags(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task #v2.0", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "v2.0" {
		t.Errorf("expected tagsToAdd=['v2.0'], got %v", result.tagsToAdd)
	}
	if result.title != "Task" {
		t.Errorf("expected title 'Task', got '%s'", result.title)
	}
}

func TestParseQuickEdit_StandbyStatus(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Status: models.StatusInProgress}

	result, err := parseQuickEdit("Task $standby", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.statusSet || result.status != models.StatusStandby {
		t.Errorf("expected status standby, got %s (set=%v)", result.status, result.statusSet)
	}

	result, err = parseQuickEdit("Task $hold", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.statusSet || result.status != models.StatusStandby {
		t.Errorf("expected status standby from $hold, got %s (set=%v)", result.status, result.statusSet)
	}
}

func TestApplyQuickEdit(t *testing.T) {
	task := &models.Task{
		ID:       "test-task-id",
		Title:    "Original",
		Priority: 3,
		Tags:     []string{"old"},
		Status:   models.StatusTodo,
	}
	project := &models.Project{
		Tasks: []*models.Task{task},
	}

	tomorrow := time.Now().AddDate(0, 0, 1)
	result := &quickEditResult{
		title:        "Updated",
		titleSet:     true,
		priority:     1,
		prioritySet:  true,
		tagsToAdd:    []string{"new"},
		tagsToRemove: []string{"old"},
		dueDate:      &tomorrow,
		dueDateSet:   true,
		status:       models.StatusInProgress,
		statusSet:    true,
	}

	if statusErr := applyQuickEdit(task, project, result); statusErr != "" {
		t.Fatalf("unexpected status error: %s", statusErr)
	}

	if task.Title != "Updated" {
		t.Errorf("expected title 'Updated', got '%s'", task.Title)
	}
	if task.Priority != 1 {
		t.Errorf("expected priority 1, got %d", task.Priority)
	}
	if len(task.Tags) != 1 || task.Tags[0] != "new" {
		t.Errorf("expected tags ['new'], got %v", task.Tags)
	}
	if task.DueDate == nil {
		t.Error("expected due date to be set")
	}
	if task.Status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", task.Status)
	}
}

func TestHasChanges(t *testing.T) {
	original := &quickEditResult{
		title:    "Original",
		priority: 3,
	}

	// No changes
	result := &quickEditResult{
		title:    "Original",
		titleSet: true,
	}
	if result.hasChanges(original) {
		t.Error("expected no changes for same title")
	}

	// Title changed
	result = &quickEditResult{
		title:    "Changed",
		titleSet: true,
	}
	if !result.hasChanges(original) {
		t.Error("expected changes for different title")
	}

	// Priority changed
	result = &quickEditResult{
		title:       "Original",
		titleSet:    true,
		priority:    1,
		prioritySet: true,
	}
	if !result.hasChanges(original) {
		t.Error("expected changes for different priority")
	}

	// Tags added
	result = &quickEditResult{
		title:     "Original",
		titleSet:  true,
		tagsToAdd: []string{"new"},
	}
	if !result.hasChanges(original) {
		t.Error("expected changes for added tags")
	}
}

func TestRenderQuickEditPreview(t *testing.T) {
	original := &quickEditResult{
		title:    "Original",
		priority: 3,
		section:  "Backlog",
	}

	// Priority change
	result := &quickEditResult{
		title:       "Original",
		titleSet:    true,
		priority:    1,
		prioritySet: true,
	}
	preview := renderQuickEditPreview(result, original)
	if preview != "-> !1" {
		t.Errorf("expected '-> !1', got '%s'", preview)
	}

	// Tags
	result = &quickEditResult{
		title:        "Original",
		titleSet:     true,
		tagsToAdd:    []string{"new"},
		tagsToRemove: []string{"old"},
	}
	preview = renderQuickEditPreview(result, original)
	if preview != "-> +#new -#old" {
		t.Errorf("expected '-> +#new -#old', got '%s'", preview)
	}

	// No changes
	result = &quickEditResult{
		title:    "Original",
		titleSet: true,
	}
	preview = renderQuickEditPreview(result, original)
	if preview != "(no changes)" {
		t.Errorf("expected '(no changes)', got '%s'", preview)
	}
}
