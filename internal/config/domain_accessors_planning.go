package config

// RemoveDurableTaskLocators removes matching Today and Next-order references.
func (p *PlanningState) RemoveDurableTaskLocators(locators []ProjectTaskLocator) int {
	if p == nil || len(locators) == 0 {
		return 0
	}
	remove := make(map[ProjectTaskLocator]bool, len(locators))
	for _, locator := range locators {
		if locator.Valid() {
			remove[locator] = true
		}
	}
	removed := 0
	filter := func(items []ProjectTaskLocator) []ProjectTaskLocator {
		kept := items[:0]
		for _, item := range items {
			if remove[item] {
				removed++
				continue
			}
			kept = append(kept, item)
		}
		return kept
	}
	p.Today = filter(p.Today)
	p.Next = filter(p.Next)
	return removed
}

// ReassignDurablePlanningProjectFrom moves matching task references from one
// project identity to another while preserving collection order.
func (p *PlanningState) ReassignDurablePlanningProjectFrom(taskIDs map[string]bool, fromProjectID, toProjectID string) int {
	if p == nil || fromProjectID == "" || toProjectID == "" || fromProjectID == toProjectID {
		return 0
	}
	changed := 0
	update := func(items []ProjectTaskLocator) {
		for i := range items {
			if items[i].ProjectID == fromProjectID && taskIDs[items[i].TaskID] {
				items[i].ProjectID = toProjectID
				changed++
			}
		}
	}
	update(p.Today)
	update(p.Next)
	return changed
}

// GetToday returns the durable project-qualified Today membership in its
// workspace-wide order.
func (p *PlanningState) GetToday() []ProjectTaskLocator {
	if p == nil {
		return nil
	}
	return p.Today
}

// SetToday replaces Today membership/order and keeps the first occurrence of
// each valid project-qualified identity.
func (p *PlanningState) SetToday(items []ProjectTaskLocator) {
	if p == nil {
		return
	}
	p.Today = canonicalPlanningRefs(items)
}

// AddToToday appends one project-qualified task to Today if it is not already a
// member.
func (p *PlanningState) AddToToday(taskID, projectID string) bool {
	if p == nil {
		return false
	}
	locator := ProjectTaskLocator{TaskID: taskID, ProjectID: projectID}
	if !locator.Valid() || containsPlanningRef(p.Today, locator) {
		return false
	}
	p.Today = append(p.Today, locator)
	return true
}

// RemoveFromToday removes exactly one project-qualified Today membership.
func (p *PlanningState) RemoveFromTodayInProject(taskID, projectID string) bool {
	if p == nil {
		return false
	}
	return removePlanningRef(&p.Today, ProjectTaskLocator{TaskID: taskID, ProjectID: projectID})
}

// GetNext returns the durable project-qualified Next membership in order.
func (p *PlanningState) GetNext() []ProjectTaskLocator {
	if p == nil {
		return nil
	}
	return p.Next
}

// SetNext replaces Next membership/order and keeps the first
// occurrence of each valid project-qualified identity.
func (p *PlanningState) SetNext(items []ProjectTaskLocator) {
	if p == nil {
		return
	}
	p.Next = canonicalPlanningRefs(items)
}

// AddToNext appends one project-qualified Next membership if absent.
func (p *PlanningState) AddToNext(taskID, projectID string) bool {
	if p == nil {
		return false
	}
	locator := ProjectTaskLocator{TaskID: taskID, ProjectID: projectID}
	if !locator.Valid() || containsPlanningRef(p.Next, locator) {
		return false
	}
	p.Next = append(p.Next, locator)
	return true
}

// RemoveFromNextInProject removes exactly one project-qualified Next membership.
func (p *PlanningState) RemoveFromNextInProject(taskID, projectID string) bool {
	if p == nil {
		return false
	}
	return removePlanningRef(&p.Next, ProjectTaskLocator{TaskID: taskID, ProjectID: projectID})
}

// NormalizePlanningReferences removes incomplete and duplicate identities while
// preserving the first-seen order of each collection. Workspace existence and
// workflow-state eligibility remain application-owned reconciliation concerns.
func (p *PlanningState) NormalizePlanningReferences() bool {
	if p == nil {
		return false
	}
	today := canonicalPlanningRefs(p.Today)
	next := canonicalPlanningRefs(p.Next)
	changed := len(today) != len(p.Today) || len(next) != len(p.Next)
	p.Today = today
	p.Next = next
	return changed
}

func canonicalPlanningRefs(items []ProjectTaskLocator) []ProjectTaskLocator {
	if items == nil {
		return nil
	}
	out := make([]ProjectTaskLocator, 0, len(items))
	seen := make(map[ProjectTaskLocator]bool, len(items))
	for _, item := range items {
		if !item.Valid() || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func containsPlanningRef(items []ProjectTaskLocator, locator ProjectTaskLocator) bool {
	for _, item := range items {
		if item == locator {
			return true
		}
	}
	return false
}

func removePlanningRef(items *[]ProjectTaskLocator, locator ProjectTaskLocator) bool {
	if items == nil || !locator.Valid() {
		return false
	}
	filtered := (*items)[:0]
	removed := false
	for _, item := range *items {
		if item == locator {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	*items = filtered
	return removed
}
