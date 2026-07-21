package tui

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TaskWorkspaceData is one revision-consistent query input. The source owns
// loading; the query owns filtering, sorting, grouping, and identity creation.
type TaskWorkspaceData struct {
	Projects     []*models.Project
	SavedFilters []config.SavedFilter
	Recent       []config.RecentTaskVisit
	FocusQueue   []config.FocusQueueItem
	Automation   *config.AutomationConfig
}

// TaskWorkspaceDataSource is the read adapter consumed by the shared query.
// Layouts never load projects or configuration directly.
type TaskWorkspaceDataSource interface {
	LoadTaskWorkspaceData(EntityScope) (TaskWorkspaceData, error)
}

type taskWorkspaceProjectLoader interface {
	Load(*models.Project) error
}

// RuntimeTaskWorkspaceSource adapts RuntimeState and the existing project
// service to the shared query. Every layout therefore observes the same scope,
// saved-filter, recent-task, and automation inputs.
type RuntimeTaskWorkspaceSource struct {
	runtime *config.RuntimeState
	loader  taskWorkspaceProjectLoader
}

func NewRuntimeTaskWorkspaceSource(runtime *config.RuntimeState, loader taskWorkspaceProjectLoader) *RuntimeTaskWorkspaceSource {
	return &RuntimeTaskWorkspaceSource{runtime: runtime, loader: loader}
}

func (s *RuntimeTaskWorkspaceSource) LoadTaskWorkspaceData(scope EntityScope) (TaskWorkspaceData, error) {
	if s == nil || s.runtime == nil {
		return TaskWorkspaceData{}, fmt.Errorf("task workspace runtime is unavailable")
	}
	var projects []*models.Project
	switch scope.Kind {
	case ScopeApp:
		projects = s.runtime.ActiveProjects()
	case ScopeProject:
		for _, project := range s.runtime.ActiveProjects() {
			if project != nil && project.ID == scope.ProjectID {
				projects = []*models.Project{project}
				break
			}
		}
		if len(projects) == 0 {
			return TaskWorkspaceData{}, fmt.Errorf("task workspace project %q is unavailable", scope.ProjectID)
		}
	default:
		return TaskWorkspaceData{}, fmt.Errorf("unsupported task workspace scope %d", scope.Kind)
	}

	var loadErrors []error
	for _, project := range projects {
		if project == nil || s.loader == nil {
			continue
		}
		if err := s.loader.Load(project); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("load project %s: %w", project.ID, err))
		}
	}
	if err := errors.Join(loadErrors...); err != nil {
		return TaskWorkspaceData{}, err
	}
	return TaskWorkspaceData{
		Projects:     projects,
		SavedFilters: slices.Clone(s.runtime.Planning().GetSavedFilters()),
		Recent:       slices.Clone(s.runtime.Machine().GetRecentTaskVisits()),
		FocusQueue:   slices.Clone(s.runtime.Planning().GetFocusQueue()),
		Automation:   s.runtime.Automation(),
	}, nil
}

// TaskWorkspaceQuery implements the shared scope/focus/filter pipeline.
type TaskWorkspaceQuery struct {
	source TaskWorkspaceDataSource
	now    func() time.Time
}

func NewTaskWorkspaceQuery(source TaskWorkspaceDataSource) TaskWorkspaceQuery {
	return TaskWorkspaceQuery{source: source, now: time.Now}
}

