package tui

import (
	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
)

// settingsSection describes one sub-tab section within a settings tab.
type settingsSection struct {
	Name       string
	FieldCount int
}

// Tab index constants for settings tabs. Used with tabFieldStart to compute
// absolute focus indices, avoiding fragile hardcoded values.
const (
	tabGeneral       = 0
	tabPalette       = 1
	tabReview        = 2
	tabBehavior      = 3
	tabAttachments   = 4
	tabOutliner      = 5
	tabNotifications = 6
	tabAppearance    = 7
)

// settingsVisualTabNames expose the four durable preference categories.
var settingsVisualTabNames = []string{"General", "Appearance", "Review", "Notifications"}

// visualSubTabRef maps a visual sub-tab to its field-group index.
type visualSubTabRef struct {
	name        string
	fieldTab    int
	fieldSubTab int
}

// visualTabSubTabs defines the sub-tab structure for each visual tab.
var visualTabSubTabs = [][]visualSubTabRef{
	// vtab 0: General = durable General/Behavior fields + Attachments
	{
		{"General", tabGeneral, 0},
		{"Recent Activity", tabBehavior, 0},
		{"Linked Tasks", tabBehavior, 1},
		{"Pinned Tasks", tabBehavior, 2},
		{"Recent Tasks", tabBehavior, 3},
		{"Section Limits", tabBehavior, 4},
		{"Startup", tabBehavior, 5},
		{"Home", tabBehavior, 6},
		{"Attachments", tabAttachments, 0},
	},
	// vtab 1: Appearance = old Palette sub-sections + old Appearance
	{
		{"Palette Colors", tabPalette, 0},
		{"Background & Cursor", tabPalette, 1},
		{"Priorities", tabPalette, 2},
		{"Appearance", tabAppearance, 0},
	},
	// vtab 2: Review (single section, no sub-tab bar)
	{
		{"Review", tabReview, 0},
	},
	// vtab 3: Notifications
	{
		{"Reviews", tabNotifications, 0},
		{"Tasks", tabNotifications, 1},
		{"Projects", tabNotifications, 2},
	},
}

// visualSubTabNamesForVTab returns the display names for visual sub-tabs within vt,
// or nil when there is only one section (no sub-tab bar needed).
func visualSubTabNamesForVTab(vt int) []string {
	if vt < 0 || vt >= len(visualTabSubTabs) {
		return nil
	}
	refs := visualTabSubTabs[vt]
	if len(refs) <= 1 {
		return nil
	}
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.name
	}
	return names
}

var (
	settingsTabNames = []string{"General", "Palette", "Review", "Behavior", "Attachments", "Outliner", "Notifications", "Appearance"}

	// tabFieldCount declares how many focusable fields each tab has.
	tabFieldCount = []int{4, 18, 5, 15, 2, 24, 15, 5}

	// tabFieldStart holds the starting focus index for each tab (computed in init).
	tabFieldStart []int

	// settingsTabSections defines the sub-tab sections for each settings tab.
	// nil means the tab has no sub-tabs (single section).
	settingsTabSections = [][]settingsSection{
		nil, // 0: General
		{{"Palette Colors", 10}, {"Background & Cursor", 3}, {"Priorities", 5}}, // 1: Palette
		nil, // 2: Review
		{{"Recent Activity", 2}, {"Linked Tasks", 1}, {"Pinned Tasks", 1}, {"Recent Tasks", 1}, {"Section Limits", 7}, {"Startup", 1}, {"Home", 2}}, // 3: Behavior
		nil, // 4: Attachments
		{{"Sidebar", 2}, {"Section State", 7}, {"Column Visibility", 15}}, // 5: Outliner
		{{"Reviews", 4}, {"Tasks", 9}, {"Projects", 2}},                   // 6: Notifications
		nil, // 7: Appearance
	}

	// weeklyReviewDayOptions lists the display labels for weekly review day selector.
	weeklyReviewDayOptions = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	// outlinerSectionLabels are the display labels for each toggle.
	outlinerSectionLabels = []string{"Active", "Inbox", "Next", "Waiting", "Review", "Standby", "Someday"}

	// priorityModeOptions lists the display labels for priority display mode.
	priorityModeOptions = []string{"All", "Conditional", "Hidden"}

	// weekEndDayOptions lists the display labels for the week-end day selector.
	weekEndDayOptions = []string{"Friday", "Saturday", "Sunday"}

	// weekEndDayValues maps cursor index to config string value.
	weekEndDayValues = []string{"fri", "sat", "sun"}

	// timeFormatOptions lists the display labels for the time format selector.
	timeFormatOptions = []string{"12-hour (5:00pm)", "24-hour (17:00)"}

	// timeFormatValues maps cursor index to config string value.
	timeFormatValues = []string{"12h", "24h"}

	// startViewOptions lists display labels for the start view selector.
	startViewOptions = []string{"Home", "Planner", "Tasks", "Project", "Review", "Manage"}

	// startViewValues maps cursor index to config string value.
	startViewValues = []string{
		config.StartViewHome,
		config.StartViewPlanner,
		config.StartViewTasks,
		config.StartViewProject,
		config.StartViewReview,
		config.StartViewManage,
	}

	// defaultStartViewCursor is the cursor index in startViewValues whose value
	// matches config.DefaultStartView. Computed in init() so the settings UI
	// stays in sync if either side moves. Falls back to 0 when no match is
	// found, which keeps the selector pointed at a real option.
	defaultStartViewCursor int
)

