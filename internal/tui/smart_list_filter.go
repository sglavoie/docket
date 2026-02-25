package tui

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// passesSmartListCriteria returns true if the task matches all (AND) or any (OR)
// of the smart list's criteria.
func passesSmartListCriteria(task *models.Task, project *models.Project, sl *config.SmartList, cfg *config.Config) bool {
	if len(sl.Criteria) == 0 {
		return true
	}

	isAnd := sl.EffectiveLogic() == "and"
	for _, c := range sl.Criteria {
		passes := evaluateCriterion(task, project, c, cfg)
		if isAnd && !passes {
			return false
		}
		if !isAnd && passes {
			return true
		}
	}
	return isAnd // AND: all passed; OR: none passed
}

// evaluateCriterion checks a single criterion against a task.
func evaluateCriterion(task *models.Task, project *models.Project, c config.SmartListCriterion, cfg *config.Config) bool {
	switch c.Field {
	case "type":
		return evalStringOp(string(task.Type), c.Operation, c.Value)

	case "status":
		return evalStringOp(string(task.Status), c.Operation, c.Value)

	case "priority":
		taskVal := task.Priority
		targetVal, err := strconv.Atoi(c.Value)
		if err != nil {
			return false
		}
		return evalNumericOp(taskVal, c.Operation, targetVal)

	case "tags":
		if c.Operation == "contains" {
			return task.HasTag(c.Value)
		}
		return false

	case "due_date":
		return evalDueDate(task, c.Value)

	case "section":
		return evalStringOp(task.Section, c.Operation, c.Value)

	case "score_min":
		threshold, err := strconv.ParseFloat(c.Value, 64)
		if err != nil {
			return false
		}
		if cfg == nil {
			return false
		}
		profile := cfg.GetProjectScoringProfile(project.ID)
		if profile == nil {
			return false
		}
		score := scoring.ComputeScore(task, profile)
		return score >= threshold

	case "actionable":
		wantActionable := strings.EqualFold(c.Value, "true")
		isActionable := task.Status != models.StatusBlocked && !project.HasIncompleteDependencies(task)
		return isActionable == wantActionable

	case "created_days":
		return evalDaysDelta(task.CreatedAt, c.Operation, c.Value)

	case "updated_days":
		return evalDaysDelta(task.UpdatedAt, c.Operation, c.Value)

	case "completed_days":
		if task.CompletedAt == nil {
			return false
		}
		return evalDaysDelta(*task.CompletedAt, c.Operation, c.Value)
	}

	return false
}

// evalStringOp compares two strings with the given operation.
func evalStringOp(actual, op, target string) bool {
	switch op {
	case "equals":
		return strings.EqualFold(actual, target)
	case "not_equals":
		return !strings.EqualFold(actual, target)
	case "contains":
		return strings.Contains(strings.ToLower(actual), strings.ToLower(target))
	}
	return false
}

// evalNumericOp compares two integers with the given operation.
func evalNumericOp(actual int, op string, target int) bool {
	switch op {
	case "equals":
		return actual == target
	case "not_equals":
		return actual != target
	case "greater_than":
		return actual > target
	case "less_than":
		return actual < target
	}
	return false
}

// evalDueDate checks task due date against a named bucket.
func evalDueDate(task *models.Task, value string) bool {
	switch value {
	case "overdue":
		return task.IsOverdue()
	case "today":
		return task.IsDueToday()
	case "this_week":
		return task.IsDueThisWeek()
	case "none":
		return task.DueDate == nil
	}
	return false
}

// evalDaysDelta computes the number of days since the given timestamp and
// compares it against the threshold with the given operation.
func evalDaysDelta(t time.Time, op, value string) bool {
	threshold, err := strconv.Atoi(value)
	if err != nil {
		return false
	}
	now := time.Now()
	dayStart := util.StartOfDay(t)
	nowStart := util.StartOfDay(now)
	daysDelta := int(math.Round(nowStart.Sub(dayStart).Hours() / 24))

	switch op {
	case "less_than":
		return daysDelta < threshold
	case "greater_than":
		return daysDelta > threshold
	case "equals":
		return daysDelta == threshold
	}
	return false
}