func (q TaskWorkspaceQuery) Run(state *TaskWorkspaceState) (TaskWorkspaceSnapshot, error) {
	if state == nil {
		return TaskWorkspaceSnapshot{}, fmt.Errorf("task workspace state is required")
	}
	if q.source == nil {
		return TaskWorkspaceSnapshot{}, fmt.Errorf("task workspace data source is required")
	}
	data, err := q.source.LoadTaskWorkspaceData(state.Scope)
	if err != nil {
		return TaskWorkspaceSnapshot{}, err
	}
	now := time.Now()
	if q.now != nil {
		now = q.now()
	}
	automation := data.Automation
	if automation == nil {
		automation = &config.AutomationConfig{}
	}
	items := workspaceItems(data.Projects)
	filter, err := taskWorkspaceSavedFilter(state, data.SavedFilters)
	if err != nil {
		return TaskWorkspaceSnapshot{}, err
	}
	recentRank := taskWorkspaceRecentRanks(data.Recent)
	items = slices.DeleteFunc(items, func(item TaskWorkspaceItem) bool {
		return !taskWorkspaceFocusMatches(item, state.Focus, filter, recentRank, automation, now) ||
			!taskWorkspaceAdvancedFilterMatches(item, &state.AdvancedFilters, automation) ||
			!taskWorkspaceSearchMatches(item, state.SearchQuery)
	})
	items = taskWorkspaceZoomItems(items, state)
	taskWorkspaceSortItems(items, state.Sorting, automation, recentRank, taskWorkspaceFocusQueueRanks(data.FocusQueue), state.Focus)
	state.reconcileVisible(items)
	options := make([]TaskWorkspaceSavedFilterOption, len(data.SavedFilters))
	for i, filter := range data.SavedFilters {
		options[i] = TaskWorkspaceSavedFilterOption{ID: filter.ID, Name: filter.Name}
	}
	return TaskWorkspaceSnapshot{
		Items: items, Groups: taskWorkspaceGroups(items, state.Grouping, now), SavedFilters: options, Automation: automation,
	}, nil
}

func taskWorkspaceSearchMatches(item TaskWorkspaceItem, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	if item.Task == nil {
		return false
	}
	return strings.Contains(strings.ToLower(item.Task.Title), query) ||
		strings.Contains(strings.ToLower(item.Task.Description), query) ||
		(item.Project != nil && strings.Contains(strings.ToLower(item.Project.Name), query))
}

func workspaceItems(projects []*models.Project) []TaskWorkspaceItem {
	var items []TaskWorkspaceItem
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.ID == "" {
				continue
			}
			items = append(items, TaskWorkspaceItem{
				Identity: TaskIdentity{ProjectID: project.ID, TaskID: task.ID}, Task: task, Project: project,
			})
		}
	}
	return items
}

func taskWorkspaceSavedFilter(state *TaskWorkspaceState, filters []config.SavedFilter) (*config.SavedFilter, error) {
	if state.Focus != WorkspaceSavedFilter {
		return nil, nil
	}
	for i := range filters {
		if filters[i].ID == state.SavedFilterID {
			return &filters[i], nil
		}
	}
	return nil, fmt.Errorf("task workspace saved filter %q was not found", state.SavedFilterID)
}

func taskWorkspaceFocusMatches(
	item TaskWorkspaceItem,
	focus TaskWorkspaceFocus,
	filter *config.SavedFilter,
	recent map[TaskIdentity]int,
	automation *config.AutomationConfig,
	now time.Time,
) bool {
	task, project := item.Task, item.Project
	if task == nil {
		return false
	}
	switch focus {
	case WorkspaceActive:
		return task.InSection("") && !task.IsSnoozedAt(now)
	case WorkspaceAll:
		return !task.IsTerminalState() && !task.IsSnoozedAt(now)
	case WorkspaceInbox:
		return task.IsInState(models.StateInbox) && !task.IsSnoozedAt(now)
	case WorkspaceNext:
		return task.IsInState(models.StateNext) && !task.IsSnoozedAt(now)
	case WorkspaceWaiting:
		return task.IsInState(models.StateWaiting) && !task.IsSnoozedAt(now)
	case WorkspaceReview:
		return task.IsInState(models.StateReview) && !task.IsSnoozedAt(now)
	case WorkspaceSomeday:
		return task.IsInState(models.StateSomeday) && !task.IsSnoozedAt(now)
	case WorkspaceCompleted:
		return task.IsTerminalState()
	case WorkspaceDueSoon:
		return taskWorkspaceDueSoon(task, now, 7)
	case WorkspaceHighPriority:
		return !task.IsTerminalState() && task.Priority <= 2
	case WorkspaceBlocked:
		return !task.IsTerminalState() && task.IsDependencyBlocked(project)
	case WorkspaceParked:
		return !task.IsTerminalState() && task.IsParked()
	case WorkspaceSnoozed:
		return !task.IsTerminalState() && task.IsSnoozedAt(now)
	case WorkspacePinned:
		return !task.IsTerminalState() && task.Pinned
	case WorkspaceRecent:
		_, ok := recent[item.Identity]
		return ok
	case WorkspaceSavedFilter:
		return taskWorkspaceSavedFilterMatches(task, project, filter, automation)
	default:
		return false
	}
}

