package tui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

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
	lighten := func(c int64) int64 {
		return c + int64(float64(255-c)*factor)
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
	// Derived colors
	ColorBadgeText = ColorMuted
	ColorLightGray = ColorMuted
	ColorDiffContext = ColorMuted
	ColorSubtleBorder = ColorBorder

	colorNormalText = p.NormalText
}

// rebuildStyles reassigns every style variable using the current palette colors.
// This must be called after rebuildPalette so styles pick up user overrides.
func rebuildStyles() {
	// --- Text styles ---
	NormalStyle = lipgloss.NewStyle()
	BoldStyle = lipgloss.NewStyle().Bold(true)
	if colorNormalText != "" {
		fg := lipgloss.Color(colorNormalText)
		NormalStyle = NormalStyle.Foreground(fg)
		BoldStyle = BoldStyle.Foreground(fg)
	}

	// Title styles
	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ColorSecondary).MarginBottom(1)
	DimStyle = lipgloss.NewStyle().Faint(true)
	MutedStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	// Status colors
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

	// Priority styles
	PriorityHighStyle = lipgloss.NewStyle().Foreground(ColorError)
	PriorityMediumStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	PriorityLowStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	DueDateStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	DueTodayStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	BlockedByDepsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(ColorError)

	// Task type colors
	EpicStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	StoryStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	TaskStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	BugStyle = lipgloss.NewStyle().Foreground(ColorError)
	IdeaStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	NoteStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	ActionStyle = lipgloss.NewStyle().Foreground(ColorOrange)

	// Task type badge styles
	EpicBadgeStyle = makeBadgeStyle("135", "53")
	StoryBadgeStyle = makeBadgeStyle("39", "17")
	TaskBadgeStyle = makeBadgeStyle("42", "22")
	BugBadgeStyle = makeBadgeStyle("196", "52")
	IdeaBadgeStyle = makeBadgeStyle("214", "94")
	NoteBadgeStyle = makeBadgeStyle("248", "237")
	ActionBadgeStyle = makeBadgeStyle("208", "94")

	// Task status badge styles
	TodoBadgeStyle = makeBadgeStyle("248", "236")
	InProgressBadgeStyle = makeBadgeStyle("214", "94")
	ReviewBadgeStyle = makeBadgeStyle("33", "17") // ColorSecondary
	DoneBadgeStyle = makeBadgeStyle("42", "22")
	BlockedBadgeStyle = makeBadgeStyle("196", "52")
	StandbyBadgeStyle = makeBadgeStyle("141", "53")
	CanceledBadgeStyle = makeBadgeStyle("248", "236")
	StageBadgeStyle = makeBadgeStyle("183", "236")

	// Container styles
	BoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(1, 2)
	SelectedBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)

	// Help bar
	HelpStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	HelpKeyStyle = lipgloss.NewStyle().Foreground(ColorSecondary)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().Foreground(ColorHighlight).Background(ColorBackground).Padding(0, 1)

	// Tree view
	TreeBranchStyle = lipgloss.NewStyle().Foreground(ColorBorder)

	// Progress bar colors
	ProgressFullStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	ProgressEmptyStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	// Scrollbar styles
	ScrollbarThumbStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	ScrollbarTrackStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	// Badge styles
	ArchivedBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1)

	// Confirmation prompt style
	ConfirmStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1).MarginTop(1)

	// Prefix key popup
	PrefixPopupStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorLightGray).Padding(0, 1)
	PrefixKeyStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	PrefixDescStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	// Form input styles
	InputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Padding(0, 1)
	InputFocusedStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(0, 1)
	FormInputBorderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorBorder)
	FormInputBorderFocusedStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ColorPrimary)

	// Validation/warning
	ValidationErrorStyle = lipgloss.NewStyle().Foreground(ColorError).MarginTop(1)
	WarningMessageStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Field label styles
	FieldLabelStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FieldLabelFocusedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	FieldDescriptionStyle = lipgloss.NewStyle().Foreground(ColorMuted).Faint(true)

	// Selector option styles
	SelectorOptionStyle = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	SelectorOptionSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSelectedFg).Padding(0, 1)

	// Success/info/loading/error status styles
	SuccessStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	InfoStatusStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	LoadingStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	ErrorStatusStyle = lipgloss.NewStyle().Foreground(ColorError)

	// Help view styles
	HelpViewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	HelpCategoryStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginTop(1)
	HelpFilterActiveStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Focus mode style
	FocusModeStyle = lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Background(ColorBackground)

	// View mode styles
	ViewModeActiveStyle = makeViewModeStyle("34")   // ColorViewActive
	ViewModeNextStyle = makeViewModeStyle("61")     // ColorViewNext
	ViewModeInboxStyle = makeViewModeStyle("33")    // ColorSecondary
	ViewModeWaitingStyle = makeViewModeStyle("214") // ColorWarning
	ViewModeSomedayStyle = makeViewModeStyle("135") // ColorViewSomeday
	ViewModeCompletedStyle = makeViewModeStyle("136")
	ViewModeReviewStyle = makeViewModeStyle("30")   // ColorViewReview
	ViewModeStandbyStyle = makeViewModeStyle("141") // ColorViewStandby
	ViewModeSnoozedStyle = makeViewModeStyle("95")
	ViewModeAllStyle = makeViewModeStyle("60")

	// Filter view styles
	FilterBadgeStyle = lipgloss.NewStyle().Background(ColorWarning).Foreground(lipgloss.Color("0")).Padding(0, 1)
	SavedFilterBadgeStyle = lipgloss.NewStyle().Background(ColorSecondary).Foreground(lipgloss.Color("15")).Bold(true).Padding(0, 1)
	FilterBadgeInactiveStyle = lipgloss.NewStyle().Background(ColorBackground).Foreground(ColorBadgeText).Padding(0, 1)
	FilterViewBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	FilterItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	FilterItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
	FilterValueStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	FilterClearStyle = lipgloss.NewStyle().Foreground(ColorError)

	// Task jumper styles
	TaskJumperTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	TaskJumperListStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(ColorBorder)
	TaskJumperPreviewStyle = lipgloss.NewStyle().Padding(0, 1)
	TaskJumperSelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	// Due date overview styles
	DueDateOverviewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	GroupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginTop(1)
	SectionHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)

	// Selection styles
	SelectedStyle = lipgloss.NewStyle().Background(ColorSelectedBg).Foreground(ColorSelectedFg).Bold(true)
	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)
	SelectionCursorStyle = lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
	SelectionRowStyle = lipgloss.NewStyle().Background(ColorSelectedBg)
	CursorFlashRowStyle = lipgloss.NewStyle().Background(ColorFlashBg)

	// Multi-select styles
	SelectionStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	SelectionCheckStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	SelectionCountStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

	// Tag manager styles
	TagManagerBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPrimary).Padding(1, 2)
	TagManagerItemStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	TagManagerItemSelectedStyle = lipgloss.NewStyle().Foreground(ColorSelectedFg).Bold(true)
	TagManagerCountStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	TagManagerErrorStyle = lipgloss.NewStyle().Foreground(ColorError)

	// Section badge styles
	SectionBadgeStyle = lipgloss.NewStyle().Foreground(ColorBadgeText).Background(ColorBackground)
	ChildrenBadgeStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Background(ColorBackground)

	// Pinned project styles
	PinnedProjectStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	PinnedNumberStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

	// Active/triaged styles
	ActiveStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	TriagedBadgeStyle = lipgloss.NewStyle().Foreground(ColorWarning).Italic(true)

	// Priority view styles
	PriorityViewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	PriorityWaitingHeaderStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1)

	// Suggestion styles
	SuggestionBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent).Padding(0, 1)
	SuggestionHeaderStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	SuggestionReasonStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	SuggestionStarStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	// Priority level styles
	Priority1Style = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	Priority2Style = makeColorStyle(ColorWarning)
	Priority3Style = makeColorStyle(ColorPrimary)
	Priority4Style = makeColorStyle(ColorMuted)
	Priority5Style = makeColorStyle(ColorMuted).Faint(true)

	// Workflow rule syntax highlighting
	RuleKeywordStyle = makeColorStyle(ColorRuleKeyword)
	RuleOperatorStyle = makeColorStyle(ColorRuleOperator)
	RuleHighlightStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	// Highlight style
	HighlightStyle = lipgloss.NewStyle().Background(ColorHighlight).Foreground(lipgloss.Color("0")).Bold(true)

	// Common inline styles
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

	// Focus indicator (pre-rendered)
	FocusIndicatorStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	FocusIndicator = FocusIndicatorStyle.Render("▐")

	// Pinned marker (pre-rendered)
	PinnedMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("·")

	// Sidebar styles
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

	// Selector and list styles
	SelectorTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).MarginBottom(1)
	SelectorSearchStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).MarginBottom(1)
	SelectorCountStyle = lipgloss.NewStyle().Foreground(ColorMuted).MarginBottom(1)
	SelectorNoResultsStyle = lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).MarginTop(1).MarginBottom(1)
	SelectorBaseStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(ColorBorder)

	// Empty/error state
	EmptyErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	SeparatorStyle = lipgloss.NewStyle().Foreground(ColorMuted)

	// Category badge styles
	CategoryCommandStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	CategoryTaskStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	CategoryProjectStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryTagStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySectionStyle = lipgloss.NewStyle().Foreground(ColorPurple)
	CategorySavedSearchStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySavedFilterStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryNoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	CategoryViewStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

	// Selection indicators
	SelectedCheckboxStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	PinIndicatorStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Due date urgency
	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	HeaderBoldStyle = lipgloss.NewStyle().Bold(true)
	PreviewWarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Overlay styles
	OverlaySplitStyle = lipgloss.NewStyle().Foreground(ColorBorder)
	OverlayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPurple)
	OverlaySelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	OverlayDetailStyle = lipgloss.NewStyle().Foreground(ColorLightGray)
	OverlayBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorPurple).Padding(1, 2)

	// Diff styles
	DiffAddStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	DiffRemoveStyle = lipgloss.NewStyle().Foreground(ColorError)
	DiffContextStyle = lipgloss.NewStyle().Foreground(ColorDiffContext)

	// Alert indicator (defined in alert_indicator.go)
	alertIndicatorStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
}

