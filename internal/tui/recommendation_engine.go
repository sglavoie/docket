package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// DefaultStaleThreshold marks tasks unedited for longer than this window with
// ReasonStale. Anything older than two weeks needs attention.
const DefaultStaleThreshold = 14 * 24 * time.Hour

// DefaultRecencyWindow flags tasks updated within this window as
// ReasonRecentlyActive — the fallback "momentum" reason.
const DefaultRecencyWindow = 3 * 24 * time.Hour

// RecommendationEngineInput collects everything the engine needs to produce a
// deterministic RecommendedAction. Projects must already be loaded; the
// engine performs no I/O and never touches storage. DailyPlan is the
// persisted plan slice (typically cfg.GetDailyPlan()). Now is the engine
// clock used for due-date, staleness, recency, and carry-over computation —
// callers should pass time.Now() in production and a fixed clock from tests.
//
// Engine output is always recomputed in memory; RecommendedAction values are
// not stable across changes to Projects/DailyPlan/Now and must not be
// persisted across sessions. Cockpit surfaces (sessions 4-8) recompute on
// each view render.
type RecommendationEngineInput struct {
	Projects       []*models.Project
	DailyPlan      []config.DailyPlanItem
	Now            time.Time
	StaleThreshold time.Duration
	RecencyWindow  time.Duration
}

// Recommend evaluates input deterministically and returns the next-action
// recommendation. Given identical inputs (including a fixed Now), output is
// byte-identical: same Kind, same Task/Project pointers, same ordered
// Reasons slice, same GeneratedAt.
//
// Ranking order matches the redesign brief:
//  1. Urgent actionable work (overdue, due today)
//  2. Planned-today actionable work
//  3. Carried-over planned work (planned for past date, still open)
//  4. Blocked review decisions
//  5. Inbox triage decisions
//  6. High-priority actionable work (priority 1 or 2)
//  7. Stale active work (UpdatedAt older than StaleThreshold)
//  8. Recently active / momentum fallbacks
//  9. Plain backlog -> PlanToday
//
// The selected RecommendationKind reflects what the cockpit should ask the
// user to do next — work on a specific task (WorkOnTask / ReviewBlocked) or
// take a cockpit-wide decision (TriageInbox / PlanToday / AllDone).
func Recommend(input RecommendationEngineInput) RecommendedAction {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	staleThreshold := input.StaleThreshold
	if staleThreshold <= 0 {
		staleThreshold = DefaultStaleThreshold
	}
	recencyWindow := input.RecencyWindow
	if recencyWindow <= 0 {
		recencyWindow = DefaultRecencyWindow
	}

	today := util.StartOfDay(now)
	todayKey := today.Format(util.DateFormat)
	plannedToday, carriedOver := indexDailyPlan(input.DailyPlan, todayKey)

	var actionable, inbox []recommendCandidate
	for _, ft := range focus.CollectActionableTasks(input.Projects) {
		if ft.Task == nil || ft.Project == nil {
			continue
		}
		cand := recommendCandidate{task: ft.Task, project: ft.Project}
		if ft.Task.Section == models.SectionInbox {
			cand.reasons = []RecommendationReason{ReasonInboxNeedsTriage}
			inbox = append(inbox, cand)
			continue
		}
		classifyActionable(&cand, now, today, recencyWindow, staleThreshold, plannedToday, carriedOver)
		actionable = append(actionable, cand)
	}

	blocked := collectBlockedCandidates(input.Projects, today, plannedToday, carriedOver)

	sortActionableCandidates(actionable)
	sortInboxCandidates(inbox)
	sortBlockedCandidates(blocked)

	return chooseRecommendationKind(actionable, inbox, blocked, now)
}

// recommendCandidate is the engine's internal scoring envelope. tier groups
// candidates by ranking-bucket so the kind selection layer can ask "is the
// top actionable task urgent?" without re-walking reasons. Lower tier ranks
// higher.
type recommendCandidate struct {
	task    *models.Task
	project *models.Project
	tier    int
	reasons []RecommendationReason
}

// candidateTier scale. Order mirrors the engine's ranking spec; values are
// not exported because the cockpit consumes Reasons, not tiers.
const (
	tierOverdue        = 0
	tierDueToday       = 1
	tierPlannedToday   = 2
	tierCarriedOver    = 3
	tierHighPriority   = 4
	tierStale          = 5
	tierRecentlyActive = 6
	tierFallback       = 7
)

