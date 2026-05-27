package scoring

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// MaxNormalizedScoreValue is the upper bound for normalized scores (0 to MaxNormalizedScoreValue).
const MaxNormalizedScoreValue = 300

// MaxScoreSentinel is returned when a formula's denominator is zero,
// ensuring zero-effort items sort to top.
const MaxScoreSentinel = 9999.0

// MaxNormalizedScore returns the maximum possible normalized score for a profile.
func MaxNormalizedScore(profile *ScoringProfile) int {
	_ = profile // reserved for future per-profile max
	return MaxNormalizedScoreValue
}

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
// Returns 0 when profile is nil.
func ComputeScore(task *models.Task, profile *ScoringProfile) float64 {
	if profile == nil {
		return 0
	}
	results := resolveAllDimensions(task, profile)
	return applyFormula(results, profile.Formula)
}

// ApplyFormula computes the final score from resolved dimension values using the given formula.
func ApplyFormula(results []DimensionResult, formula Formula) float64 {
	return applyFormula(results, formula)
}

// ScoreBreakdown returns per-dimension detail for display purposes.
// Returns an empty slice when profile is nil.
func ScoreBreakdown(task *models.Task, profile *ScoringProfile) []DimensionResult {
	if profile == nil {
		return nil
	}
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
// A nil task falls straight through to the dimension default so callers can
// inspect a profile's default behavior without constructing a fake Task.
func resolveDimension(task *models.Task, dim *ScoringDimension) (int, string) {
	if task != nil {
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
	case "bug.workaround":
		if task.BugAttrs != nil {
			return task.BugAttrs.Workaround
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
	case "story.persona":
		if task.StoryAttrs != nil {
			return task.StoryAttrs.Persona
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
//
// NaN results from Pow(negative, fractional) are treated as 0: a mathematically
// undefined value should not artificially boost a task to the top of the ranking.
// +Inf (from Pow(0, negative)) keeps the legacy MaxScoreSentinel semantic since
// it represents "zero effort, infinite value" — consistent with the divide-by-
// zero treatment in formulaRICE.
//
// An empty results slice returns 0 (matching formulaWeightedSum) rather than
// the neutral multiplicative identity 1: a profile with no dimensions has no
// signal, and returning 1 would silently rank such tasks above legitimate
// zero-scoring tasks.
func formulaMultiply(results []DimensionResult) float64 {
	if len(results) == 0 {
		return 0
	}
	product := 1.0
	for _, r := range results {
		if r.Weight == 1 {
			product *= float64(r.Value)
			continue
		}
		val := math.Pow(float64(r.Value), r.Weight)
		if math.IsNaN(val) {
			return 0
		}
		if math.IsInf(val, 0) {
			// 0 * Inf is NaN (undefined): an already-zero product must not
			// be artificially boosted to the sentinel by a later +Inf factor.
			if product == 0 {
				return 0
			}
			return MaxScoreSentinel
		}
		product *= val
	}
	return product
}

// formulaRICE: product of numerators / product of denominators.
// Weights are applied as exponents (Pow(value, weight)) so a Weight=1 dim
// contributes its raw value while a higher weight scales the dim geometrically
// — matching formulaMultiply's semantics, since RICE's numerator and
// denominator are themselves products. NaN from Pow(negative, fractional)
// short-circuits to 0 to keep undefined values from poisoning sort order.
//
// Division by zero yields a large score (zero effort = do it first), unless
// the numerator is also zero — 0/0 is mathematically undefined and should
// not artificially top-rank a task with no value AND no cost.
//
// A negative denominator product (possible when a dim has Min < 0 and the
// signed value survives Pow with an odd integer weight) would otherwise
// produce a negative quotient that breaks descending-score sort order. Treat
// it as 0 (no signal) rather than letting it leak into ranking.
//
// An empty results slice returns 0 rather than the neutral 1/1=1: a profile
// with no dimensions has no signal, matching formulaWeightedSum's behavior.
func formulaRICE(results []DimensionResult) float64 {
	if len(results) == 0 {
		return 0
	}
	num := 1.0
	den := 1.0
	for _, r := range results {
		factor := float64(r.Value)
		if r.Weight != 1 {
			factor = math.Pow(float64(r.Value), r.Weight)
			if math.IsNaN(factor) {
				return 0
			}
			if math.IsInf(factor, 0) {
				return MaxScoreSentinel
			}
		}
		switch r.Role {
		case RoleNumerator:
			num *= factor
		case RoleDenominator:
			den *= factor
		default:
			num *= factor // no role = numerator
		}
	}
	if den <= 0 {
		if den == 0 && num != 0 {
			return MaxScoreSentinel
		}
		return 0
	}
	return num / den
}

// formulaWSJF: sum of numerators / sum of denominators.
// Division by zero yields a large score (zero job size = do it first), unless
// the numerator is also zero — 0/0 is undefined and should not artificially
// top-rank an item with no value AND no cost.
//
// A negative denominator sum (possible when a dim has Min < 0 and the user
// records a negative value) would otherwise yield a negative quotient that
// breaks descending-score sort order; treat it as 0 (no signal).
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
	if den <= 0 {
		if den == 0 && num != 0 {
			return MaxScoreSentinel
		}
		return 0
	}
	return num / den
}

// clamp constrains a value to the [min, max] range.
// If min > max (misconfigured), they are swapped to avoid returning
// contradictory results.
func clamp(v, min, max int) int {
	if min > max {
		min, max = max, min
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// scoreWithStrategy builds DimensionResults using a per-dimension value selector and applies the formula.
func scoreWithStrategy(profile *ScoringProfile, selectValue func(dim ScoringDimension) int) float64 {
	results := make([]DimensionResult, len(profile.Dimensions))
	for i, dim := range profile.Dimensions {
		results[i] = DimensionResult{
			DimensionID:   dim.ID,
			DimensionName: dim.Name,
			Value:         selectValue(dim),
			Weight:        dim.Weight,
			Role:          dim.Role,
		}
	}
	return applyFormula(results, profile.Formula)
}

// MaxScore returns the theoretical maximum score for a profile.
// For each dimension, it uses Max for numerators and the smallest positive
// in-range value for denominators (negative or zero denominators would make
// the quotient flip sign or hit the divide-by-zero sentinel, neither of
// which represents the natural "maximum" of the configured range).
// Returns 0 when profile is nil.
func MaxScore(profile *ScoringProfile) float64 {
	if profile == nil {
		return 0
	}
	return scoreWithStrategy(profile, func(dim ScoringDimension) int {
		if dim.Role == RoleDenominator {
			if dim.Min > 0 {
				return dim.Min
			}
			if dim.Max > 0 {
				return 1
			}
			return 1
		}
		return dim.Max
	})
}

// MinScore returns the theoretical minimum score for a profile.
// For each dimension, it uses Min for numerators and Max for denominators.
// Returns 0 when profile is nil.
func MinScore(profile *ScoringProfile) float64 {
	if profile == nil {
		return 0
	}
	return scoreWithStrategy(profile, func(dim ScoringDimension) int {
		val := dim.Min
		if val == 0 && isMultiplicativeFormula(profile.Formula) {
			val = 1
		}
		if dim.Role == RoleDenominator {
			return dim.Max
		}
		return val
	})
}

// DefaultScore returns the score when all dimensions use their default values.
// Returns 0 when profile is nil.
func DefaultScore(profile *ScoringProfile) float64 {
	if profile == nil {
		return 0
	}
	return scoreWithStrategy(profile, func(dim ScoringDimension) int {
		return dim.Default
	})
}

// isMultiplicativeFormula returns true for formulas whose output grows
// exponentially with dimension values (product-based).
func isMultiplicativeFormula(f Formula) bool {
	return f == FormulaMultiply || f == FormulaICE || f == FormulaRICE
}

// ScorePercent normalizes a raw score to a 0-MaxNormalizedScoreValue scale using
// piecewise mapping centered on the default score (which maps to midpoint).
// Below default uses linear interpolation; above default uses logarithmic
// scaling for multiplicative formulas and linear for additive ones.
// Returns 0 when profile is nil. NaN scores are treated as 0 so a degenerate
// upstream calculation does not poison ranking; +Inf clamps to maxN, -Inf to 0.
func ScorePercent(score float64, profile *ScoringProfile) float64 {
	if profile == nil {
		return 0
	}
	if math.IsNaN(score) {
		return 0
	}
	if math.IsInf(score, 1) {
		return float64(MaxNormalizedScoreValue)
	}
	if math.IsInf(score, -1) {
		return 0
	}
	minS := MinScore(profile)
	maxS := MaxScore(profile)
	defS := DefaultScore(profile)
	maxN := float64(MaxNormalizedScoreValue)
	mid := maxN / 2

	// Degenerate: no range at all
	if maxS <= minS {
		return mid
	}

	var pct float64
	if score <= defS {
		// Below or at default: linear [minS, defS] -> [0, mid]
		if defS <= minS {
			// No below-default range: the only representable score in this
			// segment is defS itself, which must still map to mid (not 0).
			pct = mid
		} else {
			pct = (score - minS) / (defS - minS) * mid
		}
	} else {
		// Above default: [defS, maxS] -> [mid, maxN]
		if defS >= maxS {
			// No above-default range
			pct = maxN
		} else if isMultiplicativeFormula(profile.Formula) && defS > 0 && score > 0 {
			// Log scaling for product formulas: multiplicative scores grow
			// exponentially, so log-transform gives a perceptually linear bar.
			// When defS == 0 this branch is skipped and linear scaling is used
			// instead, because log(score/0) is undefined. That maps any positive
			// score to the linear upper half [mid, maxN], which is a reasonable
			// fallback ("infinitely above a zero default").
			pct = mid + math.Log(score/defS)/math.Log(maxS/defS)*mid
		} else {
			// Linear scaling for additive formulas
			pct = mid + (score-defS)/(maxS-defS)*mid
		}
	}

	// Clamp to [0, maxN]
	if pct < 0 {
		return 0
	}
	if pct > maxN {
		return maxN
	}
	return pct
}

// NormalizedScore computes the normalized 0-300 score for a task.
// Convenience wrapper around ComputeScore + ScorePercent.
// Returns 0 when profile is nil.
func NormalizedScore(task *models.Task, profile *ScoringProfile) float64 {
	if profile == nil {
		return 0
	}
	return ScorePercent(ComputeScore(task, profile), profile)
}

// FormatBreakdown returns a multi-line string showing each dimension's contribution.
func FormatBreakdown(results []DimensionResult, formula Formula, totalScore float64, normalizedScore float64) string {
	var lines []string
	for _, r := range results {
		roleStr := ""
		switch r.Role {
		case RoleNumerator:
			roleStr = " [num]"
		case RoleDenominator:
			roleStr = " [den]"
		default:
		}
		weightStr := ""
		if r.Weight != 1 {
			weightStr = fmt.Sprintf(" x%.1f", r.Weight)
		}
		lines = append(lines, fmt.Sprintf("  %s: %d (%s%s%s)", r.DimensionName, r.Value, r.Source, weightStr, roleStr))
	}
	lines = append(lines, fmt.Sprintf("  Formula: %s", formula))
	lines = append(lines, fmt.Sprintf("  Score: %d/%d (raw: %d)", int(normalizedScore), MaxNormalizedScoreValue, int(totalScore)))
	return joinLines(lines)
}

func joinLines(lines []string) string {
	var result strings.Builder
	for i, l := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(l)
	}
	return result.String()
}