// ApplyTheme reconfigures cursor/selection colors and background propagation.
// Call at startup and after settings save.
func ApplyTheme(tc ThemeConfig) {
	// 1. Rebuild palette colors from user overrides
	rebuildPalette(tc.Palette)

	// 2. Rebuild ALL styles using updated palette
	rebuildStyles()

	// 3. Cursor colors (always)
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

	// Rebuild cursor-derived styles
	SelectedStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg).
		Foreground(ColorSelectedFg).
		Bold(true)

	SelectionRowStyle = lipgloss.NewStyle().
		Background(ColorSelectedBg)

	CursorFlashRowStyle = lipgloss.NewStyle().
		Background(ColorFlashBg)

	SelectionCursorStyle = lipgloss.NewStyle().
		Foreground(ColorSelectedFg).
		Bold(true)

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

	// 4. Background propagation
	themeBgColor = tc.BgColor

	// customBg is non-nil only when the user has set a custom background color.
	// alwaysBg is the background used for styles that always have one (custom or the
	// default dark terminal background).
	var customBg color.Color
	alwaysBg := ColorBackground
	if tc.BgColor != "" {
		customBg = lipgloss.Color(tc.BgColor)
		alwaysBg = customBg
	}

	// Styles that always carry a background (custom or ColorBackground).
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

	// Container styles: background only when a custom color is configured.
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

