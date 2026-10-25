package operation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// OperationEntityKind names the domain entity an operation target identifies.
type OperationEntityKind string

const (
	EntityKindNone      OperationEntityKind = ""
	EntityKindTask      OperationEntityKind = "task"
	EntityKindProject   OperationEntityKind = "project"
	EntityKindNote      OperationEntityKind = "note"
	EntityKindTag       OperationEntityKind = "tag"
	EntityKindScope     OperationEntityKind = "scope"
	EntityKindMode      OperationEntityKind = "mode"
	EntityKindTool      OperationEntityKind = "tool"
	EntityKindOperation OperationEntityKind = "operation"
	EntityKindLens      OperationEntityKind = "lens"
)

// OperationEntityScope distinguishes an entity the whole workspace owns from
// one a single project owns. Notes and tags exist in both forms; every other
// kind leaves it unset.
type OperationEntityScope string

const (
	EntityScopeUnset OperationEntityScope = ""
	// EntityScopeGlobal covers workspace-wide notes and tags. It maps onto
	// app.NoteScopeGlobal for notes.
	EntityScopeGlobal OperationEntityScope = "global"
	// EntityScopeProject covers entities one project owns.
	EntityScopeProject OperationEntityScope = "project"
)

// EntityIdentity is one comparable, pointer-free identity for any entity an
// operation can target. Handlers reload live domain state from it instead of
// retaining a model pointer that a later rebuild would invalidate.
type EntityIdentity struct {
	Kind      OperationEntityKind
	Scope     OperationEntityScope
	ProjectID string
	TaskID    string
	// Name is a note filename, a normalized tag, or the stable catalog ID of a
	// scope, mode, tool, operation, or named lens.
	Name string
}

// LensEntity returns the identity of one saved named lens. The lens UUID is the
// stable pointer-free handle: a lens list rebuild or a machine-state reload
// never invalidates it, and a UUID the machine snapshot no longer carries is
// what a handler reports as a stale target.
func LensEntity(id string) EntityIdentity {
	return NormalizeEntityIdentity(EntityIdentity{
		Kind: EntityKindLens,
		Name: id,
	})
}

// TaskEntity returns the project-qualified identity of one task.
func TaskEntity(projectID, taskID string) EntityIdentity {
	return NormalizeEntityIdentity(EntityIdentity{
		Kind:      EntityKindTask,
		ProjectID: projectID,
		TaskID:    taskID,
	})
}

// ProjectEntity returns the identity of one project.
func ProjectEntity(projectID string) EntityIdentity {
	return NormalizeEntityIdentity(EntityIdentity{
		Kind:      EntityKindProject,
		ProjectID: projectID,
	})
}

// GlobalNoteEntity returns the identity of one workspace scratch note.
func GlobalNoteEntity(filename string) EntityIdentity {
	return NormalizeEntityIdentity(EntityIdentity{
		Kind:  EntityKindNote,
		Scope: EntityScopeGlobal,
		Name:  filename,
	})
}

// ProjectNoteEntity returns the identity of one project note.
func ProjectNoteEntity(projectID, filename string) EntityIdentity {
	return NormalizeEntityIdentity(EntityIdentity{
		Kind:      EntityKindNote,
		Scope:     EntityScopeProject,
		ProjectID: projectID,
		Name:      filename,
	})
}

// EntityForTaskIdentity converts a task identity into the typed operation
// identity. It is Task's inverse and a free function rather than a method
// because taskref owns TaskIdentity, so only that package may declare on it.
func EntityForTaskIdentity(id taskref.TaskIdentity) EntityIdentity {
	return TaskEntity(id.ProjectID, id.TaskID)
}

// Task returns the task identity a task entity names. Other kinds return the
// zero identity.
func (id EntityIdentity) Task() taskref.TaskIdentity {
	if id.Kind != EntityKindTask {
		return taskref.TaskIdentity{}
	}
	return taskref.TaskIdentity{ProjectID: id.ProjectID, TaskID: id.TaskID}
}

