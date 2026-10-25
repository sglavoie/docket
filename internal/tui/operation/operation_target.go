package operation

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/sglavoie/docket/internal/tui/taskref"
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
	Task      taskref.TaskIdentity
	ProjectID string
}

func OperationTargetFor(entity EntityIdentity) OperationTarget {
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
func (targets ResolvedOperationTargets) Tasks() []taskref.TaskIdentity {
	tasks := make([]taskref.TaskIdentity, 0, len(targets.Targets))
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
		if current := CurrentTaskIdentity(ctx); current.Valid() {
			targets = []OperationTarget{OperationTargetFor(EntityForTaskIdentity(current))}
		}
	case OperationTargetSelectedTasks:
		selected := NormalizedTaskIdentities(ctx.Selection.Tasks)
		if len(selected) == 0 && !ctx.Selection.Visual {
			if current := CurrentTaskIdentity(ctx); current.Valid() {
				selected = []taskref.TaskIdentity{current}
			}
		}
		targets = make([]OperationTarget, 0, len(selected))
		for _, task := range selected {
			targets = append(targets, OperationTargetFor(EntityForTaskIdentity(task)))
		}
	case OperationTargetCurrentProject:
		if projectID := currentProjectID(ctx); projectID != "" {
			targets = []OperationTarget{OperationTargetFor(ProjectEntity(projectID))}
		}
	case OperationTargetScopedProjects:
		projects := NormalizedProjectIDs(ctx.ScopeProjectIDs)
		if len(projects) == 0 && ctx.Entity.ProjectID != "" {
			projects = []string{ctx.Entity.ProjectID}
		}
		targets = make([]OperationTarget, 0, len(projects))
		for _, projectID := range projects {
			targets = append(targets, OperationTargetFor(ProjectEntity(projectID)))
		}
	case OperationTargetCurrentEntity:
		if current := CurrentEntityIdentity(ctx); current.Valid() {
			targets = []OperationTarget{OperationTargetFor(current)}
		}
	case OperationTargetSelectedEntities:
		selected := selectedEntityIdentities(ctx)
		targets = make([]OperationTarget, 0, len(selected))
		for _, entity := range selected {
			targets = append(targets, OperationTargetFor(entity))
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
		return ResolvedOperationTargets{}, OperationTargetRejection(descriptor)
	}
	// The entity sources degrade to the focused project when the cursor names no
	// task, so a task operation reaching one of them would act on whichever
	// project the header row belongs to. Rejecting here keeps that impossible for
	// every source, present and future, instead of at each call site.
	if descriptor.Category == OperationCategoryTask {
		for _, target := range resolved.Targets {
			if target.Entity.Kind != EntityKindTask {
				return ResolvedOperationTargets{}, OperationTargetRejection(descriptor)
			}
		}
	}
	return resolved, nil
}

// OperationTargetRejection is the one sentence every surface shows when a bound
// operation resolves no target it can act on. Registry-resolved and delegated
// operations both return it, so a task key pressed on a project header row reads
// the same however it entered the grammar.
func OperationTargetRejection(descriptor OperationDescriptor) error {
	return errors.New(OperationTargetRejectionMessage(operationTargetRejectionCategory(descriptor)))
}

// operationTargetRejectionCategory describes the entity a failed target
// resolution needed, independently of the operation's help grouping. Navigation
// operations such as Focus and Peek still require a task, while navigation
// operations such as Reorder can resolve a project under the cursor.
func operationTargetRejectionCategory(descriptor OperationDescriptor) OperationCategory {
	switch descriptor.Category {
	case OperationCategoryTask, OperationCategoryTag, OperationCategoryProject:
		return descriptor.Category
	case OperationCategoryNavigation, OperationCategoryMode, OperationCategoryScope,
		OperationCategoryFilter, OperationCategorySort, OperationCategoryNote,
		OperationCategorySystem:
		// Neutral help categories derive their target wording from the source.
	}
	source := descriptor.TargetSource
	if source == "" {
		source = defaultOperationTargetSource(descriptor.Category, descriptor.TargetCardinality)
	}
	switch source {
	case OperationTargetCurrentTask, OperationTargetSelectedTasks:
		return OperationCategoryTask
	case OperationTargetCurrentProject, OperationTargetScopedProjects:
		return OperationCategoryProject
	default:
		return descriptor.Category
	}
}

// archivedTaskTargetRejection refuses any operation that would change a task an
// archived project owns. The Fa project filter deliberately loads archived
// projects into the ordinary Outliner, so the read-only lifecycle rule has to
// hold where every operation passes rather than where the population is built.
// A visual selection is judged whole: one archived member refuses the whole
// operation so a mixed selection can never mutate its active subset.
func archivedTaskTargetRejection(descriptor OperationDescriptor, ctx CommandContext) error {
	if descriptor.Risk == OperationRiskReadOnly {
		return nil
	}
	// Target-free operations act on the workspace rather than the cursor row, so
	// capture, theme, undo, and redo stay usable while archived rows are shown.
	if descriptor.TargetCardinality == TargetCardinalityNone {
		return nil
	}
	if ctx.Selection.Visual && len(ctx.Selection.Entities) > 0 {
		if slices.ContainsFunc(ctx.Selection.Entities, entityContextNamesArchivedTask) {
			return errors.New("selection includes an archived project; restore it to change these tasks")
		}
		return nil
	}
	if entityContextNamesArchivedTask(ctx.Entity) {
		return errors.New("project is archived; restore it to change this task")
	}
	return nil
}

// checklistCursorRejection refuses every operation that would act on the owner
// task while the cursor rests on one of its checklist items. The row carries
// the owner's pointer, so ordinary target resolution finds a perfectly valid
// task and the whole task grammar would otherwise mutate one level up from
// where the user is pointing. It runs beside archivedTaskTargetRejection,
// before resolution, because delegated bindings swallow resolver errors and
// target-free operations resolve no target for a resolver guard to inspect.
func checklistCursorRejection(descriptor OperationDescriptor, ctx CommandContext) error {
	if !ctx.Entity.HasChecklistCursor || !operationRefusedOnChecklistRow(descriptor) {
		return nil
	}
	return errors.New(ChecklistRowRejectionMessage)
}

// entityContextNamesArchivedTask keys on the task identity so project lifecycle
// operations, which legitimately act on archived projects, stay untouched.
func entityContextNamesArchivedTask(entity EntityContext) bool {
	return entity.TaskID != "" && entity.Archived
}

func OperationTargetRejectionMessage(category OperationCategory) string {
	switch category {
	case OperationCategoryTask, OperationCategoryTag:
		return "move the cursor to a task to use this action"
	case OperationCategoryProject:
		return "move the cursor to a project to use this action"
	default:
		return "this action has no target here"
	}
}

func CurrentTaskIdentity(ctx CommandContext) taskref.TaskIdentity {
	if ctx.Entity.TaskID == "" {
		return taskref.TaskIdentity{}
	}
	projectID := ctx.Entity.ProjectID
	if projectID == "" && len(ctx.ScopeProjectIDs) == 1 {
		projectID = ctx.ScopeProjectIDs[0]
	}
	return taskref.TaskIdentity{ProjectID: projectID, TaskID: ctx.Entity.TaskID}
}

func currentProjectID(ctx CommandContext) string {
	if ctx.Entity.ProjectID != "" {
		return ctx.Entity.ProjectID
	}
	return firstProjectID(ctx.ScopeProjectIDs)
}

// CurrentEntityIdentity prefers the typed focused identity and degrades to the
// task and project sources so both migrated and unmigrated hosts resolve.
func CurrentEntityIdentity(ctx CommandContext) EntityIdentity {
	if current := NormalizeEntityIdentity(ctx.CurrentEntity); current.Valid() {
		return current
	}
	if task := CurrentTaskIdentity(ctx); task.Valid() {
		return EntityForTaskIdentity(task)
	}
	if projectID := currentProjectID(ctx); projectID != "" {
		return ProjectEntity(projectID)
	}
	return EntityIdentity{}
}

func selectedEntityIdentities(ctx CommandContext) []EntityIdentity {
	if selected := NormalizedEntityIdentities(ctx.Selection.EntityIdentities); len(selected) > 0 {
		return selected
	}
	if selected := TaskEntityIdentities(ctx.Selection.Tasks); len(selected) > 0 {
		return selected
	}
	if ctx.Selection.Visual {
		return nil
	}
	if current := CurrentEntityIdentity(ctx); current.Valid() {
		return []EntityIdentity{current}
	}
	return nil
}

func NormalizedTaskIdentities(tasks []taskref.TaskIdentity) []taskref.TaskIdentity {
	seen := make(map[taskref.TaskIdentity]bool, len(tasks))
	normalized := make([]taskref.TaskIdentity, 0, len(tasks))
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

func NormalizedProjectIDs(projects []string) []string {
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
	normalized := NormalizedProjectIDs(projects)
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

func ValidOperationTargetSource(source OperationTargetSource) bool {
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