// Color palette
var (
	ColorPrimary     = lipgloss.Color("39")      // Blue
	ColorSecondary   = lipgloss.Color("33")      // Cyan
	ColorSuccess     = lipgloss.Color("42")      // Green
	ColorWarning     = lipgloss.Color("214")     // Orange
	ColorError       = lipgloss.Color("196")     // Red
	ColorMuted       = lipgloss.Color("248")     // Gray
	ColorHighlight   = lipgloss.Color("#f2eb16") // Bright yellow
	ColorBackground  = lipgloss.Color("236")     // Dark gray
	ColorBorder      = lipgloss.Color("244")     // Border gray
	ColorDiffContext = lipgloss.Color("249")     // Diff context gray
	ColorBadgeText   = lipgloss.Color("249")     // Gray text for badges/inactive elements
	ColorAccent      = lipgloss.Color("226")     // Bright yellow accent
	ColorSelectedBg  = lipgloss.Color("#4c0163") // Light purple background for selections

	// View mode section colors (shared between tab bar and tutor highlighting)
	ColorViewActive   = lipgloss.Color("34")      // Blue-green
	ColorViewNext     = lipgloss.Color("61")      // Slate blue
	ColorViewReview   = lipgloss.Color("30")      // Teal (distinct from Inbox's cyan)
	ColorViewStandby  = lipgloss.Color("141")     // Violet
	ColorViewSomeday  = lipgloss.Color("135")     // Purple
	ColorSelectedFg   = lipgloss.Color("#fbeeff") // Dark purple foreground for selections
	ColorFlashBg      = lipgloss.Color("#7b2fa0") // Brighter purple for cursor flash highlight
	ColorLightGray    = lipgloss.Color("252")     // Light gray for normal text in overlays
	ColorSubtleBorder = lipgloss.Color("240")     // Subtle border/muted text (darker than ColorBorder)
	ColorPurple       = lipgloss.Color("135")     // Purple (epics, someday, category sections)
	ColorOrange       = lipgloss.Color("208")     // Orange (actions, estimated scores)

	// TaskCountColorScale is a 10-step gradient from red (low) to green (high).
	TaskCountColorScale = [10]color.Color{
		lipgloss.Color("196"), // Red
		lipgloss.Color("202"), // Red-orange
		lipgloss.Color("208"), // Orange
		lipgloss.Color("214"), // Orange-yellow
		lipgloss.Color("220"), // Yellow
		lipgloss.Color("226"), // Yellow-green
		lipgloss.Color("154"), // Light green
		lipgloss.Color("118"), // Green
		lipgloss.Color("82"),  // Bright green
		lipgloss.Color("42"),  // Full green
	}
)

// TaskCountColor maps a value within [min, max] to a color from TaskCountColorScale.
func TaskCountColor(value, min, max int) color.Color {
	if min == max {
		return TaskCountColorScale[4] // middle color
	}
	idx := (value - min) * 9 / (max - min)
	if idx < 0 {
		idx = 0
	} else if idx > 9 {
		idx = 9
	}
	return TaskCountColorScale[idx]
}

