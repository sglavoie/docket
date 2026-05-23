package tterrors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	testCases := []struct {
		name     string
		sentinel error
		message  string
	}{
		{"ErrProjectNotFound", ErrProjectNotFound, "project not found"},
		{"ErrNoCurrentProject", ErrNoCurrentProject, "no current project selected"},
		{"ErrProjectExists", ErrProjectExists, "project already exists"},
		{"ErrTaskNotFound", ErrTaskNotFound, "task not found"},
		{"ErrInvalidTaskType", ErrInvalidTaskType, "invalid task type"},
		{"ErrInvalidParent", ErrInvalidParent, "invalid parent task"},
		{"ErrCircularDep", ErrCircularDep, "circular dependency detected"},
		{"ErrConfigNotFound", ErrConfigNotFound, "config file not found"},
		{"ErrConfigSaveFailed", ErrConfigSaveFailed, "failed to save config"},
		{"ErrBackupNotFound", ErrBackupNotFound, "backup not found"},
		{"ErrParseError", ErrParseError, "failed to parse markdown"},
		{"ErrSaveFailed", ErrSaveFailed, "failed to save project"},
		{"ErrReadFailed", ErrReadFailed, "failed to read file"},
		{"ErrWriteFailed", ErrWriteFailed, "failed to write file"},
		{"ErrRequiredField", ErrRequiredField, "required field missing"},
		{"ErrInvalidValue", ErrInvalidValue, "invalid value"},
		{"ErrNoChanges", ErrNoChanges, "no changes specified"},
		{"ErrSelfDependency", ErrSelfDependency, "a task cannot depend on itself"},
		{"ErrDuplicateDep", ErrDuplicateDep, "duplicate dependency"},
		{"ErrAmbiguousIdentifier", ErrAmbiguousIdentifier, "ambiguous identifier"},
		{"ErrCanceled", ErrCanceled, "canceled"},
		{"ErrEmptyTitle", ErrEmptyTitle, "blocker title cannot be empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.sentinel.Error() != tc.message {
				t.Errorf("expected error message %q, got %q", tc.message, tc.sentinel.Error())
			}
		})
	}
}

func TestErrorsIsWithWrappedErrors(t *testing.T) {
	testCases := []struct {
		name     string
		sentinel error
	}{
		{"ErrProjectNotFound", ErrProjectNotFound},
		{"ErrNoCurrentProject", ErrNoCurrentProject},
		{"ErrProjectExists", ErrProjectExists},
		{"ErrTaskNotFound", ErrTaskNotFound},
		{"ErrInvalidTaskType", ErrInvalidTaskType},
		{"ErrInvalidParent", ErrInvalidParent},
		{"ErrCircularDep", ErrCircularDep},
		{"ErrConfigNotFound", ErrConfigNotFound},
		{"ErrConfigSaveFailed", ErrConfigSaveFailed},
		{"ErrBackupNotFound", ErrBackupNotFound},
		{"ErrParseError", ErrParseError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("additional context: %w", tc.sentinel)
			if !errors.Is(wrapped, tc.sentinel) {
				t.Errorf("errors.Is should return true for wrapped %s", tc.name)
			}
		})
	}
}

func TestErrorsIsWithDoubleWrappedErrors(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", ErrProjectNotFound))
	if !errors.Is(wrapped, ErrProjectNotFound) {
		t.Error("errors.Is should work with double-wrapped errors")
	}
}

func TestErrorsIsWithDifferentErrors(t *testing.T) {
	if errors.Is(ErrProjectNotFound, ErrTaskNotFound) {
		t.Error("errors.Is should return false for different sentinel errors")
	}

	wrapped := fmt.Errorf("context: %w", ErrProjectNotFound)
	if errors.Is(wrapped, ErrTaskNotFound) {
		t.Error("errors.Is should return false for wrapped error compared to different sentinel")
	}
}

func TestNewTaskNotFoundError(t *testing.T) {
	testCases := []struct {
		name   string
		taskID any
	}{
		{"string ID", "abc-123"},
		{"int ID", 42},
		{"short ID", 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewTaskNotFoundError(tc.taskID)
			if !errors.Is(err, ErrTaskNotFound) {
				t.Error("NewTaskNotFoundError should wrap ErrTaskNotFound")
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%v", tc.taskID)) {
				t.Errorf("error message should contain task ID %v, got %q", tc.taskID, err.Error())
			}
		})
	}
}

func TestNewProjectNotFoundError(t *testing.T) {
	testCases := []struct {
		name      string
		projectID any
	}{
		{"string ID", "project-abc"},
		{"slug", "my-project"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewProjectNotFoundError(tc.projectID)
			if !errors.Is(err, ErrProjectNotFound) {
				t.Error("NewProjectNotFoundError should wrap ErrProjectNotFound")
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%v", tc.projectID)) {
				t.Errorf("error message should contain project ID %v, got %q", tc.projectID, err.Error())
			}
		})
	}
}

