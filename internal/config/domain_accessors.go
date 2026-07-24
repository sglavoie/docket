package config

// This file gives the Settings and MachineState domains the same defaulting
// accessors the config.Config facade exposes, so a caller that reads a domain
// directly (via RuntimeState) normalizes nil/invalid values identically to the
// legacy facade. The bodies mirror their (*Config) counterparts field-for-field;
// keeping the default-value semantics on the owning domain — rather than on the
// facade — is what lets the settings editor migrate off config.Config without
// changing any observable default.

// --- Settings: behavior and review windows ---------------------------------

// GetFzfTimeout mirrors (*Config).GetFzfTimeout.
func (s *Settings) GetFzfTimeout() int {
	if s == nil || s.FzfTimeout == nil {
		return 30
	}
	if *s.FzfTimeout <= 0 {
		return 1
	}
	return *s.FzfTimeout
}

// GetReviewDueSoonDays mirrors (*Config).GetReviewDueSoonDays.
func (s *Settings) GetReviewDueSoonDays() int {
	if s.ReviewDueSoonDays == nil {
		return 7
	}
	return *s.ReviewDueSoonDays
}

// GetReviewFutureDays mirrors (*Config).GetReviewFutureDays.
func (s *Settings) GetReviewFutureDays() int {
	if s.ReviewFutureDays == nil {
		return 30
	}
	return *s.ReviewFutureDays
}

// GetReviewOldDays mirrors (*Config).GetReviewOldDays.
func (s *Settings) GetReviewOldDays() int {
	if s.ReviewOldDays == nil {
		return 14
	}
	return *s.ReviewOldDays
}

// GetReviewCompletedHours mirrors (*Config).GetReviewCompletedHours.
func (s *Settings) GetReviewCompletedHours() int {
	if s.ReviewCompletedHours == nil {
		return 24
	}
	return *s.ReviewCompletedHours
}

// GetStaleTaskDays mirrors (*Config).GetStaleTaskDays.
func (s *Settings) GetStaleTaskDays() int {
	if s.StaleTaskDays == nil {
		return 14
	}
	return *s.StaleTaskDays
}

// GetRecentActivityLimit mirrors (*Config).GetRecentActivityLimit.
func (s *Settings) GetRecentActivityLimit() int {
	if s.RecentActivityLimit == nil {
		return 20
	}
	return *s.RecentActivityLimit
}

// GetRecentActivityShowAll mirrors (*Config).GetRecentActivityShowAll.
func (s *Settings) GetRecentActivityShowAll() bool {
	if s.RecentActivityShowAll == nil {
		return true
	}
	return *s.RecentActivityShowAll
}

// GetLinkedTaskJumpToNew mirrors (*Config).GetLinkedTaskJumpToNew.
func (s *Settings) GetLinkedTaskJumpToNew() bool {
	if s.LinkedTaskJumpToNew == nil {
		return false
	}
	return *s.LinkedTaskJumpToNew
}

// GetMaxPinnedBarItems mirrors (*Config).GetMaxPinnedBarItems.
func (s *Settings) GetMaxPinnedBarItems() int {
	if s.MaxPinnedBarItems == nil {
		return DefaultMaxPinnedBarItems
	}
	return *s.MaxPinnedBarItems
}

// GetMaxRecentTaskVisits mirrors (*Config).GetMaxRecentTaskVisits.
func (s *Settings) GetMaxRecentTaskVisits() int {
	if s.MaxRecentTaskVisits == nil || *s.MaxRecentTaskVisits < 1 {
		return DefaultMaxRecentTaskVisits
	}
	if *s.MaxRecentTaskVisits > MaxRecentTaskVisitsUpperBound {
		return MaxRecentTaskVisitsUpperBound
	}
	return *s.MaxRecentTaskVisits
}

// GetSectionLimit mirrors (*Config).GetSectionLimit.
func (s *Settings) GetSectionLimit(section string) int {
	switch section {
	case "":
		return sectionLimitValue(s.SectionLimitActive, DefaultSectionLimitActive)
	case "Next":
		return sectionLimitValue(s.SectionLimitNext, DefaultSectionLimitNext)
	case "Inbox":
		return sectionLimitValue(s.SectionLimitInbox, DefaultSectionLimitInbox)
	case "Standby":
		return sectionLimitValue(s.SectionLimitStandby, DefaultSectionLimitStandby)
	case "Waiting":
		return sectionLimitValue(s.SectionLimitWaiting, DefaultSectionLimitWaiting)
	case "Someday":
		return sectionLimitValue(s.SectionLimitSomeday, DefaultSectionLimitSomeday)
	case "Review":
		return sectionLimitValue(s.SectionLimitReview, DefaultSectionLimitReview)
	default:
		return 0
	}
}

