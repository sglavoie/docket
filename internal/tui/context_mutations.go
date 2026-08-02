package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
)

// Context mutations are the presentation-neutral half of the Context Hub: a view
// collects intent, these helpers validate it and commit it. Context labels live
// on cataloged projects and commit as one checked catalog artifact through the
// application transaction boundary. Every mutation validates first and applies
// its change to a defensive copy, so a rejected or failed commit leaves the live
// runtime snapshot untouched.

var (
	// ErrDestinationContextRequired reports that a project cannot leave the
	// default context without an explicit destination: the default is the
	// fallback every other removal uses, so it has no fallback of its own.
	ErrDestinationContextRequired = errors.New("removing a project from the default context requires a destination context")

	// ErrNoDestinationContextAvailable reports that no other context exists to
	// receive a project leaving the default context, so the removal is blocked
	// rather than merely under-specified.
	ErrNoDestinationContextAvailable = errors.New("no other context exists to receive the project")

	// ErrDefaultContextProtected reports an attempt to rename or delete the
	// personal context, which every other context's removals fall back to.
	ErrDefaultContextProtected = errors.New("the personal context cannot be renamed or deleted")

	errContextRuntimeUnavailable = errors.New("runtime state is unavailable")
)

// ContextMutator applies context mutations for one workspace runtime. The
// manager and service are the checked-catalog commit boundary; a runtime with no
// writable data directory (or without them) applies catalog changes in memory,
// matching the rest of the shell's domain-commit seams.
type ContextMutator struct {
	runtime *config.RuntimeState
	manager *config.Manager
	service *app.Service
}

// NewContextMutator wires a mutator to the runtime snapshot it reads from and
// the commit boundary it writes through.
func NewContextMutator(runtime *config.RuntimeState, manager *config.Manager, service *app.Service) *ContextMutator {
	return &ContextMutator{runtime: runtime, manager: manager, service: service}
}

// DeleteRegularContext reassigns every project carrying the label to the default
// context and returns how many were reassigned. The default context itself is
// never deletable: it is the destination every other removal falls back to.
func (c *ContextMutator) DeleteRegularContext(name string) (int, error) {
	if c == nil || c.runtime == nil {
		return 0, errContextRuntimeUnavailable
	}
	if strings.TrimSpace(name) == "" {
		return 0, fmt.Errorf("context name cannot be empty")
	}
	if strings.EqualFold(name, c.runtime.DefaultContext()) {
		return 0, fmt.Errorf("%w: %q", ErrDefaultContextProtected, name)
	}

	catalog := cloneWorkspaceCatalog(c.runtime.Catalog())
	count := relabelCatalogContext(catalog, name, c.runtime.DefaultContext())
	if count == 0 {
		return 0, fmt.Errorf("context %q is not assigned to any project", name)
	}
	if err := c.commitCatalog(catalog); err != nil {
		return 0, err
	}
	return count, nil
}

// RegularContextSaveRequest describes the single save the Context Hub presents
// for a regular label: creation or an optional rename plus the membership the
// label should have afterwards. ProjectIDs is the full membership, so every
// project carrying the label and absent from it is removed. Destination is
// consulted only when the saved label is personal, because removals from every
// other label fall back to personal.
type RegularContextSaveRequest struct {
	OriginalName string
	Name         string
	ProjectIDs   []string
	Destination  string
}

// RegularContextSaveResult reports what the save applied: whether the label was
// created or renamed and across how many projects, how many projects gained or
// lost it, and the label its removals landed in.
type RegularContextSaveResult struct {
	IsNew          bool
	Renamed        bool
	RelabeledCount int
	Assigned       int
	Removed        int
	Destination    string
}

// SaveRegularContext applies creation or a rename and membership edit as one
// checked commit. Every part validates against the same snapshot and applies to
// the same clone, so a rejected or failed commit leaves the durable catalog and
// live runtime snapshot exactly as they were. The personal label cannot be
// created, renamed, or renamed away; only its membership is editable.
func (c *ContextMutator) SaveRegularContext(req RegularContextSaveRequest) (RegularContextSaveResult, error) {
	if c == nil || c.runtime == nil {
		return RegularContextSaveResult{}, errContextRuntimeUnavailable
	}
	oldName := strings.TrimSpace(req.OriginalName)
	label := strings.TrimSpace(req.Name)
	if label == "" {
		return RegularContextSaveResult{}, fmt.Errorf("context name cannot be empty")
	}

	catalog := cloneWorkspaceCatalog(c.runtime.Catalog())
	isNew := oldName == ""
	result := RegularContextSaveResult{IsNew: isNew, Renamed: !isNew && label != oldName}
	if isNew {
		if strings.EqualFold(label, c.runtime.DefaultContext()) || catalogHasContext(catalog, label) {
			return RegularContextSaveResult{}, fmt.Errorf("context %q already exists", label)
		}
		if activeSelectedProjectCount(catalog, req.ProjectIDs) == 0 {
			return RegularContextSaveResult{}, fmt.Errorf("select at least one active project")
		}
	}
	if !isNew && strings.EqualFold(oldName, c.runtime.DefaultContext()) && result.Renamed {
		return RegularContextSaveResult{}, fmt.Errorf("%w: %q", ErrDefaultContextProtected, oldName)
	}
	if !isNew && !strings.EqualFold(oldName, c.runtime.DefaultContext()) && !catalogHasContext(catalog, oldName) {
		return RegularContextSaveResult{}, fmt.Errorf("context %q is not assigned to any project", oldName)
	}
	if result.Renamed {
		// Collisions are case-insensitive, so a rename that only changes casing
		// is allowed and repaints the label's display casing.
		if strings.EqualFold(label, c.runtime.DefaultContext()) ||
			(!strings.EqualFold(oldName, label) && catalogHasContext(catalog, label)) {
			return RegularContextSaveResult{}, fmt.Errorf("context %q already exists", label)
		}
		result.RelabeledCount = relabelCatalogContext(catalog, oldName, label)
		if result.RelabeledCount == 0 {
			return RegularContextSaveResult{}, fmt.Errorf("context %q is not assigned to any project", oldName)
		}
	}

	selected := make(map[string]bool, len(req.ProjectIDs))
	for _, id := range req.ProjectIDs {
		selected[id] = true
	}

	fallback := c.runtime.DefaultContext()
	if strings.EqualFold(label, fallback) {
		// Personal has no implicit fallback, so removing one of its projects
		// requires the explicit existing destination collected by the editor.
		destination, err := defaultContextDestination(catalog, label, selected, strings.TrimSpace(req.Destination))
		if err != nil {
			return RegularContextSaveResult{}, err
		}
		fallback = destination
	}
	result.Destination = fallback
	result.Assigned, result.Removed = applyRegularMembership(catalog, label, selected, fallback)

	if !result.IsNew && !result.Renamed && result.Assigned == 0 && result.Removed == 0 {
		return result, nil
	}
	if err := c.commitCatalog(catalog); err != nil {
		return RegularContextSaveResult{}, err
	}
	return result, nil
}

