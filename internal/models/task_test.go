package models

import (
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	task := NewTask("test-id", 1, TaskTypeTask, "Test Task")

	if task.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", task.ID)
	}
	if task.ShortID != 1 {
		t.Errorf("expected ShortID 1, got %d", task.ShortID)
	}
	if task.Type != TaskTypeTask {
		t.Errorf("expected Type TaskTypeTask, got %s", task.Type)
	}
	if task.Title != "Test Task" {
		t.Errorf("expected Title 'Test Task', got '%s'", task.Title)
	}
	if task.Status != StatusTodo {
		t.Errorf("expected Status StatusTodo, got %s", task.Status)
	}
	if task.Priority != 3 {
		t.Errorf("expected Priority 3, got %d", task.Priority)
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestTaskStatus(t *testing.T) {
	tests := []struct {
		status     TaskStatus
		isComplete bool
		isActive   bool
		icon       string
	}{
		{StatusTodo, false, true, "○"},
		{StatusInProgress, false, true, "◐"},
		{StatusDone, true, false, "●"},
		{StatusBlocked, false, true, "◈"},
		{StatusCancelled, true, false, "✕"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			if tc.status.IsComplete() != tc.isComplete {
				t.Errorf("%s.IsComplete() = %v, want %v", tc.status, tc.status.IsComplete(), tc.isComplete)
			}
			if tc.status.IsActive() != tc.isActive {
				t.Errorf("%s.IsActive() = %v, want %v", tc.status, tc.status.IsActive(), tc.isActive)
			}
			if tc.status.Icon() != tc.icon {
				t.Errorf("%s.Icon() = %s, want %s", tc.status, tc.status.Icon(), tc.icon)
			}
		})
	}
}

func TestTaskStatusTransitions(t *testing.T) {
	task := NewTask("test", 1, TaskTypeTask, "Test")

	// Start
	task.Start()
	if task.Status != StatusInProgress {
		t.Errorf("after Start(), Status = %s, want in_progress", task.Status)
	}

	// Block
	task.Block()
	if task.Status != StatusBlocked {
		t.Errorf("after Block(), Status = %s, want blocked", task.Status)
	}

	// Complete
	task.Complete()
	if task.Status != StatusDone {
		t.Errorf("after Complete(), Status = %s, want done", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("after Complete(), CompletedAt should be set")
	}

	// Reopen
	task.Reopen()
	if task.Status != StatusTodo {
		t.Errorf("after Reopen(), Status = %s, want todo", task.Status)
	}
	if task.CompletedAt != nil {
		t.Error("after Reopen(), CompletedAt should be nil")
	}

	// Cancel
	task.Cancel()
	if task.Status != StatusCancelled {
		t.Errorf("after Cancel(), Status = %s, want cancelled", task.Status)
	}
	if task.CompletedAt == nil {
		t.Error("after Cancel(), CompletedAt should be set")
	}
}

func TestTaskTags(t *testing.T) {
	task := NewTask("test", 1, TaskTypeTask, "Test")

	// Add tag
	task.AddTag("backend")
	if !task.HasTag("backend") {
		t.Error("expected task to have tag 'backend'")
	}
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(task.Tags))
	}

	// Add duplicate tag (should not add)
	task.AddTag("backend")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after duplicate add, got %d", len(task.Tags))
	}

	// Add another tag
	task.AddTag("urgent")
	if len(task.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(task.Tags))
	}

	// Remove tag
	task.RemoveTag("backend")
	if task.HasTag("backend") {
		t.Error("expected task to not have tag 'backend' after removal")
	}
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after removal, got %d", len(task.Tags))
	}

	// Remove non-existent tag (should not error)
	task.RemoveTag("nonexistent")
	if len(task.Tags) != 1 {
		t.Errorf("expected 1 tag after removing non-existent, got %d", len(task.Tags))
	}
}

