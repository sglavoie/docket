package cmd

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// projectRuleResult holds the outcome of project-level rule evaluation.
type projectRuleResult struct {
	ProjectStatusSet  bool
	ProjectStatusIcon string
	ProjectRuleName   string
	TriggeredRuleIDs  []string
}

// evaluateAndApplyProjectRules evaluates project-level rules and applies the result
// if the project's status was not manually set by the user.
func evaluateAndApplyProjectRules(project *models.Project, rules []workflow.WorkflowRule) projectRuleResult {
	var res projectRuleResult

	ctx := &workflow.ProjectEvaluationContext{
		Project: project,
		Tasks:   project.Tasks,
	}

	projEval := workflow.EvaluateProjectRules(ctx, rules)
	res.TriggeredRuleIDs = projEval.TriggeredRuleIDs

	if projEval.NewStatusIcon == "" {
		return res
	}

	// Only apply if source is not "user" (manual override protection)
	if project.StatusIconSource == "user" {
		return res
	}

	project.StatusIcon = projEval.NewStatusIcon
	project.StatusIconSource = "auto"

	res.ProjectStatusSet = true
	res.ProjectStatusIcon = projEval.NewStatusIcon
	res.ProjectRuleName = projEval.AppliedRuleName
	return res
}