func init() {
	tabFieldStart = make([]int, len(tabFieldCount))
	for i := 1; i < len(tabFieldCount); i++ {
		tabFieldStart[i] = tabFieldStart[i-1] + tabFieldCount[i-1]
	}
	settingsFields = buildSettingsFieldRegistry()

	for i, v := range startViewValues {
		if v == config.DefaultStartView {
			defaultStartViewCursor = i
			break
		}
	}
}

// tabForField returns which tab a given focusIndex belongs to.
func tabForField(focusIndex int) int {
	for t := len(tabFieldStart) - 1; t >= 0; t-- {
		if focusIndex >= tabFieldStart[t] {
			return t
		}
	}
	return 0
}

// subTabFieldStart returns the absolute focusIndex for the first field of subTab within tab.
func subTabFieldStart(tab, subTab int) int {
	start := tabFieldStart[tab]
	sections := settingsTabSections[tab]
	if sections == nil {
		return start
	}
	for i := 0; i < subTab && i < len(sections); i++ {
		start += sections[i].FieldCount
	}
	return start
}

// subTabForField returns which sub-tab index a focusIndex belongs to within its tab.
func subTabForField(focusIndex int) int {
	tab := tabForField(focusIndex)
	sections := settingsTabSections[tab]
	if sections == nil {
		return 0
	}
	offset := tabFieldStart[tab]
	for i, s := range sections {
		if focusIndex < offset+s.FieldCount {
			return i
		}
		offset += s.FieldCount
	}
	return len(sections) - 1
}

