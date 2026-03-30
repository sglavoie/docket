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
}

// HasFilters returns true if any filters are active.
func (p *ParsedSearchQuery) HasFilters() bool {
	return len(p.Priorities) > 0 ||
		len(p.Tags) > 0 ||
		len(p.Statuses) > 0 ||
		len(p.Types) > 0 ||
		p.DueDateFilter != "" ||
		p.Section != "" ||
		p.ActiveOnly
}

// ParseSearchQuery extracts structured filters from a search query string.
// Syntax mirrors quick capture:
//   - !N or !N-M for priority (1-5 or range)
//   - #tag for tags
//   - @today, @tomorrow, @week, @overdue, @none for due date filters
//   - $status for status (todo, in_progress, review, standby, done, blocked, canceled)
//   - ^type for task type (epic, story, task, bug, idea, note)
//   - /section for section filter
//   - * for active tasks only
func ParseSearchQuery(input string) *ParsedSearchQuery {
	result := &ParsedSearchQuery{}

	// Extract priority: !N or !N-M (range)
	priorityRangeRe := regexp.MustCompile(`\s*!([1-5])-([1-5])\s*`)
	prioritySingleRe := regexp.MustCompile(`\s*!([1-5])\s*`)

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

	// Then single priorities
	for _, match := range prioritySingleRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			p, _ := strconv.Atoi(match[1])
			if !containsInt(result.Priorities, p) {
				result.Priorities = append(result.Priorities, p)
			}
		}
	}
	input = prioritySingleRe.ReplaceAllString(input, " ")

	// Extract tags: #word
	tagRe := regexp.MustCompile(`\s*#([\w][\w.-]*)\s*`)
	for _, match := range tagRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			result.Tags = append(result.Tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date filter: @keyword
	dueDateRe := regexp.MustCompile(`\s*@(today|tomorrow|week|overdue|none)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) == 2 {
		result.DueDateFilter = strings.ToLower(match[1])
	}
	input = dueDateRe.ReplaceAllString(input, " ")

	// Extract status: $status
	statusRe := regexp.MustCompile(`\s*\$(todo|in_progress|review|standby|done|blocked|canceled)\s*`)
	for _, match := range statusRe.FindAllStringSubmatch(input, -1) {
		if len(match) == 2 {
			if status, ok := models.ParseTaskStatus(match[1]); ok {
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

	// Section filter (case-insensitive, checks both GTD Section and CustomSection)
	if filters.Section != "" {
		if !strings.EqualFold(task.Section, filters.Section) && !strings.EqualFold(task.CustomSection, filters.Section) {
			return false
		}
	}

	// Active flag filter
	if filters.ActiveOnly && !task.Active {
		return false
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
