package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// Autocomplete mode constants
const (
	acModeNone     = iota // No autocomplete active
	acModeTag             // Completing a #tag
	acModeParent          // Completing a >parent
	acModeType            // Completing a ^type
	acModePriority        // Completing a !priority
	acModeDate            // Completing a @date
	acModeRecur           // Completing a ~recurrence
	acModeSection         // Completing a /section
	acModeStatus          // Completing a $status
	acModeProject         // Completing a %project
)

// acVisibleCount is the number of suggestions visible at once (scrollable window size)
const acVisibleCount = 7

// suggestion represents an autocomplete suggestion
type suggestion struct {
	display string // Text shown in dropdown
	value   string // Text to insert
}

// quickCaptureResult holds parsed metadata from quick capture input.
type quickCaptureResult struct {
	title             string
	taskType          models.TaskType
	priority          int
	tags              []string
	dueDate           *time.Time
	parentID          *string
	recurrence        *models.RecurrencePattern
	section           string            // Target section (empty = Inbox)
	status            models.TaskStatus // Initial status (empty = todo)
	activate          bool              // Whether to activate task (for $start)
	targetProjectID   *string           // Target project ID for cross-project capture
	targetProjectName string            // Target project name (for preview display)
}

// quickCaptureInput encapsulates the autocomplete, preview, and parsing
// infrastructure shared between QuickCaptureModel and LinkedTaskCreatorModel.
type quickCaptureInput struct {
	input textarea.Model
	err   error

	// Autocomplete state
	acMode        int
	suggestions   []suggestion
	acCursor      int
	acViewportTop int
	acTriggerPos  int
	tagCache      []string
	parentCache   []*models.Task
	sectionCache  []string
	projectCache  []*models.Project

	// Live preview
	preview    *quickCaptureResult
	previewErr string

	// Context needed for parsing/preview
	project     *models.Project
	allProjects []*models.Project

	// Layout
	width int // Available width for preview wrapping
}

// newQuickCaptureInput creates a new quickCaptureInput with default textarea settings.
func newQuickCaptureInput(project *models.Project, allProjects []*models.Project, placeholder string) quickCaptureInput {
	input := textarea.New()
	input.Placeholder = placeholder
	input.CharLimit = 200
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(lineIdx int) string { return "" })
	input.Focus()

	qci := quickCaptureInput{
		input:       input,
		project:     project,
		allProjects: allProjects,
		acMode:      acModeNone,
		suggestions: make([]suggestion, 0),
	}
	qci.initCaches()
	return qci
}

// initCaches builds tag, parent, section, and project caches for autocomplete.
func (q *quickCaptureInput) initCaches() {
	// Cache all tags from the project
	tagSet := make(map[string]bool)
	for _, task := range q.project.Tasks {
		for _, tag := range task.Tags {
			tagSet[tag] = true
		}
	}
	q.tagCache = make([]string, 0, len(tagSet))
	for tag := range tagSet {
		q.tagCache = append(q.tagCache, tag)
	}
	sort.Strings(q.tagCache)

	// Cache potential parent tasks (non-completed, can have children)
	q.parentCache = make([]*models.Task, 0)
	for _, task := range q.project.Tasks {
		if !task.Status.IsComplete() && task.Type.CanHaveChildren() {
			q.parentCache = append(q.parentCache, task)
		}
	}

	// Cache sections
	q.sectionCache = q.project.GetSections()

	// Cache non-archived projects for cross-project capture
	q.projectCache = make([]*models.Project, 0)
	if q.allProjects != nil {
		for _, p := range q.allProjects {
			if !p.Archived {
				q.projectCache = append(q.projectCache, p)
			}
		}
	}
}

// resizeTextarea applies standard form resize logic to the textarea.
func (q *quickCaptureInput) resizeTextarea(width int) {
	formFactory.ResizeTextarea(&q.input, width)
}

// handleAutocompleteKey handles keys when autocomplete is active.
// Returns true if the key was consumed.
func (q *quickCaptureInput) handleAutocompleteKey(key string) bool {
	if q.acMode == acModeNone || len(q.suggestions) == 0 {
		return false
	}

	switch key {
	case "tab", "enter":
		q.acceptSuggestion()
		q.updatePreview()
		return true
	case "esc":
		q.acMode = acModeNone
		q.suggestions = nil
		q.acCursor = 0
		q.acViewportTop = 0
		return true
	case "up", "ctrl+p":
		if q.acCursor > 0 {
			q.acCursor--
			if q.acCursor < q.acViewportTop {
				q.acViewportTop = q.acCursor
			}
		}
		return true
	case "down", "ctrl+n":
		if q.acCursor < len(q.suggestions)-1 {
			q.acCursor++
			if q.acCursor >= q.acViewportTop+acVisibleCount {
				q.acViewportTop = q.acCursor - acVisibleCount + 1
			}
		}
		return true
	}
	return false
}

// updateInput updates the textarea and refreshes autocomplete/preview state.
func (q *quickCaptureInput) updateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	q.input, cmd = q.input.Update(msg)

	q.updatePreview()
	q.refreshAutocomplete()

	return cmd
}

