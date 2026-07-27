package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// dailyReviewSectionOrder is the fixed presentation order of the Daily Review
// sections, matching the standalone model's non-empty section ordering.
var dailyReviewSectionOrder = []DailyReviewSection{
	SectionOverdue, SectionDueToday, SectionDueSoon, SectionDueLater, SectionFuture,
	SectionInProgress, SectionBlocked, SectionInReview, SectionOnStandby, SectionOld,
	SectionCompleted,
}

// dailyReviewSectionComparator returns the sort comparator each Daily Review
// section uses, ported verbatim from the standalone model's sortSections.
func dailyReviewSectionComparator(section DailyReviewSection) crossTaskLess {
	switch section {
	case SectionOverdue:
		return byOverdueDueDateThenID
	case SectionDueToday, SectionBlocked:
		return byPriorityThenID
	case SectionDueSoon, SectionDueLater, SectionFuture:
		return byDueDateThenPriority
	case SectionInProgress, SectionInReview, SectionOnStandby:
		return byPriorityThenUpdatedAt
	case SectionOld:
		return byEffectiveUpdatedAtThenID
	case SectionCompleted:
		return byCompletedAtThenID
	}
	return byPriorityThenID
}

// classifyDailyReviewTask assigns a task to its Daily Review section, mirroring
// DailyReviewModel.appendDailyReviewTask's ordered classification. The second
// return reports whether the task belongs in any section at all (terminal,
// snoozed, or otherwise unclassified tasks are dropped).
func classifyDailyReviewTask(task *models.Task, project *models.Project, th dailyReviewThresholds) (DailyReviewSection, bool) {
	if task == nil {
		return 0, false
	}
	// Completed within the recent window, else dropped (terminal tasks never
	// fall through to the live sections).
	if task.IsTerminalState() && task.CompletedAt != nil {
		if th.completedHours > 0 && task.CompletedAt.After(th.completedThreshold) {
			return SectionCompleted, true
		}
		return 0, false
	}
	if task.IsTerminalState() || task.IsSnoozed() {
		return 0, false
	}
	// Standby / Review states.
	if task.IsInState(models.StatePaused) {
		return SectionOnStandby, true
	}
	if task.IsInState(models.StateReview) {
		return SectionInReview, true
	}
	// Dated buckets.
	if task.DueDate != nil {
		dueDay := util.StartOfDay(*task.DueDate)
		switch {
		case dueDay.Before(th.today):
			return SectionOverdue, true
		case dueDay.Equal(th.today):
			return SectionDueToday, true
		case th.dueSoonDays > 0 && dueDay.Before(th.dueSoonEnd):
			return SectionDueSoon, true
		case th.futureDays > 0 && !dueDay.Before(th.futureStart):
			return SectionFuture, true
		default:
			return SectionDueLater, true
		}
	}
	// Blocked / in-progress.
	if task.IsInState(models.StateWaiting) || project.HasIncompleteDependencies(task) {
		return SectionBlocked, true
	}
	if task.IsInState(models.StateInProgress) {
		return SectionInProgress, true
	}
	// Stale (old) tasks.
	effectiveUpdate := task.UpdatedAt
	if effectiveUpdate.IsZero() {
		effectiveUpdate = task.CreatedAt
	}
	if th.oldDays > 0 && effectiveUpdate.Before(th.oldThreshold) {
		return SectionOld, true
	}
	return 0, false
}

// buildDailyReviewRows classifies every in-scope task into the Daily Review
// sections, then emits one collapsible section header per non-empty section
// followed by its sorted task rows. It reuses the standalone model's thresholds,
// classification, and per-section sort comparators so the buckets and ordering
// are identical, but renders through the shared Outliner row model.
func (m *OutlinerModel) buildDailyReviewRows() {
	projects := m.workModeScopeProjects()
	th := newDailyReviewThresholds(m.runtime.Settings(), time.Now())

	buckets := make(map[DailyReviewSection][]CrossProjectTask, len(dailyReviewSectionOrder))
	for _, project := range projects {
		if project == nil {
			continue
		}
		predicate := m.workModeFilterPredicate(project)
		for _, task := range project.Tasks {
			if task == nil || !predicate(task) {
				continue
			}
			section, ok := classifyDailyReviewTask(task, project, th)
			if !ok {
				continue
			}
			buckets[section] = append(buckets[section], CrossProjectTask{Task: task, Project: project})
		}
	}

	for _, section := range dailyReviewSectionOrder {
		entries := buckets[section]
		if len(entries) == 0 {
			continue
		}
		sortDailyReviewSection(entries, dailyReviewSectionComparator(section))
		m.appendReviewSection(getSectionName(section), reviewCollapseKey(ModeDailyReview, int(section)), entries)
	}
}

// activitySectionActive / activitySectionCompleted index the two Activity
// sections for collapse-key derivation.
const (
	activitySectionActive    = 0
	activitySectionCompleted = 1
)

// buildActivityRows renders the Activity mode: every in-scope task ordered by
// UpdatedAt descending (ID tie-break), capped at the configured recent-activity
// limit, then split into Active and Completed collapsible sections. The cap is
// applied to the raw recency order before filtering, matching the standalone
// Recent Activity model's loadTasks → applyFilter sequence.
func (m *OutlinerModel) buildActivityRows() {
	projects := m.workModeScopeProjects()
	limit := m.runtime.Settings().GetRecentActivityLimit()

	var all []CrossProjectTask
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			all = append(all, CrossProjectTask{Task: task, Project: project})
		}
	}

	sort.Slice(all, func(i, j int) bool {
		ti, tj := all[i].Task.UpdatedAt, all[j].Task.UpdatedAt
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return all[i].Task.ID < all[j].Task.ID
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	var active, completed []CrossProjectTask
	for _, dt := range all {
		predicate := m.workModeFilterPredicate(dt.Project)
		if !predicate(dt.Task) {
			continue
		}
		if dt.Task.IsComplete() {
			completed = append(completed, dt)
		} else {
			active = append(active, dt)
		}
	}

	if len(active) > 0 {
		m.appendReviewSection("Active", reviewCollapseKey(ModeActivity, activitySectionActive), active)
	}
	if len(completed) > 0 {
		m.appendReviewSection("Completed", reviewCollapseKey(ModeActivity, activitySectionCompleted), completed)
	}
}

// appendReviewSection emits one section header row plus its task rows (unless the
// section is collapsed). Task rows carry the owning project ID in cross-project
// scope so project badges resolve.
func (m *OutlinerModel) appendReviewSection(name, collapseKey string, entries []CrossProjectTask) {
	m.rows = append(m.rows, outlinerRow{
		IsSectionHeader: true,
		SectionName:     name,
		SectionCount:    len(entries),
		CollapseKey:     collapseKey,
	})
	if m.reviewCollapsedSections[collapseKey] {
		return
	}
	for _, dt := range entries {
		row := outlinerRow{task: dt.Task}
		if m.project == nil && dt.Project != nil {
			row.ProjectID = dt.Project.ID
		}
		m.rows = append(m.rows, row)
	}
}
