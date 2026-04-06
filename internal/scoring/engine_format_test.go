package scoring

import "testing"

func TestFormatScore(t *testing.T) {
	if s := FormatScore(84.7); s != "84" {
		t.Errorf("expected '84', got '%s'", s)
	}
	if s := FormatScore(0.0); s != "0" {
		t.Errorf("expected '0', got '%s'", s)
	}
}

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
