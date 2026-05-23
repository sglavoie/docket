package workflow

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// ProjectEvaluationContext holds the data needed to evaluate project-level rules.
type ProjectEvaluationContext struct {
	Project *models.Project
	Tasks   []*models.Task
}

// ProjectEvaluationResult holds the outcome of project-level rule evaluation.
type ProjectEvaluationResult struct {
	NewStatusIcon    string
	AppliedRuleName  string
	AppliedRuleID    string
	TriggeredRuleIDs []string
}

// EvaluateProjectRules evaluates project-level workflow rules against aggregate task data.
// Rules are sorted by priority (lower number = higher priority, 0 sorts last).
// Same-priority conflicts cancel out (neither is applied).
func EvaluateProjectRules(ctx *ProjectEvaluationContext, rules []WorkflowRule) *ProjectEvaluationResult {
	result := &ProjectEvaluationResult{}

	// Filter to enabled project-level rules
	var projectRules []WorkflowRule
	for _, r := range rules {
		if r.Enabled && r.GetLevel() == RuleLevelProject {
			projectRules = append(projectRules, r)
		}
	}
	if len(projectRules) == 0 {
		return result
	}

	sorted := sortRulesByPriority(projectRules)

	// Find all matching rules
	type matchInfo struct {
		rule     WorkflowRule
		statusTo string
	}
	var matches []matchInfo

	for _, rule := range sorted {
		if matchesProjectRule(ctx, &rule) {
			// Extract the target status from the first set_project_status action
			for _, action := range rule.Actions {
				if action.ActionType == ActionSetProjectStatus {
					matches = append(matches, matchInfo{rule: rule, statusTo: action.Value})
					break
				}
			}
		}
	}

	if len(matches) == 0 {
		return result
	}

	// Resolve by priority: pick the highest-priority match.
	// If multiple matches share the same best priority, they cancel out.
	bestPriority := effectivePriority(matches[0].rule.Priority)
	var winners []matchInfo
	for _, m := range matches {
		ep := effectivePriority(m.rule.Priority)
		if ep < bestPriority {
			bestPriority = ep
			winners = []matchInfo{m}
		} else if ep == bestPriority {
			winners = append(winners, m)
		}
	}

	if len(winners) == 1 {
		// Single winner: only record the applied rule so callers that
		// increment trigger counts do not inflate counts for lower-priority
		// rules that matched but were overridden.
		result.NewStatusIcon = winners[0].statusTo
		result.AppliedRuleName = winners[0].rule.Name
		result.AppliedRuleID = winners[0].rule.ID
		result.TriggeredRuleIDs = []string{winners[0].rule.ID}
	} else {
		// Same-priority conflict: surface all tied IDs so callers can report
		// which rules collided. NewStatusIcon remains empty (neither applied).
		for _, w := range winners {
			result.TriggeredRuleIDs = append(result.TriggeredRuleIDs, w.rule.ID)
		}
	}
	// len(winners) > 1: same-priority conflict, neither applied

	return result
}

// effectivePriority returns a comparable priority where 0 (unset) sorts last.
func effectivePriority(p int) int {
	if p == 0 {
		return 1<<31 - 1
	}
	return p
}

// matchesProjectRule checks whether all triggers of a project rule match.
func matchesProjectRule(ctx *ProjectEvaluationContext, rule *WorkflowRule) bool {
	// Use TriggerGroup if present
	if rule.TriggerGroup != nil {
		return matchesProjectTriggerGroup(ctx, rule.TriggerGroup)
	}

	triggers := rule.GetTriggers()
	if len(triggers) == 0 {
		return false
	}

	logic := rule.GetTriggerLogic()

	if logic == LogicOr {
		for _, t := range triggers {
			if matchProjectTrigger(ctx, t) {
				return true
			}
		}
		return false
	}

	for _, t := range triggers {
		if !matchProjectTrigger(ctx, t) {
			return false
		}
	}
	return true
}

