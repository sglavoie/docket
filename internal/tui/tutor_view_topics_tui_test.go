package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestTutorBulkOperationsTopicDocumentsSelectedTaskFieldActions(t *testing.T) {
	topic := findTutorTUITopic(t, "Bulk Operations")

	required := []string{
		"selected tasks instead of only the cursor task",
		"!1-!5  Set priority",
		"Set, clear, or pick due dates",
		"Set, clear, or pick snooze dates",
		"Workflow-state",
		"Move to section",
		"Replace tags",
		"cross-project views",
		"cross-project section moves are not supported",
		"Kanban multi-select applies supported task-field mutations to the",
		"selected cards",
		"Bulk delete is not supported in Kanban",
		"cursor task",
		"Kanban views",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Bulk Operations topic missing %q:\n%s", want, topic.Content)
		}
	}

	stale := "Multi-select is available in the outliner, project tree, and\nall-tasks views."
	if strings.Contains(topic.Content, stale) {
		t.Fatalf("Bulk Operations topic still contains stale view scope text")
	}
}

func TestTutorOutlinerDocumentsProjectViewToggle(t *testing.T) {
	topic := findTutorTUITopic(t, "Outliner")
	for _, want := range []string{
		"Ctrl+p toggles the current project's tree and outliner.",
		"cross-project outliner, Ctrl+p first opens the highlighted project's\ntree.",
	} {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Outliner topic missing %q:\n%s", want, topic.Content)
		}
	}
}

func TestRetiredTaskStageTutorAndSettingsSurfacesStayRemoved(t *testing.T) {
	forbiddenContent := []string{
		"task stage",
		"--stage",
		"stages tab",
		"setting stages",
		"set or clear stage",
		"filter by stage",
		"configured stages",
		"workflow stage",
		"s1-s9",
	}
	for _, topic := range tutorTopics() {
		title := strings.ToLower(topic.Title)
		if title == "stages" || title == "statuses" {
			t.Fatalf("tutor still exposes retired topic %q", topic.Title)
		}
		content := strings.ToLower(topic.Content)
		for _, forbidden := range forbiddenContent {
			if strings.Contains(content, forbidden) {
				t.Fatalf("tutor topic %q still advertises %q:\n%s", topic.Title, forbidden, topic.Content)
			}
		}
	}

	for _, name := range settingsTabNames {
		if name == "Stages" {
			t.Fatal("settings registry still exposes the retired Stages tab")
		}
	}
	if len(settingsTabNames) != len(tabFieldCount) || len(settingsTabNames) != len(settingsTabSections) {
		t.Fatalf("settings registries are misaligned: names=%d counts=%d sections=%d",
			len(settingsTabNames), len(tabFieldCount), len(settingsTabSections))
	}
	if tabAppearance != len(settingsTabNames)-1 {
		t.Fatalf("Appearance tab index = %d, want compacted final index %d", tabAppearance, len(settingsTabNames)-1)
	}

	workflow := findTutorTopicByTitle(t, "Workflow States")
	for _, state := range models.AllWorkflowStates() {
		if !strings.Contains(workflow.Content, state.String()) {
			t.Errorf("Workflow States topic omits canonical state %q", state)
		}
	}
	for _, mapping := range []string{",s  in_progress", ",d  done", ",b  waiting", ",o  inbox", ",x  canceled", ",r  review", ",S  paused", ",m  someday"} {
		if !strings.Contains(workflow.Content, mapping) {
			t.Errorf("Workflow States topic omits comma-prefix mapping %q", mapping)
		}
	}

	pipeline := findTutorTopicByTitle(t, "The Task Pipeline")
	for _, allowed := range []string{"six pipeline stages", "Capture", "Triage", "Plan", "Work", "Review", "Support"} {
		if !strings.Contains(pipeline.Content, allowed) {
			t.Errorf("pipeline topic lost legitimate pipeline-stage wording %q", allowed)
		}
	}
}

