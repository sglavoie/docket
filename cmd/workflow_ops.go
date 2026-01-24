package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// CaptureTaskStates creates a snapshot of task states for event trigger detection.
// Call this before making changes to tasks to capture their previous state.
func CaptureTaskStates(tasks []*models.Task) map[string]*models.Task {
	states := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		states[task.ID] = copyTaskState(task)
	}
	return states
}

// copyTaskState creates a shallow copy of task fields relevant for event detection.
func copyTaskState(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}
	// Copy tags slice
	tagsCopy := make([]string, len(task.Tags))
	copy(tagsCopy, task.Tags)

	return &models.Task{
		ID:       task.ID,
		Status:   task.Status,
		Priority: task.Priority,
		Section:  task.Section,
		Tags:     tagsCopy,
		Type:     task.Type,
		// Preserve parent/child relationships for hierarchical triggers
		Parent:   task.Parent,
		Children: task.Children,
		Level:    task.Level,
	}
}

// BuildEvaluationContext creates an evaluation context for workflow rules.
func BuildEvaluationContext(tasks []*models.Task, prevStates map[string]*models.Task) *workflow.EvaluationContext {
	changes := make([]workflow.TaskChangeContext, len(tasks))
	taskMap := make(map[string]*models.Task, len(tasks))

	for i, task := range tasks {
		taskMap[task.ID] = task
		changes[i] = workflow.TaskChangeContext{
			Task:     task,
			Previous: prevStates[task.ID], // nil if task is new
		}
	}

	return &workflow.EvaluationContext{
		Changes:  changes,
		AllTasks: tasks,
		TaskMap:  taskMap,
	}
}

// WorkflowEvalResult holds the result of evaluating workflow rules for a project.
type WorkflowEvalResult struct {
	ProjectName   string
	ModifiedTasks int
	RulesApplied  map[string]int // rule ID -> count of tasks affected
	Skipped       bool           // true if evaluation was skipped (recently evaluated)
	Error         error
}

// EvaluateProjectWorkflowRules evaluates workflow rules for a project.
// If force is false and the project was recently evaluated, evaluation is skipped.
// The project must already be loaded with tasks.
// Returns the evaluation result and updates the eval state in config.
func EvaluateProjectWorkflowRules(project *models.Project, cfg *config.Config, configManager *config.Manager, force bool) WorkflowEvalResult {
	result := WorkflowEvalResult{
		ProjectName:  project.Name,
		RulesApplied: make(map[string]int),
	}

	// Check if evaluation is needed
	if !force && !cfg.ShouldEvaluateWorkflow(project.ID) {
		result.Skipped = true
		return result
	}

	rules := cfg.GetEnabledWorkflowRules()
	if len(rules) == 0 {
		result.Skipped = true
		return result
	}

	// Run auto-unblock before rule evaluation
	project.CheckAndAutoUnblock()

	// Evaluate rules
	evalResult := workflow.EvaluateRules(project.Tasks, rules)

	result.ModifiedTasks = len(evalResult.ModifiedTasks)
	result.RulesApplied = evalResult.RuleTriggerCounts

	// Only save if there were actual modifications
	if result.ModifiedTasks > 0 {
		project.ReorderRootTasksByCompletion()

		if err := storage.BackupProject(project); err != nil {
			result.Error = fmt.Errorf("failed to create backup: %w", err)
			return result
		}

		if err := storage.SaveProject(project); err != nil {
			result.Error = fmt.Errorf("failed to save project: %w", err)
			return result
		}

		// Update trigger counts
		if len(evalResult.RuleTriggerCounts) > 0 && configManager != nil {
			cfg.IncrementRuleTriggerCounts(evalResult.RuleTriggerCounts)
		}
	}

	// Update eval state
	cfg.SetWorkflowEvalState(project.ID, config.WorkflowEvalState{
		LastEvalTime: time.Now(),
		RulesHash:    cfg.ComputeWorkflowRulesHash(),
	})

	// Persist config changes
	if configManager != nil {
		if err := configManager.Save(cfg); err != nil {
			// Non-fatal, just log
			fmt.Fprintln(os.Stderr, "Warning: failed to save workflow eval state:", err)
		}
	}

	return result
}
