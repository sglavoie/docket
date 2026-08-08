package tui

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"
)

// UITestDriver drives an AppModel directly (bypassing the tea.Program event
// loop) so an LLM-driven review harness can resize the terminal, send keys, and
// read normalized observations of unexported app state.
//
// Session 1 establishes the minimal foundation: wrap an existing AppModel,
// apply a window size, render, and parse/send key presses. The fixture and
// isolated-workspace construction (session 3), the action runner and command
// drain (session 2), and the CLI surface (session 5+) build on top of this.
type UITestDriver struct {
	app *AppModel

	// pendingCmd holds the tea.Cmd returned by the most recent dispatch. The
	// action runner (drain) processes it; direct Resize/SendKey calls only
	// record it so nothing is silently dropped.
	pendingCmd tea.Cmd

	// pendingFutures holds the result channels of commands that did not complete
	// within the drain timeout. A "wait" action re-visits these; Observe reports
	// their count as PendingAsync.
	pendingFutures []<-chan tea.Msg

	// drainTimeout overrides the per-command drain budget when positive. Zero
	// falls back to defaultDrainTimeout. The CLI wires this from --drain-timeout.
	drainTimeout time.Duration
}

// SetDrainTimeout sets the per-command drain budget. A non-positive value
// restores the default for scenario tests.
func (d *UITestDriver) SetDrainTimeout(timeout time.Duration) {
	d.drainTimeout = timeout
}

// NewUITestDriver wraps an AppModel (typically built via NewAppModel, or by the
// fixture helpers added in session 3) and applies an initial terminal size.
func NewUITestDriver(app *AppModel, width, height int) *UITestDriver {
	if _, registered := app.operationRegistry.Descriptor(operationHarnessNewProject); !registered {
		if err := app.registerHarnessOperations(); err != nil {
			panic("register TUI test operations: " + err.Error())
		}
	}
	d := &UITestDriver{app: app}
	d.Resize(width, height)
	return d
}

// App returns the underlying AppModel for direct inspection.
func (d *UITestDriver) App() *AppModel {
	return d.app
}

// Resize simulates a terminal resize by dispatching a tea.WindowSizeMsg.
func (d *UITestDriver) Resize(width, height int) {
	d.dispatch(tea.WindowSizeMsg{Width: width, Height: height})
}

// SendKey parses a human-readable key name (see parseUITestKey) and dispatches
// it as a key press. It returns an error only when the key name is invalid.
func (d *UITestDriver) SendKey(name string) error {
	key, err := parseUITestKey(name)
	if err != nil {
		return err
	}
	d.dispatch(key)
	return nil
}

// dispatch feeds a message through AppModel.Update, keeping the driver's app
// pointer in sync with the returned model and recording the resulting command.
func (d *UITestDriver) dispatch(msg tea.Msg) {
	model, cmd := d.app.Update(msg)
	if app, ok := model.(*AppModel); ok {
		d.app = app
	}
	d.pendingCmd = cmd
}

// Render returns the current plain (ANSI-stripped) screen.
func (d *UITestDriver) Render() string {
	plain, _ := normalizeScreen(d.app.View().Content, d.app.width, d.app.height)
	return plain
}

// Observe renders the current screen and returns a normalized observation of
// the app's state.
func (d *UITestDriver) Observe(opts ObserveOptions) UITestObservation {
	// Render inside a render audit so any render-time required-text truncation
	// (footer/help/chrome clipping) is captured. normalizeScreen only sees
	// geometry, which stays valid after such clips, so these issues are appended
	// separately below.
	var rawContent string
	rawContent, auditIssues := withRenderAudit(func() string {
		return d.app.View().Content
	})
	plain, issues := normalizeScreen(rawContent, d.app.width, d.app.height)
	issues = append(issues, auditIssues...)

	obs := UITestObservation{
		CurrentView:  d.app.currentView().String(),
		ViewStack:    d.viewStackNames(),
		Width:        d.app.width,
		Height:       d.app.height,
		PlainScreen:  plain,
		Status:       d.app.statusMessage,
		StatusIsInfo: d.app.statusIsInfo,
		LayoutIssues: issues,
		PendingAsync: len(d.pendingFutures),
	}
	if d.app.err != nil {
		obs.Error = d.app.err.Error()
	}
	if opts.IncludeANSI {
		obs.ANSIScreen = rawContent
	}

	// Semantic snapshot from the active view when it supports inspection.
	if view := d.app.views.Get(d.app.currentView()); view != nil {
		if inspectable, ok := view.(TUITestInspectable); ok {
			if snap := inspectable.UITestSnapshot(); len(snap) > 0 {
				obs.Snapshot = snap
			}
		}
	}
	obs.AvailableCommands = d.availableCommands()

	return obs
}

// availableCommands returns the operations the current view exposes, derived
// from the authoritative operation registry and sorted for stable output. It returns nil
// when no registry is wired or no commands apply to the current view.
func (d *UITestDriver) availableCommands() []UITestCommand {
	descriptors := d.availableOperationDescriptors()
	if len(descriptors) == 0 {
		return nil
	}
	operations := d.app.operationRegistry
	result := make([]UITestCommand, 0, len(descriptors))
	for _, descriptor := range descriptors {
		shortcut := operations.InputShortcutForOperation(InputStrategySimplified, descriptor.ID)
		result = append(result, UITestCommand{
			ID:       string(descriptor.ID),
			Name:     descriptor.Label,
			Shortcut: shortcut,
			Category: operationCategoryLabel(descriptor.Category),
			Aliases:  append([]string(nil), descriptor.Aliases...),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func (d *UITestDriver) availableOperationDescriptors() []OperationDescriptor {
	if d == nil || d.app == nil || d.app.operationRegistry == nil {
		return nil
	}
	ctx := d.app.operationCommandContext()
	ctx.View = d.app.currentView()
	return d.app.operationRegistry.InputStrategyOperations(InputStrategySimplified, ctx)
}

// viewStackNames returns the human-readable names of the current view stack,
// bottom to top. It always returns a non-nil slice for stable JSON output.
func (d *UITestDriver) viewStackNames() []string {
	path := d.app.presentationPath()
	names := make([]string, 0, len(path))
	for _, vt := range path {
		names = append(names, vt.String())
	}
	return names
}
