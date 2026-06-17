package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/workflow"
)

// TestConfigManager_FullRoundTrip ensures Save→Load→Save→Load preserves
// every non-default field set, catching silent drops on round-trip.
func TestConfigManager_FullRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "rt-config.json")

	manager := NewManager(configPath)
	cfg, err := manager.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}

	undoDepth := 42
	fzfTimeout := 15
	evalInterval := 30
	popupTimeout := 5
	dueSoon := 4
	maxPinned := 7
	maxRecent := 12
	autoTheme := false
	useEmoji := true
	taskFormCollapsed := true
	tutorDismissed := true
	infoPanelMode := int(InfoPanelProjectSummary)
	dashboardLayout := int(DashboardLayoutVertical)
	horizSplit := 55
	vertSplit := 65

	cfg.Editor = "vim"
	cfg.DefaultContext = "work"
	cfg.FzfEnabled = false
	cfg.GlowEnabled = false
	cfg.FzfTimeout = &fzfTimeout
	cfg.UndoHistoryDepth = &undoDepth
	cfg.PopupTimeoutSeconds = &popupTimeout
	cfg.WorkflowEvalInterval = &evalInterval
	cfg.ReviewDueSoonDays = &dueSoon
	cfg.MaxPinnedBarItems = &maxPinned
	cfg.MaxRecentTaskVisits = &maxRecent
	cfg.AutoTheme = &autoTheme
	cfg.UseEmoji = &useEmoji
	cfg.TaskFormCollapsed = &taskFormCollapsed
	cfg.TutorDismissed = &tutorDismissed
	cfg.InfoPanelMode = &infoPanelMode
	cfg.DashboardBottomLayout = &dashboardLayout
	cfg.DashboardHorizontalSplitPct = &horizSplit
	cfg.DashboardVerticalSplitPct = &vertSplit
	cfg.StartView = "focus-board"
	cfg.DefaultProjectView = "project_tree"
	cfg.Stages = []string{"S1", "S2", "S3"}
	cfg.CustomContexts = map[string][]string{"work": {"p1"}}
	cfg.SectionColors = map[string]string{"Active": "#ff0000"}
	cfg.CycleProjects = []string{"p1", "p2"}
	cfg.ContextCycleProjects = map[string][]string{"work": {"p1"}}
	cfg.PinnedCommands = []string{"task-add", "task-list"}
	cfg.GlobalPinnedTags = []string{"backend", "urgent"}
	cfg.CommandHistory = []string{"a", "b", "c"}
	cfg.SavedSearches = map[string]string{"q1": "tag:urgent"}
	cfg.PaletteSearchHistory = []string{"first", "second"}
	cfg.RecentPaletteItems = []RecentPaletteItem{{Category: "Commands", ID: "x", Count: 3}}
	cfg.AllTasksTabOrder = []string{"f1", "f2"}
	now := time.Now().Round(time.Second).UTC()
	cfg.LastDailyReviewAt = &now
	cfg.LastWeeklyReviewAt = &now

	cfg.AddProject(&models.Project{ID: "p1", Slug: "test", Name: "Test"})
	cfg.AddProject(&models.Project{ID: "p2", Slug: "test2", Name: "Test2"})
	cfg.ProjectViewStates["p1"] = ProjectViewState{
		PinnedTagTab:              "backend",
		FilterGitHubIssue:         true,
		OutlinerFilterGitHubIssue: true,
	}
	cfg.CrossOutlinerPinnedTagTab = "urgent"
	cfg.CrossOutlinerFilterGitHubIssue = true

	cfg.WorkflowRules = []workflow.WorkflowRule{
		{
			ID:      "r1",
			Name:    "Rule 1",
			Enabled: true,
			Source:  "default",
			Trigger: workflow.RuleTrigger{
				Field:     workflow.FieldStatus,
				Operation: workflow.OpEquals,
				Value:     "todo",
			},
			Actions: []workflow.RuleAction{
				{ActionType: workflow.ActionAddTag, Value: "new"},
			},
			Priority:     5,
			TriggerCount: 7,
		},
	}
	cfg.WorkflowEvalStates = map[string]WorkflowEvalState{
		"p1": {LastEvalTime: now, RulesHash: "abc"},
	}

	cfg.TaskTemplates = []template.TaskTemplate{
		{
			ID:          "t1",
			Name:        "Template 1",
			Enabled:     true,
			Source:      "default",
			Category:    "bug",
			Type:        models.TaskTypeBug,
			Priority:    2,
			Tags:        []string{"bug", "urgent"},
			Section:     "Active",
			Description: "Hello {{var}}",
			Children:    []template.TemplateChild{{Title: "child", Optional: true}},
			Checklist:   []string{"item1", "item2"},
			UseCount:    11,
		},
	}

	cfg.SavedFilters = []SavedFilter{
		{ID: "f1", Name: "MyFilter", Mode: "snapshot", FilterStatus: "todo", FilterGitHubIssue: true, SortAscending: true, PinnedTagTab: "backend"},
	}

	if saveErr := manager.Save(cfg); saveErr != nil {
		t.Fatalf("first Save failed: %v", saveErr)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("first Load failed: %v", err)
	}

	if saveErr := manager.Save(loaded); saveErr != nil {
		t.Fatalf("second Save failed: %v", saveErr)
	}

	loaded2, err := manager.Load()
	if err != nil {
		t.Fatalf("second Load failed: %v", err)
	}

	// Spot-check every field that could silently drop.
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Editor", loaded2.Editor, "vim"},
		{"DefaultContext", loaded2.DefaultContext, "work"},
		{"FzfEnabled", loaded2.FzfEnabled, false},
		{"GlowEnabled", loaded2.GlowEnabled, false},
		{"FzfTimeout", *loaded2.FzfTimeout, 15},
		{"UndoHistoryDepth", *loaded2.UndoHistoryDepth, 42},
		{"PopupTimeoutSeconds", *loaded2.PopupTimeoutSeconds, 5},
		{"WorkflowEvalInterval", *loaded2.WorkflowEvalInterval, 30},
		{"ReviewDueSoonDays", *loaded2.ReviewDueSoonDays, 4},
		{"MaxPinnedBarItems", *loaded2.MaxPinnedBarItems, 7},
		{"MaxRecentTaskVisits", *loaded2.MaxRecentTaskVisits, 12},
		{"AutoTheme", *loaded2.AutoTheme, false},
		{"UseEmoji", *loaded2.UseEmoji, true},
		{"TaskFormCollapsed", *loaded2.TaskFormCollapsed, true},
		{"TutorDismissed", *loaded2.TutorDismissed, true},
		{"InfoPanelMode", *loaded2.InfoPanelMode, int(InfoPanelProjectSummary)},
		{"DashboardBottomLayout", *loaded2.DashboardBottomLayout, int(DashboardLayoutVertical)},
		{"DashboardHorizontalSplitPct", *loaded2.DashboardHorizontalSplitPct, 55},
		{"DashboardVerticalSplitPct", *loaded2.DashboardVerticalSplitPct, 65},
		{"StartView", loaded2.StartView, "focus-board"},
		{"DefaultProjectView", loaded2.DefaultProjectView, "project_tree"},
		{"WorkflowRulesLen", len(loaded2.WorkflowRules), 1},
		{"WorkflowRuleName", loaded2.WorkflowRules[0].Name, "Rule 1"},
		{"WorkflowRulePriority", loaded2.WorkflowRules[0].Priority, 5},
		{"WorkflowRuleTriggerCount", int(loaded2.WorkflowRules[0].TriggerCount), 7},
		{"WorkflowRuleAction", string(loaded2.WorkflowRules[0].Actions[0].ActionType), "add_tag"},
		{"TaskTemplatesLen", len(loaded2.TaskTemplates), 1},
		{"TaskTemplateName", loaded2.TaskTemplates[0].Name, "Template 1"},
		{"TaskTemplateCategory", loaded2.TaskTemplates[0].Category, "bug"},
		{"TaskTemplatePriority", loaded2.TaskTemplates[0].Priority, 2},
		{"TaskTemplateUseCount", int(loaded2.TaskTemplates[0].UseCount), 11},
		{"TaskTemplateChildOptional", loaded2.TaskTemplates[0].Children[0].Optional, true},
		{"TaskTemplateChecklistLen", len(loaded2.TaskTemplates[0].Checklist), 2},
		{"SavedFiltersLen", len(loaded2.SavedFilters), 1},
		{"SavedFilterName", loaded2.SavedFilters[0].Name, "MyFilter"},
		{"StagesLen", len(loaded2.Stages), 3},
		{"CustomContextsLen", len(loaded2.CustomContexts["work"]), 1},
		{"CycleProjectsLen", len(loaded2.CycleProjects), 2},
		{"ContextCycleProjects", loaded2.ContextCycleProjects["work"][0], "p1"},
		{"PinnedCommandsLen", len(loaded2.PinnedCommands), 2},
		{"GlobalPinnedTagsLen", len(loaded2.GlobalPinnedTags), 2},
		{"GlobalPinnedTagsFirst", loaded2.GlobalPinnedTags[0], "backend"},
		{"CommandHistoryLen", len(loaded2.CommandHistory), 3},
		{"SavedSearches", loaded2.SavedSearches["q1"], "tag:urgent"},
		{"PaletteSearchHistoryLen", len(loaded2.PaletteSearchHistory), 2},
		{"RecentPaletteItemsLen", len(loaded2.RecentPaletteItems), 1},
		{"AllTasksTabOrderLen", len(loaded2.AllTasksTabOrder), 2},
		{"ProjectViewStatePinnedTagTab", loaded2.ProjectViewStates["p1"].PinnedTagTab, "backend"},
		{"ProjectViewStateFilterGitHubIssue", loaded2.ProjectViewStates["p1"].FilterGitHubIssue, true},
		{"ProjectViewStateOutlinerFilterGitHubIssue", loaded2.ProjectViewStates["p1"].OutlinerFilterGitHubIssue, true},
		{"CrossOutlinerPinnedTagTab", loaded2.CrossOutlinerPinnedTagTab, "urgent"},
		{"CrossOutlinerFilterGitHubIssue", loaded2.CrossOutlinerFilterGitHubIssue, true},
		{"SavedFilterPinnedTagTab", loaded2.SavedFilters[0].PinnedTagTab, "backend"},
		{"SavedFilterGitHubIssue", loaded2.SavedFilters[0].FilterGitHubIssue, true},
		{"WorkflowEvalStates", loaded2.WorkflowEvalStates["p1"].RulesHash, "abc"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
	if loaded2.LastDailyReviewAt == nil || !loaded2.LastDailyReviewAt.Equal(now) {
		t.Errorf("LastDailyReviewAt mismatch: got %v want %v", loaded2.LastDailyReviewAt, now)
	}
}
