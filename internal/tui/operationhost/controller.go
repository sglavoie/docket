// Package operationhost owns the shell half of the operation grammar: the
// descriptor authority and the invocation pipeline, the overlay flow one
// invocation can open on its way to an answer, the command context every
// invocation resolves its targets against, and the two startup checks that hold
// the registered catalog to the one-path rule and to the menu bar.
//
// The wiring moved with its state. What did not move is the operations
// themselves: every descriptor is still declared by one of the concrete
// registrars on the shell, and every handler is still a bound shell method
// traveling as an OperationBinding value, because the runtime domains, the
// application service and the coordinators live there. Nothing here reaches
// storage.
package operationhost

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

// Shell is what the operation host needs from the shell that owns it. Every
// method is a read of where the user is or a construction the shell alone can
// perform, which is what keeps the two value factories — the picker and the
// typed prompt — outside this package: each dispatches across the shell's other
// clusters, and a factory that named them all would put the whole shell behind
// this port.
type Shell interface {
	// CurrentView and ActiveView name the route the context is resolved against.
	CurrentView() presentation.PresentationID
	ActiveView() route.View

	// ActiveOutliner is the list the cursor and the project scope are read from
	// when one is mounted.
	ActiveOutliner() *outliner.OutlinerModel

	// The cursor reads. Each answers one row kind, and the context carries all of
	// them because a descriptor's applicability may name any one.
	CursorTaskAndProject() (*models.Task, *models.Project)
	CursorProjectHeaderProject() *models.Project
	CursorChecklistItem() (outliner.ChecklistCursor, bool)
	CursorNamesSectionHeader() bool
	CursorNoteIdentity() (notes.NoteRowIdentity, bool)

	// CurrentProject is the stacked project view's own project, which is the
	// scope when no Outliner is mounted at all.
	CurrentProject() *models.Project

	// TaskWorkspaceProjects is the base population — non-archived and not parked
	// — and ProjectByID resolves one project of the whole catalog.
	TaskWorkspaceProjects() []*models.Project
	ProjectByID(string) *models.Project

	// SetError retains a refusal on the shell's own feedback line.
	SetError(error)

	// NewValuePicker and NewTextPrompt build the value step a provider asks for.
	// Both dispatch across clusters this package does not own.
	NewValuePicker(
		operation.OperationExecutionRequest,
		operation.ValueProviderID,
	) (*valuepicker.OperationValuePickerModel, error)
	NewTextPrompt(
		operation.OperationExecutionRequest,
		operation.ValueProviderID,
	) (*OperationTextPromptModel, error)

	// ResumeWithoutSheet runs a resumed request when no action sheet is mounted.
	// The lens picker is the one surface an operation is raised from without a
	// sheet between, so which of the two answers is the shell's to decide.
	ResumeWithoutSheet(operation.OperationExecutionRequest) tea.Cmd

	// RefreshNamedLensPicker redraws the picker a dismissed sheet was opened
	// over, whose row may have been renamed, rewritten, or removed underneath.
	RefreshNamedLensPicker()
}

// Controller is the shell's operation-wiring cluster: the descriptor authority,
// the invocation pipeline, and the two-level overlay flow one invocation can
// open on its way to an answer — the contextual action sheet, the value step (a
// picker, or a typed prompt when the provider says so), and the confirmation a
// destructive action stops at with its frozen request. The keys already pressed
// toward an unfinished chord live here too, because a prefix is only meaningful
// against the registry that declared it.
//
// The two registries stay two registries and stay singular. Registry is the
// descriptor authority and Executor the single invocation pipeline: register
// once through this type and let the projections — direct dispatch, chord
// hints, `?` help, footer ranks, the menu bar — consume what it holds. This type
// is not a seam where a second key router, palette, or view-specific copy could
// be introduced, because it declares no dispatch table of its own.
type Controller struct {
	// Registry is the descriptor authority: every operation, its bindings, and
	// the per-surface input strategies that project them.
	Registry *operation.OperationRegistry

	// Executor is the single invocation pipeline. It is built over the registry
	// after the functional options run, so an injected registry is the one it
	// wraps.
	Executor *operation.OperationExecutor

	// ActionSheet is the host-agnostic first level of the contextual action
	// flow. The host it was opened for travels on every message it emits.
	ActionSheet *listsurface.ContextActionSheetModel

	// ValuePicker and TextPrompt are the second level: exactly one of them is
	// mounted, chosen by whether the value provider takes typed text.
	ValuePicker *valuepicker.OperationValuePickerModel
	TextPrompt  *OperationTextPromptModel

	// Confirm is the prompt a destructive contextual action stops at, and
	// pendingRequest is the executor-supplied request it froze, so cancellation
	// returns to the sheet with nothing executed.
	Confirm        chrome.ConfirmDialog
	pendingRequest operation.OperationExecutionRequest

	// Chord is the keys already pressed toward an unfinished chord. A non-empty
	// prefix owns the footer as a which-key row until the chord completes or is
	// abandoned.
	Chord []string
}

// operationConfirmAction names the contextual destructive confirmation so the
// dialog can be told apart from every other prompt the shell raises.
const operationConfirmAction = "contextual-operation"

// SurfaceGate names one entity surface and, once the surface manifests its
// operations, the startup check that holds it to the one-path rule. The table
// is the shell's, because a surface validator reads manifests declared above
// this package.
type SurfaceGate struct {
	Strategy operation.InputStrategy
	Validate func(operation.InputStrategy) error
}

