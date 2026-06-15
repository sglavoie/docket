package tui

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// CommonFilterState contains the filter fields shared across views that support
// the F-prefix filter system. Views embed this struct directly.
type CommonFilterState struct {
	Type       models.TaskType   // filter by type (epic/story/task/bug/idea)
	Status     models.TaskStatus // filter by status
	Priority   int               // 0=all, 1=high only (<=2)
	Tags       map[string]bool   // filter by tag names (AND logic)
	DueDate    DueDateFilter     // filter by due date
	Attrs      map[string]string // type.attribute -> value
	ScoreMin   float64           // minimum score threshold (0 = no filter)
	Actionable bool              // only show tasks ready to work on (no blockers)
	HideDimmed bool              // hide dimmed tasks
	Stage      string            // filter by workflow stage
}

// copyCommonFilterState returns a deep copy of a CommonFilterState.
func copyCommonFilterState(src *CommonFilterState) *CommonFilterState {
	dst := &CommonFilterState{
		Type:       src.Type,
		Status:     src.Status,
		Priority:   src.Priority,
		DueDate:    src.DueDate,
		ScoreMin:   src.ScoreMin,
		Actionable: src.Actionable,
		HideDimmed: src.HideDimmed,
		Stage:      src.Stage,
	}
	if len(src.Tags) > 0 {
		dst.Tags = make(map[string]bool, len(src.Tags))
		maps.Copy(dst.Tags, src.Tags)
	}
	if len(src.Attrs) > 0 {
		dst.Attrs = make(map[string]string, len(src.Attrs))
		maps.Copy(dst.Attrs, src.Attrs)
	}
	return dst
}

// passesCommonFilters checks if a task passes the shared filter criteria.
// Views call this from their own passesFilters method before applying
// view-specific logic. The projectID and cfg are needed for score filtering.
func passesCommonFilters(task *models.Task, f *CommonFilterState, projectID string, cfg *config.Config) bool {
	if f.HideDimmed && task.Dimmed {
		return false
	}
	if f.Type != "" && task.Type != f.Type {
		return false
	}
	if f.Status != "" && task.Status != f.Status {
		return false
	}
	if f.Priority == 1 && task.Priority > 2 {
		return false
	}
	if len(f.Tags) > 0 {
		for tag := range f.Tags {
			if !task.HasTag(tag) {
				return false
			}
		}
	}
	if f.DueDate != DueDateAll {
		switch f.DueDate {
		case DueDateOverdue:
			if !task.IsOverdue() {
				return false
			}
		case DueDateToday:
			if !task.IsDueToday() {
				return false
			}
		case DueDateThisWeek:
			if !task.IsDueThisWeek() {
				return false
			}
		case DueDateNone:
			if task.DueDate != nil {
				return false
			}
		default:
		}
	}
	if f.Stage != "" && task.Stage != f.Stage {
		return false
	}
	if !passesAttrFilters(task, f.Attrs) {
		return false
	}
	if f.ScoreMin > 0 && cfg != nil {
		if profile := cfg.GetProjectScoringProfile(projectID); profile != nil {
			if scoring.NormalizedScore(task, profile) < f.ScoreMin {
				return false
			}
		}
	}
	return true
}

// hasActiveCommonFilters returns true if any common filter is non-default.
func hasActiveCommonFilters(f *CommonFilterState) bool {
	return f.Type != "" ||
		f.Status != "" ||
		f.Priority != 0 ||
		len(f.Tags) > 0 ||
		f.DueDate != DueDateAll ||
		len(f.Attrs) > 0 ||
		f.ScoreMin > 0 ||
		f.Actionable ||
		f.HideDimmed ||
		f.Stage != ""
}

// clearCommonFilters resets all common filters to defaults.
func clearCommonFilters(f *CommonFilterState) {
	f.Type = ""
	f.Status = ""
	f.Priority = 0
	f.Tags = make(map[string]bool)
	f.DueDate = DueDateAll
	f.Attrs = make(map[string]string)
	f.ScoreMin = 0
	f.Actionable = false
	f.HideDimmed = false
	f.Stage = ""
}

