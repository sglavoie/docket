package tui

import "charm.land/lipgloss/v2"

func rebuildTextStyles() {
	NormalStyle = lipgloss.NewStyle()
	BoldStyle = lipgloss.NewStyle().Bold(true)
	if colorNormalText != "" {
		fg := lipgloss.Color(colorNormalText)
		NormalStyle = NormalStyle.Foreground(fg)
		BoldStyle = BoldStyle.Foreground(fg)
	}

	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ColorSecondary).MarginBottom(1)
	DimStyle = lipgloss.NewStyle().Foreground(ColorDimmed).Faint(true)
	DimmedTaskStyle = lipgloss.NewStyle().
		Background(ColorDimmedBg).
		Foreground(ColorDimmedFg).
		Italic(true)
	DimmedSelectedStyle = lipgloss.NewStyle().
		Background(ColorDimmedBg).
		Foreground(ColorSelectedFg).
		Italic(true)
	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	HelpStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	HelpKeyStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	InfoStatusStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	LoadingStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	ErrorStatusStyle = lipgloss.NewStyle().Foreground(ColorError)
	EmptyErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	SeparatorStyle = lipgloss.NewStyle().Foreground(ColorMuted)
}

func rebuildTaskStyles() {
	TodoStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	InProgressStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	ReviewStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	DoneStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	BlockedStyle = lipgloss.NewStyle().Foreground(ColorError)
	StandbyStyle = lipgloss.NewStyle().Foreground(ColorViewStandby)
	MaybeStyle = lipgloss.NewStyle().Foreground(ColorViewSomeday)
	CanceledStyle = lipgloss.NewStyle().Foreground(ColorMuted).Strikethrough(true)
	OverdueStyle = lipgloss.NewStyle().Foreground(ColorError)
	DueSoonStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	DependencyStyle = lipgloss.NewStyle().Foreground(ColorSecondary)

	if activeThemeName == "light" {
		BlockedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#fde8e8"))
		OverdueRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#fef3e2"))
	} else {
		BlockedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#2e1515"))
		OverdueRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#2d1f0e"))
	}

	DueDateStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	DueTodayStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	blockedDepsFg := lipgloss.Color("#c0caf5")
	if activeThemeName == "light" {
		blockedDepsFg = lipgloss.Color("#ffffff")
	}
	BlockedByDepsStyle = lipgloss.NewStyle().Foreground(blockedDepsFg).Background(ColorError)

	EpicStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	StoryStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	TaskStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	BugStyle = lipgloss.NewStyle().Foreground(ColorError)
	IdeaStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	NoteStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	ActionStyle = lipgloss.NewStyle().Foreground(ColorOrange)

	PinnedProjectStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	PinnedNumberStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	ActiveStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	TriagedBadgeStyle = lipgloss.NewStyle().Foreground(ColorWarning).Italic(true)
	PriorityWaitingHeaderStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1)
	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	PreviewWarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)
}

