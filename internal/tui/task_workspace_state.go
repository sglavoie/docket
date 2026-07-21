package tui

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TaskIdentity is the stable cursor and selection key shared by every Task
// Workspace layout. ProjectID is part of the key even though task IDs are
// normally workspace-unique, so adapters never have to infer ownership from a
// row index or from the currently selected project.
type TaskIdentity struct {
	ProjectID string
	TaskID    string
}

func (id TaskIdentity) Valid() bool { return id.ProjectID != "" && id.TaskID != "" }

// TaskWorkspaceGroupBy is the layout-neutral grouping selected through
// View -> Organize. Layouts decide how a group is rendered, but not how it is
// defined.
type TaskWorkspaceGroupBy uint8

const (
	WorkspaceGroupNone TaskWorkspaceGroupBy = iota
	WorkspaceGroupProject
	WorkspaceGroupStatus
	WorkspaceGroupSection
	WorkspaceGroupPriority
	WorkspaceGroupDueDate
)

// TaskWorkspaceColumn is a stable column identity. Layout adapters render the
// selected subset in this order; presentation widths remain layout-local.
type TaskWorkspaceColumn string

const (
	WorkspaceColumnTitle    TaskWorkspaceColumn = "title"
	WorkspaceColumnProject  TaskWorkspaceColumn = "project"
	WorkspaceColumnStatus   TaskWorkspaceColumn = "status"
	WorkspaceColumnPriority TaskWorkspaceColumn = "priority"
	WorkspaceColumnDueDate  TaskWorkspaceColumn = "due-date"
	WorkspaceColumnScore    TaskWorkspaceColumn = "score"
)

var allTaskWorkspaceColumns = []TaskWorkspaceColumn{
	WorkspaceColumnTitle,
	WorkspaceColumnProject,
	WorkspaceColumnStatus,
	WorkspaceColumnPriority,
	WorkspaceColumnDueDate,
	WorkspaceColumnScore,
}

// TaskWorkspaceBoardAxis is the shared Board organization mode.
type TaskWorkspaceBoardAxis uint8

const (
	WorkspaceBoardByStatus TaskWorkspaceBoardAxis = iota
	WorkspaceBoardBySection
	WorkspaceBoardByPriority
	WorkspaceBoardByScore
)

// TaskWorkspaceItem is the presentation-neutral row/card/node consumed by all
// three layout adapters.
type TaskWorkspaceItem struct {
	Identity TaskIdentity
	Task     *models.Task
	Project  *models.Project
}

// TaskWorkspaceGroup is a query-produced grouping. Key is stable for collapse
// bookkeeping while Label is display text.
type TaskWorkspaceGroup struct {
	Key   string
	Label string
	Items []TaskWorkspaceItem
}

type TaskWorkspaceSavedFilterOption struct {
	ID   string
	Name string
}

// TaskWorkspaceSnapshot is one reconciled query result. It is immutable by
// convention: slices returned by the controller are copied before exposure.
type TaskWorkspaceSnapshot struct {
	Items        []TaskWorkspaceItem
	Groups       []TaskWorkspaceGroup
	SavedFilters []TaskWorkspaceSavedFilterOption
	Automation   *config.AutomationConfig
}

func (s TaskWorkspaceSnapshot) Item(id TaskIdentity) (TaskWorkspaceItem, bool) {
	for _, item := range s.Items {
		if item.Identity == id {
			return item, true
		}
	}
	return TaskWorkspaceItem{}, false
}

// TaskWorkspaceState centralizes every query and organization input shared by
// Outline, List, and Board. Selection, cursor, and zoom use TaskIdentity rather
// than row positions so layout and scope changes can reconcile deterministically.
type TaskWorkspaceState struct {
	Scope         EntityScope
	Layout        TaskWorkspaceLayout
	Focus         TaskWorkspaceFocus
	SavedFilterID string

	Grouping        TaskWorkspaceGroupBy
	Sorting         SortState
	Columns         []TaskWorkspaceColumn
	BoardAxis       TaskWorkspaceBoardAxis
	SidebarVisible  bool
	PreviewVisible  bool
	AdvancedFilters FilterState
	SearchQuery     string

	selection map[TaskIdentity]struct{}
	cursor    TaskIdentity
	hasCursor bool
	zoomRoot  TaskIdentity
	hasZoom   bool
	visible   []TaskIdentity
}

