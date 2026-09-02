package config

import "fmt"

// PlanningState owns the two schema-6 durable ordered memberships.
type PlanningState struct {
	Today []ProjectTaskLocator `json:"today,omitempty"`
	Next  []ProjectTaskLocator `json:"next,omitempty"`
}

// NewPlanningState returns an empty planning state.
func NewPlanningState() *PlanningState {
	return &PlanningState{}
}

// Validate reports problems with the project-qualified planning collections.
func (p *PlanningState) Validate() error {
	seenToday := make(map[ProjectTaskLocator]bool, len(p.Today))
	for i, item := range p.Today {
		if !item.Valid() {
			return fmt.Errorf("planning: Today item %d has incomplete identity", i)
		}
		if seenToday[item] {
			return fmt.Errorf("planning: Today repeats task %q in project %q", item.TaskID, item.ProjectID)
		}
		seenToday[item] = true
	}
	seenNext := make(map[ProjectTaskLocator]bool, len(p.Next))
	for i, item := range p.Next {
		if !item.Valid() {
			return fmt.Errorf("planning: Next order item %d has incomplete identity", i)
		}
		if seenNext[item] {
			return fmt.Errorf("planning: Next order repeats task %q in project %q", item.TaskID, item.ProjectID)
		}
		seenNext[item] = true
	}
	return nil
}

// RemoveProjectReferences drops Today and Next-order entries that point at the
// removed project. It returns true when anything changed.
func (p *PlanningState) RemoveProjectReferences(id, slug string) bool {
	var todayChanged, nextChanged bool
	p.Today, todayChanged = filterProjectRefSlice(p.Today, func(item ProjectTaskLocator) string {
		return item.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	p.Next, nextChanged = filterProjectRefSlice(p.Next, func(item ProjectTaskLocator) string {
		return item.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	return todayChanged || nextChanged
}
