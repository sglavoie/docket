package bridge

import (
	"errors"
	"io/fs"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// classify maps an engine failure onto the contract's error codes by its
// sentinel, never its message. A nil error stays nil; an error nothing names is
// internal, which is a bug report rather than a user-facing refusal.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var bridgeErr *Error
	if errors.As(err, &bridgeErr) {
		return bridgeErr
	}
	var tooNew *config.SchemaTooNewError
	if errors.As(err, &tooNew) {
		return newError(CodeSchemaTooNew, "%s", err).with("found", tooNew.Found).with("supported", tooNew.Supported)
	}
	switch {
	case errors.Is(err, engine.ErrInvalidConfig):
		return newError(CodeInvalidRequest, "%s", err)
	case errors.Is(err, engine.ErrWorkspaceNotFound),
		errors.Is(err, app.ErrProjectNotFound),
		errors.Is(err, app.ErrTaskNotFound):
		return newError(CodeNotFound, "%s", err)
	case errors.Is(err, engine.ErrMigrationRequired):
		return newError(CodeMigrationRequired, "%s", err)
	case errors.Is(err, app.ErrInvalidInput),
		errors.Is(err, app.ErrInvalidTransition),
		errors.Is(err, app.ErrParentHasIncompleteChildren):
		return newError(CodeValidation, "%s", err)
	case errors.Is(err, app.ErrProtectedSystemUnfiled),
		errors.Is(err, config.ErrProtectedSystemUnfiled),
		errors.Is(err, storage.ErrCommitBlocked):
		return newError(CodeRefused, "%s", err)
	case errors.Is(err, storage.ErrStaleRevision),
		errors.Is(err, storage.ErrStaleArtifact),
		errors.Is(err, config.ErrStaleObject):
		return newError(CodeStaleRevision, "%s", err)
	case errors.Is(err, storage.ErrMergeConflict):
		return newError(CodeConflict, "%s", err)
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return newError(CodeIO, "%s", err).with("path", pathErr.Path)
	}
	logging.Warn("bridge: unclassified engine error: %v", err)
	return newError(CodeInternal, "%s", err)
}
