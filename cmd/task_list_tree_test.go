package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSubtreeHasMatch_MatchingSelf(t *testing.T) {
	task := testutil.NewTestTaskWithState("Active", models.StateReady)
	task.Children = []*models.Task{}

	matches := func(t *models.Task) bool { return !t.IsComplete() }

	if !subtreeHasMatch(task, matches) {
		t.Error("expected active task to match")
	}
}

func TestSubtreeHasMatch_NonMatchingSelfNoChildren(t *testing.T) {
	task := testutil.NewTestTaskWithState("Done", models.StateDone)
	task.Children = []*models.Task{}

	matches := func(t *models.Task) bool { return !t.IsComplete() }

	if subtreeHasMatch(task, matches) {
		t.Error("expected completed leaf task to not match")
	}
}

func TestSubtreeHasMatch_NonMatchingParentWithMatchingChild(t *testing.T) {
	parent := testutil.NewTestTaskWithState("Done Epic", models.StateDone)
	parent.Type = models.TaskTypeEpic

	child := testutil.NewTestTaskWithState("Active Task", models.StateInProgress)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}

	matches := func(t *models.Task) bool { return !t.IsComplete() }

	if !subtreeHasMatch(parent, matches) {
		t.Error("expected completed parent with active child to match via subtree")
	}
}

func TestSubtreeHasMatch_NonMatchingParentWithMatchingGrandchild(t *testing.T) {
	grandparent := testutil.NewTestTaskWithState("Done Epic", models.StateDone)
	grandparent.Type = models.TaskTypeEpic

	parent := testutil.NewTestTaskWithState("Done Story", models.StateDone)
	parent.Type = models.TaskTypeStory

	child := testutil.NewTestTaskWithState("Active Task", models.StateReady)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}
	grandparent.Children = []*models.Task{parent}

	matches := func(t *models.Task) bool { return !t.IsComplete() }

	if !subtreeHasMatch(grandparent, matches) {
		t.Error("expected completed grandparent with active grandchild to match via subtree")
	}
}

func TestSubtreeHasMatch_AllNonMatching(t *testing.T) {
	parent := testutil.NewTestTaskWithState("Done Epic", models.StateDone)
	parent.Type = models.TaskTypeEpic

	child := testutil.NewTestTaskWithState("Done Task", models.StateDone)
	child.Children = []*models.Task{}

	parent.Children = []*models.Task{child}

	matches := func(t *models.Task) bool { return !t.IsComplete() }

	if subtreeHasMatch(parent, matches) {
		t.Error("expected all-completed subtree to not match")
	}
}
