package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// OutlinerModeRow is the representation-neutral output of a purpose-built row
// strategy. Membership and ordering remain flat here; the presentation layer
// later nests relatives that are both explicit members of the same section. A
// promoted child keeps its ancestor breadcrumb, and unselected descendants are
// never added implicitly.
type OutlinerModeRow struct {
	Task       *models.Task
	Project    *models.Project
	Breadcrumb string
}

// OutlinerModeInput bundles the scoped data a canonical row strategy needs.
// CandidateTasks optionally narrows eligibility to the current focus projection;
// a present project entry, including an empty one, is authoritative.
type OutlinerModeInput struct {
	Projects []*models.Project
	TodayIDs map[TaskIdentity]bool
	// CandidateTasks optionally narrows the tasks eligible for a projection per
	// project. A missing project entry falls back to Project.Tasks.
	CandidateTasks map[string][]*models.Task
	// TodayOrder and NextOrder are the durable project-qualified planning orders.
	TodayOrder  []PlanningTask
	NextOrder   []config.ProjectTaskLocator
	TaskFilter  func(task *models.Task, project *models.Project) bool
	TaskExclude func(task *models.Task, project *models.Project) bool
	// RecencyKeys freezes each task's Recent ordering timestamp so triaging a row
	// does not move it out from under the cursor. A task the map does not name
	// orders by its live UpdatedAt.
	RecencyKeys     map[TaskIdentity]time.Time
	Now             time.Time
	IncludeSnoozed  bool
	IncludeTerminal bool
}

func (in OutlinerModeInput) now() time.Time {
	if in.Now.IsZero() {
		return time.Now()
	}
	return in.Now
}

func (in OutlinerModeInput) recencyKey(task *models.Task, project *models.Project) time.Time {
	if frozen, ok := in.RecencyKeys[taskIdentityFor(task, project)]; ok {
		return frozen
	}
	return task.UpdatedAt
}

// BuildTodayModeRows realizes the explicit ordered Today projection. Every row
// is a durable member; section-local hierarchy is added by the row appender.
func BuildTodayModeRows(in OutlinerModeInput) []OutlinerModeRow {
	projects := modeProjectIndex(in.Projects)
	now := in.now()
	rows := make([]OutlinerModeRow, 0, len(in.TodayOrder))
	seen := make(map[TaskIdentity]bool, len(in.TodayOrder))
	for _, item := range in.TodayOrder {
		if item.Task == nil || item.Project == nil {
			continue
		}
		project := projects[item.Project.ID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(item.Task.ID)
		identity := TaskIdentity{ProjectID: project.ID, TaskID: item.Task.ID}
		if task == nil || seen[identity] || !modeCandidateContains(in, identity) {
			continue
		}
		seen[identity] = true
		if !planningTodayEligible(task) || (task.IsSnoozedAt(now) && !in.IncludeSnoozed) {
			continue
		}
		if in.TaskExclude != nil && in.TaskExclude(task, project) {
			continue
		}
		if in.TaskFilter != nil && !in.TaskFilter(task, project) {
			continue
		}
		rows = append(rows, planningModeRow(task, project))
	}
	return rows
}

// BuildNextModeRows realizes the explicit ordered Next projection. Every row is
// a durable member; section-local hierarchy is added by the row appender.
func BuildNextModeRows(in OutlinerModeInput) []OutlinerModeRow {
	projects := modeProjectIndex(in.Projects)
	now := in.now()
	rows := make([]OutlinerModeRow, 0, len(in.NextOrder))
	seen := make(map[TaskIdentity]bool, len(in.NextOrder))
	for _, locator := range in.NextOrder {
		identity := TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}
		if seen[identity] || !modeCandidateContains(in, identity) {
			continue
		}
		seen[identity] = true
		project := projects[locator.ProjectID]
		if project == nil {
			continue
		}
		task := project.GetTaskByID(locator.TaskID)
		if task == nil || !task.State().IsValid() || task.IsTerminalState() ||
			(task.IsSnoozedAt(now) && !in.IncludeSnoozed) {
			continue
		}
		if in.TaskExclude != nil && in.TaskExclude(task, project) {
			continue
		}
		if in.TaskFilter != nil && !in.TaskFilter(task, project) {
			continue
		}
		rows = append(rows, planningModeRow(task, project))
	}
	return rows
}

func modeProjectIndex(projects []*models.Project) map[string]*models.Project {
	index := make(map[string]*models.Project, len(projects))
	for _, project := range projects {
		if project != nil {
			index[project.ID] = project
		}
	}
	return index
}

func modeCandidateContains(in OutlinerModeInput, identity TaskIdentity) bool {
	candidates, constrained := in.CandidateTasks[identity.ProjectID]
	if !constrained {
		return true
	}
	for _, task := range candidates {
		if task != nil && task.ID == identity.TaskID {
			return true
		}
	}
	return false
}

func planningModeRow(task *models.Task, project *models.Project) OutlinerModeRow {
	return OutlinerModeRow{
		Task:       task,
		Project:    project,
		Breadcrumb: taskAncestorBreadcrumb(task),
	}
}
