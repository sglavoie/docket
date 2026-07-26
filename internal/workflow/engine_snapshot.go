package workflow

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// CopyTaskState creates a snapshot copy of the fields that event-based
// triggers inspect (state, priority, tags, type, due date, recurrence,
// dependencies, and parent identity). This is intentionally minimal — only fields
// checked by matchesEventTrigger, matchesHierarchicalTrigger, and
// TaskStateChanged are copied.
//
// All mutable fields are independent of the source task: Tags, DependsOn,
// and Recurrence (including its non-empty Weekdays slice) are deep-copied
// so that in-place mutations to the live task — including via slices.Delete
// which reuses the underlying array — do not bleed into the snapshot.
//
// The live Parent pointer and Children slice are deliberately NOT carried into
// the snapshot. The snapshot represents the task's intrinsic state at a
// point in time, not its hierarchy. Keeping a live Parent reference would
// allow a mutation to the parent (or an indexed rewrite of Children on the
// source) to retroactively change the snapshot, defeating its purpose.
// Hierarchical triggers always operate on the live task, never on the
// snapshot.
func CopyTaskState(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}

	tagsCopy := make([]string, len(task.Tags))
	copy(tagsCopy, task.Tags)

	var recCopy *models.RecurrencePattern
	if task.Recurrence != nil {
		rc := *task.Recurrence
		if len(rc.Weekdays) > 0 {
			rc.Weekdays = append([]int(nil), rc.Weekdays...)
		}
		recCopy = &rc
	}

	var dueCopy *time.Time
	if task.DueDate != nil {
		d := *task.DueDate
		dueCopy = &d
	}

	snapshot := &models.Task{
		ID: task.ID,
		// Event matching compares this canonical snapshot directly. Legacy
		// status/section triggers, while still decodable, are compatibility
		// adapters over the same value.
		WorkflowState: task.State(),
		Priority:      task.Priority,
		Tags:          tagsCopy,
		Type:          task.Type,
		DueDate:       dueCopy,
		Description:   task.Description,
		Recurrence:    recCopy,
		Level:         task.Level,
		DependsOn:     append([]string(nil), task.DependsOn...),
	}
	if task.ParentID != nil {
		parentID := *task.ParentID
		snapshot.ParentID = &parentID
	}
	return snapshot
}

// CaptureTaskStates creates a snapshot of task states for event trigger detection.
// Call this before making changes to tasks to capture their previous state.
func CaptureTaskStates(tasks []*models.Task) map[string]*models.Task {
	states := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		states[task.ID] = CopyTaskState(task)
	}
	return states
}

// TaskStateChanged returns true if the task's workflow-relevant state changed.
func TaskStateChanged(current, prev *models.Task) bool {
	if prev == nil {
		return true
	}
	// State() collapses the legacy status+section into one canonical value, so a
	// single comparison covers both former Status and Section change checks.
	if current.State() != prev.State() ||
		current.Priority != prev.Priority ||
		current.Type != prev.Type ||
		current.Description != prev.Description {
		return true
	}
	if (current.ParentID == nil) != (prev.ParentID == nil) ||
		(current.ParentID != nil && *current.ParentID != *prev.ParentID) {
		return true
	}
	// Detect due date changes (added, removed, or modified)
	switch {
	case current.DueDate == nil && prev.DueDate == nil:
		// no change
	case current.DueDate == nil || prev.DueDate == nil:
		return true // one is nil, the other is not
	default:
		if !current.DueDate.Equal(*prev.DueDate) {
			return true
		}
	}
	if len(current.Tags) != len(prev.Tags) {
		return true
	}
	for i, tag := range current.Tags {
		if tag != prev.Tags[i] {
			return true
		}
	}
	if len(current.DependsOn) != len(prev.DependsOn) {
		return true
	}
	for i, dep := range current.DependsOn {
		if dep != prev.DependsOn[i] {
			return true
		}
	}
	// Detect recurrence changes (added, cleared, or pattern/streak modified)
	// so event-based triggers like FieldRecurrenceCompleted and
	// FieldStreakBroken see the task as changed and re-evaluate.
	if recurrenceChanged(current.Recurrence, prev.Recurrence) {
		return true
	}
	return false
}

