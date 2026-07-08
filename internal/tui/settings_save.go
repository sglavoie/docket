package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// buildTentativeThemeConfig reads all color input values and returns a ThemeConfig
// reflecting the current (possibly unsaved) form state.
func (m *SettingsModel) buildTentativeThemeConfig() ThemeConfig {
	return ThemeConfig{
		Name:     m.activeTheme,
		BgColor:  normalizeHexColor(m.bgColorInput.Value()),
		CursorBg: normalizeHexColor(m.cursorColorInput.Value()),
		CursorFg: normalizeHexColor(m.cursorTextColorInput.Value()),
		Palette: config.PaletteColors{
			NormalText:     normalizeHexColor(m.normalTextInput.Value()),
			Muted:          normalizeHexColor(m.mutedInput.Value()),
			Primary:        normalizeHexColor(m.primaryInput.Value()),
			Secondary:      normalizeHexColor(m.secondaryInput.Value()),
			Accent:         normalizeHexColor(m.accentInput.Value()),
			Success:        normalizeHexColor(m.successInput.Value()),
			Warning:        normalizeHexColor(m.warningInput.Value()),
			Error:          normalizeHexColor(m.errorColorInput.Value()),
			Border:         normalizeHexColor(m.borderInput.Value()),
			Priority1Title: normalizeHexColor(m.priority1TitleInput.Value()),
			Priority2Title: normalizeHexColor(m.priority2TitleInput.Value()),
			Priority3Title: normalizeHexColor(m.priority3TitleInput.Value()),
			Priority4Title: normalizeHexColor(m.priority4TitleInput.Value()),
			Priority5Title: normalizeHexColor(m.priority5TitleInput.Value()),
			Dimmed:         normalizeHexColor(m.activeHiddenDimmed[0]),
			DimmedBg:       normalizeHexColor(m.activeHiddenDimmed[1]),
			DimmedFg:       normalizeHexColor(m.activeHiddenDimmed[2]),
		},
	}
}

// SetSize sets the view dimensions.
func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SettingsModel) SupportsHelpShortcut() bool { return true }

func (m *SettingsModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewSettings)
}

func (m *SettingsModel) Init() tea.Cmd {
	return textinput.Blink
}

// OnExit restores the original theme when the user cancels out of settings
// without saving, so tentative palette changes are discarded.
func (m *SettingsModel) OnExit() {
	ApplyTheme(m.originalTheme)
}

