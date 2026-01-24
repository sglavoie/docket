// Package tterrors defines sentinel errors for the tt application.
// These errors enable callers to use errors.Is() for precise error handling.
package tterrors

import (
	"errors"
	"fmt"
)

// Project-related errors
var (
	ErrProjectNotFound  = errors.New("project not found")
	ErrNoCurrentProject = errors.New("no current project selected")
	ErrProjectExists    = errors.New("project already exists")
)

// Task-related errors
var (
	ErrTaskNotFound    = errors.New("task not found")
	ErrInvalidTaskType = errors.New("invalid task type")
	ErrInvalidParent   = errors.New("invalid parent task")
	ErrCircularDep     = errors.New("circular dependency detected")
)

// Config-related errors
var (
	ErrConfigNotFound   = errors.New("config file not found")
	ErrConfigSaveFailed = errors.New("failed to save config")
)

// Storage-related errors
var (
	ErrBackupNotFound = errors.New("backup not found")
	ErrParseError     = errors.New("failed to parse markdown")
	ErrSaveFailed     = errors.New("failed to save project")
	ErrReadFailed     = errors.New("failed to read file")
	ErrWriteFailed    = errors.New("failed to write file")
)

// Validation errors
var (
	ErrRequiredField = errors.New("required field missing")
	ErrInvalidValue  = errors.New("invalid value")
)

// NewTaskNotFoundError creates a task not found error with the specified task identifier.
// Use this for consistent error messaging across the codebase.
func NewTaskNotFoundError(taskID interface{}) error {
	return fmt.Errorf("%w: '%v'", ErrTaskNotFound, taskID)
}

// NewParentTaskNotFoundError creates a parent task not found error with the specified task identifier.
func NewParentTaskNotFoundError(taskID interface{}) error {
	return fmt.Errorf("parent %w: '%v'", ErrTaskNotFound, taskID)
}

// NewBlockerTaskNotFoundError creates a blocker task not found error with the specified task identifier.
func NewBlockerTaskNotFoundError(taskID interface{}) error {
	return fmt.Errorf("blocker %w: '%v'", ErrTaskNotFound, taskID)
}

// NewProjectNotFoundError creates a project not found error with the specified project identifier.
// Use this for consistent error messaging across the codebase.
func NewProjectNotFoundError(projectID interface{}) error {
	return fmt.Errorf("%w: '%v'", ErrProjectNotFound, projectID)
}

// NewProjectNotFoundInTrashError creates a project not found in trash error with the specified project identifier.
func NewProjectNotFoundInTrashError(projectID interface{}) error {
	return fmt.Errorf("%w in trash: '%v'", ErrProjectNotFound, projectID)
}

// NewRequiredFieldError creates an error for a missing required field.
// Use this for consistent validation error messaging across the codebase.
func NewRequiredFieldError(field string) error {
	return fmt.Errorf("%w: %s", ErrRequiredField, field)
}

// NewInvalidValueError creates an error for an invalid field value with a reason.
// Use this for consistent validation error messaging across the codebase.
func NewInvalidValueError(field string, value interface{}, reason string) error {
	return fmt.Errorf("%w for %s: '%v' (%s)", ErrInvalidValue, field, value, reason)
}