// recurrenceChanged reports whether two recurrence pointers represent
// a workflow-relevant change. It compares the full structural pattern
// (interval/unit/mode/weekdays as a set) plus Streak/Skipped counters
// that drive FieldStreakBroken. OriginalDay/MaxStreak are ignored
// because they don't drive any event trigger today.
func recurrenceChanged(current, prev *models.RecurrencePattern) bool {
	if current == nil && prev == nil {
		return false
	}
	if current == nil || prev == nil {
		return true
	}
	if current.Interval != prev.Interval ||
		current.Unit != prev.Unit ||
		current.Mode != prev.Mode ||
		current.Streak != prev.Streak ||
		current.Skipped != prev.Skipped {
		return true
	}
	if len(current.Weekdays) != len(prev.Weekdays) {
		return true
	}
	seen := make(map[int]int, len(current.Weekdays))
	for _, d := range current.Weekdays {
		seen[d]++
	}
	for _, d := range prev.Weekdays {
		seen[d]--
		if seen[d] < 0 {
			return true
		}
	}
	return false
}

// BuildEvaluationContext creates an evaluation context for workflow rules.
// Only tasks that actually changed (or are new) are included in Changes
// so that state-based rules don't re-fire on every operation.
// AllTasks and TaskMap still contain all tasks for hierarchical trigger lookups.
func BuildEvaluationContext(tasks []*models.Task, prevStates map[string]*models.Task, projectName, projectID string, forcedTaskIDs ...string) *EvaluationContext {
	taskMap := make(map[string]*models.Task, len(tasks))
	var changes []TaskChangeContext
	changedIDs := make(map[string]bool)
	forced := make(map[string]bool, len(forcedTaskIDs))
	for _, taskID := range forcedTaskIDs {
		forced[taskID] = true
	}

	for _, task := range tasks {
		if task == nil {
			continue
		}
		taskMap[task.ID] = task
		prev := prevStates[task.ID]
		if prev == nil || forced[task.ID] || TaskStateChanged(task, prev) {
			changes = append(changes, TaskChangeContext{
				Task:     task,
				Previous: prev,
			})
			changedIDs[task.ID] = true
		}
	}

	// A child's transition into or out of a terminal state changes the value of
	// hierarchical aggregate triggers on every ancestor. Enqueue those ancestors
	// even when none of their intrinsic fields changed. This also lets a rule that
	// completes a Story recursively enqueue its Epic.
	for _, change := range append([]TaskChangeContext(nil), changes...) {
		if change.Task == nil || change.Previous == nil ||
			change.Task.IsTerminalState() == change.Previous.IsTerminalState() {
			continue
		}
		appendAncestorChanges(change.Task, taskMap, prevStates, changedIDs, &changes)
	}

	return &EvaluationContext{
		Changes:         changes,
		AllTasks:        tasks,
		TaskMap:         taskMap,
		PreviousTaskMap: prevStates,
		ProjectName:     projectName,
		ProjectID:       projectID,
	}
}

func appendAncestorChanges(task *models.Task, taskMap, prevStates map[string]*models.Task, seen map[string]bool, changes *[]TaskChangeContext) {
	visited := make(map[string]bool)
	for task != nil && task.ParentID != nil {
		parentID := *task.ParentID
		if visited[parentID] {
			return
		}
		visited[parentID] = true
		parent := taskMap[parentID]
		if parent == nil {
			return
		}
		if !seen[parent.ID] {
			*changes = append(*changes, TaskChangeContext{Task: parent, Previous: prevStates[parent.ID]})
			seen[parent.ID] = true
		}
		task = parent
	}
}