// SettingsModel handles the settings view in the TUI.
type SettingsModel struct {
	// runtime is the domain-native snapshot the editor reads its initial values
	// from and commits back through. manager is retained only for theme-preset
	// file I/O (ListThemes/WriteTheme/DeleteTheme) and the workspace-location
	// label; the editor never reads or saves config.Config through it.
	runtime *config.RuntimeState
	manager *config.Manager
	// baseSettings/baseMachine are value copies of the settings and machine-local
	// domains captured when the editor opens. The editor mutates only the fields
	// its widgets own onto copies of these at save time, so untouched domain
	// fields (fzf/glow toggles, histories, navigation, cross-outliner state) are
	// preserved through a domain-scoped commit.
	baseSettings config.Settings
	baseMachine  config.MachineState
	// coordinator routes the durable settings/machine write through the secondary
	// feature-coordinator seam (feature_coordinator_secondary.go), which commits
	// only those two domains under a revision check and refreshes the runtime
	// snapshot on success.
	coordinator SettingsCoordinator

	// workspaceLocation/workspaceStorage are read-only, captured when the
	// settings view opens, and surfaced in the General tab so the workspace
	// data directory is discoverable. Relocation itself is the CLI-first
	// `tt workspace move` flow.
	workspaceLocation string
	workspaceStorage  string

	editorInput              textinput.Model
	externalEditorInput      textinput.Model
	contextInput             textinput.Model
	undoDepthInput           textinput.Model
	bgColorInput             textinput.Model
	cursorColorInput         textinput.Model
	cursorTextColorInput     textinput.Model
	normalTextInput          textinput.Model
	mutedInput               textinput.Model
	primaryInput             textinput.Model
	secondaryInput           textinput.Model
	accentInput              textinput.Model
	successInput             textinput.Model
	warningInput             textinput.Model
	errorColorInput          textinput.Model
	borderInput              textinput.Model
	priority1TitleInput      textinput.Model
	priority2TitleInput      textinput.Model
	priority3TitleInput      textinput.Model
	priority4TitleInput      textinput.Model
	priority5TitleInput      textinput.Model
	dueSoonInput             textinput.Model
	futureInput              textinput.Model
	oldDaysInput             textinput.Model
	completedHoursInput      textinput.Model
	staleTaskDaysInput       textinput.Model
	recentActivityInput      textinput.Model
	recentActivityShowAll    bool // Whether to show completed tasks in recent activity by default
	linkedTaskJump           bool // Whether to jump to new task after linked creation
	maxPinnedInput           textinput.Model
	maxRecentTaskVisitsInput textinput.Model
	attachmentEditorInput    textinput.Model
	codeEditorInput          textinput.Model
	outlinerCollapsed        [7]bool  // Per-section default collapse state (indexed by outlinerSectionLabels)
	outlinerColumnDefaults   [15]bool // Global default column visibility for outliner view (full AllColumns)
	outlinerPriorityMode     int      // 0=All, 1=Conditional, 2=Hidden
	crossOutlinerSidebar     bool     // Default sidebar state for cross-project outliner
	crossOutlinerCycleOnly   bool     // Restrict cross-project outliner to cycle projects only
	zebraStriping            bool     // Alternating backgrounds in task rows
	weekEndDayCursor         int      // Index into weekEndDayValues (0=fri, 1=sat, 2=sun)
	timeFormatCursor         int      // Index into timeFormatValues (0=12h, 1=24h)
	autoTheme                bool     // Sync theme with macOS system appearance
	useEmoji                 bool     // Use emoji icons instead of Unicode symbols
	contexts                 []string
	contextCursor            int // -1 for custom mode, 0+ for selecting from list

	startViewCursor         int // Index into startViewValues
	resumeCardLimitInput    textinput.Model
	homeProjectsHeightInput textinput.Model // Home Projects/Attention split % (machine-local)

	// Alert settings (Notifications tab, indices 75-89)
	alertDailyReviewEnabled    bool
	alertDailyReviewHourInput  textinput.Model
	alertWeeklyReviewEnabled   bool
	alertWeeklyReviewDay       int // 0=Sunday..6=Saturday
	alertOverdueEnabled        bool
	alertDueSoonEnabled        bool
	alertDueSoonDaysInput      textinput.Model
	alertStaleEnabled          bool
	alertStaleDaysInput        textinput.Model
	alertInboxEnabled          bool
	alertInboxThresholdInput   textinput.Model
	alertBlockedEnabled        bool
	alertBlockedDaysInput      textinput.Model
	alertProjectHealthEnabled  bool
	alertProjectStaleDaysInput textinput.Model

	// Section limit inputs (Behavior tab, indices 32-38)
	sectionLimitActiveInput  textinput.Model
	sectionLimitNextInput    textinput.Model
	sectionLimitInboxInput   textinput.Model
	sectionLimitStandbyInput textinput.Model
	sectionLimitWaitingInput textinput.Model
	sectionLimitSomedayInput textinput.Model
	sectionLimitReviewInput  textinput.Model

	originalTheme ThemeConfig // snapshot at load time, restored on cancel

	// themePreset* fields manage the save/load/reset overlay on the Palette tab.
	themePresetMode          presetMode
	themePresetInput         textinput.Model
	themePresetList          []config.ThemeFile
	themePresetCursor        int
	themePresetErr           string
	themePresetConfirmDelete ConfirmDialog
	themePresetFlash         string // transient footer flash after delete
	themeActionMenu          bool
	themeActionCursor        int

	// activeTheme is "dark" or "light" — the palette currently shown in the inputs.
	activeTheme string
	// activePresetName is the name of the preset backing the active palette
	// (empty = built-in defaults). inactivePresetName mirrors it for the other slot.
	activePresetName   string
	inactivePresetName string
	// inactiveThemeDraft holds raw hex strings for the theme NOT currently shown.
	// Ordered by palette-tab focus offset (slot 0 = NormalText … slot 19 = DimmedFg):
	//   0-8:   NormalText, Muted, Primary, Secondary, Accent, Success, Warning, Error, Border
	//   9-11:  BackgroundColor, CursorColor, CursorTextColor
	//  12-16:  Priority1Title … Priority5Title
	//  17-19:  Dimmed, DimmedBg, DimmedFg (no UI input, carried through transparently)
	inactiveThemeDraft [20]string

	// activeHiddenDimmed stores the active theme's Dimmed/DimmedBg/DimmedFg palette
	// values that have no corresponding input widget. They are preserved through
	// save/toggle operations so settings save does not silently strip them.
	activeHiddenDimmed [3]string

	tabs             TabState
	subTab           int  // active sub-tab index within current old tab (field routing)
	subTabRowFocused bool // true when shift+left/right navigates sub-tabs instead of main tabs
	visualTab        int  // active visual tab (0-4), for tab bar display
	visualSubTab     int  // active sub-tab within the visual tab, for sub-tab bar display
	focusIndex       int  // absolute index into the compacted settings field registry
	scrollOffset     int  // vertical scroll offset within current tab content
	validationErr    string

	width  int
	height int

	searchActive    bool
	searchInput     textinput.Model
	searchResults   []settingsFieldMeta
	searchCursor    int
	searchStatusMsg string
}

