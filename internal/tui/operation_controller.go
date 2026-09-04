package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// operationController is the shell's operation-wiring cluster: the descriptor
// authority, the invocation pipeline, and the two-level overlay flow one
// invocation can open on its way to an answer — the contextual action sheet, the
// value step (a picker, or a typed prompt when the provider says so), and the
// confirmation a destructive action stops at with its frozen request. The keys
// already pressed toward an unfinished chord live here too, because a prefix is
// only meaningful against the registry that declared it.
//
// It owns the wiring, not the operations. Every descriptor is still declared by
// one of the fifteen concrete registrars on AppModel, and every handler is still
// a bound AppModel method traveling as an OperationBinding value, because the
// runtime domains, the application service and the coordinators live there.
// Nothing here reaches storage.
//
// The two registries stay two registries and stay singular.
// OperationRegistry is the descriptor authority and OperationExecutor the single
// invocation pipeline: register once through this type and let the projections —
// direct dispatch, chord hints, `?` help, footer ranks, the menu bar — consume
// what it holds. This type is not a seam where a second key router, palette, or
// view-specific copy could be introduced, because it declares no dispatch table
// of its own.
type operationController struct {
	// registry is the descriptor authority: every operation, its bindings, and
	// the per-surface input strategies that project them.
	registry *operation.OperationRegistry

	// executor is the single invocation pipeline. It is built over the registry
	// after the functional options run, so an injected registry is the one it
	// wraps.
	executor *operation.OperationExecutor

	// actionSheet is the host-agnostic first level of the contextual action
	// flow. The host it was opened for travels on every message it emits.
	actionSheet *listsurface.ContextActionSheetModel

	// valuePicker and textPrompt are the second level: exactly one of them is
	// mounted, chosen by whether the value provider takes typed text.
	valuePicker *OperationValuePickerModel
	textPrompt  *OperationTextPromptModel

	// confirm is the prompt a destructive contextual action stops at, and
	// pendingRequest is the executor-supplied request it froze, so cancellation
	// returns to the sheet with nothing executed.
	confirm        chrome.ConfirmDialog
	pendingRequest operation.OperationExecutionRequest

	// chord is the keys already pressed toward an unfinished chord. A non-empty
	// prefix owns the footer as a which-key row until the chord completes or is
	// abandoned.
	chord []string
}

// operationConfirmAction names the contextual destructive confirmation so the
// dialog can be told apart from every other prompt the shell raises.
const operationConfirmAction = "contextual-operation"

// namedOperationRegistrar is one entry of the fixed registrar table: the family
// name that identifies it if it fails, and the closure that declares it.
type namedOperationRegistrar struct {
	name     string
	register func() error
}

// newOperationController builds the cluster with an empty descriptor authority.
// The executor is deliberately not built here — see bindExecutor.
func newOperationController() operationController {
	return operationController{registry: operation.NewOperationRegistry()}
}

// bindExecutor wraps whichever registry the model ended up with. It is called
// after the functional options run so an injected registry is honored: an
// executor built at construction time would wrap the one the options replaced.
func (c *operationController) bindExecutor() {
	c.executor = operation.NewOperationExecutor(c.registry)
}

// wired reports whether both halves are present. A model assembled without them
// answers no key rather than panicking on the first press.
func (c *operationController) wired() bool {
	return c != nil && c.registry != nil && c.executor != nil
}

// register declares one operation across both registries: the registry takes
// the handler-free descriptor and the executor takes the binding that makes it
// runnable. Keeping the pair in one call is what stops a descriptor from
// existing with nothing behind it.
func (c *operationController) register(
	descriptor operation.OperationDescriptor,
	binding operation.OperationBinding,
) error {
	if !c.wired() {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := c.registry.Register(descriptor); err != nil {
		return err
	}
	binding.OperationID = descriptor.ID
	return c.executor.Register(binding)
}

// registerCatalog runs the registrar table in the order it was handed, then the
// two startup checks that read the catalog it produced: the surface strategies
// first, because the menu bar resolves against a registered strategy, then the
// menu specs. All three hard-fail boot — a family that would not register, a
// surface that drifted off the one-path rule, and a menu naming an operation
// nobody registered each stop construction rather than opening the app with a
// hole in it.
func (c *operationController) registerCatalog(registrars []namedOperationRegistrar) error {
	for _, registration := range registrars {
		if err := registration.register(); err != nil {
			return fmt.Errorf("register %s operations: %w", registration.name, err)
		}
	}
	if err := c.registerSurfaceInputStrategies(); err != nil {
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
func (c *operationController) registerSurfaceInputStrategies() error {
	for _, gate := range surfaceOperationGates() {
		if gate.validate != nil {
			if err := gate.validate(gate.strategy); err != nil {
				return fmt.Errorf("validate direct and menu placement for %s: %w", gate.strategy.ID, err)
			}
		}
		if len(gate.strategy.Bindings) == 0 {
			continue
		}
		if err := c.registry.RegisterInputStrategy(gate.strategy); err != nil {
			return fmt.Errorf("register %s input strategy: %w", gate.strategy.ID, err)
		}
	}
	return nil
}

// overlayMounted reports whether any level of the contextual action flow is on
// screen. It doubles as the has-active-text-input answer because every level of
// it takes typed input — the sheet and the picker filter, the prompt composes —
// so the shell suppresses its global single-key shortcuts for all three alike.
func (c *operationController) overlayMounted() bool {
	return c.actionSheet != nil || c.valuePicker != nil || c.textPrompt != nil
}

// clearValueStep dismisses whichever value overlay is mounted. Both are cleared
// because exactly one can be, and naming the survivor would be a second place
// that has to know which.
func (c *operationController) clearValueStep() {
	c.valuePicker = nil
	c.textPrompt = nil
}

// clearOverlays dismisses the whole flow and releases the frozen request, so no
// level of it outlives the route it was opened over.
func (c *operationController) clearOverlays() {
	c.actionSheet = nil
	c.clearValueStep()
	c.confirm.Cancel()
	c.pendingRequest = operation.OperationExecutionRequest{}
}

// armConfirm freezes the executor-supplied request — which already carries
// Confirmed — and opens the prompt describing it.
func (c *operationController) armConfirm(request operation.OperationExecutionRequest, prompt string) {
	c.pendingRequest = request
	c.confirm.Show(operationConfirmAction, prompt)
}

// takeRequest hands the frozen request back and releases it in the same step,
// so a resumed action can never be confirmed twice.
func (c *operationController) takeRequest() operation.OperationExecutionRequest {
	request := c.pendingRequest
	c.pendingRequest = operation.OperationExecutionRequest{}
	return request
}

// resolveConfirmKey answers one key press against the contextual confirmation.
// Canceling releases the frozen request; the sheet beneath it is left exactly
// as it was.
func (c *operationController) resolveConfirmKey(key string) confirmAnswer {
	answer := resolveConfirmKey(&c.confirm, key)
	if answer == confirmAnswerCanceled {
		c.pendingRequest = operation.OperationExecutionRequest{}
	}
	return answer
}

// confirmOverlay renders the contextual confirmation when it is armed.
func (c *operationController) confirmOverlay(width int) (string, bool) {
	if c.confirm.IsActive() {
		return c.confirm.RenderOverlay(width), true
	}
	return "", false
}
