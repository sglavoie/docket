package models

import (
	"testing"
	"time"
)

func TestTaskCanPromote(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *Task
		allowed bool
		reason  string
	}{
		{
			name: "root task with no children can promote",
			setup: func() *Task {
				return NewTask("task", 1, TaskTypeTask, "Task")
			},
			allowed: true,
		},
		{
			name: "root story with task children can promote to epic",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				story.AddChild(task)
				return story
			},
			allowed: true,
		},
		{
			name: "epic cannot promote further",
			setup: func() *Task {
				return NewTask("epic", 1, TaskTypeEpic, "Epic")
			},
			allowed: false,
			reason:  "already at top of hierarchy",
		},
		{
			name: "task under story cannot promote to story",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				story.AddChild(task)
				return task
			},
			allowed: false,
			reason:  "story cannot be child of story",
		},
		{
			name: "story under epic cannot promote to epic",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				story := NewTask("story", 2, TaskTypeStory, "Story")
				epic.AddChild(story)
				return story
			},
			allowed: false,
			reason:  "epic cannot be child of epic",
		},
		{
			name: "bug under story can promote to story under epic grandparent",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				story := NewTask("story", 2, TaskTypeStory, "Story")
				bug := NewTask("bug", 3, TaskTypeBug, "Bug")
				epic.AddChild(story)
				story.AddChild(bug)
				return bug
			},
			allowed: false,
			reason:  "story cannot be child of story",
		},
		{
			name: "task under epic can promote to story",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				epic.AddChild(task)
				return task
			},
			allowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := tc.setup()
			allowed, reason := task.CanPromote()
			if allowed != tc.allowed {
				t.Errorf("CanPromote() = %v, want %v (reason: %s)", allowed, tc.allowed, reason)
			}
			if !allowed && tc.reason != "" && reason != tc.reason {
				t.Errorf("CanPromote() reason = %q, want %q", reason, tc.reason)
			}
		})
	}
}

func TestTaskCanDemote(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *Task
		allowed bool
		reason  string
	}{
		{
			name: "epic with no children can demote",
			setup: func() *Task {
				return NewTask("epic", 1, TaskTypeEpic, "Epic")
			},
			allowed: true,
		},
		{
			name: "story with no children can demote",
			setup: func() *Task {
				return NewTask("story", 1, TaskTypeStory, "Story")
			},
			allowed: true,
		},
		{
			name: "task cannot demote further",
			setup: func() *Task {
				return NewTask("task", 1, TaskTypeTask, "Task")
			},
			allowed: false,
			reason:  "already at bottom of hierarchy",
		},
		{
			name: "bug cannot demote further",
			setup: func() *Task {
				return NewTask("bug", 1, TaskTypeBug, "Bug")
			},
			allowed: false,
			reason:  "already at bottom of hierarchy",
		},
		{
			name: "epic with story children cannot demote",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				story := NewTask("story", 2, TaskTypeStory, "Story")
				epic.AddChild(story)
				return epic
			},
			allowed: false,
			reason:  "story cannot be child of story",
		},
		{
			name: "epic with task children can demote to story",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				epic.AddChild(task)
				return epic
			},
			allowed: true,
		},
		{
			name: "story with task children cannot demote to task",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				story.AddChild(task)
				return story
			},
			allowed: false,
			reason:  "task cannot have children",
		},
		{
			name: "epic with idea children can demote",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				idea := NewTask("idea", 2, TaskTypeIdea, "Idea")
				epic.AddChild(idea)
				return epic
			},
			allowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := tc.setup()
			allowed, reason := task.CanDemote()
			if allowed != tc.allowed {
				t.Errorf("CanDemote() = %v, want %v (reason: %s)", allowed, tc.allowed, reason)
			}
			if !allowed && tc.reason != "" && reason != tc.reason {
				t.Errorf("CanDemote() reason = %q, want %q", reason, tc.reason)
			}
		})
	}
}

