package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// Boundary catalog operations are the defensive half of every boundary
// mutation. Each one validates the whole request before touching anything, so a
// rejected operation leaves the receiver exactly as it was. That is what lets
// the shell clone a runtime snapshot, apply an operation, and abandon the clone
// when the checked commit fails without ever publishing speculative state.
//
// Membership is keyed on the durable boundary ID, never the display name, so a
// rename cannot move a project and a delete must name an explicit destination
// rather than leaving members dangling.

var (
	// ErrBoundaryNotFound reports an operation naming a boundary the catalog
	// does not define, including an unknown reassignment destination.
	ErrBoundaryNotFound = errors.New("config: boundary not found")

	// ErrBoundaryNameConflict reports a create or rename that would leave two
	// boundaries visually indistinguishable.
	ErrBoundaryNameConflict = errors.New("config: boundary name already exists")

	// ErrInvalidBoundaryName reports an empty or overlong boundary name.
	ErrInvalidBoundaryName = errors.New("config: invalid boundary name")

	// ErrBoundaryDestinationInvalid reports a deletion whose members would be
	// reassigned to the boundary being deleted.
	ErrBoundaryDestinationInvalid = errors.New("config: invalid boundary destination")

	// ErrCatalogProjectNotFound reports an assignment naming a project absent
	// from the catalog.
	ErrCatalogProjectNotFound = errors.New("config: project not found")
)

// CreateBoundary adds an empty custom boundary and returns its stored
// definition. The name is trimmed before validation and the durable ID is a
// fresh UUID, so a later rename never invalidates a project's membership.
func (w *WorkspaceCatalog) CreateBoundary(name string) (ProjectBoundary, error) {
	if w == nil {
		return ProjectBoundary{}, fmt.Errorf("workspace catalog is required")
	}
	if err := w.ValidateBoundaryName(name, ""); err != nil {
		return ProjectBoundary{}, err
	}
	created := ProjectBoundary{ID: uuid.New().String(), Name: strings.TrimSpace(name)}
	w.Boundaries = append(w.Boundaries, created)
	return created, nil
}

// RenameBoundary retitles one custom boundary in place. A rename that only
// changes casing is accepted, because the conflict check ignores the boundary
// being renamed.
func (w *WorkspaceCatalog) RenameBoundary(id, name string) error {
	if w == nil {
		return fmt.Errorf("workspace catalog is required")
	}
	if id == models.ActiveBoundaryID {
		return fmt.Errorf("%w: boundary %q cannot be renamed", ErrProtectedActiveBoundary, id)
	}
	index := w.boundaryIndex(id)
	if index < 0 {
		return fmt.Errorf("%w: %q", ErrBoundaryNotFound, id)
	}
	if err := w.ValidateBoundaryName(name, id); err != nil {
		return err
	}
	w.Boundaries[index].Name = strings.TrimSpace(name)
	return nil
}

// ValidateBoundaryName reports why name cannot title a boundary, so a surface
// collecting one can refuse it inline instead of discovering the rejection only
// after asking the shell to commit. exceptID lets a rename keep its own name.
func (w *WorkspaceCatalog) ValidateBoundaryName(name, exceptID string) error {
	if w == nil {
		return fmt.Errorf("workspace catalog is required")
	}
	trimmed, err := validateBoundaryName(name)
	if err != nil {
		return err
	}
	if w.boundaryNameTaken(trimmed, exceptID) {
		return fmt.Errorf("%w: %q", ErrBoundaryNameConflict, trimmed)
	}
	return nil
}

// DeleteBoundary removes one custom boundary and moves every member — archived
// projects included — into destinationID, returning the IDs it reassigned so
// the caller can name them in the transaction it commits.
func (w *WorkspaceCatalog) DeleteBoundary(id, destinationID string) ([]string, error) {
	if w == nil {
		return nil, fmt.Errorf("workspace catalog is required")
	}
	if id == models.ActiveBoundaryID {
		return nil, fmt.Errorf("%w: boundary %q cannot be deleted", ErrProtectedActiveBoundary, id)
	}
	index := w.boundaryIndex(id)
	if index < 0 {
		return nil, fmt.Errorf("%w: %q", ErrBoundaryNotFound, id)
	}
	if destinationID == id {
		return nil, fmt.Errorf("%w: %q cannot receive its own members", ErrBoundaryDestinationInvalid, id)
	}
	if _, known := w.Boundary(destinationID); !known {
		return nil, fmt.Errorf("%w: %q", ErrBoundaryNotFound, destinationID)
	}

	var moved []string
	now := time.Now()
	for i := range w.Projects {
		if w.Projects[i].BoundaryID != id {
			continue
		}
		w.Projects[i].BoundaryID = destinationID
		w.Projects[i].UpdatedAt = now
		moved = append(moved, w.Projects[i].ID)
	}
	w.Boundaries = slices.Delete(w.Boundaries, index, index+1)
	return moved, nil
}

