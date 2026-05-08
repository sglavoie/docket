package tui

import (
	"fmt"
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// ZoomEntry records the state at a single zoom level so cursor position can
// be restored when zooming back out.
type ZoomEntry struct {
	TaskID      string // UUID of the task that was zoomed into
	CursorIndex int    // Cursor position at the moment of zoom-in
}

// CrossProjectZoomEntry records the state at a single zoom level for the
// cross-project outliner. It extends ZoomEntry with a ProjectID field to
// scope zoom resolution to the correct project, preventing UUID collisions
// across projects from causing incorrect zoom resolution.
type CrossProjectZoomEntry struct {
	TaskID      string // UUID of the task that was zoomed into
	ProjectID   string // ID of the project owning the task
	CursorIndex int    // Cursor position at the moment of zoom-in
}

// OutlinerModel is the outliner view for navigating tasks in a hierarchical,
// zoomable tree. It implements the View interface as a standalone struct
// (not extending ProjectTreeModel) to allow independent evolution of
// navigation semantics, sort modes, and zoom-stack behavior.
type OutlinerModel struct {
	BaseSearchableView
	InteractiveViewState
	PrimaryViewBundle
	MultiSelectState

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	project *models.Project

	zoomStack      []ZoomEntry             // Zoom stack for single-project mode
	crossZoomStack []CrossProjectZoomEntry // Zoom stack for cross-project mode (m.project == nil)
	zoomNotice     string                  // Transient message shown when zoom target is removed

	rows              []outlinerRow // Flattened rows, rebuilt on each render
	activePerspective int           // 0-9 index into PerspectivePresets; defaults to "All" (key 1)

	sort SortState // Current sort field and direction

	filter           FilterState      // Unified filter state (common + sections + actionable)
	scoreFilterInput ScoreFilterInput // Inline score filter threshold input

	customSectionTab  map[string]int  // Active custom section tab index per projectID (0 = All)
	expandedNodes     map[string]bool // Fold state: true=expanded (default all expanded)
	collapsedSections map[string]bool // Section collapse state for All perspective

	sidebar   *SidebarComponent   // Sidebar detail panel
	filterBar *FilterBarComponent // Filter bar component (dedicated line for filter/sort indicators)

	deleteTarget *models.Task // Task to delete when confirmation is accepted
	err          error        // Last error from an operation

	undo UndoContext // Action label and context for next undo snapshot

	// Column visibility config (loaded from project view state)
	columnConfig *ColumnConfig

	activeSavedFilter string // Name of the currently loaded saved filter (if any)

	// Active criteria filter (set when a criteria-mode SavedFilter is loaded)
	activeCriteriaFilter *config.SavedFilter

	// Inline preview mode: show description/checklist below cursor task
	previewMode bool

	// OpenAtTask, when set, positions the cursor on the given task the first
	// time rebuildRows produces a non-empty row list. Cleared after first use.
	OpenAtTask *models.Task

	// Cross-project mode fields (used when m.project == nil)
	filterProjects        map[string]bool   // Project IDs to include (empty = all)
	loadedProjects        []*models.Project // Successfully loaded projects
	crossLoading          bool              // True while cross-project load is in progress
	crossSpinnerVisible   bool              // True once the 200ms delay has elapsed during load
	crossLoadGen          int               // Generation counter; discards stale load results
	crossUndoCtx          UndoContext       // Undo context scoped to cross-project operations
	crossSpinner          SpinnerModel      // View-local spinner, independent of global save spinner
	crossQuickEditProject *models.Project   // Resolved project for quick edit in cross-project mode

	// Memoization guard for crossProjectRebuildRows
	crossLastPerspective int
	crossLastFilterStr   string
	crossLastZoomDepth   int
	crossLastTaskCount   int
	crossForceRebuild    bool // When true, bypasses the memoization guard on next rebuild
}

// primaryProjectView interface implementation for shared key dispatch.
func (m *OutlinerModel) getProject() *models.Project       { return m.project }
func (m *OutlinerModel) getConfig() *config.Config         { return m.config }
func (m *OutlinerModel) getConfigManager() *config.Manager { return m.configManager }
func (m *OutlinerModel) getCursorTask() *models.Task {
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if !row.IsSectionHeader && !row.IsSeparator {
			return row.task
		}
	}
	return nil
}
func (m *OutlinerModel) reloadFromDisk() tea.Cmd {
	if m.project == nil {
		return m.reloadCrossProjects()
	}
	if err := storage.LoadProject(m.project); err != nil {
		logging.Warn("failed to load project %s: %v", m.project.Name, err)
		m.rebuildRows()
		return func() tea.Msg { return StatusMsg(fmt.Sprintf("Failed to reload project: %v", err)) }
	}
	m.rebuildRows()
	return nil
}

