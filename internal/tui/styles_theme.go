package tui

import (
	"fmt"
	"image/color"
	"strconv"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
)

// Default cursor/selection colors.
const (
	DefaultSelectedBg = "#4c0163"
	DefaultSelectedFg = "#fbeeff"
	DefaultFlashBg    = "#7b2fa0"
)

// ThemeConfig bundles all user-configurable theme values for ApplyTheme.
type ThemeConfig struct {
	BgColor  string
	CursorBg string
	CursorFg string
	Palette  config.PaletteColors
}

// NewThemeConfig constructs a ThemeConfig from the application config.
func NewThemeConfig(cfg *config.Config) ThemeConfig {
	return ThemeConfig{
		BgColor:  cfg.TUIBackgroundColor,
		CursorBg: cfg.TUICursorColor,
		CursorFg: cfg.TUICursorTextColor,
		Palette:  cfg.Palette,
	}
}

// themeBgColor holds the current TUI background color (empty = none/terminal default).
var themeBgColor string

// colorNormalText tracks whether NormalText palette override is set.
var colorNormalText string

// ThemeSpacer renders an empty block of the given width, filled with the theme
// background color when set. Used to plug "holes" in horizontal joins.
func ThemeSpacer(width int) string {
	s := lipgloss.NewStyle().Width(width)
	if themeBgColor != "" {
		s = s.Background(lipgloss.Color(themeBgColor))
	}
	return s.Render("")
}

// lightenHex takes a 6-digit hex color (#rrggbb) and lightens each channel
// toward 255 by factor (0.0 = unchanged, 1.0 = white). Returns #rrggbb.
func lightenHex(hex string, factor float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return hex
	}
	r, err1 := strconv.ParseInt(hex[1:3], 16, 64)
	g, err2 := strconv.ParseInt(hex[3:5], 16, 64)
	b, err3 := strconv.ParseInt(hex[5:7], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return hex
	}
	lighten := func(channel int64) int64 {
		return channel + int64(float64(255-channel)*factor)
	}
	return fmt.Sprintf("#%02x%02x%02x", lighten(r), lighten(g), lighten(b))
}

// rebuildPalette resets the palette color variables from user overrides.
// Empty fields fall back to ANSI defaults.
func rebuildPalette(p config.PaletteColors) {
	if p.Primary != "" {
		ColorPrimary = lipgloss.Color(p.Primary)
	} else {
		ColorPrimary = lipgloss.Color("39")
	}
	if p.Secondary != "" {
		ColorSecondary = lipgloss.Color(p.Secondary)
	} else {
		ColorSecondary = lipgloss.Color("33")
	}
	if p.Success != "" {
		ColorSuccess = lipgloss.Color(p.Success)
	} else {
		ColorSuccess = lipgloss.Color("42")
	}
	if p.Warning != "" {
		ColorWarning = lipgloss.Color(p.Warning)
	} else {
		ColorWarning = lipgloss.Color("214")
	}
	if p.Error != "" {
		ColorError = lipgloss.Color(p.Error)
	} else {
		ColorError = lipgloss.Color("196")
	}
	if p.Muted != "" {
		ColorMuted = lipgloss.Color(p.Muted)
	} else {
		ColorMuted = lipgloss.Color("248")
	}
	if p.Dimmed != "" {
		ColorDimmed = lipgloss.Color(p.Dimmed)
	} else {
		ColorDimmed = lipgloss.Color("240")
	}
	if p.DimmedBg != "" {
		ColorDimmedBg = lipgloss.Color(p.DimmedBg)
	} else {
		ColorDimmedBg = lipgloss.Color("237")
	}
	if p.DimmedFg != "" {
		ColorDimmedFg = lipgloss.Color(p.DimmedFg)
	} else {
		ColorDimmedFg = lipgloss.Color("250")
	}
	if p.Border != "" {
		ColorBorder = lipgloss.Color(p.Border)
	} else {
		ColorBorder = lipgloss.Color("244")
	}
	if p.Accent != "" {
		ColorHighlight = lipgloss.Color(p.Accent)
		ColorAccent = lipgloss.Color(p.Accent)
	} else {
		ColorHighlight = lipgloss.Color("#f2eb16")
		ColorAccent = lipgloss.Color("226")
	}

	ColorBadgeText = ColorMuted
	ColorLightGray = ColorMuted
	ColorDiffContext = ColorMuted
	ColorSubtleBorder = ColorBorder

	colorNormalText = p.NormalText
}

