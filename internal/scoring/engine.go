package scoring

import (
	"fmt"
	"math"
	"strconv"

	"github.com/sglavoie/tt/internal/models"
)

// DimensionResult holds the resolved value and metadata for a single dimension.
type DimensionResult struct {
	DimensionID   string
	DimensionName string
	Value         int
	Source        string // "manual", "derived", or "default"
	Weight        float64
	Role          DimensionRole
}

// ComputeScore calculates the composite score for a task using the given profile.
// Returns the score as a float64. The displayed score is typically truncated to int.
func ComputeScore(task *models.Task, profile *ScoringProfile) float64 {
	results := resolveAllDimensions(task, profile)
	return applyFormula(results, profile.Formula)
}

// ApplyFormula computes the final score from resolved dimension values using the given formula.
func ApplyFormula(results []DimensionResult, formula Formula) float64 {
	return applyFormula(results, formula)
}

// ScoreBreakdown returns per-dimension detail for display purposes.
func ScoreBreakdown(task *models.Task, profile *ScoringProfile) []DimensionResult {
	return resolveAllDimensions(task, profile)
}

// resolveAllDimensions resolves each dimension value using the priority:
// manual override -> derived from task fields -> default.
func resolveAllDimensions(task *models.Task, profile *ScoringProfile) []DimensionResult {
	results := make([]DimensionResult, len(profile.Dimensions))
	for i, dim := range profile.Dimensions {
		value, source := resolveDimension(task, &dim)
		results[i] = DimensionResult{
			DimensionID:   dim.ID,
			DimensionName: dim.Name,
			Value:         value,
			Source:        source,
			Weight:        dim.Weight,
			Role:          dim.Role,
		}
	}
	return results
}

// resolveDimension resolves a single dimension value for a task.
// Priority: manual override in ScoreDimensions -> derived mapping -> default.
func resolveDimension(task *models.Task, dim *ScoringDimension) (int, string) {
	// Check for manual override
	if task.ScoreDimensions != nil {
		if val, ok := task.ScoreDimensions[dim.ID]; ok {
			return clamp(val, dim.Min, dim.Max), "manual"
		}
	}

	// Try derived mapping
	if dim.Source == SourceDerived && dim.DerivedMapping != nil {
		if val, ok := resolveField(task, dim.DerivedMapping); ok {
			return clamp(val, dim.Min, dim.Max), "derived"
		}
	}

	return dim.Default, "default"
}

// resolveField extracts an integer value from a task field using a DerivedMapping.
func resolveField(task *models.Task, dm *DerivedMapping) (int, bool) {
	fieldValue := getTaskFieldValue(task, dm.Field)
	if fieldValue == "" {
		return 0, false
	}

	// If ValueMap is provided and non-empty, look up the string value
	if len(dm.ValueMap) > 0 {
		if mapped, ok := dm.ValueMap[fieldValue]; ok {
			return mapped, true
		}
		return 0, false
	}

	// Empty ValueMap means direct numeric passthrough
	if val, err := strconv.Atoi(fieldValue); err == nil {
		return val, true
	}
	return 0, false
}

// getTaskFieldValue extracts a string value from a task field by path.
// Supported paths: "priority", "bug.severity", "bug.reproducible", "bug.workaround",
// "task.effort", "task.worktype", "idea.effort", "idea.value", "idea.category",
// "story.points", "story.persona", "epic.scope", "epic.milestone".
func getTaskFieldValue(task *models.Task, field string) string {
	switch field {
	case "priority":
		return strconv.Itoa(task.Priority)
	case "bug.severity":
		if task.BugAttrs != nil {
			return string(task.BugAttrs.Severity)
		}
	case "bug.reproducible":
		if task.BugAttrs != nil {
			return string(task.BugAttrs.Reproducible)
		}
	case "task.effort":
		if task.TaskAttrs != nil {
			return string(task.TaskAttrs.Effort)
		}
	case "task.worktype":
		if task.TaskAttrs != nil {
			return string(task.TaskAttrs.WorkType)
		}
	case "idea.effort":
		if task.IdeaAttrs != nil {
			return string(task.IdeaAttrs.Effort)
		}
	case "idea.value":
		if task.IdeaAttrs != nil {
			return string(task.IdeaAttrs.Value)
		}
	case "idea.category":
		if task.IdeaAttrs != nil {
			return string(task.IdeaAttrs.Category)
		}
	case "story.points":
		if task.StoryAttrs != nil {
			return strconv.Itoa(task.StoryAttrs.StoryPoints)
		}
	case "epic.scope":
		if task.EpicAttrs != nil {
			return string(task.EpicAttrs.Scope)
		}
	case "epic.milestone":
		if task.EpicAttrs != nil {
			return task.EpicAttrs.Milestone
		}
	}
	return ""
}

