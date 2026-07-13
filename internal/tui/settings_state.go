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
	tabProjectTree   = 5
	tabOutliner      = 6
	tabKanban        = 7
	tabNotifications = 8
	tabStages        = 9
	tabAppearance    = 10
)

// settingsVisualTabNames are the 5 consolidated tab names shown in the tab bar.
var settingsVisualTabNames = []string{"General", "Appearance", "Views", "Review", "Notifications"}

// visualSubTabRef maps a visual sub-tab to the underlying old tab and sub-tab index.
type visualSubTabRef struct {
	name      string
	oldTab    int
	oldSubTab int
}

// visualTabSubTabs defines the sub-tab structure for each of the 5 visual tabs.
// The old tab routing (tabForField, subTabForField, etc.) is unchanged; this
// layer is purely for navigation and display.
var visualTabSubTabs = [][]visualSubTabRef{
	// vtab 0: General = old General + old Behavior sub-sections + old Attachments
	{
		{"General", tabGeneral, 0},
		{"Recent Activity", tabBehavior, 0},
		{"Linked Tasks", tabBehavior, 1},
		{"Pinned Tasks", tabBehavior, 2},
		{"Recent Tasks", tabBehavior, 3},
		{"Popups", tabBehavior, 4},
		{"Section Limits", tabBehavior, 5},
		{"Startup", tabBehavior, 6},
		{"Task Form", tabBehavior, 7},
		{"Today", tabBehavior, 8},
		{"Dashboard", tabBehavior, 9},
		{"Attachments", tabAttachments, 0},
	},
	// vtab 1: Appearance = old Palette sub-sections + old Appearance
	{
		{"Palette Colors", tabPalette, 0},
		{"Background & Cursor", tabPalette, 1},
		{"Priorities", tabPalette, 2},
		{"Appearance", tabAppearance, 0},
	},
	// vtab 2: Views = old Project Tree + old Outliner sub-sections + old Kanban
	{
		{"Project Tree", tabProjectTree, 0},
		{"Sidebar", tabOutliner, 0},
		{"Section State", tabOutliner, 1},
		{"Column Visibility", tabOutliner, 2},
		{"Kanban", tabKanban, 0},
	},
	// vtab 3: Review (single section, no sub-tab bar)
	{
		{"Review", tabReview, 0},
	},
	// vtab 4: Notifications = old Notifications sub-sections + old Stages
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
	settingsTabNames = []string{"General", "Palette", "Review", "Behavior", "Attachments", "Project Tree", "Outliner", "Kanban", "Notifications", "Stages", "Appearance"}

	// tabFieldCount declares how many focusable fields each tab has.
	tabFieldCount = []int{5, 18, 5, 20, 2, 15, 24, 9, 15, 0, 5}

	// tabFieldStart holds the starting focus index for each tab (computed in init).
	tabFieldStart []int

	// settingsFieldCount is the total number of focusable fields across all tabs (computed in init).
	settingsFieldCount int

	// settingsTabSections defines the sub-tab sections for each settings tab.
	// nil means the tab has no sub-tabs (single section).
	settingsTabSections = [][]settingsSection{
		nil, // 0: General
		{{"Palette Colors", 10}, {"Background & Cursor", 3}, {"Priorities", 5}}, // 1: Palette
		nil, // 2: Review
		{{"Recent Activity", 2}, {"Linked Tasks", 1}, {"Pinned Tasks", 1}, {"Recent Tasks", 1}, {"Popups", 1}, {"Section Limits", 7}, {"Startup", 1}, {"Task Form", 1}, {"Today", 1}, {"Dashboard", 4}}, // 3: Behavior
		nil, // 4: Attachments
		nil, // 5: Project Tree
		{{"Sidebar", 2}, {"Section State", 7}, {"Column Visibility", 15}}, // 6: Outliner
		nil, // 7: Kanban
		{{"Reviews", 4}, {"Tasks", 9}, {"Projects", 2}}, // 8: Notifications
		nil, // 9: Stages
		nil, // 10: Appearance
	}

	// weeklyReviewDayOptions lists the display labels for weekly review day selector.
	weeklyReviewDayOptions = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	// outlinerSectionNames maps toggle index (0-6) to the section key used in config.
	// Active uses "" (empty string) since tasks with no explicit section are "Active".
	outlinerSectionNames = []string{"", "Inbox", "Next", "Waiting", "Review", "Standby", "Someday"}

	// outlinerSectionLabels are the display labels for each toggle.
	outlinerSectionLabels = []string{"Active", "Inbox", "Next", "Waiting", "Review", "Standby", "Someday"}

	// defaultProjectViewOptions lists the display labels for the default project view selector.
	defaultProjectViewOptions = []string{"Outliner", "Project Tree"}

	// defaultProjectViewValues maps cursor index to config string value.
	defaultProjectViewValues = []string{"outliner", "project_tree"}

	// priorityModeOptions lists the display labels for priority display mode.
	priorityModeOptions = []string{"All", "Conditional", "Hidden"}

	// priorityModeValues maps cursor index to config string value.
	priorityModeValues = []string{PriorityModeAll, PriorityModeConditional, PriorityModeHidden}

	// weekEndDayOptions lists the display labels for the week-end day selector.
	weekEndDayOptions = []string{"Friday", "Saturday", "Sunday"}

	// weekEndDayValues maps cursor index to config string value.
	weekEndDayValues = []string{"fri", "sat", "sun"}

	// timeFormatOptions lists the display labels for the time format selector.
	timeFormatOptions = []string{"12-hour (5:00pm)", "24-hour (17:00)"}

	// timeFormatValues maps cursor index to config string value.
	timeFormatValues = []string{"12h", "24h"}

	// kanbanModeOptions lists the display labels for the kanban mode selector.
	kanbanModeOptions = []string{"Section (GTD)", "Status", "Priority", "Score Buckets"}

	// kanbanModeValues maps cursor index to the KanbanMode ID string.
	kanbanModeValues = []string{"section", "status", "priority", "score_buckets"}

	// kanbanModeLabels are short labels shown in the enabled-modes checkbox list.
	kanbanModeLabels = []string{"Section (GTD)", "Status", "Priority", "Score Buckets"}

	// kanbanDetailHeightOptions lists the allowed detail pane heights.
	kanbanDetailHeightOptions = []string{"1", "2", "3", "4", "5"}

	// startViewOptions lists display labels for the start view selector.
	startViewOptions = []string{"Dashboard", "Today", "Cross-Project Outliner", "All Tasks", "Daily Review", "Backlog", "Priority", "Due Dates"}

	// startViewValues maps cursor index to config string value.
	startViewValues = []string{"dashboard", "focus-board", "cross-project-outliner", "all-tasks", "daily-review", "backlog", "priority", "due-dates"}

	// defaultStartViewCursor is the cursor index in startViewValues whose value
	// matches config.DefaultStartView. Computed in init() so the settings UI
	// stays in sync if either side moves. Falls back to 0 when no match is
	// found, which keeps the selector pointed at a real option.
	defaultStartViewCursor int

	// focusBoardModeOptions lists display labels for the Today default mode selector.
	focusBoardModeOptions = []string{"List", "Tunnel", "Zen"}

	// focusBoardModeValues maps cursor index to config string value.
	focusBoardModeValues = []string{"list", "tunnel", "zen"}

	// dashboardLayoutOptions lists display labels for the dashboard bottom layout selector.
	// Cursor index maps directly to the DashboardBottomLayout enum value.
	dashboardLayoutOptions = []string{"Horizontal", "Vertical", "List only"}
)

