package tui

import (
	"errors"
	"fmt"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
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
	// falls back to defaultDrainTimeout.
	drainTimeout time.Duration

	// relaunch rebuilds an AppModel from the owning workspace's durable state
	// alone, which is what the "restart" action drives. It is nil for a driver
	// wrapping an AppModel nobody owns the workspace of, and such a driver
	// refuses to restart rather than silently doing something weaker.
	relaunch func() (*AppModel, error)

	// moveToFreshMachine repoints machine-local state at another machine's home,
	// which the "switch_machine" action drives before relaunching. An empty name
	// asks for a home no machine has used; a name asks for that machine's own
	// home, minted on first use and returned to afterwards. It is nil for a
	// driver wrapping an AppModel nobody owns the workspace of.
	moveToFreshMachine func(machine string) error

	// now is this driver's virtual clock and timers the ticks waiting on it. A
	// "wait" action advances the clock rather than sleeping; see
	// uitest_clock_helpers_test.go.
	now      time.Time
	timers   []uitestTimer
	timerSeq uint64

	// clipboard is the last text the app copied while a scenario held the
	// clipboard seam open, which the clipboard_contains assertion reads. It is
	// empty both before the first copy and outside a scenario run, since nothing
	// else installs the seam.
	clipboard string
}

// SetDrainTimeout sets the per-command drain budget. A non-positive value
// restores the default for scenario tests.
func (d *UITestDriver) SetDrainTimeout(timeout time.Duration) {
	d.drainTimeout = timeout
}

// NewUITestDriver wraps an AppModel (typically built via NewAppModel, or by the
// fixture helpers added in session 3) and applies an initial terminal size.
func NewUITestDriver(app *AppModel, width, height int) *UITestDriver {
	if _, registered := app.operations.Registry.Descriptor(operationHarnessNewProject); !registered {
		if err := app.registerHarnessOperations(); err != nil {
			panic("register TUI test operations: " + err.Error())
		}
	}
	d := &UITestDriver{app: app, now: time.Now()}
	d.Resize(width, height)
	return d
}

// Restart replaces the app with one built from durable state alone, keeping the
// terminal size and the virtual clock. Nothing else survives: the runtime state
// is re-read from disk, so a scenario asserting after a restart is asserting on
// what was actually persisted rather than on what a live session remembered.
// The launch message is dispatched and drained the way the first launch is, so
// the screen the next action observes is a settled root.
func (d *UITestDriver) Restart() error {
	if d.relaunch == nil {
		return errors.New("restart needs a workspace-owned driver")
	}
	width, height := d.app.viewNav.width, d.app.viewNav.height
	app, err := d.relaunch()
	if err != nil {
		return err
	}
	if _, registered := app.operations.Registry.Descriptor(operationHarnessNewProject); !registered {
		if err := app.registerHarnessOperations(); err != nil {
			return fmt.Errorf("register TUI test operations: %w", err)
		}
	}
	d.app = app
	d.pendingCmd = nil
	d.pendingFutures = nil
	d.Resize(width, height)
	if app.lifecycle.startup != nil {
		d.dispatch(app.lifecycle.startup)
		d.drain()
	}
	d.bootLaunchCommands()
	return nil
}

// SwitchMachine moves the workspace onto another machine and relaunches on it.
// The synced directory is untouched; only the machine-local home moves.
//
// An unnamed switch lands on an empty home, so the new app starts with no theme,
// no active-lens pointer, and no machine-local lens copy: whatever the scenario
// then still sees is what the portable workspace carried, which is the only
// thing a real second machine would have received. A named switch addresses a
// durable machine instead — returning to a name lands back on that machine's own
// state, which is what makes a round trip between two machines expressible.
func (d *UITestDriver) SwitchMachine(machine string) error {
	if d.moveToFreshMachine == nil {
		return errors.New("switch_machine needs a workspace-owned driver")
	}
	if err := d.moveToFreshMachine(machine); err != nil {
		return err
	}
	return d.Restart()
}

