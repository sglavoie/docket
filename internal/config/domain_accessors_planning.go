package config

import (
	"strings"
	"time"
)

// This file gives the PlanningState domain the same defaulting/mutation
// accessors the config.Config facade exposes for the planning-owned bookkeeping
// the alert engine and review flows consume: alert dismissals and daily/weekly
// review completion timestamps. The bodies mirror their (*Config) counterparts
// field-for-field, keeping the observable behavior identical, but live on the
// owning domain so the TUI alert/review consumers read and commit this state
// through RuntimeState.Planning without dereferencing config.Config. Per the
// domain ownership map, review completion and dismissals are Planning state and
// must never be folded into the Settings domain.

// --- PlanningState: alert dismissals ---------------------------------------

// GetAlertDismissals mirrors (*Config).GetAlertDismissals.
func (p *PlanningState) GetAlertDismissals() []AlertDismissal {
	return p.AlertDismissals
}

// SetAlertDismissals mirrors (*Config).SetAlertDismissals.
func (p *PlanningState) SetAlertDismissals(d []AlertDismissal) {
	p.AlertDismissals = d
}

// PruneExpiredAlertDismissals mirrors (*Config).PruneExpiredAlertDismissals.
func (p *PlanningState) PruneExpiredAlertDismissals() bool {
	if len(p.AlertDismissals) == 0 {
		return false
	}
	cutoff := time.Now().Add(-alertDismissalMaxAge)
	kept := make([]AlertDismissal, 0, len(p.AlertDismissals))
	for _, d := range p.AlertDismissals {
		if d.DismissedAt.After(cutoff) {
			kept = append(kept, d)
		}
	}
	if len(kept) == len(p.AlertDismissals) {
		return false
	}
	p.AlertDismissals = kept
	return true
}

// DismissAlert mirrors (*Config).DismissAlert.
func (p *PlanningState) DismissAlert(key, category string) {
	cutoff := time.Now().Add(-alertDismissalMaxAge)
	for i, d := range p.AlertDismissals {
		if d.Key == key {
			if d.DismissedAt.After(cutoff) {
				return
			}
			p.AlertDismissals[i].Category = category
			p.AlertDismissals[i].DismissedAt = time.Now()
			return
		}
	}
	p.AlertDismissals = append(p.AlertDismissals, AlertDismissal{
		Key:         key,
		Category:    category,
		DismissedAt: time.Now(),
	})
}

// IsAlertDismissed mirrors (*Config).IsAlertDismissed.
func (p *PlanningState) IsAlertDismissed(key string) bool {
	cutoff := time.Now().Add(-alertDismissalMaxAge)
	for _, d := range p.AlertDismissals {
		if d.Key == key && d.DismissedAt.After(cutoff) {
			return true
		}
	}
	return false
}

// UndismissAlert mirrors (*Config).UndismissAlert.
func (p *PlanningState) UndismissAlert(key string) bool {
	for i, d := range p.AlertDismissals {
		if d.Key == key {
			p.AlertDismissals = append(p.AlertDismissals[:i], p.AlertDismissals[i+1:]...)
			return true
		}
	}
	return false
}

// ClearAlertDismissals mirrors (*Config).ClearAlertDismissals.
func (p *PlanningState) ClearAlertDismissals() {
	p.AlertDismissals = nil
}

// --- PlanningState: review completion --------------------------------------

// GetLastDailyReviewAt mirrors (*Config).GetLastDailyReviewAt.
func (p *PlanningState) GetLastDailyReviewAt() *time.Time {
	return p.LastDailyReviewAt
}

// SetLastDailyReviewAt mirrors (*Config).SetLastDailyReviewAt.
func (p *PlanningState) SetLastDailyReviewAt(t time.Time) {
	p.LastDailyReviewAt = &t
}

// ClearLastDailyReviewAt mirrors (*Config).ClearLastDailyReviewAt.
func (p *PlanningState) ClearLastDailyReviewAt() {
	p.LastDailyReviewAt = nil
}

