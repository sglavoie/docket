package models

import (
	"strings"
	"testing"
)

func TestBugReproducible_IsValid(t *testing.T) {
	tests := []struct {
		value BugReproducible
		valid bool
	}{
		{BugReproducibleYes, true},
		{BugReproducibleNo, true},
		{BugReproducibleSometimes, true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.value), func(t *testing.T) {
			if got := tc.value.IsValid(); got != tc.valid {
				t.Errorf("BugReproducible(%q).IsValid() = %v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}

func TestBugSeverity_IsValid(t *testing.T) {
	tests := []struct {
		value BugSeverity
		valid bool
	}{
		{BugSeverityCritical, true},
		{BugSeverityMajor, true},
		{BugSeverityMinor, true},
		{BugSeverityCosmetic, true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.value), func(t *testing.T) {
			if got := tc.value.IsValid(); got != tc.valid {
				t.Errorf("BugSeverity(%q).IsValid() = %v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}

func TestBugAttrs_Validate(t *testing.T) {
	tests := []struct {
		name    string
		attrs   *BugAttrs
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
			attrs: &BugAttrs{
				Reproducible: BugReproducibleYes,
				Severity:     BugSeverityMajor,
				Workaround:   "Use different browser",
			},
			wantErr: false,
		},
		{
			name:    "empty attrs",
			attrs:   &BugAttrs{},
			wantErr: false,
		},
		{
			name: "invalid reproducible",
			attrs: &BugAttrs{
				Reproducible: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid bug.reproducible",
		},
		{
			name: "invalid severity",
			attrs: &BugAttrs{
				Severity: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid bug.severity",
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

func TestBugAttrs_IsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		attrs *BugAttrs
		empty bool
	}{
		{"nil", nil, true},
		{"empty struct", &BugAttrs{}, true},
		{"reproducible set", &BugAttrs{Reproducible: BugReproducibleYes}, false},
		{"severity set", &BugAttrs{Severity: BugSeverityMajor}, false},
		{"workaround set", &BugAttrs{Workaround: "test"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.attrs.IsEmpty(); got != tc.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tc.empty)
			}
		})
	}
}

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

func TestStoryAttrs_Validate(t *testing.T) {
	tests := []struct {
		name    string
		attrs   *StoryAttrs
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
			attrs: &StoryAttrs{
				StoryPoints: 5,
				Persona:     "Admin user",
			},
			wantErr: false,
		},
		{
			name: "zero points",
			attrs: &StoryAttrs{
				StoryPoints: 0,
			},
			wantErr: false,
		},
		{
			name: "negative points",
			attrs: &StoryAttrs{
				StoryPoints: -1,
			},
			wantErr: true,
			errMsg:  "invalid story.points",
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

func TestStoryAttrs_IsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		attrs *StoryAttrs
		empty bool
	}{
		{"nil", nil, true},
		{"empty struct", &StoryAttrs{}, true},
		{"points set", &StoryAttrs{StoryPoints: 3}, false},
		{"persona set", &StoryAttrs{Persona: "test"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.attrs.IsEmpty(); got != tc.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tc.empty)
			}
		})
	}
}

func TestEpicScope_IsValid(t *testing.T) {
	tests := []struct {
		value EpicScope
		valid bool
	}{
		{EpicScopeDefined, true},
		{EpicScopeEvolving, true},
		{EpicScopeUndefined, true},
		{"invalid", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(string(tc.value), func(t *testing.T) {
			if got := tc.value.IsValid(); got != tc.valid {
				t.Errorf("EpicScope(%q).IsValid() = %v, want %v", tc.value, got, tc.valid)
			}
		})
	}
}

func TestEpicAttrs_Validate(t *testing.T) {
	tests := []struct {
		name    string
		attrs   *EpicAttrs
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
			attrs: &EpicAttrs{
				Scope:          EpicScopeDefined,
				Milestone:      "Q1 2025",
				SuccessMetrics: "100 active users",
			},
			wantErr: false,
		},
		{
			name: "invalid scope",
			attrs: &EpicAttrs{
				Scope: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid epic.scope",
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

func TestEpicAttrs_IsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		attrs *EpicAttrs
		empty bool
	}{
		{"nil", nil, true},
		{"empty struct", &EpicAttrs{}, true},
		{"scope set", &EpicAttrs{Scope: EpicScopeDefined}, false},
		{"milestone set", &EpicAttrs{Milestone: "Q1"}, false},
		{"metrics set", &EpicAttrs{SuccessMetrics: "100 users"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.attrs.IsEmpty(); got != tc.empty {
				t.Errorf("IsEmpty() = %v, want %v", got, tc.empty)
			}
		})
	}
}

func TestTask_ClearTypeAttrs(t *testing.T) {
	task := &Task{
		BugAttrs:   &BugAttrs{Severity: BugSeverityMajor},
		IdeaAttrs:  &IdeaAttrs{Category: IdeaCategoryFeature},
		TaskAttrs:  &TaskAttrs{Effort: EffortSmall},
		StoryAttrs: &StoryAttrs{StoryPoints: 5},
		EpicAttrs:  &EpicAttrs{Scope: EpicScopeDefined},
	}

	task.ClearTypeAttrs()

	if task.BugAttrs != nil {
		t.Error("BugAttrs should be nil after ClearTypeAttrs()")
	}
	if task.IdeaAttrs != nil {
		t.Error("IdeaAttrs should be nil after ClearTypeAttrs()")
	}
	if task.TaskAttrs != nil {
		t.Error("TaskAttrs should be nil after ClearTypeAttrs()")
	}
	if task.StoryAttrs != nil {
		t.Error("StoryAttrs should be nil after ClearTypeAttrs()")
	}
	if task.EpicAttrs != nil {
		t.Error("EpicAttrs should be nil after ClearTypeAttrs()")
	}
}
