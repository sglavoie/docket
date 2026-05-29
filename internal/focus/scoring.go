package focus

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// ScoredFocusTask wraps a FocusTask with its computed score and reason.
type ScoredFocusTask struct {
	FocusTask
	Score  int
	Reason string
}

// computeCrossProjectScore calculates the suggestion score for a task.
// Higher scores = more recommended as the next task to work on.
// Returns 0 when task is nil; nil project is tolerated and skips
// project-scoped scoring contributions.
func computeCrossProjectScore(task *models.Task, project *models.Project, cfg *config.Config, now time.Time) int {
	if task == nil {
		return 0
	}
	score := 0

	// Priority weight: priority 1 = 5000, priority 5 = 1000.
	// Clamp malformed/imported values so priority 0 cannot outrank critical
	// work and extreme values cannot bury otherwise actionable tasks.
	priority := min(max(task.Priority, models.PriorityMin), models.PriorityMax)
	score += (6 - priority) * 1000

	// Bug type bonus
	if task.Type == models.TaskTypeBug {
		score += 100
	}

	// Due date proximity. Overdue and near-term due dates should increase
	// focus priority; far-future due dates are neutral rather than penalized.
	if task.DueDate != nil {
		daysUntil := util.DaysBetween(now, *task.DueDate)
		switch {
		case daysUntil < 0:
			score += (-daysUntil)*50 + 300
		case daysUntil == 0:
			score += 300
		case daysUntil <= 7:
			score += (8 - daysUntil) * 25
		}
	}

	// In-progress bonus
	if task.Status == models.StatusInProgress {
		score += 500
	}

	// Review bonus
	if task.Status == models.StatusReview {
		score += 250
	}

	// Active flag bonus
	if task.Active {
		score += 200
	}

	// Leaf task bonus: prefer actionable leaf tasks over containers
	if isLeafTask(task) {
		score += 800
	}

	// Container penalty: penalize tasks with incomplete children
	if len(task.Children) > 0 && !allChildrenComplete(task) {
		score -= 1000
	}

	// Task age: older tasks get a small boost to avoid stagnation
	// 30 points per completed week, capped at 10 weeks (max 300 points).
	// Negative daysSinceCreated (clock skew / future-dated import) clamps to
	// 0 so the age path never deducts from the score.
	daysSinceCreated := max(util.DaysBetween(task.CreatedAt, now), 0)
	ageBonus := min(daysSinceCreated/7, 10)
	score += ageBonus * 30

	// Recent momentum: boost tasks with recent activity. A negative value
	// means the task was updated in the future (clock skew); treat as today.
	daysSinceUpdated := util.DaysBetween(task.UpdatedAt, now)
	switch {
	case daysSinceUpdated <= 0:
		score += 400
	case daysSinceUpdated <= 3:
		score += 200
	}

	// Unblocking bonus: prefer tasks that unblock others. Guard nil project
	// since callers may compute scores for cross-project FocusTasks where the
	// project pointer was not resolved. Completed dependents are excluded
	// because they do not need unblocking — only open dependents count.
	blockedByThisCount := countIncompleteDependents(task, project)
	unblockingBonus := min(blockedByThisCount, 3)
	score += unblockingBonus * 150

	// Section scoring: Next is boosted, Waiting/Review/Standby/Someday penalized
	switch task.Section {
	case models.SectionNext:
		score += 100
	case models.SectionWaiting:
		score -= 100
	case models.SectionReview:
		score -= 100
	case models.SectionStandby:
		score -= 200
	case models.SectionSomeday:
		score -= 300
	}

	// Composite scoring profile integration
	if cfg != nil && project != nil {
		if profile := cfg.GetProjectScoringProfile(project.ID); profile != nil {
			pct := scoring.NormalizedScore(task, profile)
			score += int(pct)
		}
	}

	return score
}

// getSuggestionReason generates a human-readable explanation for why a task is suggested.
// Returns an empty string when task is nil. Nil project is tolerated and disables
// dependency- and profile-based reasons.
func getSuggestionReason(task *models.Task, project *models.Project, cfg *config.Config, now time.Time) string {
	if task == nil {
		return ""
	}
	var reasons []string
	today := util.StartOfDay(now)

	switch task.Priority {
	case 1:
		reasons = append(reasons, "critical priority")
	case 2:
		reasons = append(reasons, "high priority")
	}

	if task.DueDate != nil {
		dueDay := util.StartOfDay(*task.DueDate)

		if dueDay.Before(today) {
			reasons = append(reasons, "overdue")
		} else if dueDay.Equal(today) {
			reasons = append(reasons, "due today")
		} else if !dueDay.After(today.AddDate(0, 0, 7)) {
			reasons = append(reasons, "due this week")
		}
	}

	if task.Status == models.StatusInProgress {
		reasons = append(reasons, "in progress")
	}

	if task.Active {
		reasons = append(reasons, "flagged active")
	}

	blockedByThisCount := countIncompleteDependents(task, project)
	if blockedByThisCount > 0 {
		if blockedByThisCount == 1 {
			reasons = append(reasons, "unblocks 1 task")
		} else {
			reasons = append(reasons, fmt.Sprintf("unblocks %d tasks", blockedByThisCount))
		}
	}

	daysSinceUpdated := util.DaysBetween(task.UpdatedAt, now)
	if daysSinceUpdated <= 3 && task.Status != models.StatusInProgress {
		reasons = append(reasons, "recent activity")
	}

	if cfg != nil && project != nil {
		if profile := cfg.GetProjectScoringProfile(project.ID); profile != nil {
			pct := scoring.NormalizedScore(task, profile)
			maxScore := scoring.MaxNormalizedScore(profile)
			if int(pct) > 0 {
				reasons = append(reasons, fmt.Sprintf("composite score %d/%d", int(pct), maxScore))
			}
		}
	}

	if len(reasons) == 0 {
		return "Ready to work on"
	}

	var result strings.Builder
	for i, r := range reasons {
		if i > 0 {
			result.WriteString(" + ")
		}
		if i == 0 && r != "" {
			// Guard against empty reason to avoid r[:1] panic. Use rune-aware
			// slicing so a leading multi-byte rune is not split.
			runes := []rune(r)
			result.WriteString(strings.ToUpper(string(runes[:1])) + string(runes[1:]))
		} else {
			result.WriteString(r)
		}
	}
	return result.String()
}

