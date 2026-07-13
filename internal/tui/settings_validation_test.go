package tui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
)

func TestValidateIntInRange(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		min, max  int
		wantVal   int
		wantErr   bool
		errMsg    string
	}{
		{"valid in range", "5", "count", 0, 10, 5, false, ""},
		{"at min", "0", "count", 0, 10, 0, false, ""},
		{"at max", "10", "count", 0, 10, 10, false, ""},
		{"empty required", "", "count", 0, 10, 0, true, "count is required"},
		{"whitespace only", "  ", "count", 0, 10, 0, true, "count is required"},
		{"not a number", "abc", "count", 0, 10, 0, true, "count must be a number"},
		{"below min", "-1", "count", 0, 10, 0, true, "count must be 0-10"},
		{"above max", "11", "count", 0, 10, 0, true, "count must be 0-10"},
		{"unbounded max", "999999", "count", 0, math.MaxInt, 999999, false, ""},
		{"below min unbounded", "-1", "count", 0, math.MaxInt, 0, true, "count must be 0 or greater"},
		{"with whitespace", "  7  ", "count", 0, 10, 7, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateIntInRange(tt.input, tt.fieldName, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIntInRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("validateIntInRange() error = %q, want %q", err.Error(), tt.errMsg)
			}
			if got != tt.wantVal {
				t.Errorf("validateIntInRange() = %d, want %d", got, tt.wantVal)
			}
		})
	}
}

func TestValidateIntOptionalInRange(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		fieldName string
		min, max  int
		wantVal   int
		wantSet   bool
		wantErr   bool
	}{
		{"empty is ok", "", "hours", 0, 23, 0, false, false},
		{"whitespace empty", "  ", "hours", 0, 23, 0, false, false},
		{"valid value", "12", "hours", 0, 23, 12, true, false},
		{"not a number", "abc", "hours", 0, 23, 0, false, true},
		{"out of range", "25", "hours", 0, 23, 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, set, err := validateIntOptionalInRange(tt.input, tt.fieldName, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if set != tt.wantSet {
				t.Errorf("set = %v, want %v", set, tt.wantSet)
			}
			if got != tt.wantVal {
				t.Errorf("value = %d, want %d", got, tt.wantVal)
			}
		})
	}
}

func TestValidateFocusedField_SectionLimitReview(t *testing.T) {
	m := NewSettingsModel(config.NewConfig(), nil)
	m.focusIndex = subTabFieldStart(tabBehavior, 5) + 6
	m.sectionLimitReviewInput.SetValue("6a")

	m.validateFocusedField()

	if got, want := m.validationErr, "Review limit must be a number"; got != want {
		t.Fatalf("validationErr = %q, want %q", got, want)
	}

	rendered := m.renderBehaviorSectionLimitsSection()
	if !strings.Contains(rendered, "Review limit must be a number") {
		t.Fatalf("rendered section limits missing Review validation error:\n%s", rendered)
	}
}

