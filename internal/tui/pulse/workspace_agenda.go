package pulse

import (
	"cmp"
	"slices"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// AgendaLimit bounds how many tasks an agenda names. A host lists them in a
// menu or a widget, neither of which has room for more, and Total still says
// how many there are.
const AgendaLimit = 20

// AgendaKind is why a task is on the agenda.
type AgendaKind uint8

const (
	// AgendaOverdue is a task whose deadline has passed. It outranks Today, so
	// an overdue task planned for today is listed once, as overdue.
	AgendaOverdue AgendaKind = iota
	AgendaToday
)

// AgendaItem is one task the pulse's Today or Overdue count includes, named by
// its stored identity so a host can show it without a slug or a ShortID.
type AgendaItem struct {
	Kind        AgendaKind
	ProjectID   string
	TaskID      string
	Title       string
	ProjectName string
	Priority    int
	Due         *time.Time
	DueHasTime  bool
	// Reviewable is whether acknowledging it is allowed now, as an
	// Arrival's is, so a host offers that action only where R would accept it.
	Reviewable bool
}

// WorkspaceAgenda is the tasks behind the pulse's Today and Overdue counts:
// at most AgendaLimit of them, overdue first, and Total distinct tasks in all.
type WorkspaceAgenda struct {
	Items []AgendaItem
	Total int
}

// ComputeWorkspaceAgenda lists the work ComputeWorkspacePulse counts as Today
// or Overdue, over the same population and with the same exclusions, so a
// host's list never disagrees with the count beside it. Overdue tasks lead,
// oldest deadline first; Today follows by priority, then deadline. Ties fall
// to the project's name and the task's ShortID, so the order is stable.
func ComputeWorkspaceAgenda(projects []*models.Project, todayIDs map[taskref.TaskIdentity]bool, now time.Time) WorkspaceAgenda {
	type candidate struct {
		item    AgendaItem
		shortID int
	}
	var candidates []candidate
	seen := make(map[taskref.TaskIdentity]bool)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.State().IsTerminal() || task.IsDeferredAt(now) ||
				models.TaskEffectivelyHidden(project, task) {
				continue
			}
			identity := taskref.For(task, project)
			if !identity.Valid() || seen[identity] {
				continue
			}
			kind := AgendaToday
			switch {
			case TaskIsOverdueAt(task, now):
				kind = AgendaOverdue
			case todayIDs[identity] && app.PlanningTodayEligible(task.State()):
			default:
				continue
			}
			seen[identity] = true
			candidates = append(candidates, candidate{
				item: AgendaItem{
					Kind:        kind,
					ProjectID:   project.ID,
					TaskID:      task.ID,
					Title:       task.Title,
					ProjectName: project.Name,
					Priority:    task.Priority,
					Due:         task.DueDate,
					DueHasTime:  task.DueHasTime(),
					Reviewable:  models.TaskAllowsExplicitReview(task, now),
				},
				shortID: task.ShortID,
			})
		}
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if c := cmp.Compare(a.item.Kind, b.item.Kind); c != 0 {
			return c
		}
		priority := cmp.Compare(models.EffectivePriority(a.item.Priority), models.EffectivePriority(b.item.Priority))
		due := compareAgendaDue(a.item.Due, b.item.Due)
		if a.item.Kind == AgendaOverdue {
			priority, due = due, priority
		}
		if priority != 0 {
			return priority
		}
		if due != 0 {
			return due
		}
		if c := cmp.Compare(a.item.ProjectName, b.item.ProjectName); c != 0 {
			return c
		}
		return cmp.Compare(a.shortID, b.shortID)
	})
	agenda := WorkspaceAgenda{Total: len(candidates)}
	for _, c := range candidates[:min(len(candidates), AgendaLimit)] {
		agenda.Items = append(agenda.Items, c.item)
	}
	return agenda
}

// compareAgendaDue orders deadlines earliest first, with no deadline last.
func compareAgendaDue(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	default:
		return a.Compare(*b)
	}
}

// Equal reports whether two agendas name the same tasks the same way, so an
// unchanged one is never sent again.
func (a WorkspaceAgenda) Equal(b WorkspaceAgenda) bool {
	return a.Total == b.Total && slices.EqualFunc(a.Items, b.Items, func(x, y AgendaItem) bool {
		return x.Kind == y.Kind && x.ProjectID == y.ProjectID && x.TaskID == y.TaskID &&
			x.Title == y.Title && x.ProjectName == y.ProjectName && x.Priority == y.Priority &&
			x.DueHasTime == y.DueHasTime && x.Reviewable == y.Reviewable && compareAgendaDue(x.Due, y.Due) == 0
	})
}
