package tui

import (
	"cmp"
	"slices"
	"sort"
	"strconv"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// OutlinerModeSection is the representation-neutral grouped output shared by
// Due and Priority. Blocked and Parked are intentionally flat fixed-order
// strategies and therefore return OutlinerModeRow directly.
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

// orderedModeTasks collects the in-scope candidate tasks in project display
// order while retaining each project's stable model order as an explicit
// deterministic tie-break. CandidateTasks optionally narrows a zoomed scope.
func orderedModeTasks(in OutlinerModeInput) []orderedModeTask {
	projects := slices.Clone(in.Projects)
	sortProjectsByDisplayOrder(projects)

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

func effectiveModePriority(task *models.Task) int {
	if task == nil || models.ValidatePriority(task.Priority, false) != nil {
		return models.PriorityDefault
	}
	return task.Priority
}

func isComputedOpenWork(task *models.Task, in OutlinerModeInput) bool {
	if task == nil || task.IsTerminalState() {
		return false
	}
	return in.IncludeSnoozed || !task.IsSnoozedAt(in.now())
}

// BuildDueModeSections returns dated open work in the accepted local
// calendar-day buckets. Next Seven Days is tomorrow through today+7 inclusive.
func BuildDueModeSections(in OutlinerModeInput) []OutlinerModeSection {
	sections := []OutlinerModeSection{
		{Name: "Overdue", CollapseKey: canonicalModeCollapseKey(ModeDue, "overdue")},
		{Name: "Today", CollapseKey: canonicalModeCollapseKey(ModeDue, "today")},
		{Name: "Next Seven Days", CollapseKey: canonicalModeCollapseKey(ModeDue, "next-seven-days")},
		{Name: "Later", CollapseKey: canonicalModeCollapseKey(ModeDue, "later")},
	}
	today := util.StartOfDay(in.now())
	laterStart := today.AddDate(0, 0, 8)

	buckets := make([][]orderedModeTask, len(sections))
	for _, candidate := range orderedModeTasks(in) {
		task := candidate.row.Task
		if !isComputedOpenWork(task, in) || task.DueDate == nil {
			continue
		}
		dueDay := util.StartOfDay(*task.DueDate)
		bucket := 3
		switch {
		case dueDay.Before(today):
			bucket = 0
		case dueDay.Equal(today):
			bucket = 1
		case dueDay.Before(laterStart):
			bucket = 2
		}
		buckets[bucket] = append(buckets[bucket], candidate)
	}

	var result []OutlinerModeSection
	for i := range sections {
		sort.SliceStable(buckets[i], func(left, right int) bool {
			l, r := buckets[i][left], buckets[i][right]
			if !l.row.Task.DueDate.Equal(*r.row.Task.DueDate) {
				return l.row.Task.DueDate.Before(*r.row.Task.DueDate)
			}
			return compareOrderedModeTasks(l, r) < 0
		})
		for _, candidate := range buckets[i] {
			sections[i].Rows = append(sections[i].Rows, candidate.row)
		}
		if len(sections[i].Rows) > 0 {
			result = append(result, sections[i])
		}
	}
	return result
}

// BuildPriorityModeSections returns every open task in exact P1-P5 order.
// Within a priority bucket, project display and stable model order decide ties.
func BuildPriorityModeSections(in OutlinerModeInput) []OutlinerModeSection {
	sections := make([]OutlinerModeSection, models.PriorityMax)
	for priority := models.PriorityMin; priority <= models.PriorityMax; priority++ {
		sections[priority-1] = OutlinerModeSection{
			Name:        "P" + strconv.Itoa(priority),
			CollapseKey: canonicalModeCollapseKey(ModePriority, "p"+strconv.Itoa(priority)),
		}
	}
	for _, candidate := range orderedModeTasks(in) {
		if !isComputedOpenWork(candidate.row.Task, in) {
			continue
		}
		priority := effectiveModePriority(candidate.row.Task)
		sections[priority-1].Rows = append(sections[priority-1].Rows, candidate.row)
	}
	return slices.DeleteFunc(sections, func(section OutlinerModeSection) bool {
		return len(section.Rows) == 0
	})
}

// BuildBlockedModeRows returns dependency-blocked open work ordered by
// priority, dated-before-undated due date, project display order, stable model
// order, and task ID.
func BuildBlockedModeRows(in OutlinerModeInput) []OutlinerModeRow {
	var candidates []orderedModeTask
	for _, candidate := range orderedModeTasks(in) {
		task, project := candidate.row.Task, candidate.row.Project
		if !isComputedOpenWork(task, in) || !task.IsDependencyBlocked(project) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		l, r := candidates[left], candidates[right]
		if c := cmp.Compare(effectiveModePriority(l.row.Task), effectiveModePriority(r.row.Task)); c != 0 {
			return c < 0
		}
		switch {
		case l.row.Task.DueDate != nil && r.row.Task.DueDate == nil:
			return true
		case l.row.Task.DueDate == nil && r.row.Task.DueDate != nil:
			return false
		case l.row.Task.DueDate != nil && r.row.Task.DueDate != nil &&
			!l.row.Task.DueDate.Equal(*r.row.Task.DueDate):
			return l.row.Task.DueDate.Before(*r.row.Task.DueDate)
		}
		return compareOrderedModeTasks(l, r) < 0
	})
	return orderedCandidatesToRows(candidates)
}

// BuildParkedModeRows returns parked-note open work newest-first, followed by
// priority, project display order, stable model order, and task ID. A non-empty
// legacy parked note without ParkedAt is retained defensively and sorts last.
func BuildParkedModeRows(in OutlinerModeInput) []OutlinerModeRow {
	var candidates []orderedModeTask
	for _, candidate := range orderedModeTasks(in) {
		task := candidate.row.Task
		if !isComputedOpenWork(task, in) || !task.IsParked() {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		l, r := candidates[left], candidates[right]
		switch {
		case l.row.Task.ParkedAt != nil && r.row.Task.ParkedAt == nil:
			return true
		case l.row.Task.ParkedAt == nil && r.row.Task.ParkedAt != nil:
			return false
		case l.row.Task.ParkedAt != nil && r.row.Task.ParkedAt != nil &&
			!l.row.Task.ParkedAt.Equal(*r.row.Task.ParkedAt):
			return l.row.Task.ParkedAt.After(*r.row.Task.ParkedAt)
		}
		if c := cmp.Compare(effectiveModePriority(l.row.Task), effectiveModePriority(r.row.Task)); c != 0 {
			return c < 0
		}
		return compareOrderedModeTasks(l, r) < 0
	})
	return orderedCandidatesToRows(candidates)
}

func orderedCandidatesToRows(candidates []orderedModeTask) []OutlinerModeRow {
	rows := make([]OutlinerModeRow, len(candidates))
	for i := range candidates {
		rows[i] = candidates[i].row
	}
	return rows
}