func taskWorkspaceDueSoon(task *models.Task, now time.Time, days int) bool {
	if task == nil || task.DueDate == nil || task.IsTerminalState() {
		return false
	}
	if task.HasTime {
		return !task.DueDate.Before(now) && !task.DueDate.After(now.AddDate(0, 0, days))
	}
	today := util.StartOfDay(now)
	due := util.StartOfDay(*task.DueDate)
	return !due.Before(today) && !due.After(today.AddDate(0, 0, days))
}

func taskWorkspaceSavedFilterMatches(task *models.Task, project *models.Project, filter *config.SavedFilter, automation *config.AutomationConfig) bool {
	if filter == nil || (task.IsTerminalState() && !filter.IncludeCompleted) {
		return false
	}
	if filter.Mode == "criteria" {
		return passesSavedFilterCriteria(task, project, filter, automation)
	}
	tags := make(map[string]bool, len(filter.FilterTags))
	for _, tag := range filter.FilterTags {
		tags[tag] = true
	}
	common := CommonFilterState{
		Type:        models.TaskType(filter.FilterType),
		Status:      models.TaskStatus(filter.FilterStatus),
		Priority:    filter.FilterPriority,
		Tags:        tags,
		DueDate:     DueDateFilter(filter.FilterDueDate),
		Attrs:       filter.FilterAttrs,
		ScoreMin:    filter.FilterScoreMin,
		Actionable:  filter.FilterActionable,
		HideDimmed:  filter.FilterHideDimmed,
		GitHubIssue: filter.FilterGitHubIssue,
		Parked:      filter.FilterParked,
	}
	if !passesCommonFilters(task, &common, project, automation) ||
		(common.Actionable && !taskIsActionableState(task, project)) {
		return false
	}
	if len(filter.FilterSections) > 0 && !slices.Contains(filter.FilterSections, taskSectionBucket(task)) {
		return false
	}
	return true
}

func taskWorkspaceAdvancedFilterMatches(item TaskWorkspaceItem, filter *FilterState, automation *config.AutomationConfig) bool {
	if filter == nil {
		return true
	}
	if !passesCommonFilters(item.Task, &filter.CommonFilterState, item.Project, automation) ||
		(filter.Actionable && !taskIsActionableState(item.Task, item.Project)) {
		return false
	}
	if len(filter.Sections) > 0 && !filter.Sections[taskSectionBucket(item.Task)] {
		return false
	}
	return true
}

func taskWorkspaceRecentRanks(visits []config.RecentTaskVisit) map[TaskIdentity]int {
	result := make(map[TaskIdentity]int, len(visits))
	for i, visit := range visits {
		id := TaskIdentity{ProjectID: visit.ProjectID, TaskID: visit.TaskID}
		if _, exists := result[id]; id.Valid() && !exists {
			result[id] = i
		}
	}
	return result
}

func taskWorkspaceFocusQueueRanks(items []config.FocusQueueItem) map[TaskIdentity]int {
	ranks := make(map[TaskIdentity]int, len(items))
	for i, item := range items {
		id := TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}
		if !id.Valid() {
			continue
		}
		if _, exists := ranks[id]; !exists {
			ranks[id] = i
		}
	}
	return ranks
}

func taskWorkspaceZoomItems(items []TaskWorkspaceItem, state *TaskWorkspaceState) []TaskWorkspaceItem {
	root, ok := state.ZoomRoot()
	if !ok {
		return items
	}
	byID := make(map[TaskIdentity]TaskWorkspaceItem, len(items))
	for _, item := range items {
		byID[item.Identity] = item
	}
	if _, exists := byID[root]; !exists {
		state.ClearZoom()
		return items
	}
	return slices.DeleteFunc(items, func(item TaskWorkspaceItem) bool {
		current := item
		visited := make(map[TaskIdentity]struct{})
		for {
			if current.Identity == root {
				return false
			}
			if current.Task == nil || current.Task.ParentID == nil || *current.Task.ParentID == "" {
				return true
			}
			parentID := TaskIdentity{ProjectID: current.Identity.ProjectID, TaskID: *current.Task.ParentID}
			if _, cycle := visited[parentID]; cycle {
				return true
			}
			visited[parentID] = struct{}{}
			parent, exists := byID[parentID]
			if !exists {
				return true
			}
			current = parent
		}
	})
}

