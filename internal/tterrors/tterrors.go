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
	ErrSelfDependency  = errors.New("a task cannot depend on itself")
	ErrDuplicateDep    = errors.New("duplicate dependency")
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
	ErrRequiredField       = errors.New("required field missing")
	ErrInvalidValue        = errors.New("invalid value")
	ErrAmbiguousIdentifier = errors.New("ambiguous identifier")
	ErrNoChanges           = errors.New("no changes specified")
)

// User interaction errors
var (
	ErrCanceled   = errors.New("canceled")
	ErrEmptyTitle = errors.New("blocker title cannot be empty")
)

// NewTaskNotFoundError creates a task not found error with the specified task identifier.
// Use this for consistent error messaging across the codebase.
func NewTaskNotFoundError(taskID any) error {
	return fmt.Errorf("%w: '%v' (tried short ID, UUID, and title prefix)", ErrTaskNotFound, taskID)
}

// NewParentTaskNotFoundError creates a parent task not found error with the specified task identifier.
func NewParentTaskNotFoundError(taskID any) error {
	return fmt.Errorf("parent %w: '%v'", ErrTaskNotFound, taskID)
}

// NewBlockerTaskNotFoundError creates a blocker task not found error with the specified task identifier.
func NewBlockerTaskNotFoundError(taskID any) error {
	return fmt.Errorf("blocker %w: '%v'", ErrTaskNotFound, taskID)
}

// NewProjectNotFoundError creates a project not found error with the specified project identifier.
// Use this for consistent error messaging across the codebase.
func NewProjectNotFoundError(projectID any) error {
	return fmt.Errorf("%w: '%v'", ErrProjectNotFound, projectID)
}

// NewProjectNotFoundInTrashError creates a project not found in trash error with the specified project identifier.
func NewProjectNotFoundInTrashError(projectID any) error {
	return fmt.Errorf("%w in trash: '%v'", ErrProjectNotFound, projectID)
}

// NewRequiredFieldError creates an error for a missing required field.
// Use this for consistent validation error messaging across the codebase.
func NewRequiredFieldError(field string) error {
	return fmt.Errorf("%w: %s", ErrRequiredField, field)
}

// NewInvalidValueError creates an error for an invalid field value with a reason.
// Use this for consistent validation error messaging across the codebase.
func NewInvalidValueError(field string, value any, reason string) error {
	return fmt.Errorf("%w for %s: '%v' (%s)", ErrInvalidValue, field, value, reason)
}

// NewLoadProjectError creates an error for failing to load a project.
// Use this when project loading fails to provide consistent error messaging.
func NewLoadProjectError(name string, err error) error {
	return fmt.Errorf("%w '%s': %w", ErrReadFailed, name, err)
}

// NewSaveProjectError creates an error for failing to save a project.
// Use this when project saving fails to provide consistent error messaging.
func NewSaveProjectError(name string, err error) error {
	return fmt.Errorf("%w '%s': %w", ErrSaveFailed, name, err)
}

// NewInvalidTaskTypeError creates an error for an invalid task type.
// Use this for consistent task type validation error messaging.
func NewInvalidTaskTypeError(taskType string) error {
	return fmt.Errorf("%w: '%s'", ErrInvalidTaskType, taskType)
}

// NewAmbiguousIdentifierError creates an error when an identifier matches multiple items.
// candidates should be formatted strings like "#1 Login, #3 Logout".
func NewAmbiguousIdentifierError(identifier string, candidates string) error {
	return fmt.Errorf("%w %q: matches %s (use task short ID)", ErrAmbiguousIdentifier, identifier, candidates)
}

// NewNoChangesError creates an error when no modification flags are provided to an edit command.
func NewNoChangesError(flags string) error {
	return fmt.Errorf("%w; use %s", ErrNoChanges, flags)
}

// NewSaveConfigError creates an error for failing to save the config file.
func NewSaveConfigError(err error) error {
	return fmt.Errorf("%w: %w", ErrConfigSaveFailed, err)
}