func indexDailyPlan(plan []config.DailyPlanItem, todayKey string) (planned, carried map[string]bool) {
	planned = make(map[string]bool, len(plan))
	carried = make(map[string]bool, len(plan))
	for _, item := range plan {
		if item.TaskID == "" {
			continue
		}
		key := planKey(item.TaskID, item.ProjectID)
		switch {
		case item.PlannedDate == todayKey:
			planned[key] = true
		case item.PlannedDate != "" && item.PlannedDate < todayKey:
			carried[key] = true
		}
	}
	return planned, carried
}

func planKey(taskID, projectID string) string {
	return projectID + "::" + taskID
}

func classifyActionable(c *recommendCandidate, now, today time.Time, recencyWindow, staleThreshold time.Duration, plannedToday, carriedOver map[string]bool) {
	task := c.task
	project := c.project
	key := planKey(task.ID, project.ID)

	var reasons []RecommendationReason
	tier := tierFallback

	if task.DueDate != nil {
		dueDay := util.StartOfDay(*task.DueDate)
		switch {
		case dueDay.Before(today):
			reasons = append(reasons, ReasonOverdue)
			if tier > tierOverdue {
				tier = tierOverdue
			}
		case dueDay.Equal(today):
			reasons = append(reasons, ReasonDueToday)
			if tier > tierDueToday {
				tier = tierDueToday
			}
		}
	}

	if plannedToday[key] {
		reasons = append(reasons, ReasonPlannedToday)
		if tier > tierPlannedToday {
			tier = tierPlannedToday
		}
	}

	if carriedOver[key] {
		reasons = append(reasons, ReasonCarriedOver)
		if tier > tierCarriedOver {
			tier = tierCarriedOver
		}
	}

	if task.Priority == 1 || task.Priority == 2 {
		reasons = append(reasons, ReasonHighPriority)
		if tier > tierHighPriority {
			tier = tierHighPriority
		}
	}

	if !task.UpdatedAt.IsZero() {
		elapsed := now.Sub(task.UpdatedAt)
		if elapsed > staleThreshold {
			reasons = append(reasons, ReasonStale)
			if tier > tierStale {
				tier = tierStale
			}
		}
		if elapsed >= 0 && elapsed <= recencyWindow {
			reasons = append(reasons, ReasonRecentlyActive)
			if tier > tierRecentlyActive {
				tier = tierRecentlyActive
			}
		}
	}

	c.reasons = reasons
	c.tier = tier
}

func collectBlockedCandidates(projects []*models.Project, today time.Time, plannedToday, carriedOver map[string]bool) []recommendCandidate {
	var blocked []recommendCandidate
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.Status.IsComplete() || task.IsSnoozed() {
				continue
			}
			isBlocked := task.Status == models.StatusBlocked || focus.HasIncompleteDependencies(task, project, projects)
			if !isBlocked {
				continue
			}
			cand := recommendCandidate{task: task, project: project}
			reasons := []RecommendationReason{ReasonBlocked}
			tier := tierFallback
			key := planKey(task.ID, project.ID)
			if task.DueDate != nil {
				dueDay := util.StartOfDay(*task.DueDate)
				switch {
				case dueDay.Before(today):
					reasons = append(reasons, ReasonOverdue)
					if tier > tierOverdue {
						tier = tierOverdue
					}
				case dueDay.Equal(today):
					reasons = append(reasons, ReasonDueToday)
					if tier > tierDueToday {
						tier = tierDueToday
					}
				}
			}
			if plannedToday[key] {
				reasons = append(reasons, ReasonPlannedToday)
				if tier > tierPlannedToday {
					tier = tierPlannedToday
				}
			}
			if carriedOver[key] {
				reasons = append(reasons, ReasonCarriedOver)
				if tier > tierCarriedOver {
					tier = tierCarriedOver
				}
			}
			if task.Priority == 1 || task.Priority == 2 {
				reasons = append(reasons, ReasonHighPriority)
				if tier > tierHighPriority {
					tier = tierHighPriority
				}
			}
			cand.reasons = reasons
			cand.tier = tier
			blocked = append(blocked, cand)
		}
	}
	return blocked
}

