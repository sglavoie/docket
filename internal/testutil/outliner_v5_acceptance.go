package testutil

// CaptureSigilAcceptance describes one canonical quick-capture token. Form is
// the complete advertised shape (for example, "!1-5"), not merely its prefix.
type CaptureSigilAcceptance struct {
	Form        string
	Field       string
	ValuePolicy string
}

// PersistenceDomainAcceptance describes one of the deliberately small v5
// configuration domains. Project Markdown, notes, and attachment binaries are
// workspace content rather than additional configuration domains.
type PersistenceDomainAcceptance struct {
	Name     string
	Location string
	Contents string
}

// OutlinerV5Contract is the machine-checkable destination for the outliner-only
// simplification. Production intentionally does not consume this fixture:
// migration sessions compare their own surfaces to it as they cut over.
type OutlinerV5Contract struct {
	SchemaVersion       int
	PrimaryPresentation string
	InitialMode         string
	InitialScope        string
	Modes               []string
	States              []string
	DirectGrammar       []KeyAcceptance
	FilterGrammar       []KeyAcceptance
	CaptureGrammar      []CaptureSigilAcceptance
	TaskMenuActions     []string
	PersistenceDomains  []PersistenceDomainAcceptance
	CLICommands         []string
	GlobalFlags         []string
	InteractionRules    []string
	RemovedCapabilities []string
}

