package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Autocomplete mode constants
const (
	acModeNone          = iota // No autocomplete active
	acModeTag                  // Completing a #tag
	acModeParent               // Completing a >parent
	acModeType                 // Completing a ^type
	acModePriority             // Completing a !priority
	acModeDate                 // Completing a @date
	acModeRecur                // Completing a ~recurrence
	acModeSection              // Completing a /section
	acModeStatus               // Completing a $status
	acModeProject              // Completing a %project
	acModeHas                  // has:property filter (global search)
	acModeIs                   // is:state filter (global search)
	acModeNo                   // no:property filter (global search)
	acModeSearchProject        // p:project filter (global search)
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
	aliasesUsed       []string          // Alias forms used, e.g. ["tag:", "pri:"]
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
	triggers     []ACTrigger // Active triggers (may exclude %project)

	// Live preview
	preview    *quickCaptureResult
	previewErr string

	// Context needed for parsing/preview
	project           *models.Project
	allProjects       []*models.Project
	skipProjectSyntax bool // When true, %project syntax is disabled

	// Layout
	width int // Available width for preview wrapping
}

// newQuickCaptureInput creates a new quickCaptureInput with default textarea settings.
func newQuickCaptureInput(project *models.Project, allProjects []*models.Project, placeholder string, skipProject bool) quickCaptureInput {
	input := textarea.New()
	input.Placeholder = placeholder
	input.CharLimit = 200
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	input.Focus()
	applyThemedTextarea(&input)

	triggers := quickCaptureTriggers
	if skipProject {
		triggers = make([]ACTrigger, 0, len(quickCaptureTriggers)-1)
		for _, t := range quickCaptureTriggers {
			if t.Mode != acModeProject {
				triggers = append(triggers, t)
			}
		}
	}

	qci := quickCaptureInput{
		input:             input,
		project:           project,
		allProjects:       allProjects,
		skipProjectSyntax: skipProject,
		triggers:          triggers,
	}
	qci.initCaches()
	return qci
}

// initCaches builds tag, parent, section, and project caches for autocomplete.
func (q *quickCaptureInput) initCaches() {
	// Cache all tags from the project
	q.tagCache = CollectTagNamesForProject(q.project)

	// Cache potential parent tasks (non-completed, can have children)
	q.parentCache = make([]*models.Task, 0)
	for _, task := range q.project.Tasks {
		if !task.Status.IsComplete() && task.Type.CanHaveChildren() {
			q.parentCache = append(q.parentCache, task)
		}
	}

	// Cache only built-in GTD sections.
	q.sectionCache = gtdSections(q.project.GetSections())

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
	mode, pos, partial := DetectTrigger(q.input.Value(), q.triggers)
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
	q.preview, q.previewErr = parseQuickCaptureForPreview(input, q.project, q.allProjects, q.skipProjectSyntax)
}

// previewGroupLabelWidth is the padded width used for the leading group label
// column ("Task", "Route", "Schedule", "Metadata", "Aliases"). Sized to the
// longest label plus a trailing space so the body column lines up.
const previewGroupLabelWidth = 9

// renderPreview formats the parsed preview for display. The output is chunked
// into labeled groups (Task / Route / Schedule / Metadata) so the user can
// scan what the parser understood without parsing a separator-soup. Only
// populated groups are shown; alias hints go on their own line.
func (q *quickCaptureInput) renderPreview() string {
	if q.previewErr != "" {
		return ValidationErrorStyle.Render(q.previewErr)
	}
	if q.preview == nil || q.preview.title == "" {
		return ""
	}

	type previewGroup struct {
		label  string
		chunks []string
	}
	var groups []previewGroup

	typeStyle := GetTypeStyle(string(q.preview.taskType))
	taskChunks := []string{typeStyle.Render(q.preview.taskType.Icon()) + " " + q.preview.title}
	if q.preview.status != "" && q.preview.status != models.StatusTodo {
		taskChunks = append(taskChunks, q.preview.status.Icon()+" $"+statusShortcut(q.preview.status))
	}
	groups = append(groups, previewGroup{label: "Task", chunks: taskChunks})

	var routeChunks []string
	if q.preview.section != "" {
		if !models.IsGTDSection(q.preview.section) {
			routeChunks = append(routeChunks, "/"+q.preview.section+" → Inbox")
		} else {
			routeChunks = append(routeChunks, "/"+q.preview.section)
		}
	}
	if q.preview.targetProjectName != "" {
		routeChunks = append(routeChunks, "→ "+q.preview.targetProjectName)
	}
	if len(routeChunks) > 0 {
		groups = append(groups, previewGroup{label: "Route", chunks: routeChunks})
	}

	var scheduleChunks []string
	if q.preview.dueDate != nil {
		scheduleChunks = append(scheduleChunks, "@"+q.preview.dueDate.Format("Jan 2"))
	}
	if q.preview.recurrence != nil {
		scheduleChunks = append(scheduleChunks, "~"+q.preview.recurrence.String())
	}
	if len(scheduleChunks) > 0 {
		groups = append(groups, previewGroup{label: "Schedule", chunks: scheduleChunks})
	}

	var metaChunks []string
	if q.preview.priority != 3 {
		metaChunks = append(metaChunks, fmt.Sprintf("!%d", q.preview.priority))
	}
	if len(q.preview.tags) > 0 {
		tagStrs := make([]string, len(q.preview.tags))
		for i, t := range q.preview.tags {
			tagStrs[i] = "#" + t
		}
		metaChunks = append(metaChunks, strings.Join(tagStrs, " "))
	}
	if len(metaChunks) > 0 {
		groups = append(groups, previewGroup{label: "Metadata", chunks: metaChunks})
	}

	var lines []string
	for _, g := range groups {
		body := strings.Join(g.chunks, " · ")
		lines = append(lines, q.renderPreviewLine(g.label, body))
	}

	if len(q.preview.aliasesUsed) > 0 {
		body := MutedStyle.Render(strings.Join(q.preview.aliasesUsed, ", "))
		lines = append(lines, q.renderPreviewLine("Aliases", body))
	}

	return strings.Join(lines, "\n")
}