func (m *SettingsModel) saveSettings() tea.Cmd {
	editor := strings.TrimSpace(m.editorInput.Value())
	context := strings.TrimSpace(m.contextInput.Value())
	undoDepth, _ := strconv.Atoi(strings.TrimSpace(m.undoDepthInput.Value()))
	dueSoon, _ := strconv.Atoi(strings.TrimSpace(m.dueSoonInput.Value()))
	futureDays, _ := strconv.Atoi(strings.TrimSpace(m.futureInput.Value()))
	oldDays, _ := strconv.Atoi(strings.TrimSpace(m.oldDaysInput.Value()))
	completedHours, _ := strconv.Atoi(strings.TrimSpace(m.completedHoursInput.Value()))

	m.config.Editor = editor
	m.config.DefaultContext = context
	m.config.UndoHistoryDepth = &undoDepth

	activeTheme := config.Theme{
		BackgroundColor: normalizeHexColor(m.bgColorInput.Value()),
		CursorColor:     normalizeHexColor(m.cursorColorInput.Value()),
		CursorTextColor: normalizeHexColor(m.cursorTextColorInput.Value()),
		Palette: config.PaletteColors{
			NormalText:     normalizeHexColor(m.normalTextInput.Value()),
			Muted:          normalizeHexColor(m.mutedInput.Value()),
			Primary:        normalizeHexColor(m.primaryInput.Value()),
			Secondary:      normalizeHexColor(m.secondaryInput.Value()),
			Accent:         normalizeHexColor(m.accentInput.Value()),
			Success:        normalizeHexColor(m.successInput.Value()),
			Warning:        normalizeHexColor(m.warningInput.Value()),
			Error:          normalizeHexColor(m.errorColorInput.Value()),
			Border:         normalizeHexColor(m.borderInput.Value()),
			Priority1Title: normalizeHexColor(m.priority1TitleInput.Value()),
			Priority2Title: normalizeHexColor(m.priority2TitleInput.Value()),
			Priority3Title: normalizeHexColor(m.priority3TitleInput.Value()),
			Priority4Title: normalizeHexColor(m.priority4TitleInput.Value()),
			Priority5Title: normalizeHexColor(m.priority5TitleInput.Value()),
			Dimmed:         normalizeHexColor(m.activeHiddenDimmed[0]),
			DimmedBg:       normalizeHexColor(m.activeHiddenDimmed[1]),
			DimmedFg:       normalizeHexColor(m.activeHiddenDimmed[2]),
		},
	}
	inactiveTheme := draftToTheme(m.inactiveThemeDraft)
	if m.activeTheme == "light" {
		m.config.ThemeLight = activeTheme
		m.config.ThemeLightName = m.activePresetName
		m.config.ThemeDark = inactiveTheme
		m.config.ThemeDarkName = m.inactivePresetName
	} else {
		m.config.ThemeDark = activeTheme
		m.config.ThemeDarkName = m.activePresetName
		m.config.ThemeLight = inactiveTheme
		m.config.ThemeLightName = m.inactivePresetName
	}
	m.config.ActiveTheme = m.activeTheme
	m.config.DefaultProjectView = defaultProjectViewValues[m.defaultViewCursor]
	m.config.ReviewDueSoonDays = &dueSoon
	m.config.ReviewFutureDays = &futureDays
	m.config.ReviewOldDays = &oldDays
	m.config.ReviewCompletedHours = &completedHours
	staleTaskDays, _ := strconv.Atoi(strings.TrimSpace(m.staleTaskDaysInput.Value()))
	m.config.StaleTaskDays = &staleTaskDays
	recentActivityLimit, _ := strconv.Atoi(strings.TrimSpace(m.recentActivityInput.Value()))
	m.config.RecentActivityLimit = &recentActivityLimit
	m.config.RecentActivityShowAll = &m.recentActivityShowAll
	m.config.LinkedTaskJumpToNew = &m.linkedTaskJump
	maxPinned, _ := strconv.Atoi(strings.TrimSpace(m.maxPinnedInput.Value()))
	m.config.MaxPinnedBarItems = &maxPinned
	maxRecentVisits, _ := strconv.Atoi(strings.TrimSpace(m.maxRecentTaskVisitsInput.Value()))
	m.config.MaxRecentTaskVisits = &maxRecentVisits
	popupTimeout, _ := strconv.Atoi(strings.TrimSpace(m.popupTimeoutInput.Value()))
	m.config.PopupTimeoutSeconds = &popupTimeout
	sectionLimitActive, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitActiveInput.Value()))
	m.config.SectionLimitActive = &sectionLimitActive
	sectionLimitNext, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitNextInput.Value()))
	m.config.SectionLimitNext = &sectionLimitNext
	sectionLimitInbox, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitInboxInput.Value()))
	m.config.SectionLimitInbox = &sectionLimitInbox
	sectionLimitStandby, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitStandbyInput.Value()))
	m.config.SectionLimitStandby = &sectionLimitStandby
	sectionLimitWaiting, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitWaitingInput.Value()))
	m.config.SectionLimitWaiting = &sectionLimitWaiting
	sectionLimitSomeday, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitSomedayInput.Value()))
	m.config.SectionLimitSomeday = &sectionLimitSomeday
	sectionLimitReview, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitReviewInput.Value()))
	m.config.SectionLimitReview = &sectionLimitReview
	m.config.AttachmentEditor = strings.TrimSpace(m.attachmentEditorInput.Value())
	m.config.CodeEditor = strings.TrimSpace(m.codeEditorInput.Value())
	m.config.ExternalEditor = strings.TrimSpace(m.externalEditorInput.Value())
	m.config.TreeDefaultColumns = treeColumnDefaultsToMap(m.treeColumnDefaults)
	m.config.OutlinerDefaultColumns = outlinerColumnDefaultsToMap(m.outlinerColumnDefaults)

	m.config.TreePriorityMode = priorityModeValues[m.treePriorityMode]
	m.config.OutlinerPriorityMode = priorityModeValues[m.outlinerPriorityMode]
	if m.config.TreePriorityMode == PriorityModeConditional {
		m.config.TreePriorityMode = ""
	}
	if m.config.OutlinerPriorityMode == PriorityModeConditional {
		m.config.OutlinerPriorityMode = ""
	}

	m.config.CrossOutlinerSidebarEnabled = boolPtr(m.crossOutlinerSidebar)
	m.config.CrossOutlinerCycleOnly = boolPtr(m.crossOutlinerCycleOnly)
	m.config.StartView = startViewValues[m.startViewCursor]
	m.config.TaskFormCollapsed = boolPtr(m.taskFormCollapsed)
	m.config.SetFocusBoardDefaultMode(focusBoardModeValues[m.focusBoardModeCursor])
	m.config.SetDashboardBottomLayout(config.DashboardBottomLayout(m.dashboardLayoutCursor))
	if pct, err := strconv.Atoi(strings.TrimSpace(m.dashboardHorizSplitInput.Value())); err == nil {
		m.config.SetDashboardHorizontalSplitPct(pct)
	}
	if pct, err := strconv.Atoi(strings.TrimSpace(m.dashboardVertSplitInput.Value())); err == nil {
		m.config.SetDashboardVerticalSplitPct(pct)
	}
	if limit, err := strconv.Atoi(strings.TrimSpace(m.resumeCardLimitInput.Value())); err == nil {
		m.config.ResumeCardLimit = &limit
	}
	m.config.TUIZebraStriping = boolPtr(m.zebraStriping)
	m.config.WeekEndDay = weekEndDayValues[m.weekEndDayCursor]
	m.config.TimeFormat = timeFormatValues[m.timeFormatCursor]
	m.config.AutoTheme = boolPtr(m.autoTheme)
	m.config.UseEmoji = boolPtr(m.useEmoji)

	m.config.Stages = append(make([]string, 0, len(m.stages)), m.stages...)

	m.config.AlertDailyReviewEnabled = boolPtr(m.alertDailyReviewEnabled)
	m.config.AlertWeeklyReviewEnabled = boolPtr(m.alertWeeklyReviewEnabled)
	m.config.AlertOverdueEnabled = boolPtr(m.alertOverdueEnabled)
	m.config.AlertDueSoonEnabled = boolPtr(m.alertDueSoonEnabled)
	m.config.AlertStaleEnabled = boolPtr(m.alertStaleEnabled)
	m.config.AlertInboxEnabled = boolPtr(m.alertInboxEnabled)
	m.config.AlertBlockedEnabled = boolPtr(m.alertBlockedEnabled)
	m.config.AlertProjectHealthEnabled = boolPtr(m.alertProjectHealthEnabled)

	alertDailyHour, _ := strconv.Atoi(strings.TrimSpace(m.alertDailyReviewHourInput.Value()))
	m.config.AlertDailyReviewHour = &alertDailyHour
	m.config.AlertWeeklyReviewDay = &m.alertWeeklyReviewDay

	alertDueSoonDays, _ := strconv.Atoi(strings.TrimSpace(m.alertDueSoonDaysInput.Value()))
	m.config.AlertDueSoonDays = &alertDueSoonDays
	alertStaleDays, _ := strconv.Atoi(strings.TrimSpace(m.alertStaleDaysInput.Value()))
	m.config.AlertStaleDays = &alertStaleDays
	alertInboxThreshold, _ := strconv.Atoi(strings.TrimSpace(m.alertInboxThresholdInput.Value()))
	m.config.AlertInboxThreshold = &alertInboxThreshold
	alertBlockedDays, _ := strconv.Atoi(strings.TrimSpace(m.alertBlockedDaysInput.Value()))
	m.config.AlertBlockedDays = &alertBlockedDays
	alertProjectStaleDays, _ := strconv.Atoi(strings.TrimSpace(m.alertProjectStaleDaysInput.Value()))
	m.config.AlertProjectStaleDays = &alertProjectStaleDays

	collapsed := make(map[string]bool)
	for i, name := range outlinerSectionNames {
		if m.outlinerCollapsed[i] {
			collapsed[name] = true
		}
	}
	if len(collapsed) > 0 {
		m.config.OutlinerCollapsedSections = collapsed
	} else {
		m.config.OutlinerCollapsedSections = nil
	}

	// Kanban settings.
	ks := &m.config.KanbanSettings
	ks.DefaultMode = kanbanModeValues[m.kanbanDefaultModeCursor]
	ks.DefaultStartColumn = strings.TrimSpace(m.kanbanStartColInput.Value())
	showEmpty := m.kanbanShowEmptyColumns
	ks.ShowEmptyColumns = &showEmpty
	detailVisible := m.kanbanDetailPaneVisible
	ks.DetailPaneVisible = &detailVisible
	detailHeight := m.kanbanDetailHeightCursor + 1 // convert 0-based cursor to 1-based height
	ks.DetailPaneHeight = &detailHeight
	var enabledModes []string
	allEnabled := true
	for i, on := range m.kanbanEnabledModes {
		if on {
			enabledModes = append(enabledModes, kanbanModeValues[i])
		} else {
			allEnabled = false
		}
	}
	if allEnabled {
		ks.EnabledModes = nil // nil means all enabled (compact JSON)
	} else {
		ks.EnabledModes = enabledModes
	}

	mgr := m.configManager
	cfg := m.config
	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save settings: %w", err)}
		}

		return SettingsSavedMsg{}
	}
}