// RankCrossProjectTasks scores all tasks, applies cross-project balance penalty,
// sorts descending, and excludes dismissed tasks. FocusTasks with a nil Task
// or nil Project are skipped so callers can pass loosely-built slices.
func RankCrossProjectTasks(tasks []FocusTask, dismissedIDs map[string]bool, cfg *config.Config) []ScoredFocusTask {
	now := time.Now()
	var scored []ScoredFocusTask
	for _, ft := range tasks {
		if ft.Task == nil || ft.Project == nil {
			continue
		}
		if dismissedIDs[ft.Task.ID] {
			continue
		}
		s := computeCrossProjectScore(ft.Task, ft.Project, cfg, now)
		r := getSuggestionReason(ft.Task, ft.Project, cfg, now)
		scored = append(scored, ScoredFocusTask{
			FocusTask: ft,
			Score:     s,
			Reason:    r,
		})
	}

	// Sort descending by score; ties break on task ID for deterministic order
	// across iterations (callers feed in slices built from map iteration whose
	// order is randomized by Go).
	sortByScoreThenID := func(s []ScoredFocusTask) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].Score != s[j].Score {
				return s[i].Score > s[j].Score
			}
			return s[i].Task.ID < s[j].Task.ID
		})
	}
	sortByScoreThenID(scored)

	// Apply cross-project balance penalty: tasks from over-represented projects
	// get penalized so the top results interleave projects.
	projectCount := make(map[string]int)
	for i := range scored {
		pid := scored[i].Project.ID
		scored[i].Score -= projectCount[pid] * 300
		projectCount[pid]++
	}

	// Re-sort after balance penalty
	sortByScoreThenID(scored)

	return scored
}

// countIncompleteDependents returns the number of open (non-complete)
// tasks that depend on the given task. A completed dependent does not
// need this task to unblock it, so it must not contribute to the
// "unblocks N tasks" reasoning or the score bonus.
func countIncompleteDependents(task *models.Task, project *models.Project) int {
	if task == nil || project == nil {
		return 0
	}
	count := 0
	for _, dependent := range project.Tasks {
		if dependent == nil {
			continue
		}
		if !dependent.HasDependency(task.ID) {
			continue
		}
		if dependent.Status.IsComplete() {
			continue
		}
		count++
	}
	return count
}

func hasIncompleteDependencies(task *models.Task, project *models.Project, projects []*models.Project) bool {
	if task == nil || project == nil {
		return false
	}
	for _, depID := range task.DependsOn {
		ref := models.ParseTaskRef(depID)
		if ref.IsCrossProject() {
			dep := getTaskByRef(projects, ref)
			if dep == nil || !dep.IsComplete() {
				return true
			}
			continue
		}
		dep := getTaskByID(project, depID)
		if dep == nil || !dep.IsComplete() {
			return true
		}
	}
	return false
}

func getTaskByRef(projects []*models.Project, ref models.TaskRef) *models.Task {
	if ref.IsEmpty() {
		return nil
	}
	for _, project := range projects {
		if project == nil || project.ID != ref.ProjectID {
			continue
		}
		return getTaskByID(project, ref.TaskID)
	}
	return nil
}

func getTaskByID(project *models.Project, id string) *models.Task {
	if project == nil || id == "" {
		return nil
	}
	for _, task := range project.Tasks {
		if task != nil && task.ID == id {
			return task
		}
	}
	return nil
}

// allChildrenComplete returns true if all children of the task are complete.
// Nil tasks and nil children are treated as complete (they cannot represent
// open work).
func allChildrenComplete(task *models.Task) bool {
	if task == nil {
		return true
	}
	for _, child := range task.Children {
		if child == nil {
			continue
		}
		if !child.Status.IsComplete() {
			return false
		}
	}
	return true
}

func isLeafTask(task *models.Task) bool {
	if task == nil {
		return false
	}
	for _, child := range task.Children {
		if child != nil {
			return false
		}
	}
	return true
}