func taskWorkspaceSortItems(
	items []TaskWorkspaceItem,
	sorting SortState,
	automation *config.AutomationConfig,
	recent map[TaskIdentity]int,
	focusQueue map[TaskIdentity]int,
	focus TaskWorkspaceFocus,
) {
	if sorting.Field == SortByDefault {
		if focus == WorkspaceRecent {
			sort.SliceStable(items, func(i, j int) bool { return recent[items[i].Identity] < recent[items[j].Identity] })
		} else if focus == WorkspaceNext && len(focusQueue) > 0 {
			sort.SliceStable(items, func(i, j int) bool {
				left, leftIndexed := focusQueue[items[i].Identity]
				right, rightIndexed := focusQueue[items[j].Identity]
				switch {
				case leftIndexed != rightIndexed:
					return leftIndexed
				case leftIndexed:
					return left < right
				case items[i].Task.Priority != items[j].Task.Priority:
					return items[i].Task.Priority < items[j].Task.Priority
				default:
					return items[i].Task.UpdatedAt.After(items[j].Task.UpdatedAt)
				}
			})
		}
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		comparison := CompareTasksCrossProject(
			items[i].Task, items[j].Task, sorting.Field,
			items[i].Identity.ProjectID, items[j].Identity.ProjectID, automation,
		)
		return ApplySortDirection(comparison, sorting.Ascending)
	})
}

func taskWorkspaceGroups(items []TaskWorkspaceItem, grouping TaskWorkspaceGroupBy, now time.Time) []TaskWorkspaceGroup {
	if grouping == WorkspaceGroupNone {
		return []TaskWorkspaceGroup{{Key: "all", Label: "All", Items: slices.Clone(items)}}
	}
	groups := make(map[string]*TaskWorkspaceGroup)
	order := make([]string, 0)
	for _, item := range items {
		key, label := taskWorkspaceGroupIdentity(item, grouping, now)
		group := groups[key]
		if group == nil {
			group = &TaskWorkspaceGroup{Key: key, Label: label}
			groups[key] = group
			order = append(order, key)
		}
		group.Items = append(group.Items, item)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return cmp.Compare(groups[order[i]].Label, groups[order[j]].Label) < 0
	})
	result := make([]TaskWorkspaceGroup, 0, len(order))
	for _, key := range order {
		result = append(result, *groups[key])
	}
	return result
}

func taskWorkspaceGroupIdentity(item TaskWorkspaceItem, grouping TaskWorkspaceGroupBy, now time.Time) (string, string) {
	switch grouping {
	case WorkspaceGroupProject:
		return "project:" + item.Identity.ProjectID, item.Project.Name
	case WorkspaceGroupStatus:
		state := item.Task.State().String()
		return "status:" + state, state
	case WorkspaceGroupSection:
		section := taskSectionBucket(item.Task)
		if section == "" {
			section = "Active"
		}
		return "section:" + section, section
	case WorkspaceGroupPriority:
		priority := strconv.Itoa(item.Task.Priority)
		return "priority:" + priority, "Priority " + priority
	case WorkspaceGroupDueDate:
		if item.Task.DueDate == nil {
			return "due:none", "No due date"
		}
		today, due := util.StartOfDay(now), util.StartOfDay(*item.Task.DueDate)
		switch {
		case due.Before(today):
			return "due:overdue", "Overdue"
		case due.Equal(today):
			return "due:today", "Today"
		case !due.After(today.AddDate(0, 0, 7)):
			return "due:soon", "Due soon"
		default:
			return "due:later", "Later"
		}
	default:
		return "all", "All"
	}
}