// syncVisualFromOld sets visualTab and visualSubTab to match the current
// tabs.Active (old tab) and subTab (old sub-tab index).
func (m *SettingsModel) syncVisualFromOld() {
	for vt, refs := range visualTabSubTabs {
		for vs, ref := range refs {
			if ref.fieldTab == m.tabs.Active && ref.fieldSubTab == m.subTab {
				m.visualTab = vt
				m.visualSubTab = vs
				return
			}
		}
	}
	// Fallback: match by old tab only
	for vt, refs := range visualTabSubTabs {
		for vs, ref := range refs {
			if ref.fieldTab == m.tabs.Active {
				m.visualTab = vt
				m.visualSubTab = vs
				return
			}
		}
	}
}

// setFromVisual updates tabs.Active, subTab, and focusIndex from the current
// visualTab and visualSubTab.
func (m *SettingsModel) setFromVisual() {
	if m.visualTab >= len(visualTabSubTabs) {
		return
	}
	refs := visualTabSubTabs[m.visualTab]
	if m.visualSubTab >= len(refs) {
		m.visualSubTab = 0
	}
	ref := refs[m.visualSubTab]
	m.tabs.Active = ref.fieldTab
	m.subTab = ref.fieldSubTab
	m.focusIndex = firstSupportedSettingsField(ref.fieldTab, ref.fieldSubTab)
}

func firstSupportedSettingsField(tab, subTab int) int {
	for _, field := range settingsFields {
		if field.Tab == tab && field.SubTab == subTab {
			return field.FocusIndex
		}
	}
	return subTabFieldStart(tab, subTab)
}

func (m *SettingsModel) stepSupportedSettingsField(delta int) {
	if len(settingsFields) == 0 {
		return
	}
	current := 0
	for i, field := range settingsFields {
		if field.FocusIndex == m.focusIndex {
			current = i
			break
		}
	}
	next := (current + delta + len(settingsFields)) % len(settingsFields)
	m.focusIndex = settingsFields[next].FocusIndex
}