func TestTaskProgress(t *testing.T) {
	parent := NewTask("parent", 1, TaskTypeEpic, "Parent Epic")

	// Leaf task returns 0 or 100
	if parent.GetProgress() != 0 {
		t.Errorf("expected 0 progress for incomplete leaf, got %f", parent.GetProgress())
	}

	parent.Complete()
	if parent.GetProgress() != 100 {
		t.Errorf("expected 100 progress for complete leaf, got %f", parent.GetProgress())
	}
	parent.Reopen()

	// Add children
	child1 := NewTask("child1", 2, TaskTypeTask, "Child 1")
	child2 := NewTask("child2", 3, TaskTypeTask, "Child 2")

	parent.AddChild(child1)
	parent.AddChild(child2)

	// 0% complete
	if parent.GetProgress() != 0 {
		t.Errorf("expected 0 progress with no complete children, got %f", parent.GetProgress())
	}

	// 50% complete
	child1.Complete()
	if parent.GetProgress() != 50 {
		t.Errorf("expected 50 progress with 1/2 complete, got %f", parent.GetProgress())
	}

	// 100% complete
	child2.Complete()
	if parent.GetProgress() != 100 {
		t.Errorf("expected 100 progress with 2/2 complete, got %f", parent.GetProgress())
	}
}

func TestTaskHierarchy(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")

	// Test IsRoot and IsLeaf
	if !epic.IsRoot() {
		t.Error("epic should be root")
	}
	if !epic.IsLeaf() {
		t.Error("epic should be leaf initially")
	}

	// Add child
	epic.AddChild(story)
	if story.Parent == nil || story.Parent.ID != epic.ID {
		t.Error("story's parent should be epic")
	}
	if story.IsRoot() {
		t.Error("story should not be root")
	}
	if epic.IsLeaf() {
		t.Error("epic should not be leaf after adding child")
	}
	if story.Level != 1 {
		t.Errorf("story level should be 1, got %d", story.Level)
	}

	// Add grandchild
	story.AddChild(task)
	if task.Level != 2 {
		t.Errorf("task level should be 2, got %d", task.Level)
	}
}

func TestTaskCanAddChild(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")
	bug := NewTask("bug", 4, TaskTypeBug, "Bug")
	idea := NewTask("idea", 5, TaskTypeIdea, "Idea")

	// Epic can have story, task, bug, idea
	if !epic.CanAddChild(TaskTypeStory) {
		t.Error("epic should accept story child")
	}
	if !epic.CanAddChild(TaskTypeTask) {
		t.Error("epic should accept task child")
	}
	if !epic.CanAddChild(TaskTypeBug) {
		t.Error("epic should accept bug child")
	}
	if !epic.CanAddChild(TaskTypeIdea) {
		t.Error("epic should accept idea child")
	}
	if epic.CanAddChild(TaskTypeEpic) {
		t.Error("epic should not accept epic child")
	}

	// Story can have task, bug
	if !story.CanAddChild(TaskTypeTask) {
		t.Error("story should accept task child")
	}
	if !story.CanAddChild(TaskTypeBug) {
		t.Error("story should accept bug child")
	}
	if story.CanAddChild(TaskTypeStory) {
		t.Error("story should not accept story child")
	}

	// Leaf types cannot have children
	if task.CanAddChild(TaskTypeTask) {
		t.Error("task should not accept children")
	}
	if bug.CanAddChild(TaskTypeTask) {
		t.Error("bug should not accept children")
	}
	if idea.CanAddChild(TaskTypeTask) {
		t.Error("idea should not accept children")
	}
}

func TestTaskCountChildren(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story1 := NewTask("story1", 2, TaskTypeStory, "Story 1")
	story2 := NewTask("story2", 3, TaskTypeStory, "Story 2")
	task1 := NewTask("task1", 4, TaskTypeTask, "Task 1")
	task2 := NewTask("task2", 5, TaskTypeTask, "Task 2")

	epic.AddChild(story1)
	epic.AddChild(story2)
	story1.AddChild(task1)
	story1.AddChild(task2)

	// Epic has 2 direct + 2 nested = 4 descendants
	if epic.CountChildren() != 4 {
		t.Errorf("expected 4 children, got %d", epic.CountChildren())
	}

	// Story1 has 2 children
	if story1.CountChildren() != 2 {
		t.Errorf("expected 2 children for story1, got %d", story1.CountChildren())
	}

	// Task1 has no children
	if task1.CountChildren() != 0 {
		t.Errorf("expected 0 children for task1, got %d", task1.CountChildren())
	}
}

