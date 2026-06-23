package tui

import (
	"strings"
	"testing"
)

func TestTutorBulkOperationsTopicDocumentsSelectedTaskFieldActions(t *testing.T) {
	topic := findTutorTUITopic(t, "Bulk Operations")

	required := []string{
		"selected tasks instead of only the cursor task",
		"!1-!5  Set priority",
		"Set, clear, or pick due dates",
		"Set, clear, or pick snooze dates",
		"Set or clear stage",
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

func TestTutorQuickCaptureTopicTeachesCalmPlaceholderAndSyntaxGate(t *testing.T) {
	topic := findTutorTUITopic(t, "Quick Capture")

	required := []string{
		`"Type a task title…"`,
		"?: syntax",
		"Task",
		"Route",
		"Schedule",
		"Metadata",
		"Aliases",
		"Linked Task Creator",
		"linked tasks always belong to the\nparent's project",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Quick Capture topic missing %q:\n%s", want, topic.Content)
		}
	}

	// Stale strings: the old sigil-laden placeholder, the always-visible chrome
	// strip, the old "?: help" footer hint, and the old "(via:" alias suffix
	// must never reappear in tutor copy.
	stale := []string{
		"^type Task title !urgent",
		"^type · !N/word",
		"?: help",
		"(via:",
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
		`"Type a task title…"`,
		"?: syntax",
		"Task group",
		"Route group",
		"Schedule group",
		"Metadata group",
		"Aliases row",
		"Linked Task Creator",
		"omitted in global capture",
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
		"skip task",
		"mark task done",
		"edit inbox task",
		"manage inbox task tags",
		"undo last triage action",
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
		"Inbox hotspots",
		"i        Quick Capture",
		"O        Open inbox triage",
		"ctrl+i   Global Inbox Processor",
		"T on the dashboard still opens View Trash",
	}
	for _, want := range required {
		if !strings.Contains(topic.Content, want) {
			t.Fatalf("Dashboard at a glance topic missing %q:\n%s", want, topic.Content)
		}
	}

	// Session-7 stale-string assertions: the legacy per-project line label
	// "Inbox backlog:" and the legacy pipeline stage label "Organize(" were
	// removed from the dashboard chrome and must not reappear in tutor copy.
	stale := []string{
		"Inbox backlog:",
		"Organize(",
	}
	for _, notWant := range stale {
		if strings.Contains(topic.Content, notWant) {
			t.Fatalf("Dashboard at a glance topic still contains stale wording %q:\n%s", notWant, topic.Content)
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
