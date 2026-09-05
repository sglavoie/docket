// Package notesops owns the shell half of Notes: the workspace-wide note
// search index the Outliner's lowest search tier reads, the note operations the
// Notes surface registers, and the surface sheet those operations are reached
// from.
//
// The index moved with the behavior. Its generation, its owner fingerprint, and
// the rows it adopted are this package's, because every read and every
// invalidation of them is here. Everything below stays where it was: the Notes
// surface itself is internal/tui/notes, and the mutations commit through the
// application service the shell hands in.
//
// Notes stay location-qualified and exclude archived projects, and nothing here
// gives them tags, task links, pins, or bulk actions.
package notesops

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// Shell is what the notes cluster needs from the shell that owns it.
type Shell interface {
	// Outliner is the view whose search tier the index feeds and whose
	// population an adopted index invalidates.
	Outliner() *outliner.OutlinerModel

	// Registry and Executor are the descriptor authority and the invocation
	// pipeline the note operations register themselves with.
	Registry() *operation.OperationRegistry
	Executor() *operation.OperationExecutor

	// RegisterOperation registers one descriptor and its binding.
	RegisterOperation(operation.OperationDescriptor, operation.OperationBinding) error

	// MountActionSheet holds the Notes surface sheet. The overlay stack is the
	// shell's.
	MountActionSheet(*listsurface.ContextActionSheetModel)

	// ManageHub is the Manage destination the Notes surface is a module of.
	ManageHub() *projecthub.ManageHubModel

	// DomainService is the application service a note mutation commits through,
	// and DomainRuntime is the workspace the note locations are resolved below.
	DomainService() *app.Service
	DomainRuntime() *config.RuntimeState

	// AvailableProjects is every non-archived project — the note owners, since
	// Notes excludes archived projects. ProjectByID resolves one of them.
	AvailableProjects() []*models.Project
	ProjectByID(string) *models.Project
}

// Controller is the notes cluster. It holds the one piece of state the cluster
// owns — the workspace-wide note search index — and nothing else: the surface,
// its preview offset, and its scope all live on the Notes view model. The shell
// travels in per call, so the zero value is usable.
type Controller struct {
	index noteIndex
}
