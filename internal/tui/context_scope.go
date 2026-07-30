package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ContextScopeKind classifies the project scope a context selection resolves to.
type ContextScopeKind int

const (
	// ContextScopeAll spans every active project.
	ContextScopeAll ContextScopeKind = iota
	// ContextScopeRegular spans the active projects carrying one context label.
	ContextScopeRegular
	// ContextScopeCustom spans a saved, machine-local group of project IDs.
	ContextScopeCustom
)

// ContextScope names an active project scope. The zero value is the All scope.
type ContextScope struct {
	Kind ContextScopeKind
	Name string
}

// AllContextScope returns the scope spanning every active project.
func AllContextScope() ContextScope {
	return ContextScope{Kind: ContextScopeAll}
}

// RegularContextScope returns the scope for a project context label.
func RegularContextScope(name string) ContextScope {
	if name == "" {
		return AllContextScope()
	}
	return ContextScope{Kind: ContextScopeRegular, Name: name}
}

// CustomContextScope returns the scope for a saved custom project group.
func CustomContextScope(name string) ContextScope {
	if name == "" {
		return AllContextScope()
	}
	return ContextScope{Kind: ContextScopeCustom, Name: name}
}

// IsAll reports whether the scope spans every active project.
func (s ContextScope) IsAll() bool { return s.Kind == ContextScopeAll }

// IsCustom reports whether the scope targets a saved custom context.
func (s ContextScope) IsCustom() bool { return s.Kind == ContextScopeCustom }

// Projects resolves the scope to the active projects it selects. All and
// regular scopes keep the catalog's pinned-first order; a custom scope keeps its
// stored ID order. Every kind admits exactly the projects satisfying
// config.IsActiveContextMember, so a scope resolves to the same set the
// active-only cross-project loader will load and never previews a project the
// Outliner would drop.
func (s ContextScope) Projects(runtime *config.RuntimeState) []*models.Project {
	switch s.Kind {
	case ContextScopeCustom:
		return runtime.CustomContextProjects(s.Name)
	case ContextScopeRegular:
		var matched []*models.Project
		for _, p := range runtime.ActiveProjects() {
			if p.Context == s.Name {
				matched = append(matched, p)
			}
		}
		return matched
	default:
		return runtime.ActiveProjects()
	}
}

// ProjectIDs resolves the scope to the IDs of the projects it selects.
func (s ContextScope) ProjectIDs(runtime *config.RuntimeState) []string {
	projects := s.Projects(runtime)
	ids := make([]string, 0, len(projects))
	for _, p := range projects {
		ids = append(ids, p.ID)
	}
	return ids
}

// selectableContextScope degrades a regular scope with no active members to All.
// Regular scopes are derived from the labels active projects carry, so a label
// that loses its last active member stops being listed: keeping it active would
// scope the Outliner to a name the user can neither see marked active nor
// reselect. Custom scopes are stored by name and stay selectable while empty, so
// they are returned unchanged.
func selectableContextScope(runtime *config.RuntimeState, scope ContextScope) ContextScope {
	if scope.Kind != ContextScopeRegular || len(scope.Projects(runtime)) > 0 {
		return scope
	}
	return AllContextScope()
}

// ContextScopeSummary pairs a scope with the counts pickers and the context hub
// render. ArchivedProjectCount and StaleReferenceCount are only ever non-zero
// for custom scopes: those store project IDs that outlive archival and deletion,
// whereas All and regular scopes are derived from the live active-project set.
type ContextScopeSummary struct {
	Scope                ContextScope
	ActiveProjectCount   int
	ArchivedProjectCount int
	StaleReferenceCount  int
}

// SummarizeContextScope computes the active, archived, and stale counts
// a scope resolves to.
func SummarizeContextScope(runtime *config.RuntimeState, scope ContextScope) ContextScopeSummary {
	summary := ContextScopeSummary{
		Scope:              scope,
		ActiveProjectCount: len(scope.Projects(runtime)),
	}
	if scope.IsCustom() {
		summary.ArchivedProjectCount = runtime.CustomContextArchivedProjectCount(scope.Name)
		summary.StaleReferenceCount = runtime.CustomContextStaleReferenceCount(scope.Name)
	}
	return summary
}

// ContextScopeSummaries returns every selectable scope: All first, then custom
// contexts alphabetically, then project contexts alphabetically. It is the
// single construction point for context lists.
func ContextScopeSummaries(runtime *config.RuntimeState) []ContextScopeSummary {
	labels := make(map[string]struct{})
	for _, p := range runtime.ActiveProjects() {
		if p.Context != "" {
			labels[p.Context] = struct{}{}
		}
	}
	regularNames := make([]string, 0, len(labels))
	for name := range labels {
		regularNames = append(regularNames, name)
	}
	sort.Strings(regularNames)

	customNames := runtime.Machine().GetCustomContextNames()

	scopes := make([]ContextScope, 0, 1+len(customNames)+len(regularNames))
	scopes = append(scopes, AllContextScope())
	for _, name := range customNames {
		scopes = append(scopes, CustomContextScope(name))
	}
	for _, name := range regularNames {
		scopes = append(scopes, RegularContextScope(name))
	}

	summaries := make([]ContextScopeSummary, 0, len(scopes))
	for _, scope := range scopes {
		summaries = append(summaries, SummarizeContextScope(runtime, scope))
	}
	return summaries
}