// priorityModeToCursor converts a config string to cursor index.
// Empty string defaults to 1 (Conditional).
func priorityModeToCursor(mode string) int {
	switch mode {
	case PriorityModeAll:
		return 0
	case PriorityModeHidden:
		return 2
	default:
		return 1
	}
}

// initTreeColumnDefaults converts a config map to a [15]bool array indexed by
// the Project Tree column slice (AllColumns filtered to exclude OutlinerOnly).
// nil map or missing keys default to true (visible).
func initTreeColumnDefaults(cfgMap map[string]bool) [15]bool {
	var result [15]bool
	idx := 0
	for _, col := range AllColumns {
		if col.OutlinerOnly {
			continue
		}
		if cfgMap != nil {
			if v, ok := cfgMap[col.ID]; ok {
				result[idx] = v
				idx++
				continue
			}
		}
		result[idx] = true
		idx++
	}
	return result
}

// initOutlinerColumnDefaults converts a config map to a [15]bool array indexed
// by the full AllColumns slice. nil map or missing keys default to true.
func initOutlinerColumnDefaults(cfgMap map[string]bool) [15]bool {
	var result [15]bool
	for i, col := range AllColumns {
		if cfgMap != nil {
			if v, ok := cfgMap[col.ID]; ok {
				result[i] = v
				continue
			}
		}
		result[i] = true
	}
	return result
}

