package config

// ExtractProjectReferences snapshots project-scoped references directly from
// the three owned runtime domains.
// without recomposing the legacy facade.
func (rs *RuntimeState) ExtractProjectReferences(id, slug string) ProjectReferences {
	matches := func(pid string) bool { return pid == id || (slug != "" && pid == slug) }
	catalog, planning := rs.Catalog(), rs.Planning()
	refs := ProjectReferences{
		WasActionInbox: catalog.protectedProjectID() != "" && matches(catalog.protectedProjectID()),
	}
	for _, item := range planning.GetNext() {
		if matches(item.ProjectID) {
			refs.NextOrder = append(refs.NextOrder, item)
		}
	}
	for _, item := range planning.Today {
		if matches(item.ProjectID) {
			refs.Today = append(refs.Today, item)
		}
	}
	return refs
}

// ApplyProjectReferences re-establishes a restored project's captured portable
// references directly on the catalog and planning domains.
func (d *Domains) ApplyProjectReferences(projectID string, refs ProjectReferences) {
	if d == nil || projectID == "" {
		return
	}
	d.ApplySyncedProjectReferences(projectID, refs)
}

// ApplySyncedProjectReferences restores references owned by the portable
// catalog and planning domains.
func (d *Domains) ApplySyncedProjectReferences(projectID string, refs ProjectReferences) {
	if d == nil || projectID == "" {
		return
	}
	if refs.WasActionInbox && d.Catalog != nil {
		d.Catalog.SystemUnfiledProjectID = projectID
		d.Catalog.SystemInboxProjectID = projectID
	}
	if d.Planning != nil {
		existingNext := make(map[ProjectTaskLocator]bool, len(d.Planning.GetNext()))
		for _, item := range d.Planning.GetNext() {
			existingNext[item] = true
		}
		for _, item := range refs.NextOrder {
			item.ProjectID = projectID
			if existingNext[item] {
				continue
			}
			d.Planning.AddToNext(item.TaskID, item.ProjectID)
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