func TestTutorDistinguishesActiveSectionFromFocusQueue(t *testing.T) {
	cli := findTutorTopicByTitle(t, "Focus")
	for _, want := range []string{
		"next in the\npipeline",
		"does not mark\nthe task Active",
		"leaves existing queue items alone",
	} {
		if !strings.Contains(cli.Content, want) {
			t.Errorf("CLI Focus topic missing %q:\n%s", want, cli.Content)
		}
	}

	concepts := findTutorTopicByTitle(t, "Focus & Suggestions")
	for _, want := range []string{
		"separate from the Active section",
		"q to\ntoggle queue membership",
		"without changing existing queue items",
	} {
		if !strings.Contains(concepts.Content, want) {
			t.Errorf("Focus & Suggestions topic missing %q:\n%s", want, concepts.Content)
		}
	}

	planning := findTutorTopicByTitle(t, "Sections (GTD)")
	for _, want := range []string{
		"# Active And Queue",
		"independent, ordered planning list",
		"Queue\nand Today are mutually exclusive",
	} {
		if !strings.Contains(planning.Content, want) {
			t.Errorf("Sections topic missing %q:\n%s", want, planning.Content)
		}
	}
}

func TestTutorCLIExamplesUseSupportedSectionAndStateSyntax(t *testing.T) {
	quickCapture := findTutorTopicByTitle(t, "Quick Capture")
	for _, want := range []string{
		"The tt add command accepts inline modifiers.",
		`tt add "Fix login bug !1 #backend"`,
		`tt add "Research caching #perf"`,
		"# Sections in TUI Quick Capture",
		"This sigil is not parsed by the tt add command.",
		"Fix login bug !1 #backend /inbox",
		"Research caching #perf /next",
		"Deploy service #ops #urgent /review",
		`tt task add "Deploy service" --tag ops --tag urgent --section Next`,
	} {
		if !strings.Contains(quickCapture.Content, want) {
			t.Errorf("Quick Capture topic missing supported syntax %q:\n%s", want, quickCapture.Content)
		}
	}
	for _, stale := range []string{
		"section assignment with @",
		"@active",
		"@next",
		`tt add "Fix login bug !1 #backend /`,
		`tt add "Research caching #perf /`,
		`tt add "Deploy service #ops #urgent /`,
	} {
		if strings.Contains(quickCapture.Content, stale) {
			t.Errorf("Quick Capture topic still contains stale syntax %q:\n%s", stale, quickCapture.Content)
		}
	}

	taskManagement := findTutorTopicByTitle(t, "Task Management")
	for _, want := range []string{"tt task state ready <id>", "tt task state next <id>"} {
		if !strings.Contains(taskManagement.Content, want) {
			t.Errorf("Task Management topic missing supported command %q:\n%s", want, taskManagement.Content)
		}
	}
	if strings.Contains(taskManagement.Content, "tt task section") {
		t.Errorf("Task Management topic still contains retired task-section command:\n%s", taskManagement.Content)
	}
}

