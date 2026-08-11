package tui

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// This file makes scenarios executable in-process. It drives the AppModel one
// action at a time (key/text/resize/wait/assert), draining the tea.Cmd values
// each action produces with a bounded budget so a blocking command (e.g. a
// tea.Tick status timeout) cannot hang the review harness. Commands that do not
// complete within the timeout are recorded as pending async work and can be
// drained later by a "wait" action.

const (
	// defaultDrainTimeout bounds how long a single command may run before it is
	// abandoned to the pending-async set and draining moves on.
	defaultDrainTimeout = 25 * time.Millisecond
	// maxDrainSteps bounds how many commands a single action may drain, guarding
	// against a command that keeps producing follow-up commands forever.
	maxDrainSteps = 64
	// defaultWaitTimeout bounds how long a "wait" action blocks while resolving
	// previously abandoned commands.
	defaultWaitTimeout = 2 * time.Second
)

// drainOptions tunes the bounded command drain. The zero value is not valid;
// use defaultDrainOptions or override individual fields.
type drainOptions struct {
	perCmdTimeout time.Duration
	maxSteps      int
}

func defaultDrainOptions() drainOptions {
	return drainOptions{perCmdTimeout: defaultDrainTimeout, maxSteps: maxDrainSteps}
}

// drainOpts returns the driver's drain budget, honoring an overridden
// per-command timeout when one was set via SetDrainTimeout.
func (d *UITestDriver) drainOpts() drainOptions {
	opts := defaultDrainOptions()
	if d.drainTimeout > 0 {
		opts.perCmdTimeout = d.drainTimeout
	}
	return opts
}

// UITestActionError is the structured failure returned when an action cannot be
// applied. It carries the action's index within the scenario, the action
// payload itself, and the latest observation so a caller can see the screen at
// the point of failure.
type UITestActionError struct {
	Index  int          `json:"index"`
	Action UITestAction `json:"action"`
	// Label mirrors Action.Label so a caller can name the failing user-level
	// step without reaching into the action payload. Empty when the action had
	// no label.
	Label       string            `json:"label,omitempty"`
	Reason      string            `json:"reason"`
	Observation UITestObservation `json:"observation"`
}

func (e *UITestActionError) Error() string {
	if e.Label != "" {
		return fmt.Sprintf("action %d (%s) %q: %s", e.Index, e.Action.Type, e.Label, e.Reason)
	}
	return fmt.Sprintf("action %d (%s): %s", e.Index, e.Action.Type, e.Reason)
}

// RunScenario applies each action in order, capturing an observation after every
// step. It stops at the first failing action and returns the observations
// gathered so far alongside a *UITestActionError describing the failure.
func (d *UITestDriver) RunScenario(sc UITestScenario, opts ObserveOptions) ([]UITestObservation, error) {
	observations := make([]UITestObservation, 0, len(sc.Actions))
	for i, action := range sc.Actions {
		if err := d.runAction(action); err != nil {
			return observations, &UITestActionError{
				Index:       i,
				Action:      action,
				Label:       action.Label,
				Reason:      err.Error(),
				Observation: d.Observe(opts),
			}
		}
		observations = append(observations, d.Observe(opts))
	}
	return observations, nil
}

// ApplyAction runs a single action against the driver, preserving all driver
// state across calls, and returns the resulting observation. On failure it
// returns a *UITestActionError carrying the failure-point observation, mirroring
// RunScenario's error contract. It is the per-request entry point the
// interactive `tt tui-test session` mode drives, one stdin line at a time.
func (d *UITestDriver) ApplyAction(a UITestAction, opts ObserveOptions) (UITestObservation, error) {
	if err := d.runAction(a); err != nil {
		obs := d.Observe(opts)
		return obs, &UITestActionError{
			Action:      a,
			Label:       a.Label,
			Reason:      err.Error(),
			Observation: obs,
		}
	}
	return d.Observe(opts), nil
}

