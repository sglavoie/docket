package bridge

import (
	"errors"
	"fmt"
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
		return explainRefusal(newError(CodeValidation, "%s", err), err)
	case errors.Is(err, app.ErrProtectedSystemUnfiled),
		errors.Is(err, config.ErrProtectedSystemUnfiled),
		errors.Is(err, storage.ErrCommitBlocked):
		return newError(CodeRefused, "%s", err)
	case errors.Is(err, engine.ErrInvalidResolution):
		return newError(CodeValidation, "%s", err).with("field", "resolution")
	case errors.Is(err, storage.ErrStaleRevision),
		errors.Is(err, storage.ErrStaleArtifact),
		errors.Is(err, engine.ErrConflictStale),
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

// explainRefusal carries the shared engine's own refusal into details, so a
// client words a lifecycle or field refusal from the values the engine judged
// rather than by parsing its message: the refused field and reason, the two
// states of a forbidden transition, or the children blocking a completion. A
// batch refusal also names the capture line it came from.
func explainRefusal(refusal *Error, err error) *Error {
	var line int
	if _, scanErr := fmt.Sscanf(err.Error(), "line %d:", &line); scanErr == nil && line > 0 {
		refusal = refusal.with("line", line)
	}
	var invalid *app.InvalidInputError
	var transition *app.TransitionError
	var blocked *app.ChildrenBlockedError
	switch {
	case errors.As(err, &blocked):
		children := make([]map[string]any, 0, len(blocked.Children))
		for _, child := range blocked.Children {
			children = append(children, map[string]any{"short_id": child.ShortID, "title": child.Title, "state": child.Status})
		}
		return refusal.with("kind", "incomplete_children").with("short_id", blocked.ParentShortID).with("children", children)
	case errors.As(err, &transition):
		return refusal.with("kind", "invalid_transition").with("from", transition.From).with("to", transition.To).
			with("reason", transition.Reason)
	case errors.As(err, &invalid):
		return refusal.with("kind", "invalid_input").with("field", invalid.Field).with("reason", invalid.Reason)
	}
	return refusal
}