// applyFormula computes the final score from resolved dimension values.
func applyFormula(results []DimensionResult, formula Formula) float64 {
	switch formula {
	case FormulaWeightedSum:
		return formulaWeightedSum(results)
	case FormulaMultiply:
		return formulaMultiply(results)
	case FormulaRICE:
		return formulaRICE(results)
	case FormulaWSJF:
		return formulaWSJF(results)
	case FormulaICE:
		return formulaMultiply(results) // ICE is a product formula
	default:
		return formulaWeightedSum(results)
	}
}

// formulaWeightedSum: sum of (value * weight) for all dimensions.
func formulaWeightedSum(results []DimensionResult) float64 {
	var total float64
	for _, r := range results {
		total += float64(r.Value) * r.Weight
	}
	return total
}

// formulaMultiply: product of all dimension values (weights applied as exponents).
func formulaMultiply(results []DimensionResult) float64 {
	product := 1.0
	for _, r := range results {
		if r.Weight == 1 {
			product *= float64(r.Value)
		} else {
			product *= math.Pow(float64(r.Value), r.Weight)
		}
	}
	return product
}

// formulaRICE: product of numerators / product of denominators.
// Division by zero yields a large score (zero effort = do it first).
func formulaRICE(results []DimensionResult) float64 {
	num := 1.0
	den := 1.0
	for _, r := range results {
		switch r.Role {
		case RoleNumerator:
			num *= float64(r.Value)
		case RoleDenominator:
			den *= float64(r.Value)
		default:
			num *= float64(r.Value) // no role = numerator
		}
	}
	if den == 0 {
		return 9999
	}
	return num / den
}

// formulaWSJF: sum of numerators / sum of denominators.
// Division by zero yields a large score.
func formulaWSJF(results []DimensionResult) float64 {
	var num, den float64
	for _, r := range results {
		switch r.Role {
		case RoleNumerator:
			num += float64(r.Value) * r.Weight
		case RoleDenominator:
			den += float64(r.Value) * r.Weight
		default:
			num += float64(r.Value) * r.Weight
		}
	}
	if den == 0 {
		return 9999
	}
	return num / den
}

// clamp constrains a value to the [min, max] range.
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// FormatScore formats a float score as a display string (truncated to integer).
func FormatScore(score float64) string {
	return fmt.Sprintf("%d", int(score))
}

// FormatBreakdown returns a multi-line string showing each dimension's contribution.
func FormatBreakdown(results []DimensionResult, formula Formula, totalScore float64) string {
	var lines []string
	for _, r := range results {
		roleStr := ""
		if r.Role == RoleNumerator {
			roleStr = " [num]"
		} else if r.Role == RoleDenominator {
			roleStr = " [den]"
		}
		weightStr := ""
		if r.Weight != 1 {
			weightStr = fmt.Sprintf(" x%.1f", r.Weight)
		}
		lines = append(lines, fmt.Sprintf("  %s: %d (%s%s%s)", r.DimensionName, r.Value, r.Source, weightStr, roleStr))
	}
	lines = append(lines, fmt.Sprintf("  Formula: %s", formula))
	lines = append(lines, fmt.Sprintf("  Score: %d", int(totalScore)))
	return joinLines(lines)
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
