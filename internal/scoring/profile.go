package scoring

import (
	"fmt"
	"strings"
)

// Formula represents the type of scoring calculation to apply.
type Formula string

const (
	FormulaWeightedSum Formula = "weighted_sum"
	FormulaRICE        Formula = "rice"
	FormulaWSJF        Formula = "wsjf"
	FormulaICE         Formula = "ice"
	FormulaMultiply    Formula = "multiply"
)

// ValidFormulas returns all valid formula types.
func ValidFormulas() []Formula {
	return []Formula{FormulaWeightedSum, FormulaRICE, FormulaWSJF, FormulaICE, FormulaMultiply}
}

// IsValid returns true if the formula is a recognized type.
func (f Formula) IsValid() bool {
	for _, v := range ValidFormulas() {
		if f == v {
			return true
		}
	}
	return false
}

// DimensionSource indicates where a dimension's value comes from.
type DimensionSource string

const (
	SourceManual  DimensionSource = "manual"
	SourceDerived DimensionSource = "derived"
)

// DimensionRole indicates the role of a dimension in RICE/WSJF-style formulas.
type DimensionRole string

const (
	RoleNone        DimensionRole = ""
	RoleNumerator   DimensionRole = "numerator"
	RoleDenominator DimensionRole = "denominator"
)

// DerivedMapping maps a task field path to integer values.
// Field is something like "bug.severity" or "priority".
// ValueMap converts string field values to integers (e.g., "critical" -> 5).
type DerivedMapping struct {
	Field    string         `json:"field"`
	ValueMap map[string]int `json:"value_map"`
}

// ScoringDimension represents one axis of a scoring profile.
type ScoringDimension struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Min            int             `json:"min"`
	Max            int             `json:"max"`
	Default        int             `json:"default"`
	Weight         float64         `json:"weight"`
	Source         DimensionSource `json:"source"`
	DerivedMapping *DerivedMapping `json:"derived_mapping,omitempty"`
	Role           DimensionRole   `json:"role,omitempty"`
}

// Validate checks that a dimension's configuration is consistent.
func (d *ScoringDimension) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("dimension ID is required")
	}
	if d.Name == "" {
		return fmt.Errorf("dimension %q: name is required", d.ID)
	}
	if d.Min > d.Max {
		return fmt.Errorf("dimension %q: min (%d) > max (%d)", d.ID, d.Min, d.Max)
	}
	if d.Default < d.Min || d.Default > d.Max {
		return fmt.Errorf("dimension %q: default (%d) outside range [%d, %d]", d.ID, d.Default, d.Min, d.Max)
	}
	if d.Source == SourceDerived && d.DerivedMapping == nil {
		return fmt.Errorf("dimension %q: derived source requires a derived_mapping", d.ID)
	}
	return nil
}

// ScoringProfile defines a complete scoring model with dimensions and a formula.
type ScoringProfile struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	UsageHint   string             `json:"usage_hint,omitempty"`
	Formula     Formula            `json:"formula"`
	Dimensions  []ScoringDimension `json:"dimensions"`
	Builtin     bool               `json:"builtin,omitempty"`
}

// Validate checks that a profile is well-formed: all dimensions are valid,
// the formula is recognized, and role-based formulas have the right structure.
func (p *ScoringProfile) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("profile ID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("profile %q: name is required", p.ID)
	}
	if !p.Formula.IsValid() {
		return fmt.Errorf("profile %q: unknown formula %q", p.ID, p.Formula)
	}
	if len(p.Dimensions) == 0 {
		return fmt.Errorf("profile %q: at least one dimension is required", p.ID)
	}

	ids := make(map[string]bool)
	for i := range p.Dimensions {
		if err := p.Dimensions[i].Validate(); err != nil {
			return fmt.Errorf("profile %q: %w", p.ID, err)
		}
		if ids[p.Dimensions[i].ID] {
			return fmt.Errorf("profile %q: duplicate dimension ID %q", p.ID, p.Dimensions[i].ID)
		}
		ids[p.Dimensions[i].ID] = true
	}

	if p.Formula == FormulaRICE || p.Formula == FormulaWSJF {
		hasNum, hasDen := false, false
		for _, d := range p.Dimensions {
			if d.Role == RoleNumerator {
				hasNum = true
			}
			if d.Role == RoleDenominator {
				hasDen = true
			}
		}
		if !hasNum {
			return fmt.Errorf("profile %q: %s formula requires at least one numerator dimension", p.ID, p.Formula)
		}
		if !hasDen {
			return fmt.Errorf("profile %q: %s formula requires at least one denominator dimension", p.ID, p.Formula)
		}
	}

	return nil
}