// NamedRegistrar is one entry of the fixed registrar table: the family name
// that identifies it if it fails, and the closure that declares it.
type NamedRegistrar struct {
	Name     string
	Register func() error
}

// NewController builds the cluster with an empty descriptor authority. The
// executor is deliberately not built here — see BindExecutor.
func NewController() Controller {
	return Controller{Registry: operation.NewOperationRegistry()}
}

// BindExecutor wraps whichever registry the model ended up with. It is called
// after the functional options run so an injected registry is honored: an
// executor built at construction time would wrap the one the options replaced.
func (c *Controller) BindExecutor() {
	c.Executor = operation.NewOperationExecutor(c.Registry)
}

// Wired reports whether both halves are present. A model assembled without them
// answers no key rather than panicking on the first press.
func (c *Controller) Wired() bool {
	return c != nil && c.Registry != nil && c.Executor != nil
}

// Register declares one operation across both registries: the registry takes
// the handler-free descriptor and the executor takes the binding that makes it
// runnable. Keeping the pair in one call is what stops a descriptor from
// existing with nothing behind it.
func (c *Controller) Register(
	descriptor operation.OperationDescriptor,
	binding operation.OperationBinding,
) error {
	if !c.Wired() {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := c.Registry.Register(descriptor); err != nil {
		return err
	}
	binding.OperationID = descriptor.ID
	return c.Executor.Register(binding)
}

// RegisterCatalog runs the registrar table in the order it was handed, then the
// two startup checks that read the catalog it produced: the surface strategies
// first, because the menu bar resolves against a registered strategy, then the
// menu specs. All three hard-fail boot — a family that would not register, a
// surface that drifted off the one-path rule, and a menu naming an operation
// nobody registered each stop construction rather than opening the app with a
// hole in it.
func (c *Controller) RegisterCatalog(registrars []NamedRegistrar, gates []SurfaceGate) error {
	for _, registration := range registrars {
		if err := registration.Register(); err != nil {
			return fmt.Errorf("register %s operations: %w", registration.Name, err)
		}
	}
	if err := c.registerSurfaceInputStrategies(gates); err != nil {
		return err
	}
	// The menu bar is a projection of the catalog above, so it is held to the
	// catalog here: a menu naming an operation nobody registered fails startup
	// rather than opening with a row missing.
	return c.validateMenuBarSpecs()
}

// registerSurfaceInputStrategies holds every manifested surface to the one-path
// rule and then installs its grammar. A surface that declares no bindings yet
// names only its identity, so it passes through without a strategy.
func (c *Controller) registerSurfaceInputStrategies(gates []SurfaceGate) error {
	for _, gate := range gates {
		if gate.Validate != nil {
			if err := gate.Validate(gate.Strategy); err != nil {
				return fmt.Errorf("validate direct and menu placement for %s: %w", gate.Strategy.ID, err)
			}
		}
		if len(gate.Strategy.Bindings) == 0 {
			continue
		}
		if err := c.Registry.RegisterInputStrategy(gate.Strategy); err != nil {
			return fmt.Errorf("register %s input strategy: %w", gate.Strategy.ID, err)
		}
	}
	return nil
}

// OverlayMounted reports whether any level of the contextual action flow is on
// screen. It doubles as the has-active-text-input answer because every level of
// it takes typed input — the sheet and the picker filter, the prompt composes —
// so the shell suppresses its global single-key shortcuts for all three alike.
func (c *Controller) OverlayMounted() bool {
	return c.ActionSheet != nil || c.ValuePicker != nil || c.TextPrompt != nil
}

// ClearValueStep dismisses whichever value overlay is mounted. Both are cleared
// because exactly one can be, and naming the survivor would be a second place
// that has to know which.
func (c *Controller) ClearValueStep() {
	c.ValuePicker = nil
	c.TextPrompt = nil
}

// ClearOverlays dismisses the whole flow and releases the frozen request, so no
// level of it outlives the route it was opened over.
func (c *Controller) ClearOverlays() {
	c.ActionSheet = nil
	c.ClearValueStep()
	c.Confirm.Cancel()
	c.pendingRequest = operation.OperationExecutionRequest{}
}

// ArmConfirm freezes the executor-supplied request — which already carries
// Confirmed — and opens the prompt describing it.
func (c *Controller) ArmConfirm(request operation.OperationExecutionRequest, prompt string) {
	c.pendingRequest = request
	c.Confirm.Show(operationConfirmAction, prompt)
}

// TakeRequest hands the frozen request back and releases it in the same step,
// so a resumed action can never be confirmed twice.
func (c *Controller) TakeRequest() operation.OperationExecutionRequest {
	request := c.pendingRequest
	c.pendingRequest = operation.OperationExecutionRequest{}
	return request
}

// ResolveConfirmKey answers one key press against the contextual confirmation.
// Canceling releases the frozen request; the sheet beneath it is left exactly
// as it was.
func (c *Controller) ResolveConfirmKey(key string) chrome.ConfirmAnswer {
	answer := chrome.ResolveConfirmKey(&c.Confirm, key)
	if answer == chrome.ConfirmAnswerCanceled {
		c.pendingRequest = operation.OperationExecutionRequest{}
	}
	return answer
}

// ConfirmOverlay renders the contextual confirmation when it is armed.
func (c *Controller) ConfirmOverlay(width int) (string, bool) {
	if c.Confirm.IsActive() {
		return c.Confirm.RenderOverlay(width), true
	}
	return "", false
}