// Base styles
var (
	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			MarginBottom(1)

	// DimStyle applies the ANSI faint attribute to visually de-emphasize unfocused panes.
	DimStyle = lipgloss.NewStyle().Faint(true)

	// Text styles
	NormalStyle = lipgloss.NewStyle()

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	// Status colors
	TodoStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	InProgressStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	ReviewStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	DoneStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	BlockedStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	StandbyStyle = lipgloss.NewStyle().
			Foreground(ColorViewStandby)

	CanceledStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Strikethrough(true)

	OverdueStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	DueSoonStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DependencyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	// Priority styles
	PriorityHighStyle = lipgloss.NewStyle().
				Foreground(ColorError) // Red for priority 1

	PriorityMediumStyle = lipgloss.NewStyle().
				Foreground(ColorWarning) // Orange for priority 2

	PriorityLowStyle = lipgloss.NewStyle().
				Foreground(ColorMuted) // Dimmed gray for priority 3-5

	// Due date styles
	DueDateStyle = lipgloss.NewStyle().
			Foreground(ColorMuted) // Gray for normal due dates

	DueTodayStyle = lipgloss.NewStyle().
			Foreground(ColorWarning) // Orange for tasks due today

	// BlockedByDepsStyle is used for tasks with incomplete dependencies
	// Uses red background to create a visual "outline" effect on the status icon
	BlockedByDepsStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("231")). // White text
				Background(ColorError)             // Red background

	// Task type colors
	EpicStyle = lipgloss.NewStyle().
			Foreground(ColorPurple) // Purple

	StoryStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary) // Blue

	TaskStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess) // Green

	BugStyle = lipgloss.NewStyle().
			Foreground(ColorError) // Red

	IdeaStyle = lipgloss.NewStyle().
			Foreground(ColorWarning) // Orange

	NoteStyle = lipgloss.NewStyle().
			Foreground(ColorMuted) // Gray

	ActionStyle = lipgloss.NewStyle().
			Foreground(ColorOrange) // Orange (quick to-dos)

	// Task type badge styles (icon with background for visual distinction)
	EpicBadgeStyle   = makeBadgeStyle("135", "53")  // Purple
	StoryBadgeStyle  = makeBadgeStyle("39", "17")   // Blue
	TaskBadgeStyle   = makeBadgeStyle("42", "22")   // Green
	BugBadgeStyle    = makeBadgeStyle("196", "52")  // Red
	IdeaBadgeStyle   = makeBadgeStyle("214", "94")  // Orange
	NoteBadgeStyle   = makeBadgeStyle("248", "237") // Gray
	ActionBadgeStyle = makeBadgeStyle("208", "94")  // Orange on dark orange

	// Task status badge styles (icon with background for visual distinction)
	TodoBadgeStyle       = makeBadgeStyle("248", "236") // Gray
	InProgressBadgeStyle = makeBadgeStyle("214", "94")  // Orange
	ReviewBadgeStyle     = makeBadgeStyle("33", "17")   // Blue (ColorSecondary)
	DoneBadgeStyle       = makeBadgeStyle("42", "22")   // Green
	BlockedBadgeStyle    = makeBadgeStyle("196", "52")  // Red
	StandbyBadgeStyle    = makeBadgeStyle("141", "53")  // Violet
	CanceledBadgeStyle   = makeBadgeStyle("248", "236") // Gray
	StageBadgeStyle      = makeBadgeStyle("183", "236") // Lavender on dark gray

	// Container styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	SelectedBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	// Help bar
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Background(ColorBackground).
			Padding(0, 1)

	// Tree view
	TreeIndent = "  "

	// Form layout
	FormLabelWidth = 11 // Width of label column in form card layouts

	TreeBranchStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	// Progress bar colors
	ProgressFullStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	ProgressEmptyStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// Scrollbar styles (vertical track on right edge of scrollable lists)
	ScrollbarThumbStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary) // Blue 39

	ScrollbarTrackStyle = lipgloss.NewStyle().
				Foreground(ColorMuted) // Gray 241

	// Badge styles
	ArchivedBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	// Confirmation prompt style
	ConfirmStyle = lipgloss.NewStyle().
			Background(ColorWarning).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			MarginTop(1)

	// Prefix key popup style (which-key style hints)
	PrefixPopupStyle = lipgloss.NewStyle().
				Background(ColorBackground).
				Foreground(ColorLightGray).
				Padding(0, 1)

	PrefixKeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	PrefixDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Form input styles
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	FormInputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	FormInputBorderFocusedStyle = lipgloss.NewStyle().
					Border(lipgloss.NormalBorder()).
					BorderForeground(ColorPrimary)

	// Validation error style
	ValidationErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError).
				MarginTop(1)

	// Warning message style (e.g. unsaved changes)
	WarningMessageStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	// Field label styles
	FieldLabelStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FieldLabelFocusedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	FieldDescriptionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Faint(true)

	// Selector option styles
	SelectorOptionStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	SelectorOptionSelectedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorSelectedFg).
					Padding(0, 1)

	// Success message style
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// Info status bar style (neutral blue for transient informational messages)
	InfoStatusStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// Loading message style (accent color for in-progress operations)
	LoadingStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	// Error status bar style (red for errors)
	ErrorStatusStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	// Help view styles
	HelpViewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	HelpCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)

	HelpFilterActiveStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	// Focus mode style
	FocusModeStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Background(ColorBackground)

	// View mode styles
	ViewModeActiveStyle    = makeViewModeStyle("34")  // ColorViewActive
	ViewModeNextStyle      = makeViewModeStyle("61")  // ColorViewNext
	ViewModeInboxStyle     = makeViewModeStyle("33")  // ColorSecondary
	ViewModeWaitingStyle   = makeViewModeStyle("214") // ColorWarning
	ViewModeSomedayStyle   = makeViewModeStyle("135") // ColorViewSomeday
	ViewModeCompletedStyle = makeViewModeStyle("136")
	ViewModeReviewStyle    = makeViewModeStyle("30")  // ColorViewReview
	ViewModeStandbyStyle   = makeViewModeStyle("141") // ColorViewStandby
	ViewModeSnoozedStyle   = makeViewModeStyle("95")
	ViewModeAllStyle       = makeViewModeStyle("60")

	// Filter view styles
	FilterBadgeStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	SavedFilterBadgeStyle = lipgloss.NewStyle().
				Background(ColorSecondary).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	FilterBadgeInactiveStyle = lipgloss.NewStyle().
					Background(ColorBackground). // Dark gray (matches SectionBadgeStyle)
					Foreground(ColorBadgeText).  // Gray text
					Padding(0, 1)

	FilterViewBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	FilterItemStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	FilterItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorSelectedFg).
				Bold(true)

	FilterValueStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	FilterClearStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	// Task jumper styles
	TaskJumperTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	TaskJumperListStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, true, false, false).
				BorderForeground(ColorBorder)

	TaskJumperPreviewStyle = lipgloss.NewStyle().
				Padding(0, 1)

	TaskJumperSelectedStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg).
				Foreground(ColorSelectedFg).
				Bold(true)

	TaskJumperPreviewHeaderStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary)

	// Due date overview styles
	DueDateOverviewTitleStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary)

	// GroupHeaderStyle is used for section headers in priority, due date, and similar views.
	GroupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginTop(1)

	// SectionHeaderStyle is like GroupHeaderStyle but without MarginTop,
	// for views where each entry must be exactly one line (line counting).
	SectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)

	SelectedStyle = lipgloss.NewStyle().
			Background(ColorSelectedBg).
			Foreground(ColorSelectedFg).
			Bold(true)

	// Convenience style for selected item text (bold + purple fg, no bg)
	SelectedItemTextStyle = BoldStyle.Foreground(ColorSelectedFg)

	// Selection row styles (unified across all list views)
	SelectionCursorStyle = lipgloss.NewStyle().
				Foreground(ColorSelectedFg).
				Bold(true)

	SelectionRowStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg)

	CursorFlashRowStyle = lipgloss.NewStyle().
				Background(ColorFlashBg)

	// Multi-select styles
	SelectionStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	SelectionCheckStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	SelectionCountStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	// Tag manager styles
	TagManagerBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	TagManagerItemStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	TagManagerItemSelectedStyle = lipgloss.NewStyle().
					Foreground(ColorSelectedFg).
					Bold(true)

	TagManagerCountStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	TagManagerErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	// Section badge style for tree view
	SectionBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorBadgeText). // Gray text
				Background(ColorBackground) // Dark gray background

	// Children in other sections badge (shown on parent when children are hidden by view filter)
	ChildrenBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).   // Blue - info indicator
				Background(ColorBackground) // Dark gray background (matches SectionBadgeStyle)

	// Pinned project style
	PinnedProjectStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	// Pinned project number shortcut style
	PinnedNumberStyle = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	// Active task style (yellow to make it pop out)
	ActiveStyle = lipgloss.NewStyle().
			Foreground(ColorAccent) // Bright yellow

	// Triaged badge style (shown in inbox processor when task will leave inbox)
	TriagedBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorWarning). // Orange
				Italic(true)

	// Priority view styles
	PriorityViewTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	PriorityWaitingHeaderStyle = lipgloss.NewStyle().
					Foreground(ColorMuted).
					Italic(true).
					MarginTop(1)

	// Suggestion styles (for Priority View smart suggestions)
	SuggestionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorAccent). // Bright yellow
				Padding(0, 1)

	SuggestionHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorAccent). // Bright yellow
				Bold(true)

	SuggestionReasonStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true)

	SuggestionStarStyle = lipgloss.NewStyle().
				Foreground(ColorAccent). // Bright yellow (matches ActiveStyle)
				Bold(true)

	// Priority level styles (for priority picker and palette)
	Priority1Style = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	Priority2Style = makeColorStyle(ColorWarning)
	Priority3Style = makeColorStyle(ColorPrimary)
	Priority4Style = makeColorStyle(ColorMuted)
	Priority5Style = makeColorStyle(ColorMuted).Faint(true)

	// Workflow rule syntax highlighting styles
	ColorRuleKeyword   = lipgloss.Color("14") // Cyan for IF/AND/OR/THEN
	ColorRuleOperator  = lipgloss.Color("13") // Magenta for =, !=, <, >, :
	RuleKeywordStyle   = makeColorStyle(ColorRuleKeyword)
	RuleOperatorStyle  = makeColorStyle(ColorRuleOperator)
	RuleHighlightStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true) // Yellow for focused field

	// HighlightStyle is used to highlight matched characters in search results.
	HighlightStyle = lipgloss.NewStyle().Background(ColorHighlight).Foreground(lipgloss.Color("0")).Bold(true)

	// Common inline styles used across multiple files
	MatchHighlightStyle       = lipgloss.NewStyle().Foreground(ColorHighlight)
	TagInlineStyle            = lipgloss.NewStyle().Foreground(ColorSecondary)
	PriorityInlineStyle       = lipgloss.NewStyle().Foreground(ColorWarning)
	CheckmarkStyle            = lipgloss.NewStyle().Foreground(ColorSuccess)
	WarningIconStyle          = lipgloss.NewStyle().Foreground(ColorWarning)
	ErrorIconStyle            = lipgloss.NewStyle().Foreground(ColorError)
	SuccessBoldStyle          = BoldStyle.Foreground(ColorSuccess)
	BoldPrimaryStyle          = BoldStyle.Foreground(ColorPrimary)
	BoldSecondaryStyle        = BoldStyle.Foreground(ColorSecondary)
	BoldPrimaryUnderlineStyle = BoldStyle.Foreground(ColorPrimary).Underline(true)
	PurpleStyle               = lipgloss.NewStyle().Foreground(ColorPurple)
	OrangeStyle               = lipgloss.NewStyle().Foreground(ColorWarning)
)

