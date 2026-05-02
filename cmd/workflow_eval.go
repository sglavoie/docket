package cmd

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// WorkflowEvalResult holds the result of evaluating workflow rules for a project.
type WorkflowEvalResult struct {
	ProjectName       string
	ModifiedTasks     int
	RulesApplied      map[string]int // rule ID -> count of tasks affected
	Skipped           bool           // true if evaluation was skipped (recently evaluated)
	ProjectStatusSet  bool           // true if project status was auto-set
	ProjectStatusIcon string         // new project status icon key (if set)
	ProjectRuleName   string         // name of the project rule that matched
	Error             error
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
		if err := SaveProject(project, nil); err != nil {
			result.Error = err
			return result
		}

		// Update trigger counts
		if len(evalResult.RuleTriggerCounts) > 0 && configManager != nil {
			cfg.IncrementRuleTriggerCounts(evalResult.RuleTriggerCounts)
		}
	}

	// Evaluate project-level rules against aggregate task data
	projResult := evaluateAndApplyProjectRules(project, cfg, rules)
	result.ProjectStatusSet = projResult.ProjectStatusSet
	result.ProjectStatusIcon = projResult.ProjectStatusIcon
	result.ProjectRuleName = projResult.ProjectRuleName
	if projResult.ProjectStatusSet {
		for _, id := range projResult.TriggeredRuleIDs {
			result.RulesApplied[id]++
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
			logging.Warn("failed to save workflow eval state: %v", err)
		}
	}

	return result
}
