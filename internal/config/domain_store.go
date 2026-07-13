package config

import "github.com/sglavoie/tt/internal/models"

// Narrow store interfaces for each configuration domain. New code depends on
// only the domain it needs instead of the whole Manager/Config surface.
// DomainFileStore (domain_file_store.go) is the canonical implementation that
// persists each domain to its own synced file.

// WorkspaceCatalogStore loads and saves the workspace catalog.
type WorkspaceCatalogStore interface {
	LoadCatalog() (*WorkspaceCatalog, error)
	SaveCatalog(*WorkspaceCatalog) error
}

// PlanningStore loads and saves planning collections.
type PlanningStore interface {
	LoadPlanning() (*PlanningState, error)
	SavePlanning(*PlanningState) error
}

// SettingsStore loads and saves portable settings.
type SettingsStore interface {
	LoadSettings() (*Settings, error)
	SaveSettings(*Settings) error
}

// AutomationStore loads and saves automation configuration.
type AutomationStore interface {
	LoadAutomation() (*AutomationConfig, error)
	SaveAutomation(*AutomationConfig) error
}

// MachineStore loads and saves machine-local state.
type MachineStore interface {
	LoadMachine() (*MachineState, error)
	SaveMachine(*MachineState) error
}

// Domains bundles the five decomposed aggregates. It is the in-memory facade
// between the legacy Config file and the domain-oriented model.
type Domains struct {
	Catalog    *WorkspaceCatalog
	Planning   *PlanningState
	Settings   *Settings
	Automation *AutomationConfig
	Machine    *MachineState
}

