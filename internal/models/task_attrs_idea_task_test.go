package models

import (
	"strings"
	"testing"
)

func TestIdeaCategory_IsValid(t *testing.T) {
	tests := []struct {
		value IdeaCategory
		valid bool
	}{
		{IdeaCategoryFeature, true},
		{IdeaCategoryImprovement, true},
		{IdeaCategoryRefactor, true},
		{IdeaCategoryResearch, true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.value), func(t *testing.T) {
			if got := tc.value.IsValid(); got != tc.valid {
				t.Errorf("IdeaCategory(%q).IsValid() = %v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}

func TestEffortEstimate_IsValid(t *testing.T) {
	tests := []struct {
		value EffortEstimate
		valid bool
	}{
		{EffortSmall, true},
		{EffortMedium, true},
		{EffortLarge, true},
		{EffortUnknown, true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.value), func(t *testing.T) {
			if got := tc.value.IsValid(); got != tc.valid {
				t.Errorf("EffortEstimate(%q).IsValid() = %v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}

func TestIdeaValue_IsValid(t *testing.T) {
	tests := []struct {
		value IdeaValue
		valid bool
	}{
		{IdeaValueLow, true},
		{IdeaValueMedium, true},
		{IdeaValueHigh, true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.value), func(t *testing.T) {
			if got := tc.value.IsValid(); got != tc.valid {
				t.Errorf("IdeaValue(%q).IsValid() = %v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}

func TestIdeaAttrs_Validate(t *testing.T) {
	tests := []struct {
		name    string
		attrs   *IdeaAttrs
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil attrs",
			attrs:   nil,
			wantErr: false,
		},
		{
			name: "valid attrs",
			attrs: &IdeaAttrs{
				Category:  IdeaCategoryFeature,
				Effort:    EffortMedium,
				Value:     IdeaValueHigh,
				Validated: true,
			},
			wantErr: false,
		},
		{
			name: "invalid category",
			attrs: &IdeaAttrs{
				Category: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid idea.category",
		},
		{
			name: "invalid effort",
			attrs: &IdeaAttrs{
				Effort: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid idea.effort",
		},
		{
			name: "invalid value",
			attrs: &IdeaAttrs{
				Value: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid idea.value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.attrs.Validate()
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tc.errMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIdeaAttrs_IsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		attrs *IdeaAttrs
		empty bool
	}{
		{"nil", nil, true},
		{"empty struct", &IdeaAttrs{}, true},
		{"category set", &IdeaAttrs{Category: IdeaCategoryFeature}, false},
		{"effort set", &IdeaAttrs{Effort: EffortSmall}, false},
		{"value set", &IdeaAttrs{Value: IdeaValueHigh}, false},
		{"validated true", &IdeaAttrs{Validated: true}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.attrs.IsEmpty(); got != tc.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tc.empty)
			}
		})
	}
}

func TestWorkType_IsValid(t *testing.T) {
	tests := []struct {
		value WorkType
		valid bool
	}{
		{WorkTypeDevelopment, true},
		{WorkTypeDocumentation, true},
		{WorkTypeTesting, true},
		{WorkTypeDevOps, true},
		{WorkTypeDesign, true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.value), func(t *testing.T) {
			if got := tc.value.IsValid(); got != tc.valid {
				t.Errorf("WorkType(%q).IsValid() = %v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}

func TestTaskAttrs_Validate(t *testing.T) {
	tests := []struct {
		name    string
		attrs   *TaskAttrs
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil attrs",
			attrs:   nil,
			wantErr: false,
		},
		{
			name: "valid attrs",
			attrs: &TaskAttrs{
				Effort:   EffortSmall,
				WorkType: WorkTypeDevelopment,
			},
			wantErr: false,
		},
		{
			name: "invalid effort",
			attrs: &TaskAttrs{
				Effort: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid task.effort",
		},
		{
			name: "invalid worktype",
			attrs: &TaskAttrs{
				WorkType: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid task.worktype",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.attrs.Validate()
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tc.errMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTaskAttrs_IsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		attrs *TaskAttrs
		empty bool
	}{
		{"nil", nil, true},
		{"empty struct", &TaskAttrs{}, true},
		{"effort set", &TaskAttrs{Effort: EffortSmall}, false},
		{"worktype set", &TaskAttrs{WorkType: WorkTypeTesting}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.attrs.IsEmpty(); got != tc.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tc.empty)
			}
		})
	}
}
