package tui

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// --- Action runner -----------------------------------------------------------

func TestUITestActionResizeAndAssertView(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	sc := UITestScenario{Actions: []UITestAction{
		{Type: UITestActionResize, Width: 80, Height: 24},
		{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"},
	}}

	obs, err := driver.RunScenario(sc, ObserveOptions{})
	if err != nil {
		t.Fatalf("RunScenario error: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("got %d observations, want 2", len(obs))
	}
	if obs[0].Width != 80 || obs[0].Height != 24 {
		t.Fatalf("after resize got %dx%d, want 80x24", obs[0].Width, obs[0].Height)
	}
}

func TestUITestActionInvalidKeyReturnsStructuredError(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	sc := UITestScenario{Actions: []UITestAction{
		{Type: UITestActionResize, Width: 80, Height: 24},
		{Type: UITestActionKey, Key: "ctrl+"}, // invalid
	}}

	obs, err := driver.RunScenario(sc, ObserveOptions{})
	if err == nil {
		t.Fatal("expected error for invalid key, got nil")
	}
	// One observation captured before the failing action.
	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1", len(obs))
	}

	var actErr *UITestActionError
	if !errors.As(err, &actErr) {
		t.Fatalf("error is not *UITestActionError: %T", err)
	}
	if actErr.Index != 1 {
		t.Errorf("error index = %d, want 1", actErr.Index)
	}
	if actErr.Action.Key != "ctrl+" {
		t.Errorf("error action key = %q, want ctrl+", actErr.Action.Key)
	}
	if actErr.Observation.CurrentView == "" {
		t.Error("error observation should carry the latest screen state")
	}
}

func TestUITestActionUnknownType(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)
	sc := UITestScenario{Actions: []UITestAction{{Type: "bogus"}}}

	if _, err := driver.RunScenario(sc, ObserveOptions{}); err == nil {
		t.Fatal("expected error for unknown action type, got nil")
	}
}

func TestUITestActionTextTypesLiteralRunes(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)
	// Text actions are literal input, not key syntax; "+" must type a plus
	// instead of being parsed as a modifier separator.
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionText, Text: "ab+"}}}

	if _, err := driver.RunScenario(sc, ObserveOptions{}); err != nil {
		t.Fatalf("RunScenario error: %v", err)
	}
}