func TestTutorUndoTopicDocumentsRetainedUndoWithoutHistoryViewer(t *testing.T) {
	topic := findTutorTUITopic(t, "Undo")

	required := []string{
		"# Undo",
		"Press u to restore the previous task state",
		"Press ctrl+r to redo",
		"tt undo",
		"tt redo",
		"undo_history_depth",
		"restore is aborted instead of partially applied",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Undo topic missing %q:\n%s", want, topic.Content)
		}
	}

	stale := []string{
		"# Undo " + "History",
		"Ctrl+" + "Z",
		"ctrl+" + "z",
		"⌃" + "Z",
		"Undo / Redo " + "History",
		"Undo / Redo " + "Stack",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Undo topic contains stale viewer wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func findTutorTUITopic(t *testing.T, title string) TutorTopic {
	t.Helper()

	for _, topic := range tutorTUITopics() {
		if topic.Title == title {
			return topic
		}
	}
	t.Fatalf("tutor TUI topic %q not found", title)
	return TutorTopic{}
}

func findTutorTopicByTitle(t *testing.T, title string) TutorTopic {
	t.Helper()

	for _, topic := range tutorTopics() {
		if topic.Title == title {
			return topic
		}
	}
	t.Fatalf("tutor topic %q not found in tutorTopics()", title)
	return TutorTopic{}
}

func TestTutorFilteringTopicsDocumentParkedFilter(t *testing.T) {
	tuiTopic := findTutorTUITopic(t, "Filtering")
	tuiRequired := []string{
		"Fk  Filter to parked tasks",
		"Filter by score threshold",
		"Saved filters shown",
		"share the capped tab row",
		`Parked is built in`,
		"the seven built-in All Tasks tabs fill the",
		"visible cap",
		"the first pinned saved-filter tab can put By Score",
		"and saved-filter tabs behind More…",
	}
	for _, want := range tuiRequired {
		if !strings.Contains(tuiTopic.Content, want) {
			t.Fatalf("TUI Filtering topic missing %q:\n%s", want, tuiTopic.Content)
		}
	}

	cliTopic := findTutorTopicByTitle(t, "Search")
	cliRequired := []string{
		"parked state",
		"tt task list --parked",
	}
	for _, want := range cliRequired {
		if !strings.Contains(cliTopic.Content, want) {
			t.Fatalf("CLI Search topic missing %q:\n%s", want, cliTopic.Content)
		}
	}
}

func TestTutorQuickCaptureTopicTeachesCalmPlaceholderAndSyntaxGate(t *testing.T) {
	topic := findTutorTUITopic(t, "Quick Capture")

	required := []string{
		";i from anywhere",
		`"Type a task title…"`,
		"?: syntax",
		"Task",
		"Route",
		"Schedule",
		"Metadata",
		"Aliases",
		"Linked Task Creator",
		"linked tasks always belong to the\nparent's project",
		// The `?` reference now names the grouped syntax guide from session 2.
		"Priority & Tags",
		"Placement",
		"Project Routing",
		// Session 5: the fixed composer, scrollable syntax pane, scroll keys,
		// neutral heading, footer states, and Esc precedence must be taught.
		"Fixed Composer & Scrollable Syntax",
		`"Capture Syntax"`,
		"Ctrl+J/K",
		"PgUp/PgDn",
		"Footer States",
		"? close syntax",
		"Esc close syntax",
		"dismisses autocomplete",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Quick Capture topic missing %q:\n%s", want, topic.Content)
		}
	}

	// Stale strings: the old sigil-laden placeholder, the always-visible chrome
	// strip, the old "?: help" footer hint, the old "(via:" alias suffix, and the
	// pre-redesign "overlay" wording (Quick Capture is now a full-width view)
	// must never reappear in tutor copy.
	stale := []string{
		"^type Task title !urgent",
		"^type · !N/word",
		"?: help",
		"(via:",
		"ctrl+i",
		"overlay",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Quick Capture topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestTutorQuickCaptureGrammarTopicDocumentsGroupTaxonomy(t *testing.T) {
	topic := findTutorTUITopic(t, "Quick Capture Grammar")

	required := []string{
		"Press ;i from any view",
		`"Type a task title…"`,
		"?: syntax",
		"Task group",
		"Route group",
		"Schedule group",
		"Metadata group",
		"Aliases row",
		"Linked Task Creator",
		"omitted in global capture",
		// The opening section now names the grouped syntax guide from session 2.
		"Priority & Tags",
		"Placement",
		"Project Routing",
		// Session 5: neutral heading, fixed composer, and independent scroll keys.
		"Scrollable Syntax Reference",
		`"Capture Syntax"`,
		"Ctrl+J/K",
		"PgUp/PgDn",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Quick Capture Grammar topic missing %q:\n%s", want, topic.Content)
		}
	}

	stale := []string{
		"?: help",
		"(via:",
		"^type Task title !urgent",
		"ctrl+i",
		// Quick Capture is a full-width view now, not an overlay.
		"overlay",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Quick Capture Grammar topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestTutorInboxProcessingTopicTeachesTriageVocabulary(t *testing.T) {
	topic := findTutorTUITopic(t, "Inbox Processing")

	required := []string{
		"Where should this task go next?",
		"n   Skip task",
		"e   Edit task",
		"D   Done",
		"T   Tags",
		"u   Undo",
		"?   Toggle help",
		"m* Route",
		"d* Schedule",
		"z* Snooze",
		",* Status",
		"!/1-5 Priority",
		"c* Linked",
		"t* task ops",
		"w Today",
		"skip task",
		"mark task done",
		"edit inbox task",
		"manage inbox task tags",
		"undo last triage action",
		"plan today",
		"route to section",
		"send to waiting",
		"set inbox task due date",
		"create subtask from inbox",
		"manage inbox dependencies",
		"delete inbox task",
		"triage inbox",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Inbox Processing topic missing %q:\n%s", want, topic.Content)
		}
	}

	// The session-5 redesign removed these from the visible chrome; tutor copy
	// must not re-document them as primary triage actions.
	stale := []string{
		"Operations\n", // legacy panel title (Operations as a header line)
		"ctrl+s: Section",
		"ctrl+w: Waiting",
		"j/k: Navigate",
		"gg/G: First/last",
		",s/d/b/x/o",
		"undo inbox action", // old palette name (renamed to "undo last triage action")
		"assign section",    // demoted to alias of "route to section"
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Inbox Processing topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}

	// The due-date triage palette command moved from `D` (which is now Done)
	// to the chord `dd`. Tutor must never re-document `D` as the due-date key.
	if strings.Contains(topic.Content, "set inbox task due date    (D)") {
		t.Fatalf("Inbox Processing topic must not bind D to due date; it is reserved for Done:\n%s", topic.Content)
	}
}

func TestTutorHelpTopicTeachesIntentGroupingAndFullReference(t *testing.T) {
	topic := findTutorTUITopic(t, "Help")

	required := []string{
		"?",
		":",
		`ctrl+\`,
		"full reference",
		"groups items by user intent",
		"Triage",
		"Route & Schedule",
		"Resolve",
		"Refine",
		"Jump to View",
		"keybinding explorer",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Help topic missing %q:\n%s", want, topic.Content)
		}
	}

	// Per session-8 notes: tutor must never say first-level help is grouped by
	// prefix — that is only true of the keybinding explorer (ctrl+\).
	stale := []string{
		"grouped by prefix",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Help topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestTutorCommandPaletteTopicSurfacesTriageVocabulary(t *testing.T) {
	topic := findTutorTUITopic(t, "Command Palette")

	required := []string{
		`ctrl+\`,
		"full reference",
		"triage inbox",
		"skip task",
		"mark task done",
		"undo last triage action",
		"set inbox task due date",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Command Palette topic missing %q:\n%s", want, topic.Content)
		}
	}
}

func TestTutorDashboardTopicTeachesTriageProminenceAndEntryPoints(t *testing.T) {
	topic := findTutorTUITopic(t, "Dashboard at a glance")

	required := []string{
		"📥 N to triage",
		"Triage",
		"i        Quick Capture",
		"O        Open inbox triage",
		"ctrl+i   Global Inbox Processor",
		"T on the dashboard still opens View Trash",
		// Session 12: dashboard topic must teach the cockpit-first mental model.
		"tt opens on the Today cockpit by default",
		"Next Action",
		"Pipeline Hub",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Dashboard at a glance topic missing %q:\n%s", want, topic.Content)
		}
	}

	// Stale wording from earlier dashboard chrome iterations must not reappear.
	// "Inbox backlog:" is the legacy per-project line label (session 7).
	// "Organize(" is the legacy pipeline stage label (session 10 renamed it to Triage).
	// "Inbox hotspots:" is the muted alerts row removed in session 9 — the tutor
	// must not re-teach removed UI affordances.
	stale := []string{
		"Inbox backlog:",
		"Organize(",
		"Inbox hotspots:",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Dashboard at a glance topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestTutorGettingStartedPipelineTopicTeachesSixStageFlow(t *testing.T) {
	topic := findTutorTopicByTitle(t, "The pipeline")

	required := []string{
		"six-stage flow",
		"Capture → Triage → Plan → Work → Review → Support",
		";i      Capture",
		"ctrl+i  Triage",
		";a      Triage",
		";q      Plan",
		";w      Work",
		";r      Review",
		";B      Support",
		"Pipeline Hub",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("The pipeline topic missing %q:\n%s", want, topic.Content)
		}
	}

	// Pre-session-10 vocabulary must not survive in onboarding copy.
	stale := []string{
		"five-stage",
		"Organize",
		";f      Work",
		"ctrl+i  Capture",
		"Capture → Organize",
		"Focus Board (kanban)",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("The pipeline topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestTutorGettingStartedDailyRhythmTeachesTodayDefaultAndHub(t *testing.T) {
	topic := findTutorTopicByTitle(t, "Daily rhythm")

	required := []string{
		"tt opens on Today by default",
		"Next Action",
		";w returns to Today from anywhere",
		"Pipeline Hub",
		";;",
		"Capture / Triage / Plan / Work / Review / Support",
		"📥 N to triage",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Daily rhythm topic missing %q:\n%s", want, topic.Content)
		}
	}

	// The Focus Board moniker and ;f shortcut were merged into Today (;w) and
	// no longer exist in the chrome — onboarding must not advertise them.
	stale := []string{
		"stay in Focus Board",
		";f opens",
		"Focus Board (;f)",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Daily rhythm topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestTutorGettingStartedFirstTaskTeachesTodayAndHubLanding(t *testing.T) {
	topic := findTutorTopicByTitle(t, "Your first task")

	required := []string{
		"tt opens on the Today cockpit by default",
		"Next Action",
		"Open Today (;w)",
		";; to open the Pipeline Hub",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Your first task topic missing %q:\n%s", want, topic.Content)
		}
	}

	stale := []string{
		"Open Focus Board (;f)",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Your first task topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestTutorPipelineViewsTopicTeachesSixColumnHub(t *testing.T) {
	topic := findTutorTUITopic(t, "Pipeline Views")

	required := []string{
		"Capture, Triage, Plan, Work,\nReview, Support",
		"six-column overview",
		"Next Action",
		";; hub",
		": cmd",
		"? help",
		"Triage:   ctrl+i inbox, ;a all tasks",
		"Plan:     ;q backlog",
		"Work:     ;w today",
		"Support:  ;B blockers",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Pipeline Views topic missing %q:\n%s", want, topic.Content)
		}
	}

	stale := []string{
		"five-column",
		"Organize",
		"Execute:",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Pipeline Views topic still contains stale wording %q:\n%s", notWant, topic.Content)
		}
	}
}

func TestNoTutorTopicReferencesRemovedFocusBoardShortcut(t *testing.T) {
	// Session 12 (and the redesign more broadly) folded the standalone Focus
	// Board view into the Today cockpit at ;w. The ;f shortcut no longer
	// exists. Tutor copy must not advertise it anywhere — a single regression
	// here would resurrect the wrong mental model for new users.
	for _, topic := range tutorTopics() {
		if strings.Contains(topic.Content, ";f ") || strings.Contains(topic.Content, "(;f)") {
			t.Fatalf("tutor topic %q still references removed ;f shortcut:\n%s", topic.Title, topic.Content)
		}
	}
}

func TestTutorGettingStartedFirstTaskTeachesTriageEntryPoints(t *testing.T) {
	topic := findTutorTopicByTitle(t, "Your first task")

	required := []string{
		`"Type a task title…"`,
		"?: syntax",
		"Where should this task go next?",
		"O        Triage the selected project's inbox",
		"ctrl+i   Triage every project's inbox",
		"📥 N to triage",
		"Triage",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Your first task topic missing %q:\n%s", want, topic.Content)
		}
	}
}