// rebuildStyles reassigns every style variable using the current palette colors.
// This must be called after rebuildPalette so styles pick up user overrides.
func rebuildStyles() {
	NormalStyle = lipgloss.NewStyle()
	BoldStyle = lipgloss.NewStyle().Bold(true)
	if colorNormalText != "" {
		fg := lipgloss.Color(colorNormalText)
		NormalStyle = NormalStyle.Foreground(fg)
		BoldStyle = BoldStyle.Foreground(fg)
	}

	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ColorSecondary).MarginBottom(1)
	DimStyle = lipgloss.NewStyle().Faint(true)
	DimmedTaskStyle = lipgloss.NewStyle().
		Background(ColorDimmedBg).
		Foreground(ColorDimmedFg).
		Italic(true)
	DimmedSelectedStyle = lipgloss.NewStyle().
		Background(ColorDimmedBg).
		Foreground(ColorSelectedFg).
		Italic(true)
	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)

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

	EpicBadgeStyle = makeBadgeStyle("135", "53")
	StoryBadgeStyle = makeBadgeStyle("39", "17")
	TaskBadgeStyle = makeBadgeStyle("42", "22")
	BugBadgeStyle = makeBadgeStyle("196", "52")
	IdeaBadgeStyle = makeBadgeStyle("214", "94")
	NoteBadgeStyle = makeBadgeStyle("248", "237")
	ActionBadgeStyle = makeBadgeStyle("208", "94")

	TodoBadgeStyle = makeBadgeStyle("248", "236")
	InProgressBadgeStyle = makeBadgeStyle("214", "94")
	ReviewBadgeStyle = makeBadgeStyle("33", "17")
	DoneBadgeStyle = makeBadgeStyle("42", "22")
	BlockedBadgeStyle = makeBadgeStyle("196", "52")
	StandbyBadgeStyle = makeBadgeStyle("141", "53")
	CanceledBadgeStyle = makeBadgeStyle("248", "236")
	StageBadgeStyle = makeBadgeStyle("183", "236")

	BoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 2)
	SelectedBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)

	HelpStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	HelpKeyStyle = lipgloss.NewStyle().Foreground(ColorSecondary)

	StatusBarStyle = lipgloss.NewStyle().Foreground(ColorHighlight).Background(ColorBackground).Padding(0, 1)

	TreeBranchStyle = lipgloss.NewStyle().Foreground(ColorBorder)

	ProgressFullStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	ProgressEmptyStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	ScrollbarThumbStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	ScrollbarTrackStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	ArchivedBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1)

	ConfirmStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1).MarginTop(1)

	PrefixPopupStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorLightGray).Padding(0, 1)
	PrefixKeyStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	PrefixDescStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	InputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	InputFocusedStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(0, 1)
	FormInputBorderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorBorder)
	FormInputBorderFocusedStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorPrimary)

	ValidationErrorStyle = lipgloss.NewStyle().Foreground(ColorError).MarginTop(1)
	WarningMessageStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	FieldLabelStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FieldLabelFocusedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	FieldDescriptionStyle = lipgloss.NewStyle().Foreground(ColorMuted).Faint(true)

	SelectorOptionStyle = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	SelectorOptionSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSelectedFg).Padding(0, 1)

	SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	InfoStatusStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	LoadingStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	ErrorStatusStyle = lipgloss.NewStyle().Foreground(ColorError)

	HelpViewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	HelpCategoryStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginTop(1)
	HelpFilterActiveStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	FocusModeStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Background(ColorBackground)

	ViewModeActiveStyle = makeViewModeStyle("34")
	ViewModeNextStyle = makeViewModeStyle("61")
	ViewModeInboxStyle = makeViewModeStyle("33")
	ViewModeWaitingStyle = makeViewModeStyle("214")
	ViewModeSomedayStyle = makeViewModeStyle("135")
	ViewModeCompletedStyle = makeViewModeStyle("136")
	ViewModeReviewStyle = makeViewModeStyle("30")
	ViewModeStandbyStyle = makeViewModeStyle("141")
	ViewModeSnoozedStyle = makeViewModeStyle("95")
	ViewModeAllStyle = makeViewModeStyle("60")

	FilterBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1)
	SavedFilterBadgeStyle = lipgloss.NewStyle().Background(ColorSecondary).Foreground(lipgloss.Color("15")).Bold(true).Padding(0, 1)
	FilterBadgeInactiveStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorBadgeText).Padding(0, 1)
	FilterViewBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	FilterItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FilterItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
	FilterValueStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	FilterClearStyle = lipgloss.NewStyle().Foreground(ColorError)

	TaskJumperTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	TaskJumperListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(ColorBorder)
	TaskJumperPreviewStyle = lipgloss.NewStyle().Padding(0, 1)
	TaskJumperSelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	DueDateOverviewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	GroupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginTop(1)
	SectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)

	SelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)
	SelectionRowStyle = lipgloss.NewStyle().Background(ColorSelectedBg)
	CursorFlashRowStyle = lipgloss.NewStyle().Background(ColorFlashBg)
	SelectedTaskTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSelectedFg)

	SelectionStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	SelectionCheckStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	SelectionCountStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

	TagManagerBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	TagManagerItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	TagManagerItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
	TagManagerCountStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	TagManagerErrorStyle = lipgloss.NewStyle().Foreground(ColorError)

	SectionBadgeStyle = lipgloss.NewStyle().Foreground(ColorBadgeText).Background(ColorBackground)
	ChildrenBadgeStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBackground)

	PinnedProjectStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	PinnedNumberStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

	ActiveStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	TriagedBadgeStyle = lipgloss.NewStyle().Foreground(ColorWarning).Italic(true)

	PriorityViewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	PriorityWaitingHeaderStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1)

	SuggestionBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent).Padding(0, 1)
	SuggestionHeaderStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	SuggestionReasonStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	SuggestionStarStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	Priority1Style = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	Priority2Style = makeColorStyle(ColorWarning)
	Priority3Style = makeColorStyle(ColorPrimary)
	Priority4Style = makeColorStyle(ColorMuted)
	Priority5Style = makeColorStyle(ColorMuted).Faint(true)

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

	FocusIndicatorStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	FocusIndicator = FocusIndicatorStyle.Render("▐")

	PinnedMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("·")

	SidebarStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1)
	SidebarHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	SidebarSectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Underline(true)
	TagStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Background(ColorBackground).Padding(0, 1)
	SidebarFocusBadgeStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	SidebarCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	SidebarCardFocusedStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(0, 1)
	SidebarCardTitleStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	DashboardCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	DashboardCardTitleStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)

	SelectorTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	SelectorSearchStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginBottom(1)
	SelectorCountStyle = lipgloss.NewStyle().Foreground(ColorMuted).MarginBottom(1)
	SelectorNoResultsStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1).MarginBottom(1)
	SelectorBaseStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(ColorBorder)

	EmptyErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	SeparatorStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	CategoryCommandStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	CategoryTaskStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	CategoryProjectStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryTagStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySectionStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	CategorySavedSearchStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySavedFilterStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryNoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	CategoryViewStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

	SelectedCheckboxStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	PinIndicatorStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	HeaderBoldStyle = lipgloss.NewStyle().Bold(true)
	PreviewWarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	OverlaySplitStyle = lipgloss.NewStyle().Foreground(ColorBorder)
	OverlayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPurple)
	OverlaySelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	OverlayDetailStyle = lipgloss.NewStyle().Foreground(ColorLightGray)
	OverlayBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPurple).Padding(1, 2)

	DiffAddStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	DiffRemoveStyle = lipgloss.NewStyle().Foreground(ColorError)
	DiffContextStyle = lipgloss.NewStyle().Foreground(ColorDiffContext)

	alertIndicatorStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
}