// GetLastWeeklyReviewAt mirrors (*Config).GetLastWeeklyReviewAt.
func (p *PlanningState) GetLastWeeklyReviewAt() *time.Time {
	return p.LastWeeklyReviewAt
}

// SetLastWeeklyReviewAt mirrors (*Config).SetLastWeeklyReviewAt.
func (p *PlanningState) SetLastWeeklyReviewAt(t time.Time) {
	p.LastWeeklyReviewAt = &t
}

// ClearLastWeeklyReviewAt mirrors (*Config).ClearLastWeeklyReviewAt.
func (p *PlanningState) ClearLastWeeklyReviewAt() {
	p.LastWeeklyReviewAt = nil
}

// --- PlanningState: focus queue --------------------------------------------

// GetFocusQueue mirrors (*Config).GetFocusQueue.
func (p *PlanningState) GetFocusQueue() []FocusQueueItem {
	return p.FocusQueue
}

// SetFocusQueue mirrors (*Config).SetFocusQueue.
func (p *PlanningState) SetFocusQueue(items []FocusQueueItem) {
	p.FocusQueue = items
}

// AddToFocusQueue appends a task to the Next order index (idempotent by task ID).
// Membership in Next is the task's StateNext workflow state; this only records
// ordering, so callers set StateNext through the task-mutation boundary
// separately. Mirrors (*Config).AddToFocusQueue.
func (p *PlanningState) AddToFocusQueue(taskID, projectID string) bool {
	if taskID == "" {
		return false
	}
	for _, item := range p.FocusQueue {
		if item.TaskID == taskID {
			return false
		}
	}
	p.FocusQueue = append(p.FocusQueue, FocusQueueItem{TaskID: taskID, ProjectID: projectID})
	return true
}

