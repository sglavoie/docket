package models

import "testing"

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
				task, err := NewTask("task", 1, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				return task
			},
			allowed: true,
		},
		{
			name: "root story with task children can promote to epic",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				task, err := NewTask("task", 2, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(task); err != nil {
					t.Fatal(err)
				}
				return story
			},
			allowed: true,
		},
		{
			name: "epic cannot promote further",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				return epic
			},
			allowed: false,
			reason:  "already at top of hierarchy",
		},
		{
			name: "task under story cannot promote to story",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				task, err := NewTask("task", 2, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(task); err != nil {
					t.Fatal(err)
				}
				return task
			},
			allowed: false,
			reason:  "story cannot be child of story",
		},
		{
			name: "story under epic cannot promote to epic",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				story, err := NewTask("story", 2, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				if err := epic.AddChild(story); err != nil {
					t.Fatal(err)
				}
				return story
			},
			allowed: false,
			reason:  "epic cannot be child of epic",
		},
		{
			name: "bug under story can promote to story under epic grandparent",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				story, err := NewTask("story", 2, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				bug, err := NewTask("bug", 3, TaskTypeBug, "Bug")
				if err != nil {
					t.Fatal(err)
				}
				if err := epic.AddChild(story); err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(bug); err != nil {
					t.Fatal(err)
				}
				return bug
			},
			allowed: false,
			reason:  "story cannot be child of story",
		},
		{
			name: "task under epic can promote to story",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				task, err := NewTask("task", 2, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				if err := epic.AddChild(task); err != nil {
					t.Fatal(err)
				}
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
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				return epic
			},
			allowed: true,
		},
		{
			name: "story with no children can demote",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				return story
			},
			allowed: true,
		},
		{
			name: "task cannot demote further",
			setup: func() *Task {
				task, err := NewTask("task", 1, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				return task
			},
			allowed: false,
			reason:  "already at bottom of hierarchy",
		},
		{
			name: "bug cannot demote further",
			setup: func() *Task {
				bug, err := NewTask("bug", 1, TaskTypeBug, "Bug")
				if err != nil {
					t.Fatal(err)
				}
				return bug
			},
			allowed: false,
			reason:  "already at bottom of hierarchy",
		},
		{
			name: "epic with story children cannot demote",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				story, err := NewTask("story", 2, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				if err := epic.AddChild(story); err != nil {
					t.Fatal(err)
				}
				return epic
			},
			allowed: false,
			reason:  "story cannot be child of story",
		},
		{
			name: "epic with task children can demote to story",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				task, err := NewTask("task", 2, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				if err := epic.AddChild(task); err != nil {
					t.Fatal(err)
				}
				return epic
			},
			allowed: true,
		},
		{
			name: "story with task children cannot demote to task",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				task, err := NewTask("task", 2, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(task); err != nil {
					t.Fatal(err)
				}
				return story
			},
			allowed: false,
			reason:  "task cannot have children",
		},
		{
			name: "epic with idea children can demote",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				idea, err := NewTask("idea", 2, TaskTypeIdea, "Idea")
				if err != nil {
					t.Fatal(err)
				}
				if err := epic.AddChild(idea); err != nil {
					t.Fatal(err)
				}
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
				task, err := NewTask("task", 1, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				return task
			},
			left:    false,
			allowed: true,
		},
		{
			name: "root task can cycle left to idea",
			setup: func() *Task {
				task, err := NewTask("task", 1, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				return task
			},
			left:    true,
			allowed: true,
		},
		{
			name: "task under story can cycle to bug",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				task, err := NewTask("task", 2, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(task); err != nil {
					t.Fatal(err)
				}
				return task
			},
			left:    false,
			allowed: true,
		},
		{
			name: "task under story can cycle to idea",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				task, err := NewTask("task", 2, TaskTypeTask, "Task")
				if err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(task); err != nil {
					t.Fatal(err)
				}
				return task
			},
			left:    true,
			allowed: true,
		},
		{
			name: "bug under story can cycle to task",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				bug, err := NewTask("bug", 2, TaskTypeBug, "Bug")
				if err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(bug); err != nil {
					t.Fatal(err)
				}
				return bug
			},
			left:    true,
			allowed: true,
		},
		{
			name: "bug under story can cycle right to idea",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				bug, err := NewTask("bug", 2, TaskTypeBug, "Bug")
				if err != nil {
					t.Fatal(err)
				}
				if err := story.AddChild(bug); err != nil {
					t.Fatal(err)
				}
				return bug
			},
			left:    false,
			allowed: true,
		},
		{
			name: "idea under epic can cycle to any leaf type",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				idea, err := NewTask("idea", 2, TaskTypeIdea, "Idea")
				if err != nil {
					t.Fatal(err)
				}
				if err := epic.AddChild(idea); err != nil {
					t.Fatal(err)
				}
				return idea
			},
			left:    true,
			allowed: true,
		},
		{
			name: "epic cannot cycle leaf types",
			setup: func() *Task {
				epic, err := NewTask("epic", 1, TaskTypeEpic, "Epic")
				if err != nil {
					t.Fatal(err)
				}
				return epic
			},
			left:    false,
			allowed: false,
			reason:  "not a leaf type",
		},
		{
			name: "story cannot cycle leaf types",
			setup: func() *Task {
				story, err := NewTask("story", 1, TaskTypeStory, "Story")
				if err != nil {
					t.Fatal(err)
				}
				return story
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
