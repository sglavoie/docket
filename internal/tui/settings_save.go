package tui

import (
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

	// Apply the editor's widget values onto copies of the domain snapshots
	// captured when the view opened, so every field the editor does not own
	// (fzf/glow, histories, navigation, cross-outliner state) is preserved.
	s := m.baseSettings
	mach := m.baseMachine

	mach.Editor = editor
	s.DefaultContext = context
	s.UndoHistoryDepth = &undoDepth

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
		s.ThemeLight = activeTheme
		s.ThemeLightName = m.activePresetName
		s.ThemeDark = inactiveTheme
		s.ThemeDarkName = m.inactivePresetName
	} else {
		s.ThemeDark = activeTheme
		s.ThemeDarkName = m.activePresetName
		s.ThemeLight = inactiveTheme
		s.ThemeLightName = m.inactivePresetName
	}
	s.ActiveTheme = m.activeTheme
	s.ReviewDueSoonDays = &dueSoon
	s.ReviewFutureDays = &futureDays
	s.ReviewOldDays = &oldDays
	s.ReviewCompletedHours = &completedHours
	staleTaskDays, _ := strconv.Atoi(strings.TrimSpace(m.staleTaskDaysInput.Value()))
	s.StaleTaskDays = &staleTaskDays
	recentActivityLimit, _ := strconv.Atoi(strings.TrimSpace(m.recentActivityInput.Value()))
	s.RecentActivityLimit = &recentActivityLimit
	s.RecentActivityShowAll = &m.recentActivityShowAll
	s.LinkedTaskJumpToNew = &m.linkedTaskJump
	maxPinned, _ := strconv.Atoi(strings.TrimSpace(m.maxPinnedInput.Value()))
	s.MaxPinnedBarItems = &maxPinned
	maxRecentVisits, _ := strconv.Atoi(strings.TrimSpace(m.maxRecentTaskVisitsInput.Value()))
	s.MaxRecentTaskVisits = &maxRecentVisits
	sectionLimitActive, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitActiveInput.Value()))
	s.SectionLimitActive = &sectionLimitActive
	sectionLimitNext, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitNextInput.Value()))
	s.SectionLimitNext = &sectionLimitNext
	sectionLimitInbox, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitInboxInput.Value()))
	s.SectionLimitInbox = &sectionLimitInbox
	sectionLimitStandby, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitStandbyInput.Value()))
	s.SectionLimitStandby = &sectionLimitStandby
	sectionLimitWaiting, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitWaitingInput.Value()))
	s.SectionLimitWaiting = &sectionLimitWaiting
	sectionLimitSomeday, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitSomedayInput.Value()))
	s.SectionLimitSomeday = &sectionLimitSomeday
	sectionLimitReview, _ := strconv.Atoi(strings.TrimSpace(m.sectionLimitReviewInput.Value()))
	s.SectionLimitReview = &sectionLimitReview
	mach.AttachmentEditor = strings.TrimSpace(m.attachmentEditorInput.Value())
	mach.CodeEditor = strings.TrimSpace(m.codeEditorInput.Value())
	mach.ExternalEditor = strings.TrimSpace(m.externalEditorInput.Value())
	selectedStartView := startViewValues[m.startViewCursor]
	if selectedStartView != mach.GetStartView() {
		mach.SetStartView(selectedStartView)
	} else {
		mach.StartView = selectedStartView
	}
	if limit, err := strconv.Atoi(strings.TrimSpace(m.resumeCardLimitInput.Value())); err == nil {
		s.ResumeCardLimit = &limit
	}
	s.TUIZebraStriping = boolPtr(m.zebraStriping)
	s.WeekEndDay = weekEndDayValues[m.weekEndDayCursor]
	s.TimeFormat = timeFormatValues[m.timeFormatCursor]
	s.AutoTheme = boolPtr(m.autoTheme)
	s.UseEmoji = boolPtr(m.useEmoji)

	s.AlertDailyReviewEnabled = boolPtr(m.alertDailyReviewEnabled)
	s.AlertWeeklyReviewEnabled = boolPtr(m.alertWeeklyReviewEnabled)
	s.AlertOverdueEnabled = boolPtr(m.alertOverdueEnabled)
	s.AlertDueSoonEnabled = boolPtr(m.alertDueSoonEnabled)
	s.AlertStaleEnabled = boolPtr(m.alertStaleEnabled)
	s.AlertInboxEnabled = boolPtr(m.alertInboxEnabled)
	s.AlertBlockedEnabled = boolPtr(m.alertBlockedEnabled)
	s.AlertProjectHealthEnabled = boolPtr(m.alertProjectHealthEnabled)

	alertDailyHour, _ := strconv.Atoi(strings.TrimSpace(m.alertDailyReviewHourInput.Value()))
	s.AlertDailyReviewHour = &alertDailyHour
	s.AlertWeeklyReviewDay = &m.alertWeeklyReviewDay

	alertDueSoonDays, _ := strconv.Atoi(strings.TrimSpace(m.alertDueSoonDaysInput.Value()))
	s.AlertDueSoonDays = &alertDueSoonDays
	alertStaleDays, _ := strconv.Atoi(strings.TrimSpace(m.alertStaleDaysInput.Value()))
	s.AlertStaleDays = &alertStaleDays
	alertInboxThreshold, _ := strconv.Atoi(strings.TrimSpace(m.alertInboxThresholdInput.Value()))
	s.AlertInboxThreshold = &alertInboxThreshold
	alertBlockedDays, _ := strconv.Atoi(strings.TrimSpace(m.alertBlockedDaysInput.Value()))
	s.AlertBlockedDays = &alertBlockedDays
	alertProjectStaleDays, _ := strconv.Atoi(strings.TrimSpace(m.alertProjectStaleDaysInput.Value()))
	s.AlertProjectStaleDays = &alertProjectStaleDays

	// Persist the mutated working copy back into the base snapshots so a
	// subsequent preset save/reset in the same session commits from the
	// just-saved values, then commit only the settings + machine domains.
	m.baseSettings = s
	m.baseMachine = mach
	return m.coordinator.SaveThen(&s, &mach, func() tea.Msg { return SettingsSavedMsg{} })
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
// This is the complete set across all tabs and modes (including palette presets).
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
		// Standard form commit and explicit theme-action menu controls.
		"ctrl+s",
		// Palette preset load overlay actions
		"D",
		// Confirmation keys (palette reset)
		"y", "Y", "n", "N",
		// Help overlay
		"?",
	}
}