func rebuildBadgeStyles() {
	EpicBadgeStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	StoryBadgeStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	TaskBadgeStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	BugBadgeStyle = lipgloss.NewStyle().Foreground(ColorError)
	IdeaBadgeStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	NoteBadgeStyle = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorDimmedBg)
	ActionBadgeStyle = lipgloss.NewStyle().Foreground(ColorOrange)

	TodoBadgeStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	InProgressBadgeStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	ReviewBadgeStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	DoneBadgeStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	BlockedBadgeStyle = lipgloss.NewStyle().Foreground(ColorError)
	if activeThemeName == "light" {
		StandbyBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6d28d9"))
	} else {
		StandbyBadgeStyle = lipgloss.NewStyle().Foreground(ColorViewStandby)
	}
	if activeThemeName == "light" {
		MaybeBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6d28d9"))
	} else {
		MaybeBadgeStyle = lipgloss.NewStyle().Foreground(ColorViewSomeday)
	}
	CanceledBadgeStyle = lipgloss.NewStyle().Foreground(ColorMuted).Background(ColorBackground)

	// ColorWarning on light theme is dark brown (#92400e); use white text for
	// readable contrast (~7:1). Dark theme Warning is navy bg color where
	// black text works well.
	warningFg := lipgloss.Color("#1a1b26")
	if activeThemeName == "light" {
		warningFg = lipgloss.Color("15")
	}
	ArchivedBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(warningFg).Padding(0, 1)
	FilterBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(warningFg).Padding(0, 1)
	SavedFilterBadgeStyle = lipgloss.NewStyle().Background(ColorSecondary).Foreground(lipgloss.Color("15")).Bold(true).Padding(0, 1)
	FilterBadgeInactiveStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorBadgeText).Padding(0, 1)
	SectionBadgeStyle = lipgloss.NewStyle().Foreground(ColorBadgeText).Background(ColorBackground)
	ChildrenBadgeStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBackground)
	TagStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBackground).Padding(0, 1)
}

func rebuildLayoutStyles() {
	BoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 2)
	StatusBarStyle = lipgloss.NewStyle().Foreground(ColorHighlight).Background(ColorBackground).Padding(0, 1)
	ProgressFullStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	ProgressEmptyStyle = lipgloss.NewStyle().Foreground(ColorProgressEmpty)
	ScrollbarThumbStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	ScrollbarTrackStyle = lipgloss.NewStyle().Foreground(ColorBorder)
	PrefixPopupStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorLightGray).Padding(0, 1)
	PrefixKeyStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	PrefixDescStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	TaskJumperListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(ColorBorder)
	TaskJumperPreviewStyle = lipgloss.NewStyle().Padding(0, 1)
	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	GroupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginTop(1)
	SectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)

	SuggestionBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent).Padding(0, 1)
	SuggestionHeaderStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	SuggestionReasonStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	SuggestionStarStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	// Derive the tab content tint: 1-shade offset from the effective background.
	bgHex := "#1a1b26"
	if activeThemeName == "light" {
		bgHex = "#f6f6f0"
	}
	if themeBgColor != "" {
		bgHex = themeBgColor
	}
	var tabBgHex string
	if activeThemeName == "light" {
		tabBgHex = darkenHex(bgHex, 0.03)
	} else {
		tabBgHex = lightenHex(bgHex, 0.05)
	}
	TabContentBgStyle = lipgloss.NewStyle().Background(lipgloss.Color(tabBgHex))
}

func rebuildFormStyles() {
	ValidationErrorStyle = lipgloss.NewStyle().Foreground(ColorError).MarginTop(1)
	WarningMessageStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	FieldLabelStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FieldLabelFocusedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	FieldDescriptionStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	SelectorOptionStyle = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	SelectorOptionSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSelectedTextFg).Padding(0, 1)

	FilterViewBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	FilterItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FilterItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedTextFg).Bold(true)
	FilterValueStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	FilterClearStyle = lipgloss.NewStyle().Foreground(ColorError)

	TagManagerBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	TagManagerItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	TagManagerItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedTextFg).Bold(true)
	TagManagerCountStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	TagManagerErrorStyle = lipgloss.NewStyle().Foreground(ColorError)

	SelectorTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	SelectorSearchStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginBottom(1)
	SelectorCountStyle = lipgloss.NewStyle().Foreground(ColorMuted).MarginBottom(1)
	SelectorNoResultsStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1).MarginBottom(1)
	SelectorBaseStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(ColorBorder)
}

