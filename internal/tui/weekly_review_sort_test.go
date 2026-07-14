package tui

import (
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestWeeklyReviewSort_Determinism asserts that sortSections produces the
// same ordering across runs when equal-key items are present. Without a
// stable tiebreak by ID, sort.Slice may flip equal elements.
func TestWeeklyReviewSort_Determinism(t *testing.T) {
	mkProject := func(id string, no, inbox, blocked, inprog int) WeeklyReviewProject {
		return WeeklyReviewProject{
			Project:         &models.Project{ID: id, Slug: id, Name: id},
			NoActivityDays:  no,
			InboxCount:      inbox,
			BlockedCount:    blocked,
			InProgressCount: inprog,
		}
	}
	mkTask := func(id string, pri int, due *time.Time, updated, created time.Time) WeeklyReviewTask {
		return WeeklyReviewTask{
			Task: &models.Task{
				ID:        id,
				Priority:  pri,
				DueDate:   due,
				UpdatedAt: updated,
				CreatedAt: created,
			},
			Project: &models.Project{ID: "p"},
		}
	}

	due := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	t0 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	// Build two parallel models with identical input. Each section has
	// multiple items that tie on the primary sort key.
	build := func() *WeeklyReviewModel {
		cfg := newTestConfig()
		m := newTestWeeklyReviewModel(cfg)
		m.staleProjects = []WeeklyReviewProject{
			mkProject("pa", 10, 0, 0, 0),
			mkProject("pb", 10, 0, 0, 0),
			mkProject("pc", 10, 0, 0, 0),
		}
		m.inboxBacklog = []WeeklyReviewProject{
			mkProject("pa", 0, 5, 0, 0),
			mkProject("pb", 0, 5, 0, 0),
		}
		m.blockedWork = []WeeklyReviewProject{
			mkProject("pa", 0, 0, 3, 0),
			mkProject("pb", 0, 0, 3, 0),
		}
		m.overcommitted = []WeeklyReviewProject{
			mkProject("pa", 0, 0, 0, 7),
			mkProject("pb", 0, 0, 0, 7),
		}
		m.staleTasks = []WeeklyReviewTask{
			mkTask("ta", 0, nil, t0, t0),
			mkTask("tb", 0, nil, t0, t0),
			mkTask("tc", 0, nil, t0, t0),
		}
		m.upcomingWeek = []WeeklyReviewTask{
			mkTask("ua", 2, &due, t0, t0),
			mkTask("ub", 2, &due, t0, t0),
			mkTask("uc", 2, &due, t0, t0),
		}
		m.recurringOverview = []WeeklyReviewTask{
			mkTask("ra", 1, &due, t0, t0),
			mkTask("rb", 1, &due, t0, t0),
		}
		return m
	}

	collectProjIDs := func(s []WeeklyReviewProject) []string {
		out := make([]string, len(s))
		for i, p := range s {
			out[i] = p.Project.ID
		}
		return out
	}
	collectTaskIDs := func(s []WeeklyReviewTask) []string {
		out := make([]string, len(s))
		for i, w := range s {
			out[i] = w.Task.ID
		}
		return out
	}

	// First sort.
	m1 := build()
	m1.sortSections()
	// Second sort with same input. Since sort.Slice is randomized for
	// equal elements, a second pass starting from a *different* input
	// permutation must still converge to the same canonical order.
	m2 := build()
	// Permute input order so any reliance on input order surfaces.
	m2.staleProjects[0], m2.staleProjects[2] = m2.staleProjects[2], m2.staleProjects[0]
	m2.inboxBacklog[0], m2.inboxBacklog[1] = m2.inboxBacklog[1], m2.inboxBacklog[0]
	m2.blockedWork[0], m2.blockedWork[1] = m2.blockedWork[1], m2.blockedWork[0]
	m2.overcommitted[0], m2.overcommitted[1] = m2.overcommitted[1], m2.overcommitted[0]
	m2.staleTasks[0], m2.staleTasks[2] = m2.staleTasks[2], m2.staleTasks[0]
	m2.upcomingWeek[0], m2.upcomingWeek[2] = m2.upcomingWeek[2], m2.upcomingWeek[0]
	m2.recurringOverview[0], m2.recurringOverview[1] = m2.recurringOverview[1], m2.recurringOverview[0]
	m2.sortSections()

	check := func(name string, a, b []string) {
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: non-deterministic ordering\n run1=%v\n run2=%v", name, a, b)
		}
	}
	check("staleProjects", collectProjIDs(m1.staleProjects), collectProjIDs(m2.staleProjects))
	check("inboxBacklog", collectProjIDs(m1.inboxBacklog), collectProjIDs(m2.inboxBacklog))
	check("blockedWork", collectProjIDs(m1.blockedWork), collectProjIDs(m2.blockedWork))
	check("overcommitted", collectProjIDs(m1.overcommitted), collectProjIDs(m2.overcommitted))
	check("staleTasks", collectTaskIDs(m1.staleTasks), collectTaskIDs(m2.staleTasks))
	check("upcomingWeek", collectTaskIDs(m1.upcomingWeek), collectTaskIDs(m2.upcomingWeek))
	check("recurringOverview", collectTaskIDs(m1.recurringOverview), collectTaskIDs(m2.recurringOverview))
}
