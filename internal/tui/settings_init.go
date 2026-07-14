package tui

import (
	"strconv"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
)

// NewSettingsModel creates a new settings model reading its initial values from
// the domain-native runtime snapshot in the dependency bundle. It reads the
// portable settings and machine-local domains directly (no config.Config) and
// commits changes back through those two domains under a revision check.
func NewSettingsModel(deps *shellDeps) *SettingsModel {
	rs := deps.Runtime
	mgr := deps.Manager
	settings := rs.Settings()
	machine := rs.Machine()

	undoDepthValue := config.DefaultUndoHistoryDepth
	if settings.UndoHistoryDepth != nil {
		undoDepthValue = *settings.UndoHistoryDepth
	}

	contexts := rs.AllContexts()

	activeName := settings.ActiveTheme
	if activeName == "" {
		activeName = "dark"
	}

	activeTheme := settings.ActiveThemeConfig()

	// The manager can be nil in isolated render tests; fall back to the runtime's
	// own data dir so the workspace panel still shows a sensible location.
	workspaceDataDir := rs.DataDir()
	if mgr != nil {
		workspaceDataDir = config.WorkspaceRootFromConfigPath(mgr.GetConfigPath())
	}

	var inactiveCfg config.Theme
	var activePresetName, inactivePresetName string
	if activeName == "light" {
		inactiveCfg = settings.ThemeDark
		activePresetName = settings.ThemeLightName
		inactivePresetName = settings.ThemeDarkName
	} else {
		inactiveCfg = settings.ThemeLight
		activePresetName = settings.ThemeDarkName
		inactivePresetName = settings.ThemeLightName
	}

	return &SettingsModel{
		runtime:                  rs,
		manager:                  mgr,
		baseSettings:             *settings,
		baseMachine:              *machine,
		coordinator:              newSettingsCoordinator(rs),
		workspaceLocation:        config.DisplayPath(workspaceDataDir),
		workspaceStorage:         config.StorageLocationLabel(workspaceDataDir),
		editorInput:              newSettingsInput("code", 256, 40, machine.Editor, true),
		externalEditorInput:      newSettingsInput("code", 256, 40, machine.ExternalEditor, false),
		contextInput:             newSettingsInput("personal", 50, 30, settings.DefaultContext, false),
		undoDepthInput:           newSettingsInput(strconv.Itoa(config.DefaultUndoHistoryDepth), 4, 10, strconv.Itoa(undoDepthValue), false),
		bgColorInput:             newSettingsInput("(none)", 7, 10, activeTheme.BackgroundColor, false),
		cursorColorInput:         newSettingsInput("(none)", 7, 10, activeTheme.CursorColor, false),
		cursorTextColorInput:     newSettingsInput("(none)", 7, 10, activeTheme.CursorTextColor, false),
		normalTextInput:          newSettingsInput("(none)", 7, 10, activeTheme.Palette.NormalText, false),
		mutedInput:               newSettingsInput("(none)", 7, 10, activeTheme.Palette.Muted, false),
		primaryInput:             newSettingsInput("(none)", 7, 10, activeTheme.Palette.Primary, false),
		secondaryInput:           newSettingsInput("(none)", 7, 10, activeTheme.Palette.Secondary, false),
		accentInput:              newSettingsInput("(none)", 7, 10, activeTheme.Palette.Accent, false),
		successInput:             newSettingsInput("(none)", 7, 10, activeTheme.Palette.Success, false),
		warningInput:             newSettingsInput("(none)", 7, 10, activeTheme.Palette.Warning, false),
		errorColorInput:          newSettingsInput("(none)", 7, 10, activeTheme.Palette.Error, false),
		borderInput:              newSettingsInput("(none)", 7, 10, activeTheme.Palette.Border, false),
		priority1TitleInput:      newSettingsInput("(none)", 7, 10, activeTheme.Palette.Priority1Title, false),
		priority2TitleInput:      newSettingsInput("(none)", 7, 10, activeTheme.Palette.Priority2Title, false),
		priority3TitleInput:      newSettingsInput("(none)", 7, 10, activeTheme.Palette.Priority3Title, false),
		priority4TitleInput:      newSettingsInput("(none)", 7, 10, activeTheme.Palette.Priority4Title, false),
		priority5TitleInput:      newSettingsInput("(none)", 7, 10, activeTheme.Palette.Priority5Title, false),
		dueSoonInput:             newSettingsInput("7", 3, 6, strconv.Itoa(settings.GetReviewDueSoonDays()), false),
		futureInput:              newSettingsInput("30", 3, 6, strconv.Itoa(settings.GetReviewFutureDays()), false),
		oldDaysInput:             newSettingsInput("14", 3, 6, strconv.Itoa(settings.GetReviewOldDays()), false),
		completedHoursInput:      newSettingsInput("24", 3, 6, strconv.Itoa(settings.GetReviewCompletedHours()), false),
		staleTaskDaysInput:       newSettingsInput("14", 3, 6, strconv.Itoa(settings.GetStaleTaskDays()), false),
		recentActivityInput:      newSettingsInput("20", 3, 6, strconv.Itoa(settings.GetRecentActivityLimit()), false),
		recentActivityShowAll:    settings.GetRecentActivityShowAll(),
		linkedTaskJump:           settings.GetLinkedTaskJumpToNew(),
		maxPinnedInput:           newSettingsInput(strconv.Itoa(config.DefaultMaxPinnedBarItems), 2, 6, strconv.Itoa(settings.GetMaxPinnedBarItems()), false),
		maxRecentTaskVisitsInput: newSettingsInput(strconv.Itoa(config.DefaultMaxRecentTaskVisits), 2, 6, strconv.Itoa(settings.GetMaxRecentTaskVisits()), false),
		popupTimeoutInput:        newSettingsInput("0", 2, 6, strconv.Itoa(settings.GetPopupTimeoutSeconds()), false),
		attachmentEditorInput:    newSettingsInput("(OS default)", 256, 40, machine.AttachmentEditor, false),
		codeEditorInput:          newSettingsInput("code", 256, 40, machine.CodeEditor, false),
		defaultViewCursor:        settingsOptionCursor(defaultProjectViewValues, settings.GetDefaultProjectView(), 0),
		outlinerCollapsed:        initOutlinerCollapsed(machine),
		treeColumnDefaults:       initTreeColumnDefaults(machine.TreeDefaultColumns),
		outlinerColumnDefaults:   initOutlinerColumnDefaults(machine.OutlinerDefaultColumns),
		treePriorityMode:         priorityModeToCursor(machine.TreePriorityMode),
		outlinerPriorityMode:     priorityModeToCursor(machine.OutlinerPriorityMode),
		crossOutlinerSidebar:     machine.CrossOutlinerSidebarEnabled == nil || *machine.CrossOutlinerSidebarEnabled,
		crossOutlinerCycleOnly:   machine.CrossOutlinerCycleOnly != nil && *machine.CrossOutlinerCycleOnly,
		zebraStriping:            settings.ZebraStriping(),
		weekEndDayCursor:         settingsOptionCursor(weekEndDayValues, settings.GetWeekEndDay(), 2),
		timeFormatCursor:         settingsOptionCursor(timeFormatValues, settings.TimeFormat, 1),
		autoTheme:                settings.GetAutoTheme(),
		useEmoji:                 settings.GetUseEmoji(),
		contexts:                 contexts,
		contextCursor:            settingsOptionCursor(contexts, settings.DefaultContext, -1),
		alertDailyReviewEnabled:  settings.GetAlertDailyReviewEnabled(),
		alertDailyReviewHourInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertDailyReviewHour),
			2,
			6,
			strconv.Itoa(settings.GetAlertDailyReviewHour()),
			false,
		),
		alertWeeklyReviewEnabled: settings.GetAlertWeeklyReviewEnabled(),
		alertWeeklyReviewDay:     settings.GetAlertWeeklyReviewDay(),
		alertOverdueEnabled:      settings.GetAlertOverdueEnabled(),
		alertDueSoonEnabled:      settings.GetAlertDueSoonEnabled(),
		alertDueSoonDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertDueSoonDays),
			3,
			6,
			strconv.Itoa(settings.GetAlertDueSoonDays()),
			false,
		),
		alertStaleEnabled: settings.GetAlertStaleEnabled(),
		alertStaleDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertStaleDays),
			3,
			6,
			strconv.Itoa(settings.GetAlertStaleDays()),
			false,
		),
		alertInboxEnabled: settings.GetAlertInboxEnabled(),
		alertInboxThresholdInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertInboxThreshold),
			3,
			6,
			strconv.Itoa(settings.GetAlertInboxThreshold()),
			false,
		),
		alertBlockedEnabled: settings.GetAlertBlockedEnabled(),
		alertBlockedDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertBlockedDays),
			3,
			6,
			strconv.Itoa(settings.GetAlertBlockedDays()),
			false,
		),
		alertProjectHealthEnabled: settings.GetAlertProjectHealthEnabled(),
		alertProjectStaleDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertProjectStaleDays),
			3,
			6,
			strconv.Itoa(settings.GetAlertProjectStaleDays()),
			false,
		),
		sectionLimitActiveInput:  newSettingsInput(strconv.Itoa(config.DefaultSectionLimitActive), 3, 6, strconv.Itoa(settings.GetSectionLimit("")), false),
		sectionLimitNextInput:    newSettingsInput(strconv.Itoa(config.DefaultSectionLimitNext), 3, 6, strconv.Itoa(settings.GetSectionLimit("Next")), false),
		sectionLimitInboxInput:   newSettingsInput(strconv.Itoa(config.DefaultSectionLimitInbox), 3, 6, strconv.Itoa(settings.GetSectionLimit("Inbox")), false),
		sectionLimitStandbyInput: newSettingsInput(strconv.Itoa(config.DefaultSectionLimitStandby), 3, 6, strconv.Itoa(settings.GetSectionLimit("Standby")), false),
		sectionLimitWaitingInput: newSettingsInput(strconv.Itoa(config.DefaultSectionLimitWaiting), 3, 6, strconv.Itoa(settings.GetSectionLimit("Waiting")), false),
		sectionLimitSomedayInput: newSettingsInput(strconv.Itoa(config.DefaultSectionLimitSomeday), 3, 6, strconv.Itoa(settings.GetSectionLimit("Someday")), false),
		sectionLimitReviewInput:  newSettingsInput(strconv.Itoa(config.DefaultSectionLimitReview), 3, 6, strconv.Itoa(settings.GetSectionLimit("Review")), false),
		themePresetInput:         newSettingsInput("preset name", 50, 30, "", false),
		originalTheme:            NewThemeConfig(settings),
		activeTheme:              activeName,
		activePresetName:         activePresetName,
		inactivePresetName:       inactivePresetName,
		inactiveThemeDraft:       themeToInactiveDraft(inactiveCfg),
		activeHiddenDimmed:       themeHiddenDimmed(activeTheme),
		startViewCursor:          settingsOptionCursor(startViewValues, machine.GetStartView(), defaultStartViewCursor),
		taskFormCollapsed:        machine.GetTaskFormCollapsed(),
		focusBoardModeCursor:     settingsOptionCursor(focusBoardModeValues, settings.GetFocusBoardDefaultMode(), 0),
		dashboardLayoutCursor:    initDashboardLayoutCursor(machine.GetDashboardBottomLayout()),
		dashboardHorizSplitInput: newSettingsInput(strconv.Itoa(config.DefaultDashboardHorizontalSplitPct), 2, 6, strconv.Itoa(machine.GetDashboardHorizontalSplitPct()), false),
		dashboardVertSplitInput:  newSettingsInput(strconv.Itoa(config.DefaultDashboardVerticalSplitPct), 2, 6, strconv.Itoa(machine.GetDashboardVerticalSplitPct()), false),
		resumeCardLimitInput:     newSettingsInput(strconv.Itoa(config.DefaultResumeCardLimit), 2, 6, strconv.Itoa(settings.GetResumeCardLimit()), false),
		kanbanDefaultModeCursor:  settingsOptionCursor(kanbanModeValues, settings.KanbanSettings.DefaultMode, 0),
		kanbanStartColInput:      newSettingsInput("inbox", 30, 20, settings.KanbanSettings.DefaultStartColumn, false),
		kanbanShowEmptyColumns:   settings.KanbanSettings.ShowEmptyColumns == nil || *settings.KanbanSettings.ShowEmptyColumns,
		kanbanDetailPaneVisible:  settings.KanbanSettings.DetailPaneVisible != nil && *settings.KanbanSettings.DetailPaneVisible,
		kanbanDetailHeightCursor: initKanbanDetailHeight(settings.KanbanSettings.DetailPaneHeight),
		kanbanEnabledModes:       initKanbanEnabledModes(settings.KanbanSettings.EnabledModes),
		focusIndex:               0,
		tabs:                     TabState{Names: settingsTabNames},
		width:                    80,
		height:                   24,
	}
}

