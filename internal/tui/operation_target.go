package tui

import (
	"fmt"
	"sort"
)

// OperationTargetSource declares which part of the active UI context supplies
// an operation's target. It is execution metadata: descriptors remain free of
// model pointers and handlers receive stable identities only.
type OperationTargetSource string

const (
	OperationTargetCurrentTask    OperationTargetSource = "current-task"
	OperationTargetSelectedTasks  OperationTargetSource = "selected-tasks"
	OperationTargetCurrentProject OperationTargetSource = "current-project"
	OperationTargetScopedProjects OperationTargetSource = "scoped-projects"
	// OperationTargetCurrentEntity resolves the focused row whatever entity it
	// names. It falls back to the task and project sources so hosts that have
	// not yet filled CommandContext.CurrentEntity keep working.
	OperationTargetCurrentEntity OperationTargetSource = "current-entity"
	// OperationTargetSelectedEntities resolves the typed multi-selection and
	// falls back to the task selection, then the focused entity.
	OperationTargetSelectedEntities OperationTargetSource = "selected-entities"
	OperationTargetGlobal           OperationTargetSource = "global"
)

// OperationTarget is one resolved target. Entity is the canonical identity;
// Task and ProjectID stay populated so existing task and project handlers keep
// reading the fields they already use.
type OperationTarget struct {
	Entity    EntityIdentity
	Task      TaskIdentity
	ProjectID string
}

func operationTargetFor(entity EntityIdentity) OperationTarget {
	return OperationTarget{
		Entity:    entity,
		Task:      entity.Task(),
		ProjectID: entity.ProjectID,
	}
}

// ResolvedOperationTargets is the identity-only target set passed to operation
// handlers. Global operations intentionally have cardinality zero while still
// retaining an explicit source.
type ResolvedOperationTargets struct {
	Source  OperationTargetSource
	Targets []OperationTarget
}

// Count returns the cardinality checked against the descriptor.
func (targets ResolvedOperationTargets) Count() int {
	if targets.Source == OperationTargetGlobal {
		return 0
	}
	return len(targets.Targets)
}

// Tasks returns a defensive copy of every resolved task identity.
func (targets ResolvedOperationTargets) Tasks() []TaskIdentity {
	tasks := make([]TaskIdentity, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Task.Valid() {
			tasks = append(tasks, target.Task)
		}
	}
	return tasks
}

// Entities returns a defensive copy of every resolved typed identity.
func (targets ResolvedOperationTargets) Entities() []EntityIdentity {
	entities := make([]EntityIdentity, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Entity.Valid() {
			entities = append(entities, target.Entity)
		}
	}
	return entities
}

// ProjectIDs returns a defensive copy of every resolved project identity.
func (targets ResolvedOperationTargets) ProjectIDs() []string {
	projects := make([]string, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.ProjectID != "" {
			projects = append(projects, target.ProjectID)
		}
	}
	return projects
}

// OperationTargetResolver converts a pointer-free CommandContext into the
// exact identity set required by a descriptor.
type OperationTargetResolver struct{}

// Resolve resolves and validates one operation target set.
func (OperationTargetResolver) Resolve(
	descriptor OperationDescriptor,
	ctx CommandContext,
) (ResolvedOperationTargets, error) {
	source := descriptor.TargetSource
	if source == "" {
		source = defaultOperationTargetSource(descriptor.Category, descriptor.TargetCardinality)
	}

	var targets []OperationTarget
	switch source {
	case OperationTargetGlobal:
		targets = nil
	case OperationTargetCurrentTask:
		if current := currentTaskIdentity(ctx); current.Valid() {
			targets = []OperationTarget{operationTargetFor(current.Entity())}
		}
	case OperationTargetSelectedTasks:
		selected := normalizedTaskIdentities(ctx.Selection.Tasks)
		if len(selected) == 0 && !ctx.Selection.Visual {
			if current := currentTaskIdentity(ctx); current.Valid() {
				selected = []TaskIdentity{current}
			}
		}
		targets = make([]OperationTarget, 0, len(selected))
		for _, task := range selected {
			targets = append(targets, operationTargetFor(task.Entity()))
		}
	case OperationTargetCurrentProject:
		if projectID := currentProjectID(ctx); projectID != "" {
			targets = []OperationTarget{operationTargetFor(ProjectEntity(projectID))}
		}
	case OperationTargetScopedProjects:
		projects := normalizedProjectIDs(ctx.ScopeProjectIDs)
		if len(projects) == 0 && ctx.Entity.ProjectID != "" {
			projects = []string{ctx.Entity.ProjectID}
		}
		targets = make([]OperationTarget, 0, len(projects))
		for _, projectID := range projects {
			targets = append(targets, operationTargetFor(ProjectEntity(projectID)))
		}
	case OperationTargetCurrentEntity:
		if current := currentEntityIdentity(ctx); current.Valid() {
			targets = []OperationTarget{operationTargetFor(current)}
		}
	case OperationTargetSelectedEntities:
		selected := selectedEntityIdentities(ctx)
		targets = make([]OperationTarget, 0, len(selected))
		for _, entity := range selected {
			targets = append(targets, operationTargetFor(entity))
		}
	default:
		return ResolvedOperationTargets{}, fmt.Errorf(
			"operation %s has invalid target source %q",
			descriptor.ID,
			source,
		)
	}

	resolved := ResolvedOperationTargets{Source: source, Targets: targets}
	if !descriptor.TargetCardinality.Accepts(resolved.Count()) {
		return ResolvedOperationTargets{}, fmt.Errorf(
			"operation %s requires target cardinality %s; resolved %d from %s",
			descriptor.ID,
			descriptor.TargetCardinality,
			resolved.Count(),
			source,
		)
	}
	return resolved, nil
}