// Decompose splits a Config into its five owning domains. Every persisted
// Config field has an explicit owner (see configFieldOwner in
// domain_coverage_test.go for the mechanically verified migration map); the
// only fields not carried into a domain are the retired custom-stage vocabulary
// (c.Stages, c.CrossOutlinerFilterStage), which are intentionally dropped, and
// the runtime workspace-location container (c.DataDir). A nil config yields
// fully defaulted domains.
func Decompose(c *Config) *Domains {
	if c == nil {
		return &Domains{
			Catalog:    NewWorkspaceCatalog(),
			Planning:   NewPlanningState(),
			Settings:   NewSettings(),
			Automation: NewAutomationConfig(),
			Machine:    NewMachineState(),
		}
	}

	catalog := &WorkspaceCatalog{
		SchemaVersion:        c.Version,
		Projects:             make([]CatalogProject, 0, len(c.Projects)),
		SystemInboxProjectID: c.ActionInboxProjectID,
	}
	if catalog.SchemaVersion <= 0 {
		catalog.SchemaVersion = CurrentConfigVersion
	}
	for _, p := range c.Projects {
		if p == nil {
			continue
		}
		catalog.Projects = append(catalog.Projects, catalogProjectFromModel(p))
	}

	planning := &PlanningState{
		FocusQueue:         c.FocusQueue,
		DailyPlan:          c.DailyPlan,
		FocusDismissedIDs:  c.FocusDismissedIDs,
		AlertDismissals:    c.AlertDismissals,
		SavedViews:         c.SavedViews,
		SavedFilters:       c.SavedFilters,
		SmartLists:         c.SmartLists,
		LastDailyReviewAt:  c.LastDailyReviewAt,
		LastWeeklyReviewAt: c.LastWeeklyReviewAt,
		TodayClearedAt:     c.TodayClearedAt,
		StreakCount:        c.StreakCount,
		StreakLastDate:     c.StreakLastDate,
	}

	settings := &Settings{
		DefaultContext:        c.DefaultContext,
		FzfEnabled:            c.FzfEnabled,
		GlowEnabled:           c.GlowEnabled,
		FzfTimeout:            c.FzfTimeout,
		UndoHistoryDepth:      c.UndoHistoryDepth,
		PopupTimeoutSeconds:   c.PopupTimeoutSeconds,
		TimeFormat:            c.TimeFormat,
		WeekEndDay:            c.WeekEndDay,
		UseEmoji:              c.UseEmoji,
		LinkedTaskJumpToNew:   c.LinkedTaskJumpToNew,
		TutorDismissed:        c.TutorDismissed,
		DefaultProjectView:    c.DefaultProjectView,
		FocusBoardDefaultMode: c.FocusBoardDefaultMode,
		GlobalPinnedTags:      c.GlobalPinnedTags,
		SectionColors:         c.SectionColors,

		MaxActivityEntries:    c.MaxActivityEntries,
		RecentActivityLimit:   c.RecentActivityLimit,
		RecentActivityShowAll: c.RecentActivityShowAll,
		MaxPinnedBarItems:     c.MaxPinnedBarItems,
		MaxRecentTaskVisits:   c.MaxRecentTaskVisits,
		ResumeCardLimit:       c.ResumeCardLimit,

		ReviewDueSoonDays:    c.ReviewDueSoonDays,
		ReviewFutureDays:     c.ReviewFutureDays,
		ReviewOldDays:        c.ReviewOldDays,
		ReviewCompletedHours: c.ReviewCompletedHours,

		WeeklyStaleProjectDays:    c.WeeklyStaleProjectDays,
		WeeklyStaleTaskDays:       c.WeeklyStaleTaskDays,
		StaleTaskDays:             c.StaleTaskDays,
		PriorityEscalationDays:    c.PriorityEscalationDays,
		WeeklyInboxThreshold:      c.WeeklyInboxThreshold,
		WeeklyBlockedThreshold:    c.WeeklyBlockedThreshold,
		WeeklyInProgressThreshold: c.WeeklyInProgressThreshold,

		SectionLimitActive:  c.SectionLimitActive,
		SectionLimitNext:    c.SectionLimitNext,
		SectionLimitInbox:   c.SectionLimitInbox,
		SectionLimitStandby: c.SectionLimitStandby,
		SectionLimitWaiting: c.SectionLimitWaiting,
		SectionLimitSomeday: c.SectionLimitSomeday,
		SectionLimitReview:  c.SectionLimitReview,

		ActiveTheme:        c.ActiveTheme,
		AutoTheme:          c.AutoTheme,
		ThemeDark:          c.ThemeDark,
		ThemeDarkName:      c.ThemeDarkName,
		ThemeDarkRev:       c.ThemeDarkRev,
		ThemeLight:         c.ThemeLight,
		ThemeLightName:     c.ThemeLightName,
		ThemeLightRev:      c.ThemeLightRev,
		Palette:            c.Palette,
		KanbanSettings:     c.KanbanSettings,
		TUIBackgroundColor: c.TUIBackgroundColor,
		TUICursorColor:     c.TUICursorColor,
		TUICursorTextColor: c.TUICursorTextColor,
		TUIZebraStriping:   c.TUIZebraStriping,

		AlertDailyReviewEnabled:   c.AlertDailyReviewEnabled,
		AlertWeeklyReviewEnabled:  c.AlertWeeklyReviewEnabled,
		AlertOverdueEnabled:       c.AlertOverdueEnabled,
		AlertDueSoonEnabled:       c.AlertDueSoonEnabled,
		AlertStaleEnabled:         c.AlertStaleEnabled,
		AlertInboxEnabled:         c.AlertInboxEnabled,
		AlertBlockedEnabled:       c.AlertBlockedEnabled,
		AlertProjectHealthEnabled: c.AlertProjectHealthEnabled,
		AlertDueSoonDays:          c.AlertDueSoonDays,
		AlertStaleDays:            c.AlertStaleDays,
		AlertInboxThreshold:       c.AlertInboxThreshold,
		AlertBlockedDays:          c.AlertBlockedDays,
		AlertProjectStaleDays:     c.AlertProjectStaleDays,
		AlertDailyReviewHour:      c.AlertDailyReviewHour,
		AlertWeeklyReviewDay:      c.AlertWeeklyReviewDay,
	}

	automation := &AutomationConfig{
		WorkflowRules:           c.WorkflowRules,
		DefaultRulesInitialized: c.DefaultRulesInitialized,
		WorkflowEvalStates:      c.WorkflowEvalStates,
		WorkflowEvalInterval:    c.WorkflowEvalInterval,
		TaskTemplates:           c.TaskTemplates,
		DefaultTemplatesInit:    c.DefaultTemplatesInit,
		ScoringProfiles:         c.ScoringProfiles,
		ProjectScoringProfile:   c.ProjectScoringProfile,
	}

	machine := &MachineState{
		Editor:           c.Editor,
		AttachmentEditor: c.AttachmentEditor,
		CodeEditor:       c.CodeEditor,
		ExternalEditor:   c.ExternalEditor,

		CurrentProject:       c.CurrentProject,
		CycleProjects:        c.CycleProjects,
		ContextCycleProjects: c.ContextCycleProjects,
		CustomContexts:       c.CustomContexts,
		RecentTaskVisits:     c.RecentTaskVisits,
		StartView:            c.StartView,
		FilterContext:        c.FilterContext,
		FilterCustomContext:  c.FilterCustomContext,
		ScoreFilterCustom:    c.ScoreFilterCustom,

		RecentPaletteItems:   c.RecentPaletteItems,
		LastPaletteCategory:  c.LastPaletteCategory,
		PaletteSearchHistory: c.PaletteSearchHistory,
		GlobalSearchHistory:  c.GlobalSearchHistory,
		CommandHistory:       c.CommandHistory,
		PinnedCommands:       c.PinnedCommands,
		SavedSearches:        c.SavedSearches,

		ProjectViewStates:           c.ProjectViewStates,
		ShowInfoPanel:               c.ShowInfoPanel,
		InfoPanelMode:               c.InfoPanelMode,
		DashboardBottomLayout:       c.DashboardBottomLayout,
		DashboardHorizontalSplitPct: c.DashboardHorizontalSplitPct,
		DashboardVerticalSplitPct:   c.DashboardVerticalSplitPct,
		OutlinerCollapsedSections:   c.OutlinerCollapsedSections,
		TreeDefaultColumns:          c.TreeDefaultColumns,
		OutlinerDefaultColumns:      c.OutlinerDefaultColumns,
		TreePriorityMode:            c.TreePriorityMode,
		OutlinerPriorityMode:        c.OutlinerPriorityMode,
		AllTasksTabOrder:            c.AllTasksTabOrder,
		TaskFormCollapsed:           c.TaskFormCollapsed,

		CrossOutlinerSidebarEnabled:    c.CrossOutlinerSidebarEnabled,
		CrossOutlinerCycleOnly:         c.CrossOutlinerCycleOnly,
		CrossOutlinerCollapsedSections: c.CrossOutlinerCollapsedSections,
		CrossOutlinerFoldState:         c.CrossOutlinerFoldState,
		CrossOutlinerColumnToggles:     c.CrossOutlinerColumnToggles,
		CrossOutlinerFilterType:        c.CrossOutlinerFilterType,
		CrossOutlinerFilterStatus:      c.CrossOutlinerFilterStatus,
		CrossOutlinerFilterPriority:    c.CrossOutlinerFilterPriority,
		CrossOutlinerFilterDueDate:     c.CrossOutlinerFilterDueDate,
		CrossOutlinerFilterActionable:  c.CrossOutlinerFilterActionable,
		CrossOutlinerFilterHideDimmed:  c.CrossOutlinerFilterHideDimmed,
		CrossOutlinerFilterGitHubIssue: c.CrossOutlinerFilterGitHubIssue,
		CrossOutlinerFilterParked:      c.CrossOutlinerFilterParked,
		CrossOutlinerFilterScoreMin:    c.CrossOutlinerFilterScoreMin,
		CrossOutlinerFilterTags:        c.CrossOutlinerFilterTags,
		CrossOutlinerFilterSections:    c.CrossOutlinerFilterSections,
		CrossOutlinerFilterAttrs:       c.CrossOutlinerFilterAttrs,
		CrossOutlinerPinnedTagTab:      c.CrossOutlinerPinnedTagTab,
		CrossOutlinerSortField:         c.CrossOutlinerSortField,
		CrossOutlinerSortAscending:     c.CrossOutlinerSortAscending,
		CrossOutlinerFilterBarEnabled:  c.CrossOutlinerFilterBarEnabled,
	}

	return &Domains{
		Catalog:    catalog,
		Planning:   planning,
		Settings:   settings,
		Automation: automation,
		Machine:    machine,
	}
}

