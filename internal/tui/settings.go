package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
)

var (
	settingsTabNames = []string{"General", "Review", "Behavior", "Attachments", "Project Tree", "Outliner", "Notifications", "Stages"}

	// tabFieldCount declares how many focusable fields each tab has.
	tabFieldCount = []int{4, 4, 5, 2, 13, 21, 15, 1}

	// tabFieldStart holds the starting focus index for each tab (computed in init).
	tabFieldStart []int

	// settingsFieldCount is the total number of focusable fields across all tabs (computed in init).
	settingsFieldCount int

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

// SettingsModel handles the settings view in the TUI.
type SettingsModel struct {
	config        *config.Config
	configManager *config.Manager

	editorInput              textinput.Model
	contextInput             textinput.Model
	undoDepthInput           textinput.Model
	dueSoonInput             textinput.Model
	futureInput              textinput.Model
	oldDaysInput             textinput.Model
	completedHoursInput      textinput.Model
	recentActivityInput      textinput.Model
	recentActivityShowAll    bool // Whether to show completed tasks in recent activity by default
	linkedTaskJump           bool // Whether to jump to new task after linked creation
	maxPinnedInput           textinput.Model
	maxRecentTaskVisitsInput textinput.Model
	attachmentEditorInput    textinput.Model
	codeEditorInput          textinput.Model
	defaultViewCursor        int      // Index into defaultProjectViewOptions
	outlinerCollapsed        [7]bool  // Per-section default collapse state (indexed by outlinerSectionNames)
	treeColumnDefaults       [13]bool // Global default column visibility for tree view
	outlinerColumnDefaults   [13]bool // Global default column visibility for outliner view
	treePriorityMode         int      // 0=All, 1=Conditional, 2=Hidden
	outlinerPriorityMode     int      // 0=All, 1=Conditional, 2=Hidden
	crossOutlinerSidebar     bool     // Default sidebar state for cross-project outliner
	contexts                 []string
	contextCursor            int // -1 for custom mode, 0+ for selecting from list

	// Stages settings (Stages tab, index 64)
	savedStages        []string // snapshot at load time, for dirty detection
	stages             []string
	stageInput         textinput.Model
	stageCursor        int
	stageEditMode      bool // true when adding or editing a stage
	stageAddMode       bool // true when adding (vs editing)
	stageConfirmDelete ConfirmDialog

	// Alert settings (Notifications tab, indices 49-63)
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

	tabs               TabState
	focusIndex         int // 0=editor .. 48=outlinerColumnDefaults, 49-63=alert settings
	scrollOffset       int // vertical scroll offset within current tab content
	validationErr      string
	stageValidationErr string // timed error shown in stages tab (auto-clears)

	width  int
	height int
}

// NewSettingsModel creates a new settings model.
func NewSettingsModel(cfg *config.Config, manager *config.Manager) *SettingsModel {
	editorInput := textinput.New()
	editorInput.Placeholder = "code"
	editorInput.Focus()
	editorInput.CharLimit = 256
	editorInput.Width = 40
	editorInput.SetValue(cfg.Editor)

	contextInput := textinput.New()
	contextInput.Placeholder = "personal"
	contextInput.CharLimit = 50
	contextInput.Width = 30
	contextInput.SetValue(cfg.DefaultContext)

	undoDepthInput := textinput.New()
	undoDepthInput.Placeholder = strconv.Itoa(config.DefaultUndoHistoryDepth)
	undoDepthInput.CharLimit = 4
	undoDepthInput.Width = 10
	undoDepthValue := config.DefaultUndoHistoryDepth
	if cfg.UndoHistoryDepth != nil {
		undoDepthValue = *cfg.UndoHistoryDepth
	}
	undoDepthInput.SetValue(strconv.Itoa(undoDepthValue))

	dueSoonInput := textinput.New()
	dueSoonInput.Placeholder = "7"
	dueSoonInput.CharLimit = 3
	dueSoonInput.Width = 6
	dueSoonInput.SetValue(strconv.Itoa(cfg.GetReviewDueSoonDays()))

	futureInput := textinput.New()
	futureInput.Placeholder = "30"
	futureInput.CharLimit = 3
	futureInput.Width = 6
	futureInput.SetValue(strconv.Itoa(cfg.GetReviewFutureDays()))

	oldDaysInput := textinput.New()
	oldDaysInput.Placeholder = "14"
	oldDaysInput.CharLimit = 3
	oldDaysInput.Width = 6
	oldDaysInput.SetValue(strconv.Itoa(cfg.GetReviewOldDays()))

	completedHoursInput := textinput.New()
	completedHoursInput.Placeholder = "24"
	completedHoursInput.CharLimit = 3
	completedHoursInput.Width = 6
	completedHoursInput.SetValue(strconv.Itoa(cfg.GetReviewCompletedHours()))

	recentActivityInput := textinput.New()
	recentActivityInput.Placeholder = "20"
	recentActivityInput.CharLimit = 3
	recentActivityInput.Width = 6
	recentActivityInput.SetValue(strconv.Itoa(cfg.GetRecentActivityLimit()))

	maxPinnedInput := textinput.New()
	maxPinnedInput.Placeholder = strconv.Itoa(config.DefaultMaxPinnedBarItems)
	maxPinnedInput.CharLimit = 2
	maxPinnedInput.Width = 6
	maxPinnedInput.SetValue(strconv.Itoa(cfg.GetMaxPinnedBarItems()))

	maxRecentTaskVisitsInput := textinput.New()
	maxRecentTaskVisitsInput.Placeholder = strconv.Itoa(config.DefaultMaxRecentTaskVisits)
	maxRecentTaskVisitsInput.CharLimit = 2
	maxRecentTaskVisitsInput.Width = 6
	maxRecentTaskVisitsInput.SetValue(strconv.Itoa(cfg.GetMaxRecentTaskVisits()))

	attachmentEditorInput := textinput.New()
	attachmentEditorInput.Placeholder = "(OS default)"
	attachmentEditorInput.CharLimit = 256
	attachmentEditorInput.Width = 40
	attachmentEditorInput.SetValue(cfg.GetAttachmentEditor())

	codeEditorInput := textinput.New()
	codeEditorInput.Placeholder = "code"
	codeEditorInput.CharLimit = 256
	codeEditorInput.Width = 40
	codeEditorInput.SetValue(cfg.CodeEditor) // Use raw value, not getter (to distinguish default from explicit)

	// Determine default view cursor from config
	defaultViewCursor := 0
	configView := cfg.GetDefaultProjectView()
	for i, v := range defaultProjectViewValues {
		if v == configView {
			defaultViewCursor = i
			break
		}
	}

	// Get all existing contexts
	contexts := cfg.GetAllContexts()

	// Find current context in list (or -1 for custom)
	contextCursor := -1
	for i, ctx := range contexts {
		if ctx == cfg.DefaultContext {
			contextCursor = i
			break
		}
	}

	// Initialize column default toggles from config
	treeColumnDefaults := initColumnDefaults(cfg.TreeDefaultColumns)
	outlinerColumnDefaults := initColumnDefaults(cfg.OutlinerDefaultColumns)

	// Initialize outliner collapsed toggles from config
	var outlinerCollapsed [7]bool
	collapsedMap := cfg.GetOutlinerCollapsedSections()
	for i, sectionName := range outlinerSectionNames {
		outlinerCollapsed[i] = collapsedMap[sectionName]
	}

	// Stage input
	stageInput := textinput.New()
	stageInput.Placeholder = "Stage name"
	stageInput.CharLimit = 30
	stageInput.Width = 30

	// Copy stages from config
	stages := make([]string, len(cfg.Stages))
	copy(stages, cfg.Stages)

	// Alert setting inputs
	alertDailyReviewHourInput := textinput.New()
	alertDailyReviewHourInput.Placeholder = strconv.Itoa(config.DefaultAlertDailyReviewHour)
	alertDailyReviewHourInput.CharLimit = 2
	alertDailyReviewHourInput.Width = 6
	alertDailyReviewHourInput.SetValue(strconv.Itoa(cfg.GetAlertDailyReviewHour()))

	alertDueSoonDaysInput := textinput.New()
	alertDueSoonDaysInput.Placeholder = strconv.Itoa(config.DefaultAlertDueSoonDays)
	alertDueSoonDaysInput.CharLimit = 3
	alertDueSoonDaysInput.Width = 6
	alertDueSoonDaysInput.SetValue(strconv.Itoa(cfg.GetAlertDueSoonDays()))

	alertStaleDaysInput := textinput.New()
	alertStaleDaysInput.Placeholder = strconv.Itoa(config.DefaultAlertStaleDays)
	alertStaleDaysInput.CharLimit = 3
	alertStaleDaysInput.Width = 6
	alertStaleDaysInput.SetValue(strconv.Itoa(cfg.GetAlertStaleDays()))

	alertInboxThresholdInput := textinput.New()
	alertInboxThresholdInput.Placeholder = strconv.Itoa(config.DefaultAlertInboxThreshold)
	alertInboxThresholdInput.CharLimit = 3
	alertInboxThresholdInput.Width = 6
	alertInboxThresholdInput.SetValue(strconv.Itoa(cfg.GetAlertInboxThreshold()))

	alertBlockedDaysInput := textinput.New()
	alertBlockedDaysInput.Placeholder = strconv.Itoa(config.DefaultAlertBlockedDays)
	alertBlockedDaysInput.CharLimit = 3
	alertBlockedDaysInput.Width = 6
	alertBlockedDaysInput.SetValue(strconv.Itoa(cfg.GetAlertBlockedDays()))

	alertProjectStaleDaysInput := textinput.New()
	alertProjectStaleDaysInput.Placeholder = strconv.Itoa(config.DefaultAlertProjectStaleDays)
	alertProjectStaleDaysInput.CharLimit = 3
	alertProjectStaleDaysInput.Width = 6
	alertProjectStaleDaysInput.SetValue(strconv.Itoa(cfg.GetAlertProjectStaleDays()))

	return &SettingsModel{
		config:                     cfg,
		configManager:              manager,
		editorInput:                editorInput,
		contextInput:               contextInput,
		undoDepthInput:             undoDepthInput,
		dueSoonInput:               dueSoonInput,
		futureInput:                futureInput,
		oldDaysInput:               oldDaysInput,
		completedHoursInput:        completedHoursInput,
		recentActivityInput:        recentActivityInput,
		recentActivityShowAll:      cfg.GetRecentActivityShowAll(),
		defaultViewCursor:          defaultViewCursor,
		linkedTaskJump:             cfg.GetLinkedTaskJumpToNew(),
		maxPinnedInput:             maxPinnedInput,
		maxRecentTaskVisitsInput:   maxRecentTaskVisitsInput,
		attachmentEditorInput:      attachmentEditorInput,
		codeEditorInput:            codeEditorInput,
		outlinerCollapsed:          outlinerCollapsed,
		treeColumnDefaults:         treeColumnDefaults,
		outlinerColumnDefaults:     outlinerColumnDefaults,
		treePriorityMode:           priorityModeToCursor(cfg.TreePriorityMode),
		outlinerPriorityMode:       priorityModeToCursor(cfg.OutlinerPriorityMode),
		crossOutlinerSidebar:       cfg.CrossOutlinerSidebarEnabled == nil || *cfg.CrossOutlinerSidebarEnabled,
		savedStages:                append([]string{}, stages...),
		stages:                     stages,
		stageInput:                 stageInput,
		alertDailyReviewEnabled:    cfg.GetAlertDailyReviewEnabled(),
		alertDailyReviewHourInput:  alertDailyReviewHourInput,
		alertWeeklyReviewEnabled:   cfg.GetAlertWeeklyReviewEnabled(),
		alertWeeklyReviewDay:       cfg.GetAlertWeeklyReviewDay(),
		alertOverdueEnabled:        cfg.GetAlertOverdueEnabled(),
		alertDueSoonEnabled:        cfg.GetAlertDueSoonEnabled(),
		alertDueSoonDaysInput:      alertDueSoonDaysInput,
		alertStaleEnabled:          cfg.GetAlertStaleEnabled(),
		alertStaleDaysInput:        alertStaleDaysInput,
		alertInboxEnabled:          cfg.GetAlertInboxEnabled(),
		alertInboxThresholdInput:   alertInboxThresholdInput,
		alertBlockedEnabled:        cfg.GetAlertBlockedEnabled(),
		alertBlockedDaysInput:      alertBlockedDaysInput,
		alertProjectHealthEnabled:  cfg.GetAlertProjectHealthEnabled(),
		alertProjectStaleDaysInput: alertProjectStaleDaysInput,
		contexts:                   contexts,
		contextCursor:              contextCursor,
		focusIndex:                 0,
		tabs:                       TabState{Names: settingsTabNames},
		width:                      80,
		height:                     24,
	}
}

// SetSize sets the view dimensions.
func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SettingsModel) SupportsHelpShortcut() bool { return false }