// refreshAutocomplete detects triggers and updates suggestion list.
func (q *quickCaptureInput) refreshAutocomplete() {
	mode, pos, partial := q.detectAutocompleteTrigger()
	if mode != acModeNone {
		if mode != q.acMode || pos != q.acTriggerPos {
			q.acCursor = 0
			q.acViewportTop = 0
		}
		q.acMode = mode
		q.acTriggerPos = pos
		switch mode {
		case acModeTag:
			q.suggestions = q.getTagSuggestions(partial)
		case acModeParent:
			q.suggestions = q.getParentSuggestions(partial)
		case acModeType:
			q.suggestions = q.getTypeSuggestions(partial)
		case acModePriority:
			q.suggestions = q.getPrioritySuggestions(partial)
		case acModeDate:
			q.suggestions = q.getDateSuggestions(partial)
		case acModeRecur:
			q.suggestions = q.getRecurrenceSuggestions(partial)
		case acModeSection:
			q.suggestions = q.getSectionSuggestions(partial)
		case acModeStatus:
			q.suggestions = q.getStatusSuggestions(partial)
		case acModeProject:
			q.suggestions = q.getProjectSuggestions(partial)
		}
		if q.acCursor >= len(q.suggestions) {
			q.acCursor = max(0, len(q.suggestions)-1)
		}
		if q.acCursor < q.acViewportTop {
			q.acViewportTop = q.acCursor
		} else if q.acCursor >= q.acViewportTop+acVisibleCount {
			q.acViewportTop = q.acCursor - acVisibleCount + 1
		}
		if len(q.suggestions) == 0 {
			q.acMode = acModeNone
		}
	} else {
		q.acMode = acModeNone
		q.suggestions = nil
	}
}

// updatePreview updates the live preview state.
func (q *quickCaptureInput) updatePreview() {
	input := strings.ReplaceAll(q.input.Value(), "\n", " ")
	input = strings.TrimSpace(input)
	if input == "" {
		q.preview = nil
		q.previewErr = ""
		return
	}
	q.preview, q.previewErr = parseQuickCaptureForPreview(input, q.project, q.allProjects)
}

// renderPreview formats the parsed preview for display.
func (q *quickCaptureInput) renderPreview() string {
	if q.previewErr != "" {
		return ValidationErrorStyle.Render(q.previewErr)
	}
	if q.preview == nil || q.preview.title == "" {
		return ""
	}

	var parts []string

	// Type and title
	typeStyle := GetTypeStyle(string(q.preview.taskType))
	parts = append(parts, typeStyle.Render(q.preview.taskType.Icon())+" "+q.preview.title)

	if q.preview.priority != 3 {
		parts = append(parts, fmt.Sprintf("!%d", q.preview.priority))
	}

	if len(q.preview.tags) > 0 {
		tagStrs := make([]string, len(q.preview.tags))
		for i, t := range q.preview.tags {
			tagStrs[i] = "#" + t
		}
		parts = append(parts, strings.Join(tagStrs, " "))
	}

	if q.preview.dueDate != nil {
		parts = append(parts, "@"+q.preview.dueDate.Format("Jan 2"))
	}

	if q.preview.recurrence != nil {
		parts = append(parts, "~"+q.preview.recurrence.String())
	}

	if q.preview.section != "" {
		parts = append(parts, "/"+q.preview.section)
	}

	if q.preview.status != "" && q.preview.status != models.StatusTodo {
		parts = append(parts, q.preview.status.Icon()+" $"+string(q.preview.status))
	}

	if q.preview.targetProjectName != "" {
		parts = append(parts, "→ "+q.preview.targetProjectName)
	}

	previewText := strings.Join(parts, " · ")
	
	// Wrap text if width is set and preview exceeds it
	if q.width > 0 {
		previewText = wrapPreviewText(previewText, q.width)
	}
	
	return previewText
}

// wrapPreviewText wraps preview text to fit within the specified width.
func wrapPreviewText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}

	var lines []string
	remaining := text

	for len(remaining) > width {
		// Find a good break point (prefer breaking at " · " separator, then space)
		breakPoint := width
		
		// First try to break at a " · " within the width
		if sepIdx := strings.LastIndex(remaining[:width], " · "); sepIdx > 0 {
			breakPoint = sepIdx + 3 // Include the " · " separator
		} else {
			// Otherwise try to break at last space
			for i := width; i > 0; i-- {
				if remaining[i-1] == ' ' {
					breakPoint = i - 1
					break
				}
			}
		}
		
		lines = append(lines, remaining[:breakPoint])
		remaining = strings.TrimLeft(remaining[breakPoint:], " ")
	}
	
	if remaining != "" {
		lines = append(lines, remaining)
	}
	
	return strings.Join(lines, "\n")
}

// renderAutocomplete renders the autocomplete dropdown.
func (q *quickCaptureInput) renderAutocomplete() string {
	if q.acMode == acModeNone || len(q.suggestions) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	if q.acViewportTop > 0 {
		b.WriteString(MutedStyle.Render("  ↑ " + fmt.Sprintf("%d more", q.acViewportTop)))
		b.WriteString("\n")
	}

	visibleEnd := q.acViewportTop + acVisibleCount
	if visibleEnd > len(q.suggestions) {
		visibleEnd = len(q.suggestions)
	}

	for i := q.acViewportTop; i < visibleEnd; i++ {
		s := q.suggestions[i]
		if i == q.acCursor {
			b.WriteString(SelectedStyle.Render("> " + s.display))
		} else {
			b.WriteString(MutedStyle.Render("  " + s.display))
		}
		b.WriteString("\n")
	}

	remaining := len(q.suggestions) - visibleEnd
	if remaining > 0 {
		b.WriteString(MutedStyle.Render("  ↓ " + fmt.Sprintf("%d more", remaining)))
		b.WriteString("\n")
	}

	return b.String()
}

// isAutocompleteActive returns whether autocomplete is currently showing.
func (q *quickCaptureInput) isAutocompleteActive() bool {
	return q.acMode != acModeNone && len(q.suggestions) > 0
}

// inputValue returns the current text value, newlines replaced with spaces and trimmed.
func (q *quickCaptureInput) inputValue() string {
	input := strings.ReplaceAll(q.input.Value(), "\n", " ")
	return strings.TrimSpace(input)
}