func init() {
	tabFieldStart = make([]int, len(tabFieldCount))
	offset := 0
	for i, count := range tabFieldCount {
		tabFieldStart[i] = offset
		offset += count
	}
	settingsFieldCount = offset
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
	config        *config.Config
	configManager *config.Manager
	// coordinator routes the durable config write through the secondary
	// feature-coordinator seam (feature_coordinator_secondary.go).
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
	popupTimeoutInput        textinput.Model
	attachmentEditorInput    textinput.Model
	codeEditorInput          textinput.Model
	defaultViewCursor        int      // Index into defaultProjectViewOptions
	outlinerCollapsed        [7]bool  // Per-section default collapse state (indexed by outlinerSectionNames)
	treeColumnDefaults       [15]bool // Global default column visibility for tree view
	outlinerColumnDefaults   [15]bool // Global default column visibility for outliner view (full AllColumns)
	treePriorityMode         int      // 0=All, 1=Conditional, 2=Hidden
	outlinerPriorityMode     int      // 0=All, 1=Conditional, 2=Hidden
	crossOutlinerSidebar     bool     // Default sidebar state for cross-project outliner
	crossOutlinerCycleOnly   bool     // Restrict cross-project outliner to cycle projects only
	zebraStriping            bool     // Alternating row backgrounds in task list views
	weekEndDayCursor         int      // Index into weekEndDayValues (0=fri, 1=sat, 2=sun)
	timeFormatCursor         int      // Index into timeFormatValues (0=12h, 1=24h)
	autoTheme                bool     // Sync theme with macOS system appearance
	useEmoji                 bool     // Use emoji icons instead of Unicode symbols
	contexts                 []string
	contextCursor            int // -1 for custom mode, 0+ for selecting from list

	startViewCursor      int  // Index into startViewValues
	taskFormCollapsed    bool // Whether task form starts collapsed
	focusBoardModeCursor int  // Index into focusBoardModeValues

	// Dashboard bottom layout settings (Behavior tab, subTab 9)
	dashboardLayoutCursor    int // Index into dashboardLayoutOptions (maps to DashboardBottomLayout)
	dashboardHorizSplitInput textinput.Model
	dashboardVertSplitInput  textinput.Model
	resumeCardLimitInput     textinput.Model

	// Kanban settings (Kanban tab, index 94-104)
	kanbanDefaultModeCursor  int             // Index into kanbanModeValues
	kanbanStartColInput      textinput.Model // Default start column text input
	kanbanShowEmptyColumns   bool            // Show columns even when empty
	kanbanDetailPaneVisible  bool            // Bottom detail pane shown by default
	kanbanDetailHeightCursor int             // Index into kanbanDetailHeightOptions (0=1 line .. 4=5 lines)
	kanbanEnabledModes       [4]bool         // Enabled mode toggles (indexed by AllKanbanModes order)

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

	tabs               TabState
	subTab             int  // active sub-tab index within current old tab (field routing)
	subTabRowFocused   bool // true when shift+left/right navigates sub-tabs instead of main tabs
	visualTab          int  // active visual tab (0-4), for tab bar display
	visualSubTab       int  // active sub-tab within the visual tab, for sub-tab bar display
	focusIndex         int  // 0=editor .. 74=outlinerColumnDefaults, 75-89=alert settings, 90=stages, 91=appearance
	scrollOffset       int  // vertical scroll offset within current tab content
	validationErr      string
	stageValidationErr string // timed error shown in stages tab (auto-clears)

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
			if ref.oldTab == m.tabs.Active && ref.oldSubTab == m.subTab {
				m.visualTab = vt
				m.visualSubTab = vs
				return
			}
		}
	}
	// Fallback: match by old tab only
	for vt, refs := range visualTabSubTabs {
		for vs, ref := range refs {
			if ref.oldTab == m.tabs.Active {
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
	m.tabs.Active = ref.oldTab
	m.subTab = ref.oldSubTab
	m.focusIndex = subTabFieldStart(ref.oldTab, ref.oldSubTab)
}