func TestValidateFocusedField_UsesCurrentSettingsFieldLayout(t *testing.T) {
	tests := []struct {
		name       string
		focusIndex int
		mutate     func(*SettingsModel)
		wantErr    string
	}{
		{
			name:       "stale badge days",
			focusIndex: tabFieldStart[tabReview] + 4,
			mutate: func(m *SettingsModel) {
				m.staleTaskDaysInput.SetValue("bad")
			},
			wantErr: "stale badge days must be a number",
		},
		{
			name:       "popup timeout",
			focusIndex: subTabFieldStart(tabBehavior, 4),
			mutate: func(m *SettingsModel) {
				m.popupTimeoutInput.SetValue("31")
			},
			wantErr: "popup timeout must be 0-30",
		},
		{
			name:       "dashboard vertical split",
			focusIndex: subTabFieldStart(tabBehavior, 9) + 2,
			mutate: func(m *SettingsModel) {
				m.dashboardVertSplitInput.SetValue("9")
			},
			wantErr: "vertical split % must be 10-90",
		},
		{
			name:       "project stale days alert",
			focusIndex: subTabFieldStart(tabNotifications, 2) + 1,
			mutate: func(m *SettingsModel) {
				m.alertProjectStaleDaysInput.SetValue("nope")
			},
			wantErr: "project stale days must be a number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewSettingsModel(config.NewConfig(), nil)
			m.focusIndex = tt.focusIndex
			tt.mutate(m)

			m.validateFocusedField()

			if got := m.validationErr; got != tt.wantErr {
				t.Fatalf("validationErr = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestValidatePriorityTitleColors(t *testing.T) {
	tests := []struct {
		name    string
		p1      string
		wantErr bool
	}{
		{"valid 6-digit hex", "#ff6600", false},
		{"valid 6-digit no hash", "ff6600", false},
		{"valid 3-digit hex", "#abc", false},
		{"valid 3-digit no hash", "abc", false},
		{"empty is valid", "", false},
		{"whitespace only is valid", "  ", false},
		{"invalid hex", "gggggg", true},
		{"invalid format", "not-a-color", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p1 := textinput.New()
			p1.SetValue(tt.p1)
			m := &SettingsModel{
				priority1TitleInput: p1,
			}
			err := m.validatePriorityTitleColors()
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePriorityTitleColors() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// All 5 inputs: valid combination
	t.Run("all valid", func(t *testing.T) {
		m := newTestSettingsModelWithPriorities("#ffffff", "#d0d0d0", "#b8b8b8", "#9e9e9e", "#6a6a6a")
		if err := m.validatePriorityTitleColors(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// One invalid among the 5
	t.Run("p3 invalid", func(t *testing.T) {
		m := newTestSettingsModelWithPriorities("#ffffff", "#d0d0d0", "not-hex", "#9e9e9e", "#6a6a6a")
		if err := m.validatePriorityTitleColors(); err == nil {
			t.Error("expected error for invalid p3 color")
		}
	})
}

func newTestSettingsModelWithPriorities(p1, p2, p3, p4, p5 string) *SettingsModel {
	makeInput := func(val string) textinput.Model {
		ti := textinput.New()
		ti.SetValue(val)
		return ti
	}
	return &SettingsModel{
		priority1TitleInput: makeInput(p1),
		priority2TitleInput: makeInput(p2),
		priority3TitleInput: makeInput(p3),
		priority4TitleInput: makeInput(p4),
		priority5TitleInput: makeInput(p5),
	}
}

func TestValidateEditorCommand_FlaggedEditorString(t *testing.T) {
	// Create a fake editor binary so the PATH lookup succeeds.
	dir := t.TempDir()
	fakeEditor := filepath.Join(dir, "myeditor")
	if err := os.WriteFile(fakeEditor, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake editor: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", origPath) })
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("setenv PATH: %v", err)
	}

	m := &SettingsModel{}

	// "myeditor -u NORC" — executable found, extra flags must be accepted.
	if err := m.validateEditorCommand("myeditor -u NORC"); err != nil {
		t.Errorf("validateEditorCommand('myeditor -u NORC') unexpected error: %v", err)
	}

	// "myeditor" alone — still valid.
	if err := m.validateEditorCommand("myeditor"); err != nil {
		t.Errorf("validateEditorCommand('myeditor') unexpected error: %v", err)
	}

	// Unknown executable with flags — must still fail.
	if err := m.validateEditorCommand("ghosteditor --some-flag"); err == nil {
		t.Error("validateEditorCommand('ghosteditor --some-flag') expected error for unknown command")
	}
}

func TestValidateEditorCommand_AbsolutePathWithFlags(t *testing.T) {
	// Create a real executable in a temp dir so the absolute-path branch is exercised.
	dir := t.TempDir()
	fakeEditor := filepath.Join(dir, "myeditor")
	if err := os.WriteFile(fakeEditor, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake editor: %v", err)
	}

	m := &SettingsModel{}

	// "/tmp/.../myeditor -u NORC" — only the exe part is stat'd.
	editorStr := fakeEditor + " -u NORC"
	if err := m.validateEditorCommand(editorStr); err != nil {
		t.Errorf("validateEditorCommand(%q) unexpected error: %v", editorStr, err)
	}

	// Absolute path alone is also valid.
	if err := m.validateEditorCommand(fakeEditor); err != nil {
		t.Errorf("validateEditorCommand(%q) unexpected error: %v", fakeEditor, err)
	}
}
