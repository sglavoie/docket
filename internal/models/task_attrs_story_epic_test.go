package models

import (
	"strings"
	"testing"
)

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
