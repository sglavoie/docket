package tui

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// ParsedSearchQuery holds structured filters extracted from a search query.
type ParsedSearchQuery struct {
	TextQuery     string              // Remaining text after extracting filters
	Priorities    []int               // Empty = all priorities
	Tags          []string            // AND logic - task must have all tags
	Statuses      []models.TaskStatus // Empty = all statuses
	Types         []models.TaskType   // Empty = all types
	DueDateFilter string              // "today", "tomorrow", "week", "overdue", "none", ""
	Section       string              // Empty = all sections
	ActiveOnly    bool                // Only show active tasks
	HasPresence   []string            // has:X filters (normalized names)
	NoPresence    []string            // no:X filters (negation of has:)
	IsState       []string            // is:X filters
	ProjectSlug   string              // p:slug project filter
}

// HasFilters returns true if any filters are active.
func (p *ParsedSearchQuery) HasFilters() bool {
	return len(p.Priorities) > 0 ||
		len(p.Tags) > 0 ||
		len(p.Statuses) > 0 ||
		len(p.Types) > 0 ||
		p.DueDateFilter != "" ||
		p.Section != "" ||
		p.ActiveOnly ||
		len(p.HasPresence) > 0 ||
		len(p.NoPresence) > 0 ||
		len(p.IsState) > 0 ||
		p.ProjectSlug != ""
}

// HasTaskOnlyFilters returns true if any task-specific filters are active
// that have no meaning for notes (which lack these properties).
func (p *ParsedSearchQuery) HasTaskOnlyFilters() bool {
	return len(p.Priorities) > 0 ||
		len(p.Statuses) > 0 ||
		len(p.Types) > 0 ||
		p.DueDateFilter != "" ||
		p.Section != "" ||
		p.ActiveOnly
}