// renderPreviewLine emits a single "Label   body" line, wrapping the body
// when width is set so no rendered line exceeds q.width visual columns.
// Continuation lines are indented to the label column for readability.
func (q *quickCaptureInput) renderPreviewLine(label, body string) string {
	prefix := MutedStyle.Render(fmt.Sprintf("%-*s", previewGroupLabelWidth, label))
	if q.width <= 0 {
		return prefix + body
	}
	bodyWidth := q.width - previewGroupLabelWidth
	if bodyWidth <= 0 {
		// Width is too narrow for any body alongside the label; let the
		// label sit on its own line and wrap the body on the next.
		return prefix + "\n" + strings.Join(wrapStyled(body, q.width), "\n")
	}
	wrapped := wrapStyled(body, bodyWidth)
	indent := strings.Repeat(" ", previewGroupLabelWidth)
	for i, line := range wrapped {
		if i == 0 {
			wrapped[i] = prefix + line
		} else {
			wrapped[i] = indent + line
		}
	}
	return strings.Join(wrapped, "\n")
}

// renderSyntaxGuide returns the formatted sigil/alias reference shown when
// the user toggles the gated syntax help. Shared by Quick Capture and the
// linked-task creator so both surfaces stay in lockstep.
func (q *quickCaptureInput) renderSyntaxGuide() string {
	prefixStyle := MatchHighlightStyle
	type row struct{ prefix, desc, examples string }
	rows := []row{
		{"^type", "Task type", "^epic  ^story  ^task  ^bug  ^idea  · or: type:bug"},
		{"!N/!word", "Priority (1-5)", "!1  !urgent/!high → !1  !normal → !3  !low → !5  · or: pri:2"},
		{"#tag", "Add tag", "#backend  #urgent  (multiple allowed)  · or: tag:backend"},
		{"@date", "Due date", "@tomorrow  @friday  @+3d · or: due:tomorrow  due:fri  due:3d"},
		{"~recur", "Recurrence", "~daily  ~weekly  ~2w  ~monthly  · or: recur:weekly"},
		{">parent", "Parent task", ">42  >Login  (short ID or title prefix)  · or: parent:42"},
		{"/section", "Section", "/next  /inbox  /waiting  /someday  · or: sec:next"},
		{"$status", "Initial status", "$start  $blocked  $done  $review  · or: status:done"},
	}
	if !q.skipProjectSyntax {
		rows = append(rows, row{"%project", "Target project", "%myproject  (slug or name)  · or: p:myproject"})
	}
	var b strings.Builder
	b.WriteString(MutedStyle.Render("Quick Capture Syntax"))
	b.WriteString("\n\n")
	for _, r := range rows {
		b.WriteString("  ")
		b.WriteString(prefixStyle.Render(fmt.Sprintf("%-10s", r.prefix)))
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%-16s", r.desc)))
		b.WriteString("  ")
		b.WriteString(MutedStyle.Render(r.examples))
		b.WriteString("\n")
	}
	return b.String()
}

// statusShortcut maps a TaskStatus back to its quick-capture keyword.
func statusShortcut(s models.TaskStatus) string {
	switch s {
	case models.StatusInProgress:
		return "start"
	case models.StatusReview:
		return "review"
	case models.StatusStandby:
		return "standby"
	case models.StatusBlocked:
		return "blocked"
	case models.StatusDone:
		return "done"
	case models.StatusCanceled:
		return "canceled"
	case models.StatusMaybe:
		return "maybe"
	default:
		return string(s)
	}
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