func TestTaskCountCompletedChildren(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	task1 := NewTask("task1", 2, TaskTypeTask, "Task 1")
	task2 := NewTask("task2", 3, TaskTypeTask, "Task 2")
	task3 := NewTask("task3", 4, TaskTypeTask, "Task 3")

	epic.AddChild(task1)
	epic.AddChild(task2)
	epic.AddChild(task3)

	if epic.CountCompletedChildren() != 0 {
		t.Errorf("expected 0 completed, got %d", epic.CountCompletedChildren())
	}

	task1.Complete()
	if epic.CountCompletedChildren() != 1 {
		t.Errorf("expected 1 completed, got %d", epic.CountCompletedChildren())
	}

	task2.Complete()
	task3.Cancel()
	if epic.CountCompletedChildren() != 3 {
		t.Errorf("expected 3 completed (including cancelled), got %d", epic.CountCompletedChildren())
	}
}

func TestParseTaskStatus(t *testing.T) {
	tests := []struct {
		input string
		want  TaskStatus
		valid bool
	}{
		{"todo", StatusTodo, true},
		{"in_progress", StatusInProgress, true},
		{"done", StatusDone, true},
		{"blocked", StatusBlocked, true},
		{"cancelled", StatusCancelled, true},
		{"invalid", TaskStatus("invalid"), false},
		{"", TaskStatus(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, valid := ParseTaskStatus(tc.input)
			if got != tc.want {
				t.Errorf("ParseTaskStatus(%s) = %s, want %s", tc.input, got, tc.want)
			}
			if valid != tc.valid {
				t.Errorf("ParseTaskStatus(%s) valid = %v, want %v", tc.input, valid, tc.valid)
			}
		})
	}
}

func TestTaskUpdatedAt(t *testing.T) {
	task := NewTask("test", 1, TaskTypeTask, "Test")
	originalUpdate := task.UpdatedAt

	// Wait a tiny bit to ensure time difference
	time.Sleep(time.Millisecond)

	task.Start()
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after Start()")
	}

	originalUpdate = task.UpdatedAt
	time.Sleep(time.Millisecond)

	task.AddTag("test")
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after AddTag()")
	}
}

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
			name: "epic with idea children cannot demote",
			setup: func() *Task {
				epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
				idea := NewTask("idea", 2, TaskTypeIdea, "Idea")
				epic.AddChild(idea)
				return epic
			},
			allowed: false,
			reason:  "idea cannot be child of story",
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
			name: "task under story cannot cycle to idea",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				task := NewTask("task", 2, TaskTypeTask, "Task")
				story.AddChild(task)
				return task
			},
			left:    true,
			allowed: false,
			reason:  "idea cannot be child of story",
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
			name: "bug under story cannot cycle right to idea",
			setup: func() *Task {
				story := NewTask("story", 1, TaskTypeStory, "Story")
				bug := NewTask("bug", 2, TaskTypeBug, "Bug")
				story.AddChild(bug)
				return bug
			},
			left:    false,
			allowed: false,
			reason:  "idea cannot be child of story",
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

func TestCanIndent(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")
	bug := NewTask("bug", 4, TaskTypeBug, "Bug")

	// Task can indent under Epic
	ok, reason := task.CanIndent(epic)
	if !ok {
		t.Errorf("task should be able to indent under epic: %s", reason)
	}

	// Task can indent under Story
	ok, reason = task.CanIndent(story)
	if !ok {
		t.Errorf("task should be able to indent under story: %s", reason)
	}

	// Task cannot indent under another Task (leaf type)
	ok, _ = task.CanIndent(bug)
	if ok {
		t.Error("task should not be able to indent under bug (leaf type)")
	}

	// Story can indent under Epic
	ok, reason = story.CanIndent(epic)
	if !ok {
		t.Errorf("story should be able to indent under epic: %s", reason)
	}

	// Story cannot indent under another Story
	story2 := NewTask("story2", 5, TaskTypeStory, "Story 2")
	ok, _ = story.CanIndent(story2)
	if ok {
		t.Error("story should not be able to indent under another story")
	}

	// Epic cannot indent under Story
	ok, _ = epic.CanIndent(story)
	if ok {
		t.Error("epic should not be able to indent under story")
	}

	// Cannot indent with nil parent
	ok, _ = task.CanIndent(nil)
	if ok {
		t.Error("should not be able to indent with nil parent")
	}
}

