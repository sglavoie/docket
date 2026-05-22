package tui

import (
	"image/color"
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// KanbanModel is the top-level BubbleTea model for both per-project and
// cross-project kanban views.
type KanbanModel struct {
	config        *config.Config
	configManager *config.Manager
	svc           *ProjectService

	// Scope
	project        *models.Project // nil for cross-project mode
	crossProject   bool
	loadedProjects []*models.Project

	// Column state
	columns      []KanbanColumn
	activeCol    int // index into columns of the focused column
	colScrollOff int // horizontal scroll offset (first visible column index)

	// Current mode & enabled modes
	mode         KanbanMode
	enabledModes []KanbanMode // subset of AllKanbanModes; nil means all enabled

	// Raw card list (before distribution)
	allCards []KanbanCard

	// UI state
	detailPaneVisible bool
	showAxisManager   bool
	showModePicker    bool
	axisCursor        int // cursor position within axis manager popup
	width             int
	height            int
	accentColor       color.Color

	// Dispatcher routes key events through the declarative KeyMap.
	Dispatcher *keymap.Dispatcher

	// Multi-select state
	MultiSelectState

	// Filter state
	FilterMixin

	// Perspective lens (cross-project mode only). PerspectiveInactive (-1)
	// means no lens is applied; 0..9 selects a preset from PerspectivePresets().
	activePerspective int

	// Status message (temporary, auto-clears)
	statusMsg string

	// Flash animation: task ID briefly highlighted in the target column after H/L move.
	flashCardID string

	// openLoops mirrors AppModel.openLoops for the title-row Zeigarnik counter.
	openLoops OpenLoops
}

// setOpenLoops injects the latest cross-project open-loop counts into the view.
func (m *KanbanModel) setOpenLoops(o OpenLoops) { m.openLoops = o }

// NewKanbanModel creates a per-project kanban view.
func NewKanbanModel(
	project *models.Project,
	cfg *config.Config,
	cfgMgr *config.Manager,
	svc *ProjectService,
) *KanbanModel {
	m := newKanbanBase(cfg, cfgMgr, svc)
	m.project = project
	m.loadedProjects = []*models.Project{project}
	m.load()
	return m
}

// NewCrossProjectKanbanModel creates a cross-project kanban view.
func NewCrossProjectKanbanModel(
	cfg *config.Config,
	cfgMgr *config.Manager,
	svc *ProjectService,
) *KanbanModel {
	m := newKanbanBase(cfg, cfgMgr, svc)
	m.crossProject = true
	_, projects, _ := LoadCrossScopeTasks(cfg, svc, CrossScopeOpts{ActiveOnly: true})
	m.loadedProjects = projects
	m.load()
	return m
}

// newKanbanBase initializes fields common to both constructors.
func newKanbanBase(cfg *config.Config, cfgMgr *config.Manager, svc *ProjectService) *KanbanModel {
	ks := cfg.KanbanSettings
	mode := KanbanModeFromID(ks.DefaultMode)
	if ks.DefaultMode == "" {
		mode = KanbanModeSection
	}

	m := &KanbanModel{
		config:            cfg,
		configManager:     cfgMgr,
		svc:               svc,
		mode:              mode,
		FilterMixin:       FilterMixin{filter: newFilterState()},
		activePerspective: PerspectiveInactive,
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)

	// Resolve enabled modes from config.
	if len(ks.EnabledModes) > 0 {
		for _, id := range ks.EnabledModes {
			m.enabledModes = append(m.enabledModes, KanbanModeFromID(id))
		}
	}

	// Detail pane: nil → hidden by default.
	if ks.DetailPaneVisible != nil {
		m.detailPaneVisible = *ks.DetailPaneVisible
	}

	return m
}

// load synchronously loads tasks and rebuilds columns.
func (m *KanbanModel) load() {
	m.allCards = loadKanbanItems(m.loadedProjects, m.svc)
	m.rebuildCols()
	m.initStartColumn()
}

// initStartColumn positions activeCol on the configured start column.
// Falls back to the first non-empty column, then column 0.
func (m *KanbanModel) initStartColumn() {
	if len(m.columns) == 0 {
		return
	}
	target := m.config.KanbanSettings.DefaultStartColumn
	if target == "" {
		target = "inbox"
	}
	// Try the configured column if it has tasks.
	for i, col := range m.columns {
		if col.ID == target && len(col.Tasks) > 0 {
			m.activeCol = i
			m.adjustHorizontalScroll()
			return
		}
	}
	// Fall back to first non-empty column.
	for i, col := range m.columns {
		if len(col.Tasks) > 0 {
			m.activeCol = i
			m.adjustHorizontalScroll()
			return
		}
	}
	// Last resort: column 0.
	m.activeCol = 0
}

// getCursorCard returns the KanbanCard currently under the cursor in the
// active column, or nil if the column is empty.
func (m *KanbanModel) getCursorCard() *KanbanCard {
	if m.activeCol < 0 || m.activeCol >= len(m.columns) {
		return nil
	}
	col := &m.columns[m.activeCol]
	if len(col.Tasks) == 0 || col.Cursor < 0 || col.Cursor >= len(col.Tasks) {
		return nil
	}
	return &col.Tasks[col.Cursor]
}

// filteredCards returns m.allCards filtered by the active CommonFilterState.
// Canceled tasks are excluded in all modes except Status. In cross-project
// mode an active perspective lens further narrows the result.
func (m *KanbanModel) filteredCards() []KanbanCard {
	hideCanceled := m.mode != KanbanModeStatus
	hasFilters := hasActiveCommonFilters(&m.filter)
	var perspFilter func(*models.Task) bool
	if m.crossProject {
		perspFilter = PerspectiveLensFilter(m.activePerspective)
	}
	if !hideCanceled && !hasFilters && perspFilter == nil {
		return m.allCards
	}
	result := make([]KanbanCard, 0, len(m.allCards))
	for _, card := range m.allCards {
		if hideCanceled && card.Task.Status == models.StatusCanceled {
			continue
		}
		if hasFilters {
			projectID := ""
			if card.Project != nil {
				projectID = card.Project.ID
			}
			if !passesCommonFilters(card.Task, &m.filter, projectID, m.config) {
				continue
			}
		}
		if perspFilter != nil && !perspFilter(card.Task) {
			continue
		}
		result = append(result, card)
	}
	return result
}

// rebuildCols distributes m.allCards into columns for the current mode.
func (m *KanbanModel) rebuildCols() {
	ks := m.config.KanbanSettings
	stages := collectStages(m.config.Stages, m.loadedProjects)
	customSections := collectCustomSections(m.allCards)

	m.columns = rebuildColumns(
		m.filteredCards(),
		m.mode,
		m.columns,
		ks.SectionColumnOrder,
		ks.StatusColumnOrder,
		stages,
		customSections,
		m.config,
	)

	// Clamp active column index.
	if m.activeCol >= len(m.columns) {
		m.activeCol = max(0, len(m.columns)-1)
	}
}

// Init satisfies the tea.Model interface.
func (m *KanbanModel) Init() tea.Cmd {
	return nil
}

// View satisfies the tea.Model interface.
func (m *KanbanModel) View() tea.View {
	return tea.NewView(kanbanRender(m))
}

// SetSize satisfies the View interface.
func (m *KanbanModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetAccentColor satisfies the AccentColorSetter interface.
func (m *KanbanModel) SetAccentColor(c color.Color) { m.accentColor = c }

// AccentColor returns the current accent color.
func (m *KanbanModel) AccentColor() color.Color {
	if m.accentColor != nil {
		return m.accentColor
	}
	return ColorPrimary
}

// IsFullHeight returns true — kanban renders its own full-height content.
func (m *KanbanModel) IsFullHeight() bool { return true }

// HasTextInput returns false — no text input is active for now.
func (m *KanbanModel) HasTextInput() bool { return false }

// HasPendingPrefix returns true while a multi-key prefix sequence is active.
func (m *KanbanModel) HasPendingPrefix() bool {
	return m.Dispatcher != nil && m.Dispatcher.HasPendingPrefix()
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *KanbanModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix() {
		return nil
	}
	prefix := m.Dispatcher.PendingPrefix()
	var currentStage string
	var currentPriority int
	if task, _ := m.getTaskRef(); task != nil {
		currentStage = task.Stage
		currentPriority = task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	// In Kanban, starting a task also moves its card to the In Progress column
	hints[","] = overrideHintDetail(hints[","], "s", "move to In Progress column")
	hints["g"] = KanbanNavigationPrefixHints
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true))
	return &PrefixPopupInfo{
		Prefix: prefix,
		Hints:  hints,
	}
}

// KanbanNavigationPrefixHints contains "g" prefix hints for kanban views.
var KanbanNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to top of column"},
	{Key: "p", Description: "project", Detail: "Go to task in project"},
}

// ShouldCaptureEsc returns true when the axis manager popup, multi-select mode,
// or another modal sub-component is open, preventing the global Esc handler
// from popping the view.
func (m *KanbanModel) ShouldCaptureEsc() bool {
	return m.showAxisManager || m.showModePicker || m.multiSelectMode || (m.Dispatcher != nil && m.Dispatcher.HasPendingPrefix())
}

// enabledModeList returns the ordered list of modes that are currently enabled.
// If m.enabledModes is nil (no restriction), all modes are returned.
func (m *KanbanModel) enabledModeList() []KanbanMode {
	if len(m.enabledModes) == 0 {
		return AllKanbanModes()
	}
	return m.enabledModes
}