// bootLaunchCommands runs the launch commands the driver does not get from the
// startup message. AppModel.Init raises two beside it, and both matter here:
//
//   - the workspace-wide note read the search's notes tier ranks against, without
//     which a scenario would search a permanently empty index; and
//   - the periodic-save tick, which is the cadence the shell refreshes workspace
//     state on. It is armed against the driver's virtual clock rather than real
//     time, so it costs a scenario nothing until a wait action names a span long
//     enough to reach it — and naming that span is the only way a scenario can
//     observe something only the tick does, such as the outstanding-conflict
//     count another machine's sync wrote.
func (d *UITestDriver) bootLaunchCommands() {
	if cmd := d.app.requestNoteSearchIndexLoad(); cmd != nil {
		d.pendingCmd = cmd
		d.drain()
	}
	d.pendingCmd = status.TickCmd(periodicSaveInterval, func(at time.Time) tea.Msg {
		return status.PeriodicSaveMsg{At: at}
	})
	d.drain()
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
	plain, _ := normalizeScreen(d.app.View().Content, d.app.viewNav.width, d.app.viewNav.height)
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
	plain, issues := normalizeScreen(rawContent, d.app.viewNav.width, d.app.viewNav.height)
	issues = append(issues, auditIssues...)

	obs := UITestObservation{
		CurrentView:  d.app.currentView().String(),
		ViewStack:    d.viewStackNames(),
		Width:        d.app.viewNav.width,
		Height:       d.app.viewNav.height,
		PlainScreen:  plain,
		Status:       d.app.feedback.message,
		StatusIsInfo: d.app.feedback.info,
		LayoutIssues: issues,
		PendingAsync: len(d.pendingFutures),
	}
	if d.app.feedback.err != nil {
		obs.Error = d.app.feedback.err.Error()
	}
	if opts.IncludeANSI {
		obs.ANSIScreen = rawContent
	}

	// Semantic snapshot from the active view when it supports inspection.
	if view := d.app.viewNav.views.Get(d.app.currentView()); view != nil {
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
	operations := d.app.operations.Registry
	result := make([]UITestCommand, 0, len(descriptors))
	for _, descriptor := range descriptors {
		shortcut := operations.InputShortcutForOperation(operation.InputStrategySimplified, descriptor.ID)
		result = append(result, UITestCommand{
			ID:       string(descriptor.ID),
			Name:     descriptor.Label,
			Shortcut: shortcut,
			Category: operation.OperationCategoryLabel(descriptor.Category),
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

func (d *UITestDriver) availableOperationDescriptors() []operation.OperationDescriptor {
	if d == nil || d.app == nil || d.app.operations.Registry == nil {
		return nil
	}
	ctx := d.app.operationCommandContext()
	ctx.View = d.app.currentView()
	return d.app.operations.Registry.InputStrategyOperations(operation.InputStrategySimplified, ctx)
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

// newUITestApp builds a minimal, fully-renderable AppModel (cross-project
// Outliner registered) for driver tests, without the real-data side effects of
// NewAppModel.
func newUITestApp() *AppModel {
	app := newTestAppModel()
	app.operations.Registry = operation.NewOperationRegistry()
	app.operations.Executor = operation.NewOperationExecutor(app.operations.Registry)
	o := outliner.NewOutlinerModel(nil, app.domain.manager, app.domain.runtime)
	app.crossOutliner = o
	app.viewNav.views.Register(presentation.ViewCrossProjectOutliner, o)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
	if err := app.registerCoreInputOperations(); err != nil {
		panic(err)
	}
	if err := app.registerTaskActionOperations(); err != nil {
		panic(err)
	}
	if err := app.filters.RegisterFilterOperations(app); err != nil {
		panic(err)
	}
	if err := app.filters.RegisterSortOperations(app); err != nil {
		panic(err)
	}
	if err := app.registerProjectOperations(); err != nil {
		panic(err)
	}
	if err := app.registerProjectActionSheetOperation(); err != nil {
		panic(err)
	}
	if err := app.registerHarnessOperations(); err != nil {
		panic(err)
	}
	if err := app.registerChecklistRowOperations(); err != nil {
		panic(err)
	}
	if err := app.operations.Registry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		panic(err)
	}
	app.wireOutlinerOperationExecutor(app.crossOutliner)
	return app
}
