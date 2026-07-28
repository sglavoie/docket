package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
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
	// scope, mode, tool, or operation.
	Name string
}

// TaskEntity returns the project-qualified identity of one task.
func TaskEntity(projectID, taskID string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind:      EntityKindTask,
		ProjectID: projectID,
		TaskID:    taskID,
	})
}

// ProjectEntity returns the identity of one project.
func ProjectEntity(projectID string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind:      EntityKindProject,
		ProjectID: projectID,
	})
}

// GlobalNoteEntity returns the identity of one workspace scratch note.
func GlobalNoteEntity(filename string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind:  EntityKindNote,
		Scope: EntityScopeGlobal,
		Name:  filename,
	})
}

// ProjectNoteEntity returns the identity of one project note.
func ProjectNoteEntity(projectID, filename string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind:      EntityKindNote,
		Scope:     EntityScopeProject,
		ProjectID: projectID,
		Name:      filename,
	})
}

// WorkspaceTagEntity returns the identity of one tag aggregated across the
// workspace.
func WorkspaceTagEntity(name string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind:  EntityKindTag,
		Scope: EntityScopeGlobal,
		Name:  name,
	})
}

// ProjectTagEntity returns the identity of one tag inside one project.
func ProjectTagEntity(projectID, name string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind:      EntityKindTag,
		Scope:     EntityScopeProject,
		ProjectID: projectID,
		Name:      name,
	})
}

// ScopeEntity returns the identity of one context scope, reusing the stable
// scope catalog ID the Scope Hub already lists rows under.
func ScopeEntity(scope ContextScope) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{
		Kind: EntityKindScope,
		Name: string(contextScopeListItemID(scope)),
	})
}

// ModeEntity returns the identity of one Outliner catalog mode.
func ModeEntity(id OutlinerModeID) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindMode, Name: string(id)})
}

// ToolEntity returns the identity of one promoted or secondary tool.
func ToolEntity(id string) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindTool, Name: id})
}

// OperationEntity returns the identity of one registered operation, letting an
// operation target another operation.
func OperationEntity(id OperationID) EntityIdentity {
	return normalizeEntityIdentity(EntityIdentity{Kind: EntityKindOperation, Name: string(id)})
}

// Entity converts a task identity into the typed operation identity.
func (id TaskIdentity) Entity() EntityIdentity { return TaskEntity(id.ProjectID, id.TaskID) }

// Task returns the task identity a task entity names. Other kinds return the
// zero identity.
func (id EntityIdentity) Task() TaskIdentity {
	if id.Kind != EntityKindTask {
		return TaskIdentity{}
	}
	return TaskIdentity{ProjectID: id.ProjectID, TaskID: id.TaskID}
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
		EntityKindOperation:
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

func normalizeEntityIdentity(id EntityIdentity) EntityIdentity {
	id.ProjectID = strings.TrimSpace(id.ProjectID)
	id.TaskID = strings.TrimSpace(id.TaskID)
	id.Name = strings.TrimSpace(id.Name)
	if id.Kind == EntityKindTag {
		id.Name = models.NormalizeTag(id.Name)
	}
	return id
}

// normalizedEntityIdentities drops malformed and duplicate identities and
// returns the remainder in one deterministic order.
func normalizedEntityIdentities(identities []EntityIdentity) []EntityIdentity {
	seen := make(map[EntityIdentity]bool, len(identities))
	normalized := make([]EntityIdentity, 0, len(identities))
	for _, identity := range identities {
		identity = normalizeEntityIdentity(identity)
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

// taskEntityIdentities converts selected task identities into typed identities
// so hosts fill the typed selection without duplicating normalization.
func taskEntityIdentities(tasks []TaskIdentity) []EntityIdentity {
	normalized := normalizedTaskIdentities(tasks)
	identities := make([]EntityIdentity, 0, len(normalized))
	for _, task := range normalized {
		identities = append(identities, task.Entity())
	}
	return identities
}

// commandContextCurrentEntity derives the focused typed identity from the
// untyped entity snapshot every host already fills in.
func commandContextCurrentEntity(entity EntityContext) EntityIdentity {
	switch {
	case entity.ProjectID != "" && entity.TaskID != "":
		return TaskEntity(entity.ProjectID, entity.TaskID)
	case entity.ProjectID != "":
		return ProjectEntity(entity.ProjectID)
	default:
		return EntityIdentity{}
	}
}