// commonFilterParts returns the list of active filter description strings.
func commonFilterParts(f *CommonFilterState) []string {
	var parts []string

	if f.Priority == 1 {
		parts = append(parts, "!Hi")
	}
	if f.Type != "" {
		parts = append(parts, "type:"+string(f.Type))
	}
	if f.Status != "" {
		parts = append(parts, "status:"+string(f.Status))
	}
	if len(f.Tags) > 0 {
		tagNames := make([]string, 0, len(f.Tags))
		for tag := range f.Tags {
			tagNames = append(tagNames, tag)
		}
		sort.Strings(tagNames)
		for i, tag := range tagNames {
			tagNames[i] = "#" + tag
		}
		parts = append(parts, strings.Join(tagNames, " "))
	}
	switch f.DueDate {
	case DueDateOverdue:
		parts = append(parts, "due:overdue")
	case DueDateToday:
		parts = append(parts, "due:today")
	case DueDateThisWeek:
		parts = append(parts, "due:week")
	case DueDateNone:
		parts = append(parts, "due:none")
	default:
	}
	if len(f.Attrs) > 0 {
		keys := make([]string, 0, len(f.Attrs))
		for k := range f.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		attrParts := make([]string, 0, len(keys))
		for _, k := range keys {
			attrParts = append(attrParts, k+"="+f.Attrs[k])
		}
		parts = append(parts, "attr:"+strings.Join(attrParts, ","))
	}
	if f.ScoreMin > 0 {
		parts = append(parts, fmt.Sprintf("score>=%.0f", f.ScoreMin))
	}
	if f.Actionable {
		parts = append(parts, "ready")
	}
	if f.HideDimmed {
		parts = append(parts, "no-dim")
	}
	if f.Stage != "" {
		parts = append(parts, "stage:"+f.Stage)
	}

	return parts
}

// renderCommonFilterIndicator returns a compact badge of all active common filters.
// Returns empty string if no filters are active.
func renderCommonFilterIndicator(f *CommonFilterState) string {
	return renderFilterBadge(commonFilterParts(f))
}

func buildTagCmdFromProject(project *models.Project, selected map[string]bool, source ViewType, title string, excludeCompleted bool) tea.Cmd {
	tags := BuildProjectTagList(project, excludeCompleted)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: selected, Source: source, Title: title}
	}
}

func buildTagCmdFromProjectTasks(projects []*models.Project, tasks []*models.Task, selected map[string]bool, source ViewType, title string, excludeCompleted bool) tea.Cmd {
	tags := BuildCatalogTagList(projects, tasks, excludeCompleted)
	return func() tea.Msg {
		return ShowTagFilterMsg{Tags: tags, Selected: selected, Source: source, Title: title}
	}
}

// CollectTaskTagNames returns sorted unique tag names from task usage only.
func CollectTaskTagNames(tasks []*models.Task) []string {
	return tagNamesFromInfos(BuildTagList(tasks, false))
}

// CollectTagNamesForProject returns sorted unique tag names from the project's
// catalog and task usage.
func CollectTagNamesForProject(project *models.Project) []string {
	return tagNamesFromInfos(BuildProjectTagList(project, false))
}

// CollectTagNamesForProjects returns sorted unique tag names from all project
// catalogs and task usages.
func CollectTagNamesForProjects(projects []*models.Project) []string {
	return tagNamesFromInfos(BuildProjectsTagList(projects, false))
}

func tagNamesFromInfos(infos []TagInfo) []string {
	tags := make([]string, len(infos))
	for i, info := range infos {
		tags[i] = info.Name
	}
	return tags
}

func projectsFromCrossProjectTasks(items []CrossProjectTask) []*models.Project {
	projects := make([]*models.Project, 0, len(items))
	for _, item := range items {
		projects = append(projects, item.Project)
	}
	return uniqueProjects(projects)
}

func tasksFromCrossProjectTasks(items []CrossProjectTask) []*models.Task {
	tasks := make([]*models.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, item.Task)
	}
	return tasks
}

func projectsFromFocusTasks(items []FocusTask) []*models.Project {
	projects := make([]*models.Project, 0, len(items))
	for _, item := range items {
		projects = append(projects, item.Project)
	}
	return uniqueProjects(projects)
}

func tasksFromFocusTasks(items []FocusTask) []*models.Task {
	tasks := make([]*models.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, item.Task)
	}
	return tasks
}

func projectsFromBoardRows(rows []boardRow) []*models.Project {
	projects := make([]*models.Project, 0, len(rows))
	for _, row := range rows {
		projects = append(projects, row.Project)
	}
	return uniqueProjects(projects)
}

