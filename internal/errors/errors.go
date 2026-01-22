// Package errors defines sentinel errors for the tt application.
// These errors enable callers to use errors.Is() for precise error handling.
package errors

import "errors"

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
)
