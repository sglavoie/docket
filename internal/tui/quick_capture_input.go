package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

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

// quickCaptureTriggers defines the trigger characters for quick capture autocomplete.
var quickCaptureTriggers = []ACTrigger{
	{'#', acModeTag},
	{'>', acModeParent},
	{'^', acModeType},
	{'!', acModePriority},
	{'@', acModeDate},
	{'~', acModeRecur},
	{'/', acModeSection},
	{'$', acModeStatus},
	{'%', acModeProject},
}

// quickCaptureInput encapsulates the autocomplete, preview, and parsing
// infrastructure shared between QuickCaptureModel and LinkedTaskCreatorModel.
type quickCaptureInput struct {
	input textarea.Model
	err   error

	// Autocomplete state
	ac           AutocompleteState
	tagCache     []string
	parentCache  []*models.Task
	sectionCache []string
	projectCache []*models.Project

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
	input.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	input.Focus()

	qci := quickCaptureInput{
		input:       input,
		project:     project,
		allProjects: allProjects,
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
	return q.ac.HandleKey(key, func() {
		q.acceptSuggestion()
		q.updatePreview()
	})
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
	mode, pos, partial := DetectTrigger(q.input.Value(), quickCaptureTriggers)
	if mode == acModeNone {
		q.ac.Clear()
		return
	}

	var items []ACSuggestion
	switch mode {
	case acModeTag:
		items = q.getTagSuggestions(partial)
	case acModeParent:
		items = q.getParentSuggestions(partial)
	case acModeType:
		items = q.getTypeSuggestions(partial)
	case acModePriority:
		items = q.getPrioritySuggestions(partial)
	case acModeDate:
		items = q.getDateSuggestions(partial)
	case acModeRecur:
		items = q.getRecurrenceSuggestions(partial)
	case acModeSection:
		items = q.getSectionSuggestions(partial)
	case acModeStatus:
		items = q.getStatusSuggestions(partial)
	case acModeProject:
		items = q.getProjectSuggestions(partial)
	}
	q.ac.UpdateSuggestions(mode, pos, items)
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
		if !models.IsGTDSection(q.preview.section) {
			parts = append(parts, "/"+q.preview.section+" → Inbox")
		} else {
			parts = append(parts, "/"+q.preview.section)
		}
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
	if !q.ac.IsActive() {
		return ""
	}
	return "\n" + q.ac.RenderDropdown("")
}

// isAutocompleteActive returns whether autocomplete is currently showing.
func (q *quickCaptureInput) isAutocompleteActive() bool {
	return q.ac.IsActive()
}

// inputValue returns the current text value, newlines replaced with spaces and trimmed.
func (q *quickCaptureInput) inputValue() string {
	input := strings.ReplaceAll(q.input.Value(), "\n", " ")
	return strings.TrimSpace(input)
}
