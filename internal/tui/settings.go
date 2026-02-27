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
	settingsTabNames = []string{"General", "Review", "Behavior", "Attachments", "Project Tree", "Kanban", "Outliner"}

	// Focus index ranges per tab: tab 0 → fields 0-3, tab 1 → fields 4-7, tab 2 → fields 8-11, tab 3 → fields 12-13, tab 4 → fields 14-26, tab 5 → fields 27-41, tab 6 → fields 42-61
	tabFieldStart = []int{0, 4, 8, 12, 14, 27, 42}

	// outlinerSectionNames maps toggle index (0-6) to the section key used in config.
	// Active uses "" (empty string) since tasks with no explicit section are "Active".
	outlinerSectionNames = []string{"", "Inbox", "Next", "Waiting", "Review", "Standby", "Someday"}

	// outlinerSectionLabels are the display labels for each toggle.
	outlinerSectionLabels = []string{"Active", "Inbox", "Next", "Waiting", "Review", "Standby", "Someday"}

	// defaultProjectViewOptions lists the display labels for the default project view selector.
	defaultProjectViewOptions = []string{"Outliner", "Project Tree", "Kanban"}

	// defaultProjectViewValues maps cursor index to config string value.
	defaultProjectViewValues = []string{"outliner", "project_tree", "kanban"}

	// priorityModeOptions lists the display labels for priority display mode.
	priorityModeOptions = []string{"All", "Conditional", "Hidden"}

	// priorityModeValues maps cursor index to config string value.
	priorityModeValues = []string{PriorityModeAll, PriorityModeConditional, PriorityModeHidden}
)

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

	editorInput            textinput.Model
	contextInput           textinput.Model
	undoDepthInput         textinput.Model
	dueSoonInput           textinput.Model
	futureInput            textinput.Model
	oldDaysInput           textinput.Model
	completedHoursInput    textinput.Model
	recentActivityInput    textinput.Model
	recentActivityShowAll  bool // Whether to show completed tasks in recent activity by default
	linkedTaskJump         bool // Whether to jump to new task after linked creation
	maxPinnedInput         textinput.Model
	attachmentEditorInput  textinput.Model
	codeEditorInput        textinput.Model
	defaultViewCursor      int // Index into defaultProjectViewOptions
	kanbanMaxColumnsInput  textinput.Model
	kanbanMaxSwimlaneInput textinput.Model
	outlinerCollapsed      [7]bool  // Per-section default collapse state (indexed by outlinerSectionNames)
	treeColumnDefaults     [13]bool // Global default column visibility for tree view
	kanbanColumnDefaults   [13]bool // Global default column visibility for kanban view
	outlinerColumnDefaults [13]bool // Global default column visibility for outliner view
	treePriorityMode       int      // 0=All, 1=Conditional, 2=Hidden
	kanbanPriorityMode     int      // 0=All, 1=Conditional, 2=Hidden
	outlinerPriorityMode   int      // 0=All, 1=Conditional, 2=Hidden
	contexts               []string
	contextCursor          int // -1 for custom mode, 0+ for selecting from list

	tabs          TabState
	focusIndex    int // 0=editor, 1=context, 2=undoDepth, 3=defaultView, 4=dueSoon, 5=future, 6=oldDays, 7=completedHours, 8=recentActivityLimit, 9=recentActivityShowAll, 10=linkedTaskJump, 11=maxPinnedBarItems, 12=attachmentEditor, 13=codeEditor, 14-26=treeColumnDefaults, 27=kanbanMaxColumns, 28=kanbanMaxSwimlaneRows, 29-41=kanbanColumnDefaults, 42-48=outlinerSectionToggles, 49-61=outlinerColumnDefaults
	scrollOffset  int // vertical scroll offset within current tab content
	validationErr string

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

	kanbanMaxColumnsInput := textinput.New()
	kanbanMaxColumnsInput.Placeholder = strconv.Itoa(config.DefaultKanbanMaxColumns)
	kanbanMaxColumnsInput.CharLimit = 2
	kanbanMaxColumnsInput.Width = 6
	kanbanMaxColumnsInput.SetValue(strconv.Itoa(cfg.GetKanbanMaxColumns()))

	kanbanMaxSwimlaneInput := textinput.New()
	kanbanMaxSwimlaneInput.Placeholder = strconv.Itoa(config.DefaultKanbanMaxSwimlaneRows)
	kanbanMaxSwimlaneInput.CharLimit = 2
	kanbanMaxSwimlaneInput.Width = 6
	kanbanMaxSwimlaneInput.SetValue(strconv.Itoa(cfg.GetKanbanMaxSwimlaneRows()))

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
	kanbanColumnDefaults := initColumnDefaults(cfg.KanbanDefaultColumns)
	outlinerColumnDefaults := initColumnDefaults(cfg.OutlinerDefaultColumns)

	// Initialize outliner collapsed toggles from config
	var outlinerCollapsed [7]bool
	collapsedMap := cfg.GetOutlinerCollapsedSections()
	for i, sectionName := range outlinerSectionNames {
		outlinerCollapsed[i] = collapsedMap[sectionName]
	}

	return &SettingsModel{
		config:                 cfg,
		configManager:          manager,
		editorInput:            editorInput,
		contextInput:           contextInput,
		undoDepthInput:         undoDepthInput,
		dueSoonInput:           dueSoonInput,
		futureInput:            futureInput,
		oldDaysInput:           oldDaysInput,
		completedHoursInput:    completedHoursInput,
		recentActivityInput:    recentActivityInput,
		recentActivityShowAll:  cfg.GetRecentActivityShowAll(),
		defaultViewCursor:      defaultViewCursor,
		linkedTaskJump:         cfg.GetLinkedTaskJumpToNew(),
		maxPinnedInput:         maxPinnedInput,
		attachmentEditorInput:  attachmentEditorInput,
		codeEditorInput:        codeEditorInput,
		kanbanMaxColumnsInput:  kanbanMaxColumnsInput,
		kanbanMaxSwimlaneInput: kanbanMaxSwimlaneInput,
		outlinerCollapsed:      outlinerCollapsed,
		treeColumnDefaults:     treeColumnDefaults,
		kanbanColumnDefaults:   kanbanColumnDefaults,
		outlinerColumnDefaults: outlinerColumnDefaults,
		treePriorityMode:       priorityModeToCursor(cfg.TreePriorityMode),
		kanbanPriorityMode:     priorityModeToCursor(cfg.KanbanPriorityMode),
		outlinerPriorityMode:   priorityModeToCursor(cfg.OutlinerPriorityMode),
		contexts:               contexts,
		contextCursor:          contextCursor,
		focusIndex:             0,
		tabs:                   TabState{Names: settingsTabNames},
		width:                  80,
		height:                 24,
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
		m.config.AttachmentEditor = strings.TrimSpace(m.attachmentEditorInput.Value())
		m.config.CodeEditor = strings.TrimSpace(m.codeEditorInput.Value())
		kanbanMaxCols, _ := strconv.Atoi(strings.TrimSpace(m.kanbanMaxColumnsInput.Value()))
		m.config.KanbanMaxColumns = &kanbanMaxCols
		kanbanMaxRows, _ := strconv.Atoi(strings.TrimSpace(m.kanbanMaxSwimlaneInput.Value()))
		m.config.KanbanMaxSwimlaneRows = &kanbanMaxRows

		// Persist column default visibility
		m.config.TreeDefaultColumns = columnDefaultsToMap(m.treeColumnDefaults)
		m.config.KanbanDefaultColumns = columnDefaultsToMap(m.kanbanColumnDefaults)
		m.config.OutlinerDefaultColumns = columnDefaultsToMap(m.outlinerColumnDefaults)

		// Persist priority display modes (empty string = conditional, omitted from JSON)
		m.config.TreePriorityMode = priorityModeValues[m.treePriorityMode]
		m.config.KanbanPriorityMode = priorityModeValues[m.kanbanPriorityMode]
		m.config.OutlinerPriorityMode = priorityModeValues[m.outlinerPriorityMode]
		// Omit "conditional" from config (it's the default)
		if m.config.TreePriorityMode == PriorityModeConditional {
			m.config.TreePriorityMode = ""
		}
		if m.config.KanbanPriorityMode == PriorityModeConditional {
			m.config.KanbanPriorityMode = ""
		}
		if m.config.OutlinerPriorityMode == PriorityModeConditional {
			m.config.OutlinerPriorityMode = ""
		}

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

func (m *SettingsModel) HasTextInput() bool     { return true }
func (m *SettingsModel) ShouldCaptureEsc() bool { return true }
func (m *SettingsModel) IsFullHeight() bool     { return false }
func (m *SettingsModel) StatusBarLabel() string { return "Settings" }
