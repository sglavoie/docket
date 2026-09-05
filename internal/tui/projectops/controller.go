// Package projectops owns the project cluster: every project act the shell
// holds open while it waits for an answer, the descriptors those acts are
// reached by, the Manage hub projection they are performed from, and the
// project-tag vocabulary the same rules are edited through.
//
// The state moved with the behavior. Arming a prompt, answering its keys, and
// handing the pending edit back are this package's; every commit stays on the
// shell, because the runtime domains, the application service, and the recovery
// feedback live there. Nothing here reaches storage, so the boundary can add no
// catalog write of its own.
//
// The three axes stay three axes. No field here names a task lifecycle state: a
// task's workflow state is never read or written by a project act.
package projectops

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// Shell is what the project cluster needs from the shell that owns it.
type Shell interface {
	// Outliner is the view whose scope and population a project act reads.
	Outliner() *outliner.OutlinerModel

	// Registry and Executor are the descriptor authority and the invocation
	// pipeline the project operations register themselves with.
	Registry() *operation.OperationRegistry
	Executor() *operation.OperationExecutor

	// RegisterOperation registers one descriptor and its binding.
	RegisterOperation(operation.OperationDescriptor, operation.OperationBinding) error

	// MountActionSheet holds the vocabulary sheet this cluster opens. The
	// overlay stack is the shell's.
	MountActionSheet(*listsurface.ContextActionSheetModel)

	// OperationCommandContext is the view-scoped context a target-free sheet is
	// built from.
	OperationCommandContext() operation.CommandContext

	// ManageHub is the Manage destination the project hub is a module of.
	ManageHub() *projecthub.ManageHubModel

	// AvailableProjects is every non-archived project, parked ones included;
	// ArchivedProjects completes the catalog the hub and the tag vocabulary
	// count over. ProjectByID resolves one of them.
	AvailableProjects() []*models.Project
	ArchivedProjects() []*models.Project
	ProjectByID(string) *models.Project

	// AllProjectTags is the workspace-wide project-tag vocabulary, and
	// ProjectTagVocabularyCounts is the same vocabulary counted by the facet
	// that narrows on it.
	AllProjectTags() []string
	ProjectTagVocabularyCounts() map[string]int

	// IsSystemUnfiledProject names the one project a delete must refuse.
	IsSystemUnfiledProject(string) bool
}
