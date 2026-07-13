package workflow

import "github.com/sglavoie/tt/internal/models"

const maxRuleDepth = 10

// RuleConflict represents a conflict when multiple rules try to set the same field.
type RuleConflict struct {
	TaskID    string
	TaskTitle string
	Field     string
	Rules     []string // Rule names that conflict
}

// DeferredMove represents a task that should be moved to a different project.
// The move is deferred because the engine doesn't have access to project storage.
type DeferredMove struct {
	TaskID      string
	Destination string // project name
}

// FocusChange represents a requested Focus Queue membership change produced by a
// set_active action. The engine has no access to the config-backed Focus Queue,
// so the change is deferred: a config-aware caller applies it (add/remove
// membership) after evaluation. Under the workflow-state model, Focus Queue
// membership is the canonical replacement for the retired Task.Active flag, so
// set_active no longer mutates the task itself.
type FocusChange struct {
	TaskID string
	Add    bool // true = add to Focus Queue, false = remove
}

// EvaluationResult contains the result of rule evaluation.
type EvaluationResult struct {
	ModifiedTasks     []*models.Task
	Conflicts         []RuleConflict
	AppliedRules      map[string][]string // taskID -> list of applied rule names
	RuleTriggerCounts map[string]int      // ruleID -> count of tasks affected
	NewFollowupTasks  []*models.Task      // tasks created by create_followup action
	DeferredMoves     []DeferredMove      // moves requested by move_to_project action
	FocusChanges      []FocusChange       // Focus Queue changes requested by set_active action
}

// TaskChangeContext provides context for event-based triggers.
// It contains both the current task state and the previous state (if available).
type TaskChangeContext struct {
	Task     *models.Task // Current task state
	Previous *models.Task // Previous state (nil for new tasks)
}

// EvaluationContext provides all context needed for rule evaluation.
type EvaluationContext struct {
	Changes     []TaskChangeContext     // Tasks with change context
	AllTasks    []*models.Task          // All tasks for hierarchical lookups
	TaskMap     map[string]*models.Task // taskID -> task for dependency lookups
	ProjectName string                  // Current project name (for scope matching)
	ProjectID   string                  // Current project ID (for scope matching)
}

// executionContext tracks rule applications to prevent infinite loops.
type executionContext struct {
	applied map[string]map[string]bool // taskID -> ruleID -> applied
	depth   int
}

func newExecutionContext() *executionContext {
	return &executionContext{
		applied: make(map[string]map[string]bool),
		depth:   0,
	}
}

func (ctx *executionContext) markApplied(taskID, ruleID string) {
	if ctx.applied[taskID] == nil {
		ctx.applied[taskID] = make(map[string]bool)
	}
	ctx.applied[taskID][ruleID] = true
}

func (ctx *executionContext) wasApplied(taskID, ruleID string) bool {
	if ctx.applied[taskID] == nil {
		return false
	}
	return ctx.applied[taskID][ruleID]
}

// buildTaskMap creates a map of taskID -> task for fast lookups.
// Nil entries in the input are skipped so a malformed slice does not panic.
func buildTaskMap(tasks []*models.Task) map[string]*models.Task {
	m := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		m[task.ID] = task
	}
	return m
}

// fieldSetter tracks which rule wants to set a field, with its priority.
type fieldSetter struct {
	ruleName string
	ruleID   string
	priority int
}