// Status icon constants for TUI rendering. These mirror the icons returned by
// models.TaskStatus.Icon() so that TUI code can reference them without needing
// a typed TaskStatus value.
const (
	StatusIconTodo       = "□"
	StatusIconInProgress = "▶"
	StatusIconReview     = "◑"
	StatusIconStandby    = "⏸"
	StatusIconDone       = "✓"
	StatusIconBlocked    = "⊘"
	StatusIconCanceled   = "✕"
)

// Priority icon constants for TUI rendering. Each level gets a distinct shape
// so that status is not conveyed by color alone.
const (
	PriorityIcon1 = "◉" // critical  — bullseye
	PriorityIcon2 = "●" // high      — filled circle
	PriorityIcon3 = "◆" // medium    — filled diamond
	PriorityIcon4 = "◇" // low       — hollow diamond
	PriorityIcon5 = "·" // minimal   — tiny dot
)

// PriorityIcon returns the unicode icon for a priority level (1–5).
func PriorityIcon(priority int) string {
	switch priority {
	case 1:
		return PriorityIcon1
	case 2:
		return PriorityIcon2
	case 3:
		return PriorityIcon3
	case 4:
		return PriorityIcon4
	case 5:
		return PriorityIcon5
	default:
		return PriorityIcon3
	}
}

