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

var (
	settingsTabNames = []string{"General", "Palette", "Review", "Behavior", "Attachments", "Project Tree", "Outliner", "Kanban", "Notifications", "Stages", "Appearance"}

	// tabFieldCount declares how many focusable fields each tab has.
	tabFieldCount = []int{5, 18, 4, 13, 2, 14, 22, 11, 15, 1, 3}

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
		{{"Recent Activity", 2}, {"Linked Tasks", 1}, {"Pinned Tasks", 1}, {"Recent Tasks", 1}, {"Popups", 1}, {"Section Limits", 7}}, // 3: Behavior
		nil, // 4: Attachments
		nil, // 5: Project Tree
		{{"Sidebar", 1}, {"Section State", 7}, {"Column Visibility", 14}}, // 6: Outliner
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
	kanbanModeOptions = []string{"Section (GTD)", "Status", "Priority", "Stages", "Custom Sections", "Score Buckets"}

	// kanbanModeValues maps cursor index to the KanbanMode ID string.
	kanbanModeValues = []string{"section", "status", "priority", "stages", "custom_sections", "score_buckets"}

	// kanbanModeLabels are short labels shown in the enabled-modes checkbox list.
	kanbanModeLabels = []string{"Section (GTD)", "Status", "Priority", "Stages", "Custom Sections", "Score Buckets"}

	// kanbanDetailHeightOptions lists the allowed detail pane heights.
	kanbanDetailHeightOptions = []string{"1", "2", "3", "4", "5"}
)

func init() {
	tabFieldStart = make([]int, len(tabFieldCount))
	offset := 0
	for i, count := range tabFieldCount {
		tabFieldStart[i] = offset
		offset += count
	}
	settingsFieldCount = offset
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

// subTabNamesForTab returns the section names for tab, or nil if it has no sub-tabs.
func subTabNamesForTab(tab int) []string {
	if tab < 0 || tab >= len(settingsTabSections) || settingsTabSections[tab] == nil {
		return nil
	}
	names := make([]string, len(settingsTabSections[tab]))
	for i, s := range settingsTabSections[tab] {
		names[i] = s.Name
	}
	return names
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

	editorInput              textinput.Model
	notesEditorInput         textinput.Model
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
	treeColumnDefaults       [14]bool // Global default column visibility for tree view
	outlinerColumnDefaults   [14]bool // Global default column visibility for outliner view
	treePriorityMode         int      // 0=All, 1=Conditional, 2=Hidden
	outlinerPriorityMode     int      // 0=All, 1=Conditional, 2=Hidden
	crossOutlinerSidebar     bool     // Default sidebar state for cross-project outliner
	zebraStriping            bool     // Alternating row backgrounds in task list views
	weekEndDayCursor         int      // Index into weekEndDayValues (0=fri, 1=sat, 2=sun)
	timeFormatCursor         int      // Index into timeFormatValues (0=12h, 1=24h)
	contexts                 []string
	contextCursor            int // -1 for custom mode, 0+ for selecting from list

	// Kanban settings (Kanban tab, index 94-104)
	kanbanDefaultModeCursor  int             // Index into kanbanModeValues
	kanbanStartColInput      textinput.Model // Default start column text input
	kanbanShowEmptyColumns   bool            // Show columns even when empty
	kanbanDetailPaneVisible  bool            // Bottom detail pane shown by default
	kanbanDetailHeightCursor int             // Index into kanbanDetailHeightOptions (0=1 line .. 4=5 lines)
	kanbanEnabledModes       [6]bool         // Enabled mode toggles (indexed by AllKanbanModes order)

	// Stages settings (Stages tab, index 76)
	savedStages        []string // snapshot at load time, for dirty detection
	stages             []string
	stageInput         textinput.Model
	stageCursor        int
	stageEditMode      bool // true when adding or editing a stage
	stageAddMode       bool // true when adding (vs editing)
	stageConfirmDelete ConfirmDialog

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
	subTab             int  // active sub-tab index within current tab
	subTabRowFocused   bool // true when shift+left/right navigates sub-tabs instead of main tabs
	focusIndex         int  // 0=editor .. 74=outlinerColumnDefaults, 75-89=alert settings, 90=stages, 91=appearance
	scrollOffset       int  // vertical scroll offset within current tab content
	validationErr      string
	stageValidationErr string // timed error shown in stages tab (auto-clears)

	width  int
	height int
}
