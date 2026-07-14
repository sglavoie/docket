package config

// ExtractProjectReferences snapshots project-scoped references directly from
// the five owned runtime domains. It mirrors Config.ExtractProjectReferences
// without recomposing the legacy facade.
func (rs *RuntimeState) ExtractProjectReferences(id, slug string) ProjectReferences {
	matches := func(pid string) bool { return pid == id || (slug != "" && pid == slug) }
	catalog, planning, automation, machine := rs.Catalog(), rs.Planning(), rs.Automation(), rs.Machine()
	refs := ProjectReferences{
		WasCurrent:     machine.CurrentProject != "" && matches(machine.CurrentProject),
		WasActionInbox: catalog.SystemInboxProjectID != "" && matches(catalog.SystemInboxProjectID),
	}
	if v, ok := lookupProjectKey(automation.ProjectScoringProfile, id, slug); ok {
		refs.ScoringProfile = &v
	}
	if v, ok := lookupProjectKey(machine.ProjectViewStates, id, slug); ok {
		vv := v
		refs.ViewState = &vv
	}
	if v, ok := lookupProjectKey(automation.WorkflowEvalStates, id, slug); ok {
		vv := v
		refs.WorkflowEval = &vv
	}
	for _, item := range planning.FocusQueue {
		if matches(item.ProjectID) {
			refs.FocusQueue = append(refs.FocusQueue, item)
		}
	}
	for _, item := range planning.DailyPlan {
		if matches(item.ProjectID) {
			refs.DailyPlan = append(refs.DailyPlan, item)
		}
	}
	for _, visit := range machine.RecentTaskVisits {
		if matches(visit.ProjectID) {
			refs.RecentTaskVisits = append(refs.RecentTaskVisits, visit)
		}
	}
	for _, pid := range machine.CycleProjects {
		if matches(pid) {
			refs.InCycleProjects = true
			break
		}
	}
	refs.ContextCycleKeys = contextKeysContaining(machine.ContextCycleProjects, matches)
	refs.CustomContextKeys = contextKeysContaining(machine.CustomContexts, matches)
	return refs
}

// ApplyProjectReferences re-establishes a restored project's captured
// references directly on the five explicit domains. Restore adapters that
// commit the synced workspace before repairing machine-local state can call
// ApplySyncedProjectReferences first, retain refs, and call
// MachineState.ApplyProjectReferences from their main update loop.
func (d *Domains) ApplyProjectReferences(projectID string, refs ProjectReferences) {
	if d == nil || projectID == "" {
		return
	}
	d.ApplySyncedProjectReferences(projectID, refs)
	if d.Machine != nil {
		d.Machine.ApplyProjectReferences(projectID, refs)
	}
}

// ApplySyncedProjectReferences restores only references owned by the portable
// catalog, planning, and automation domains. Machine-local references are
// deliberately excluded so callers can commit them separately and retain the
// captured ProjectReferences as retry data until machine.json succeeds.
func (d *Domains) ApplySyncedProjectReferences(projectID string, refs ProjectReferences) {
	if d == nil || projectID == "" {
		return
	}
	if refs.WasActionInbox && d.Catalog != nil {
		d.Catalog.SystemInboxProjectID = projectID
	}
	if refs.ScoringProfile != nil && d.Automation != nil {
		if d.Automation.ProjectScoringProfile == nil {
			d.Automation.ProjectScoringProfile = make(map[string]string)
		}
		d.Automation.ProjectScoringProfile[projectID] = *refs.ScoringProfile
	}
	if refs.WorkflowEval != nil && d.Automation != nil {
		if d.Automation.WorkflowEvalStates == nil {
			d.Automation.WorkflowEvalStates = make(map[string]WorkflowEvalState)
		}
		d.Automation.WorkflowEvalStates[projectID] = *refs.WorkflowEval
	}

	if d.Planning != nil {
		existingFocus := make(map[string]bool, len(d.Planning.FocusQueue))
		for _, item := range d.Planning.FocusQueue {
			existingFocus[item.TaskID] = true
		}
		for _, item := range refs.FocusQueue {
			if existingFocus[item.TaskID] {
				continue
			}
			item.ProjectID = projectID
			d.Planning.FocusQueue = append(d.Planning.FocusQueue, item)
			existingFocus[item.TaskID] = true
		}

		existingPlan := make(map[string]bool, len(d.Planning.DailyPlan))
		for _, item := range d.Planning.DailyPlan {
			existingPlan[item.TaskID] = true
		}
		for _, item := range refs.DailyPlan {
			if existingPlan[item.TaskID] {
				continue
			}
			item.ProjectID = projectID
			d.Planning.DailyPlan = append(d.Planning.DailyPlan, item)
			existingPlan[item.TaskID] = true
		}
	}
}

// ApplyProjectReferences restores only the machine-local portion of a captured
// project-reference bundle. It reports whether the bundle contained any
// machine-local recovery data, allowing callers to skip an unnecessary
// machine.json commit while retaining the same payload for a retry on failure.
func (m *MachineState) ApplyProjectReferences(projectID string, refs ProjectReferences) bool {
	if m == nil || projectID == "" {
		return false
	}
	changed := refs.WasCurrent || refs.ViewState != nil || len(refs.RecentTaskVisits) > 0 ||
		refs.InCycleProjects || len(refs.ContextCycleKeys) > 0 || len(refs.CustomContextKeys) > 0
	if refs.WasCurrent {
		m.CurrentProject = projectID
	}
	if refs.ViewState != nil {
		if m.ProjectViewStates == nil {
			m.ProjectViewStates = make(map[string]ProjectViewState)
		}
		m.ProjectViewStates[projectID] = *refs.ViewState
	}

	existingVisits := make(map[string]bool, len(m.RecentTaskVisits))
	for _, visit := range m.RecentTaskVisits {
		existingVisits[visit.TaskID] = true
	}
	for _, visit := range refs.RecentTaskVisits {
		if existingVisits[visit.TaskID] {
			continue
		}
		visit.ProjectID = projectID
		m.RecentTaskVisits = append(m.RecentTaskVisits, visit)
		existingVisits[visit.TaskID] = true
	}

	if refs.InCycleProjects && !containsString(m.CycleProjects, projectID) {
		m.CycleProjects = append(m.CycleProjects, projectID)
	}
	for _, key := range refs.ContextCycleKeys {
		addProjectToContextMap(&m.ContextCycleProjects, key, projectID)
	}
	for _, key := range refs.CustomContextKeys {
		addProjectToContextMap(&m.CustomContexts, key, projectID)
	}
	return changed
}
