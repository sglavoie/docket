package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
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

func TestParseQuickEditClearPriorityWithBangZero(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Original", Priority: models.PriorityHigh}
	project := &models.Project{Name: "test", Tasks: []*models.Task{task}}

	result, err := parseQuickEdit("Updated title !0", task, project)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	if !result.prioritySet || result.priority != models.PriorityUnset {
		t.Fatalf("priority = %d set=%v, want Unset", result.priority, result.prioritySet)
	}
	req := taskLineEditRequest(taskref.TaskIdentity{ProjectID: "project", TaskID: task.ID}, result)
	if req.Priority == nil || *req.Priority != models.PriorityUnset {
		t.Fatalf("request priority = %v, want Unset", req.Priority)
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
		t.Errorf("tagsToRemove = %v, want [old]", result.tagsToRemove)
	}
	if result.title != "Task" {
		t.Errorf("title = %q, want Task", result.title)
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

func TestParseQuickEdit_State(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", WorkflowState: models.StateBacklog}

	result, err := parseQuickEdit("Task $doing", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.stateSet || result.state != models.StateDoing {
		t.Errorf("expected state doing, got %s (set=%v)", result.state, result.stateSet)
	}
}

func TestParseQuickEdit_RetiredSectionTokenRemainsTitleText(t *testing.T) {
	project := &models.Project{
		Name: "test",
	}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task /inbox", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.title != "Task /inbox" {
		t.Errorf("retired section token parsed as metadata: title=%q", result.title)
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
		Name: "test",
	}
	task := &models.Task{Title: "Original", Priority: 3}

	result, err := parseQuickEdit("New title !2 #urgent @tomorrow $doing", task, project)
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
	if result.state != models.StateDoing {
		t.Errorf("expected state doing, got %s", result.state)
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
		t.Errorf("tagsToRemove = %v, want [old-tag]", result.tagsToRemove)
	}
	if result.title != "Task" {
		t.Errorf("expected Task title, got %q", result.title)
	}
}

func TestParseQuickEdit_DottedTags(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task #v2.0", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "v2-0" {
		t.Errorf("expected canonical tagsToAdd=['v2-0'], got %v", result.tagsToAdd)
	}
	if result.title != "Task" {
		t.Errorf("expected title 'Task', got '%s'", result.title)
	}
}

func TestParseQuickEditSingletonAndConflictErrors(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Original", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task}}
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "priority", input: "Task !1 !2", want: "duplicate priority"},
		{name: "due", input: "Task @today @tomorrow", want: "duplicate due"},
		{name: "destination", input: "Task $today $next", want: "duplicate destination"},
		{name: "project", input: "Task %project %other", want: "duplicate project"},
		{name: "parent", input: "Task >0 >1", want: "duplicate parent"},
		{name: "tag add remove", input: "Task #same -#same", want: "conflicting tag patch"},
		{name: "unknown priority", input: "Task !9", want: "invalid priority: !9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseQuickEdit(tt.input, task, project, []*models.Project{project})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parse error = %v, want %q", err, tt.want)
			}
		})
	}
}
