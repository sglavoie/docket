package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// QueryDateLayout is the one calendar-date spelling a clause value may carry.
const QueryDateLayout = "2006-01-02"

// validateQueryValue holds a clause value to the vocabulary its field and
// operator can actually evaluate, and returns the canonical spelling. Refusing
// here is what keeps Admits total: every clause that exists is a clause the
// evaluator can answer without guessing.
func validateQueryValue(field QueryField, operator QueryOperator, value string) (string, error) {
	switch operator {
	case QueryOperatorNone, QueryOperatorPresent:
		if value != "" {
			return "", fmt.Errorf("query field %q with operator %q takes no value", field, operator)
		}
		return "", nil
	case QueryOperatorContains:
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("query field %q needs a non-empty value", field)
		}
		return value, nil
	default:
	}

	switch field {
	case QueryFieldDependency:
		return validateQueryReferenceValue(value)
	case QueryFieldPriority:
		return validateQueryPriorityValue(value)
	case QueryFieldTag, QueryFieldContext, QueryFieldProjectTag:
		return validateQueryTagValue(field, value)
	case QueryFieldProject:
		return validateQueryProjectValue(value)
	case QueryFieldDue:
		return validateQueryDueValue(value)
	case QueryFieldDefer:
		return validateQueryDayValue(field, value)
	case QueryFieldState:
		return validateQueryStateValue(value)
	case QueryFieldCompleted:
		return validateQueryAgeValue(value)
	case QueryFieldParked, QueryFieldArchived:
		return validateQueryPopulationValue(field, value)
	default:
		return "", fmt.Errorf("query field %q does not carry a value", field)
	}
}

func validateQueryReferenceValue(value string) (string, error) {
	projectID, taskID, found := strings.Cut(value, TaskReferenceSeparator)
	if !found {
		return "", fmt.Errorf("dependency %q is not a project-qualified locator", value)
	}
	if err := ValidateTaskReference(projectID, taskID); err != nil {
		return "", err
	}
	return value, nil
}

func validateQueryPriorityValue(value string) (string, error) {
	priority, err := strconv.Atoi(value)
	if err != nil {
		return "", fmt.Errorf("priority %q is not a number", value)
	}
	if err := ValidatePriority(priority, false); err != nil {
		return "", err
	}
	return strconv.Itoa(priority), nil
}

func validateQueryTagValue(field QueryField, value string) (string, error) {
	if value == queryClearValue {
		return "", fmt.Errorf("query field %q spells the unset reading with the none operator, not %q", field, value)
	}
	if err := ValidateTag(value); err != nil {
		return "", err
	}
	return value, nil
}

// validateQueryProjectValue accepts a project slug, lowercased. It checks shape
// alone and never that the project exists: a clause naming a renamed or deleted
// slug stays a valid clause that admits nothing.
func validateQueryProjectValue(value string) (string, error) {
	slug := strings.ToLower(value)
	if strings.TrimSpace(slug) == "" {
		return "", fmt.Errorf("query field %q needs a project slug", QueryFieldProject)
	}
	if strings.IndexFunc(slug, unicode.IsSpace) >= 0 || strings.ContainsRune(slug, '"') {
		return "", fmt.Errorf("project slug %q cannot hold whitespace or quotes", value)
	}
	return slug, nil
}

func validateQueryStateValue(value string) (string, error) {
	state := WorkflowState(value)
	if !state.IsValid() {
		return "", fmt.Errorf("unknown workflow state %q", value)
	}
	return string(state), nil
}

// validateQueryDueValue accepts a relative bucket, an exact calendar day, or a
// forward window. A window is what lets a due clause say "the next few hours"
// without freezing an instant into a persisted query.
func validateQueryDueValue(value string) (string, error) {
	switch value {
	case QueryDueOverdue, QueryDueToday, QueryDueWeek:
		return value, nil
	default:
	}
	if strings.HasPrefix(value, "+") {
		if _, _, err := parseQueryWindow(value); err != nil {
			return "", err
		}
		return value, nil
	}
	return validateQueryDayValue(QueryFieldDue, value)
}

func validateQueryDayValue(field QueryField, value string) (string, error) {
	if _, err := time.Parse(QueryDateLayout, value); err != nil {
		return "", fmt.Errorf("query field %q needs a %s date, got %q", field, QueryDateLayout, value)
	}
	return value, nil
}

// validateQueryAgeValue accepts the "Nd" completion-age threshold. Days are the
// only unit the completion clock is read at, so an hour threshold would promise
// a precision the stored timestamp comparison does not have.
func validateQueryAgeValue(value string) (string, error) {
	digits, found := strings.CutSuffix(value, "d")
	if !found {
		return "", fmt.Errorf("completion age %q must be a day count such as 90d", value)
	}
	days, err := strconv.Atoi(digits)
	if err != nil || days <= 0 {
		return "", fmt.Errorf("completion age %q must be a positive day count", value)
	}
	return strconv.Itoa(days) + "d", nil
}

func validateQueryPopulationValue(field QueryField, value string) (string, error) {
	if value == QueryPopulationInclude || value == QueryPopulationOnly {
		return value, nil
	}
	return "", fmt.Errorf("query field %q accepts %q or %q, got %q", field, QueryPopulationInclude, QueryPopulationOnly, value)
}

// parseQueryWindow reads a "+Nh", "+Nd", or "+Nw" forward window and reports
// both its magnitude and whether it is measured in hours. Hours are an instant
// comparison; days and weeks are local calendar days, matching how a date-only
// due date is stored at local midnight.
func parseQueryWindow(value string) (int, bool, error) {
	body, found := strings.CutPrefix(value, "+")
	if !found || body == "" {
		return 0, false, fmt.Errorf("window %q must start with +", value)
	}
	unit := body[len(body)-1:]
	count, err := strconv.Atoi(body[:len(body)-1])
	if err != nil || count <= 0 {
		return 0, false, fmt.Errorf("window %q must carry a positive count", value)
	}
	switch unit {
	case "h":
		return count, true, nil
	case "d":
		return count, false, nil
	case "w":
		return count * 7, false, nil
	default:
		return 0, false, fmt.Errorf("window %q must end in h, d, or w", value)
	}
}
