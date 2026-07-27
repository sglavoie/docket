package workflow

import (
	"slices"
	"sort"

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
// A nil evalCtx returns an empty result so callers don't have to defensively
// pre-check before invoking the engine.
func EvaluateRulesWithContext(evalCtx *EvaluationContext, rules []WorkflowRule) *EvaluationResult {
	result := &EvaluationResult{
		ModifiedTasks:     []*models.Task{},
		Conflicts:         []RuleConflict{},
		AppliedRules:      make(map[string][]string),
		RuleTriggerCounts: make(map[string]int),
	}

	if evalCtx == nil || len(rules) == 0 {
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
		if task == nil {
			continue
		}
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
				if shouldSkipAllChildrenDoneCompletion(task, prev, rule, action, evalCtx) {
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
		preActionSnapshot := CopyTaskState(task)

		modified := false
		for _, mr := range matched {
			rule := mr.rule

			ruleApplied := false
			// Track whether every action was deferred purely because its
			// Condition did not match. Actions skipped for any other reason
			// (field-setter conflict, no-op) count as a permanent decision
			// for this pass and the rule is marked applied. Rules whose
			// only blocker is an unmet Condition must remain eligible for
			// a recursive pass that may make the condition true.
			onlyConditionSkips := true
			anyActionConsidered := false
			for _, action := range rule.Actions {
				anyActionConsidered = true
				if action.Condition != nil && !matchesTriggerWithContext(task, prev, *action.Condition, evalCtx) {
					continue
				}
				// The action's condition (if any) matched, so any further
				// skip/no-op below counts as a final decision.
				onlyConditionSkips = false
				field := getFieldFromAction(action.ActionType)
				if field != "" {
					if skipSet, ok := skippedFieldRules[field]; ok && skipSet[rule.ID] {
						continue
					}
				}
				if shouldSkipAllChildrenDoneCompletion(task, prev, rule, action, evalCtx) {
					continue
				}
				if applyActionWithContext(task, action, evalCtx, result) {
					modified = true
					ruleApplied = true
				}
			}
			if ruleApplied || !onlyConditionSkips || !anyActionConsidered {
				ctx.markApplied(task.ID, rule.ID)
			}
			if ruleApplied {
				result.AppliedRules[task.ID] = append(result.AppliedRules[task.ID], rule.Name)
				result.RuleTriggerCounts[rule.ID]++
			}
		}

		if modified {
			if !containsTask(result.ModifiedTasks, task) {
				result.ModifiedTasks = append(result.ModifiedTasks, task)
			}
			newChanges := []TaskChangeContext{{Task: task, Previous: preActionSnapshot}}
			if task.IsTerminalState() != preActionSnapshot.IsTerminalState() {
				seen := map[string]bool{task.ID: true}
				appendAncestorChanges(task, evalCtx.TaskMap, evalCtx.PreviousTaskMap, seen, &newChanges)
			}
			newEvalCtx := &EvaluationContext{
				Changes:         newChanges,
				AllTasks:        evalCtx.AllTasks,
				TaskMap:         evalCtx.TaskMap,
				PreviousTaskMap: evalCtx.PreviousTaskMap,
				ProjectName:     evalCtx.ProjectName,
				ProjectID:       evalCtx.ProjectID,
				TransitionState: evalCtx.TransitionState,
			}
			evaluateRulesWithContextRecursive(newEvalCtx, rules, ctx, result)
		}
	}
}

// containsTask reports whether tasks already contains t (pointer equality).
// Used to deduplicate ModifiedTasks during recursive rule evaluation.
func containsTask(tasks []*models.Task, t *models.Task) bool {
	return slices.Contains(tasks, t)
}

func shouldSkipAllChildrenDoneCompletion(task, prev *models.Task, rule WorkflowRule, action RuleAction, evalCtx *EvaluationContext) bool {
	if task == nil {
		return false
	}
	target, stateChanging := actionTargetState(action)
	if !stateChanging || !target.IsTerminal() {
		return false
	}
	if !triggerGroupContainsField(rule.GetTriggerGroup(), FieldAllChildrenDone) {
		return false
	}
	// Preserve the defensive behavior for manually assembled/legacy contextual
	// contexts that carry only the changed task snapshot, not a full previous
	// hierarchy map.
	if evalCtx == nil || evalCtx.PreviousTaskMap == nil {
		return prev != nil && prev.IsTerminalState() && !task.IsTerminalState()
	}
	// In contextual mutation evaluation, an all_children_done auto-completion is
	// an event: it is eligible only when the direct-child aggregate changed from
	// false to true. A section/priority/edit on a parent whose children were
	// already terminal must never complete that parent.
	return previousAllChildrenTerminal(task.ID, evalCtx.PreviousTaskMap) || !currentAllChildrenTerminal(task.ID, evalCtx.TaskMap)
}

func previousAllChildrenTerminal(parentID string, previous map[string]*models.Task) bool {
	return allChildrenTerminal(parentID, previous)
}

func currentAllChildrenTerminal(parentID string, current map[string]*models.Task) bool {
	return allChildrenTerminal(parentID, current)
}

func allChildrenTerminal(parentID string, tasks map[string]*models.Task) bool {
	hasChildren := false
	for _, task := range tasks {
		if task == nil || task.ParentID == nil || *task.ParentID != parentID {
			continue
		}
		hasChildren = true
		if !task.IsTerminalState() {
			return false
		}
	}
	return hasChildren
}

func triggerGroupContainsField(group *TriggerGroup, field TriggerField) bool {
	if group == nil {
		return false
	}
	for _, trigger := range group.Triggers {
		if trigger.Field == field {
			return true
		}
	}
	for i := range group.Groups {
		if triggerGroupContainsField(&group.Groups[i], field) {
			return true
		}
	}
	return false
}

func getFieldFromAction(actionType ActionType) string {
	switch actionType {
	case ActionSetState:
		return "state"
	case ActionSetPriority:
		return "priority"
	case ActionSetType:
		return "type"
	case ActionMoveToProject:
		return "move_to_project"
	default:
		return ""
	}
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
