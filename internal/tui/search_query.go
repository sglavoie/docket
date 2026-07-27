package tui

import (
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/util"
)

// ParsedSearchQuery holds structured filters extracted from a search query.
type ParsedSearchQuery struct {
	TextQuery     string   // Remaining text after extracting filters
	Priorities    []int    // Empty = all priorities
	Tags          []string // AND logic - task must have all tags
	States        []models.WorkflowState
	Types         []models.TaskType // Empty = all types
	DueDateFilter string            // "today", "tomorrow", "week", "overdue", "none", ""
	HasPresence   []string          // has:X filters (normalized names)
	NoPresence    []string          // no:X filters (negation of has:)
	IsState       []string          // is:X filters
	ProjectSlug   string            // p:slug project filter
}

// HasFilters returns true if any filters are active.
func (p *ParsedSearchQuery) HasFilters() bool {
	return len(p.Priorities) > 0 ||
		len(p.Tags) > 0 ||
		len(p.States) > 0 ||
		len(p.Types) > 0 ||
		p.DueDateFilter != "" ||
		len(p.HasPresence) > 0 ||
		len(p.NoPresence) > 0 ||
		len(p.IsState) > 0 ||
		p.ProjectSlug != ""
}

// HasTaskOnlyFilters returns true if any task-specific filters are active
// that have no meaning for notes (which lack these properties).
func (p *ParsedSearchQuery) HasTaskOnlyFilters() bool {
	return len(p.Priorities) > 0 ||
		len(p.States) > 0 ||
		len(p.Types) > 0 ||
		p.DueDateFilter != ""
}

// ParseSearchQuery extracts structured filters through the shared text grammar.
// Unknown search tokens remain free text instead of silently becoming filters.
func ParseSearchQuery(input string) *ParsedSearchQuery {
	result := &ParsedSearchQuery{}
	parsed := textgrammar.Parse(input, textgrammar.SurfaceSearch)
	rejected := make([]string, 0)

	for _, token := range parsed.TokensFor(textgrammar.FieldPriority) {
		values, ok := textgrammar.ParsePriority(token.Value, true)
		if !ok {
			rejected = append(rejected, token.Raw)
			continue
		}
		for _, priority := range values {
			if !containsInt(result.Priorities, priority) {
				result.Priorities = append(result.Priorities, priority)
			}
		}
	}
	for _, token := range parsed.TokensFor(textgrammar.FieldTag) {
		result.Tags = append(result.Tags, token.Value)
	}
	if tokens := parsed.TokensFor(textgrammar.FieldDue); len(tokens) > 0 {
		result.DueDateFilter = strings.ToLower(tokens[0].Value)
	}
	for _, token := range parsed.TokensFor(textgrammar.FieldState) {
		match, ok := textgrammar.ParseState(textgrammar.SurfaceSearch, token.Value)
		if !ok {
			rejected = append(rejected, token.Raw)
			continue
		}
		for _, state := range match.States {
			if !slices.Contains(result.States, state) {
				result.States = append(result.States, state)
			}
		}
	}
	for _, token := range parsed.TokensFor(textgrammar.FieldTaskType) {
		taskType, ok := models.ParseTaskTypeInsensitive(token.Value)
		if ok {
			result.Types = append(result.Types, taskType)
		} else if !strings.EqualFold(token.Value, "note") {
			rejected = append(rejected, token.Raw)
		}
	}
	for _, token := range parsed.TokensFor(textgrammar.FieldHas) {
		result.HasPresence = append(result.HasPresence, normalizeHasFilter(strings.ToLower(token.Value)))
	}
	for _, token := range parsed.TokensFor(textgrammar.FieldNo) {
		result.NoPresence = append(result.NoPresence, normalizeHasFilter(strings.ToLower(token.Value)))
	}
	for _, token := range parsed.TokensFor(textgrammar.FieldIs) {
		value := strings.ToLower(token.Value)
		if textgrammar.ContainsDefinitionValue(textgrammar.SurfaceSearch, textgrammar.FieldIs, value) {
			result.IsState = append(result.IsState, value)
		} else {
			rejected = append(rejected, token.Raw)
		}
	}
	if tokens := parsed.TokensFor(textgrammar.FieldProject); len(tokens) > 0 {
		result.ProjectSlug = strings.ToLower(tokens[0].Value)
	}

	result.TextQuery = textgrammar.NormalizeText(strings.Join(append([]string{parsed.Text}, rejected...), " "))

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

	if len(filters.States) > 0 {
		if !slices.Contains(filters.States, task.State()) {
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
		if task.IsTerminalState() {
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
	case "dimmed":
		return task.Dimmed
	case "blocked":
		return task.IsInState(models.StateWaiting)
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
