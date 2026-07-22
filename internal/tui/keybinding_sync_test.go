package tui

import (
	"sort"
	"strings"
	"testing"
)

// session2SyncViews returns the task-ops and primary views implemented in session 2.
// ViewCrossProjectOutliner is excluded: it shares OutlinerModel but has a different
// (smaller) registry; the bidirectional check would produce false invisible-binding
// failures. ViewProjectRecentActivity is tested separately (same model, one less key).
func session2SyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewDailyReview, &DailyReviewModel{}},
		{ViewWeeklyReview, &WeeklyReviewModel{}},
		{ViewFocusBoard, &FocusBoardModel{}},
		{ViewRecentActivity, &RecentActivityModel{}},
		{ViewProjectRecentActivity, &RecentActivityModel{}},
		{ViewInboxProcessor, &InboxProcessorModel{}},
		{ViewGlobalInboxProcessor, &GlobalInboxProcessorModel{}},
		{ViewOutliner, &OutlinerModel{}},
	}
}

// session3SyncViews returns views introduced in session 3 (palette presets, settings).
func session3SyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewSettings, &SettingsModel{}},
		{ViewRecurrenceEditor, &RecurrenceEditorModel{}},
	}
}

// session4SyncViews returns the task detail view registered in session 4.
func session4SyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewTaskDetail, &TaskDetailModel{}},
	}
}

// session6SyncViews returns the pipeline hub view introduced in session 6.
func session6SyncViews() []syncTestCase {
	return []syncTestCase{}
}

// session8SyncViews returns the capture views introduced in session 8 (clipboard paste).
func session8SyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewQuickCapture, &QuickCaptureModel{}},
		{ViewTaskForm, &TaskFormModel{}},
	}
}

// session9SyncViews returns the roadmap view introduced in session 9.
func session9SyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewRoadmap, &RoadmapModel{}},
	}
}

// conflictSyncViews returns the conflict-management resolution view.
func conflictSyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewConflictResolution, &ConflictResolutionModel{}},
	}
}

// captureParitySyncViews returns the capture views that adopted the shared Quick
// Capture interaction/footer contract. The Linked Task Creator gained a
// declarative keymap in the Quick Capture UX wave so both hosts advertise the
// same local bindings and are covered by the structural sync invariants.
func captureParitySyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewLinkedTaskCreator, &LinkedTaskCreatorModel{}},
	}
}

// syncTestCase pairs a PresentationID with a KeySyncView implementation.
type syncTestCase struct {
	view    PresentationID
	handler KeySyncView
}

// session1SyncViews returns the 19 simple/secondary views implemented in session 1.
func session1SyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewDashboard, &DashboardModel{}},
		{ViewTaskChecklist, &TaskChecklistModel{}},
		{ViewTaskComments, &TaskCommentsModel{}},
		{ViewTaskDecisions, &TaskDecisionsModel{}},
		{ViewTaskAttachments, &TaskAttachmentsModel{}},
		{ViewMessageLog, &MessageLogViewModel{}},
		{ViewNotificationCenter, &NotificationCenterModel{}},
		{ViewAsk, &AskViewModel{}},
		{ViewWorkflowRules, &WorkflowRulesModel{}},
		{ViewWorkflowDryRun, &WorkflowDryRunModel{}},
		{ViewWorkflowWizard, &WorkflowWizardModel{}},
		{ViewTaskTemplates, &TaskTemplatesModel{}},
		{ViewFilterBuilder, &FilterBuilderModel{}},
		{ViewCrossProjectLinker, &CrossProjectLinkerModel{}},
		{ViewTutor, &TutorModel{}},
		{ViewScoreProfileManager, &ScoreProfileManagerModel{}},
		{ViewProjectPreview, &ProjectPreviewModel{}},
		{ViewCustomContextEditor, &CustomContextEditorModel{}},
		{ViewBulkEdit, &BulkEditModel{}},
		{ViewBlockers, &BlockersModel{}},
		{ViewMetricsDashboard, &MetricsDashboardModel{}},
	}
}

// TestKeybindingSyncAllViews verifies structural invariants for each view's keymap:
//   - HandledKeys() is non-empty (keymap exists and has bindings)
//   - No duplicate keys in HandledKeys()
func TestKeybindingSyncAllViews(t *testing.T) {
	allViews := append(session1SyncViews(), session2SyncViews()...)
	allViews = append(allViews, session3SyncViews()...)
	allViews = append(allViews, session4SyncViews()...)
	allViews = append(allViews, session6SyncViews()...)
	allViews = append(allViews, session8SyncViews()...)
	allViews = append(allViews, session9SyncViews()...)
	allViews = append(allViews, captureParitySyncViews()...)
	allViews = append(allViews, conflictSyncViews()...)
	for _, tc := range allViews {
		t.Run(tc.view.String(), func(t *testing.T) {
			keys := tc.handler.HandledKeys()
			if len(keys) == 0 {
				t.Errorf("HandledKeys() returned no keys for view %s", tc.view)
				return
			}
			seen := make(map[string]bool, len(keys))
			for _, k := range keys {
				if seen[k] {
					t.Errorf("duplicate key %q in HandledKeys() for view %s", k, tc.view)
				}
				seen[k] = true
			}
		})
	}
}