// runAction applies a single action, draining any commands it produces.
func (d *UITestDriver) runAction(a UITestAction) error {
	switch a.Type {
	case UITestActionKey:
		if err := d.SendKey(a.Key); err != nil {
			return err
		}
		d.drain()
	case UITestActionText:
		for _, r := range a.Text {
			d.dispatch(tea.KeyPressMsg{Code: r, Text: string(r)})
			d.drain()
		}
	case UITestActionResize:
		d.Resize(a.Width, a.Height)
		d.drain()
	case UITestActionWait:
		d.Wait(defaultWaitTimeout)
	case UITestActionAssert:
		return d.runAssert(a)
	case UITestActionCommand:
		return d.runCommand(a.Command)
	default:
		return fmt.Errorf("unknown action type %q", a.Type)
	}
	return nil
}

// runCommand resolves a registered operation by stable ID or case-insensitive
// label and routes its result through the full app. Discovery remains restricted
// to the production input strategy, but an explicit scenario command may invoke
// any applicable registered operation so lower-level layout tests can reach
// stable setup routes. These setup-only operations are never returned in
// AvailableCommands.
func (d *UITestDriver) runCommand(name string) error {
	registry := d.app.operationRegistry
	if registry == nil || d.app.operationExecutor == nil {
		return fmt.Errorf("command %q: no operation registry is wired", name)
	}
	view := d.app.currentView()
	descriptors := d.availableOperationDescriptors()
	var match *OperationDescriptor
	find := func(candidates []OperationDescriptor) *OperationDescriptor {
		for i := range candidates {
			if strings.EqualFold(string(candidates[i].ID), name) ||
				strings.EqualFold(candidates[i].Label, name) {
				return &candidates[i]
			}
		}
		return nil
	}
	match = find(descriptors)
	if match == nil {
		ctx := d.app.operationCommandContext()
		ctx.View = view
		ctx.Capabilities.Add(CapabilityHarness)
		match = find(registry.Discover(ctx))
	}
	if match == nil {
		return fmt.Errorf("command %q is not available in view %q", name, view.String())
	}

	ctx := d.app.operationCommandContext()
	ctx.View = view
	ctx.Capabilities.Add(CapabilityHarness)
	result := d.app.operationExecutor.Execute(OperationExecutionRequest{
		ID:      match.ID,
		Context: ctx,
	})
	if result.Err != nil {
		return fmt.Errorf("command %q failed: %w", name, result.Err)
	}
	msg := result.AsMessage()
	switch m := msg.(type) {
	case nil:
		d.pendingCmd = nil
	case OperationCommandResultMsg:
		// The handler deferred its real work into a tea.Cmd; drain that instead
		// of routing the wrapper as a message.
		d.pendingCmd = m.Cmd
	default:
		// Route the result through the full app update path (global/search/view
		// handlers), keeping the driver's app pointer in sync.
		d.dispatch(msg)
	}
	d.drain()
	return nil
}

// runAssert evaluates an assertion against a fresh observation.
func (d *UITestDriver) runAssert(a UITestAction) error {
	obs := d.Observe(ObserveOptions{})
	switch a.Assert {
	case "contains":
		if !strings.Contains(obs.PlainScreen, a.Value) {
			return fmt.Errorf("screen does not contain %q", a.Value)
		}
	case "not_contains":
		if strings.Contains(obs.PlainScreen, a.Value) {
			return fmt.Errorf("screen unexpectedly contains %q", a.Value)
		}
	case "count_contains":
		if got := strings.Count(obs.PlainScreen, a.Value); got != a.Count {
			return fmt.Errorf("screen contains %q %d time(s), want %d", a.Value, got, a.Count)
		}
	case "view":
		if obs.CurrentView != a.Value {
			return fmt.Errorf("current view is %q, want %q", obs.CurrentView, a.Value)
		}
	case "no_layout_issues":
		if len(obs.LayoutIssues) > 0 {
			return fmt.Errorf("found %d layout issue(s): %s", len(obs.LayoutIssues), summarizeIssues(obs.LayoutIssues))
		}
	case "status_contains":
		if !strings.Contains(obs.Status, a.Value) {
			return fmt.Errorf("status %q does not contain %q", obs.Status, a.Value)
		}
	case "":
		return fmt.Errorf("assert action missing an assertion kind")
	default:
		return fmt.Errorf("unknown assertion %q", a.Assert)
	}
	return nil
}

