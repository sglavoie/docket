package models

import "time"

// DecisionCategory represents the type/category of a design decision.
type DecisionCategory string

const (
	DecisionCategoryDesign       DecisionCategory = "design"
	DecisionCategoryArchitecture DecisionCategory = "architecture"
	DecisionCategoryUX           DecisionCategory = "ux"
	DecisionCategoryTradeOff     DecisionCategory = "trade-off"
	DecisionCategoryScope        DecisionCategory = "scope"
	DecisionCategoryPerformance  DecisionCategory = "performance"
	DecisionCategorySecurity     DecisionCategory = "security"
)

// ValidDecisionCategories returns all valid decision category values.
func ValidDecisionCategories() []DecisionCategory {
	return []DecisionCategory{
		DecisionCategoryDesign,
		DecisionCategoryArchitecture,
		DecisionCategoryUX,
		DecisionCategoryTradeOff,
		DecisionCategoryScope,
		DecisionCategoryPerformance,
		DecisionCategorySecurity,
	}
}

// IsValidDecisionCategory reports whether s is a valid DecisionCategory.
func IsValidDecisionCategory(s string) bool {
	for _, c := range ValidDecisionCategories() {
		if string(c) == s {
			return true
		}
	}
	return false
}

// Decision represents a design/architecture decision captured on a task.
type Decision struct {
	Text      string           `json:"text"`
	Rationale string           `json:"rationale,omitempty"`
	Category  DecisionCategory `json:"category"`
	CreatedAt time.Time        `json:"created_at"`
	Resolved  bool             `json:"resolved,omitempty"`
}