func rebuildViewStyles() {
	HelpViewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	HelpCategoryStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginTop(1)
	HelpFilterActiveStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	FocusModeStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Background(ColorBackground)

	ViewModeActiveStyle = makeViewModeStyle("#2e4a30", "#15803d")
	ViewModeWorkStyle = makeViewModeStyle("#4a321e", "#9a3412")
	ViewModeNextStyle = makeViewModeStyle("#2d3a6a", "#3730a3")
	ViewModeInboxStyle = makeViewModeStyle("#1e3a5f", "#1d4ed8")
	ViewModeWaitingStyle = makeViewModeStyle("#5c3a1e", "#92400e")
	ViewModeSomedayStyle = makeViewModeStyle("#3d2a5c", "#6d28d9")
	ViewModeCompletedStyle = makeViewModeStyle("#3b3f5c", "#4b5563")
	ViewModeReviewStyle = makeViewModeStyle("#1e3a3f", "#0f766e")
	ViewModeStandbyStyle = makeViewModeStyle("#4a2e6a", "#7c3aed")
	ViewModeSnoozedStyle = makeViewModeStyle("#4a2e3a", "#7c2d12")
	ViewModeCanceledStyle = makeViewModeStyle("#4a2430", "#991b1b")
	ViewModeAllStyle = makeViewModeStyle("#283352", "#0369a1")
}

func rebuildSelectionStyles() {
	TaskJumperSelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	SelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedTextFg)
	SelectionRowStyle = lipgloss.NewStyle().Background(ColorSelectedBg)
	CursorFlashRowStyle = lipgloss.NewStyle().Background(ColorFlashBg)
	SelectedTaskTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSelectedTextFg)
	SelectionStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	SelectionCheckStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	SelectionCountStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	SelectedCheckboxStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	FocusIndicatorStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	FocusIndicator = FocusIndicatorStyle.Render("▐")
	PinnedMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("·")
}

func rebuildPanelStyles() {
	PanelCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	PanelCardTitleStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	DashboardCardTitleStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	OverlaySplitStyle = lipgloss.NewStyle().Foreground(ColorBorder)
	OverlayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPurple)
	OverlaySelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	OverlayDetailStyle = lipgloss.NewStyle().Foreground(ColorLightGray)
	OverlayBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPurple).Padding(1, 2)
}

func rebuildCategoryStyles() {
	CategoryCommandStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	CategoryTaskStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	CategoryProjectStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryTagStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySectionStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	CategorySavedFilterStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryNoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#73daca"))
	CategoryViewStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
}

func rebuildPriorityTitleStyles() {
	TitlePriority1Style = lipgloss.NewStyle().Foreground(ColorPriority1Title).Bold(true).Italic(true)
	TitlePriority2Style = lipgloss.NewStyle().Foreground(ColorPriority2Title).Bold(true)
	TitlePriority3Style = lipgloss.NewStyle().Foreground(ColorPriority3Title)
	TitlePriority4Style = lipgloss.NewStyle().Foreground(ColorPriority4Title)
	TitlePriority5Style = lipgloss.NewStyle().Foreground(ColorPriority5Title)
}

func rebuildMiscStyles() {
	RuleKeywordStyle = makeColorStyle(ColorRuleKeyword)
	RuleOperatorStyle = makeColorStyle(ColorRuleOperator)
	RuleHighlightStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	highlightFg := lipgloss.Color("#1a1b26")
	if activeThemeName == "light" {
		highlightFg = lipgloss.Color("#ffffff")
	}
	HighlightStyle = lipgloss.NewStyle().Background(ColorHighlight).Foreground(highlightFg).Bold(true)
	MatchHighlightStyle = lipgloss.NewStyle().Foreground(ColorHighlight)
	TagInlineStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CheckmarkStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	WarningIconStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	ErrorIconStyle = lipgloss.NewStyle().Foreground(ColorError)
	SuccessBoldStyle = BoldStyle.Foreground(ColorSuccess)
	BoldPrimaryStyle = BoldStyle.Foreground(ColorPrimary)
	BoldSecondaryStyle = BoldStyle.Foreground(ColorSecondary)
	BoldPrimaryUnderlineStyle = BoldStyle.Foreground(ColorPrimary).Underline(true)
	PurpleStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	alertIndicatorStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
}