// GetStatusStyle returns the appropriate style for a task status.
func GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "todo":
		return TodoStyle
	case "in_progress":
		return InProgressStyle
	case "review":
		return ReviewStyle
	case "standby":
		return StandbyStyle
	case "done":
		return DoneStyle
	case "blocked":
		return BlockedStyle
	case "canceled":
		return CanceledStyle
	default:
		return NormalStyle
	}
}

// GetPriorityStyle returns the appropriate style for a priority level.
func GetPriorityStyle(priority int) lipgloss.Style {
	switch priority {
	case 1:
		return Priority1Style
	case 2:
		return Priority2Style
	case 3:
		return Priority3Style
	case 4:
		return Priority4Style
	case 5:
		return Priority5Style
	default:
		return NormalStyle
	}
}

// GetTypeStyle returns the appropriate style for a task type.
func GetTypeStyle(taskType string) lipgloss.Style {
	switch taskType {
	case "epic":
		return EpicStyle
	case "story":
		return StoryStyle
	case "task":
		return TaskStyle
	case "bug":
		return BugStyle
	case "idea":
		return IdeaStyle
	case "note":
		return NoteStyle
	case "action":
		return ActionStyle
	default:
		return NormalStyle
	}
}

// GetTypeBadge returns the task type icon rendered as a styled badge with background.
func GetTypeBadge(taskType string, icon string) string {
	switch taskType {
	case "epic":
		return EpicBadgeStyle.Render(icon)
	case "story":
		return StoryBadgeStyle.Render(icon)
	case "task":
		return TaskBadgeStyle.Render(icon)
	case "bug":
		return BugBadgeStyle.Render(icon)
	case "idea":
		return IdeaBadgeStyle.Render(icon)
	case "note":
		return NoteBadgeStyle.Render(icon)
	case "action":
		return ActionBadgeStyle.Render(icon)
	default:
		return NormalStyle.Render(icon)
	}
}