// --- Settings: appearance --------------------------------------------------

// ActiveThemeConfig mirrors (*Config).ActiveThemeConfig.
func (s *Settings) ActiveThemeConfig() Theme {
	if s.ActiveTheme == "light" {
		return s.ThemeLight
	}
	return s.ThemeDark
}

// SetActiveTheme writes a theme to the selected portable settings slot.
func (s *Settings) SetActiveTheme(name string, theme Theme) {
	switch name {
	case "light":
		s.ThemeLight = theme
	case "dark":
		s.ThemeDark = theme
	}
}

// ZebraStriping mirrors (*Config).ZebraStriping.
func (s *Settings) ZebraStriping() bool {
	return s.TUIZebraStriping != nil && *s.TUIZebraStriping
}

// GetAutoTheme mirrors (*Config).GetAutoTheme.
func (s *Settings) GetAutoTheme() bool {
	if s.AutoTheme == nil {
		return true
	}
	return *s.AutoTheme
}

// GetUseEmoji mirrors (*Config).GetUseEmoji.
func (s *Settings) GetUseEmoji() bool {
	return s.UseEmoji != nil && *s.UseEmoji
}

// --- Settings: onboarding --------------------------------------------------

// GetTutorDismissed mirrors (*Config).GetTutorDismissed.
func (s *Settings) GetTutorDismissed() bool {
	if s.TutorDismissed == nil {
		return false
	}
	return *s.TutorDismissed
}

// SetTutorDismissed mirrors (*Config).SetTutorDismissed.
func (s *Settings) SetTutorDismissed(dismissed bool) {
	s.TutorDismissed = &dismissed
}

// GetOnboardingProgress returns a copy of the durable onboarding milestones.
// A workspace written before guided onboarding was introduced starts empty.
func (s *Settings) GetOnboardingProgress() OnboardingProgress {
	if s == nil || s.Onboarding == nil {
		return OnboardingProgress{}
	}
	return *s.Onboarding
}

// SetOnboardingProgress replaces the durable onboarding milestone snapshot.
func (s *Settings) SetOnboardingProgress(progress OnboardingProgress) {
	if s == nil {
		return
	}
	s.Onboarding = &progress
}

// --- Settings: alerts ------------------------------------------------------

// GetAlertDailyReviewEnabled mirrors (*Config).GetAlertDailyReviewEnabled.
func (s *Settings) GetAlertDailyReviewEnabled() bool {
	if s.AlertDailyReviewEnabled == nil {
		return true
	}
	return *s.AlertDailyReviewEnabled
}

// GetAlertWeeklyReviewEnabled mirrors (*Config).GetAlertWeeklyReviewEnabled.
func (s *Settings) GetAlertWeeklyReviewEnabled() bool {
	if s.AlertWeeklyReviewEnabled == nil {
		return true
	}
	return *s.AlertWeeklyReviewEnabled
}

// GetAlertOverdueEnabled mirrors (*Config).GetAlertOverdueEnabled.
func (s *Settings) GetAlertOverdueEnabled() bool {
	if s.AlertOverdueEnabled == nil {
		return true
	}
	return *s.AlertOverdueEnabled
}

// GetAlertDueSoonEnabled mirrors (*Config).GetAlertDueSoonEnabled.
func (s *Settings) GetAlertDueSoonEnabled() bool {
	if s.AlertDueSoonEnabled == nil {
		return true
	}
	return *s.AlertDueSoonEnabled
}

// GetAlertStaleEnabled mirrors (*Config).GetAlertStaleEnabled.
func (s *Settings) GetAlertStaleEnabled() bool {
	if s.AlertStaleEnabled == nil {
		return true
	}
	return *s.AlertStaleEnabled
}

