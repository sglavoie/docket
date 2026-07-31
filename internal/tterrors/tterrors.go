// Package tterrors defines shared sentinel errors for command, storage, and
// external-interaction boundaries.
package tterrors

import (
	"errors"
	"fmt"
)

var (
	ErrProjectNotFound  = errors.New("project not found")
	ErrNoCurrentProject = errors.New("no current project selected")
	ErrParseError       = errors.New("failed to parse markdown")
	ErrSaveFailed       = errors.New("failed to save project")
	ErrCanceled         = errors.New("canceled")
)

func NewProjectNotFoundError(projectID any) error {
	return fmt.Errorf("%w: '%v'", ErrProjectNotFound, projectID)
}

func NewSaveProjectError(name string, err error) error {
	if err == nil {
		return fmt.Errorf("%w '%s'", ErrSaveFailed, name)
	}
	return fmt.Errorf("%w '%s': %w", ErrSaveFailed, name, err)
}