func summarizeIssues(issues []UITestIssue) string {
	kinds := make([]string, 0, len(issues))
	for _, iss := range issues {
		kinds = append(kinds, iss.Kind)
	}
	return strings.Join(kinds, ", ")
}

// drain processes the command recorded by the most recent dispatch with the
// default bounded budget.
func (d *UITestDriver) drain() {
	cmd := d.pendingCmd
	d.pendingCmd = nil
	if cmd == nil {
		return
	}
	d.processCmds([]tea.Cmd{cmd}, d.drainOpts())
}

// processCmds walks a queue of commands breadth-first. Each command runs with a
// per-command timeout; a BatchMsg expands into its child commands, any other
// message is fed back through AppModel.Update and its follow-up command queued.
// Commands that time out are abandoned to pendingFutures for a later "wait".
func (d *UITestDriver) processCmds(queue []tea.Cmd, opts drainOptions) {
	steps := 0
	for len(queue) > 0 && steps < opts.maxSteps {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		steps++

		ch := runCmd(cmd)
		select {
		case msg := <-ch:
			queue = append(queue, d.handleMsg(msg)...)
		case <-time.After(opts.perCmdTimeout):
			// The command is still running. Keep its buffered channel so a
			// later Wait can resolve it, and move on.
			d.pendingFutures = append(d.pendingFutures, ch)
		}
	}
}

// handleMsg turns a drained message into any follow-up commands. A BatchMsg or a
// tea.Sequence message is unpacked into its child commands (nested groups are
// handled by re-queuing); every other message is dispatched through Update, whose
// returned command is queued for further draining.
func (d *UITestDriver) handleMsg(msg tea.Msg) []tea.Cmd {
	switch m := msg.(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		return []tea.Cmd(m)
	default:
		if cmds, ok := commandGroupMessage(msg); ok {
			return cmds
		}
		model, cmd := d.app.Update(m)
		if app, ok := model.(*AppModel); ok {
			d.app = app
		}
		if cmd == nil {
			return nil
		}
		return []tea.Cmd{cmd}
	}
}

// commandGroupMessage unpacks a message that is really a list of commands to run.
// tea.Sequence emits an unexported message type, so the only way to recognize it
// from outside the package is structurally: a slice whose elements are tea.Cmd.
// Draining the children in order matches what the real runtime does closely
// enough for scenarios, which never depend on a sequence step blocking.
func commandGroupMessage(msg tea.Msg) ([]tea.Cmd, bool) {
	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Slice {
		return nil, false
	}
	if value.Type().Elem() != reflect.TypeOf(tea.Cmd(nil)) {
		return nil, false
	}
	cmds := make([]tea.Cmd, 0, value.Len())
	for index := range value.Len() {
		cmd, ok := value.Index(index).Interface().(tea.Cmd)
		if !ok {
			return nil, false
		}
		cmds = append(cmds, cmd)
	}
	return cmds, true
}

// Wait resolves previously abandoned commands, blocking up to the given total
// timeout across all of them. Commands that resolve have their messages drained
// normally; commands still running when the budget is exhausted remain pending.
func (d *UITestDriver) Wait(timeout time.Duration) {
	futures := d.pendingFutures
	d.pendingFutures = nil
	if len(futures) == 0 {
		return
	}

	var followups []tea.Cmd
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for len(futures) > 0 {
		cases := make([]reflect.SelectCase, 0, len(futures)+1)
		for _, ch := range futures {
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)})
		}
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(timer.C)})
		chosen, value, ok := reflect.Select(cases)
		if chosen == len(futures) {
			break
		}
		if ok {
			followups = append(followups, d.handleMsg(value.Interface())...)
		}
		futures = append(futures[:chosen], futures[chosen+1:]...)
	}
	d.pendingFutures = append(d.pendingFutures, futures...)
	if len(followups) > 0 {
		d.processCmds(followups, d.drainOpts())
	}
}

// runCmd starts a command in its own goroutine and returns a buffered channel
// that will receive the command's message. The buffer of one means an abandoned
// (timed-out) command can still complete without leaking a blocked goroutine.
func runCmd(cmd tea.Cmd) <-chan tea.Msg {
	ch := make(chan tea.Msg, 1)
	go func() {
		ch <- cmd()
	}()
	return ch
}