// NewOutlinerModel creates a new OutlinerModel for the given project.
func NewOutlinerModel(project *models.Project, cfg *config.Config, cfgManager *config.Manager) *OutlinerModel {
	// Load column toggles from config
	var outlinerToggles map[string]bool
	var outlinerPriorityMode string
	if cfg != nil && project != nil {
		if vs, ok := cfg.ProjectViewStates[project.ID]; ok {
			outlinerToggles = vs.OutlinerColumnToggles
			outlinerPriorityMode = vs.OutlinerPriorityMode
		}
	} else if cfg != nil && project == nil {
		outlinerToggles = cfg.CrossOutlinerColumnToggles
	}
	if outlinerPriorityMode == "" && cfg != nil {
		outlinerPriorityMode = cfg.OutlinerPriorityMode
	}

	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, cfgManager),
		project:           project,
		activePerspective: 1,
		sidebar:           NewSidebarComponent(),
		filterBar:         NewFilterBarComponent(),
		expandedNodes:     make(map[string]bool),
		collapsedSections: make(map[string]bool),
		customSectionTab:  make(map[string]int),
		filterProjects:    make(map[string]bool),
		filter:            NewFilterState(),
		sort:              NewSortState(),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		columnConfig:      NewColumnConfig(outlinerToggles, cfg.OutlinerDefaultColumns, DefaultOutlinerColumns, outlinerPriorityMode),
	}
	// Seed default collapsed sections from config
	maps.Copy(m.collapsedSections, cfg.GetOutlinerCollapsedSections())
	m.InitSearchInput("Search tasks...", 100, 30)
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	// Restore sidebar and filter bar state from config
	if project != nil {
		if state, exists := cfg.GetProjectViewState(project.ID); exists {
			m.sidebar.SetEnabled(state.SidebarEnabled)
			m.filterBar.SetEnabled(state.OutlinerFilterBarEnabled)
		}
	} else {
		// Cross-project outliner: use global setting (nil = true)
		if cfg.CrossOutlinerSidebarEnabled != nil {
			m.sidebar.SetEnabled(*cfg.CrossOutlinerSidebarEnabled)
		}
		// Restore persisted collapsed sections for cross-project outliner
		if len(cfg.CrossOutlinerCollapsedSections) > 0 {
			maps.Copy(m.collapsedSections, cfg.CrossOutlinerCollapsedSections)
		}
		// Restore persisted per-task fold state for cross-project outliner
		for id := range cfg.CrossOutlinerFoldState {
			m.expandedNodes[id] = false
		}
		m.filter.Type = models.TaskType(cfg.CrossOutlinerFilterType)
		m.filter.Status = models.TaskStatus(cfg.CrossOutlinerFilterStatus)
		m.filter.Priority = cfg.CrossOutlinerFilterPriority
		m.filter.DueDate = DueDateFilter(cfg.CrossOutlinerFilterDueDate)
		m.filter.Actionable = cfg.CrossOutlinerFilterActionable
		m.filter.HideDimmed = cfg.CrossOutlinerFilterHideDimmed
		m.filter.Stage = cfg.CrossOutlinerFilterStage
		m.filter.ScoreMin = cfg.CrossOutlinerFilterScoreMin
		m.sort.Field = SortField(cfg.CrossOutlinerSortField)
		m.sort.Ascending = cfg.CrossOutlinerSortAscending
		m.filterBar.SetEnabled(cfg.CrossOutlinerFilterBarEnabled)
		if len(cfg.CrossOutlinerFilterTags) > 0 {
			m.filter.Tags = make(map[string]bool, len(cfg.CrossOutlinerFilterTags))
			for _, tag := range cfg.CrossOutlinerFilterTags {
				m.filter.Tags[tag] = true
			}
		}
		if len(cfg.CrossOutlinerFilterSections) > 0 {
			m.filter.Sections = make(map[string]bool, len(cfg.CrossOutlinerFilterSections))
			for _, section := range cfg.CrossOutlinerFilterSections {
				m.filter.Sections[section] = true
			}
		}
		if len(cfg.CrossOutlinerFilterAttrs) > 0 {
			m.filter.Attrs = maps.Clone(cfg.CrossOutlinerFilterAttrs)
		}
	}
	return m
}

// Init implements tea.Model.
func (m *OutlinerModel) Init() tea.Cmd {
	if m.project == nil {
		return m.startCrossProjectLoad()
	}
	return nil
}

// Update implements tea.Model.
func (m *OutlinerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil

	case MutationFlashClearMsg:
		m.mutationFlash = false
		m.rebuildRows()
		return m, nil

	case CommentAddedMsg:
		// Rebuild rows so cloned tasks from filterTaskTree() pick up the
		// in-place comment mutation on the original task.
		m.rebuildRows()
		return m, nil

	case ParentSelectedMsg:
		if msg.ReparentTaskID == "" || m.project == nil {
			return m, nil
		}
		task := m.project.GetTaskByID(msg.ReparentTaskID)
		if task == nil {
			return m, nil
		}
		prevStates := captureTaskStates(m.project.Tasks)
		if msg.ParentID == nil {
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.ParentID = nil
			})
		} else {
			targetID := *msg.ParentID
			taskMap := make(map[string]*models.Task, len(m.project.Tasks))
			for _, t := range m.project.Tasks {
				taskMap[t.ID] = t
			}
			targetParent := taskMap[targetID]
			if targetParent != nil && task.Type.CanBeChildOf(targetParent.Type) && !models.WouldCreateCycle(taskMap, task.ID, targetID) {
				m.project.MutateTask(task.ID, func(t *models.Task) {
					t.ParentID = &targetID
				})
			}
		}
		m.project.BuildTaskTree()
		m.setUndoContext("Reparent Task", task.Title)
		m.undo.TaskIDs = []string{task.ID}
		return m, m.saveAndRefreshBulk(prevStates)

	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case outlinerRefreshMsg:
		m.rebuildRows()
		return m, nil
	case ViewDidAppearMsg:
		return m, m.handleViewDidAppear()
	case crossProjectsLoadedMsg:
		return m, m.handleCrossProjectsLoaded(msg)
	case crossSpinnerDelayMsg:
		if m.crossLoading {
			m.crossSpinnerVisible = true
			m.crossSpinner = NewSpinner()
			return m, m.crossSpinner.Tick()
		}
		return m, nil
	case crossSpinnerTickMsg:
		if m.crossSpinnerVisible {
			var cmd tea.Cmd
			m.crossSpinner, cmd = m.crossSpinner.Update(msg.inner)
			return m, crossSpinnerTickFwd(cmd)
		}
		return m, nil
	}
	return m, nil
}