// GetAlertInboxEnabled mirrors (*Config).GetAlertInboxEnabled.
func (s *Settings) GetAlertInboxEnabled() bool {
	if s.AlertInboxEnabled == nil {
		return true
	}
	return *s.AlertInboxEnabled
}

// GetAlertBlockedEnabled mirrors (*Config).GetAlertBlockedEnabled.
func (s *Settings) GetAlertBlockedEnabled() bool {
	if s.AlertBlockedEnabled == nil {
		return true
	}
	return *s.AlertBlockedEnabled
}

// GetAlertProjectHealthEnabled mirrors (*Config).GetAlertProjectHealthEnabled.
func (s *Settings) GetAlertProjectHealthEnabled() bool {
	if s.AlertProjectHealthEnabled == nil {
		return true
	}
	return *s.AlertProjectHealthEnabled
}

// GetAlertDueSoonDays mirrors (*Config).GetAlertDueSoonDays.
func (s *Settings) GetAlertDueSoonDays() int {
	if s.AlertDueSoonDays == nil {
		return DefaultAlertDueSoonDays
	}
	return *s.AlertDueSoonDays
}

// GetAlertStaleDays mirrors (*Config).GetAlertStaleDays.
func (s *Settings) GetAlertStaleDays() int {
	if s.AlertStaleDays == nil {
		return DefaultAlertStaleDays
	}
	return *s.AlertStaleDays
}

// GetAlertInboxThreshold mirrors (*Config).GetAlertInboxThreshold.
func (s *Settings) GetAlertInboxThreshold() int {
	if s.AlertInboxThreshold == nil {
		return DefaultAlertInboxThreshold
	}
	return *s.AlertInboxThreshold
}

// GetAlertBlockedDays mirrors (*Config).GetAlertBlockedDays.
func (s *Settings) GetAlertBlockedDays() int {
	if s.AlertBlockedDays == nil {
		return DefaultAlertBlockedDays
	}
	return *s.AlertBlockedDays
}

// GetAlertProjectStaleDays mirrors (*Config).GetAlertProjectStaleDays.
func (s *Settings) GetAlertProjectStaleDays() int {
	if s.AlertProjectStaleDays == nil {
		return DefaultAlertProjectStaleDays
	}
	return *s.AlertProjectStaleDays
}

// GetAlertDailyReviewHour mirrors (*Config).GetAlertDailyReviewHour.
func (s *Settings) GetAlertDailyReviewHour() int {
	if s.AlertDailyReviewHour == nil {
		return DefaultAlertDailyReviewHour
	}
	v := *s.AlertDailyReviewHour
	if v < 0 || v > 23 {
		return DefaultAlertDailyReviewHour
	}
	return v
}

// GetAlertWeeklyReviewDay mirrors (*Config).GetAlertWeeklyReviewDay.
func (s *Settings) GetAlertWeeklyReviewDay() int {
	if s.AlertWeeklyReviewDay == nil {
		return DefaultAlertWeeklyReviewDay
	}
	v := *s.AlertWeeklyReviewDay
	if v < 0 || v > 6 {
		return DefaultAlertWeeklyReviewDay
	}
	return v
}

// --- MachineState: editors, navigation, and layout -------------------------

// GetEditor mirrors (*Config).GetEditor. A nil machine snapshot uses the same
// editor default as NewMachineState so isolated domain-native views remain
// usable without a facade.
func (m *MachineState) GetEditor() string {
	if m == nil {
		return "code"
	}
	return m.Editor
}

// GetExternalEditor returns the GUI editor used for task/note reference
// snapshots, defaulting to VS Code when unset.
func (m *MachineState) GetExternalEditor() string {
	if m != nil && m.ExternalEditor != "" {
		return m.ExternalEditor
	}
	return "code"
}

// GetAttachmentEditor mirrors (*Config).GetAttachmentEditor. An empty value
// means the attachment flow should use the operating-system default opener.
func (m *MachineState) GetAttachmentEditor() string {
	if m == nil {
		return ""
	}
	return m.AttachmentEditor
}

// GetCodeEditor mirrors (*Config).GetCodeEditor: an explicit code editor wins,
// then the general editor, then the VS Code default.
func (m *MachineState) GetCodeEditor() string {
	if m != nil {
		if m.CodeEditor != "" {
			return m.CodeEditor
		}
		if m.Editor != "" {
			return m.Editor
		}
	}
	return "code"
}
