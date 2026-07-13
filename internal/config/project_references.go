package config

import "sort"

// ProjectReferences captures the configuration references tied to a single
// project so that a delete can record them in the project's trash tombstone and
// a restore can faithfully re-establish them. It is the symmetric counterpart to
// removeProjectRefs: everything that removeProjectRefs strips on delete is
// captured here and re-applied by ApplyProjectReferences on restore.
type ProjectReferences struct {
	WasCurrent        bool               `json:"was_current,omitempty"`
	WasActionInbox    bool               `json:"was_action_inbox,omitempty"`
	ScoringProfile    *string            `json:"scoring_profile,omitempty"`
	ViewState         *ProjectViewState  `json:"view_state,omitempty"`
	WorkflowEval      *WorkflowEvalState `json:"workflow_eval,omitempty"`
	FocusQueue        []FocusQueueItem   `json:"focus_queue,omitempty"`
	DailyPlan         []DailyPlanItem    `json:"daily_plan,omitempty"`
	RecentTaskVisits  []RecentTaskVisit  `json:"recent_task_visits,omitempty"`
	InCycleProjects   bool               `json:"in_cycle_projects,omitempty"`
	ContextCycleKeys  []string           `json:"context_cycle_keys,omitempty"`
	CustomContextKeys []string           `json:"custom_context_keys,omitempty"`
}

// ExtractProjectReferences snapshots every configuration reference to the given
// project id/slug. Call it before RemoveProject, which strips exactly these
// references. Matching mirrors removeProjectRefs: a reference matches on either
// the project ID or (when non-empty) the slug.
func (c *Config) ExtractProjectReferences(id, slug string) ProjectReferences {
	matches := func(pid string) bool {
		return pid == id || (slug != "" && pid == slug)
	}

	refs := ProjectReferences{
		WasCurrent:     c.CurrentProject != "" && matches(c.CurrentProject),
		WasActionInbox: c.ActionInboxProjectID != "" && matches(c.ActionInboxProjectID),
	}

	if v, ok := lookupProjectKey(c.ProjectScoringProfile, id, slug); ok {
		refs.ScoringProfile = &v
	}
	if v, ok := lookupProjectKey(c.ProjectViewStates, id, slug); ok {
		vv := v
		refs.ViewState = &vv
	}
	if v, ok := lookupProjectKey(c.WorkflowEvalStates, id, slug); ok {
		vv := v
		refs.WorkflowEval = &vv
	}

	for _, item := range c.FocusQueue {
		if matches(item.ProjectID) {
			refs.FocusQueue = append(refs.FocusQueue, item)
		}
	}
	for _, item := range c.DailyPlan {
		if matches(item.ProjectID) {
			refs.DailyPlan = append(refs.DailyPlan, item)
		}
	}
	for _, visit := range c.RecentTaskVisits {
		if matches(visit.ProjectID) {
			refs.RecentTaskVisits = append(refs.RecentTaskVisits, visit)
		}
	}

	for _, pid := range c.CycleProjects {
		if matches(pid) {
			refs.InCycleProjects = true
			break
		}
	}
	refs.ContextCycleKeys = contextKeysContaining(c.ContextCycleProjects, matches)
	refs.CustomContextKeys = contextKeysContaining(c.CustomContexts, matches)

	return refs
}

// ApplyProjectReferences re-establishes previously extracted configuration
// references, re-keying project-scoped entries to the restored project's ID.
// Duplicate task references (same task in a collection) are skipped so a restore
// followed by a re-restore does not accumulate entries.
func (c *Config) ApplyProjectReferences(projectID string, refs ProjectReferences) {
	if projectID == "" {
		return
	}

	if refs.WasCurrent {
		c.CurrentProject = projectID
	}
	if refs.WasActionInbox {
		c.ActionInboxProjectID = projectID
	}

	if refs.ScoringProfile != nil {
		if c.ProjectScoringProfile == nil {
			c.ProjectScoringProfile = make(map[string]string)
		}
		c.ProjectScoringProfile[projectID] = *refs.ScoringProfile
	}
	if refs.ViewState != nil {
		if c.ProjectViewStates == nil {
			c.ProjectViewStates = make(map[string]ProjectViewState)
		}
		c.ProjectViewStates[projectID] = *refs.ViewState
	}
	if refs.WorkflowEval != nil {
		if c.WorkflowEvalStates == nil {
			c.WorkflowEvalStates = make(map[string]WorkflowEvalState)
		}
		c.WorkflowEvalStates[projectID] = *refs.WorkflowEval
	}

	existingFocus := make(map[string]bool, len(c.FocusQueue))
	for _, item := range c.FocusQueue {
		existingFocus[item.TaskID] = true
	}
	for _, item := range refs.FocusQueue {
		if existingFocus[item.TaskID] {
			continue
		}
		item.ProjectID = projectID
		c.FocusQueue = append(c.FocusQueue, item)
		existingFocus[item.TaskID] = true
	}

	existingPlan := make(map[string]bool, len(c.DailyPlan))
	for _, item := range c.DailyPlan {
		existingPlan[item.TaskID] = true
	}
	for _, item := range refs.DailyPlan {
		if existingPlan[item.TaskID] {
			continue
		}
		item.ProjectID = projectID
		c.DailyPlan = append(c.DailyPlan, item)
		existingPlan[item.TaskID] = true
	}

	existingVisits := make(map[string]bool, len(c.RecentTaskVisits))
	for _, visit := range c.RecentTaskVisits {
		existingVisits[visit.TaskID] = true
	}
	for _, visit := range refs.RecentTaskVisits {
		if existingVisits[visit.TaskID] {
			continue
		}
		visit.ProjectID = projectID
		c.RecentTaskVisits = append(c.RecentTaskVisits, visit)
		existingVisits[visit.TaskID] = true
	}

	if refs.InCycleProjects && !containsString(c.CycleProjects, projectID) {
		c.CycleProjects = append(c.CycleProjects, projectID)
	}
	for _, key := range refs.ContextCycleKeys {
		addProjectToContextMap(&c.ContextCycleProjects, key, projectID)
	}
	for _, key := range refs.CustomContextKeys {
		addProjectToContextMap(&c.CustomContexts, key, projectID)
	}
}

func lookupProjectKey[T any](values map[string]T, id, slug string) (T, bool) {
	if id != "" {
		if v, ok := values[id]; ok {
			return v, true
		}
	}
	if slug != "" {
		if v, ok := values[slug]; ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func contextKeysContaining(m map[string][]string, matches func(string) bool) []string {
	var keys []string
	for key, ids := range m {
		for _, pid := range ids {
			if matches(pid) {
				keys = append(keys, key)
				break
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func addProjectToContextMap(m *map[string][]string, key, projectID string) {
	if *m == nil {
		*m = make(map[string][]string)
	}
	if containsString((*m)[key], projectID) {
		return
	}
	(*m)[key] = append((*m)[key], projectID)
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
