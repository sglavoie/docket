package tui

import (
	"strconv"

	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
)

// NewSettingsModel creates a new settings model.
func NewSettingsModel(cfg *config.Config, manager *config.Manager) *SettingsModel {
	undoDepthValue := config.DefaultUndoHistoryDepth
	if cfg.UndoHistoryDepth != nil {
		undoDepthValue = *cfg.UndoHistoryDepth
	}

	contexts := cfg.GetAllContexts()
	stages := append([]string{}, cfg.Stages...)

	activeName := cfg.ActiveTheme
	if activeName == "" {
		activeName = "dark"
	}

	activeTheme := cfg.ActiveThemeConfig()

	var inactiveCfg config.Theme
	if activeName == "light" {
		inactiveCfg = cfg.ThemeDark
	} else {
		inactiveCfg = cfg.ThemeLight
	}

	return &SettingsModel{
		config:                   cfg,
		configManager:            manager,
		editorInput:              newSettingsInput("code", 256, 40, cfg.Editor, true),
		contextInput:             newSettingsInput("personal", 50, 30, cfg.DefaultContext, false),
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
		dueSoonInput:             newSettingsInput("7", 3, 6, strconv.Itoa(cfg.GetReviewDueSoonDays()), false),
		futureInput:              newSettingsInput("30", 3, 6, strconv.Itoa(cfg.GetReviewFutureDays()), false),
		oldDaysInput:             newSettingsInput("14", 3, 6, strconv.Itoa(cfg.GetReviewOldDays()), false),
		completedHoursInput:      newSettingsInput("24", 3, 6, strconv.Itoa(cfg.GetReviewCompletedHours()), false),
		recentActivityInput:      newSettingsInput("20", 3, 6, strconv.Itoa(cfg.GetRecentActivityLimit()), false),
		recentActivityShowAll:    cfg.GetRecentActivityShowAll(),
		linkedTaskJump:           cfg.GetLinkedTaskJumpToNew(),
		maxPinnedInput:           newSettingsInput(strconv.Itoa(config.DefaultMaxPinnedBarItems), 2, 6, strconv.Itoa(cfg.GetMaxPinnedBarItems()), false),
		maxRecentTaskVisitsInput: newSettingsInput(strconv.Itoa(config.DefaultMaxRecentTaskVisits), 2, 6, strconv.Itoa(cfg.GetMaxRecentTaskVisits()), false),
		popupTimeoutInput:        newSettingsInput("0", 2, 6, strconv.Itoa(cfg.GetPopupTimeoutSeconds()), false),
		attachmentEditorInput:    newSettingsInput("(OS default)", 256, 40, cfg.GetAttachmentEditor(), false),
		codeEditorInput:          newSettingsInput("code", 256, 40, cfg.CodeEditor, false),
		defaultViewCursor:        settingsOptionCursor(defaultProjectViewValues, cfg.GetDefaultProjectView(), 0),
		outlinerCollapsed:        initOutlinerCollapsed(cfg),
		treeColumnDefaults:       initColumnDefaults(cfg.TreeDefaultColumns),
		outlinerColumnDefaults:   initColumnDefaults(cfg.OutlinerDefaultColumns),
		treePriorityMode:         priorityModeToCursor(cfg.TreePriorityMode),
		outlinerPriorityMode:     priorityModeToCursor(cfg.OutlinerPriorityMode),
		crossOutlinerSidebar:     cfg.CrossOutlinerSidebarEnabled == nil || *cfg.CrossOutlinerSidebarEnabled,
		zebraStriping:            cfg.ZebraStriping(),
		contexts:                 contexts,
		contextCursor:            settingsOptionCursor(contexts, cfg.DefaultContext, -1),
		savedStages:              append([]string{}, stages...),
		stages:                   stages,
		stageInput:               newSettingsInput("Stage name", 30, 30, "", false),
		alertDailyReviewEnabled:  cfg.GetAlertDailyReviewEnabled(),
		alertDailyReviewHourInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertDailyReviewHour),
			2,
			6,
			strconv.Itoa(cfg.GetAlertDailyReviewHour()),
			false,
		),
		alertWeeklyReviewEnabled: cfg.GetAlertWeeklyReviewEnabled(),
		alertWeeklyReviewDay:     cfg.GetAlertWeeklyReviewDay(),
		alertOverdueEnabled:      cfg.GetAlertOverdueEnabled(),
		alertDueSoonEnabled:      cfg.GetAlertDueSoonEnabled(),
		alertDueSoonDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertDueSoonDays),
			3,
			6,
			strconv.Itoa(cfg.GetAlertDueSoonDays()),
			false,
		),
		alertStaleEnabled: cfg.GetAlertStaleEnabled(),
		alertStaleDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertStaleDays),
			3,
			6,
			strconv.Itoa(cfg.GetAlertStaleDays()),
			false,
		),
		alertInboxEnabled: cfg.GetAlertInboxEnabled(),
		alertInboxThresholdInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertInboxThreshold),
			3,
			6,
			strconv.Itoa(cfg.GetAlertInboxThreshold()),
			false,
		),
		alertBlockedEnabled: cfg.GetAlertBlockedEnabled(),
		alertBlockedDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertBlockedDays),
			3,
			6,
			strconv.Itoa(cfg.GetAlertBlockedDays()),
			false,
		),
		alertProjectHealthEnabled: cfg.GetAlertProjectHealthEnabled(),
		alertProjectStaleDaysInput: newSettingsInput(
			strconv.Itoa(config.DefaultAlertProjectStaleDays),
			3,
			6,
			strconv.Itoa(cfg.GetAlertProjectStaleDays()),
			false,
		),
		sectionLimitActiveInput:  newSettingsInput(strconv.Itoa(config.DefaultSectionLimitActive), 3, 6, strconv.Itoa(cfg.GetSectionLimit("")), false),
		sectionLimitNextInput:    newSettingsInput(strconv.Itoa(config.DefaultSectionLimitNext), 3, 6, strconv.Itoa(cfg.GetSectionLimit("Next")), false),
		sectionLimitInboxInput:   newSettingsInput(strconv.Itoa(config.DefaultSectionLimitInbox), 3, 6, strconv.Itoa(cfg.GetSectionLimit("Inbox")), false),
		sectionLimitStandbyInput: newSettingsInput(strconv.Itoa(config.DefaultSectionLimitStandby), 3, 6, strconv.Itoa(cfg.GetSectionLimit("Standby")), false),
		sectionLimitWaitingInput: newSettingsInput(strconv.Itoa(config.DefaultSectionLimitWaiting), 3, 6, strconv.Itoa(cfg.GetSectionLimit("Waiting")), false),
		sectionLimitSomedayInput: newSettingsInput(strconv.Itoa(config.DefaultSectionLimitSomeday), 3, 6, strconv.Itoa(cfg.GetSectionLimit("Someday")), false),
		sectionLimitReviewInput:  newSettingsInput(strconv.Itoa(config.DefaultSectionLimitReview), 3, 6, strconv.Itoa(cfg.GetSectionLimit("Review")), false),
		themePresetInput:         newSettingsInput("preset name", 50, 30, "", false),
		originalTheme:            NewThemeConfig(cfg),
		activeTheme:              activeName,
		inactiveThemeDraft:       themeToInactiveDraft(inactiveCfg),
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

// themeToInactiveDraft flattens a config.Theme into the 17-slot draft array
// used by inactiveThemeDraft. Slot ordering matches palette-tab focus offsets:
//
//	0-8:  NormalText, Muted, Primary, Secondary, Accent, Success, Warning, Error, Border
//	9-11: BackgroundColor, CursorColor, CursorTextColor
//
// 12-16: Priority1Title … Priority5Title
func themeToInactiveDraft(t config.Theme) [17]string {
	return [17]string{
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
	}
}

func initOutlinerCollapsed(cfg *config.Config) [7]bool {
	var outlinerCollapsed [7]bool
	collapsedMap := cfg.GetOutlinerCollapsedSections()
	for i, sectionName := range outlinerSectionNames {
		outlinerCollapsed[i] = collapsedMap[sectionName]
	}
	return outlinerCollapsed
}
