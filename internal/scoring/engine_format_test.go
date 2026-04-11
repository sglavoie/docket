package scoring

import "testing"

func TestFormatBreakdown(t *testing.T) {
	results := []DimensionResult{
		{DimensionName: "Impact", Value: 4, Source: "manual", Weight: 2, Role: RoleNumerator},
		{DimensionName: "Effort", Value: 3, Source: "derived", Weight: 1, Role: RoleDenominator},
	}
	out := FormatBreakdown(results, FormulaRICE, 1.33, 55.0)
	if out == "" {
		t.Error("expected non-empty breakdown")
	}
}
