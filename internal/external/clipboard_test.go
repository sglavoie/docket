package external

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

func TestBuildTaskTitleText(t *testing.T) {
	task := &models.Task{Title: "My Test Task"}
	text := task.Title

	if text != "My Test Task" {
		t.Errorf("expected 'My Test Task', got %q", text)
	}
}

func TestBuildTaskTitlesText(t *testing.T) {
	tasks := []*models.Task{
		{Title: "Task One"},
		{Title: "Task Two"},
		{Title: "Task Three"},
	}

	titles := make([]string, len(tasks))
	for i, task := range tasks {
		titles[i] = task.Title
	}
	text := strings.Join(titles, "\n")

	expected := "Task One\nTask Two\nTask Three"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestBuildTaskReferenceText(t *testing.T) {
	task := &models.Task{ShortID: 42, Title: "Important Task"}
	text := fmt.Sprintf("#%d: %s", task.ShortID, task.Title)

	expected := "#42: Important Task"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestBuildTaskDetailsText(t *testing.T) {
	task := &models.Task{Description: "This is the task description."}
	text := task.Description

	if text != "This is the task description." {
		t.Errorf("expected task description, got %q", text)
	}
}

func TestBuildTasksDetailsText(t *testing.T) {
	tasks := []*models.Task{
		{Description: "First description"},
		{Description: "Second description"},
	}

	var parts []string
	for _, task := range tasks {
		parts = append(parts, task.Description)
	}
	text := strings.Join(parts, "\n---\n\n")

	expected := "First description\n---\n\nSecond description"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestBuildTaskTitleAndDescriptionText(t *testing.T) {
	task := &models.Task{Title: "Task Title", Description: "Task Description"}
	text := task.Title + "\n\n" + task.Description

	expected := "Task Title\n\nTask Description"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestBuildTasksTitleAndDescriptionText(t *testing.T) {
	tasks := []*models.Task{
		{Title: "Title One", Description: "Description One"},
		{Title: "Title Two", Description: "Description Two"},
	}

	var parts []string
	for _, task := range tasks {
		parts = append(parts, task.Title+"\n\n"+task.Description)
	}
	text := strings.Join(parts, "\n---\n\n")

	expected := "Title One\n\nDescription One\n---\n\nTitle Two\n\nDescription Two"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestBuildEmptyTasksText(t *testing.T) {
	tasks := []*models.Task{}

	titles := make([]string, len(tasks))
	for i, task := range tasks {
		titles[i] = task.Title
	}
	text := strings.Join(titles, "\n")

	if text != "" {
		t.Errorf("expected empty string for empty tasks, got %q", text)
	}
}