// OutlinerV5TargetContract returns a fresh copy of the accepted v5 product
// contract. Stable IDs use lowercase kebab-case; user-facing key spellings and
// command paths retain the spelling advertised by the product contract.
func OutlinerV5TargetContract() OutlinerV5Contract {
	return OutlinerV5Contract{
		SchemaVersion:       5,
		PrimaryPresentation: "outliner",
		InitialMode:         "all",
		InitialScope:        "all-projects",
		Modes:               []string{"all", "inbox", "next", "backlog", "due"},
		States: []string{
			"inbox",
			"ready",
			"next",
			"in_progress",
			"waiting",
			"paused",
			"someday",
			"done",
			"canceled",
		},
		DirectGrammar: []KeyAcceptance{
			{Key: "Tab", Action: "cycle-mode-forward", Area: "mode"},
			{Key: "Shift+Tab", Action: "cycle-mode-backward", Area: "mode"},
			{Key: "j / Down", Action: "move-down", Area: "navigation"},
			{Key: "k / Up", Action: "move-up", Area: "navigation"},
			{Key: "h / Left", Action: "collapse", Area: "navigation"},
			{Key: "l / Right", Action: "expand", Area: "navigation"},
			{Key: "S", Action: "toggle-all-sections", Area: "navigation"},
			{Key: "Home", Action: "first-row", Area: "navigation"},
			{Key: "End", Action: "last-row", Area: "navigation"},
			{Key: "PageUp", Action: "page-up", Area: "navigation"},
			{Key: "PageDown", Action: "page-down", Area: "navigation"},
			{Key: "Enter", Action: "inspect", Area: "navigation"},
			{Key: "a", Action: "capture", Area: "edit"},
			{Key: "A", Action: "capture-child", Area: "edit"},
			{Key: "e", Action: "edit-title", Area: "edit"},
			{Key: "E", Action: "edit-description", Area: "edit"},
			{Key: "Ctrl+E", Action: "open-external-editor", Area: "edit"},
			{Key: "si", Action: "state-inbox", Area: "state-chord"},
			{Key: "sr", Action: "state-ready-backlog", Area: "state-chord"},
			{Key: "sn", Action: "toggle-next", Area: "state-chord"},
			{Key: "sg", Action: "state-in-progress", Area: "state-chord"},
			{Key: "sw", Action: "state-waiting", Area: "state-chord"},
			{Key: "sp", Action: "state-paused", Area: "state-chord"},
			{Key: "sm", Action: "state-someday", Area: "state-chord"},
			{Key: "sd", Action: "state-done", Area: "state-chord"},
			{Key: "sc", Action: "state-canceled", Area: "state-chord"},
			{Key: "st", Action: "toggle-today", Area: "state-chord"},
			{Key: "!0", Action: "clear-priority", Area: "metadata"},
			{Key: "!1-!5", Action: "set-priority", Area: "metadata"},
			{Key: "@", Action: "set-due-date", Area: "metadata"},
			{Key: "#", Action: "set-tags", Area: "metadata"},
			{Key: "z", Action: "set-snooze", Area: "metadata"},
			{Key: "P", Action: "toggle-pin", Area: "metadata"},
			{Key: "<", Action: "outdent", Area: "structure"},
			{Key: ">", Action: "indent", Area: "structure"},
			{Key: "J", Action: "reorder-down", Area: "structure"},
			{Key: "K", Action: "reorder-up", Area: "structure"},
			{Key: "y", Action: "copy-title", Area: "clipboard"},
			{Key: "Y", Action: "copy-description", Area: "clipboard"},
			{Key: "Ctrl+Y", Action: "copy-title-and-description", Area: "clipboard"},
			{Key: "v", Action: "toggle-selection", Area: "selection"},
			{Key: "V", Action: "select-visible", Area: "selection"},
			{Key: "Space", Action: "task-menu", Area: "task-menu"},
			{Key: "c", Action: "scope-hub", Area: "global"},
			{Key: "C", Action: "select-project", Area: "global"},
			{Key: "N", Action: "notes", Area: "global"},
			{Key: "T", Action: "toggle-theme", Area: "global"},
			{Key: "/", Action: "visible-search", Area: "global"},
			{Key: "?", Action: "help", Area: "global"},
			{Key: "u", Action: "undo", Area: "global"},
			{Key: "Ctrl+R", Action: "redo", Area: "global"},
			{Key: "Esc", Action: "unwind-or-clear", Area: "global"},
			{Key: "Ctrl+C", Action: "quit", Area: "global"},
		},
		FilterGrammar: []KeyAcceptance{
			{Key: "fp0", Action: "clear-priority-filter", Area: "filter"},
			{Key: "fp1-fp5", Action: "set-priority-threshold", Area: "filter"},
			{Key: "fs", Action: "pick-state-filter", Area: "filter"},
			{Key: "ft", Action: "pick-tag-filter", Area: "filter"},
			{Key: "fd", Action: "pick-due-filter", Area: "filter"},
			{Key: "fb", Action: "toggle-blocked-only", Area: "filter"},
			{Key: "fz", Action: "toggle-snoozed-only", Area: "filter"},
			{Key: "fx", Action: "clear-filters", Area: "filter"},
		},
		CaptureGrammar: []CaptureSigilAcceptance{
			{Form: "!1-5", Field: "priority", ValuePolicy: "integer from 1 through 5"},
			{Form: "#tag", Field: "tag", ValuePolicy: "normalized tag including namespaced tags"},
			{Form: "@date", Field: "due", ValuePolicy: "supported date expression normalized to a local date"},
			{Form: "~interval", Field: "recurrence", ValuePolicy: "daily weekly monthly or an integer day week or month interval"},
			{Form: "$state", Field: "state", ValuePolicy: "one canonical v5 workflow state"},
			{Form: "%project", Field: "project", ValuePolicy: "workspace project reference"},
			{Form: ">parent", Field: "parent", ValuePolicy: "parent resolved after project routing"},
		},
		TaskMenuActions: []string{
			"Recurrence",
			"Move Project",
			"Set Parent",
			"Dependencies",
			"Checklist",
			"Comments",
			"Decisions",
			"Attachments",
			"Delete",
		},
		PersistenceDomains: []PersistenceDomainAcceptance{
			{Name: "workspace", Location: "<data>/.tt/workspace.json", Contents: "lean project catalog and protected System Inbox ID"},
			{Name: "planning", Location: "<data>/.tt/planning.json", Contents: "Today entries and Next ordering only"},
			{Name: "machine", Location: "machine-local runtime/machine.json", Contents: "custom contexts and dark/light theme only"},
		},
		CLICommands: []string{
			"tt",
			"tt doctor",
			"tt conflict",
			"tt workspace",
			"tt completion",
			"tt project export",
			"tt help",
			"tt --version",
		},
		GlobalFlags: []string{"--data-dir", "--no-color"},
		InteractionRules: []string{
			"every run starts in All Projects and All without restored navigation filters search or selection",
			"search and filters persist across mode and scope changes during one run only",
			"frequent actions are available only through direct grammar",
			"occasional task actions are available only through Space",
			"no action appears in both direct grammar and the task menu",
			"help and footer hints derive from the same operation descriptors",
			"terminal and snoozed work are hidden by default",
			"bulk mutations are atomic and structural edit Peek relation and clipboard actions reject an active selection",
			"N opens All Notes across Global and every active project",
			"Notes v scopes s searches titles and bodies and m moves one note atomically",
			"Notes support open edit create rename move delete undo and redo",
			"archived projects and Ctrl+N remain outside Notes",
		},
		RemovedCapabilities: []string{
			"alternate primary presentations and command-only duplicates",
			"mode picker command palette Finder and recent destinations",
			"AI Ask action plans harness selection providers and consent",
			"workflow rules automation templates scoring and recommendations",
			"reviews analytics metrics roadmap alerts notifications and message log",
			"tutor onboarding and activity history",
			"related links standalone blockers park dim status icons tag catalogs and pinned tags",
			"archived-task storage and indexing",
			"project trash delete purge Sandbox and Internal projects",
			"Settings TUI custom themes and custom color editing",
			"note tags task links pinning bulk actions and previews",
			"task type fields and other retired v4 task metadata",
			"daily task project CRUD context note focus search scoring workflow template theme archive sandbox roadmap TUI-test undo redo and compatibility CLI commands",
		},
	}
}

