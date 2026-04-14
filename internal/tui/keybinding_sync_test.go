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
		{ViewDueDateOverview, &DueDateOverviewModel{}},
		{ViewPriorityView, &PriorityViewModel{}},
		{ViewFocusBoard, &FocusBoardModel{}},
		{ViewAllTasks, &AllTasksViewModel{}},
		{ViewFocusView, &FocusViewModel{}},
		{ViewProjectFocusView, &ProjectFocusViewModel{}},
		{ViewRecentActivity, &RecentActivityModel{}},
		{ViewProjectRecentActivity, &RecentActivityModel{}},
		{ViewInboxProcessor, &InboxProcessorModel{}},
		{ViewGlobalInboxProcessor, &GlobalInboxProcessorModel{}},
		{ViewProjectTree, &ProjectTreeModel{}},
		{ViewOutliner, &OutlinerModel{}},
	}
}

// syncTestCase pairs a ViewType with a KeySyncView implementation.
type syncTestCase struct {
	view    ViewType
	handler KeySyncView
}

// session1SyncViews returns the 19 simple/secondary views implemented in session 1.
func session1SyncViews() []syncTestCase {
	return []syncTestCase{
		{ViewDashboard, &DashboardModel{}},
		{ViewTaskChecklist, &TaskChecklistModel{}},
		{ViewTaskComments, &TaskCommentsModel{}},
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
		{ViewKeybindingExplorer, &HelpViewModel{isExplorer: true}},
	}
}

// TestKeybindingSyncAllViews verifies that each view's HandledKeys() set matches
// the keys registered in the KeybindingRegistry:
//   - Ghost bindings: key in registry but not in HandledKeys (user sees a key that does nothing)
//   - Invisible bindings: key in HandledKeys but not in registry (key works but is undocumented)
//
// Global app keys (handled by AppModel, not by individual views) are excluded from
// the invisible-binding check since they are intentionally handled at a higher level.
func TestKeybindingSyncAllViews(t *testing.T) {
	registry := GetKeybindingRegistry()
	globalKeys := globalAppKeys()

	globalKeySet := make(map[string]bool, len(globalKeys))
	for _, k := range globalKeys {
		globalKeySet[k] = true
	}

	allViews := append(session1SyncViews(), session2SyncViews()...)
	for _, tc := range allViews {
		t.Run(tc.view.String(), func(t *testing.T) {
			// Build set of keys from registry (view-specific + global bindings).
			registryKeys := make(map[string]bool)
			for _, item := range registry.GetForView(tc.view) {
				if keysExcludedFromSync[item.Key] {
					continue
				}
				for _, k := range expandRegistryKey(item.Key) {
					registryKeys[k] = true
				}
			}

			// Build set of keys from the view's handler + global app keys.
			handlerKeys := make(map[string]bool)
			for _, k := range tc.handler.HandledKeys() {
				handlerKeys[k] = true
			}
			for _, k := range globalKeys {
				handlerKeys[k] = true
			}

			// Ghost bindings: in registry but not handled by view or global handler.
			var ghosts []string
			for k := range registryKeys {
				if !handlerKeys[k] {
					ghosts = append(ghosts, k)
				}
			}

			// Invisible bindings: handled by view but absent from registry.
			// Global keys are excluded because they are documented at the app level.
			var invisible []string
			for k := range handlerKeys {
				if !globalKeySet[k] && !registryKeys[k] {
					invisible = append(invisible, k)
				}
			}

			if len(ghosts) > 0 {
				sort.Strings(ghosts)
				t.Errorf("ghost bindings (in registry but not handled):\n  %s",
					strings.Join(ghosts, ", "))
			}
			if len(invisible) > 0 {
				sort.Strings(invisible)
				t.Errorf("invisible bindings (handled but not in registry):\n  %s",
					strings.Join(invisible, ", "))
			}
		})
	}
}

// TestViewTypeString verifies that ViewType.String() returns meaningful names
// for all views tested across sessions 1 and 2.
func TestViewTypeString(t *testing.T) {
	cases := map[ViewType]string{
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
		ViewTutor:               "Tutor",
		ViewScoreProfileManager: "ScoreProfileManager",
		ViewProjectPreview:      "ProjectPreview",
		ViewCustomContextEditor: "CustomContextEditor",
		ViewBulkEdit:            "BulkEdit",
		ViewBlockers:            "Blockers",
		ViewMetricsDashboard:    "MetricsDashboard",
		// Session 2
		ViewDailyReview:           "DailyReview",
		ViewWeeklyReview:          "WeeklyReview",
		ViewDueDateOverview:       "DueDateOverview",
		ViewPriorityView:          "PriorityView",
		ViewFocusBoard:            "FocusBoard",
		ViewAllTasks:              "AllTasks",
		ViewFocusView:             "FocusView",
		ViewProjectFocusView:      "ProjectFocusView",
		ViewRecentActivity:        "RecentActivity",
		ViewProjectRecentActivity: "ProjectRecentActivity",
		ViewInboxProcessor:        "InboxProcessor",
		ViewGlobalInboxProcessor:  "GlobalInboxProcessor",
		ViewProjectTree:           "ProjectTree",
		ViewOutliner:              "Outliner",
	}
	for v, want := range cases {
		if got := v.String(); got != want {
			t.Errorf("ViewType(%d).String() = %q, want %q", int(v), got, want)
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
		{"[/]", []string{"[", "]"}},
		{"{/}", []string{"{", "}"}},
		{"q/Esc", []string{"q", "esc"}},
		{"1-5", []string{"1", "2", "3", "4", "5"}},
		{"1-9", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}},
		{"P0-P9", []string{"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9"}},
		{".1-.9", []string{".1", ".2", ".3", ".4", ".5", ".6", ".7", ".8", ".9"}},
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