func (m *SettingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SettingsModel) saveSettings() tea.Cmd {
	return func() tea.Msg {
		editor := strings.TrimSpace(m.editorInput.Value())
		context := strings.TrimSpace(m.contextInput.Value())
		undoDepth, _ := strconv.Atoi(strings.TrimSpace(m.undoDepthInput.Value()))
		dueSoon, _ := strconv.Atoi(strings.TrimSpace(m.dueSoonInput.Value()))
		futureDays, _ := strconv.Atoi(strings.TrimSpace(m.futureInput.Value()))
		oldDays, _ := strconv.Atoi(strings.TrimSpace(m.oldDaysInput.Value()))
		completedHours, _ := strconv.Atoi(strings.TrimSpace(m.completedHoursInput.Value()))

		m.config.Editor = editor
		m.config.DefaultContext = context
		m.config.UndoHistoryDepth = &undoDepth
		m.config.DefaultProjectView = defaultProjectViewValues[m.defaultViewCursor]
		m.config.ReviewDueSoonDays = &dueSoon
		m.config.ReviewFutureDays = &futureDays
		m.config.ReviewOldDays = &oldDays
		m.config.ReviewCompletedHours = &completedHours
		recentActivityLimit, _ := strconv.Atoi(strings.TrimSpace(m.recentActivityInput.Value()))
		m.config.RecentActivityLimit = &recentActivityLimit
		m.config.RecentActivityShowAll = &m.recentActivityShowAll
		m.config.LinkedTaskJumpToNew = &m.linkedTaskJump
		maxPinned, _ := strconv.Atoi(strings.TrimSpace(m.maxPinnedInput.Value()))
		m.config.MaxPinnedBarItems = &maxPinned
		maxRecentVisits, _ := strconv.Atoi(strings.TrimSpace(m.maxRecentTaskVisitsInput.Value()))
		m.config.MaxRecentTaskVisits = &maxRecentVisits
		m.config.AttachmentEditor = strings.TrimSpace(m.attachmentEditorInput.Value())
		m.config.CodeEditor = strings.TrimSpace(m.codeEditorInput.Value())
		// Persist column default visibility
		m.config.TreeDefaultColumns = columnDefaultsToMap(m.treeColumnDefaults)
		m.config.OutlinerDefaultColumns = columnDefaultsToMap(m.outlinerColumnDefaults)

		// Persist priority display modes (empty string = conditional, omitted from JSON)
		m.config.TreePriorityMode = priorityModeValues[m.treePriorityMode]
		m.config.OutlinerPriorityMode = priorityModeValues[m.outlinerPriorityMode]
		// Omit "conditional" from config (it's the default)
		if m.config.TreePriorityMode == PriorityModeConditional {
			m.config.TreePriorityMode = ""
		}
		if m.config.OutlinerPriorityMode == PriorityModeConditional {
			m.config.OutlinerPriorityMode = ""
		}

		// Persist cross-project outliner sidebar default
		m.config.CrossOutlinerSidebarEnabled = boolPtr(m.crossOutlinerSidebar)

		// Persist stages
		m.config.Stages = make([]string, len(m.stages))
		copy(m.config.Stages, m.stages)

		// Persist alert settings
		m.config.AlertDailyReviewEnabled = boolPtr(m.alertDailyReviewEnabled)
		m.config.AlertWeeklyReviewEnabled = boolPtr(m.alertWeeklyReviewEnabled)
		m.config.AlertOverdueEnabled = boolPtr(m.alertOverdueEnabled)
		m.config.AlertDueSoonEnabled = boolPtr(m.alertDueSoonEnabled)
		m.config.AlertStaleEnabled = boolPtr(m.alertStaleEnabled)
		m.config.AlertInboxEnabled = boolPtr(m.alertInboxEnabled)
		m.config.AlertBlockedEnabled = boolPtr(m.alertBlockedEnabled)
		m.config.AlertProjectHealthEnabled = boolPtr(m.alertProjectHealthEnabled)

		alertDailyHour, _ := strconv.Atoi(strings.TrimSpace(m.alertDailyReviewHourInput.Value()))
		m.config.AlertDailyReviewHour = &alertDailyHour
		m.config.AlertWeeklyReviewDay = &m.alertWeeklyReviewDay

		alertDueSoonDays, _ := strconv.Atoi(strings.TrimSpace(m.alertDueSoonDaysInput.Value()))
		m.config.AlertDueSoonDays = &alertDueSoonDays
		alertStaleDays, _ := strconv.Atoi(strings.TrimSpace(m.alertStaleDaysInput.Value()))
		m.config.AlertStaleDays = &alertStaleDays
		alertInboxThreshold, _ := strconv.Atoi(strings.TrimSpace(m.alertInboxThresholdInput.Value()))
		m.config.AlertInboxThreshold = &alertInboxThreshold
		alertBlockedDays, _ := strconv.Atoi(strings.TrimSpace(m.alertBlockedDaysInput.Value()))
		m.config.AlertBlockedDays = &alertBlockedDays
		alertProjectStaleDays, _ := strconv.Atoi(strings.TrimSpace(m.alertProjectStaleDaysInput.Value()))
		m.config.AlertProjectStaleDays = &alertProjectStaleDays

		// Persist outliner collapsed sections
		collapsed := make(map[string]bool)
		for i, name := range outlinerSectionNames {
			if m.outlinerCollapsed[i] {
				collapsed[name] = true
			}
		}
		if len(collapsed) > 0 {
			m.config.OutlinerCollapsedSections = collapsed
		} else {
			m.config.OutlinerCollapsedSections = nil
		}

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save settings: %w", err)}
		}

		return SettingsSavedMsg{}
	}
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

