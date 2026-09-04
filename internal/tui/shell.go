package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// This file extracts three cohesive shell concerns out of the AppModel
// coordinator so that later feature-package migrations have a stable surface to
// build against instead of reaching into AppModel's fields directly:
//
//   - ShellState        — the view stack / current route / terminal geometry.
//   - WorkspaceSession   — the runtime domains, manager, and currently selected
//                          project the shell is operating within.
//   - OverlayManager     — the transient global overlays (pickers, quick-note)
//                          layered above the routed view stack.
//
// They are introduced as typed facades over the AppModel so the extraction is
// behavior-preserving: no state moves, but the shell now exposes these concerns
// through cohesive, testable APIs (and the overlay lifecycle is genuinely
// centralized here — see OverlayManager.HasActiveTextInput / ClearAll, which the
// AppModel now delegates to). Subsequent sessions migrate feature views onto
// these facades and the route factories.

// ShellContext is the shared UI contract a feature view uses to interrogate the
// shell without importing AppModel internals. It is deliberately read-only:
// mutations flow through messages, not back-references, which keeps feature
// packages free of import cycles on the shell.
type ShellContext interface {
	// CurrentRoute returns the route instance for the topmost view, bound to
	// the active project/task where the route's scope calls for it.
	CurrentRoute() Route
	// CurrentView returns the topmost view type.
	CurrentView() presentation.PresentationID
	// Size returns the current terminal width and height.
	Size() (width, height int)
}

// Compile-time assurance that AppModel satisfies the shared shell contract.
var _ ShellContext = (*AppModel)(nil)

// ShellState is a facade over the AppModel's navigation shell: its derived
// renderer path, current route, and terminal geometry.
type ShellState struct{ app *AppModel }

// Shell returns the shell-state facade for this model.
func (m *AppModel) Shell() ShellState { return ShellState{app: m} }

// CurrentView returns the topmost view type on the stack.
func (s ShellState) CurrentView() presentation.PresentationID { return s.app.currentView() }

// Stack returns renderer identities derived from the current route and overlay
// stacks (top is the last element).
func (s ShellState) Stack() []presentation.PresentationID { return s.app.presentationPath() }

// Depth returns the number of rendered route/overlay frames.
func (s ShellState) Depth() int { return len(s.app.presentationPath()) }

// AtRoot reports whether navigation is the bare Outliner root.
func (s ShellState) AtRoot() bool {
	return s.app.navigation.Primary.Depth() == 1 && !s.app.navigation.Overlays.Active()
}

// Size returns the current terminal dimensions.
func (s ShellState) Size() (int, int) { return s.app.width, s.app.height }

// CurrentRoute returns the route instance for the topmost view.
func (s ShellState) CurrentRoute() Route { return s.app.CurrentRoute() }

// CurrentRoute returns the live primary route. Overlays are skipped because
// they belong to the separate overlay stack.
func (m *AppModel) CurrentRoute() Route {
	return m.navigation.Primary.Current()
}

// CurrentView satisfies ShellContext.
func (m *AppModel) CurrentView() presentation.PresentationID { return m.currentView() }

// Size satisfies ShellContext.
func (m *AppModel) Size() (int, int) { return m.width, m.height }

// WorkspaceSession is a facade over the runtime domains, manager, and currently
// selected project the shell operates within.
type WorkspaceSession struct{ app *AppModel }

// Workspace returns the workspace-session facade for this model.
func (m *AppModel) Workspace() WorkspaceSession { return WorkspaceSession{app: m} }

// Runtime returns the startup-loaded configuration domains.
func (w WorkspaceSession) Runtime() *config.RuntimeState { return w.app.runtime }

// Manager returns the configuration manager.
func (w WorkspaceSession) Manager() *config.Manager { return w.app.configManager }

// CurrentProject returns the currently selected project, or nil.
func (w WorkspaceSession) CurrentProject() *models.Project { return w.app.currentProject }

// HasCurrentProject reports whether a project is currently selected.
func (w WorkspaceSession) HasCurrentProject() bool { return w.app.currentProject != nil }

// ProjectStack returns the stacked project ids for nested project trees.
func (w WorkspaceSession) ProjectStack() []string { return w.app.projectStack }

// ViewContext assembles the shared view construction context for the workspace,
// sized to the shell's current geometry for view construction.
func (w WorkspaceSession) ViewContext() ViewContext {
	return ViewContext{}
}

// OverlayManager centralizes transient overlays layered above the routed view
// stack. They float over the current route and are cleared together when
// navigation resets.
type OverlayManager struct{ app *AppModel }

// Overlays returns the overlay-manager facade for this model.
func (m *AppModel) Overlays() OverlayManager { return OverlayManager{app: m} }

// HasActiveTextInput reports whether any overlay is currently capturing text
// input, which the shell uses to suppress global single-key shortcuts.
func (o OverlayManager) HasActiveTextInput() bool {
	m := o.app
	return m.operationValuePicker != nil ||
		m.operationTextPrompt != nil ||
		m.contextActionSheet != nil ||
		m.lensPicker != nil
}

// Active reports whether any transient overlay is currently shown.
func (o OverlayManager) Active() bool {
	m := o.app
	return m.navigation.Overlays.Active() ||
		m.contextActionSheet != nil ||
		m.operationValuePicker != nil ||
		m.operationTextPrompt != nil ||
		m.lensPicker != nil
}

// ClearAll dismisses every transient overlay. Called when navigation collapses
// so no overlay outlives the route it was opened over.
func (o OverlayManager) ClearAll() {
	m := o.app
	m.navigation.Overlays.CancelAll()
	m.contextActionSheet = nil
	m.operationValuePicker = nil
	m.operationTextPrompt = nil
	m.lensPicker = nil
	m.operationConfirm.Cancel()
	m.pendingOperationRequest = operation.OperationExecutionRequest{}
}
