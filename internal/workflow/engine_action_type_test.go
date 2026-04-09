package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplyAction_SetTypeHierarchy(t *testing.T) {
	t.Run("rejects type change invalid under parent", func(t *testing.T) {
		story := &models.Task{ID: "s1", Type: models.TaskTypeStory, Children: []*models.Task{}}
		task := &models.Task{ID: "t1", Type: models.TaskTypeTask, Parent: story, Children: []*models.Task{}}
		story.Children = append(story.Children, task)

		modified := applyAction(task, RuleAction{ActionType: ActionSetType, Value: "epic"})
		if modified {
			t.Error("expected set_type to epic to be rejected under story parent")
		}
		if task.Type != models.TaskTypeTask {
			t.Errorf("type should remain task, got %s", task.Type)
		}
	})

	t.Run("rejects type change to leaf when task has children", func(t *testing.T) {
		child := &models.Task{ID: "c1", Type: models.TaskTypeTask, Children: []*models.Task{}}
		story := &models.Task{ID: "s1", Type: models.TaskTypeStory, Children: []*models.Task{child}}
		child.Parent = story

		modified := applyAction(story, RuleAction{ActionType: ActionSetType, Value: "task"})
		if modified {
			t.Error("expected set_type to task to be rejected when task has children")
		}
		if story.Type != models.TaskTypeStory {
			t.Errorf("type should remain story, got %s", story.Type)
		}
	})

	t.Run("rejects type change when children invalid under new type", func(t *testing.T) {
		storyChild := &models.Task{ID: "sc1", Type: models.TaskTypeStory, Children: []*models.Task{}}
		epic := &models.Task{ID: "e1", Type: models.TaskTypeEpic, Children: []*models.Task{storyChild}}
		storyChild.Parent = epic

		modified := applyAction(epic, RuleAction{ActionType: ActionSetType, Value: "story"})
		if modified {
			t.Error("expected set_type to story to be rejected when epic has story children")
		}
		if epic.Type != models.TaskTypeEpic {
			t.Errorf("type should remain epic, got %s", epic.Type)
		}
	})

	t.Run("allows valid type change with parent", func(t *testing.T) {
		story := &models.Task{ID: "s1", Type: models.TaskTypeStory, Children: []*models.Task{}}
		task := &models.Task{ID: "t1", Type: models.TaskTypeTask, Parent: story, Children: []*models.Task{}}
		story.Children = append(story.Children, task)

		modified := applyAction(task, RuleAction{ActionType: ActionSetType, Value: "bug"})
		if !modified {
			t.Error("expected set_type to bug to be allowed under story parent")
		}
		if task.Type != models.TaskTypeBug {
			t.Errorf("type should be bug, got %s", task.Type)
		}
	})

	t.Run("allows valid type change at root", func(t *testing.T) {
		task := &models.Task{ID: "t1", Type: models.TaskTypeTask, Children: []*models.Task{}}
		modified := applyAction(task, RuleAction{ActionType: ActionSetType, Value: "epic"})
		if !modified {
			t.Error("expected set_type to epic to be allowed at root")
		}
		if task.Type != models.TaskTypeEpic {
			t.Errorf("type should be epic, got %s", task.Type)
		}
	})

	t.Run("clears type attrs on type change", func(t *testing.T) {
		task := &models.Task{
			ID:       "t1",
			Type:     models.TaskTypeBug,
			BugAttrs: &models.BugAttrs{Severity: "high"},
			Children: []*models.Task{},
		}
		modified := applyAction(task, RuleAction{ActionType: ActionSetType, Value: "idea"})
		if !modified {
			t.Error("expected modification")
		}
		if task.BugAttrs != nil {
			t.Error("expected BugAttrs to be cleared after type change")
		}
	})
}
