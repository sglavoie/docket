package tui

import (
	"cmp"
	"slices"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// recentModeCutoffDays bounds Recent: activity older than this is no longer
// something the user is triaging.
const recentModeCutoffDays = 30

// OutlinerModeSection is the representation-neutral grouped output for Due.
type OutlinerModeSection struct {
	Name        string
	CollapseKey string
	Rows        []OutlinerModeRow
}

const canonicalModeCollapseKeyPrefix = "_mode:"

func canonicalModeCollapseKey(mode OutlinerModeID, bucket string) string {
	return canonicalModeCollapseKeyPrefix + string(mode) + ":" + bucket
}

type orderedModeTask struct {
	row          OutlinerModeRow
	projectOrder int
	taskOrder    int
}

type recentModeTask struct {
	orderedModeTask
	key time.Time
}

// orderedModeTasks collects the in-scope candidate tasks in the caller's
// authoritative project-catalog order while retaining each project's stable
// document order as an explicit deterministic tie-break. CandidateTasks
// optionally narrows a zoomed scope.
func orderedModeTasks(in OutlinerModeInput) []orderedModeTask {
	projects := slices.Clone(in.Projects)

	var result []orderedModeTask
	for projectOrder, project := range projects {
		if project == nil {
			continue
		}
		tasks := project.Tasks
		if candidates, ok := in.CandidateTasks[project.ID]; ok {
			tasks = candidates
		}
		modelOrder := make(map[string]int, len(project.Tasks))
		for i, task := range project.Tasks {
			if task != nil {
				modelOrder[task.ID] = i
			}
		}
		for candidateOrder, task := range tasks {
			if task == nil {
				continue
			}
			order, ok := modelOrder[task.ID]
			if !ok {
				order = len(project.Tasks) + candidateOrder
			}
			result = append(result, orderedModeTask{
				row:          OutlinerModeRow{Task: task, Project: project},
				projectOrder: projectOrder,
				taskOrder:    order,
			})
		}
	}
	return result
}

func compareOrderedModeTasks(left, right orderedModeTask) int {
	if c := cmp.Compare(left.projectOrder, right.projectOrder); c != 0 {
		return c
	}
	if c := cmp.Compare(left.taskOrder, right.taskOrder); c != 0 {
		return c
	}
	return cmp.Compare(left.row.Task.ID, right.row.Task.ID)
}

func isComputedOpenWork(task *models.Task, in OutlinerModeInput) bool {
	if task == nil || (task.IsTerminalState() && !in.IncludeTerminal) {
		return false
	}
	return in.IncludeSnoozed || !task.IsSnoozedAt(in.now())
}

// BuildDueModeSections returns dated open work in the accepted local
// calendar-day buckets. Next Seven Days is tomorrow through today+7 inclusive.
func BuildDueModeSections(in OutlinerModeInput) []OutlinerModeSection {
	in.Now = in.now()
	sections := []OutlinerModeSection{
		{Name: "Overdue", CollapseKey: canonicalModeCollapseKey(ModeDue, "overdue")},
		{Name: "Today", CollapseKey: canonicalModeCollapseKey(ModeDue, "today")},
		{Name: "Next Seven Days", CollapseKey: canonicalModeCollapseKey(ModeDue, "next-seven-days")},
		{Name: "Later", CollapseKey: canonicalModeCollapseKey(ModeDue, "later")},
	}
	now := in.now()

	buckets := make([][]orderedModeTask, len(sections))
	for _, candidate := range orderedModeTasks(in) {
		task := candidate.row.Task
		if !isComputedOpenWork(task, in) || task.DueDate == nil {
			continue
		}
		bucket := 3
		switch days := util.DaysBetween(now, *task.DueDate); {
		case days < 0:
			bucket = 0
		case days == 0:
			bucket = 1
		case days <= 7:
			bucket = 2
		}
		buckets[bucket] = append(buckets[bucket], candidate)
	}

	var result []OutlinerModeSection
	for i := range sections {
		sort.SliceStable(buckets[i], func(left, right int) bool {
			l, r := buckets[i][left], buckets[i][right]
			if days := util.DaysBetween(*l.row.Task.DueDate, *r.row.Task.DueDate); days != 0 {
				return days > 0
			}
			return compareOrderedModeTasks(l, r) < 0
		})
		for _, candidate := range buckets[i] {
			candidate.row.Breadcrumb = taskAncestorBreadcrumb(candidate.row.Task)
			sections[i].Rows = append(sections[i].Rows, candidate.row)
		}
		if len(sections[i].Rows) > 0 {
			result = append(result, sections[i])
		}
	}
	return result
}

// BuildRecentModeSections returns tasks touched within the cutoff, newest first,
// in local calendar-day buckets. Terminal and snoozed membership is the caller's
// decision through IncludeTerminal and IncludeSnoozed.
func BuildRecentModeSections(in OutlinerModeInput) []OutlinerModeSection {
	in.Now = in.now()
	sections := []OutlinerModeSection{
		{Name: "Today", CollapseKey: canonicalModeCollapseKey(ModeRecent, "today")},
		{Name: "Yesterday", CollapseKey: canonicalModeCollapseKey(ModeRecent, "yesterday")},
		{Name: "Last 7 Days", CollapseKey: canonicalModeCollapseKey(ModeRecent, "last-seven-days")},
		{Name: "Last 30 Days", CollapseKey: canonicalModeCollapseKey(ModeRecent, "last-thirty-days")},
	}
	now := in.now()

	buckets := make([][]recentModeTask, len(sections))
	for _, candidate := range orderedModeTasks(in) {
		task := candidate.row.Task
		if !isComputedOpenWork(task, in) {
			continue
		}
		key := in.recencyKey(task, candidate.row.Project)
		age := util.DaysBetween(key, now)
		if age > recentModeCutoffDays {
			continue
		}
		bucket := 3
		switch {
		case age <= 0:
			bucket = 0
		case age == 1:
			bucket = 1
		case age <= 7:
			bucket = 2
		}
		buckets[bucket] = append(buckets[bucket], recentModeTask{orderedModeTask: candidate, key: key})
	}

	var result []OutlinerModeSection
	for i := range sections {
		sort.SliceStable(buckets[i], func(left, right int) bool {
			l, r := buckets[i][left], buckets[i][right]
			if !l.key.Equal(r.key) {
				return l.key.After(r.key)
			}
			return compareOrderedModeTasks(l.orderedModeTask, r.orderedModeTask) < 0
		})
		for _, candidate := range buckets[i] {
			candidate.row.Breadcrumb = taskAncestorBreadcrumb(candidate.row.Task)
			sections[i].Rows = append(sections[i].Rows, candidate.row)
		}
		if len(sections[i].Rows) > 0 {
			result = append(result, sections[i])
		}
	}
	return result
}
