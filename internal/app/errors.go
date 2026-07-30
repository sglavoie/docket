package app

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors let adapters classify a use-case failure with errors.Is
// rather than matching message text. Stale-revision conflicts are surfaced
// verbatim from the storage commit boundary (storage.ErrStaleRevision), so an
// adapter that wants to reload-and-retry inspects the commit error directly.
var (
	// ErrInvalidInput marks a request that fails validation before any mutation
	// is attempted (empty title, out-of-range priority, malformed tag, ...).
	ErrInvalidInput = errors.New("app: invalid input")
	// ErrProjectNotFound marks a request targeting a project absent from the
	// loaded snapshot.
	ErrProjectNotFound = errors.New("app: project not found")
	// ErrProtectedSystemInbox marks a project lifecycle request that targets
	// the catalog-owned system Inbox. Adapters supply its durable catalog ID;
	// the application layer never infers protection from a display name or slug.
	ErrProtectedSystemInbox = errors.New("app: protected system Inbox")
	// ErrTaskNotFound marks a request targeting a task absent from the loaded
	// snapshot.
	ErrTaskNotFound = errors.New("app: task not found")
	// ErrAmbiguousTaskID marks an unqualified task UUID that exists in more than
	// one project. Callers must retry with a TaskLocator so the application layer
	// never guesses which owning project to mutate.
	ErrAmbiguousTaskID = errors.New("app: ambiguous task id")
	// ErrInvalidTransition marks a state transition the workflow model forbids
	// (e.g. leaving a terminal state without reopening first).
	ErrInvalidTransition = errors.New("app: invalid state transition")
	// ErrDependencyBlocked marks a completion refused because the task still has
	// unresolved dependencies. Dependency blocking is derived, not a workflow
	// state, so it gates completion rather than replacing the target state.
	ErrDependencyBlocked = errors.New("app: task has unresolved dependencies")
	// ErrDestructiveConfirmationRequired marks a destructive use case (delete, bulk
	// delete) invoked without explicit intent. Destructive requests must set their
	// Confirm flag; the check runs before any mutation, so an unconfirmed request
	// never removes a task or touches storage.
	ErrDestructiveConfirmationRequired = errors.New("app: destructive action requires explicit confirmation")
	// ErrParentHasIncompleteChildren marks a terminal transition (complete or
	// cancel) refused because the task still has incomplete direct children. A
	// parent must never become done or canceled while a child is still open; this
	// gate mirrors the legacy CLI completion invariant at the application layer, so
	// the illegal completed-parent-with-incomplete-child state is rejected before
	// any mutation rather than silently reconciled after the fact.
	ErrParentHasIncompleteChildren = errors.New("app: task has incomplete children")
)

// IncompleteChild describes one direct child that blocks a parent's terminal
// transition. It is presentation-neutral so a CLI or TUI adapter can render the
// same detail the legacy completion check produced.
type IncompleteChild struct {
	// ShortID is the child's per-project short numeric ID.
	ShortID int
	// Title is the child's title.
	Title string
	// Status is the child's workflow-state label (e.g. "ready", "in_progress").
	Status string
}

// ChildrenBlockedError describes a terminal transition refused because the task
// still has incomplete direct children, carrying the parent's short ID and each
// blocking child. It unwraps to ErrParentHasIncompleteChildren so adapters can
// classify the failure while still reporting the specific children.
type ChildrenBlockedError struct {
	ParentShortID int
	Children      []IncompleteChild
}

func (e *ChildrenBlockedError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "task #%d has %d incomplete children:", e.ParentShortID, len(e.Children))
	for _, c := range e.Children {
		fmt.Fprintf(&b, "\n  - #%d %s (%s)", c.ShortID, c.Title, c.Status)
	}
	return b.String()
}

func (e *ChildrenBlockedError) Unwrap() error { return ErrParentHasIncompleteChildren }

// InvalidInputError describes which field failed validation and why. It unwraps
// to ErrInvalidInput so callers can match the class while still reporting the
// specific field to the user.
type InvalidInputError struct {
	Field  string
	Reason string
}

func (e *InvalidInputError) Error() string {
	return fmt.Sprintf("app: invalid %s: %s", e.Field, e.Reason)
}

func (e *InvalidInputError) Unwrap() error { return ErrInvalidInput }

func invalidInput(field, reason string) error {
	return &InvalidInputError{Field: field, Reason: reason}
}

// TransitionError describes a forbidden workflow-state transition, carrying the
// current and target states and the model's human-readable reason. It unwraps
// to ErrInvalidTransition.
type TransitionError struct {
	From   string
	To     string
	Reason string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("app: cannot transition from %q to %q: %s", e.From, e.To, e.Reason)
}

func (e *TransitionError) Unwrap() error { return ErrInvalidTransition }
