package tui

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// CommonFilterState contains filter fields shared across views.
type CommonFilterState struct {
	Type        models.TaskType         // filter by type (epic/story/task/bug/idea)
	States      models.WorkflowStateSet // filter by one or more canonical workflow states
	Priority    int                     // 0=all, 1=high only (<=2)
	Tags        map[string]bool         // filter by tag names (AND logic)
	DueDate     DueDateFilter           // filter by due date
	Attrs       map[string]string       // type.attribute -> value
	ScoreMin    float64                 // minimum score threshold (0 = no filter)
	Actionable  bool                    // only show tasks ready to work on (no blockers)
	HideDimmed  bool                    // hide dimmed tasks
	GitHubIssue bool                    // only show tasks linked to a GitHub issue
	Parked      bool                    // only show parked tasks
	Blocked     bool                    // only show tasks blocked by an unresolved dependency
	Snoozed     bool                    // only show currently snoozed tasks
	Pinned      bool                    // only show pinned tasks

	// statesSet distinguishes no state predicate from an explicit empty
	// match-nothing set produced by intersecting contradictory legacy filters.
	statesSet bool
}

// copyCommonFilterState returns a deep copy of a CommonFilterState.
func copyCommonFilterState(src *CommonFilterState) *CommonFilterState {
	dst := &CommonFilterState{
		Type:        src.Type,
		States:      cloneCommonWorkflowStateSet(src.States),
		Priority:    src.Priority,
		DueDate:     src.DueDate,
		ScoreMin:    src.ScoreMin,
		Actionable:  src.Actionable,
		HideDimmed:  src.HideDimmed,
		GitHubIssue: src.GitHubIssue,
		Parked:      src.Parked,
		Blocked:     src.Blocked,
		Snoozed:     src.Snoozed,
		Pinned:      src.Pinned,
		statesSet:   src.statesSet,
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

func cloneCommonWorkflowStateSet(states models.WorkflowStateSet) models.WorkflowStateSet {
	cloned, err := models.NewWorkflowStateSet(states.Values()...)
	if err != nil {
		panic(err)
	}
	return cloned
}

func (f *CommonFilterState) hasStateFilter() bool {
	return f.statesSet || !f.States.IsEmpty()
}

func (f *CommonFilterState) setStateFilter(states models.WorkflowStateSet, active bool) {
	f.States = cloneCommonWorkflowStateSet(states)
	f.statesSet = active || !states.IsEmpty()
}

// passesCommonFilters checks if a task passes the shared filter criteria.
// Views call this from their own passesFilters method before applying
// view-specific logic. The project is needed for score filtering (its ID keys
// the scoring profile) and for the derived dependency-blocked filter (which
// resolves the task's unresolved dependencies within its project). A nil
// project disables both project-scoped filters; the state filter is
// project-independent because WorkflowState is derived from the task alone.
func passesCommonFilters(task *models.Task, f *CommonFilterState, project *models.Project, auto *config.AutomationConfig) bool {
	projectID := ""
	if project != nil {
		projectID = project.ID
	}
	if f.HideDimmed && task.Dimmed {
		return false
	}
	if f.Type != "" && task.Type != f.Type {
		return false
	}
	if f.hasStateFilter() && !f.States.Contains(task.State()) {
		return false
	}
	if f.Blocked && !task.IsDependencyBlocked(project) {
		return false
	}
	if f.Priority == 1 && task.Priority > 2 {
		return false
	}
	if f.GitHubIssue && !task.HasGitHubIssue() {
		return false
	}
	if f.Parked && !task.IsParked() {
		return false
	}
	if f.Snoozed && !task.IsSnoozed() {
		return false
	}
	if f.Pinned && !task.Pinned {
		return false
	}
	if len(f.Tags) > 0 {
		for tag := range f.Tags {
			if !task.HasTag(tag) {
				return false
			}
		}
	}
	if !passesDueDateFilter(task, f.DueDate) {
		return false
	}
	if !passesAttrFilters(task, f.Attrs) {
		return false
	}
	if f.ScoreMin > 0 && auto != nil {
		if profile := auto.GetProjectScoringProfile(projectID); profile != nil {
			if scoring.NormalizedScore(task, profile) < f.ScoreMin {
				return false
			}
		}
	}
	return true
}

// passesDueDateFilter reports whether a task satisfies the given due-date
// filter. DueDateAll (and any unknown value) matches every task.
func passesDueDateFilter(task *models.Task, filter DueDateFilter) bool {
	switch filter {
	case DueDateOverdue:
		return task.IsOverdue()
	case DueDateToday:
		return task.IsDueToday()
	case DueDateThisWeek:
		return task.IsDueThisWeek()
	case DueDateNone:
		return task.DueDate == nil
	default:
		return true
	}
}

// hasActiveCommonFilters returns true if any common filter is non-default.
func hasActiveCommonFilters(f *CommonFilterState) bool {
	return f.Type != "" ||
		f.hasStateFilter() ||
		f.Priority != 0 ||
		len(f.Tags) > 0 ||
		f.DueDate != DueDateAll ||
		len(f.Attrs) > 0 ||
		f.ScoreMin > 0 ||
		f.Actionable ||
		f.HideDimmed ||
		f.GitHubIssue ||
		f.Parked ||
		f.Blocked ||
		f.Snoozed ||
		f.Pinned
}

// clearCommonFilters resets all common filters to defaults.
func clearCommonFilters(f *CommonFilterState) {
	f.Type = ""
	f.States = models.WorkflowStateSet{}
	f.statesSet = false
	f.Priority = 0
	f.Tags = make(map[string]bool)
	f.DueDate = DueDateAll
	f.Attrs = make(map[string]string)
	f.ScoreMin = 0
	f.Actionable = false
	f.HideDimmed = false
	f.GitHubIssue = false
	f.Parked = false
	f.Blocked = false
	f.Snoozed = false
	f.Pinned = false
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
	if f.hasStateFilter() {
		states := f.States.Values()
		stateNames := make([]string, len(states))
		for i, state := range states {
			stateNames[i] = string(state)
		}
		value := "none"
		if len(stateNames) > 0 {
			value = strings.Join(stateNames, ",")
		}
		parts = append(parts, "state:"+value)
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
	if f.GitHubIssue {
		parts = append(parts, "issue")
	}
	if f.Parked {
		parts = append(parts, "parked")
	}
	if f.Snoozed {
		parts = append(parts, "snoozed")
	}
	if f.Pinned {
		parts = append(parts, "pinned")
	}
	if f.Blocked {
		parts = append(parts, "blocked")
	}

	return parts
}

// renderCommonFilterIndicator returns a compact badge of all active common filters.
// Returns empty string if no filters are active.
func renderCommonFilterIndicator(f *CommonFilterState) string {
	return renderFilterBadge(commonFilterParts(f))
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
	CommonFilterState // embedded common filter fields
}

// NewFilterState returns an initialized FilterState with allocated maps.
func NewFilterState() FilterState {
	return FilterState{
		CommonFilterState: newFilterState(),
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

// commonFilterView exposes the filter state and refresh hook used by typed
// filter result messages.
type commonFilterView interface {
	getFilter() *CommonFilterState
	refreshAndClamp()
}

// FilterMixin provides shared filter state. Views that embed this struct get
// getFilter, HasActiveFilters, ClearAllFilters, and
// renderActiveFilterIndicator for free.
// Views with custom filter logic (extra filter fields) override the methods
// they need; the embedded defaults still handle the common cases.
type FilterMixin struct {
	filter CommonFilterState
	// Active criteria filter (set when a criteria-mode SavedFilter is loaded)
	activeCriteriaFilter *config.SavedFilter
}

// newFilterState returns an initialized CommonFilterState with allocated maps.
func newFilterState() CommonFilterState {
	return CommonFilterState{
		Tags:  make(map[string]bool),
		Attrs: make(map[string]string),
	}
}

func (fm *FilterMixin) getFilter() *CommonFilterState { return &fm.filter }
func (fm *FilterMixin) HasActiveFilters() bool {
	return hasActiveCommonFilters(&fm.filter) || fm.activeCriteriaFilter != nil
}
func (fm *FilterMixin) ClearAllFilters() {
	clearCommonFilters(&fm.filter)
	fm.activeCriteriaFilter = nil
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
