package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// computeNextActionRecommendation builds the engine input from the model's
// cached rows + daily-plan slice and returns the recommendation that drives
// the cockpit's Next Action strip. Production callers (AppModel) populate
// cfg.DailyPlan with proper PlannedDate values; tests that bypass loadItems()
// only set m.dailyPlan, so the helper synthesizes today's plan entries from
// that slice when the config is empty.
func (m *FocusBoardModel) computeNextActionRecommendation() RecommendedAction {
	projects := m.collectFocusBoardProjects()
	plan := m.resolveDailyPlanForRecommendation()
	return Recommend(RecommendationEngineInput{
		Projects:  projects,
		DailyPlan: plan,
		Now:       time.Now(),
	})
}

// collectFocusBoardProjects walks every cached row slice and returns the
// unique projects backing them. Projects are returned in first-seen order so
// the engine sees a stable iteration order across renders.
func (m *FocusBoardModel) collectFocusBoardProjects() []*models.Project {
	seen := make(map[string]bool, 16)
	out := make([]*models.Project, 0, 16)
	visit := func(rows []boardRow) {
		for _, r := range rows {
			if r.Project == nil {
				continue
			}
			if seen[r.Project.ID] {
				continue
			}
			seen[r.Project.ID] = true
			out = append(out, r.Project)
		}
	}
	visit(m.todayRows)
	visit(m.nextRows)
	visit(m.closedRows)
	visit(m.allRows)
	return out
}

// resolveDailyPlanForRecommendation chooses the daily-plan slice the engine
// should consume. The persisted planning snapshot wins when populated (production
// path); otherwise the helper falls back to *m.dailyPlan with today's PlannedDate
// so in-memory test fixtures can drive the engine without a committed plan.
func (m *FocusBoardModel) resolveDailyPlanForRecommendation() []config.DailyPlanItem {
	if m.runtime != nil {
		if items := m.runtime.Planning().GetDailyPlan(); len(items) > 0 {
			return items
		}
	}
	if m.dailyPlan == nil || len(*m.dailyPlan) == 0 {
		return nil
	}
	todayKey := util.StartOfDay(time.Now()).Format(util.DateFormat)
	items := make([]config.DailyPlanItem, 0, len(*m.dailyPlan))
	for _, ft := range *m.dailyPlan {
		if ft.Task == nil || ft.Project == nil {
			continue
		}
		items = append(items, config.DailyPlanItem{
			TaskID:      ft.Task.ID,
			ProjectID:   ft.Project.ID,
			PlannedDate: todayKey,
		})
	}
	return items
}

// hasAnyProjectsForRecommendation reports whether the cockpit has any project
// backing it. Drives the first-run vs caught-up branch in the strip's AllDone
// copy. Cheap because it shares the same row iteration as the recommendation
// builder above.
func (m *FocusBoardModel) hasAnyProjectsForRecommendation() bool {
	check := func(rows []boardRow) bool {
		for _, r := range rows {
			if r.Project != nil {
				return true
			}
		}
		return false
	}
	return check(m.todayRows) || check(m.nextRows) || check(m.closedRows) || check(m.allRows)
}