// OutlinerV5Baseline is a point-in-time characterization of the pre-v5
// repository. It is historical data, not a second destination contract.
type OutlinerV5Baseline struct {
	RecordedAtCommit   string
	CleanInstall       []string
	States             []string
	Modes              []string
	TabCycle           []string
	DailyLoop          []string
	Navigation         []string
	PersistenceDomains []string
	Storage            []string
	CLICommands        []string
}

// OutlinerV5CurrentBaseline returns the behavior observed at the plan's
// grounding commit. Keeping this inventory separate from the destination lets
// later sessions demonstrate deliberate removals without tests pretending those
// removals already exist.
func OutlinerV5CurrentBaseline() OutlinerV5Baseline {
	return OutlinerV5Baseline{
		RecordedAtCommit: "549400c3",
		CleanInstall: []string{
			"domain bootstrap creates workspace planning settings and automation files plus machine-local state",
			"TUI startup ensures a visible System Inbox and a hidden Sandbox project",
			"portable settings default to the personal context",
			"machine state defaults the external editor to code",
		},
		States: []string{
			"inbox",
			"ready",
			"next",
			"in_progress",
			"waiting",
			"paused",
			"review",
			"someday",
			"done",
			"canceled",
		},
		Modes: []string{
			"all",
			"inbox",
			"next",
			"backlog",
			"due",
			"priority",
			"blocked",
			"parked",
			"daily-review",
			"activity",
		},
		TabCycle: []string{"all", "inbox", "next", "backlog"},
		DailyLoop: []string{
			"direct task loop is capture edit start done backlog today next and contextual actions",
			"Space opens a contextual action sheet with a broader operation catalog",
			"Review remains a canonical workflow state",
			"mode picker Finder weekly review tag manager and notifications remain discoverable surfaces",
		},
		Navigation: []string{
			"the cross-project Outliner is the sole root presentation",
			"startup reapplies the persisted machine StartupRoute including mode search and filters",
			"Tab and Shift+Tab cycle four work modes while the mode picker exposes six additional modes",
			"Ctrl+Esc resets child surfaces transient filters search and selection",
		},
		PersistenceDomains: []string{"workspace", "planning", "settings", "automation", "machine"},
		Storage: []string{
			"active task Markdown is stored per project",
			"completed archives and archive indexes are separate durable storage",
			"notes and attachment binaries are project content",
			"transactions undo ledgers and conflict bundles live below the workspace internal directory",
			"settings own behavior appearance alerts and AI consent while automation owns workflows templates and scoring",
		},
		CLICommands: []string{
			"add",
			"ask",
			"attach",
			"attachment",
			"attachments",
			"blocker",
			"completion",
			"conflict",
			"context",
			"dash",
			"doctor",
			"epic",
			"focus",
			"help",
			"note",
			"project",
			"redo",
			"roadmap",
			"sandbox",
			"score",
			"search",
			"select",
			"tag",
			"task",
			"template",
			"theme",
			"undo",
			"workflow",
			"workspace",
		},
	}
}