// treeColumnDefaultsToMap converts a [15]bool array (filtered AllColumns) back
// to a config map. Returns nil when every column is visible.
func treeColumnDefaultsToMap(defaults [15]bool) map[string]bool {
	allTrue := true
	for _, v := range defaults {
		if !v {
			allTrue = false
			break
		}
	}
	if allTrue {
		return nil
	}
	m := make(map[string]bool, 15)
	idx := 0
	for _, col := range AllColumns {
		if col.OutlinerOnly {
			continue
		}
		m[col.ID] = defaults[idx]
		idx++
	}
	return m
}

// outlinerColumnDefaultsToMap converts a [15]bool array (full AllColumns) back
// to a config map. Returns nil when every column is visible.
func outlinerColumnDefaultsToMap(defaults [15]bool) map[string]bool {
	allTrue := true
	for _, v := range defaults {
		if !v {
			allTrue = false
			break
		}
	}
	if allTrue {
		return nil
	}
	m := make(map[string]bool, 15)
	for i, col := range AllColumns {
		m[col.ID] = defaults[i]
	}
	return m
}

// draftToTheme converts the 20-slot inactiveThemeDraft array back to a config.Theme.
// Slot ordering mirrors themeToInactiveDraft in settings_init.go.
func draftToTheme(d [20]string) config.Theme {
	return config.Theme{
		Palette: config.PaletteColors{
			NormalText:     normalizeHexColor(d[0]),
			Muted:          normalizeHexColor(d[1]),
			Primary:        normalizeHexColor(d[2]),
			Secondary:      normalizeHexColor(d[3]),
			Accent:         normalizeHexColor(d[4]),
			Success:        normalizeHexColor(d[5]),
			Warning:        normalizeHexColor(d[6]),
			Error:          normalizeHexColor(d[7]),
			Border:         normalizeHexColor(d[8]),
			Priority1Title: normalizeHexColor(d[12]),
			Priority2Title: normalizeHexColor(d[13]),
			Priority3Title: normalizeHexColor(d[14]),
			Priority4Title: normalizeHexColor(d[15]),
			Priority5Title: normalizeHexColor(d[16]),
			Dimmed:         normalizeHexColor(d[17]),
			DimmedBg:       normalizeHexColor(d[18]),
			DimmedFg:       normalizeHexColor(d[19]),
		},
		BackgroundColor: normalizeHexColor(d[9]),
		CursorColor:     normalizeHexColor(d[10]),
		CursorTextColor: normalizeHexColor(d[11]),
	}
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

func (m *SettingsModel) HasTextInput() bool     { return true }
func (m *SettingsModel) ShouldCaptureEsc() bool { return true }
func (m *SettingsModel) IsFullHeight() bool     { return true }
func (m *SettingsModel) StatusBarLabel() string { return "Settings" }

// HandledKeys returns all keys the Settings view's Update method can process.
// This is the complete set across all tabs and modes (palette presets, stages, etc.).
func (m *SettingsModel) HandledKeys() []string {
	return []string{
		// Field navigation
		"tab", "shift+tab", "shift+left", "shift+right", "shift+up", "shift+down", "space", "enter",
		// Cancel / back
		"esc",
		// Selector / tab navigation
		"h", "l", "left", "right",
		// Search overlay
		"/",
		// Reset focused field to default
		"ctrl+d",
		// Palette preset shortcuts (palette tab)
		"ctrl+s", "ctrl+o", "ctrl+r",
		// Stages tab navigation and actions
		"j", "k", "up", "down",
		"a", "e", "d", "delete", "backspace",
		"J", "K",
		// Palette preset load overlay actions
		"D",
		// Confirmation keys (stage delete, palette reset)
		"y", "Y", "n", "N",
		// Help overlay
		"?",
	}
}

// GetConfirmDialog implements ConfirmDialogProvider for the settings view.
func (m *SettingsModel) GetConfirmDialog() *ConfirmDialog {
	return &m.stageConfirmDelete
}