// initColumnDefaults converts a config map to a [13]bool array using AllColumns order.
// nil map or missing keys default to true (visible).
func initColumnDefaults(cfgMap map[string]bool) [13]bool {
	var result [13]bool
	for i, col := range AllColumns {
		if cfgMap != nil {
			if v, ok := cfgMap[col.ID]; ok {
				result[i] = v
				continue
			}
		}
		result[i] = true // default visible
	}
	return result
}

// columnDefaultsToMap converts a [13]bool array back to a config map.
// Returns nil if all columns are visible (to keep config clean).
func columnDefaultsToMap(defaults [13]bool) map[string]bool {
	allTrue := true
	for _, v := range defaults {
		if !v {
			allTrue = false
			break
		}
	}
	if allTrue {
		return nil
	}
	m := make(map[string]bool, 13)
	for i, col := range AllColumns {
		m[col.ID] = defaults[i]
	}
	return m
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

func (m *SettingsModel) HasTextInput() bool     { return true }
func (m *SettingsModel) ShouldCaptureEsc() bool { return true }
func (m *SettingsModel) IsFullHeight() bool     { return false }
func (m *SettingsModel) StatusBarLabel() string { return "Settings" }

// GetConfirmDialog implements ConfirmDialogProvider for the settings view.
func (m *SettingsModel) GetConfirmDialog() *ConfirmDialog {
	return &m.stageConfirmDelete
}
