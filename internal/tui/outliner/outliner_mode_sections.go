package outliner

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/util"
)

// recentModeCutoffDays bounds Recent: activity older than this is no longer
// something the user is triaging.
const recentModeCutoffDays = 30

// OutlinerModeSection is the representation-neutral grouped output for Due.
type OutlinerModeSection struct {
	Name        string
	CollapseKey string
	Rows        []OutlinerModeRow
	// Lead is the section's order without its stored-order tiebreak, which is
	// what a project block ranks by; a tie falls to the project order. Nil
	// ranks blocks by position alone.
	Lead func(a, b OutlinerModeRow) int
}

const canonicalModeCollapseKeyPrefix = "_mode:"

func canonicalModeCollapseKey(mode chrome.OutlinerModeID, bucket string) string {
	return canonicalModeCollapseKeyPrefix + string(mode) + ":" + bucket
}

// canonicalModeProjectCollapseKeyPrefix namespaces the project groups a
// canonical mode owns. Each grouping surface keeps a disjoint key namespace so a
// fold left behind in one mode can never bind to a row in the mode you enter
// next; collapsedSections is one map that survives every mode change.
const canonicalModeProjectCollapseKeyPrefix = "_mode_project:"

// modeProjectCollapseKey identifies one project's group in a mode that emits no
// bucket header of its own.
func modeProjectCollapseKey(mode chrome.OutlinerModeID, projectID string) string {
	return canonicalModeProjectCollapseKeyPrefix + string(mode) + ":" + projectID
}

// modeSectionProjectCollapseKey identifies one project's group inside one bucket
// of a computed mode. It derives from the section's own key so the bucket
// vocabulary lives in exactly one place.
func modeSectionProjectCollapseKey(sectionCollapseKey, projectID string) string {
	return canonicalModeProjectCollapseKeyPrefix +
		strings.TrimPrefix(sectionCollapseKey, canonicalModeCollapseKeyPrefix) + ":" + projectID
}

type orderedModeTask struct {
	row          OutlinerModeRow
	projectOrder int
	taskOrder    int
}

// orderedModeTasks collects the in-scope candidate tasks in the caller's
// authoritative project-catalog order while retaining each project's stable
// document order as an explicit deterministic tie-break. CandidateTasks
// optionally narrows a focused scope.
//
// A project that is not narrowed is walked in document order, so a task's
// position is its index; only a narrowed project needs the index map that finds
// each candidate's document position. Several sections read this on every row
// build, and indexing every task of a large workspace each time was most of
// what it cost.
func orderedModeTasks(in OutlinerModeInput) []orderedModeTask {
	projects := slices.Clone(in.Projects)

	count := 0
	for _, project := range projects {
		if project == nil {
			continue
		}
		if candidates, ok := in.CandidateTasks[project.ID]; ok {
			count += len(candidates)
		} else {
			count += len(project.Tasks)
		}
	}
	result := make([]orderedModeTask, 0, count)
	for projectOrder, project := range projects {
		if project == nil {
			continue
		}
		candidates, narrowed := in.CandidateTasks[project.ID]
		if !narrowed {
			for order, task := range project.Tasks {
				if task != nil {
					result = append(result, orderedModeTask{
						row:          OutlinerModeRow{Task: task, Project: project},
						projectOrder: projectOrder,
						taskOrder:    order,
					})
				}
			}
			continue
		}
		modelOrder := make(map[string]int, len(project.Tasks))
		for i, task := range project.Tasks {
			if task != nil {
				modelOrder[task.ID] = i
			}
		}
		for candidateOrder, task := range candidates {
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
	return in.IncludeDeferred || !task.IsDeferredAt(in.now())
}

// BuildDueModeSections returns dated open work in the accepted local
// calendar-day buckets. Next Seven Days is tomorrow through today+7 inclusive.
func BuildDueModeSections(in OutlinerModeInput) []OutlinerModeSection {
	in.Now = in.now()
	sections := []OutlinerModeSection{
		{Name: "Overdue", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "overdue")},
		{Name: "Today", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "today")},
		{Name: "Next Seven Days", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "next-seven-days")},
		{Name: "Later", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "later")},
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

	// A deadline leads, priority breaks a same-day tie, and stored order breaks
	// the rest.
	lead := func(l, r OutlinerModeRow) int {
		if days := util.DaysBetween(*l.Task.DueDate, *r.Task.DueDate); days != 0 {
			return -cmp.Compare(days, 0)
		}
		return comparePriorityBand(l.Task, r.Task)
	}
	var result []OutlinerModeSection
	for i := range sections {
		sections[i].Lead = lead
		sort.SliceStable(buckets[i], func(left, right int) bool {
			l, r := buckets[i][left], buckets[i][right]
			if c := lead(l.row, r.row); c != 0 {
				return c < 0
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
// in local calendar-day buckets. Terminal and deferred membership is the caller's
// decision through IncludeTerminal and IncludeDeferred.
func BuildRecentModeSections(in OutlinerModeInput) []OutlinerModeSection {
	in.Now = in.now()
	sections := []OutlinerModeSection{
		{Name: "Today", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "today")},
		{Name: "Yesterday", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "yesterday")},
		{Name: "Last 7 Days", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "last-seven-days")},
		{Name: "Last 30 Days", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "last-thirty-days")},
	}
	now := in.now()

	buckets := make([][]orderedModeTask, len(sections))
	for _, candidate := range orderedModeTasks(in) {
		task := candidate.row.Task
		if !isComputedOpenWork(task, in) {
			continue
		}
		// Stored instants load in UTC, and DaysBetween compares calendar dates,
		// so the key is read in now's location before the day is taken.
		key := in.recencyKey(task, candidate.row.Project).In(now.Location())
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
		buckets[bucket] = append(buckets[bucket], candidate)
	}

	// The newest touch leads, priority breaks an exact tie, and stored order
	// breaks the rest.
	lead := func(l, r OutlinerModeRow) int {
		if c := in.recencyKey(r.Task, r.Project).Compare(in.recencyKey(l.Task, l.Project)); c != 0 {
			return c
		}
		return comparePriorityBand(l.Task, r.Task)
	}
	var result []OutlinerModeSection
	for i := range sections {
		sections[i].Lead = lead
		sort.SliceStable(buckets[i], func(left, right int) bool {
			l, r := buckets[i][left], buckets[i][right]
			if c := lead(l.row, r.row); c != 0 {
				return c < 0
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