// AssignProjectBoundary moves one cataloged project into a boundary and reports
// whether that changed anything. The catalog owns membership only: clearing the
// planning memberships a project loses by leaving Active is the caller's job,
// because only the caller can commit both domains in one transaction.
func (w *WorkspaceCatalog) AssignProjectBoundary(projectID, boundaryID string) (bool, error) {
	if w == nil {
		return false, fmt.Errorf("workspace catalog is required")
	}
	if _, known := w.Boundary(boundaryID); !known {
		return false, fmt.Errorf("%w: %q", ErrBoundaryNotFound, boundaryID)
	}
	index := w.projectIndex(projectID)
	if index < 0 {
		return false, fmt.Errorf("%w: %q", ErrCatalogProjectNotFound, projectID)
	}
	if w.IsSystemUnfiledProject(projectID) && boundaryID != models.ActiveBoundaryID {
		return false, fmt.Errorf(
			"%w: project %q cannot leave the %s boundary",
			ErrProtectedSystemUnfiled,
			projectID,
			ActiveBoundaryName,
		)
	}
	if w.Projects[index].BoundaryID == boundaryID {
		return false, nil
	}
	w.Projects[index].BoundaryID = boundaryID
	w.Projects[index].UpdatedAt = time.Now()
	return true, nil
}

// CatalogProjectByID returns a copy of one cataloged project. Callers preparing
// a boundary mutation need the project's slug and archived flag, both of which
// decide whether planning cleanup belongs in the transaction.
func (w *WorkspaceCatalog) CatalogProjectByID(projectID string) (CatalogProject, bool) {
	index := w.projectIndex(projectID)
	if index < 0 {
		return CatalogProject{}, false
	}
	return w.Projects[index], true
}

func (w *WorkspaceCatalog) boundaryIndex(id string) int {
	if w == nil || id == "" {
		return -1
	}
	return slices.IndexFunc(w.Boundaries, func(b ProjectBoundary) bool { return b.ID == id })
}

func (w *WorkspaceCatalog) projectIndex(id string) int {
	if w == nil || id == "" {
		return -1
	}
	return slices.IndexFunc(w.Projects, func(p CatalogProject) bool { return p.ID == id })
}

// boundaryNameTaken reports whether name collides with an existing boundary,
// comparing case-insensitively so two boundaries can never be visually
// indistinguishable. exceptID lets a rename keep its own name.
func (w *WorkspaceCatalog) boundaryNameTaken(name, exceptID string) bool {
	for _, boundary := range w.Boundaries {
		if boundary.ID == exceptID {
			continue
		}
		if boundaryNamesEqual(boundary.Name, name) {
			return true
		}
	}
	return false
}

// boundaryNamesEqual is the one equality predicate every boundary-name
// uniqueness check uses. Load validation and create/rename must fold names
// identically, or a hand-edited or synced catalog can persist a pair the
// supported APIs would have refused — Unicode simple folding equates runes such
// as the Greek final sigma with sigma, which lowercasing does not.
func boundaryNamesEqual(a, b string) bool {
	return strings.EqualFold(a, b)
}

// validateCustomBoundaryID rejects any persisted non-Active identity that
// CreateBoundary could not have minted. Only the canonical UUID text a fresh
// uuid.New().String() produces is accepted, which is also what keeps an
// internal presentation sentinel such as "project-hub:filter:archived" from
// ever loading as a stored boundary ID.
func validateCustomBoundaryID(id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return fmt.Errorf("boundary %q is not a canonical UUID", id)
	}
	return nil
}

// validateBoundaryName trims and range-checks a user-supplied name. Stored names
// are always trimmed, which is why validateBoundaries rejects an untrimmed one
// while this accepts and normalizes it.
func validateBoundaryName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("%w: name cannot be empty", ErrInvalidBoundaryName)
	}
	if n := utf8.RuneCountInString(trimmed); n > MaxBoundaryNameLength {
		return "", fmt.Errorf(
			"%w: name is %d characters, max %d",
			ErrInvalidBoundaryName, n, MaxBoundaryNameLength,
		)
	}
	return trimmed, nil
}