// GetStatusBadge returns the task status icon rendered as a styled badge with background.
func GetStatusBadge(status string, icon string) string {
	switch status {
	case "todo":
		return TodoBadgeStyle.Render(icon)
	case "in_progress":
		return InProgressBadgeStyle.Render(icon)
	case "review":
		return ReviewBadgeStyle.Render(icon)
	case "standby":
		return StandbyBadgeStyle.Render(icon)
	case "done":
		return DoneBadgeStyle.Render(icon)
	case "blocked":
		return BlockedBadgeStyle.Render(icon)
	case "canceled":
		return CanceledBadgeStyle.Render(icon)
	default:
		return NormalStyle.Render(icon)
	}
}

// GetStageBadge returns the stage name rendered as a styled badge.
// Returns an empty string when stage is empty.
func GetStageBadge(stage string) string {
	if stage == "" {
		return ""
	}
	return StageBadgeStyle.Render(stage)
}

// RenderProgressBar renders a simple progress bar.
func RenderProgressBar(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	empty := width - filled

	var bar strings.Builder
	for range filled {
		bar.WriteString(ProgressFullStyle.Render("█"))
	}
	for range empty {
		bar.WriteString(ProgressEmptyStyle.Render("░"))
	}

	return bar.String()
}

// SectionColorEntry represents a color in the section color palette.
type SectionColorEntry struct {
	Name  string
	Color color.Color
}

// SectionColorPalette defines the available colors for sections.
var SectionColorPalette = []SectionColorEntry{
	{"gray", lipgloss.Color("245")},
	{"red", lipgloss.Color("196")},
	{"orange", ColorOrange},
	{"yellow", lipgloss.Color("220")},
	{"green", lipgloss.Color("42")},
	{"cyan", lipgloss.Color("44")},
	{"blue", lipgloss.Color("33")},
	{"purple", ColorPurple},
	{"magenta", lipgloss.Color("198")},
	{"white", lipgloss.Color("255")},
}

// GetViewModeStyleForSection returns the ViewMode style for a section name.
// This maps section strings to their corresponding ViewMode styles, providing
// consistent styling between the view mode indicator and section headers.
func GetViewModeStyleForSection(section string) lipgloss.Style {
	switch section {
	case "": // Active section
		return ViewModeActiveStyle
	case "Next":
		return ViewModeNextStyle
	case "Inbox":
		return ViewModeInboxStyle
	case "Waiting":
		return ViewModeWaitingStyle
	case "Someday":
		return ViewModeSomedayStyle
	case "Review":
		return ViewModeReviewStyle
	case "Standby":
		return ViewModeStandbyStyle
	case "All":
		return ViewModeAllStyle
	case "Active":
		return ViewModeActiveStyle
	case "Completed":
		return ViewModeCompletedStyle
	case "Snoozed":
		return ViewModeSnoozedStyle
	default:
		return ViewModeActiveStyle
	}
}

// GetSectionColorStyle returns a style for a section badge given a color name.
func GetSectionColorStyle(colorName string) lipgloss.Style {
	bg := ColorBackground
	if themeBgColor != "" {
		bg = lipgloss.Color(themeBgColor)
	}
	for _, pc := range SectionColorPalette {
		if pc.Name == colorName {
			return lipgloss.NewStyle().
				Foreground(pc.Color).
				Background(bg)
		}
	}
	// Default to gray
	return SectionBadgeStyle
}

// GetSectionColorIndex returns the index of a color name in the palette.
func GetSectionColorIndex(colorName string) int {
	for i, pc := range SectionColorPalette {
		if pc.Name == colorName {
			return i
		}
	}
	return 0 // Default to gray
}

// GetSectionColorByIndex returns the color name at the given palette index.
func GetSectionColorByIndex(index int) string {
	if index < 0 || index >= len(SectionColorPalette) {
		return "gray"
	}
	return SectionColorPalette[index].Name
}

// GetSectionColorValue returns the color.Color for a section color name.
// Returns nil if the color name is empty or not found.
func GetSectionColorValue(colorName string) color.Color {
	if colorName == "" {
		return nil
	}
	for _, pc := range SectionColorPalette {
		if pc.Name == colorName {
			return pc.Color
		}
	}
	return nil
}

// Sidebar styles
var (
	SidebarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1)

	SidebarHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

	SidebarSectionHeaderStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorSecondary).
					Underline(true)

	TagStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Background(ColorBackground).
			Padding(0, 1)

	SidebarFocusBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorAccent). // Bright yellow like ActiveStyle
				Bold(true)

	// Sidebar card styles (compact boxes for sidebar sections)
	SidebarCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	SidebarCardFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	SidebarCardTitleStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// Dashboard card styles (full-width cards for dashboard sections)
	DashboardCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1)

	DashboardCardTitleStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)
)