func currentTaskIdentity(ctx CommandContext) TaskIdentity {
	if ctx.Entity.TaskID == "" {
		return TaskIdentity{}
	}
	projectID := ctx.Entity.ProjectID
	if projectID == "" && len(ctx.ScopeProjectIDs) == 1 {
		projectID = ctx.ScopeProjectIDs[0]
	}
	return TaskIdentity{ProjectID: projectID, TaskID: ctx.Entity.TaskID}
}

func currentProjectID(ctx CommandContext) string {
	if ctx.Entity.ProjectID != "" {
		return ctx.Entity.ProjectID
	}
	return firstProjectID(ctx.ScopeProjectIDs)
}

// currentEntityIdentity prefers the typed focused identity and degrades to the
// task and project sources so both migrated and unmigrated hosts resolve.
func currentEntityIdentity(ctx CommandContext) EntityIdentity {
	if current := normalizeEntityIdentity(ctx.CurrentEntity); current.Valid() {
		return current
	}
	if task := currentTaskIdentity(ctx); task.Valid() {
		return task.Entity()
	}
	if projectID := currentProjectID(ctx); projectID != "" {
		return ProjectEntity(projectID)
	}
	return EntityIdentity{}
}

func selectedEntityIdentities(ctx CommandContext) []EntityIdentity {
	if selected := normalizedEntityIdentities(ctx.Selection.EntityIdentities); len(selected) > 0 {
		return selected
	}
	if selected := taskEntityIdentities(ctx.Selection.Tasks); len(selected) > 0 {
		return selected
	}
	if ctx.Selection.Visual {
		return nil
	}
	if current := currentEntityIdentity(ctx); current.Valid() {
		return []EntityIdentity{current}
	}
	return nil
}

func normalizedTaskIdentities(tasks []TaskIdentity) []TaskIdentity {
	seen := make(map[TaskIdentity]bool, len(tasks))
	normalized := make([]TaskIdentity, 0, len(tasks))
	for _, task := range tasks {
		if !task.Valid() || seen[task] {
			continue
		}
		seen[task] = true
		normalized = append(normalized, task)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].ProjectID != normalized[j].ProjectID {
			return normalized[i].ProjectID < normalized[j].ProjectID
		}
		return normalized[i].TaskID < normalized[j].TaskID
	})
	return normalized
}

func normalizedProjectIDs(projects []string) []string {
	seen := make(map[string]bool, len(projects))
	normalized := make([]string, 0, len(projects))
	for _, projectID := range projects {
		if projectID == "" || seen[projectID] {
			continue
		}
		seen[projectID] = true
		normalized = append(normalized, projectID)
	}
	sort.Strings(normalized)
	return normalized
}

func firstProjectID(projects []string) string {
	normalized := normalizedProjectIDs(projects)
	if len(normalized) == 0 {
		return ""
	}
	return normalized[0]
}

func defaultOperationTargetSource(
	category OperationCategory,
	cardinality TargetCardinality,
) OperationTargetSource {
	if cardinality == TargetCardinalityNone {
		return OperationTargetGlobal
	}
	switch category {
	case OperationCategoryTask, OperationCategoryTag:
		if cardinality == TargetCardinalityOne {
			return OperationTargetCurrentTask
		}
		return OperationTargetSelectedTasks
	case OperationCategoryProject, OperationCategoryNote:
		if cardinality == TargetCardinalityOne {
			return OperationTargetCurrentProject
		}
		return OperationTargetScopedProjects
	default:
		if cardinality == TargetCardinalityOne {
			return OperationTargetCurrentProject
		}
		return OperationTargetScopedProjects
	}
}

func validOperationTargetSource(source OperationTargetSource) bool {
	switch source {
	case OperationTargetCurrentTask,
		OperationTargetSelectedTasks,
		OperationTargetCurrentProject,
		OperationTargetScopedProjects,
		OperationTargetCurrentEntity,
		OperationTargetSelectedEntities,
		OperationTargetGlobal:
		return true
	default:
		return false
	}
}
