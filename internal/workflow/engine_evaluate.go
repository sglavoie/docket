package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// EvaluateRules evaluates all enabled rules against the given tasks.
// Returns which tasks were modified and any conflicts detected.
// This is the legacy function that doesn't support event-based triggers.
func EvaluateRules(tasks []*models.Task, rules []WorkflowRule) *EvaluationResult {
	changes := make([]TaskChangeContext, len(tasks))
	for i, task := range tasks {
		changes[i] = TaskChangeContext{Task: task, Previous: nil}
	}

	evalCtx := &EvaluationContext{
		Changes:  changes,
		AllTasks: tasks,
		TaskMap:  buildTaskMap(tasks),
	}

	return EvaluateRulesWithContext(evalCtx, rules)
}

// sortRulesByPriority returns a stable-sorted copy of rules: non-zero priorities
// ascending (1 first), then zero-priority rules in original order.
func sortRulesByPriority(rules []WorkflowRule) []WorkflowRule {
	sorted := make([]WorkflowRule, len(rules))
	copy(sorted, rules)
	sort.SliceStable(sorted, func(i, j int) bool {
		pi, pj := sorted[i].Priority, sorted[j].Priority
		if pi == 0 && pj == 0 {
			return false
		}
		if pi == 0 {
			return false
		}
		if pj == 0 {
			return true
		}
		return pi < pj
	})
	return sorted
}

// EvaluateRulesWithContext evaluates rules with full context for event and hierarchical triggers.
// This is the preferred function when change detection is needed.
func EvaluateRulesWithContext(evalCtx *EvaluationContext, rules []WorkflowRule) *EvaluationResult {
	result := &EvaluationResult{
		ModifiedTasks:     []*models.Task{},
		Conflicts:         []RuleConflict{},
		AppliedRules:      make(map[string][]string),
		RuleTriggerCounts: make(map[string]int),
	}

	if len(rules) == 0 {
		return result
	}

	sorted := sortRulesByPriority(rules)
	ctx := newExecutionContext()
	evaluateRulesWithContextRecursive(evalCtx, sorted, ctx, result)
	return result
}