func TestTaskCanCycleLeafType(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *Task
		left    bool
		allowed bool
		reason  string
	}{
		{
			name: "root task can cycle right to bug",
			setup: func() *Task {
				return NewTask("task", 1, TaskTypeTask, "Task")
			},
			left:    false,
			allowed: true,
		},
		{
			name: "root task can cycle left to idea",
			setup: func() *Task {
				return NewTask("task", 1, TaskTypeTask, "Task")
			},
			left:    true,
			allowed: true,
		},
		{
			name: "task under story can cycle to bug",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				story.AddChild(task)
				return task
			},
			left:    false,
			allowed: true,
		},
		{
			name: "task under story can cycle to idea",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				story.AddChild(task)
				return task
			},
			left:    true,
			allowed: true,
		},
		{
			name: "bug under story can cycle to task",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				bug := NewTask("bug", 2, TaskTypeBug, "Bug")
				story.AddChild(bug)
				return bug
			},
			left:    true,
			allowed: true,
		},
		{
			name: "bug under story can cycle right to idea",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				bug := NewTask("bug", 2, TaskTypeBug, "Bug")
				story.AddChild(bug)
				return bug
			},
			left:    false,
			allowed: true,
		},
		{
			name: "idea under epic can cycle to any leaf type",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				idea := NewTask("idea", 2, TaskTypeIdea, "Idea")
				epic.AddChild(idea)
				return idea
			},
			left:    true,
			allowed: true,
		},
		{
			name: "epic cannot cycle leaf types",
			setup: func() *Task {
				return NewTask("epic", 1, TaskTypeEpic, "Epic")
			},
			left:    false,
			allowed: false,
			reason:  "not a leaf type",
		},
		{
			name: "story cannot cycle leaf types",
			setup: func() *Task {
				return NewTask("story", 1, TaskTypeStory, "Story")
			},
			left:    true,
			allowed: false,
			reason:  "not a leaf type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := tc.setup()
			allowed, reason := task.CanCycleLeafType(tc.left)
			if allowed != tc.allowed {
				t.Errorf("CanCycleLeafType(%v) = %v, want %v (reason: %s)", tc.left, allowed, tc.allowed, reason)
			}
			if !allowed && tc.reason != "" && reason != tc.reason {
				t.Errorf("CanCycleLeafType(%v) reason = %q, want %q", tc.left, reason, tc.reason)
			}
		})
	}
}

func TestTaskPromote(t *testing.T) {
	task := NewTask("task", 1, TaskTypeTask, "Task")
	originalUpdate := task.UpdatedAt
	time.Sleep(time.Millisecond)

	err := task.Promote()
	if err != nil {
		t.Errorf("Promote() returned error: %v", err)
	}
	if task.Type != TaskTypeStory {
		t.Errorf("after Promote(), Type = %s, want story", task.Type)
	}
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after Promote()")
	}

	// Promote again to epic
	err = task.Promote()
	if err != nil {
		t.Errorf("second Promote() returned error: %v", err)
	}
	if task.Type != TaskTypeEpic {
		t.Errorf("after second Promote(), Type = %s, want epic", task.Type)
	}

	// Cannot promote epic
	err = task.Promote()
	if err == nil {
		t.Error("Promote() on epic should return error")
	}
}

func TestTaskDemote(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	originalUpdate := epic.UpdatedAt
	time.Sleep(time.Millisecond)

	err := epic.Demote()
	if err != nil {
		t.Errorf("Demote() returned error: %v", err)
	}
	if epic.Type != TaskTypeStory {
		t.Errorf("after Demote(), Type = %s, want story", epic.Type)
	}
	if !epic.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after Demote()")
	}

	// Demote again to task
	err = epic.Demote()
	if err != nil {
		t.Errorf("second Demote() returned error: %v", err)
	}
	if epic.Type != TaskTypeTask {
		t.Errorf("after second Demote(), Type = %s, want task", epic.Type)
	}

	// Cannot demote task
	err = epic.Demote()
	if err == nil {
		t.Error("Demote() on task should return error")
	}
}

func TestTaskCycleLeafType(t *testing.T) {
	task := NewTask("task", 1, TaskTypeTask, "Task")
	originalUpdate := task.UpdatedAt
	time.Sleep(time.Millisecond)

	// Cycle right: Task -> Bug
	err := task.CycleLeafType(false)
	if err != nil {
		t.Errorf("CycleLeafType(false) returned error: %v", err)
	}
	if task.Type != TaskTypeBug {
		t.Errorf("after CycleLeafType(false), Type = %s, want bug", task.Type)
	}
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after CycleLeafType()")
	}

	// Cycle right: Bug -> Idea
	err = task.CycleLeafType(false)
	if err != nil {
		t.Errorf("CycleLeafType(false) returned error: %v", err)
	}
	if task.Type != TaskTypeIdea {
		t.Errorf("after second CycleLeafType(false), Type = %s, want idea", task.Type)
	}

	// Cycle left: Idea -> Bug
	err = task.CycleLeafType(true)
	if err != nil {
		t.Errorf("CycleLeafType(true) returned error: %v", err)
	}
	if task.Type != TaskTypeBug {
		t.Errorf("after CycleLeafType(true), Type = %s, want bug", task.Type)
	}
}

func TestTaskPromoteWithInvalidHierarchy(t *testing.T) {
	// Story under epic with story child - cannot demote epic
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	epic.AddChild(story)

	err := epic.Demote()
	if err == nil {
		t.Error("Demote() epic with story child should return error")
	}

	// But task under story cannot promote because result would be invalid
	task := NewTask("task", 3, TaskTypeTask, "Task")
	story.AddChild(task)

	err = task.Promote()
	if err == nil {
		t.Error("Promote() task under story should return error")
	}
}
