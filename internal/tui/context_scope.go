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

// IsAll reports whether the scope spans every active project.
func (s ContextScope) IsAll() bool { return s.Kind == ContextScopeAll }

// Projects resolves the scope to the active projects it selects. Both scope
// kinds keep the catalog's pinned-first order and admit exactly the projects
// satisfying config.IsActiveContextMember, so a scope resolves to the same set
// the active-only cross-project loader will load.
func (s ContextScope) Projects(runtime *config.RuntimeState) []*models.Project {
	switch s.Kind {
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
// The protected personal context stays listed while empty, but an active empty
// scope still falls back so the Outliner never remains stranded on no projects.
func selectableContextScope(runtime *config.RuntimeState, scope ContextScope) ContextScope {
	if scope.Kind != ContextScopeRegular || len(scope.Projects(runtime)) > 0 {
		return scope
	}
	return AllContextScope()
}

// ContextScopeSummary pairs a scope with the active count used by pickers and
// the Scope Hub render.
type ContextScopeSummary struct {
	Scope              ContextScope
	ActiveProjectCount int
}

// SummarizeContextScope computes the active count a scope resolves to.
func SummarizeContextScope(runtime *config.RuntimeState, scope ContextScope) ContextScopeSummary {
	return ContextScopeSummary{
		Scope:              scope,
		ActiveProjectCount: len(scope.Projects(runtime)),
	}
}

// ContextScopeSummaries returns every selectable scope: All first, then regular
// project-label contexts alphabetically. The protected personal context is
// always present even when no active project currently carries it.
func ContextScopeSummaries(runtime *config.RuntimeState) []ContextScopeSummary {
	labels := map[string]struct{}{runtime.DefaultContext(): {}}
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

	scopes := make([]ContextScope, 0, 1+len(regularNames))
	scopes = append(scopes, AllContextScope())
	for _, name := range regularNames {
		scopes = append(scopes, RegularContextScope(name))
	}

	summaries := make([]ContextScopeSummary, 0, len(scopes))
	for _, scope := range scopes {
		summaries = append(summaries, SummarizeContextScope(runtime, scope))
	}
	return summaries
}