func newSettingsInput(placeholder string, charLimit, width int, value string, focused bool) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = charLimit
	input.SetWidth(width)
	input.SetValue(value)
	if focused {
		input.Focus()
	}
	applyThemedTextInput(&input)
	return input
}

func settingsOptionCursor(options []string, value string, fallback int) int {
	for i, option := range options {
		if option == value {
			return i
		}
	}
	return fallback
}

// themeToInactiveDraft flattens a config.Theme into the 20-slot draft array
// used by inactiveThemeDraft. Slot ordering matches palette-tab focus offsets:
//
//	 0-8:   NormalText, Muted, Primary, Secondary, Accent, Success, Warning, Error, Border
//	 9-11:  BackgroundColor, CursorColor, CursorTextColor
//	12-16:  Priority1Title … Priority5Title
//	17-19:  Dimmed, DimmedBg, DimmedFg (no UI input, carried through transparently)
func themeToInactiveDraft(t config.Theme) [20]string {
	return [20]string{
		t.Palette.NormalText,
		t.Palette.Muted,
		t.Palette.Primary,
		t.Palette.Secondary,
		t.Palette.Accent,
		t.Palette.Success,
		t.Palette.Warning,
		t.Palette.Error,
		t.Palette.Border,
		t.BackgroundColor,
		t.CursorColor,
		t.CursorTextColor,
		t.Palette.Priority1Title,
		t.Palette.Priority2Title,
		t.Palette.Priority3Title,
		t.Palette.Priority4Title,
		t.Palette.Priority5Title,
		t.Palette.Dimmed,
		t.Palette.DimmedBg,
		t.Palette.DimmedFg,
	}
}

