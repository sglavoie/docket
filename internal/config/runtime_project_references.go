package config

// ExtractProjectReferences snapshots project-scoped references directly from
// the three owned runtime domains.
// without recomposing the legacy facade.
func (rs *RuntimeState) ExtractProjectReferences(id, slug string) ProjectReferences {
	matches := func(pid string) bool { return pid == id || (slug != "" && pid == slug) }
	catalog, planning, machine := rs.Catalog(), rs.Planning(), rs.Machine()
	refs := ProjectReferences{
		WasActionInbox: catalog.SystemInboxProjectID != "" && matches(catalog.SystemInboxProjectID),
	}
	for _, item := range planning.NextOrder {
		if matches(item.ProjectID) {
			refs.NextOrder = append(refs.NextOrder, item)
		}
	}
	for _, item := range planning.Today {
		if matches(item.ProjectID) {
			refs.Today = append(refs.Today, item)
		}
	}
	refs.CustomContextKeys = contextKeysContaining(machine.CustomContexts, matches)
	return refs
}

// ApplyProjectReferences re-establishes a restored project's captured
// references directly on the three explicit domains. Restore adapters that
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
// catalog and planning domains. Machine-local references are
// deliberately excluded so callers can commit them separately and retain the
// captured ProjectReferences as retry data until machine.json succeeds.
func (d *Domains) ApplySyncedProjectReferences(projectID string, refs ProjectReferences) {
	if d == nil || projectID == "" {
		return
	}
	if refs.WasActionInbox && d.Catalog != nil {
		d.Catalog.SystemInboxProjectID = projectID
	}
	if d.Planning != nil {
		existingNext := make(map[ProjectTaskLocator]bool, len(d.Planning.NextOrder))
		for _, item := range d.Planning.NextOrder {
			existingNext[item] = true
		}
		for _, item := range refs.NextOrder {
			item.ProjectID = projectID
			if existingNext[item] {
				continue
			}
			d.Planning.NextOrder = append(d.Planning.NextOrder, item)
			existingNext[item] = true
		}

		existingToday := make(map[ProjectTaskLocator]bool, len(d.Planning.Today))
		for _, item := range d.Planning.Today {
			existingToday[item] = true
		}
		for _, item := range refs.Today {
			item.ProjectID = projectID
			if existingToday[item] {
				continue
			}
			d.Planning.Today = append(d.Planning.Today, item)
			existingToday[item] = true
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
	for _, key := range refs.CustomContextKeys {
		addProjectToContextMap(&m.CustomContexts, key, projectID)
	}
	return len(refs.CustomContextKeys) > 0
}