// ApplyTo writes the domain values back onto a Config, deriving each project's
// absolute Path from the config's data directory. Projects already present in
// the config are matched by ID so their computed (unpersisted) fields survive;
// cataloged projects with no match are appended as fresh models. It is the
// inverse of Decompose for the fields the domains own.
func (d *Domains) ApplyTo(c *Config) {
	if d == nil || c == nil {
		return
	}

	if d.Catalog != nil {
		if d.Catalog.SchemaVersion > 0 {
			c.Version = d.Catalog.SchemaVersion
		}
		c.ActionInboxProjectID = d.Catalog.SystemInboxProjectID
		c.Projects = d.mergeCatalogProjects(c)
	}

	if p := d.Planning; p != nil {
		c.FocusQueue = p.FocusQueue
		c.DailyPlan = p.DailyPlan
		c.FocusDismissedIDs = p.FocusDismissedIDs
		c.AlertDismissals = p.AlertDismissals
		c.SavedViews = p.SavedViews
		c.SavedFilters = p.SavedFilters
		c.SmartLists = p.SmartLists
		c.LastDailyReviewAt = p.LastDailyReviewAt
		c.LastWeeklyReviewAt = p.LastWeeklyReviewAt
		c.TodayClearedAt = p.TodayClearedAt
		c.StreakCount = p.StreakCount
		c.StreakLastDate = p.StreakLastDate
	}

	if s := d.Settings; s != nil {
		c.DefaultContext = s.DefaultContext
		c.FzfEnabled = s.FzfEnabled
		c.GlowEnabled = s.GlowEnabled
		c.FzfTimeout = s.FzfTimeout
		c.UndoHistoryDepth = s.UndoHistoryDepth
		c.PopupTimeoutSeconds = s.PopupTimeoutSeconds
		c.TimeFormat = s.TimeFormat
		c.WeekEndDay = s.WeekEndDay
		c.UseEmoji = s.UseEmoji
		c.LinkedTaskJumpToNew = s.LinkedTaskJumpToNew
		c.TutorDismissed = s.TutorDismissed
		c.DefaultProjectView = s.DefaultProjectView
		c.FocusBoardDefaultMode = s.FocusBoardDefaultMode
		c.GlobalPinnedTags = s.GlobalPinnedTags
		c.SectionColors = s.SectionColors

		c.MaxActivityEntries = s.MaxActivityEntries
		c.RecentActivityLimit = s.RecentActivityLimit
		c.RecentActivityShowAll = s.RecentActivityShowAll
		c.MaxPinnedBarItems = s.MaxPinnedBarItems
		c.MaxRecentTaskVisits = s.MaxRecentTaskVisits
		c.ResumeCardLimit = s.ResumeCardLimit

		c.ReviewDueSoonDays = s.ReviewDueSoonDays
		c.ReviewFutureDays = s.ReviewFutureDays
		c.ReviewOldDays = s.ReviewOldDays
		c.ReviewCompletedHours = s.ReviewCompletedHours

		c.WeeklyStaleProjectDays = s.WeeklyStaleProjectDays
		c.WeeklyStaleTaskDays = s.WeeklyStaleTaskDays
		c.StaleTaskDays = s.StaleTaskDays
		c.PriorityEscalationDays = s.PriorityEscalationDays
		c.WeeklyInboxThreshold = s.WeeklyInboxThreshold
		c.WeeklyBlockedThreshold = s.WeeklyBlockedThreshold
		c.WeeklyInProgressThreshold = s.WeeklyInProgressThreshold

		c.SectionLimitActive = s.SectionLimitActive
		c.SectionLimitNext = s.SectionLimitNext
		c.SectionLimitInbox = s.SectionLimitInbox
		c.SectionLimitStandby = s.SectionLimitStandby
		c.SectionLimitWaiting = s.SectionLimitWaiting
		c.SectionLimitSomeday = s.SectionLimitSomeday
		c.SectionLimitReview = s.SectionLimitReview

		c.ActiveTheme = s.ActiveTheme
		c.AutoTheme = s.AutoTheme
		c.ThemeDark = s.ThemeDark
		c.ThemeDarkName = s.ThemeDarkName
		c.ThemeDarkRev = s.ThemeDarkRev
		c.ThemeLight = s.ThemeLight
		c.ThemeLightName = s.ThemeLightName
		c.ThemeLightRev = s.ThemeLightRev
		c.Palette = s.Palette
		c.KanbanSettings = s.KanbanSettings
		c.TUIBackgroundColor = s.TUIBackgroundColor
		c.TUICursorColor = s.TUICursorColor
		c.TUICursorTextColor = s.TUICursorTextColor
		c.TUIZebraStriping = s.TUIZebraStriping

		c.AlertDailyReviewEnabled = s.AlertDailyReviewEnabled
		c.AlertWeeklyReviewEnabled = s.AlertWeeklyReviewEnabled
		c.AlertOverdueEnabled = s.AlertOverdueEnabled
		c.AlertDueSoonEnabled = s.AlertDueSoonEnabled
		c.AlertStaleEnabled = s.AlertStaleEnabled
		c.AlertInboxEnabled = s.AlertInboxEnabled
		c.AlertBlockedEnabled = s.AlertBlockedEnabled
		c.AlertProjectHealthEnabled = s.AlertProjectHealthEnabled
		c.AlertDueSoonDays = s.AlertDueSoonDays
		c.AlertStaleDays = s.AlertStaleDays
		c.AlertInboxThreshold = s.AlertInboxThreshold
		c.AlertBlockedDays = s.AlertBlockedDays
		c.AlertProjectStaleDays = s.AlertProjectStaleDays
		c.AlertDailyReviewHour = s.AlertDailyReviewHour
		c.AlertWeeklyReviewDay = s.AlertWeeklyReviewDay
	}

	if a := d.Automation; a != nil {
		c.WorkflowRules = a.WorkflowRules
		c.DefaultRulesInitialized = a.DefaultRulesInitialized
		c.WorkflowEvalStates = a.WorkflowEvalStates
		c.WorkflowEvalInterval = a.WorkflowEvalInterval
		c.TaskTemplates = a.TaskTemplates
		c.DefaultTemplatesInit = a.DefaultTemplatesInit
		c.ScoringProfiles = a.ScoringProfiles
		c.ProjectScoringProfile = a.ProjectScoringProfile
	}

	if m := d.Machine; m != nil {
		c.Editor = m.Editor
		c.AttachmentEditor = m.AttachmentEditor
		c.CodeEditor = m.CodeEditor
		c.ExternalEditor = m.ExternalEditor

		c.CurrentProject = m.CurrentProject
		c.CycleProjects = m.CycleProjects
		c.ContextCycleProjects = m.ContextCycleProjects
		c.CustomContexts = m.CustomContexts
		c.RecentTaskVisits = m.RecentTaskVisits
		c.StartView = m.StartView
		c.FilterContext = m.FilterContext
		c.FilterCustomContext = m.FilterCustomContext
		c.ScoreFilterCustom = m.ScoreFilterCustom

		c.RecentPaletteItems = m.RecentPaletteItems
		c.LastPaletteCategory = m.LastPaletteCategory
		c.PaletteSearchHistory = m.PaletteSearchHistory
		c.GlobalSearchHistory = m.GlobalSearchHistory
		c.CommandHistory = m.CommandHistory
		c.PinnedCommands = m.PinnedCommands
		c.SavedSearches = m.SavedSearches

		c.ProjectViewStates = m.ProjectViewStates
		c.ShowInfoPanel = m.ShowInfoPanel
		c.InfoPanelMode = m.InfoPanelMode
		c.DashboardBottomLayout = m.DashboardBottomLayout
		c.DashboardHorizontalSplitPct = m.DashboardHorizontalSplitPct
		c.DashboardVerticalSplitPct = m.DashboardVerticalSplitPct
		c.OutlinerCollapsedSections = m.OutlinerCollapsedSections
		c.TreeDefaultColumns = m.TreeDefaultColumns
		c.OutlinerDefaultColumns = m.OutlinerDefaultColumns
		c.TreePriorityMode = m.TreePriorityMode
		c.OutlinerPriorityMode = m.OutlinerPriorityMode
		c.AllTasksTabOrder = m.AllTasksTabOrder
		c.TaskFormCollapsed = m.TaskFormCollapsed

		c.CrossOutlinerSidebarEnabled = m.CrossOutlinerSidebarEnabled
		c.CrossOutlinerCycleOnly = m.CrossOutlinerCycleOnly
		c.CrossOutlinerCollapsedSections = m.CrossOutlinerCollapsedSections
		c.CrossOutlinerFoldState = m.CrossOutlinerFoldState
		c.CrossOutlinerColumnToggles = m.CrossOutlinerColumnToggles
		c.CrossOutlinerFilterType = m.CrossOutlinerFilterType
		c.CrossOutlinerFilterStatus = m.CrossOutlinerFilterStatus
		c.CrossOutlinerFilterPriority = m.CrossOutlinerFilterPriority
		c.CrossOutlinerFilterDueDate = m.CrossOutlinerFilterDueDate
		c.CrossOutlinerFilterActionable = m.CrossOutlinerFilterActionable
		c.CrossOutlinerFilterHideDimmed = m.CrossOutlinerFilterHideDimmed
		c.CrossOutlinerFilterGitHubIssue = m.CrossOutlinerFilterGitHubIssue
		c.CrossOutlinerFilterParked = m.CrossOutlinerFilterParked
		c.CrossOutlinerFilterScoreMin = m.CrossOutlinerFilterScoreMin
		c.CrossOutlinerFilterTags = m.CrossOutlinerFilterTags
		c.CrossOutlinerFilterSections = m.CrossOutlinerFilterSections
		c.CrossOutlinerFilterAttrs = m.CrossOutlinerFilterAttrs
		c.CrossOutlinerPinnedTagTab = m.CrossOutlinerPinnedTagTab
		c.CrossOutlinerSortField = m.CrossOutlinerSortField
		c.CrossOutlinerSortAscending = m.CrossOutlinerSortAscending
		c.CrossOutlinerFilterBarEnabled = m.CrossOutlinerFilterBarEnabled
	}

	// Retired fields (custom-stage vocabulary) have no domain owner and are
	// intentionally NOT recomposed: c.Stages and c.CrossOutlinerFilterStage are
	// dropped on a domain round trip. See configFieldOwner in
	// domain_coverage_test.go for the mechanically verified migration map.
	// c.DataDir is the workspace-location container resolved at runtime (flags /
	// pointer), not a domain-owned persisted field, so ApplyTo never touches it.
}

