package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSubtreeHasMatch_MatchingSelf(t *testing.T) {
	task := testutil.NewTestTaskWithStatus("Active", models.StatusTodo)
	task.Children = []*models.Task{}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if !subtreeHasMatch(task, matches) {
		t.Error("expected active task to match")
	}
}

func TestSubtreeHasMatch_NonMatchingSelfNoChildren(t *testing.T) {
	task := testutil.NewTestTaskWithStatus("Done", models.StatusDone)
	task.Children = []*models.Task{}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if subtreeHasMatch(task, matches) {
		t.Error("expected completed leaf task to not match")
	}
}

func TestSubtreeHasMatch_NonMatchingParentWithMatchingChild(t *testing.T) {
	parent := testutil.NewTestTaskWithStatus("Done Epic", models.StatusDone)
	parent.Type = models.TaskTypeEpic

	child := testutil.NewTestTaskWithStatus("Active Task", models.StatusInProgress)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if !subtreeHasMatch(parent, matches) {
		t.Error("expected completed parent with active child to match via subtree")
	}
}

func TestSubtreeHasMatch_NonMatchingParentWithMatchingGrandchild(t *testing.T) {
	grandparent := testutil.NewTestTaskWithStatus("Done Epic", models.StatusDone)
	grandparent.Type = models.TaskTypeEpic

	parent := testutil.NewTestTaskWithStatus("Done Story", models.StatusDone)
	parent.Type = models.TaskTypeStory

	child := testutil.NewTestTaskWithStatus("Active Task", models.StatusTodo)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}
	grandparent.Children = []*models.Task{parent}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if !subtreeHasMatch(grandparent, matches) {
		t.Error("expected completed grandparent with active grandchild to match via subtree")
	}
}

func TestSubtreeHasMatch_AllNonMatching(t *testing.T) {
	parent := testutil.NewTestTaskWithStatus("Done Epic", models.StatusDone)
	parent.Type = models.TaskTypeEpic

	child := testutil.NewTestTaskWithStatus("Done Task", models.StatusDone)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}

	matches := func(t *models.Task) bool { return !t.Status.IsComplete() }

	if subtreeHasMatch(parent, matches) {
		t.Error("expected all-completed subtree to not match")
	}
}