// matchesProjectTriggerGroup recursively evaluates a nested trigger group for project rules.
func matchesProjectTriggerGroup(ctx *ProjectEvaluationContext, group *TriggerGroup) bool {
	logic := group.getLogic()

	var results []bool
	for _, t := range group.Triggers {
		results = append(results, matchProjectTrigger(ctx, t))
	}
	for _, sub := range group.Groups {
		results = append(results, matchesProjectTriggerGroup(ctx, &sub))
	}

	if len(results) == 0 {
		return false
	}

	if logic == LogicOr {
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	}

	for _, r := range results {
		if !r {
			return false
		}
	}
	return true
}

// matchProjectTrigger evaluates a single project-level trigger against aggregate task data.
func matchProjectTrigger(ctx *ProjectEvaluationContext, trigger RuleTrigger) bool {
	switch trigger.Field {
	case FieldHasInProgressTasks:
		has := false
		for _, t := range ctx.Tasks {
			if t.Status == models.StatusInProgress {
				has = true
				break
			}
		}
		return compareBool(has, trigger.Value)

	case FieldActiveTaskCount:
		count := 0
		for _, t := range ctx.Tasks {
			if t.Status.IsActive() {
				count++
			}
		}
		return compareValues(strconv.Itoa(count), trigger.Operation, trigger.Value)

	case FieldAllTasksStatus:
		// Check if all non-complete tasks share the same status
		var status string
		allSame := true
		for _, t := range ctx.Tasks {
			if t.Status.IsComplete() {
				continue
			}
			if status == "" {
				status = string(t.Status)
			} else if !strings.EqualFold(status, string(t.Status)) {
				allSame = false
				break
			}
		}
		if !allSame || status == "" {
			return false
		}
		return compareValues(status, trigger.Operation, trigger.Value)

	case FieldMajorityTaskType:
		return matchesMajority(ctx, func(t *models.Task) string { return string(t.Type) }, trigger)

	case FieldMajorityTaskStatus:
		return matchesMajority(ctx, func(t *models.Task) string { return string(t.Status) }, trigger)

	case FieldCompletionRatio:
		total := len(ctx.Tasks)
		if total == 0 {
			return compareValues("0", trigger.Operation, trigger.Value)
		}
		done := 0
		for _, t := range ctx.Tasks {
			if t.Status.IsComplete() {
				done++
			}
		}
		ratio := done * 100 / total
		return compareValues(strconv.Itoa(ratio), trigger.Operation, trigger.Value)

	case FieldDaysSinceUpdate:
		latest := ctx.Project.UpdatedAt
		for _, t := range ctx.Tasks {
			if t.UpdatedAt.After(latest) {
				latest = t.UpdatedAt
			}
		}
		if latest.IsZero() {
			return false
		}
		days := max(util.DaysBetween(latest, time.Now()), 0)
		return compareValues(strconv.Itoa(days), trigger.Operation, trigger.Value)

	case FieldProjectStatus:
		return compareValues(ctx.Project.StatusIcon, trigger.Operation, trigger.Value)

	case FieldHasTasksWithStatus:
		for _, t := range ctx.Tasks {
			if strings.EqualFold(string(t.Status), trigger.Value) {
				return true
			}
		}
		return false

	default:
		return false
	}
}

// matchesMajority checks if >50% of active tasks have the given field value.
func matchesMajority(ctx *ProjectEvaluationContext, fieldGetter func(*models.Task) string, trigger RuleTrigger) bool {
	counts := make(map[string]int)
	total := 0
	for _, t := range ctx.Tasks {
		if !t.Status.IsActive() {
			continue
		}
		total++
		counts[fieldGetter(t)]++
	}
	if total == 0 {
		return false
	}

	// Sort keys to find the majority deterministically
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if counts[k]*2 > total { // strictly >50%
			return compareValues(k, trigger.Operation, trigger.Value)
		}
	}
	return false
}