// NewTaskWorkspaceState validates route parameters and installs canonical
// organization defaults. The returned value is ready for an initial query.
func NewTaskWorkspaceState(params TaskWorkspaceParams) (*TaskWorkspaceState, error) {
	route, err := NewRoute(TaskWorkspace, params)
	if err != nil {
		return nil, err
	}
	canonical := route.Params().(TaskWorkspaceParams)
	return &TaskWorkspaceState{
		Scope:           canonical.Scope,
		Layout:          canonical.Layout,
		Focus:           canonical.Focus,
		SavedFilterID:   canonical.SavedFilterID,
		Sorting:         NewSortState(),
		Columns:         []TaskWorkspaceColumn{WorkspaceColumnTitle, WorkspaceColumnStatus, WorkspaceColumnPriority, WorkspaceColumnDueDate},
		BoardAxis:       WorkspaceBoardByStatus,
		AdvancedFilters: NewFilterState(),
		selection:       make(map[TaskIdentity]struct{}),
	}, nil
}

// RouteParams projects the current shared query identity back to the typed
// route. Transient organization and selection state deliberately stay outside
// route history.
func (s *TaskWorkspaceState) RouteParams() TaskWorkspaceParams {
	return TaskWorkspaceParams{
		Scope: s.Scope, Layout: s.Layout, Focus: s.Focus, SavedFilterID: s.SavedFilterID,
	}
}

// SetRouteParams changes the shared route identity without resetting cursor or
// selection. The next reconciliation retains visible identities and prunes only
// entities hidden by the new query.
func (s *TaskWorkspaceState) SetRouteParams(params TaskWorkspaceParams) error {
	route, err := NewRoute(TaskWorkspace, params)
	if err != nil {
		return err
	}
	p := route.Params().(TaskWorkspaceParams)
	s.Scope, s.Layout, s.Focus, s.SavedFilterID = p.Scope, p.Layout, p.Focus, p.SavedFilterID
	if s.hasZoom && p.Scope.Kind == ScopeProject && s.zoomRoot.ProjectID != p.Scope.ProjectID {
		s.ClearZoom()
	}
	return nil
}

func (s *TaskWorkspaceState) SetLayout(layout TaskWorkspaceLayout) error {
	if layout > WorkspaceBoard {
		return fmt.Errorf("unknown task workspace layout %d", layout)
	}
	s.Layout = layout
	return nil
}

func (s *TaskWorkspaceState) SetFocus(focus TaskWorkspaceFocus, savedFilterID string) error {
	params := s.RouteParams()
	params.Focus, params.SavedFilterID = focus, savedFilterID
	return s.SetRouteParams(params)
}

func (s *TaskWorkspaceState) SetScope(scope EntityScope) error {
	params := s.RouteParams()
	params.Scope = scope
	return s.SetRouteParams(params)
}

func (s *TaskWorkspaceState) SetGrouping(grouping TaskWorkspaceGroupBy) error {
	if grouping > WorkspaceGroupDueDate {
		return fmt.Errorf("unknown task workspace grouping %d", grouping)
	}
	s.Grouping = grouping
	return nil
}

func (s *TaskWorkspaceState) SetSorting(sorting SortState) error {
	if sorting.Field < SortByDefault || sorting.Field > SortByScore {
		return fmt.Errorf("unknown task workspace sort field %d", sorting.Field)
	}
	s.Sorting = sorting
	return nil
}

func (s *TaskWorkspaceState) SetColumns(columns []TaskWorkspaceColumn) error {
	seen := make(map[TaskWorkspaceColumn]struct{}, len(columns))
	for _, column := range columns {
		if !slices.Contains(allTaskWorkspaceColumns, column) {
			return fmt.Errorf("unknown task workspace column %q", column)
		}
		if _, duplicate := seen[column]; duplicate {
			return fmt.Errorf("duplicate task workspace column %q", column)
		}
		seen[column] = struct{}{}
	}
	s.Columns = slices.Clone(columns)
	return nil
}

func (s *TaskWorkspaceState) SetBoardAxis(axis TaskWorkspaceBoardAxis) error {
	if axis > WorkspaceBoardByScore {
		return fmt.Errorf("unknown task workspace board axis %d", axis)
	}
	s.BoardAxis = axis
	return nil
}

// SetSearchQuery updates the shared text query used by every layout. Search is
// session-local presentation state, but it belongs to the workspace rather
// than to Outline or List so switching layouts cannot silently change the
// result set.
func (s *TaskWorkspaceState) SetSearchQuery(query string) {
	s.SearchQuery = query
}