func evaluateRulesWithContextRecursive(evalCtx *EvaluationContext, rules []WorkflowRule, ctx *executionContext, result *EvaluationResult) { //nolint:gocyclo
	if ctx.depth >= maxRuleDepth {
		return
	}
	ctx.depth++
	defer func() { ctx.depth-- }()

	for _, change := range evalCtx.Changes {
		task := change.Task
		prev := change.Previous

		fieldSetters := make(map[string][]fieldSetter)

		type matchedRule struct {
			rule WorkflowRule
		}
		var matched []matchedRule

		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			if rule.GetLevel() == RuleLevelProject {
				continue
			}
			if !rule.MatchesScope(task, evalCtx.ProjectName, evalCtx.ProjectID) {
				continue
			}
			if ctx.wasApplied(task.ID, rule.ID) {
				continue
			}
			if !matchesRuleWithContext(task, prev, &rule, evalCtx) {
				continue
			}

			matched = append(matched, matchedRule{rule: rule})
			for _, action := range rule.Actions {
				if action.Condition != nil && !matchesTriggerWithContext(task, prev, *action.Condition, evalCtx) {
					continue
				}
				field := getFieldFromAction(action.ActionType)
				if field != "" {
					fieldSetters[field] = append(fieldSetters[field], fieldSetter{
						ruleName: rule.Name,
						ruleID:   rule.ID,
						priority: rule.Priority,
					})
				}
			}
		}

		skippedFieldRules := make(map[string]map[string]bool)
		for field, setters := range fieldSetters {
			if len(setters) <= 1 {
				continue
			}

			bestPriority := 0
			for _, s := range setters {
				if s.priority > 0 && (bestPriority == 0 || s.priority < bestPriority) {
					bestPriority = s.priority
				}
			}

			var winners []fieldSetter
			for _, s := range setters {
				effectivePri := s.priority
				if effectivePri == 0 {
					effectivePri = 1<<31 - 1
				}
				bestEffective := bestPriority
				if bestEffective == 0 {
					bestEffective = 1<<31 - 1
				}
				if effectivePri == bestEffective {
					winners = append(winners, s)
				}
			}

			if len(winners) > 1 {
				var names []string
				for _, w := range winners {
					names = append(names, w.ruleName)
				}
				result.Conflicts = append(result.Conflicts, RuleConflict{
					TaskID:    task.ID,
					TaskTitle: task.Title,
					Field:     field,
					Rules:     names,
				})
				if skippedFieldRules[field] == nil {
					skippedFieldRules[field] = make(map[string]bool)
				}
				for _, w := range winners {
					skippedFieldRules[field][w.ruleID] = true
				}
			} else {
				winnerID := winners[0].ruleID
				if skippedFieldRules[field] == nil {
					skippedFieldRules[field] = make(map[string]bool)
				}
				for _, s := range setters {
					if s.ruleID != winnerID {
						skippedFieldRules[field][s.ruleID] = true
					}
				}
			}
		}

		// Snapshot the task before applying actions so the recursive
		// re-evaluation can detect what changed via event triggers.
		preActionSnapshot := snapshotTaskForEvent(task)

		modified := false
		for _, mr := range matched {
			rule := mr.rule
			ctx.markApplied(task.ID, rule.ID)

			ruleApplied := false
			for _, action := range rule.Actions {
				if action.Condition != nil && !matchesTriggerWithContext(task, prev, *action.Condition, evalCtx) {
					continue
				}
				field := getFieldFromAction(action.ActionType)
				if field != "" {
					if skipSet, ok := skippedFieldRules[field]; ok && skipSet[rule.ID] {
						continue
					}
				}
				if applyActionWithContext(task, action, evalCtx, result) {
					modified = true
					ruleApplied = true
				}
			}
			if ruleApplied {
				result.AppliedRules[task.ID] = append(result.AppliedRules[task.ID], rule.Name)
				result.RuleTriggerCounts[rule.ID]++
			}
		}

		if modified {
			result.ModifiedTasks = append(result.ModifiedTasks, task)
			newChange := TaskChangeContext{Task: task, Previous: preActionSnapshot}
			newEvalCtx := &EvaluationContext{
				Changes:     []TaskChangeContext{newChange},
				AllTasks:    evalCtx.AllTasks,
				TaskMap:     evalCtx.TaskMap,
				ProjectName: evalCtx.ProjectName,
				ProjectID:   evalCtx.ProjectID,
			}
			evaluateRulesWithContextRecursive(newEvalCtx, rules, ctx, result)
		}
	}
}

func getFieldFromAction(actionType ActionType) string {
	switch actionType {
	case ActionSetStatus:
		return "status"
	case ActionSetPriority:
		return "priority"
	case ActionSetSection:
		return "section"
	default:
		return ""
	}
}

// FormatConflicts returns a human-readable description of conflicts.
func FormatConflicts(conflicts []RuleConflict) string {
	if len(conflicts) == 0 {
		return ""
	}

	var lines []string
	for _, c := range conflicts {
		lines = append(lines, fmt.Sprintf(
			"Conflict on task %q: multiple rules (%s) try to set %s",
			c.TaskTitle,
			strings.Join(c.Rules, ", "),
			c.Field,
		))
	}
	return strings.Join(lines, "\n")
}

// TestRule evaluates a single rule against a task without applying changes.
// Returns whether the rule would match and what actions would be taken.
func TestRule(task *models.Task, rule WorkflowRule) (matches bool, actions []string) {
	if !matchesRule(task, &rule) {
		return false, nil
	}

	for _, action := range rule.Actions {
		actions = append(actions, action.String())
	}
	return true, actions
}