func TestNewProjectNotFoundInTrashError(t *testing.T) {
	err := NewProjectNotFoundInTrashError("archived-project")
	if !errors.Is(err, ErrProjectNotFound) {
		t.Error("NewProjectNotFoundInTrashError should wrap ErrProjectNotFound")
	}
	if !strings.Contains(err.Error(), "trash") {
		t.Errorf("error message should contain 'trash', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "archived-project") {
		t.Errorf("error message should contain project ID, got %q", err.Error())
	}
}

func TestNewRequiredFieldError(t *testing.T) {
	testCases := []struct {
		field string
	}{
		{"title"},
		{"name"},
		{"project_id"},
	}

	for _, tc := range testCases {
		t.Run(tc.field, func(t *testing.T) {
			err := NewRequiredFieldError(tc.field)
			if !errors.Is(err, ErrRequiredField) {
				t.Error("NewRequiredFieldError should wrap ErrRequiredField")
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error message should contain field name %q, got %q", tc.field, err.Error())
			}
		})
	}
}

func TestNewInvalidValueError(t *testing.T) {
	testCases := []struct {
		name   string
		field  string
		value  any
		reason string
	}{
		{"string value", "priority", "invalid", "must be 1-5"},
		{"int value", "priority", 99, "out of range"},
		{"empty value", "status", "", "cannot be empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewInvalidValueError(tc.field, tc.value, tc.reason)
			if !errors.Is(err, ErrInvalidValue) {
				t.Error("NewInvalidValueError should wrap ErrInvalidValue")
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error message should contain field %q, got %q", tc.field, err.Error())
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("error message should contain reason %q, got %q", tc.reason, err.Error())
			}
		})
	}
}

func TestErrorsIsWithAllSentinels(t *testing.T) {
	testCases := []struct {
		name     string
		sentinel error
	}{
		{"ErrProjectNotFound", ErrProjectNotFound},
		{"ErrNoCurrentProject", ErrNoCurrentProject},
		{"ErrProjectExists", ErrProjectExists},
		{"ErrTaskNotFound", ErrTaskNotFound},
		{"ErrInvalidTaskType", ErrInvalidTaskType},
		{"ErrInvalidParent", ErrInvalidParent},
		{"ErrCircularDep", ErrCircularDep},
		{"ErrConfigNotFound", ErrConfigNotFound},
		{"ErrConfigSaveFailed", ErrConfigSaveFailed},
		{"ErrBackupNotFound", ErrBackupNotFound},
		{"ErrParseError", ErrParseError},
		{"ErrSaveFailed", ErrSaveFailed},
		{"ErrReadFailed", ErrReadFailed},
		{"ErrWriteFailed", ErrWriteFailed},
		{"ErrRequiredField", ErrRequiredField},
		{"ErrInvalidValue", ErrInvalidValue},
		{"ErrNoChanges", ErrNoChanges},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("additional context: %w", tc.sentinel)
			if !errors.Is(wrapped, tc.sentinel) {
				t.Errorf("errors.Is should return true for wrapped %s", tc.name)
			}
		})
	}
}

func TestNewNoChangesError(t *testing.T) {
	testCases := []struct {
		name  string
		flags string
	}{
		{"edit flags", "--name, --description"},
		{"help flag", "--help to see available options"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewNoChangesError(tc.flags)
			if !errors.Is(err, ErrNoChanges) {
				t.Error("NewNoChangesError should wrap ErrNoChanges")
			}
			if !strings.Contains(err.Error(), tc.flags) {
				t.Errorf("error message should contain flags %q, got %q", tc.flags, err.Error())
			}
		})
	}
}

func TestNewSaveProjectError(t *testing.T) {
	innerErr := errors.New("disk full")
	err := NewSaveProjectError("work-tasks", innerErr)

	if !errors.Is(err, ErrSaveFailed) {
		t.Error("NewSaveProjectError should wrap ErrSaveFailed")
	}
	if !strings.Contains(err.Error(), "work-tasks") {
		t.Errorf("error message should contain project name, got %q", err.Error())
	}
	if !errors.Is(err, innerErr) {
		t.Error("NewSaveProjectError should wrap the inner error")
	}
}

func TestNewSaveConfigError(t *testing.T) {
	innerErr := errors.New("permission denied")
	err := NewSaveConfigError(innerErr)

	if !errors.Is(err, ErrConfigSaveFailed) {
		t.Error("NewSaveConfigError should wrap ErrConfigSaveFailed")
	}
	if !errors.Is(err, innerErr) {
		t.Error("NewSaveConfigError should wrap the inner error")
	}
}

func TestNewAmbiguousIdentifierError(t *testing.T) {
	err := NewAmbiguousIdentifierError("login", "#1 Login, #3 Login Flow")

	if !errors.Is(err, ErrAmbiguousIdentifier) {
		t.Error("NewAmbiguousIdentifierError should wrap ErrAmbiguousIdentifier")
	}
	if !strings.Contains(err.Error(), "login") {
		t.Errorf("error message should contain identifier, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "#1 Login, #3 Login Flow") {
		t.Errorf("error message should contain candidates, got %q", err.Error())
	}
}

func TestNewInvalidTaskTypeError(t *testing.T) {
	testCases := []struct {
		name     string
		taskType string
	}{
		{"unknown type", "widget"},
		{"typo", "epci"},
		{"empty", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewInvalidTaskTypeError(tc.taskType)
			if !errors.Is(err, ErrInvalidTaskType) {
				t.Error("NewInvalidTaskTypeError should wrap ErrInvalidTaskType")
			}
			if tc.taskType != "" && !strings.Contains(err.Error(), tc.taskType) {
				t.Errorf("error message should contain task type %q, got %q", tc.taskType, err.Error())
			}
		})
	}
}