// RemoveFromFocusQueue drops a task's entry from the Next order index. It is
// ordering-only maintenance; leaving StateNext (which clears Next membership) is
// done through the task-mutation boundary separately. Mirrors
// (*Config).RemoveFromFocusQueue.
func (p *PlanningState) RemoveFromFocusQueue(taskID string) bool {
	if taskID == "" || len(p.FocusQueue) == 0 {
		return false
	}
	filtered := p.FocusQueue[:0]
	removed := false
	for _, item := range p.FocusQueue {
		if item.TaskID == taskID {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	p.FocusQueue = filtered
	return removed
}

// --- PlanningState: daily plan ---------------------------------------------

// GetDailyPlan mirrors (*Config).GetDailyPlan.
func (p *PlanningState) GetDailyPlan() []DailyPlanItem {
	return p.DailyPlan
}

// SetDailyPlan mirrors (*Config).SetDailyPlan.
func (p *PlanningState) SetDailyPlan(items []DailyPlanItem) {
	p.DailyPlan = items
}

// RemoveFromDailyPlan drops a task's Today (Daily Plan) entry, returning true
// when the task was planned. Today and Next are mutually exclusive horizons, so
// the queue and today mutations call this to enforce exclusivity when a task
// moves between them.
func (p *PlanningState) RemoveFromDailyPlan(taskID string) bool {
	if taskID == "" || len(p.DailyPlan) == 0 {
		return false
	}
	filtered := p.DailyPlan[:0]
	removed := false
	for _, item := range p.DailyPlan {
		if item.TaskID == taskID {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	p.DailyPlan = filtered
	return removed
}

// --- PlanningState: dismissed suggestions ----------------------------------

// GetFocusDismissedIDs mirrors (*Config).GetFocusDismissedIDs.
func (p *PlanningState) GetFocusDismissedIDs() []string {
	return p.FocusDismissedIDs
}

// SetFocusDismissedIDs mirrors (*Config).SetFocusDismissedIDs.
func (p *PlanningState) SetFocusDismissedIDs(ids []string) {
	p.FocusDismissedIDs = ids
}

// PruneFocusDismissedIDs mirrors (*Config).PruneFocusDismissedIDs.
func (p *PlanningState) PruneFocusDismissedIDs() bool {
	if len(p.FocusDismissedIDs) <= maxFocusDismissedIDs {
		return false
	}
	p.FocusDismissedIDs = p.FocusDismissedIDs[:maxFocusDismissedIDs]
	return true
}

// --- PlanningState: today clearing and streaks -----------------------------

// GetTodayClearedAt mirrors reading (*Config).TodayClearedAt.
func (p *PlanningState) GetTodayClearedAt() string {
	return p.TodayClearedAt
}

// SetTodayClearedAt mirrors writing (*Config).TodayClearedAt.
func (p *PlanningState) SetTodayClearedAt(day string) {
	p.TodayClearedAt = day
}

// GetStreakCount mirrors reading (*Config).StreakCount.
func (p *PlanningState) GetStreakCount() int {
	return p.StreakCount
}

// SetStreakCount mirrors writing (*Config).StreakCount.
func (p *PlanningState) SetStreakCount(n int) {
	p.StreakCount = n
}

// GetStreakLastDate mirrors reading (*Config).StreakLastDate.
func (p *PlanningState) GetStreakLastDate() string {
	return p.StreakLastDate
}

// SetStreakLastDate mirrors writing (*Config).StreakLastDate.
func (p *PlanningState) SetStreakLastDate(day string) {
	p.StreakLastDate = day
}

// --- PlanningState: saved filters (views + smart lists) -------------------
//
// Saved filters are the unified planning-owned presets that subsume the legacy
// saved views (snapshot mode) and smart lists (criteria mode). The bodies below
// mirror the (*Config) counterparts in saved_filter.go field-for-field so the TUI
// saved-filter consumers read and commit this state through RuntimeState.Planning
// without dereferencing config.Config. AllTasksTabOrder cleanup is intentionally
// NOT performed here: the tab order is machine-local layout state, not planning
// state, so it never lives on PlanningState.

// GetSavedFilters mirrors (*Config).GetSavedFilters.
func (p *PlanningState) GetSavedFilters() []SavedFilter {
	if p.SavedFilters == nil {
		return []SavedFilter{}
	}
	return p.SavedFilters
}

// GetSavedFilter mirrors (*Config).GetSavedFilter.
func (p *PlanningState) GetSavedFilter(id string) *SavedFilter {
	for i := range p.SavedFilters {
		if p.SavedFilters[i].ID == id {
			return &p.SavedFilters[i]
		}
	}
	return nil
}

// GetSavedFilterByName mirrors (*Config).GetSavedFilterByName.
func (p *PlanningState) GetSavedFilterByName(name string) *SavedFilter {
	nameLower := strings.ToLower(name)
	for i := range p.SavedFilters {
		if strings.ToLower(p.SavedFilters[i].Name) == nameLower {
			return &p.SavedFilters[i]
		}
	}
	return nil
}

// SaveFilter mirrors (*Config).SaveFilter: upsert by ID.
func (p *PlanningState) SaveFilter(f SavedFilter) {
	for i := range p.SavedFilters {
		if p.SavedFilters[i].ID == f.ID {
			p.SavedFilters[i] = f
			return
		}
	}
	p.SavedFilters = append(p.SavedFilters, f)
}

// DeleteSavedFilter removes a saved filter by ID from the planning domain,
// returning true when the filter existed. Unlike (*Config).DeleteSavedFilter it
// does not touch AllTasksTabOrder, which is machine-local layout state committed
// through the machine domain; buildTabEntries already skips tab-order keys with
// no matching filter, so a stale entry is harmless until the machine-local
// layout migration reconciles it.
func (p *PlanningState) DeleteSavedFilter(id string) bool {
	for i := range p.SavedFilters {
		if p.SavedFilters[i].ID == id {
			p.SavedFilters = append(p.SavedFilters[:i], p.SavedFilters[i+1:]...)
			return true
		}
	}
	return false
}

// GetTabFilters mirrors (*Config).GetTabFilters.
func (p *PlanningState) GetTabFilters() []SavedFilter {
	var tabs []SavedFilter
	for _, f := range p.SavedFilters {
		if f.ShowAsTab {
			tabs = append(tabs, f)
		}
	}
	return tabs
}