func tasksFromBoardRows(rows []boardRow) []*models.Task {
	tasks := make([]*models.Task, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, row.Task)
	}
	return tasks
}

func projectsFromWeeklyReviewTasks(items []WeeklyReviewTask) []*models.Project {
	projects := make([]*models.Project, 0, len(items))
	for _, item := range items {
		projects = append(projects, item.Project)
	}
	return uniqueProjects(projects)
}

func tasksFromWeeklyReviewTasks(items []WeeklyReviewTask) []*models.Task {
	tasks := make([]*models.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, item.Task)
	}
	return tasks
}

func projectsFromProjectCache(projectCache map[string]*models.Project) []*models.Project {
	projects := make([]*models.Project, 0, len(projectCache))
	for _, project := range projectCache {
		projects = append(projects, project)
	}
	return uniqueProjects(projects)
}

func tasksFromCrossProjectCards(cards []KanbanCard) []*models.Task {
	tasks := make([]*models.Task, 0, len(cards))
	for _, card := range cards {
		tasks = append(tasks, card.Task)
	}
	return tasks
}

// validateSavedStringSet returns the subset of saved that exist in valid.
func validateSavedStringSet(saved []string, valid []string) map[string]bool {
	validSet := make(map[string]bool, len(valid))
	for _, v := range valid {
		validSet[v] = true
	}
	result := make(map[string]bool)
	for _, s := range saved {
		if validSet[s] {
			result[s] = true
		}
	}
	return result
}

// FilterState contains all task filtering options including view-specific fields
// beyond the common set. Used by ProjectTree and Outliner.
type FilterState struct {
	CommonFilterState                 // embedded common filter fields
	Sections          map[string]bool // filter by section names (OR logic)
}

// NewFilterState returns an initialized FilterState with allocated maps.
func NewFilterState() FilterState {
	return FilterState{
		CommonFilterState: newFilterState(),
		Sections:          make(map[string]bool),
	}
}

// renderFilterBadge renders a styled filter badge from a parts list.
// Returns empty string if parts is empty.
func renderFilterBadge(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return FilterBadgeStyle.Render("Filters: " + strings.Join(parts, " "))
}

// FilterMixin provides standard commonFilterView bridge methods. Views that
// embed this struct get getFilter, getConfig, setFilterPendingPrefix,
// HasActiveFilters, ClearAllFilters, and renderActiveFilterIndicator for free.
// Views with custom filter logic (extra filter fields) override the methods
// they need; the embedded defaults still handle the common cases.
type FilterMixin struct {
	filter    CommonFilterState
	filterCfg *config.Config

	// dispatcher is the keymap dispatcher; when non-nil it owns prefix state.
	dispatcher *keymap.Dispatcher

	// Active criteria filter (set when a criteria-mode SavedFilter is loaded)
	activeCriteriaFilter *config.SavedFilter
}

// initFilterMixinWithDispatcher wires the mixin to a declarative dispatcher and config.
func (fm *FilterMixin) initFilterMixinWithDispatcher(d *keymap.Dispatcher, cfg *config.Config) {
	fm.dispatcher = d
	fm.filterCfg = cfg
}

// newFilterState returns an initialized CommonFilterState with allocated maps.
func newFilterState() CommonFilterState {
	return CommonFilterState{
		Tags:  make(map[string]bool),
		Attrs: make(map[string]string),
	}
}

func (fm *FilterMixin) getFilter() *CommonFilterState { return &fm.filter }
func (fm *FilterMixin) getConfig() *config.Config     { return fm.filterCfg }
func (fm *FilterMixin) setFilterPendingPrefix(p string) {
	fm.dispatcher.SetPendingPrefix(p)
}
func (fm *FilterMixin) HasActiveFilters() bool {
	return hasActiveCommonFilters(&fm.filter) || fm.activeCriteriaFilter != nil
}
func (fm *FilterMixin) ClearAllFilters() {
	clearCommonFilters(&fm.filter)
	fm.activeCriteriaFilter = nil
}
func (fm *FilterMixin) renderActiveFilterIndicator() string {
	return renderCommonFilterIndicator(&fm.filter)
}

// ActiveFilterCount returns the number of active filters.
// Implements ActiveFilterProvider. Views with extra filter fields override this.
func (fm *FilterMixin) ActiveFilterCount() int {
	count := len(commonFilterParts(&fm.filter))
	if fm.activeCriteriaFilter != nil {
		count++
	}
	return count
}
