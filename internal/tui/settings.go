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
	settingsTabNames = []string{"General", "Review", "Behavior", "Attachments", "Kanban"}

	// Focus index ranges per tab: tab 0 → fields 0-3, tab 1 → fields 4-7, tab 2 → fields 8-9, tab 3 → fields 10-11, tab 4 → fields 12-13
	tabFieldStart = []int{0, 4, 8, 10, 12}

	// defaultProjectViewOptions lists the display labels for the default project view selector.
	defaultProjectViewOptions = []string{"Outliner", "Project Tree", "Kanban"}

	// defaultProjectViewValues maps cursor index to config string value.
	defaultProjectViewValues = []string{"outliner", "project_tree", "kanban"}
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
	linkedTaskJump         bool // Whether to jump to new task after linked creation
	attachmentEditorInput  textinput.Model
	codeEditorInput        textinput.Model
	defaultViewCursor      int // Index into defaultProjectViewOptions
	kanbanMaxColumnsInput  textinput.Model
	kanbanMaxSwimlaneInput textinput.Model
	contexts               []string
	contextCursor          int // -1 for custom mode, 0+ for selecting from list

	activeTab     int
	focusIndex    int // 0=editor, 1=context, 2=undoDepth, 3=defaultView, 4=dueSoon, 5=future, 6=oldDays, 7=completedHours, 8=recentActivityLimit, 9=linkedTaskJump, 10=attachmentEditor, 11=codeEditor, 12=kanbanMaxColumns, 13=kanbanMaxSwimlaneRows
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
		defaultViewCursor:      defaultViewCursor,
		linkedTaskJump:         cfg.GetLinkedTaskJumpToNew(),
		attachmentEditorInput:  attachmentEditorInput,
		codeEditorInput:        codeEditorInput,
		kanbanMaxColumnsInput:  kanbanMaxColumnsInput,
		kanbanMaxSwimlaneInput: kanbanMaxSwimlaneInput,
		contexts:               contexts,
		contextCursor:          contextCursor,
		focusIndex:             0,
		activeTab:              0,
		width:                  80,
		height:                 24,
	}
}

// SetSize sets the view dimensions.
func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

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
		m.config.LinkedTaskJumpToNew = &m.linkedTaskJump
		m.config.AttachmentEditor = strings.TrimSpace(m.attachmentEditorInput.Value())
		m.config.CodeEditor = strings.TrimSpace(m.codeEditorInput.Value())
		kanbanMaxCols, _ := strconv.Atoi(strings.TrimSpace(m.kanbanMaxColumnsInput.Value()))
		m.config.KanbanMaxColumns = &kanbanMaxCols
		kanbanMaxRows, _ := strconv.Atoi(strings.TrimSpace(m.kanbanMaxSwimlaneInput.Value()))
		m.config.KanbanMaxSwimlaneRows = &kanbanMaxRows

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save settings: %w", err)}
		}

		return SettingsSavedMsg{}
	}
}
