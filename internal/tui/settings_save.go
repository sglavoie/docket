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
		BgColor:  normalizeHexColor(m.bgColorInput.Value()),
		CursorBg: normalizeHexColor(m.cursorColorInput.Value()),
		CursorFg: normalizeHexColor(m.cursorTextColorInput.Value()),
		Palette: config.PaletteColors{
			NormalText: normalizeHexColor(m.normalTextInput.Value()),
			Muted:      normalizeHexColor(m.mutedInput.Value()),
			Primary:    normalizeHexColor(m.primaryInput.Value()),
			Secondary:  normalizeHexColor(m.secondaryInput.Value()),
			Accent:     normalizeHexColor(m.accentInput.Value()),
			Success:    normalizeHexColor(m.successInput.Value()),
			Warning:    normalizeHexColor(m.warningInput.Value()),
			Error:      normalizeHexColor(m.errorColorInput.Value()),
			Border:     normalizeHexColor(m.borderInput.Value()),
		},
	}
}

// SetSize sets the view dimensions.
func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SettingsModel) SupportsHelpShortcut() bool { return false }

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
	m.config.TUIBackgroundColor = normalizeHexColor(m.bgColorInput.Value())
	m.config.TUICursorColor = normalizeHexColor(m.cursorColorInput.Value())
	m.config.TUICursorTextColor = normalizeHexColor(m.cursorTextColorInput.Value())
	m.config.Palette = config.PaletteColors{
		NormalText: normalizeHexColor(m.normalTextInput.Value()),
		Muted:      normalizeHexColor(m.mutedInput.Value()),
		Primary:    normalizeHexColor(m.primaryInput.Value()),
		Secondary:  normalizeHexColor(m.secondaryInput.Value()),
		Accent:     normalizeHexColor(m.accentInput.Value()),
		Success:    normalizeHexColor(m.successInput.Value()),
		Warning:    normalizeHexColor(m.warningInput.Value()),
		Error:      normalizeHexColor(m.errorColorInput.Value()),
		Border:     normalizeHexColor(m.borderInput.Value()),
	}
	m.config.DefaultProjectView = defaultProjectViewValues[m.defaultViewCursor]
	m.config.ReviewDueSoonDays = &dueSoon
	m.config.ReviewFutureDays = &futureDays
	m.config.ReviewOldDays = &oldDays
	m.config.ReviewCompletedHours = &completedHours
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
	m.config.TreeDefaultColumns = columnDefaultsToMap(m.treeColumnDefaults)
	m.config.OutlinerDefaultColumns = columnDefaultsToMap(m.outlinerColumnDefaults)

	m.config.TreePriorityMode = priorityModeValues[m.treePriorityMode]
	m.config.OutlinerPriorityMode = priorityModeValues[m.outlinerPriorityMode]
	if m.config.TreePriorityMode == PriorityModeConditional {
		m.config.TreePriorityMode = ""
	}
	if m.config.OutlinerPriorityMode == PriorityModeConditional {
		m.config.OutlinerPriorityMode = ""
	}

	m.config.CrossOutlinerSidebarEnabled = boolPtr(m.crossOutlinerSidebar)

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

// initColumnDefaults converts a config map to a [13]bool array using AllColumns order.
// nil map or missing keys default to true (visible).
func initColumnDefaults(cfgMap map[string]bool) [13]bool {
	var result [13]bool
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

// columnDefaultsToMap converts a [13]bool array back to a config map.
// Returns nil if all columns are visible (to keep config clean).
func columnDefaultsToMap(defaults [13]bool) map[string]bool {
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
	m := make(map[string]bool, 13)
	for i, col := range AllColumns {
		m[col.ID] = defaults[i]
	}
	return m
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

func (m *SettingsModel) HasTextInput() bool     { return true }
func (m *SettingsModel) ShouldCaptureEsc() bool { return true }
func (m *SettingsModel) IsFullHeight() bool     { return false }
func (m *SettingsModel) StatusBarLabel() string { return "Settings" }

// GetConfirmDialog implements ConfirmDialogProvider for the settings view.
func (m *SettingsModel) GetConfirmDialog() *ConfirmDialog {
	return &m.stageConfirmDelete
}
