package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Autocomplete mode constants
const (
	acModeNone   = iota // No autocomplete active
	acModeTag           // Completing a #tag
	acModeParent        // Completing a >parent
)

// suggestion represents an autocomplete suggestion
type suggestion struct {
	display string // Text shown in dropdown
	value   string // Text to insert
}

// QuickCaptureModel handles rapid task creation with inline metadata parsing.
type QuickCaptureModel struct {
	project *models.Project
	input   textarea.Model
	err     error
	width   int
	height  int

	// Autocomplete state
	acMode       int            // Current autocomplete mode
	suggestions  []suggestion   // Current suggestions
	acCursor     int            // Selected suggestion index
	acTriggerPos int            // Position of trigger character in input
	tagCache     []string       // Cached project tags
	parentCache  []*models.Task // Cached valid parent tasks

	// Live preview
	preview    *quickCaptureResult // Parsed preview (may be partial)
	previewErr string              // Preview validation error
}

// quickCaptureResult holds parsed metadata from quick capture input.
type quickCaptureResult struct {
	title      string
	taskType   models.TaskType
	priority   int
	tags       []string
	dueDate    *time.Time
	parentID   *string
	recurrence *models.RecurrencePattern
	section    string // Target section (empty = Inbox)
}

// NewQuickCaptureModel creates a new quick capture model.
func NewQuickCaptureModel(project *models.Project) *QuickCaptureModel {
	input := textarea.New()
	input.Placeholder = "^type Task title !priority #tag @due ~recur >parent /section"
	input.CharLimit = 200
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(lineIdx int) string { return "" })
	input.Focus()

	m := &QuickCaptureModel{
		project:     project,
		input:       input,
		width:       80, // sensible default until SetSize is called
		acMode:      acModeNone,
		suggestions: make([]suggestion, 0),
	}

	// Initialize caches
	m.initCaches()

	return m
}

// initCaches builds tag and parent caches for autocomplete.
func (m *QuickCaptureModel) initCaches() {
	// Cache all tags from the project
	tagSet := make(map[string]bool)
	for _, task := range m.project.Tasks {
		for _, tag := range task.Tags {
			tagSet[tag] = true
		}
	}
	m.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		m.tagCache = append(m.tagCache, tag)
	}
	sort.Strings(m.tagCache)

	// Cache potential parent tasks (non-completed, can have children)
	m.parentCache = make([]*models.Task, 0)
	for _, task := range m.project.Tasks {
		if !task.Status.IsComplete() && task.Type.CanHaveChildren() {
			m.parentCache = append(m.parentCache, task)
		}
	}
}

// SetSize sets the view dimensions.
func (m *QuickCaptureModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Use same resize logic as task_creator
	formFactory.ResizeTextarea(&m.input, width)
}

func (m *QuickCaptureModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *QuickCaptureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle autocomplete navigation when active
		if m.acMode != acModeNone && len(m.suggestions) > 0 {
			switch key {
			case "tab", "enter":
				m.acceptSuggestion()
				m.updatePreview()
				return m, nil
			case "esc":
				// Dismiss autocomplete
				m.acMode = acModeNone
				m.suggestions = nil
				m.acCursor = 0
				return m, nil
			case "up", "ctrl+p":
				if m.acCursor > 0 {
					m.acCursor--
				}
				return m, nil
			case "down", "ctrl+n":
				if m.acCursor < len(m.suggestions)-1 {
					m.acCursor++
				}
				return m, nil
			}
		}

		// Regular key handling
		switch key {
		case "esc":
			return m, func() tea.Msg { return CloseQuickCaptureMsg{} }

		case "enter":
			// Only create task if not in autocomplete mode
			if m.acMode == acModeNone {
				return m, m.createTask()
			}
		}
	}

	// Update textarea
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Update preview and detect autocomplete triggers
	m.updatePreview()
	mode, pos, partial := m.detectAutocompleteTrigger()
	if mode != acModeNone {
		m.acMode = mode
		m.acTriggerPos = pos
		m.acCursor = 0
		switch mode {
		case acModeTag:
			m.suggestions = m.getTagSuggestions(partial)
		case acModeParent:
			m.suggestions = m.getParentSuggestions(partial)
		}
		// If no suggestions, exit autocomplete mode
		if len(m.suggestions) == 0 {
			m.acMode = acModeNone
		}
	} else {
		m.acMode = acModeNone
		m.suggestions = nil
	}

	return m, cmd
}