func sortActionableCandidates(s []recommendCandidate) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if a.tier != b.tier {
			return a.tier < b.tier
		}
		ap, bp := clampRecommendPriority(a.task.Priority), clampRecommendPriority(b.task.Priority)
		if ap != bp {
			return ap < bp
		}
		aDue := dueOrMax(a.task.DueDate)
		bDue := dueOrMax(b.task.DueDate)
		if !aDue.Equal(bDue) {
			return aDue.Before(bDue)
		}
		if !a.task.UpdatedAt.Equal(b.task.UpdatedAt) {
			if a.tier == tierStale && b.tier == tierStale {
				return a.task.UpdatedAt.Before(b.task.UpdatedAt)
			}
			return a.task.UpdatedAt.After(b.task.UpdatedAt)
		}
		if a.task.ID != b.task.ID {
			return a.task.ID < b.task.ID
		}
		return a.project.ID < b.project.ID
	})
}

func sortInboxCandidates(s []recommendCandidate) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := s[i], s[j]
		ap, bp := clampRecommendPriority(a.task.Priority), clampRecommendPriority(b.task.Priority)
		if ap != bp {
			return ap < bp
		}
		if !a.task.CreatedAt.Equal(b.task.CreatedAt) {
			return a.task.CreatedAt.Before(b.task.CreatedAt)
		}
		if a.task.ID != b.task.ID {
			return a.task.ID < b.task.ID
		}
		return a.project.ID < b.project.ID
	})
}

func sortBlockedCandidates(s []recommendCandidate) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if a.tier != b.tier {
			return a.tier < b.tier
		}
		ap, bp := clampRecommendPriority(a.task.Priority), clampRecommendPriority(b.task.Priority)
		if ap != bp {
			return ap < bp
		}
		aDue := dueOrMax(a.task.DueDate)
		bDue := dueOrMax(b.task.DueDate)
		if !aDue.Equal(bDue) {
			return aDue.Before(bDue)
		}
		if !a.task.UpdatedAt.Equal(b.task.UpdatedAt) {
			return a.task.UpdatedAt.After(b.task.UpdatedAt)
		}
		if a.task.ID != b.task.ID {
			return a.task.ID < b.task.ID
		}
		return a.project.ID < b.project.ID
	})
}

func chooseRecommendationKind(actionable, inbox, blocked []recommendCandidate, now time.Time) RecommendedAction {
	out := RecommendedAction{GeneratedAt: now}

	// Urgent / planned / carried-over actionable work always wins.
	if len(actionable) > 0 && actionable[0].tier <= tierCarriedOver {
		top := actionable[0]
		out.Kind = RecommendationKindWorkOnTask
		out.Task = top.task
		out.Project = top.project
		out.Reasons = top.reasons
		return out
	}

	// Blocked work is not actionable yet; review it before expanding the
	// backlog or triaging non-urgent inbox items.
	if len(blocked) > 0 {
		top := blocked[0]
		out.Kind = RecommendationKindReviewBlocked
		out.Task = top.task
		out.Project = top.project
		out.Reasons = top.reasons
		return out
	}

	// No urgent/planned/blocked work: triage inbox before lower-tier work.
	if len(inbox) > 0 {
		out.Kind = RecommendationKindTriageInbox
		out.Reasons = []RecommendationReason{ReasonInboxNeedsTriage}
		return out
	}

	// Standalone high-priority, stale, or recently-active work is concrete
	// enough for a direct task recommendation.
	if len(actionable) > 0 && actionable[0].tier <= tierRecentlyActive {
		top := actionable[0]
		out.Kind = RecommendationKindWorkOnTask
		out.Task = top.task
		out.Project = top.project
		out.Reasons = top.reasons
		return out
	}

	// Actionable tasks remain, but none carry a cockpit signal strong enough
	// to choose one directly — invite the user to plan the day.
	if len(actionable) > 0 {
		out.Kind = RecommendationKindPlanToday
		return out
	}

	// No actionable, no inbox, no blocked: cockpit is at rest.
	out.Kind = RecommendationKindAllDone
	return out
}

func clampRecommendPriority(p int) int {
	if p < models.PriorityMin {
		return models.PriorityMax
	}
	if p > models.PriorityMax {
		return models.PriorityMax
	}
	return p
}

var recommendMaxTime = time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)

func dueOrMax(t *time.Time) time.Time {
	if t == nil {
		return recommendMaxTime
	}
	return *t
}