// RemoveProjectReferences fans a project deletion out to every domain, so each
// aggregate strips only the references it owns. It returns true when any domain
// changed.
func (d *Domains) RemoveProjectReferences(id, slug string) bool {
	if d == nil {
		return false
	}
	changed := false
	if d.Catalog != nil {
		changed = d.Catalog.RemoveProjectReferences(id, slug) || changed
	}
	if d.Planning != nil {
		changed = d.Planning.RemoveProjectReferences(id, slug) || changed
	}
	if d.Automation != nil {
		changed = d.Automation.RemoveProjectReferences(id, slug) || changed
	}
	if d.Machine != nil {
		changed = d.Machine.RemoveProjectReferences(id, slug) || changed
	}
	return changed
}

// mergeCatalogProjects rebuilds the config's project slice from the catalog,
// preserving computed fields of already-loaded models matched by ID.
func (d *Domains) mergeCatalogProjects(c *Config) []*models.Project {
	byID := make(map[string]*models.Project, len(c.Projects))
	for _, p := range c.Projects {
		if p != nil && p.ID != "" {
			byID[p.ID] = p
		}
	}
	merged := make([]*models.Project, 0, len(d.Catalog.Projects))
	for _, cp := range d.Catalog.Projects {
		if existing, ok := byID[cp.ID]; ok {
			// Preserve computed fields; refresh persisted fields and re-derive Path.
			refreshed := cp.ToModel(c.DataDir)
			refreshed.Tasks = existing.Tasks
			refreshed.NextShortID = existing.NextShortID
			refreshed.Sections = existing.Sections
			refreshed.SectionColors = existing.SectionColors
			refreshed.Tags = existing.Tags
			refreshed.PinnedTags = existing.PinnedTags
			refreshed.Warnings = existing.Warnings
			// Internal/sandbox projects keep their existing (non-derived) path.
			if existing.Internal || existing.Sandbox {
				refreshed.Path = existing.Path
			}
			merged = append(merged, refreshed)
			continue
		}
		merged = append(merged, cp.ToModel(c.DataDir))
	}
	return merged
}