// ApplyTheme reconfigures cursor/selection colors and background propagation.
// Call at startup and after settings save.
func ApplyTheme(tc ThemeConfig) {
	rebuildPalette(tc.Palette)
	rebuildStyles()
	buildViewAccentMap()

	effectiveBg := tc.CursorBg
	if effectiveBg == "" {
		effectiveBg = DefaultSelectedBg
	}
	effectiveFg := tc.CursorFg
	if effectiveFg == "" {
		effectiveFg = DefaultSelectedFg
	}
	effectiveFlash := lightenHex(effectiveBg, 0.4)

	ColorSelectedBg = lipgloss.Color(effectiveBg)
	ColorSelectedFg = lipgloss.Color(effectiveFg)
	ColorFlashBg = lipgloss.Color(effectiveFlash)

	SelectedStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg).
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectionRowStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg)

	CursorFlashRowStyle = lipgloss.NewStyle().
		Background(ColorFlashBg)

	TaskJumperSelectedStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg).
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)

	TagManagerItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedFg).
		Bold(true)

	FilterItemSelectedStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedFg).
		Padding(0, 1)

	DimmedSelectedStyle = lipgloss.NewStyle().
		Background(ColorDimmedBg).
		Foreground(ColorSelectedFg).
		Italic(true)

	SelectedTaskTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSelectedFg)

	themeBgColor = tc.BgColor

	var customBg color.Color
	alwaysBg := ColorBackground
	if tc.BgColor != "" {
		customBg = lipgloss.Color(tc.BgColor)
		alwaysBg = customBg
	}

	StatusBarStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorHighlight).
		Padding(0, 1), alwaysBg)

	PrefixPopupStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorLightGray).
		Padding(0, 1), alwaysBg)

	FocusModeStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true), alwaysBg)

	FilterBadgeInactiveStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorBadgeText).
		Padding(0, 1), alwaysBg)

	TagStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Padding(0, 1), alwaysBg)

	SectionBadgeStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorBadgeText), alwaysBg)

	ChildrenBadgeStyle = withOptionalBg(lipgloss.NewStyle().
		Foreground(ColorPrimary), alwaysBg)

	DashboardCardStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1), customBg)

	SidebarCardStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1), customBg)

	SidebarCardFocusedStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1), customBg)

	SidebarStyle = withOptionalBg(lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1), customBg)

	TaskJumperListStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(ColorBorder), customBg)

	TaskJumperPreviewStyle = withOptionalBg(lipgloss.NewStyle().
		Padding(0, 1), customBg)

	OverlayBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPurple).
		Padding(1, 2), customBg)

	TagManagerBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2), customBg)

	SelectorBaseStyle = withOptionalBg(lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder), customBg)

	FilterViewBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2), customBg)

	SuggestionBoxStyle = withOptionalBg(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 1), customBg)
}

// withOptionalBg applies a background color to a style if bg is non-nil.
func withOptionalBg(s lipgloss.Style, bg color.Color) lipgloss.Style {
	if bg != nil {
		return s.Background(bg)
	}
	return s
}

// makeBadgeStyle creates a badge style with the given foreground and background ANSI colors.
func makeBadgeStyle(fg, bg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(fg)).
		Background(lipgloss.Color(bg))
}

// makeViewModeStyle creates a view mode tab style with the given background ANSI color.
func makeViewModeStyle(bg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Padding(0, 1)
}

// makeColorStyle creates a style with just a foreground color.
func makeColorStyle(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}
