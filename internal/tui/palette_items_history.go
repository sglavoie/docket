package tui

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// recordSearchHistory adds the current search query to history if meaningful.
// Queries are deduplicated, with the most recent occurrence moved to the front.
// Category prefixes alone (e.g., ":task ") are not recorded.
func (m *CommandPaletteModel) recordSearchHistory() {
	if m.configManager == nil {
		return
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		return
	}

	_, searchQuery := m.parseQueryPrefix(query)
	if searchQuery == "" && strings.HasPrefix(query, ":") {
		return
	}

	if strings.HasPrefix(query, "+") {
		return
	}

	if taskIDPattern.MatchString(query) {
		return
	}

	newHistory := []string{query}
	for _, h := range m.searchHistory {
		if h != query {
			newHistory = append(newHistory, h)
		}
	}

	const maxHistorySize = 10
	if len(newHistory) > maxHistorySize {
		newHistory = newHistory[:maxHistorySize]
	}

	m.searchHistory = newHistory
	m.config.SetPaletteSearchHistory(newHistory)
	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}

// parseInlineTask extracts metadata from inline task creation input.
// Syntax: "Task title !priority #tag @due"
// Similar to quick capture but adapted for command palette.
func (m *CommandPaletteModel) parseInlineTask(input string) *inlineTaskPreview {
	result := &inlineTaskPreview{priority: 3}

	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	tagRe := regexp.MustCompile(`\s*#([\w][\w.-]*)\s*`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			result.tags = append(result.tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	dueDateRe := regexp.MustCompile(`\s*@(\S+)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
		dateStr := match[1]
		now := time.Now()
		var dueDate time.Time

		switch strings.ToLower(dateStr) {
		case "today":
			dueDate = util.StartOfDay(now)
		case "tomorrow":
			dueDate = util.StartOfDay(now).AddDate(0, 0, 1)
		default:
			if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
				dueDate = parsed
			}
		}

		if !dueDate.IsZero() {
			result.dueDate = &dueDate
		}
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	result.title = strings.TrimSpace(input)
	return result
}

// createInlineTask creates a task from the inline task preview and closes the palette.
func (m *CommandPaletteModel) createInlineTask() (tea.Model, tea.Cmd) {
	if m.inlineTaskParsed == nil || m.inlineTaskParsed.title == "" {
		return m, nil
	}

	var targetProject *models.Project
	projects := m.config.GetActiveProjects()
	if len(projects) > 0 {
		if current := m.config.GetCurrentProject(); current != nil {
			targetProject = current
		} else {
			targetProject = projects[0]
		}
	}

	if targetProject == nil {
		return m, nil
	}

	if err := storage.LoadProject(targetProject); err != nil {
		return m, nil
	}

	parsed := m.inlineTaskParsed
	return m, func() tea.Msg {
		return CommandPaletteExecuteMsg{
			ResultMsg: InlineTaskCreateMsg{
				Project:  targetProject,
				Title:    parsed.title,
				Priority: parsed.priority,
				Tags:     parsed.tags,
				DueDate:  parsed.dueDate,
			},
		}
	}
}
