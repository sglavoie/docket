// Package filterops owns the facet and sort axes of the Outliner's one
// canonical query: the descriptors each key registers, the pickers a facet
// opens, and the single clause edit every one of them commits through.
//
// The package holds that behavior beside the state it reads rather than on the
// shell, because a facet touches exactly three things — the Outliner's query,
// the operation registry the keys are declared in, and the project populations
// a picker counts over. Shell is that port, and it is deliberately narrow: a
// filter that needs more than these has stopped being a view refinement.
package filterops

import (
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// Shell is what the facets need from the shell that owns them.
type Shell interface {
	// Outliner is the view whose one canonical query every facet edits.
	Outliner() *outliner.OutlinerModel

	// Registry and Executor are the descriptor authority and the invocation
	// pipeline a facet registers itself and its value provider with.
	Registry() *operation.OperationRegistry
	Executor() *operation.OperationExecutor

	// RegisterOperation registers one descriptor and its binding.
	RegisterOperation(operation.OperationDescriptor, operation.OperationBinding) error

	// TaskWorkspaceProjects is the base Outliner population: non-archived and
	// not parked. AvailableProjects is every non-archived project, which is what
	// keeps a parked one reachable from the project-tag facet, and
	// ArchivedProjects completes the vocabulary that facet counts over.
	TaskWorkspaceProjects() []*models.Project
	AvailableProjects() []*models.Project
	ArchivedProjects() []*models.Project
}

// Controller is the facet and sort cluster. It holds no state of its own: the
// filter state lives in the Outliner's query, which is what keeps a picker, a
// chip, a count, and the row predicate from disagreeing about what is filtered.
// The shell is passed to each call rather than held, so the zero value is
// usable and nothing here outlives the shell that asked for it.
type Controller struct{}