// themeHiddenDimmed extracts the Dimmed/DimmedBg/DimmedFg values from a theme
// for storage in activeHiddenDimmed (these have no UI input widget).
func themeHiddenDimmed(t config.Theme) [3]string {
	return [3]string{t.Palette.Dimmed, t.Palette.DimmedBg, t.Palette.DimmedFg}
}

// initKanbanDetailHeight converts a *int detail pane height (1-5) to a cursor index (0-4).
// nil or out-of-range defaults to index 2 (height 3).
func initKanbanDetailHeight(h *int) int {
	if h == nil {
		return 2 // default height 3
	}
	v := *h - 1 // convert 1-based to 0-based
	if v < 0 {
		return 0
	}
	if v > 4 {
		return 4
	}
	return v
}

// initKanbanEnabledModes converts a slice of mode ID strings to a [5]bool toggle array.
// nil or empty slice means all modes are enabled.
func initKanbanEnabledModes(ids []string) [4]bool {
	var result [4]bool
	if len(ids) == 0 {
		// All enabled by default.
		for i := range result {
			result[i] = true
		}
		return result
	}
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	for i, v := range kanbanModeValues {
		result[i] = idSet[v]
	}
	return result
}

// initDashboardLayoutCursor clamps the layout enum into a valid cursor index.
func initDashboardLayoutCursor(v config.DashboardBottomLayout) int {
	i := int(v)
	if i < 0 || i >= len(dashboardLayoutOptions) {
		return 0
	}
	return i
}

func initOutlinerCollapsed(machine *config.MachineState) [7]bool {
	var outlinerCollapsed [7]bool
	collapsedMap := machine.GetOutlinerCollapsedSections()
	for i, sectionName := range outlinerSectionNames {
		outlinerCollapsed[i] = collapsedMap[sectionName]
	}
	return outlinerCollapsed
}
