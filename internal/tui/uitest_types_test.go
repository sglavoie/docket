package tui

// This file defines the shared data contract for the LLM-driven TUI review
// harness (`tt tui-test`, wired up in later sessions). The types here are pure
// data structures with JSON tags so scenarios and observations can round-trip
// through files and stdout. Session 1 establishes the foundation: the driver,
// screen normalization, and key parsing. Later sessions extend these types
// (command/assert actions, semantic snapshots, available commands, etc.).

// UITestActionType enumerates the kinds of actions a scenario can drive.
type UITestActionType string

const (
	// UITestActionKey sends a single parsed key press (see parseUITestKey).
	UITestActionKey UITestActionType = "key"
	// UITestActionText types a literal string as a sequence of key presses.
	UITestActionText UITestActionType = "text"
	// UITestActionResize changes the simulated terminal size.
	UITestActionResize UITestActionType = "resize"
	// UITestActionWait drains pending async work (implemented in session 2).
	UITestActionWait UITestActionType = "wait"
	// UITestActionCommand runs a registered command by ID (implemented later).
	UITestActionCommand UITestActionType = "command"
	// UITestActionRestart rebuilds the app from durable state alone, the way
	// quitting and relaunching `tt` does. Nothing held in memory survives it, so
	// anything the app reopens with must have been persisted.
	UITestActionRestart UITestActionType = "restart"
	// UITestActionSwitchMachine points machine-local state at another machine's
	// home and then restarts against the same workspace directory, which is what
	// opening a synced workspace on a second machine looks like. Only what the
	// portable workspace carries survives it. The optional Machine field names
	// which machine; without it the switch always lands somewhere new.
	UITestActionSwitchMachine UITestActionType = "switch_machine"
	// UITestActionAssert checks an observation invariant (implemented later).
	UITestActionAssert UITestActionType = "assert"
)

// UITestAction is a single step in a scenario. Only the fields relevant to the
// action's Type are populated; the rest stay at their zero values. Later
// sessions add command-specific fields.
type UITestAction struct {
	Type UITestActionType `json:"type"`
	// Label is an optional user-level name for this step. It is echoed back in
	// UITestActionError so a failure can name the step an agent authored (e.g.
	// "open task detail") instead of only the raw action type/index.
	Label  string `json:"label,omitempty"`
	Key    string `json:"key,omitempty"`
	Text   string `json:"text,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`

	// Machine names the machine identity UITestActionSwitchMachine moves to. It
	// is optional: an omitted name keeps the historical behavior of landing on a
	// home no machine has ever used. A name is durable within the scenario, so
	// switching back to one returns to that machine's own machine-local state
	// rather than to an empty home — which is the only way to express "machine A,
	// then machine B, then back to A".
	Machine string `json:"machine,omitempty"`

	// Command is the registered command name for UITestActionCommand. It is
	// matched case-insensitively against the current view's command set
	// (available_commands[].name); aliases and shortcuts are not accepted.
	Command string `json:"command,omitempty"`

	// Assert names the invariant to check for UITestActionAssert. Supported
	// kinds: "contains", "not_contains", "count_contains", "view",
	// "no_layout_issues", "status_contains".
	Assert string `json:"assert,omitempty"`
	// Value is the argument for the assertion (a substring for contains/
	// not_contains/count_contains/status_contains, a view name for "view"). It
	// is ignored by "no_layout_issues".
	Value string `json:"value,omitempty"`
	// Count is the exact number of substring occurrences expected by
	// "count_contains". It is ignored by every other assertion kind.
	Count int `json:"count,omitempty"`
}

// UITestScenario is an ordered list of actions to drive against the app. The
// optional Fixture and Size fields let a scenario file be self-contained: they
// name the built-in fixture and the initial terminal size to build the driver
// with. Callers fall back to their own defaults when these are empty (see
// RunScenarioFixture / ResolveSize).
type UITestScenario struct {
	Name    string         `json:"name,omitempty"`
	Fixture string         `json:"fixture,omitempty"`
	Size    string         `json:"size,omitempty"`
	Actions []UITestAction `json:"actions,omitempty"`
}

// UITestIssue describes a single normalization or layout problem detected while
// rendering the screen (invalid UTF-8, a line wider than the terminal, more
// rendered lines than the terminal height, etc.).
type UITestIssue struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	// Line is the 1-based line number the issue refers to, or 0 when the issue
	// is screen-wide.
	Line int `json:"line,omitempty"`
}

// UITestCommand is a compact description of a command the active view exposes,
// surfaced in observations so a review host can discover the available actions
// without scraping help text. It is derived from the operation registry for the
// current view (see UITestDriver.availableCommands).
type UITestCommand struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Shortcut string   `json:"shortcut,omitempty"`
	Category string   `json:"category,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
}

// TUITestInspectable is implemented by view models that expose a compact,
// stable semantic snapshot of their state for the `tt tui-test` review harness.
// Snapshots are intentionally small: high-signal fields (counts, cursor,
// selected-item title, active mode/tab/filter) rather than whole
// task/project structs. The returned map is JSON-marshaled directly into an
// observation, so keys must stay stable across renders.
type TUITestInspectable interface {
	UITestSnapshot() map[string]any
}

// UITestObservation is the normalized snapshot returned after driving the app.
// Session 1 populates the basic fields; session 7 adds the semantic snapshot and
// available commands from the active view.
type UITestObservation struct {
	CurrentView  string        `json:"current_view"`
	ViewStack    []string      `json:"view_stack"`
	Width        int           `json:"width"`
	Height       int           `json:"height"`
	PlainScreen  string        `json:"plain_screen"`
	ANSIScreen   string        `json:"ansi_screen,omitempty"`
	Status       string        `json:"status,omitempty"`
	StatusIsInfo bool          `json:"status_is_info,omitempty"`
	Error        string        `json:"error,omitempty"`
	LayoutIssues []UITestIssue `json:"layout_issues,omitempty"`
	// PendingAsync counts commands that did not complete within the drain
	// timeout and are still outstanding (drained later by a "wait" action).
	PendingAsync int `json:"pending_async,omitempty"`
	// Snapshot is a compact semantic view of the active view's state when that
	// view implements TUITestInspectable; nil (omitted) otherwise.
	Snapshot map[string]any `json:"snapshot,omitempty"`
	// AvailableCommands lists the commands the active view exposes, sorted by
	// name for stable output.
	AvailableCommands []UITestCommand `json:"available_commands,omitempty"`
}

// ObserveOptions controls what an observation includes.
type ObserveOptions struct {
	// IncludeANSI includes the raw (styled) screen alongside the plain screen.
	IncludeANSI bool
}