// Additional shared styles for common UI patterns
var (
	// Selector and list styles
	SelectorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	SelectorSearchStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				MarginBottom(1)

	SelectorCountStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				MarginBottom(1)

	SelectorNoResultsStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true).
				MarginTop(1).
				MarginBottom(1)

	SelectorBaseStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	// Empty/error state styles
	EmptyErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	// Separator for split-pane views
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Category badge styles for command palette
	CategoryCommandStyle     = lipgloss.NewStyle().Foreground(ColorPrimary)
	CategoryTaskStyle        = lipgloss.NewStyle().Foreground(ColorSuccess)
	CategoryProjectStyle     = lipgloss.NewStyle().Foreground(ColorSecondary)
	CategoryTagStyle         = lipgloss.NewStyle().Foreground(ColorWarning)
	CategorySectionStyle     = lipgloss.NewStyle().Foreground(ColorPurple)           // Purple
	CategorySavedSearchStyle = lipgloss.NewStyle().Foreground(ColorWarning)          // Orange
	CategorySavedFilterStyle = lipgloss.NewStyle().Foreground(ColorSecondary)        // Blue (matches saved filter badge)
	CategoryNoteStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("108")) // Sage green
	CategoryViewStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // Sky blue

	// Selection indicators
	SelectedCheckboxStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	PinIndicatorStyle     = lipgloss.NewStyle().Foreground(ColorWarning)

	// Due date urgency styles
	DueThisWeekStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Header style (bold, no color)
	HeaderBoldStyle = lipgloss.NewStyle().Bold(true)

	// Preview style (warning color for bulk edit, etc.)
	PreviewWarningStyle = lipgloss.NewStyle().Foreground(ColorWarning)

	// Overlay split-pane separator (vertical line between list and preview)
	OverlaySplitStyle = lipgloss.NewStyle().
				Foreground(ColorBorder)

	// Overlay panel styles (used by score profile manager, etc.)
	OverlayTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorPurple)
	OverlaySelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	OverlayDetailStyle   = lipgloss.NewStyle().Foreground(ColorLightGray)
	OverlayBoxStyle      = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPurple).
				Padding(1, 2)

	// Diff styles
	DiffAddStyle     = lipgloss.NewStyle().Foreground(ColorSuccess)
	DiffRemoveStyle  = lipgloss.NewStyle().Foreground(ColorError)
	DiffContextStyle = lipgloss.NewStyle().Foreground(ColorDiffContext)
)

// FocusIndicatorStyle is used for the ▐ bar that shows which tab row has focus.
var FocusIndicatorStyle = lipgloss.NewStyle().Foreground(ColorPrimary)

// FocusIndicator is the pre-rendered ▐ bar used to indicate focused tab rows.
var FocusIndicator = FocusIndicatorStyle.Render("▐")

// PinnedMarker is a green dot used to indicate pinned tasks in list views.
var PinnedMarker = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true).Render("·")

// RenderSelectableRow applies consistent selection styling to list items.
// When selected=true, adds cursor prefix and row background that extends to full width.
// When active=true, adds an asterisk indicator after the cursor position.
// When pinned=true, adds a green dot indicator.
// The content should already have its internal styling (badges, etc).
// RenderCursorPrefix returns the 3-character cursor prefix string based on
// selected/active/pinned state. Always exactly 3 display characters wide.
func RenderCursorPrefix(selected, active, pinned bool) string {
	activeMarker := ActiveStyle.Bold(true).Render("*")
	switch {
	case selected && active && pinned:
		return SelectionCursorStyle.Render(">") + activeMarker + PinnedMarker
	case selected && active:
		return SelectionCursorStyle.Render(">") + activeMarker + " "
	case selected && pinned:
		return SelectionCursorStyle.Render(">") + " " + PinnedMarker
	case selected:
		return SelectionCursorStyle.Render(">") + "  "
	case active && pinned:
		return " " + activeMarker + PinnedMarker
	case active:
		return " " + activeMarker + " "
	case pinned:
		return "  " + PinnedMarker
	default:
		return "   "
	}
}

// RenderSelectableRow renders a row with cursor prefix and optional full-width
// selection highlight. Uses RenderCursorPrefix for the 3-char prefix, then
// applies SelectionRowStyle with the given width when selected.
func RenderSelectableRow(selected bool, active bool, pinned bool, flash bool, content string, width int) string {
	row := RenderCursorPrefix(selected, active, pinned) + content
	if selected {
		style := SelectionRowStyle
		if flash {
			style = CursorFlashRowStyle
		}
		return style.Width(width).Render(row)
	}
	return row
}

// Hyperlink creates a clickable OSC 8 hyperlink in supported terminals.
// The displayed text can differ from the actual URL, so you can show truncated
// text while linking to the full URL.
// Format: \033]8;;URL\033\\Display Text\033]8;;\033\\
func Hyperlink(url, displayText string) string {
	return "\033]8;;" + url + "\033\\" + displayText + "\033]8;;\033\\"
}