func TestCanOutdent(t *testing.T) {
	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")

	// Set up hierarchy: epic -> story -> task
	epic.AddChild(story)
	story.AddChild(task)

	// Task can outdent (would become child of epic)
	ok, reason := task.CanOutdent()
	if !ok {
		t.Errorf("task should be able to outdent: %s", reason)
	}

	// Story can outdent (would become root)
	ok, reason = story.CanOutdent()
	if !ok {
		t.Errorf("story should be able to outdent: %s", reason)
	}

	// Epic cannot outdent (already root)
	ok, _ = epic.CanOutdent()
	if ok {
		t.Error("epic should not be able to outdent (already root)")
	}

	// Story under Epic with Bug child - Bug can outdent to become Epic child
	epic2 := NewTask("epic2", 10, TaskTypeEpic, "Epic 2")
	story2 := NewTask("story2", 11, TaskTypeStory, "Story 2")
	bugUnderStory := NewTask("bug", 12, TaskTypeBug, "Bug")
	epic2.AddChild(story2)
	story2.AddChild(bugUnderStory)

	ok, reason = bugUnderStory.CanOutdent()
	if !ok {
		t.Errorf("bug under story should be able to outdent to epic: %s", reason)
	}
}

func TestCanOutdentInvalidHierarchy(t *testing.T) {
	// Create hierarchy where outdent would be invalid
	// Story -> Task, but Task promotes to Story, cannot be root Story's sibling
	story := NewTask("story", 1, TaskTypeStory, "Story")
	task := NewTask("task", 2, TaskTypeTask, "Task")
	story.AddChild(task)

	// Task under Story with no grandparent can outdent to root
	ok, _ := task.CanOutdent()
	if !ok {
		t.Error("task under story should be able to outdent to root")
	}
}

func TestTaskDependencies(t *testing.T) {
	task := NewTask("task", 1, TaskTypeTask, "Task")
	blocker1 := NewTask("blocker1", 2, TaskTypeTask, "Blocker 1")
	blocker2 := NewTask("blocker2", 3, TaskTypeTask, "Blocker 2")

	// Initially no dependencies
	if len(task.DependsOn) != 0 {
		t.Errorf("expected 0 dependencies initially, got %d", len(task.DependsOn))
	}
	if task.HasDependency(blocker1.ID) {
		t.Error("task should not have dependency on blocker1 initially")
	}

	// Add dependency
	task.AddDependency(blocker1.ID)
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after add, got %d", len(task.DependsOn))
	}
	if !task.HasDependency(blocker1.ID) {
		t.Error("task should have dependency on blocker1 after add")
	}

	// Add duplicate dependency (should not add)
	task.AddDependency(blocker1.ID)
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after duplicate add, got %d", len(task.DependsOn))
	}

	// Add second dependency
	task.AddDependency(blocker2.ID)
	if len(task.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(task.DependsOn))
	}
	if !task.HasDependency(blocker2.ID) {
		t.Error("task should have dependency on blocker2")
	}

	// Remove dependency
	task.RemoveDependency(blocker1.ID)
	if task.HasDependency(blocker1.ID) {
		t.Error("task should not have dependency on blocker1 after removal")
	}
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after removal, got %d", len(task.DependsOn))
	}

	// Remove non-existent dependency (should not error)
	task.RemoveDependency("nonexistent")
	if len(task.DependsOn) != 1 {
		t.Errorf("expected 1 dependency after removing non-existent, got %d", len(task.DependsOn))
	}

	// Clear dependencies
	task.ClearDependencies()
	if len(task.DependsOn) != 0 {
		t.Errorf("expected 0 dependencies after clear, got %d", len(task.DependsOn))
	}
}

func TestTaskDependencyUpdatesTimestamp(t *testing.T) {
	task := NewTask("task", 1, TaskTypeTask, "Task")
	originalUpdate := task.UpdatedAt
	time.Sleep(time.Millisecond)

	task.AddDependency("blocker-id")
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after AddDependency()")
	}

	originalUpdate = task.UpdatedAt
	time.Sleep(time.Millisecond)

	task.RemoveDependency("blocker-id")
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after RemoveDependency()")
	}

	task.AddDependency("another-blocker")
	originalUpdate = task.UpdatedAt
	time.Sleep(time.Millisecond)

	task.ClearDependencies()
	if !task.UpdatedAt.After(originalUpdate) {
		t.Error("UpdatedAt should be updated after ClearDependencies()")
	}
}
