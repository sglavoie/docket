package theme

import "charm.land/lipgloss/v2"

// Manager panel styles.
var (
	SectionBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorBadgeText).
				Background(ColorBackground)

	ChildrenBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Background(ColorBackground)

	PinnedNumberStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	PinnedMarkerStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)

	ActiveStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	TriagedBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Italic(true)

	PriorityWaitingHeaderStyle = lipgloss.NewStyle().
					Foreground(ColorMuted).
					Italic(true).
					MarginTop(1)

	SuggestionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorAccent).
				Padding(0, 1)

	SuggestionHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)

	SuggestionReasonStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true)

	SuggestionStarStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true)

	TitlePriority1Style = lipgloss.NewStyle().Foreground(ColorPriority1Title).Bold(true).Italic(true)
	TitlePriority2Style = lipgloss.NewStyle().Foreground(ColorPriority2Title).Bold(true)
	TitlePriority3Style = lipgloss.NewStyle().Foreground(ColorPriority3Title)

	RuleKeywordStyle   = makeColorStyle(ColorRuleKeyword)
	RuleOperatorStyle  = makeColorStyle(ColorRuleOperator)
	RuleHighlightStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	HighlightStyle = lipgloss.NewStyle().Background(ColorHighlight).Foreground(lipgloss.Color("0")).Bold(true)

	MatchHighlightStyle       = lipgloss.NewStyle().Foreground(ColorHighlight)
	TagInlineStyle            = lipgloss.NewStyle().Foreground(ColorSecondary)
	ContextBadgeStyle         = lipgloss.NewStyle().Foreground(ColorTaskContext).Bold(true)
	CheckmarkStyle            = lipgloss.NewStyle().Foreground(ColorSuccess)
	WarningIconStyle          = lipgloss.NewStyle().Foreground(ColorWarning)
	ErrorIconStyle            = lipgloss.NewStyle().Foreground(ColorError)
	SuccessBoldStyle          = BoldStyle.Foreground(ColorSuccess)
	BoldPrimaryStyle          = BoldStyle.Foreground(ColorPrimary)
	BoldSecondaryStyle        = BoldStyle.Foreground(ColorSecondary)
	BoldPrimaryUnderlineStyle = BoldStyle.Foreground(ColorPrimary).Underline(true)
	PurpleStyle               = lipgloss.NewStyle().Foreground(ColorPurple)

	TagStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Background(ColorBackground).
			Padding(0, 1)

	PanelCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	PanelCardTitleStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// TabContentBgStyle is applied to the content area below tab bars to create
	// a subtle Common Region visual grouping. Rebuilt by rebuildLayoutStyles().
	TabContentBgStyle = lipgloss.NewStyle()
)