// applyRegularMembership assigns the label to every selected project and moves
// every other project carrying it to fallback, reporting both counts.
func applyRegularMembership(catalog *config.WorkspaceCatalog, label string, selected map[string]bool, fallback string) (assigned, removed int) {
	now := time.Now()
	for i := range catalog.Projects {
		entry := &catalog.Projects[i]
		switch {
		case selected[entry.ID] && entry.Context != label:
			entry.Context = label
			entry.UpdatedAt = now
			assigned++
		case !selected[entry.ID] && entry.Context == label:
			entry.Context = fallback
			entry.UpdatedAt = now
			removed++
		}
	}
	return assigned, removed
}

// defaultContextDestination resolves where projects removed from the default
// context go. It returns the default itself when the edit removes nothing, so a
// pure assignment into the default context never demands a destination.
func defaultContextDestination(catalog *config.WorkspaceCatalog, label string, selected map[string]bool, destination string) (string, error) {
	removes := false
	for _, p := range catalog.Projects {
		if p.Context == label && !selected[p.ID] {
			removes = true
			break
		}
	}
	if !removes {
		return label, nil
	}
	if destination == "" {
		if !catalogHasAlternateContext(catalog, label) {
			return "", fmt.Errorf("%w: %q", ErrNoDestinationContextAvailable, label)
		}
		return "", fmt.Errorf("%w: %q", ErrDestinationContextRequired, label)
	}
	if strings.EqualFold(destination, label) {
		return "", fmt.Errorf("destination context must differ from %q", label)
	}
	canonical, ok := catalogContextName(catalog, destination)
	if !ok {
		return "", fmt.Errorf("destination context %q does not exist", destination)
	}
	return canonical, nil
}

func activeSelectedProjectCount(catalog *config.WorkspaceCatalog, projectIDs []string) int {
	selected := make(map[string]bool, len(projectIDs))
	for _, id := range projectIDs {
		selected[id] = true
	}
	count := 0
	for _, project := range catalog.Projects {
		if selected[project.ID] && !project.Archived {
			count++
		}
	}
	return count
}

// catalogHasAlternateContext reports whether any label other than the one being
// edited exists to receive a project removed from the default context.
func catalogHasAlternateContext(catalog *config.WorkspaceCatalog, label string) bool {
	for _, p := range catalog.Projects {
		if p.Context != "" && !strings.EqualFold(p.Context, label) {
			return true
		}
	}
	return false
}

// catalogHasContext reports whether any cataloged project already carries a
// label matching name case-insensitively.
func catalogHasContext(catalog *config.WorkspaceCatalog, name string) bool {
	_, ok := catalogContextName(catalog, name)
	return ok
}

func catalogContextName(catalog *config.WorkspaceCatalog, name string) (string, bool) {
	for _, p := range catalog.Projects {
		if strings.EqualFold(p.Context, name) {
			return p.Context, true
		}
	}
	return "", false
}

// commitCatalog persists a mutated catalog copy as one revision-checked artifact
// through the application transaction boundary, then advances the runtime
// snapshot. A runtime without a writable workspace (isolated tests) applies the
// copy in memory instead, so no stray workspace.json is written; without the
// transaction boundary the runtime's own domain-scoped commit enforces the same
// revision check.
func (c *ContextMutator) commitCatalog(catalog *config.WorkspaceCatalog) error {
	if c.runtime.DataDir() == "" {
		c.runtime.SetCatalogInMemory(catalog)
		return nil
	}
	if c.manager == nil || c.service == nil {
		return c.runtime.CommitCatalog(catalog)
	}
	artifacts, err := catalogDomainArtifact(c.manager, catalog)
	if err != nil {
		return fmt.Errorf("failed to encode workspace catalog: %w", err)
	}
	if err := c.service.CommitConfig(artifacts...); err != nil {
		return err
	}
	return c.runtime.RefreshAfterCommit()
}

// relabelCatalogContext rewrites every project carrying oldName to newName,
// returning the number changed.
func relabelCatalogContext(catalog *config.WorkspaceCatalog, oldName, newName string) int {
	count := 0
	now := time.Now()
	for i := range catalog.Projects {
		if catalog.Projects[i].Context != oldName {
			continue
		}
		catalog.Projects[i].Context = newName
		catalog.Projects[i].UpdatedAt = now
		count++
	}
	return count
}
