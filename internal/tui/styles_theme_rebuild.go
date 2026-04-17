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
	HeaderBoldStyle = lipgloss.NewStyle().Bold(true)
}

func rebuildTaskStyles() {
	TodoStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	InProgressStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	ReviewStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	DoneStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	BlockedStyle = lipgloss.NewStyle().Foreground(ColorError)
	StandbyStyle = lipgloss.NewStyle().Foreground(ColorViewStandby)
	CanceledStyle = lipgloss.NewStyle().Foreground(ColorMuted).Strikethrough(true)
	OverdueStyle = lipgloss.NewStyle().Foreground(ColorError)
	DueSoonStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	DependencyStyle = lipgloss.NewStyle().Foreground(ColorSecondary)

	PriorityHighStyle = lipgloss.NewStyle().Foreground(ColorError)
	PriorityMediumStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	PriorityLowStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	DueDateStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	DueTodayStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	BlockedByDepsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(ColorError)

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
	PriorityViewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	PriorityWaitingHeaderStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1)
	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	PreviewWarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)
}

func rebuildBadgeStyles() {
	EpicBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	StoryBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	TaskBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	BugBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	IdeaBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	NoteBadgeStyle = makeBadgeStyle("248", "237")
	ActionBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	TodoBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	InProgressBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	ReviewBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	DoneBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	BlockedBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StandbyBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	CanceledBadgeStyle = makeBadgeStyle("248", "236")
	if activeThemeName == "light" {
		StageBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7c3aed"))
	} else {
		StageBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
	}

	ArchivedBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1)
	FilterBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1)
	SavedFilterBadgeStyle = lipgloss.NewStyle().Background(ColorSecondary).Foreground(lipgloss.Color("15")).Bold(true).Padding(0, 1)
	FilterBadgeInactiveStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorBadgeText).Padding(0, 1)
	SectionBadgeStyle = lipgloss.NewStyle().Foreground(ColorBadgeText).Background(ColorBackground)
	ChildrenBadgeStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBackground)
	TagStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBackground).Padding(0, 1)
	SidebarFocusBadgeStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
}

func rebuildLayoutStyles() {
	BoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 2)
	SelectedBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	StatusBarStyle = lipgloss.NewStyle().Foreground(ColorHighlight).Background(ColorBackground).Padding(0, 1)
	TreeBranchStyle = lipgloss.NewStyle().Foreground(ColorBorder)
	ProgressFullStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	ProgressEmptyStyle = lipgloss.NewStyle().Foreground(ColorProgressEmpty)
	ScrollbarThumbStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	ScrollbarTrackStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	ConfirmStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1).MarginTop(1)
	PrefixPopupStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorLightGray).Padding(0, 1)
	PrefixKeyStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	PrefixDescStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	TaskJumperTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	TaskJumperListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(ColorBorder)
	TaskJumperPreviewStyle = lipgloss.NewStyle().Padding(0, 1)
	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	DueDateOverviewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	GroupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginTop(1)
	SectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)

	SuggestionBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent).Padding(0, 1)
	SuggestionHeaderStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	SuggestionReasonStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	SuggestionStarStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
}

func rebuildFormStyles() {
	InputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	InputFocusedStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(0, 1)
	FormInputBorderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorBorder)
	FormInputBorderFocusedStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorPrimary)
	ValidationErrorStyle = lipgloss.NewStyle().Foreground(ColorError).MarginTop(1)
	WarningMessageStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	FieldLabelStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FieldLabelFocusedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	FieldDescriptionStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	SelectorOptionStyle = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	SelectorOptionSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSelectedFg).Padding(0, 1)

	FilterViewBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	FilterItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FilterItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
	FilterValueStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	FilterClearStyle = lipgloss.NewStyle().Foreground(ColorError)

	TagManagerBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	TagManagerItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	TagManagerItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
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

	ViewModeActiveStyle = makeViewModeStyle("34", "#15803d")
	ViewModeNextStyle = makeViewModeStyle("61", "#3730a3")
	ViewModeInboxStyle = makeViewModeStyle("33", "#1d4ed8")
	ViewModeWaitingStyle = makeViewModeStyle("214", "#92400e")
	ViewModeSomedayStyle = makeViewModeStyle("135", "#6d28d9")
	ViewModeCompletedStyle = makeViewModeStyle("136", "#4b5563")
	ViewModeReviewStyle = makeViewModeStyle("30", "#0f766e")
	ViewModeStandbyStyle = makeViewModeStyle("141", "#7c3aed")
	ViewModeSnoozedStyle = makeViewModeStyle("95", "#7c2d12")
	ViewModeAllStyle = makeViewModeStyle("60", "#0369a1")
}

func rebuildSelectionStyles() {
	TaskJumperSelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	SelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)
	SelectionRowStyle = lipgloss.NewStyle().Background(ColorSelectedBg)
	CursorFlashRowStyle = lipgloss.NewStyle().Background(ColorFlashBg)
	SelectedTaskTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSelectedFg)
	SelectionStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	SelectionCheckStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	SelectionCountStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	SelectedCheckboxStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	PinIndicatorStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	FocusIndicatorStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	FocusIndicator = FocusIndicatorStyle.Render("▐")
	PinnedMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("·")
}

func rebuildSidebarStyles() {
	SidebarStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1)
	SidebarHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	SidebarSectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Underline(true)
	SidebarCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	SidebarCardFocusedStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(0, 1)
	SidebarCardTitleStyle = lipgloss.NewStyle().Foreground(ColorMuted)
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
	CategorySavedSearchStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySavedFilterStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryNoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	CategoryViewStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
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
	HighlightStyle = lipgloss.NewStyle().Background(ColorHighlight).Foreground(lipgloss.Color("0")).Bold(true)
	MatchHighlightStyle = lipgloss.NewStyle().Foreground(ColorHighlight)
	TagInlineStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	PriorityInlineStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CheckmarkStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	WarningIconStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	ErrorIconStyle = lipgloss.NewStyle().Foreground(ColorError)
	SuccessBoldStyle = BoldStyle.Foreground(ColorSuccess)
	BoldPrimaryStyle = BoldStyle.Foreground(ColorPrimary)
	BoldSecondaryStyle = BoldStyle.Foreground(ColorSecondary)
	BoldPrimaryUnderlineStyle = BoldStyle.Foreground(ColorPrimary).Underline(true)
	PurpleStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	OrangeStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	DiffAddStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	DiffRemoveStyle = lipgloss.NewStyle().Foreground(ColorError)
	DiffContextStyle = lipgloss.NewStyle().Foreground(ColorDiffContext)
	alertIndicatorStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
}
