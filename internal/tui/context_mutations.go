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
// collects intent, these helpers validate it and commit it. Custom contexts are
// machine-local navigation state committed through a checked machine commit;
// regular context labels live on cataloged projects and commit as one checked
// catalog artifact through the application transaction boundary. Every mutation
// validates first and applies its change to a defensive copy, so a rejected or
// failed commit leaves the live runtime snapshot untouched.

var (
	// ErrDestinationContextRequired reports that a project cannot leave the
	// default context without an explicit destination: the default is the
	// fallback every other removal uses, so it has no fallback of its own.
	ErrDestinationContextRequired = errors.New("removing a project from the default context requires a destination context")

	// ErrNoDestinationContextAvailable reports that no other context exists to
	// receive a project leaving the default context, so the removal is blocked
	// rather than merely under-specified.
	ErrNoDestinationContextAvailable = errors.New("no other context exists to receive the project")

	// ErrDefaultContextProtected reports an attempt to delete the default
	// context, which every other context's removals fall back to.
	ErrDefaultContextProtected = errors.New("the default context cannot be deleted")

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

// CustomContextSaveRequest describes a custom-context create or update.
// OriginalName is empty when creating and names the existing context when
// editing; a different Name renames it. ActiveProjectIDs is the selection over
// the active projects an editor lists.
type CustomContextSaveRequest struct {
	OriginalName     string
	Name             string
	ActiveProjectIDs []string
}

// CustomContextSaveResult reports what a save produced, so a caller can render
// the outcome without re-reading the machine domain.
type CustomContextSaveResult struct {
	Name         string
	IsNew        bool
	Renamed      bool
	ProjectCount int
}

// SaveCustomContext creates, updates, or renames a custom context through a
// checked machine commit. Names collide case-insensitively.
func (c *ContextMutator) SaveCustomContext(req CustomContextSaveRequest) (CustomContextSaveResult, error) {
	if c == nil || c.runtime == nil {
		return CustomContextSaveResult{}, errContextRuntimeUnavailable
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return CustomContextSaveResult{}, fmt.Errorf("custom context name cannot be empty")
	}

	original := strings.TrimSpace(req.OriginalName)
	isNew := original == ""
	renamed := !isNew && !strings.EqualFold(name, original)
	machine := c.runtime.Machine()
	if !isNew && !machine.CustomContextExists(original) {
		return CustomContextSaveResult{}, fmt.Errorf("custom context %q not found", original)
	}
	if (isNew || renamed) && machine.CustomContextExists(name) {
		return CustomContextSaveResult{}, fmt.Errorf("custom context %q already exists", name)
	}

	members := c.customContextMembership(original, req.ActiveProjectIDs)
	if len(members) == 0 {
		return CustomContextSaveResult{}, fmt.Errorf("select at least one project")
	}

	if err := commitMachineToRuntime(c.runtime, func(m *config.MachineState) {
		if renamed {
			m.DeleteCustomContext(original)
		}
		m.SetCustomContext(name, members)
	}); err != nil {
		return CustomContextSaveResult{}, err
	}
	return CustomContextSaveResult{
		Name:         name,
		IsNew:        isNew,
		Renamed:      renamed,
		ProjectCount: len(members),
	}, nil
}

// DeleteCustomContext removes a saved custom context. It is the only path that
// discards the context's archived and no-longer-cataloged project references.
func (c *ContextMutator) DeleteCustomContext(name string) error {
	if c == nil || c.runtime == nil {
		return errContextRuntimeUnavailable
	}
	if !c.runtime.Machine().CustomContextExists(name) {
		return fmt.Errorf("custom context %q not found", name)
	}
	return commitMachineToRuntime(c.runtime, func(m *config.MachineState) {
		m.DeleteCustomContext(name)
	})
}

// customContextMembership merges a selection over active projects with the
// stored references an editor cannot list: archived, sandbox, internal, and
// no-longer-cataloged projects. config.IsActiveContextMember is the same rule
// that decides what the editor lists and what the scope resolves to, so every
// member absent from the selection because it was never offered survives, and
// only a project the user actually saw unchecked is dropped. Stored order is
// preserved for retained members; newly selected projects follow in selection
// order.
func (c *ContextMutator) customContextMembership(name string, selected []string) []string {
	pending := make(map[string]bool, len(selected))
	for _, id := range selected {
		pending[id] = true
	}
	members := make([]string, 0, len(selected))
	for _, id := range c.runtime.Machine().GetCustomContext(name) {
		if pending[id] {
			members = append(members, id)
			delete(pending, id)
			continue
		}
		if !config.IsActiveContextMember(c.runtime.GetProject(id)) {
			members = append(members, id)
		}
	}
	for _, id := range selected {
		if pending[id] {
			members = append(members, id)
			delete(pending, id)
		}
	}
	return members
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
// for a regular label: an optional rename plus the membership the label should
// have afterwards. ProjectIDs is the full membership, so every project carrying
// the label and absent from it is removed. Destination is consulted only when
// the saved label is the default context, because removals from every other
// label fall back to the default.
type RegularContextSaveRequest struct {
	OriginalName string
	Name         string
	ProjectIDs   []string
	Destination  string
}

// RegularContextSaveResult reports what the save applied: whether the label was
// renamed and across how many projects, how many projects gained or lost it, the
// label its removals landed in, and whether the workspace default context
// followed the rename.
type RegularContextSaveResult struct {
	Renamed             bool
	RelabeledCount      int
	Assigned            int
	Removed             int
	Destination         string
	DefaultContextMoved bool
}

// SaveRegularContext applies a rename and a membership edit as one checked
// commit. Both halves validate against the same snapshot and apply to the same
// clone, so a rename can never outlive a membership change that never landed: a
// rejected or failed commit leaves the durable catalog and the live runtime
// snapshot exactly as they were. Renaming the label the workspace default names
// carries the setting along in the same transaction, so the default can never
// point at a label no project carries.
func (c *ContextMutator) SaveRegularContext(req RegularContextSaveRequest) (RegularContextSaveResult, error) {
	if c == nil || c.runtime == nil {
		return RegularContextSaveResult{}, errContextRuntimeUnavailable
	}
	oldName := strings.TrimSpace(req.OriginalName)
	label := strings.TrimSpace(req.Name)
	if oldName == "" || label == "" {
		return RegularContextSaveResult{}, fmt.Errorf("context name cannot be empty")
	}

	catalog := cloneWorkspaceCatalog(c.runtime.Catalog())
	result := RegularContextSaveResult{Renamed: label != oldName}
	if result.Renamed {
		// Collisions are case-insensitive, so a rename that only changes casing
		// is allowed and repaints the label's display casing.
		if !strings.EqualFold(oldName, label) && catalogHasContext(catalog, label) {
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

	// Renaming the label the default names moves the setting with it. The
	// destination rules below then resolve against the new name, which is what the
	// default context is by the time this save lands.
	var settings *config.Settings
	fallback := c.runtime.DefaultContext()
	if result.Renamed && strings.EqualFold(oldName, fallback) {
		moved := *c.runtime.Settings()
		moved.DefaultContext = label
		settings = &moved
		fallback = label
		result.DefaultContextMoved = true
	}
	if strings.EqualFold(label, fallback) {
		// The renamed clone is the snapshot the removals are resolved against, so
		// a rename into the default label demands the same destination a direct
		// edit of that label would.
		destination, err := defaultContextDestination(catalog, label, selected, strings.TrimSpace(req.Destination))
		if err != nil {
			return RegularContextSaveResult{}, err
		}
		fallback = destination
	}
	result.Destination = fallback
	result.Assigned, result.Removed = applyRegularMembership(catalog, label, selected, fallback)

	if !result.Renamed && result.Assigned == 0 && result.Removed == 0 {
		return result, nil
	}
	if err := c.commitCatalogAndSettings(catalog, settings); err != nil {
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
	return destination, nil
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
	for _, p := range catalog.Projects {
		if strings.EqualFold(p.Context, name) {
			return true
		}
	}
	return false
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

// commitCatalogAndSettings persists a mutated catalog together with a mutated
// settings domain as one revision-checked transaction; nil settings degrades to
// the catalog-only commit. The pair must land together because a default context
// naming a label no project carries is not reachable from the hub to repair, so a
// writable workspace without the transaction boundary refuses the save rather
// than writing the two domains one after the other.
func (c *ContextMutator) commitCatalogAndSettings(catalog *config.WorkspaceCatalog, settings *config.Settings) error {
	if settings == nil {
		return c.commitCatalog(catalog)
	}
	if c.runtime.DataDir() == "" {
		c.runtime.SetCatalogInMemory(catalog)
		c.runtime.SetSettingsInMemory(settings)
		return nil
	}
	if c.manager == nil || c.service == nil {
		return fmt.Errorf("renaming the default context requires a transactional commit boundary")
	}
	artifacts, err := catalogAndSettingsDomainArtifacts(c.manager, catalog, settings)
	if err != nil {
		return fmt.Errorf("failed to encode workspace catalog and settings: %w", err)
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