// Valid reports whether the identity carries exactly the fields its kind
// defines. A malformed identity never resolves to a target.
func (id EntityIdentity) Valid() bool {
	switch id.Kind {
	case EntityKindTask:
		return id.Scope == EntityScopeUnset && id.Name == "" &&
			id.ProjectID != "" && id.TaskID != ""
	case EntityKindProject:
		return id.Scope == EntityScopeUnset && id.Name == "" &&
			id.TaskID == "" && id.ProjectID != ""
	case EntityKindNote, EntityKindTag:
		if id.TaskID != "" || id.Name == "" {
			return false
		}
		switch id.Scope {
		case EntityScopeGlobal:
			return id.ProjectID == ""
		case EntityScopeProject:
			return id.ProjectID != ""
		default:
			return false
		}
	case EntityKindScope,
		EntityKindMode,
		EntityKindTool,
		EntityKindOperation,
		EntityKindLens:
		return id.Scope == EntityScopeUnset && id.ProjectID == "" &&
			id.TaskID == "" && id.Name != ""
	default:
		return false
	}
}

// String renders a stable diagnostic form used in execution errors.
func (id EntityIdentity) String() string {
	switch id.Kind {
	case EntityKindNone:
		return "none"
	case EntityKindTask:
		return fmt.Sprintf("task:%s/%s", id.ProjectID, id.TaskID)
	case EntityKindProject:
		return fmt.Sprintf("project:%s", id.ProjectID)
	case EntityKindNote, EntityKindTag:
		if id.Scope == EntityScopeProject {
			return fmt.Sprintf("%s:%s/%s", id.Kind, id.ProjectID, id.Name)
		}
		return fmt.Sprintf("%s:%s", id.Kind, id.Name)
	default:
		return fmt.Sprintf("%s:%s", id.Kind, id.Name)
	}
}

func NormalizeEntityIdentity(id EntityIdentity) EntityIdentity {
	id.ProjectID = strings.TrimSpace(id.ProjectID)
	id.TaskID = strings.TrimSpace(id.TaskID)
	id.Name = strings.TrimSpace(id.Name)
	if id.Kind == EntityKindTag {
		id.Name = models.NormalizeTag(id.Name)
	}
	return id
}

// NormalizedEntityIdentities drops malformed and duplicate identities and
// returns the remainder in one deterministic order.
func NormalizedEntityIdentities(identities []EntityIdentity) []EntityIdentity {
	seen := make(map[EntityIdentity]bool, len(identities))
	normalized := make([]EntityIdentity, 0, len(identities))
	for _, identity := range identities {
		identity = NormalizeEntityIdentity(identity)
		if !identity.Valid() || seen[identity] {
			continue
		}
		seen[identity] = true
		normalized = append(normalized, identity)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return entityIdentityLess(normalized[i], normalized[j])
	})
	return normalized
}

func entityIdentityLess(left, right EntityIdentity) bool {
	for _, pair := range [][2]string{
		{string(left.Kind), string(right.Kind)},
		{string(left.Scope), string(right.Scope)},
		{left.ProjectID, right.ProjectID},
		{left.TaskID, right.TaskID},
		{left.Name, right.Name},
	} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	return false
}

// TaskEntityIdentities converts selected task identities into typed identities
// so hosts fill the typed selection without duplicating normalization.
func TaskEntityIdentities(tasks []taskref.TaskIdentity) []EntityIdentity {
	normalized := NormalizedTaskIdentities(tasks)
	identities := make([]EntityIdentity, 0, len(normalized))
	for _, task := range normalized {
		identities = append(identities, EntityForTaskIdentity(task))
	}
	return identities
}

// CommandContextCurrentEntity derives the focused typed identity from the
// untyped entity snapshot every host already fills in.
func CommandContextCurrentEntity(entity EntityContext) EntityIdentity {
	switch {
	case entity.ProjectID != "" && entity.TaskID != "":
		return TaskEntity(entity.ProjectID, entity.TaskID)
	case entity.ProjectID != "":
		return ProjectEntity(entity.ProjectID)
	default:
		return EntityIdentity{}
	}
}

// CommandContextNamesNonTaskEntity reports whether the focused row positively
// identifies something other than a task. An empty identity is not a non-task
// cursor: it means the host has not named the row at all.
func CommandContextNamesNonTaskEntity(ctx CommandContext) bool {
	current := NormalizeEntityIdentity(ctx.CurrentEntity)
	return current.Valid() && current.Kind != EntityKindTask
}