// TestPresentationIDString verifies that PresentationID.String() returns meaningful names
// for all views tested across sessions 1 and 2.
func TestPresentationIDString(t *testing.T) {
	cases := map[PresentationID]string{
		// Session 1
		ViewDashboard:           "Dashboard",
		ViewTaskChecklist:       "TaskChecklist",
		ViewTaskComments:        "TaskComments",
		ViewTaskAttachments:     "TaskAttachments",
		ViewMessageLog:          "MessageLog",
		ViewNotificationCenter:  "NotificationCenter",
		ViewAsk:                 "Ask",
		ViewWorkflowRules:       "WorkflowRules",
		ViewWorkflowDryRun:      "WorkflowDryRun",
		ViewWorkflowWizard:      "WorkflowWizard",
		ViewTaskTemplates:       "TaskTemplates",
		ViewFilterBuilder:       "FilterBuilder",
		ViewCrossProjectLinker:  "CrossProjectLinker",
		ViewTutor:               "Reference",
		ViewScoreProfileManager: "ScoreProfileManager",
		ViewProjectPreview:      "ProjectPreview",
		ViewCustomContextEditor: "CustomContextEditor",
		ViewBulkEdit:            "BulkEdit",
		ViewBlockers:            "Blockers",
		ViewMetricsDashboard:    "MetricsDashboard",
		// Session 3
		ViewSettings:         "Settings",
		ViewRecurrenceEditor: "RecurrenceEditor",
		// Session 2
		ViewDailyReview:           "DailyReview",
		ViewWeeklyReview:          "WeeklyReview",
		ViewFocusBoard:            "Planner",
		ViewRecentActivity:        "RecentActivity",
		ViewProjectRecentActivity: "ProjectRecentActivity",
		ViewInboxProcessor:        "InboxProcessor",
		ViewGlobalInboxProcessor:  "GlobalInboxProcessor",
		ViewOutliner:              "Outliner",
		// Session 4
		ViewTaskDetail: "TaskDetail",
		// Session 6
		// Session 8
		ViewQuickCapture: "QuickCapture",
		ViewTaskForm:     "TaskForm",
		// Session 9
		ViewRoadmap: "Roadmap",
		// Capture parity (Quick Capture UX wave)
		ViewLinkedTaskCreator: "LinkedTaskCreator",
		// Conflict management
		ViewConflictResolution: "ConflictResolution",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("PresentationID(%d).String() = %q, want %q", int(v), got, want)
		}
	}
}

// TestExpandRegistryKeyCoverage verifies that expandRegistryKey correctly handles
// the ambiguous and multi-key display strings used in the registry.
func TestExpandRegistryKeyCoverage(t *testing.T) {
	cases := []struct {
		display string
		want    []string
	}{
		{"ctrl+U/D", []string{"ctrl+u", "ctrl+d"}},
		{"ctrl+d/u", []string{"ctrl+d", "ctrl+u"}},
		{"gg/G", []string{"g", "G"}},
		{"j/k", []string{"j", "k"}},
		{"h/l", []string{"h", "l"}},
		{"shift+← / shift+→", []string{"shift+left", "shift+right"}},
		{"shift+↑ / shift+↓", []string{"shift+up", "shift+down"}},
		{"shift+←/→", []string{"shift+left", "shift+right"}},
		{"shift+↑/↓", []string{"shift+up", "shift+down"}},
		{"Tab / shift+← / shift+→", []string{"tab", "shift+left", "shift+right"}},
		{"{ / }", []string{"{", "}"}},
		{"Esc", []string{"esc"}},
		{"1-5", []string{"1", "2", "3", "4", "5"}},
		{"1-9", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}},
		{"P0-P9", []string{"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9"}},
		{"a/A", []string{"a", "A"}},
		{"d/D", []string{"d", "D"}},
	}
	for _, c := range cases {
		got := expandRegistryKey(c.display)
		gotSorted := append([]string(nil), got...)
		wantSorted := append([]string(nil), c.want...)
		sort.Strings(gotSorted)
		sort.Strings(wantSorted)
		if strings.Join(gotSorted, ",") != strings.Join(wantSorted, ",") {
			t.Errorf("expandRegistryKey(%q) = %v, want %v", c.display, got, c.want)
		}
	}
}

// TestCaptureViewsShareKeymapParity asserts the two capture hosts (Quick Capture
// and the Linked Task Creator) advertise an identical set of local bindings and
// that neither keymap re-registers the global ?/Esc keys. Those two keys are
// handled by AppModel-level globals, so duplicating them in a view-local keymap
// would create competing help entries.
func TestCaptureViewsShareKeymapParity(t *testing.T) {
	sortedSet := func(keys []string) []string {
		out := append([]string(nil), keys...)
		sort.Strings(out)
		return out
	}

	qc := (&QuickCaptureModel{}).buildKeyMap().HandledKeys()
	ltc := (&LinkedTaskCreatorModel{}).buildKeyMap().HandledKeys()

	if strings.Join(sortedSet(qc), ",") != strings.Join(sortedSet(ltc), ",") {
		t.Fatalf("capture hosts must advertise identical local bindings:\n  QuickCapture      = %v\n  LinkedTaskCreator = %v", sortedSet(qc), sortedSet(ltc))
	}

	for _, keys := range [][]string{qc, ltc} {
		for _, k := range keys {
			if k == "?" || k == "esc" {
				t.Fatalf("capture keymap must not re-register global key %q (handled by app-level globals)", k)
			}
		}
	}

	// The newly advertised local bindings must be present on the Linked Task
	// Creator: create/open/edit/paste plus the gated syntax scroll/page keys.
	handled := make(map[string]bool, len(ltc))
	for _, k := range ltc {
		handled[k] = true
	}
	for _, want := range []string{"enter", "ctrl+d", "ctrl+o", "ctrl+v", "ctrl+j", "ctrl+k", "pgup", "pgdown"} {
		if !handled[want] {
			t.Fatalf("LinkedTaskCreator keymap missing advertised binding %q; have %v", want, sortedSet(ltc))
		}
	}
}