// ParseSearchQuery extracts structured filters from a search query string.
// Syntax mirrors quick capture:
//   - !N, !N-M, or !alias for priority (1-5, range, or word/abbreviation)
//   - #tag for tags
//   - @today, @tomorrow, @week, @overdue, @none for due date filters
//   - $status for status (todo, in_progress, review, standby, maybe, done, blocked, canceled)
//   - ^type for task type (epic, story, task, bug, idea, note)
//   - /section for section filter
//   - * for active tasks only
func ParseSearchQuery(input string) *ParsedSearchQuery {
	result := &ParsedSearchQuery{}

	// Extract priority: !N, !N-M (range), or word/abbreviation aliases
	priorityRangeRe := regexp.MustCompile(`\s*!([1-5])-([1-5])\s*`)
	priorityTokenRe := regexp.MustCompile(`(?i)\s*!(critical|urgent|high|normal|medium|lowest|low|cr|ur|hi|no|med|me|lo|c|u|h|n|m|l|[1-5])\s*`)

	// Handle range first
	for _, match := range priorityRangeRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 3 {
			start, _ := strconv.Atoi(match[1])
			end, _ := strconv.Atoi(match[2])
			if start > end {
				start, end = end, start
			}
			for p := start; p <= end; p++ {
				if !containsInt(result.Priorities, p) {
					result.Priorities = append(result.Priorities, p)
				}
			}
		}
	}
	input = priorityRangeRe.ReplaceAllString(input, " ")

	// Then single priorities (numeric or abbreviation)
	for _, match := range priorityTokenRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			p := parsePriorityToken(match[1])
			if p >= 1 && p <= 5 && !containsInt(result.Priorities, p) {
				result.Priorities = append(result.Priorities, p)
			}
		}
	}
	input = priorityTokenRe.ReplaceAllString(input, " ")

	// Extract tags: #word
	tagRe := regexp.MustCompile(`\s*#([\w][\w.-]*)\s*`)
	for _, match := range tagRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			result.Tags = append(result.Tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date filter: @keyword
	dueDateRe := regexp.MustCompile(`(?i)\s*@(today|tomorrow|week|overdue|none)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) == 2 {
		result.DueDateFilter = strings.ToLower(match[1])
	}
	input = dueDateRe.ReplaceAllString(input, " ")

	// Extract status: $status
	statusRe := regexp.MustCompile(`(?i)\s*\$(todo|in_progress|review|standby|maybe|done|blocked|canceled)\s*`)
	for _, match := range statusRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			if status, ok := models.ParseTaskStatus(strings.ToLower(match[1])); ok {
				result.Statuses = append(result.Statuses, status)
			}
		}
	}
	input = statusRe.ReplaceAllString(input, " ")

	// Extract type: ^type
	typeRe := regexp.MustCompile(`(?i)\s*\^(epic|story|task|bug|idea|note)\s*`)
	for _, match := range typeRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			if taskType, ok := models.ParseTaskTypeInsensitive(match[1]); ok {
				result.Types = append(result.Types, taskType)
			}
		}
	}
	input = typeRe.ReplaceAllString(input, " ")

	// Extract section: /section
	sectionRe := regexp.MustCompile(`\s*/(\S+)\s*`)
	if match := sectionRe.FindStringSubmatch(input); len(match) == 2 {
		result.Section = match[1]
	}
	input = sectionRe.ReplaceAllString(input, " ")

	// Extract has: filters
	hasRe := regexp.MustCompile(`(?i)\bhas:(\w+)`)
	for _, match := range hasRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			result.HasPresence = append(result.HasPresence, normalizeHasFilter(strings.ToLower(match[1])))
		}
	}
	input = hasRe.ReplaceAllString(input, " ")

	// Extract no: filters
	noRe := regexp.MustCompile(`(?i)\bno:(\w+)`)
	for _, match := range noRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			result.NoPresence = append(result.NoPresence, normalizeHasFilter(strings.ToLower(match[1])))
		}
	}
	input = noRe.ReplaceAllString(input, " ")

	// Extract is: filters
	isRe := regexp.MustCompile(`(?i)\bis:(\w+)`)
	for _, match := range isRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			result.IsState = append(result.IsState, strings.ToLower(match[1]))
		}
	}
	input = isRe.ReplaceAllString(input, " ")

	// Extract p: / project: filter
	projectRe := regexp.MustCompile(`(?i)\b(?:project:|p:)(\S+)`)
	if match := projectRe.FindStringSubmatch(input); len(match) == 2 {
		result.ProjectSlug = strings.ToLower(match[1])
	}
	input = projectRe.ReplaceAllString(input, " ")

	// Extract active flag: *
	activeRe := regexp.MustCompile(`\s*\*\s*`)
	if activeRe.MatchString(input) {
		result.ActiveOnly = true
	}
	input = activeRe.ReplaceAllString(input, " ")

	// Clean up remaining text query
	result.TextQuery = strings.TrimSpace(input)
	result.TextQuery = regexp.MustCompile(`\s+`).ReplaceAllString(result.TextQuery, " ")

	return result
}

// TaskMatchesFilters checks if a task matches all the specified filters.
func TaskMatchesFilters(task *models.Task, filters *ParsedSearchQuery) bool {
	if filters == nil {
		return true
	}

	// Priority filter
	if len(filters.Priorities) > 0 {
		if !containsInt(filters.Priorities, task.Priority) {
			return false
		}
	}

	// Tag filter (AND logic - must have all specified tags)
	if len(filters.Tags) > 0 {
		for _, tag := range filters.Tags {
			if !task.HasTag(tag) {
				return false
			}
		}
	}

	// Status filter
	if len(filters.Statuses) > 0 {
		found := slices.Contains(filters.Statuses, task.Status)
		if !found {
			return false
		}
	}

	// Type filter
	if len(filters.Types) > 0 {
		found := slices.Contains(filters.Types, task.Type)
		if !found {
			return false
		}
	}

	// Due date filter
	if filters.DueDateFilter != "" {
		if !matchesDueDateFilter(task, filters.DueDateFilter) {
			return false
		}
	}

	// Section filter (case-insensitive)
	if filters.Section != "" {
		if !strings.EqualFold(task.Section, filters.Section) {
			return false
		}
	}

	// Active flag filter
	if filters.ActiveOnly && !task.Active {
		return false
	}

	// has: filters (must have the property)
	for _, f := range filters.HasPresence {
		if !matchesHasFilter(task, f) {
			return false
		}
	}

	// no: filters (must NOT have the property)
	for _, f := range filters.NoPresence {
		if matchesHasFilter(task, f) {
			return false
		}
	}

	// is: filters (must match the state)
	for _, f := range filters.IsState {
		if !matchesIsFilter(task, f) {
			return false
		}
	}

	return true
}

// matchesDueDateFilter checks if a task matches the due date filter.
func matchesDueDateFilter(task *models.Task, filter string) bool {
	now := time.Now()
	today := util.StartOfDay(now)

	switch filter {
	case "none":
		return task.DueDate == nil
	case "today":
		if task.DueDate == nil {
			return false
		}
		dueDay := util.StartOfDay(*task.DueDate)
		return dueDay.Equal(today)
	case "tomorrow":
		if task.DueDate == nil {
			return false
		}
		tomorrow := today.AddDate(0, 0, 1)
		dueDay := util.StartOfDay(*task.DueDate)
		return dueDay.Equal(tomorrow)
	case "week":
		if task.DueDate == nil {
			return false
		}
		endOfWeek := today.AddDate(0, 0, 7)
		dueDay := util.StartOfDay(*task.DueDate)
		return !dueDay.Before(today) && dueDay.Before(endOfWeek)
	case "overdue":
		if task.DueDate == nil {
			return false
		}
		// Only active (non-complete) tasks can be overdue
		if task.Status.IsComplete() {
			return false
		}
		dueDay := util.StartOfDay(*task.DueDate)
		return dueDay.Before(today)
	}
	return true
}

// containsInt checks if a slice contains an integer.
func containsInt(slice []int, val int) bool {
	return slices.Contains(slice, val)
}

// normalizeHasFilter maps aliases to canonical filter names.
func normalizeHasFilter(s string) string {
	switch s {
	case "desc":
		return "description"
	case "deps":
		return "dependency"
	case "subtasks":
		return "children"
	}
	return s
}

// matchesHasFilter returns true if the task has the specified property.
func matchesHasFilter(task *models.Task, filter string) bool {
	switch filter {
	case "description":
		return len(task.Description) > 0
	case "comment":
		return len(task.Comments) > 0
	case "checklist":
		return len(task.Checklist) > 0
	case "attachment":
		return len(task.Attachments) > 0
	case "issue":
		return task.HasGitHubIssue()
	case "due":
		return task.DueDate != nil
	case "dependency":
		return len(task.DependsOn) > 0
	case "relation":
		return len(task.RelatedTo) > 0
	case "recurrence":
		return task.Recurrence != nil
	case "url":
		return task.URL != ""
	case "stage":
		return task.Stage != ""
	case "parent":
		return task.ParentID != nil
	case "children":
		return len(task.Children) > 0
	case "tag":
		return len(task.Tags) > 0
	}
	return false
}

// matchesIsFilter returns true if the task matches the specified state.
func matchesIsFilter(task *models.Task, filter string) bool {
	switch filter {
	case "note":
		return false
	case "pinned":
		return task.Pinned
	case "active":
		return task.Active
	case "dimmed":
		return task.Dimmed
	case "blocked":
		return task.Status == models.StatusBlocked
	case "overdue":
		return task.IsOverdue()
	case "snoozed":
		return task.IsSnoozed()
	case "recurring":
		return task.Recurrence != nil
	case "leaf":
		return len(task.Children) == 0
	case "root":
		return task.ParentID == nil
	}
	return false
}
