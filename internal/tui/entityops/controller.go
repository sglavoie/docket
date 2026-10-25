// Package entityops owns the three task-owned entity families that hang off a
// task rather than off the shell: attachments, comments, and checklist rows.
// Each is a descriptor catalog, the action sheet its Space menu opens, and the
// coordinator call its mutations commit through.
//
// They share a package because they share a shape — a panel below one task, a
// manifest of its own, and no state on the shell at all — and because their
// port is the same short list either way. Shell is that port: the registry and
// executor they register with, the panel the current route is holding, and the
// one coordinator every mutation here travels through.
package entityops

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// Shell is what the task-owned entity families need from the shell that owns
// them.
type Shell interface {
	// Registry and Executor are the descriptor authority and the invocation
	// pipeline each family registers itself with.
	Registry() *operation.OperationRegistry
	Executor() *operation.OperationExecutor

	// RegisterOperation registers one descriptor and its binding.
	RegisterOperation(operation.OperationDescriptor, operation.OperationBinding) error

	// MountActionSheet mounts the sheet a family's Space menu opens. The sheet
	// is the shell's overlay, so the shell is what holds it.
	MountActionSheet(*listsurface.ContextActionSheetModel)

	// CurrentView and RenderedView are how a family reaches the live panel it is
	// acting on: the route the shell is showing, and the renderer installed
	// under a presentation ID.
	CurrentView() presentation.PresentationID
	RenderedView(presentation.PresentationID) tea.Model

	// SubviewCoordinator is the one application-backed seam every checklist
	// mutation commits through.
	SubviewCoordinator() coordinator.SubviewCoordinator

	// OutlinerPrimary is the base Outliner, when the shell is holding one.
	OutlinerPrimary() (*outliner.OutlinerModel, bool)

	// TaskAndProjectForIdentity resolves a stable identity out of the current
	// snapshot, which is what keeps a refusal reading the same task the
	// invocation named.
	TaskAndProjectForIdentity(taskref.TaskIdentity) (*models.Task, *models.Project)
}

// Controller is the entity-family cluster. It holds no state of its own: an
// attachment list, a comment thread, and a checklist all live on the task, and
// the panels that show them are the shell's overlays. The shell travels in per
// call, so the zero value is usable.
type Controller struct{}