func (s *TaskWorkspaceState) Cursor() (TaskIdentity, bool) { return s.cursor, s.hasCursor }

func (s *TaskWorkspaceState) SetCursor(id TaskIdentity) bool {
	if !id.Valid() || !slices.Contains(s.visible, id) {
		return false
	}
	s.cursor, s.hasCursor = id, true
	return true
}

func (s *TaskWorkspaceState) MoveCursor(delta int) {
	if len(s.visible) == 0 {
		s.cursor, s.hasCursor = TaskIdentity{}, false
		return
	}
	idx := slices.Index(s.visible, s.cursor)
	if idx < 0 {
		idx = 0
	}
	idx = max(0, min(len(s.visible)-1, idx+delta))
	s.cursor, s.hasCursor = s.visible[idx], true
}

func (s *TaskWorkspaceState) ZoomRoot() (TaskIdentity, bool) { return s.zoomRoot, s.hasZoom }

func (s *TaskWorkspaceState) SetZoomRoot(id TaskIdentity) bool {
	if !id.Valid() || !slices.Contains(s.visible, id) {
		return false
	}
	s.zoomRoot, s.hasZoom = id, true
	return true
}

func (s *TaskWorkspaceState) ClearZoom() {
	s.zoomRoot, s.hasZoom = TaskIdentity{}, false
}

func (s *TaskWorkspaceState) ToggleSelection(id TaskIdentity) bool {
	if !id.Valid() || !slices.Contains(s.visible, id) {
		return false
	}
	if _, selected := s.selection[id]; selected {
		delete(s.selection, id)
	} else {
		s.selection[id] = struct{}{}
	}
	return true
}

func (s *TaskWorkspaceState) ClearSelection() { clear(s.selection) }

func (s *TaskWorkspaceState) Selected() []TaskIdentity {
	result := make([]TaskIdentity, 0, len(s.selection))
	for _, id := range s.visible {
		if _, selected := s.selection[id]; selected {
			result = append(result, id)
		}
	}
	return result
}

// Targets returns the visible multi-selection, or the cursor identity when no
// selection is active. Mutation adapters use this one rule for all layouts.
func (s *TaskWorkspaceState) Targets() []TaskIdentity {
	if selected := s.Selected(); len(selected) > 0 {
		return selected
	}
	if s.hasCursor {
		return []TaskIdentity{s.cursor}
	}
	return nil
}

// reconcileVisible preserves the cursor by identity, selects the first result
// only when the prior cursor disappeared, and prunes selections that the new
// scope/focus/filter query hid.
func (s *TaskWorkspaceState) reconcileVisible(items []TaskWorkspaceItem) {
	visible := make([]TaskIdentity, 0, len(items))
	visibleSet := make(map[TaskIdentity]struct{}, len(items))
	for _, item := range items {
		if !item.Identity.Valid() {
			continue
		}
		visible = append(visible, item.Identity)
		visibleSet[item.Identity] = struct{}{}
	}
	for id := range s.selection {
		if _, ok := visibleSet[id]; !ok {
			delete(s.selection, id)
		}
	}
	if _, ok := visibleSet[s.cursor]; !s.hasCursor || !ok {
		if len(visible) == 0 {
			s.cursor, s.hasCursor = TaskIdentity{}, false
		} else {
			s.cursor, s.hasCursor = visible[0], true
		}
	}
	s.visible = visible
}

// applyPresentationOrder installs the navigable order produced by the active
// layout after the shared query has reconciled scope/filter visibility. Folded
// rows disappear from navigation without deleting their selection; unfolding
// restores that selection as long as the task still belongs to the query.
func (s *TaskWorkspaceState) applyPresentationOrder(visible []TaskIdentity) {
	s.visible = slices.Clone(visible)
	if len(s.visible) == 0 {
		s.cursor, s.hasCursor = TaskIdentity{}, false
		return
	}
	if !s.hasCursor || !slices.Contains(s.visible, s.cursor) {
		s.cursor, s.hasCursor = s.visible[0], true
	}
}

// TaskWorkspaceMode is the contract implemented by Outline, List, and Board.
// The controller owns query and identity state; a mode only applies a snapshot
// and reports which layout it renders.
type TaskWorkspaceMode interface {
	WorkspaceLayout() TaskWorkspaceLayout
	ApplyWorkspaceSnapshot(*TaskWorkspaceState, TaskWorkspaceSnapshot)
}
