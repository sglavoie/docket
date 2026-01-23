package tterrors

import (
	"errors"
	"fmt"
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
