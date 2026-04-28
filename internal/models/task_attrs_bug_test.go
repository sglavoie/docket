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