func TestUITestScenarioCommandPaletteIgnoresRemovedExplorerShortcut(t *testing.T) {
	sc := UITestScenario{
		Name:    "command palette ignores removed explorer shortcut",
		Fixture: FixtureRich,
		Size:    "100x30",
		Actions: []UITestAction{
			{Type: UITestActionKey, Key: ":"},
			{Type: UITestActionKey, Key: `ctrl+\`},
			{Type: UITestActionAssert, Assert: "view", Value: "CommandPalette"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("removed explorer shortcut scenario should pass, got error: %+v", result.Error)
	}
}

// --- Command drain -----------------------------------------------------------

// immediateCmd returns a command that produces an ignorable message.
func immediateMsgCmd(counter *int32) tea.Cmd {
	return func() tea.Msg {
		atomic.AddInt32(counter, 1)
		return uitestNoopMsg{}
	}
}

// uitestNoopMsg is a message type AppModel.Update ignores, so draining it
// produces no follow-up commands. It lets drain tests count executions without
// depending on real app behavior.
type uitestNoopMsg struct{}

func TestUITestCommandDrainRunsImmediateCommands(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	var ran int32
	driver.pendingCmd = tea.Batch(
		immediateMsgCmd(&ran),
		immediateMsgCmd(&ran),
		immediateMsgCmd(&ran),
	)
	driver.drain()

	if got := atomic.LoadInt32(&ran); got != 3 {
		t.Fatalf("ran %d commands, want 3", got)
	}
	if n := len(driver.pendingFutures); n != 0 {
		t.Fatalf("pendingFutures = %d, want 0", n)
	}
}

func TestUITestCommandDrainNestedBatches(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	var ran int32
	inner := tea.Batch(immediateMsgCmd(&ran), immediateMsgCmd(&ran))
	// A command whose message is itself a BatchMsg of more commands.
	nested := func() tea.Msg { return inner() }
	driver.pendingCmd = tea.Batch(immediateMsgCmd(&ran), nested)
	driver.drain()

	if got := atomic.LoadInt32(&ran); got != 3 {
		t.Fatalf("ran %d commands, want 3", got)
	}
}

func TestUITestCommandDrainTimeoutMarksPending(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	blocking := func() tea.Msg {
		time.Sleep(200 * time.Millisecond)
		return uitestNoopMsg{}
	}
	driver.processCmds([]tea.Cmd{blocking}, drainOptions{perCmdTimeout: 10 * time.Millisecond, maxSteps: maxDrainSteps})

	if n := len(driver.pendingFutures); n != 1 {
		t.Fatalf("pendingFutures = %d, want 1", n)
	}
	if obs := driver.Observe(ObserveOptions{}); obs.PendingAsync != 1 {
		t.Fatalf("PendingAsync = %d, want 1", obs.PendingAsync)
	}
}

func TestUITestCommandDrainWaitResolvesPending(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	var ran int32
	slow := func() tea.Msg {
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&ran, 1)
		return uitestNoopMsg{}
	}
	// 10ms timeout is shorter than the 30ms command, so it is abandoned.
	driver.processCmds([]tea.Cmd{slow}, drainOptions{perCmdTimeout: 10 * time.Millisecond, maxSteps: maxDrainSteps})
	if len(driver.pendingFutures) != 1 {
		t.Fatalf("expected 1 pending future before wait, got %d", len(driver.pendingFutures))
	}

	driver.Wait(time.Second)
	if len(driver.pendingFutures) != 0 {
		t.Fatalf("pendingFutures after wait = %d, want 0", len(driver.pendingFutures))
	}
	if got := atomic.LoadInt32(&ran); got != 1 {
		t.Fatalf("command ran %d times, want 1", got)
	}
}

func TestUITestCommandDrainRespectsMaxSteps(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	var ran int32
	// A command that always re-schedules itself would loop forever without the
	// step bound. handleMsg only re-queues via Update, which ignores the noop
	// msg, so instead build a batch larger than maxSteps and cap the steps low.
	cmds := make([]tea.Cmd, 10)
	for i := range cmds {
		cmds[i] = immediateMsgCmd(&ran)
	}
	driver.processCmds(cmds, drainOptions{perCmdTimeout: defaultDrainTimeout, maxSteps: 4})

	if got := atomic.LoadInt32(&ran); got != 4 {
		t.Fatalf("ran %d commands, want 4 (capped by maxSteps)", got)
	}
}

// --- Command actions ---------------------------------------------------------

// newUITestAppWithCommand builds a driver test app with a single registered
// command. Views scopes the command; pass nil/empty for a global command.
func newCommandTestApp(t *testing.T, cmd Command) *AppModel {
	t.Helper()
	app := newUITestApp()
	app.commandRegistry = NewCommandRegistry()
	if err := app.commandRegistry.Register(cmd); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return app
}

func TestUITestActionCommandOpensView(t *testing.T) {
	app := newCommandTestApp(t, Command{
		Name:     "Open Search",
		Category: CategorySearch,
		Handler:  func() tea.Msg { return ShowCommandPaletteMsg{} },
	})
	driver := NewUITestDriver(app, 100, 30)

	// Case-insensitive match against available_commands[].name; no palette UI.
	sc := UITestScenario{Actions: []UITestAction{
		{Type: UITestActionCommand, Command: "open search"},
		{Type: UITestActionAssert, Assert: "view", Value: "CommandPalette"},
	}}
	if _, err := driver.RunScenario(sc, ObserveOptions{}); err != nil {
		t.Fatalf("RunScenario error: %v", err)
	}
}

func TestUITestActionCommandUnknownFails(t *testing.T) {
	app := newCommandTestApp(t, Command{
		Name:    "Open Search",
		Handler: func() tea.Msg { return ShowCommandPaletteMsg{} },
	})
	driver := NewUITestDriver(app, 100, 30)

	err := driver.runCommand("no such command")
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
	if driver.App().currentView() != ViewDashboard {
		t.Fatalf("view changed on unknown command: %s", driver.App().currentView())
	}
}

func TestUITestActionCommandViewScopedRejected(t *testing.T) {
	// Command is only available in the outliner, not the dashboard.
	app := newCommandTestApp(t, Command{
		Name:    "Outliner Only",
		Views:   []PresentationID{ViewOutliner},
		Handler: func() tea.Msg { return ShowCommandPaletteMsg{} },
	})
	driver := NewUITestDriver(app, 100, 30)

	if err := driver.runCommand("Outliner Only"); err == nil {
		t.Fatal("expected view-scoped command to be rejected on the dashboard")
	}
}

func TestUITestActionCommandNilRegistry(t *testing.T) {
	app := newUITestApp()
	app.commandRegistry = nil
	driver := NewUITestDriver(app, 100, 30)

	if err := driver.runCommand("anything"); err == nil {
		t.Fatal("expected error when no command registry is wired")
	}
}

func TestUITestActionCommandUnwrapsPaletteResult(t *testing.T) {
	var ran int32
	app := newCommandTestApp(t, Command{
		Name: "Deferred Work",
		Handler: func() tea.Msg {
			return PaletteCommandResultMsg{Cmd: func() tea.Msg {
				atomic.AddInt32(&ran, 1)
				return uitestNoopMsg{}
			}}
		},
	})
	driver := NewUITestDriver(app, 100, 30)

	if err := driver.runCommand("Deferred Work"); err != nil {
		t.Fatalf("runCommand error: %v", err)
	}
	// The carried tea.Cmd must have been drained, not routed as a message.
	if got := atomic.LoadInt32(&ran); got != 1 {
		t.Fatalf("carried command ran %d times, want 1", got)
	}
}

// --- Assertions --------------------------------------------------------------

func TestUITestAssertContainsAndNotContains(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)
	screen := driver.Render()
	if screen == "" {
		t.Fatal("empty screen; cannot pick a substring to assert on")
	}

	// The dashboard renders "Today"/"Dashboard"-ish content; assert on a token
	// guaranteed present by observing first.
	obs := driver.Observe(ObserveOptions{})
	present := firstWord(obs.PlainScreen)
	if present == "" {
		t.Fatal("could not find a non-empty token on screen")
	}

	if err := driver.runAssert(UITestAction{Assert: "contains", Value: present}); err != nil {
		t.Fatalf("contains %q failed: %v", present, err)
	}
	if err := driver.runAssert(UITestAction{Assert: "contains", Value: "definitely-absent"}); err == nil {
		t.Fatal("contains of absent string should fail")
	}
	if err := driver.runAssert(UITestAction{Assert: "not_contains", Value: "definitely-absent"}); err != nil {
		t.Fatalf("not_contains of absent string should pass: %v", err)
	}
	if err := driver.runAssert(UITestAction{Assert: "not_contains", Value: present}); err == nil {
		t.Fatal("not_contains of present string should fail")
	}
}

func TestUITestAssertView(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	if err := driver.runAssert(UITestAction{Assert: "view", Value: "Dashboard"}); err != nil {
		t.Fatalf("view=Dashboard should pass: %v", err)
	}
	if err := driver.runAssert(UITestAction{Assert: "view", Value: "Nope"}); err == nil {
		t.Fatal("view=Nope should fail")
	}
}

func TestUITestAssertStatusContains(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)
	driver.App().statusMessage = "task saved"

	if err := driver.runAssert(UITestAction{Assert: "status_contains", Value: "saved"}); err != nil {
		t.Fatalf("status_contains=saved should pass: %v", err)
	}
	if err := driver.runAssert(UITestAction{Assert: "status_contains", Value: "deleted"}); err == nil {
		t.Fatal("status_contains=deleted should fail")
	}
}

func TestUITestAssertNoLayoutIssues(t *testing.T) {
	// A well-sized terminal renders without overflow issues.
	driver := NewUITestDriver(newUITestApp(), 120, 40)
	if err := driver.runAssert(UITestAction{Assert: "no_layout_issues"}); err != nil {
		t.Fatalf("no_layout_issues should pass at 120x40: %v", err)
	}
}

func TestUITestAssertUnknownKind(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)
	if err := driver.runAssert(UITestAction{Assert: "bogus"}); err == nil {
		t.Fatal("unknown assertion kind should fail")
	}
	if err := driver.runAssert(UITestAction{Assert: ""}); err == nil {
		t.Fatal("missing assertion kind should fail")
	}
}

// firstWord returns the first whitespace-delimited token in s, or "".
func firstWord(s string) string {
	start := -1
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			if start >= 0 {
				return s[start:i]
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		return s[start:]
	}
	return ""
}
