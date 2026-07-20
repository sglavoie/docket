package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestPlannerShortcutGuidanceMatchesRouting pins the maintained planning
// guidance — keybinding registry labels, tutor snippets, and the inbox
// close-message hints — to what the ;q / ;w shortcuts actually route to. The
// Planner consolidation left ;q opening the Next tab and ;w the Today tab, but
// legacy copy kept calling ;q "Backlog". This test resolves the live routing
// and then compares it against every maintained snippet so the drift cannot
// silently return.
func TestPlannerShortcutGuidanceMatchesRouting(t *testing.T) {
	// 1. Resolve the live routing: drive the semicolon dispatcher and read the
	//    Planner tab each shortcut opens.
	qTab := routeSemicolonPlannerTab(t, 'q')
	wTab := routeSemicolonPlannerTab(t, 'w')
	if qTab != "Next" {
		t.Fatalf(";q routes to Planner %q tab, want Next", qTab)
	}
	if wTab != "Today" {
		t.Fatalf(";w routes to Planner %q tab, want Today", wTab)
	}

	// 2. The keybinding registry labels must name the tab each shortcut opens.
	desc := map[string]string{}
	for _, item := range GetKeybindingRegistry().GetGlobalBindings() {
		desc[item.Key] = item.Description
	}
	if got := desc[";q"]; !strings.Contains(got, "Planner") || !strings.Contains(got, qTab) {
		t.Errorf(";q registry label = %q, want Planner + %q", got, qTab)
	}
	if got := desc[";w"]; !strings.Contains(got, "Planner") || !strings.Contains(got, wTab) {
		t.Errorf(";w registry label = %q, want Planner + %q", got, wTab)
	}

	// 3. Collect every maintained guidance snippet that could pair a shortcut
	//    with a tab: tutor topics, pipeline footer hints, and the live inbox
	//    close messages.
	snippets := map[string]string{}
	for _, topic := range tutorTopics() {
		snippets["tutor topic "+topic.Title] = topic.Content
	}
	for _, vt := range []ViewType{
		ViewQuickCapture, ViewBulkCapture, ViewAllTasks, ViewGlobalInboxProcessor,
		ViewCrossProjectOutliner, ViewCrossProjectKanban, ViewRoadmap, ViewDailyReview,
		ViewFocusBoard, ViewWeeklyReview, ViewMetricsDashboard, ViewRecentActivity,
	} {
		if h := pipelineNextHint(vt); h != "" {
			snippets["pipeline next hint: "+h] = h
		}
	}
	closeMessages := inboxCloseGuidanceSnippets(t)
	for name, msg := range closeMessages {
		snippets[name] = msg
	}

	// 4. Compare: no snippet may pair ;q or ;w with a Planner tab other than
	//    the one it opens (this is what catches the retired ";q Backlog" copy).
	assertShortcutTabPairing(t, snippets, ";q", qTab)
	assertShortcutTabPairing(t, snippets, ";w", wTab)

	// 5. The inbox close messages must positively name the routed tab so the
	//    hint stays actionable, not merely free of the wrong label.
	for name, msg := range closeMessages {
		if strings.Contains(name, "clear") && !strings.Contains(msg, ";w "+wTab) {
			t.Errorf("%s = %q, must offer ;w %s", name, msg, wTab)
		}
		if !strings.Contains(msg, ";q "+qTab) {
			t.Errorf("%s = %q, must offer ;q %s", name, msg, qTab)
		}
	}
}

// routeSemicolonPlannerTab drives the ;<key> dispatcher and returns the Planner
// tab name the resulting navigation command opens.
func routeSemicolonPlannerTab(t *testing.T, key rune) string {
	t.Helper()
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	handled, cmd := app.handlePendingSemicolonPrefix(tea.KeyPressMsg{Code: key, Text: string(key)})
	if !handled || cmd == nil {
		t.Fatalf(";%c did not produce a navigation command", key)
	}
	msg := cmd()
	show, ok := msg.(ShowFocusBoardMsg)
	if !ok {
		t.Fatalf(";%c routed to %T, want ShowFocusBoardMsg", key, msg)
	}
	if !show.SetTab || show.Tab < 0 || show.Tab >= len(focusBoardTabNames) {
		t.Fatalf(";%c produced an invalid Planner tab: %+v", key, show)
	}
	return focusBoardTabNames[show.Tab]
}

// assertShortcutTabPairing fails if any snippet pairs shortcut with a Planner
// tab name other than allowed (case-insensitive, single-space adjacency — the
// inline "shortcut Tab" hint form).
func assertShortcutTabPairing(t *testing.T, snippets map[string]string, shortcut, allowed string) {
	t.Helper()
	for name, text := range snippets {
		lower := strings.ToLower(text)
		for _, tab := range focusBoardTabNames {
			if strings.EqualFold(tab, allowed) {
				continue
			}
			if strings.Contains(lower, strings.ToLower(shortcut+" "+tab)) {
				t.Errorf("%s pairs %s with %q, but %s opens the %q tab", name, shortcut, tab, shortcut, allowed)
			}
		}
	}
}

// inboxCloseGuidanceSnippets drives the four inbox close paths and returns their
// status-bar hint strings keyed by a descriptive name.
func inboxCloseGuidanceSnippets(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}

	localClear, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	localClear.pushView(ViewInboxProcessor)
	localClear.views.Register(ViewInboxProcessor, &InboxProcessorModel{})
	localClear.handleInboxMessages(CloseInboxProcessorMsg{ProcessedCount: 2, SkippedCount: 1})
	out["local inbox clear message"] = localClear.statusMessage

	localPaused, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	localPaused.pushView(ViewInboxProcessor)
	localPaused.views.Register(ViewInboxProcessor, &InboxProcessorModel{})
	localPaused.handleInboxMessages(CloseInboxProcessorMsg{ProcessedCount: 0, SkippedCount: 2})
	out["local inbox paused message"] = localPaused.statusMessage

	globalClear, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	globalClear.pushView(ViewGlobalInboxProcessor)
	globalClear.handleInboxMessages(CloseGlobalInboxProcessorMsg{ProcessedCount: 3, SkippedCount: 1})
	out["global inbox clear message"] = globalClear.statusMessage

	globalPaused, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	globalPaused.pushView(ViewGlobalInboxProcessor)
	globalPaused.handleInboxMessages(CloseGlobalInboxProcessorMsg{ProcessedCount: 0, SkippedCount: 3})
	out["global inbox paused message"] = globalPaused.statusMessage

	return out
}