// GetDimension returns a dimension by ID, or nil if not found.
func (p *ScoringProfile) GetDimension(id string) *ScoringDimension {
	for i := range p.Dimensions {
		if p.Dimensions[i].ID == id {
			return &p.Dimensions[i]
		}
	}
	return nil
}

// NewRICEProfile creates a preset RICE scoring profile.
// RICE = (Reach * Impact * Confidence) / Effort
func NewRICEProfile() ScoringProfile {
	return ScoringProfile{
		ID:          "rice",
		Name:        "RICE",
		Description: "Reach * Impact * Confidence / Effort",
		UsageHint:   "Best for product teams evaluating feature impact against implementation effort.",
		Formula:     FormulaRICE,
		Builtin:     true,
		Dimensions: []ScoringDimension{
			{
				ID: "reach", Name: "Reach", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceManual, Role: RoleNumerator,
			},
			{
				ID: "impact", Name: "Impact", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceManual, Role: RoleNumerator,
			},
			{
				ID: "confidence", Name: "Confidence", Min: 1, Max: 5, Default: 3,
				Weight: 1, Source: SourceManual, Role: RoleNumerator,
			},
			{
				ID: "effort", Name: "Effort", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceDerived, Role: RoleDenominator,
				DerivedMapping: &DerivedMapping{
					Field:    "task.effort",
					ValueMap: map[string]int{"small": 2, "medium": 5, "large": 8, "unknown": 5},
				},
			},
		},
	}
}

// NewWSJFProfile creates a preset WSJF scoring profile.
// WSJF = (Business Value + Time Criticality + Risk Reduction) / Job Size
func NewWSJFProfile() ScoringProfile {
	return ScoringProfile{
		ID:          "wsjf",
		Name:        "WSJF",
		Description: "(Business Value + Time Criticality + Risk Reduction) / Job Size",
		UsageHint:   "Best for SAFe/agile environments prioritizing by cost of delay relative to job size.",
		Formula:     FormulaWSJF,
		Builtin:     true,
		Dimensions: []ScoringDimension{
			{
				ID: "business_value", Name: "Business Value", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceManual, Role: RoleNumerator,
			},
			{
				ID: "time_criticality", Name: "Time Criticality", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceManual, Role: RoleNumerator,
			},
			{
				ID: "risk_reduction", Name: "Risk Reduction", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceManual, Role: RoleNumerator,
			},
			{
				ID: "job_size", Name: "Job Size", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceDerived, Role: RoleDenominator,
				DerivedMapping: &DerivedMapping{
					Field:    "story.points",
					ValueMap: map[string]int{}, // direct passthrough for numeric fields
				},
			},
		},
	}
}

// NewICEProfile creates a preset ICE scoring profile.
// ICE = Impact * Confidence * Ease (product formula)
func NewICEProfile() ScoringProfile {
	return ScoringProfile{
		ID:          "ice",
		Name:        "ICE",
		Description: "Impact * Confidence * Ease",
		UsageHint:   "Best for quick, lightweight prioritization with simple subjective dimensions.",
		Formula:     FormulaMultiply,
		Builtin:     true,
		Dimensions: []ScoringDimension{
			{
				ID: "impact", Name: "Impact", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceManual,
			},
			{
				ID: "confidence", Name: "Confidence", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceManual,
			},
			{
				ID: "ease", Name: "Ease", Min: 1, Max: 10, Default: 5,
				Weight: 1, Source: SourceManual,
			},
		},
	}
}

// PresetProfiles returns all built-in scoring profile factories.
func PresetProfiles() map[string]func() ScoringProfile {
	return map[string]func() ScoringProfile{
		"rice": NewRICEProfile,
		"wsjf": NewWSJFProfile,
		"ice":  NewICEProfile,
	}
}

// PresetNames returns the names of available presets.
func PresetNames() []string {
	return []string{"rice", "wsjf", "ice"}
}

// FormulaDescription returns a human-readable description of a formula.
func FormulaDescription(f Formula) string {
	switch f {
	case FormulaWeightedSum:
		return "Sum of (dimension * weight) for each dimension"
	case FormulaRICE:
		return "Product of numerators / product of denominators"
	case FormulaWSJF:
		return "Sum of numerators / sum of denominators"
	case FormulaICE:
		return "Product of all dimensions"
	case FormulaMultiply:
		return "Product of all dimensions"
	default:
		return string(f)
	}
}

// FormatProfileSummary returns a single-line summary of a profile.
func FormatProfileSummary(p *ScoringProfile) string {
	dimNames := make([]string, len(p.Dimensions))
	for i, d := range p.Dimensions {
		dimNames[i] = d.Name
	}
	return fmt.Sprintf("%s (%s) [%s]", p.Name, p.Formula, strings.Join(dimNames, ", "))
}