func (m *QuickCaptureModel) View() string {
	var b strings.Builder
	b.WriteString(m.input.View())
	b.WriteString("\n")

	// Show autocomplete suggestions if active
	if m.acMode != acModeNone && len(m.suggestions) > 0 {
		b.WriteString("\n")
		for i, s := range m.suggestions {
			if i == m.acCursor {
				b.WriteString(SelectedStyle.Render("> " + s.display))
			} else {
				b.WriteString(MutedStyle.Render("  " + s.display))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Show live preview or error
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	} else if preview := m.renderPreview(); preview != "" {
		b.WriteString(preview)
		b.WriteString("\n\n")
	}

	// Help text
	b.WriteString(MutedStyle.Render("^type · !N · #tag · @date · ~recur · >parent · /section"))
	b.WriteString("\n")
	if m.acMode != acModeNone {
		b.WriteString(MutedStyle.Render("Tab: accept · ↑↓: navigate · Esc: dismiss"))
	} else {
		b.WriteString(MutedStyle.Render("Enter: create · Esc: cancel"))
	}

	// Same card width calculation as task_creator
	cardWidth := m.width - 4
	if cardWidth > 80 {
		cardWidth = 80
	}
	config := OverlayCardConfig{
		Title:        "Quick Capture",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}

// parseQuickCapture extracts metadata from the input string.
// Syntax: "^bug Task title !1 #backend #urgent @tomorrow >3"
func parseQuickCapture(input string, project *models.Project) (*quickCaptureResult, error) {
	result := &quickCaptureResult{
		taskType: models.TaskTypeTask, // default type
		priority: 3,                   // default priority
	}

	// Extract task type: ^epic, ^story, ^task, ^bug, ^idea, ^note (case-insensitive)
	typeRe := regexp.MustCompile(`(?i)\s*\^(epic|story|task|bug|idea|note)\s*`)
	if match := typeRe.FindStringSubmatch(input); len(match) > 1 {
		taskType, ok := models.ParseTaskTypeInsensitive(match[1])
		if !ok {
			return nil, fmt.Errorf("invalid task type: %s", match[1])
		}
		result.taskType = taskType
		input = typeRe.ReplaceAllString(input, " ")
	}

	// Extract priority: !1 through !5
	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags: #word
	tagRe := regexp.MustCompile(`\s*#(\w+)\s*`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			result.tags = append(result.tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date: @value
	dueDateRe := regexp.MustCompile(`\s*@(\S+)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
		date, err := ParseDate(match[1])
		if err != nil {
			return nil, fmt.Errorf("invalid date: %s", match[1])
		}
		result.dueDate = &date
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Extract recurrence: ~pattern (e.g., ~daily, ~weekly, ~2w)
	recurRe := regexp.MustCompile(`\s*~(\S+)\s*`)
	if match := recurRe.FindStringSubmatch(input); len(match) > 1 {
		pattern := models.ParseRecurrence(match[1])
		if pattern == nil {
			return nil, fmt.Errorf("invalid recurrence: %s", match[1])
		}
		result.recurrence = pattern
		input = recurRe.ReplaceAllString(input, " ")
	}

	// Extract parent: >ref (short ID or title prefix)
	parentRe := regexp.MustCompile(`\s*>(\S+)\s*`)
	if match := parentRe.FindStringSubmatch(input); len(match) > 1 {
		ref := match[1]
		parent := resolveParent(ref, project)
		if parent == nil {
			return nil, fmt.Errorf("parent not found: %s", ref)
		}
		if !result.taskType.CanBeChildOf(parent.Type) {
			return nil, fmt.Errorf("%s cannot be child of %s", result.taskType.HeadingPrefix(), parent.Type.HeadingPrefix())
		}
		result.parentID = &parent.ID
		input = parentRe.ReplaceAllString(input, " ")
	}

	// Extract section: /SectionName (case-insensitive matching)
	sectionRe := regexp.MustCompile(`\s*/(\S+)\s*`)
	if match := sectionRe.FindStringSubmatch(input); len(match) > 1 {
		sectionName := match[1]
		// Find matching section (case-insensitive)
		found := false
		for _, s := range project.GetSections() {
			if strings.EqualFold(s, sectionName) {
				result.section = s
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("section not found: %s", sectionName)
		}
		input = sectionRe.ReplaceAllString(input, " ")
	}

	// Clean up title
	result.title = strings.TrimSpace(input)
	result.title = regexp.MustCompile(`\s+`).ReplaceAllString(result.title, " ")

	if result.title == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	return result, nil
}

// resolveParent finds a task by short ID or title prefix.
func resolveParent(ref string, project *models.Project) *models.Task {
	// Try numeric short ID first
	if shortID, err := strconv.Atoi(ref); err == nil {
		for _, task := range project.Tasks {
			if task.ShortID == shortID {
				return task
			}
		}
	}

	// Fall back to case-insensitive title prefix match
	refLower := strings.ToLower(ref)
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), refLower) {
			return task
		}
	}

	return nil
}

// parseQuickCaptureForPreview parses input for live preview without failing early.
// Returns partial results and any validation error string.
func parseQuickCaptureForPreview(input string, project *models.Project) (*quickCaptureResult, string) {
	result := &quickCaptureResult{
		taskType: models.TaskTypeTask,
		priority: 3,
	}
	var errMsg string

	// Extract task type
	typeRe := regexp.MustCompile(`(?i)\s*\^(epic|story|task|bug|idea|note)\s*`)
	if match := typeRe.FindStringSubmatch(input); len(match) > 1 {
		if taskType, ok := models.ParseTaskTypeInsensitive(match[1]); ok {
			result.taskType = taskType
		}
		input = typeRe.ReplaceAllString(input, " ")
	}

	// Extract priority
	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags
	tagRe := regexp.MustCompile(`\s*#(\w+)\s*`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			result.tags = append(result.tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date (capture error but don't stop)
	dueDateRe := regexp.MustCompile(`\s*@(\S+)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
		date, err := ParseDate(match[1])
		if err != nil {
			errMsg = fmt.Sprintf("invalid date: %s", match[1])
		} else {
			result.dueDate = &date
		}
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Extract recurrence
	recurRe := regexp.MustCompile(`\s*~(\S+)\s*`)
	if match := recurRe.FindStringSubmatch(input); len(match) > 1 {
		pattern := models.ParseRecurrence(match[1])
		if pattern == nil && errMsg == "" {
			errMsg = fmt.Sprintf("invalid recurrence: %s", match[1])
		} else if pattern != nil {
			result.recurrence = pattern
		}
		input = recurRe.ReplaceAllString(input, " ")
	}

	// Extract parent
	parentRe := regexp.MustCompile(`\s*>(\S+)\s*`)
	if match := parentRe.FindStringSubmatch(input); len(match) > 1 {
		ref := match[1]
		parent := resolveParent(ref, project)
		if parent == nil && errMsg == "" {
			errMsg = fmt.Sprintf("parent not found: %s", ref)
		} else if parent != nil {
			if !result.taskType.CanBeChildOf(parent.Type) && errMsg == "" {
				errMsg = fmt.Sprintf("%s cannot be child of %s", result.taskType.HeadingPrefix(), parent.Type.HeadingPrefix())
			} else {
				result.parentID = &parent.ID
			}
		}
		input = parentRe.ReplaceAllString(input, " ")
	}

	// Extract section
	sectionRe := regexp.MustCompile(`\s*/(\S+)\s*`)
	if match := sectionRe.FindStringSubmatch(input); len(match) > 1 {
		sectionName := match[1]
		found := false
		for _, s := range project.GetSections() {
			if strings.EqualFold(s, sectionName) {
				result.section = s
				found = true
				break
			}
		}
		if !found && errMsg == "" {
			errMsg = fmt.Sprintf("section not found: %s", sectionName)
		}
		input = sectionRe.ReplaceAllString(input, " ")
	}

	// Clean up title
	result.title = strings.TrimSpace(input)
	result.title = regexp.MustCompile(`\s+`).ReplaceAllString(result.title, " ")

	return result, errMsg
}

// detectAutocompleteTrigger scans the input for autocomplete triggers.
// Returns the trigger type, trigger position, and partial text after trigger.
func (m *QuickCaptureModel) detectAutocompleteTrigger() (int, int, string) {
	input := m.input.Value()
	if len(input) == 0 {
		return acModeNone, 0, ""
	}

	// Scan backward from end to find most recent trigger
	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == ' ' || ch == '\n' {
			// Hit whitespace before finding trigger
			break
		}
		if ch == '#' {
			partial := input[i+1:]
			return acModeTag, i, partial
		}
		if ch == '>' {
			partial := input[i+1:]
			return acModeParent, i, partial
		}
	}
	return acModeNone, 0, ""
}

// getTagSuggestions returns matching tags for the partial query.
func (m *QuickCaptureModel) getTagSuggestions(partial string) []suggestion {
	type scoredTag struct {
		tag   string
		score int
	}

	var scored []scoredTag
	for _, tag := range m.tagCache {
		score := fuzzy.Score(tag, partial)
		if score > 0 {
			scored = append(scored, scoredTag{tag: tag, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top 5
	suggestions := make([]suggestion, 0, 5)
	for i := 0; i < len(scored) && i < 5; i++ {
		suggestions = append(suggestions, suggestion{
			display: "#" + scored[i].tag,
			value:   scored[i].tag,
		})
	}
	return suggestions
}

// getParentSuggestions returns matching parent tasks for the partial query.
func (m *QuickCaptureModel) getParentSuggestions(partial string) []suggestion {
	type scoredParent struct {
		task  *models.Task
		score int
	}

	var scored []scoredParent
	for _, task := range m.parentCache {
		// Build searchable text: "#ShortID Type Title"
		searchText := fmt.Sprintf("#%d %s %s", task.ShortID, task.Type.HeadingPrefix(), task.Title)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredParent{task: task, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top 5
	suggestions := make([]suggestion, 0, 5)
	for i := 0; i < len(scored) && i < 5; i++ {
		task := scored[i].task
		// Truncate title if too long
		title := task.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		suggestions = append(suggestions, suggestion{
			display: fmt.Sprintf("%s #%d %s", task.Type.Icon(), task.ShortID, title),
			value:   strconv.Itoa(task.ShortID),
		})
	}
	return suggestions
}

// acceptSuggestion inserts the selected suggestion into the input.
func (m *QuickCaptureModel) acceptSuggestion() {
	if m.acMode == acModeNone || len(m.suggestions) == 0 {
		return
	}

	selected := m.suggestions[m.acCursor]
	input := m.input.Value()

	// Replace from trigger position to end with the completed value
	var newInput string
	switch m.acMode {
	case acModeTag:
		newInput = input[:m.acTriggerPos] + "#" + selected.value + " "
	case acModeParent:
		newInput = input[:m.acTriggerPos] + ">" + selected.value + " "
	}

	m.input.SetValue(newInput)
	// Move cursor to end
	m.input.CursorEnd()

	// Reset autocomplete state
	m.acMode = acModeNone
	m.suggestions = nil
	m.acCursor = 0
}

// updatePreview updates the live preview state.
func (m *QuickCaptureModel) updatePreview() {
	input := strings.ReplaceAll(m.input.Value(), "\n", " ")
	input = strings.TrimSpace(input)
	if input == "" {
		m.preview = nil
		m.previewErr = ""
		return
	}
	m.preview, m.previewErr = parseQuickCaptureForPreview(input, m.project)
}

// renderPreview formats the parsed preview for display.
func (m *QuickCaptureModel) renderPreview() string {
	if m.previewErr != "" {
		return ValidationErrorStyle.Render(m.previewErr)
	}
	if m.preview == nil || m.preview.title == "" {
		return ""
	}

	var parts []string

	// Type and title
	typeStyle := GetTypeStyle(string(m.preview.taskType))
	parts = append(parts, typeStyle.Render(m.preview.taskType.Icon())+" "+m.preview.title)

	// Priority (only show if not default)
	if m.preview.priority != 3 {
		parts = append(parts, fmt.Sprintf("!%d", m.preview.priority))
	}

	// Tags
	if len(m.preview.tags) > 0 {
		tagStrs := make([]string, len(m.preview.tags))
		for i, t := range m.preview.tags {
			tagStrs[i] = "#" + t
		}
		parts = append(parts, strings.Join(tagStrs, " "))
	}

	// Due date
	if m.preview.dueDate != nil {
		parts = append(parts, "@"+m.preview.dueDate.Format("Jan 2"))
	}

	// Recurrence
	if m.preview.recurrence != nil {
		parts = append(parts, "~"+m.preview.recurrence.String())
	}

	// Section (only show if explicitly set)
	if m.preview.section != "" {
		parts = append(parts, "/"+m.preview.section)
	}

	return MutedStyle.Render(strings.Join(parts, " · "))
}

func (m *QuickCaptureModel) createTask() tea.Cmd {
	return func() tea.Msg {
		// Replace newlines with spaces since this is a single task title
		input := strings.ReplaceAll(m.input.Value(), "\n", " ")
		input = strings.TrimSpace(input)
		if input == "" {
			m.err = fmt.Errorf("task title cannot be empty")
			return nil
		}

		result, err := parseQuickCapture(input, m.project)
		if err != nil {
			m.err = err
			return nil
		}

		// Determine target section
		targetSection := result.section
		if targetSection == "" {
			targetSection = models.SectionInbox
		}

		// Ensure target section exists (Inbox gets red color)
		if !m.project.HasSection(targetSection) {
			m.project.EnsureSection(targetSection)
			if targetSection == models.SectionInbox {
				m.project.SetSectionColor(models.SectionInbox, "red")
			}
		}

		// Create task
		task := &models.Task{
			ID:         uuid.New().String(),
			ShortID:    m.project.GetNextShortID(),
			Type:       result.taskType,
			Title:      result.title,
			Status:     models.StatusTodo,
			Priority:   result.priority,
			Section:    targetSection,
			ParentID:   result.parentID,
			DueDate:    result.dueDate,
			Recurrence: result.recurrence,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		for _, tag := range result.tags {
			task.AddTag(tag)
		}

		// Add to project at the beginning so it's prominently visible
		m.project.Tasks = append([]*models.Task{task}, m.project.Tasks...)
		m.project.BuildTaskTree()
		m.project.ReorderRootTasksByCompletion()

		// Backup and save
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reload to rebuild tree properly
		storage.LoadProject(m.project)

		return TaskCreatedMsg{Task: task}
	}
}
