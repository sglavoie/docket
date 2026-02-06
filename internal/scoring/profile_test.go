package scoring

import (
	"testing"
)

func TestFormulaIsValid(t *testing.T) {
	tests := []struct {
		formula Formula
		valid   bool
	}{
		{FormulaWeightedSum, true},
		{FormulaRICE, true},
		{FormulaWSJF, true},
		{FormulaICE, true},
		{FormulaMultiply, true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.formula.IsValid(); got != tt.valid {
			t.Errorf("Formula(%q).IsValid() = %v, want %v", tt.formula, got, tt.valid)
		}
	}
}

func TestDimensionValidate(t *testing.T) {
	tests := []struct {
		name    string
		dim     ScoringDimension
		wantErr bool
	}{
		{
			name:    "valid manual dimension",
			dim:     ScoringDimension{ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
			wantErr: false,
		},
		{
			name:    "empty ID",
			dim:     ScoringDimension{ID: "", Name: "Impact", Min: 1, Max: 5, Default: 3, Source: SourceManual},
			wantErr: true,
		},
		{
			name:    "empty name",
			dim:     ScoringDimension{ID: "impact", Name: "", Min: 1, Max: 5, Default: 3, Source: SourceManual},
			wantErr: true,
		},
		{
			name:    "min > max",
			dim:     ScoringDimension{ID: "impact", Name: "Impact", Min: 10, Max: 5, Default: 5, Source: SourceManual},
			wantErr: true,
		},
		{
			name:    "default below min",
			dim:     ScoringDimension{ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 0, Source: SourceManual},
			wantErr: true,
		},
		{
			name:    "default above max",
			dim:     ScoringDimension{ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 6, Source: SourceManual},
			wantErr: true,
		},
		{
			name:    "derived without mapping",
			dim:     ScoringDimension{ID: "effort", Name: "Effort", Min: 1, Max: 5, Default: 3, Source: SourceDerived},
			wantErr: true,
		},
		{
			name: "derived with mapping",
			dim: ScoringDimension{
				ID: "effort", Name: "Effort", Min: 1, Max: 5, Default: 3,
				Source:         SourceDerived,
				DerivedMapping: &DerivedMapping{Field: "task.effort", ValueMap: map[string]int{"small": 1}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dim.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProfileValidate(t *testing.T) {
	tests := []struct {
		name    string
		profile ScoringProfile
		wantErr bool
	}{
		{
			name: "valid weighted sum",
			profile: ScoringProfile{
				ID: "test", Name: "Test", Formula: FormulaWeightedSum,
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
				},
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			profile: ScoringProfile{
				ID: "", Name: "Test", Formula: FormulaWeightedSum,
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
				},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			profile: ScoringProfile{
				ID: "test", Name: "", Formula: FormulaWeightedSum,
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid formula",
			profile: ScoringProfile{
				ID: "test", Name: "Test", Formula: "bogus",
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
				},
			},
			wantErr: true,
		},
		{
			name:    "no dimensions",
			profile: ScoringProfile{ID: "test", Name: "Test", Formula: FormulaWeightedSum},
			wantErr: true,
		},
		{
			name: "duplicate dimension IDs",
			profile: ScoringProfile{
				ID: "test", Name: "Test", Formula: FormulaWeightedSum,
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
					{ID: "a", Name: "B", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual},
				},
			},
			wantErr: true,
		},
		{
			name: "RICE missing denominator",
			profile: ScoringProfile{
				ID: "test", Name: "Test", Formula: FormulaRICE,
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual, Role: RoleNumerator},
				},
			},
			wantErr: true,
		},
		{
			name: "RICE missing numerator",
			profile: ScoringProfile{
				ID: "test", Name: "Test", Formula: FormulaRICE,
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual, Role: RoleDenominator},
				},
			},
			wantErr: true,
		},
		{
			name: "RICE valid",
			profile: ScoringProfile{
				ID: "test", Name: "Test", Formula: FormulaRICE,
				Dimensions: []ScoringDimension{
					{ID: "a", Name: "A", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual, Role: RoleNumerator},
					{ID: "b", Name: "B", Min: 1, Max: 5, Default: 3, Weight: 1, Source: SourceManual, Role: RoleDenominator},
				},
			},
			wantErr: false,
		},
		{
			name: "WSJF valid",
			profile: ScoringProfile{
				ID: "test", Name: "Test", Formula: FormulaWSJF,
				Dimensions: []ScoringDimension{
					{ID: "bv", Name: "BV", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleNumerator},
					{ID: "js", Name: "JS", Min: 1, Max: 10, Default: 5, Weight: 1, Source: SourceManual, Role: RoleDenominator},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPresetProfilesValid(t *testing.T) {
	for name, factory := range PresetProfiles() {
		p := factory()
		if err := p.Validate(); err != nil {
			t.Errorf("preset %q is invalid: %v", name, err)
		}
	}
}

func TestGetDimension(t *testing.T) {
	p := NewRICEProfile()
	if d := p.GetDimension("reach"); d == nil {
		t.Error("expected to find 'reach' dimension")
	}
	if d := p.GetDimension("nonexistent"); d != nil {
		t.Error("expected nil for nonexistent dimension")
	}
}

func TestPresetProfilesHaveUsageHint(t *testing.T) {
	for name, factory := range PresetProfiles() {
		p := factory()
		if p.UsageHint == "" {
			t.Errorf("preset %q has empty UsageHint", name)
		}
	}
}

func TestFormatProfileSummary(t *testing.T) {
	p := NewICEProfile()
	s := FormatProfileSummary(&p)
	if s == "" {
		t.Error("expected non-empty summary")
	}
}
