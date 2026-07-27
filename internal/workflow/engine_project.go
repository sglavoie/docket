package workflow

import (
	"sort"
	"strconv"
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
// Returns an empty result when ctx or ctx.Project is nil so callers don't have
// to defensively pre-check before invoking the engine.
func EvaluateProjectRules(ctx *ProjectEvaluationContext, rules []WorkflowRule) *ProjectEvaluationResult {
	result := &ProjectEvaluationResult{}
	if ctx == nil || ctx.Project == nil {
		return result
	}

	// Filter to enabled project-level rules
	var projectRules []WorkflowRule
	for _, r := range rules {
		if r.Enabled && r.GetLevel() == RuleLevelProject && r.MatchesScope(nil, ctx.Project.Name, ctx.Project.ID) {
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
	return matchesProjectTriggerGroupAtDepth(ctx, group, 0)
}

// matchesProjectTriggerGroupAtDepth bounds recursion to maxTriggerGroupDepth
// so that a corrupted or hand-crafted rule cannot blow the stack at
// evaluation time. Mirrors matchesTriggerGroupAtDepth on the task path.
func matchesProjectTriggerGroupAtDepth(ctx *ProjectEvaluationContext, group *TriggerGroup, depth int) bool {
	if group == nil || depth >= maxTriggerGroupDepth {
		return false
	}

	logic := group.getLogic()

	var results []bool
	for _, t := range group.Triggers {
		results = append(results, matchProjectTrigger(ctx, t))
	}
	for i := range group.Groups {
		results = append(results, matchesProjectTriggerGroupAtDepth(ctx, &group.Groups[i], depth+1))
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
		return compareBoolOp(anyTaskMatches(ctx.Tasks, func(t *models.Task) bool {
			return t.IsInState(models.StateInProgress)
		}), trigger.Operation, trigger.Value)

	case FieldActiveTaskCount:
		count := countTasks(ctx.Tasks, func(t *models.Task) bool { return !t.IsTerminalState() })
		return compareValues(strconv.Itoa(count), trigger.Operation, trigger.Value)

	case FieldAllTasksState:
		return matchAllTasksState(ctx, trigger)

	case FieldMajorityTaskType:
		return matchesMajority(ctx, func(t *models.Task) string { return string(t.Type) }, trigger)

	case FieldMajorityTaskState:
		return matchesMajorityTaskState(ctx, trigger)

	case FieldCompletionRatio:
		return matchCompletionRatio(ctx, trigger)

	case FieldDaysSinceUpdate:
		return matchDaysSinceUpdate(ctx, trigger)

	case FieldProjectStatus:
		return compareValues(ctx.Project.StatusIcon, trigger.Operation, trigger.Value)

	case FieldHasTasksWithState:
		return matchHasTasksWithState(ctx.Tasks, trigger)

	default:
		return false
	}
}

// anyTaskMatches reports whether any non-nil task satisfies pred.
func anyTaskMatches(tasks []*models.Task, pred func(*models.Task) bool) bool {
	for _, t := range tasks {
		if t == nil {
			continue
		}
		if pred(t) {
			return true
		}
	}
	return false
}

// countTasks counts non-nil tasks satisfying pred.
func countTasks(tasks []*models.Task, pred func(*models.Task) bool) int {
	count := 0
	for _, t := range tasks {
		if t == nil {
			continue
		}
		if pred(t) {
			count++
		}
	}
	return count
}

// matchAllTasksState evaluates exact operations against one shared canonical
// state. Set operations use cohort semantics: in requires every nonterminal
// task to belong to the set, while not_in requires every nonterminal task to
// fall outside it. All variants require at least one nonterminal task.
func matchAllTasksState(ctx *ProjectEvaluationContext, trigger RuleTrigger) bool {
	states, err := ParseWorkflowStateSet(trigger.Value)
	if err != nil {
		return false
	}
	if (trigger.Operation == OpEquals || trigger.Operation == OpNotEquals) && states.Len() != 1 {
		return false
	}

	if trigger.Operation == OpIn || trigger.Operation == OpNotIn {
		found := false
		for _, task := range ctx.Tasks {
			if task == nil || task.IsTerminalState() {
				continue
			}
			found = true
			contains := states.Contains(task.State())
			if (trigger.Operation == OpIn && !contains) ||
				(trigger.Operation == OpNotIn && contains) {
				return false
			}
		}
		return found
	}

	var common models.WorkflowState
	for _, task := range ctx.Tasks {
		if task == nil || task.IsTerminalState() {
			continue
		}
		state := task.State()
		if common == "" {
			common = state
			continue
		}
		if state != common {
			return false
		}
	}
	if common == "" {
		return false
	}
	return matchesWorkflowState(common, trigger.Operation, trigger.Value)
}

// matchesMajorityTaskState evaluates exact operations against a strict
// single-state majority. Set operations treat membership as one cohort and
// require strictly more than half of nonterminal tasks to fall inside (in) or
// outside (not_in) the requested set.
func matchesMajorityTaskState(ctx *ProjectEvaluationContext, trigger RuleTrigger) bool {
	states, err := ParseWorkflowStateSet(trigger.Value)
	if err != nil {
		return false
	}
	if (trigger.Operation == OpEquals || trigger.Operation == OpNotEquals) && states.Len() != 1 {
		return false
	}

	if trigger.Operation == OpIn || trigger.Operation == OpNotIn {
		total := 0
		matching := 0
		for _, task := range ctx.Tasks {
			if task == nil || task.IsTerminalState() {
				continue
			}
			total++
			contains := states.Contains(task.State())
			if (trigger.Operation == OpIn && contains) ||
				(trigger.Operation == OpNotIn && !contains) {
				matching++
			}
		}
		return total > 0 && matching*2 > total
	}

	counts := make(map[models.WorkflowState]int)
	total := 0
	for _, task := range ctx.Tasks {
		if task == nil || task.IsTerminalState() {
			continue
		}
		total++
		counts[task.State()]++
	}
	if total == 0 {
		return false
	}
	for _, state := range models.AllWorkflowStates() {
		if counts[state]*2 > total {
			return matchesWorkflowState(state, trigger.Operation, trigger.Value)
		}
	}
	return false
}

// matchHasTasksWithState treats negative operations as the inverse of the
// positive presence query: not_in/not_equals mean no task has a matching state.
func matchHasTasksWithState(tasks []*models.Task, trigger RuleTrigger) bool {
	states, err := ParseWorkflowStateSet(trigger.Value)
	if err != nil {
		return false
	}
	if (trigger.Operation == OpEquals || trigger.Operation == OpNotEquals) && states.Len() != 1 {
		return false
	}
	has := anyTaskMatches(tasks, func(task *models.Task) bool {
		return states.Contains(task.State())
	})
	switch trigger.Operation {
	case OpEquals, OpIn:
		return has
	case OpNotEquals, OpNotIn:
		return !has
	default:
		return false
	}
}

// matchCompletionRatio compares the done/total percentage to the trigger value.
func matchCompletionRatio(ctx *ProjectEvaluationContext, trigger RuleTrigger) bool {
	total := countTasks(ctx.Tasks, func(*models.Task) bool { return true })
	if total == 0 {
		return compareValues("0", trigger.Operation, trigger.Value)
	}
	done := countTasks(ctx.Tasks, func(t *models.Task) bool { return t.IsTerminalState() })
	ratio := done * 100 / total
	return compareValues(strconv.Itoa(ratio), trigger.Operation, trigger.Value)
}

// matchDaysSinceUpdate compares the calendar days between now and the most
// recent task or project update against the trigger value.
func matchDaysSinceUpdate(ctx *ProjectEvaluationContext, trigger RuleTrigger) bool {
	latest := ctx.Project.UpdatedAt
	for _, t := range ctx.Tasks {
		if t == nil {
			continue
		}
		if t.UpdatedAt.After(latest) {
			latest = t.UpdatedAt
		}
	}
	if latest.IsZero() {
		return false
	}
	days := max(util.DaysBetween(latest, time.Now()), 0)
	return compareValues(strconv.Itoa(days), trigger.Operation, trigger.Value)
}

// matchesMajority checks if >50% of active tasks have the given field value.
func matchesMajority(ctx *ProjectEvaluationContext, fieldGetter func(*models.Task) string, trigger RuleTrigger) bool {
	counts := make(map[string]int)
	total := 0
	for _, t := range ctx.Tasks {
		if t == nil {
			continue
		}
		if t.IsTerminalState() {
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
