package tui

import (
	"math"
	"testing"

	"charm.land/bubbles/v2/textinput"
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

func TestHasDuplicateStage(t *testing.T) {
	m := &SettingsModel{
		stages: []string{"Brainstorm", "Design", "Implement", "Test"},
	}

	// Exact match
	if !m.hasDuplicateStage("Test", -1) {
		t.Error("expected duplicate for exact match 'Test'")
	}

	// Case-insensitive match
	if !m.hasDuplicateStage("test", -1) {
		t.Error("expected duplicate for case-insensitive 'test'")
	}

	// No match
	if m.hasDuplicateStage("Deploy", -1) {
		t.Error("expected no duplicate for 'Deploy'")
	}

	// Exclude self during rename (index 3 = "Test")
	if m.hasDuplicateStage("Test", 3) {
		t.Error("expected no duplicate when excluding self")
	}

	// Rename collides with different index
	if !m.hasDuplicateStage("Design", 3) {
		t.Error("expected duplicate when renaming to existing 'Design'")
	}
}

func TestCommitStageEdit_AddDuplicate(t *testing.T) {
	m := &SettingsModel{
		stages:        []string{"Brainstorm", "Design"},
		stageEditMode: true,
		stageAddMode:  true,
		stageInput:    textinput.New(),
	}
	m.stageInput.SetValue("Design")

	err := m.commitStageEdit()
	if err == nil {
		t.Fatal("expected error for duplicate stage add")
	}
	if len(m.stages) != 2 {
		t.Errorf("stages should not have changed, got %v", m.stages)
	}
	// Should stay in edit mode on error
	if !m.stageEditMode {
		t.Error("should remain in edit mode after duplicate error")
	}
}

func TestCommitStageEdit_AddNew(t *testing.T) {
	m := &SettingsModel{
		stages:        []string{"Brainstorm", "Design"},
		stageEditMode: true,
		stageAddMode:  true,
		stageInput:    textinput.New(),
	}
	m.stageInput.SetValue("Deploy")

	err := m.commitStageEdit()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.stages) != 3 || m.stages[2] != "Deploy" {
		t.Errorf("expected stages [Brainstorm Design Deploy], got %v", m.stages)
	}
}

func TestCommitStageEdit_RenameDuplicate(t *testing.T) {
	m := &SettingsModel{
		stages:        []string{"Brainstorm", "Design", "Test"},
		stageCursor:   2,
		stageEditMode: true,
		stageAddMode:  false,
		stageInput:    textinput.New(),
	}
	m.stageInput.SetValue("design") // case-insensitive dup

	err := m.commitStageEdit()
	if err == nil {
		t.Fatal("expected error for duplicate rename")
	}
	if m.stages[2] != "Test" {
		t.Errorf("stage should not have changed, got %q", m.stages[2])
	}
}

func TestCommitStageEdit_RenameSelf(t *testing.T) {
	m := &SettingsModel{
		stages:        []string{"Brainstorm", "Design"},
		stageCursor:   1,
		stageEditMode: true,
		stageAddMode:  false,
		stageInput:    textinput.New(),
	}
	m.stageInput.SetValue("Design") // same name is fine

	err := m.commitStageEdit()
	if err != nil {
		t.Fatalf("renaming to same value should succeed, got: %v", err)
	}
}
