package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// Autocomplete mode constants
const (
	acModeNone        = iota // No autocomplete active
	acModeTag                // Completing a #tag
	acModeParent             // Completing a >parent
	acModePriority           // Completing a !priority
	acModeDate               // Completing a @date
	acModeDestination        // Completing a $destination
	acModeProject            // Completing a %project
)

// acVisibleCount is the number of suggestions visible at once (scrollable window size)
const acVisibleCount = 7

// quickCaptureResult holds parsed metadata from quick capture input.
type quickCaptureResult struct {
	title             string
	priority          int
	prioritySet       bool
	tags              []string
	dueDate           *time.Time
	parentID          *string
	destination       app.CaptureDestination
	destinationSet    bool
	targetProjectID   *string // Target project ID for cross-project capture
	targetProjectName string  // Target project name (for preview display)
}

// quickCaptureInput encapsulates the compact Task Composer's autocomplete,
// preview, and parsing infrastructure.
type quickCaptureInput struct {
	input textarea.Model
	err   error

	// Autocomplete state
	ac           AutocompleteState
	tagCache     []string
	parentCache  []*models.Task
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

	// Syntax help state, shared by both capture hosts so the expanded/collapsed
	// flag and scroll offset live in one place. helpExpanded is the logical
	// toggle; syntaxVP holds the scrollable rendered guide and its offset, which
	// is preserved across autocomplete activation and clamped on resize.
	helpExpanded bool
	syntaxVP     viewport.Model
	syntaxReady  bool
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

	triggers := quickCaptureTriggers(skipProject)

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

// quickCaptureTriggers projects the canonical capture definitions into the
// autocomplete engine's presentation-specific modes. The registry owns which
// triggers exist; this switch owns only the TUI behavior attached to each field.
func quickCaptureTriggers(skipProject bool) []ACTrigger {
	definitions := textgrammar.Definitions(textgrammar.SurfaceCapture)
	triggers := make([]ACTrigger, 0, len(definitions))
	for _, definition := range definitions {
		if skipProject && definition.Field == textgrammar.FieldProject {
			continue
		}
		mode := autocompleteModeForField(definition.Field)
		if mode == acModeNone || len(definition.Trigger) != 1 {
			continue
		}
		triggers = append(triggers, ACTrigger{Char: definition.Trigger[0], Mode: mode})
	}
	return triggers
}

func autocompleteModeForField(field textgrammar.Field) int {
	switch field {
	case textgrammar.FieldTag:
		return acModeTag
	case textgrammar.FieldParent:
		return acModeParent
	case textgrammar.FieldPriority:
		return acModePriority
	case textgrammar.FieldDue:
		return acModeDate
	case textgrammar.FieldDestination:
		return acModeDestination
	case textgrammar.FieldProject:
		return acModeProject
	default:
		return acModeNone
	}
}

// initCaches builds tag, parent, and project caches for autocomplete.
func (q *quickCaptureInput) initCaches() {
	// Cache all tags from the project
	q.tagCache = CollectTagNamesForProject(q.project)

	// Cache potential parent tasks (non-completed).
	q.parentCache = make([]*models.Task, 0)
	for _, task := range q.project.Tasks {
		if !task.IsComplete() {
			q.parentCache = append(q.parentCache, task)
		}
	}

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

// ensureSyntaxViewport lazily constructs the syntax help viewport so callers can
// scroll or measure it before it has been sized by a render pass.
func (q *quickCaptureInput) ensureSyntaxViewport() {
	if !q.syntaxReady {
		q.syntaxVP = viewport.New(viewport.WithWidth(max(q.width, 1)), viewport.WithHeight(1))
		q.syntaxReady = true
	}
}

// setSyntaxViewportWidth resizes the guide viewport to the composer width and,
// when help is open, re-renders the width-measured guide and clamps the offset
// so a resize can never leave the viewport scrolled past its (possibly shorter)
// new content.
func (q *quickCaptureInput) setSyntaxViewportWidth(width int) {
	q.ensureSyntaxViewport()
	q.syntaxVP.SetWidth(max(width, 1))
	if q.helpExpanded {
		q.syntaxVP.SetContent(q.renderSyntaxGuide())
		q.clampSyntaxOffset()
	}
}

// setSyntaxViewportHeight sets the guide viewport height (the bounded lower-pane
// budget) and re-clamps the offset against the new visible height.
func (q *quickCaptureInput) setSyntaxViewportHeight(height int) {
	q.ensureSyntaxViewport()
	q.syntaxVP.SetHeight(max(height, 1))
	q.clampSyntaxOffset()
}

// clampSyntaxOffset re-applies the current offset, which the viewport clamps to
// its valid range. Used after resize so a shrunk guide or pane cannot leave a
// stale offset scrolled past the end.
func (q *quickCaptureInput) clampSyntaxOffset() {
	q.syntaxVP.SetYOffset(q.syntaxVP.YOffset())
}

// openHelp expands the syntax guide at the top. The offset resets to zero so the
// guide always opens showing its first group (Basics), matching the reported
// interaction model.
func (q *quickCaptureInput) openHelp() {
	q.helpExpanded = true
	q.ensureSyntaxViewport()
	q.syntaxVP.SetContent(q.renderSyntaxGuide())
	q.syntaxVP.GotoTop()
}

// closeHelp collapses the syntax guide. The offset is retained on the viewport
// so re-opening via toggleHelp still starts at the top (openHelp resets it),
// while autocomplete-driven hide/show preserves the mid-scroll position.
func (q *quickCaptureInput) closeHelp() {
	q.helpExpanded = false
}

// toggleHelp opens the guide at the top when closed, or closes it when open.
func (q *quickCaptureInput) toggleHelp() {
	if q.helpExpanded {
		q.closeHelp()
		return
	}
	q.openHelp()
}

// syntaxScrollLine scrolls the guide by delta lines (negative = up).
func (q *quickCaptureInput) syntaxScrollLine(delta int) {
	q.ensureSyntaxViewport()
	if delta < 0 {
		q.syntaxVP.ScrollUp(-delta)
	} else {
		q.syntaxVP.ScrollDown(delta)
	}
}

// syntaxScrollPage scrolls the guide by delta full pages (negative = up).
func (q *quickCaptureInput) syntaxScrollPage(delta int) {
	q.ensureSyntaxViewport()
	if delta < 0 {
		q.syntaxVP.PageUp()
	} else {
		q.syntaxVP.PageDown()
	}
}

// syntaxOverflows reports whether the rendered guide is taller than its visible
// viewport, i.e. scrolling is meaningful. Footer scroll hints and the scroll
// percentage are shown only in this case.
func (q *quickCaptureInput) syntaxOverflows() bool {
	return q.syntaxReady && q.syntaxVP.TotalLineCount() > q.syntaxVP.Height()
}

// syntaxScrollPercent returns the scroll position as a 0-100 percentage, clamped
// so a non-overflowing guide reports fully scrolled (100%).
func (q *quickCaptureInput) syntaxScrollPercent() int {
	if !q.syntaxOverflows() {
		return 100
	}
	pct := int(q.syntaxVP.ScrollPercent() * 100)
	return max(0, min(pct, 100))
}

// handleAutocompleteKey handles keys when autocomplete is active.
// Returns true if the key was consumed.
func (q *quickCaptureInput) handleAutocompleteKey(key string) bool {
	return q.ac.HandleKey(key, func() {
		q.acceptSuggestion()
		q.updatePreview()
	})
}

// autocompleteLiteralComplete reports whether the active tag/type token is
// already valid as typed. The composer uses this to let Enter submit a complete
// literal while Tab remains the explicit completion key.
func (q *quickCaptureInput) autocompleteLiteralComplete() bool {
	if !q.ac.IsActive() {
		return false
	}
	mode, _, partial := DetectTrigger(q.input.Value(), q.triggers)
	if mode != q.ac.Mode || partial == "" {
		return false
	}
	switch mode {
	case acModeTag:
		definition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldTag)
		parsed := textgrammar.Parse(definition.Trigger+partial, textgrammar.SurfaceCapture)
		return parsed.Text == "" && len(parsed.TokensFor(textgrammar.FieldTag)) == 1
	default:
		return false
	}
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
	value := q.input.Value()
	mode, pos, partial := DetectTrigger(value, q.triggers)
	if mode == acModeNone {
		q.ac.Clear()
		return
	}

	lineStart := strings.LastIndex(value[:pos], "\n") + 1
	parsedLine := textgrammar.Parse(value[lineStart:pos], textgrammar.SurfaceCapture)
	if field, singleton := autocompleteSingletonField(mode); singleton && len(parsedLine.TokensFor(field)) > 0 {
		q.ac.Clear()
		return
	}

	var items []ACSuggestion
	switch mode {
	case acModeTag:
		items = q.getTagSuggestions(partial)
		items = filterUsedTagSuggestions(items, parsedLine.TokensFor(textgrammar.FieldTag))
	case acModeParent:
		items = q.getParentSuggestions(partial)
	case acModePriority:
		items = q.getPrioritySuggestions(partial)
	case acModeDate:
		items = q.getDateSuggestions(partial)
	case acModeDestination:
		items = q.getDestinationSuggestions(partial)
	case acModeProject:
		items = q.getProjectSuggestions(partial)
	}
	q.ac.UpdateSuggestions(mode, pos, items)
}

func autocompleteSingletonField(mode int) (textgrammar.Field, bool) {
	switch mode {
	case acModeParent:
		return textgrammar.FieldParent, true
	case acModePriority:
		return textgrammar.FieldPriority, true
	case acModeDate:
		return textgrammar.FieldDue, true
	case acModeDestination:
		return textgrammar.FieldDestination, true
	case acModeProject:
		return textgrammar.FieldProject, true
	default:
		return "", false
	}
}

func filterUsedTagSuggestions(items []ACSuggestion, tokens []textgrammar.Token) []ACSuggestion {
	if len(tokens) == 0 {
		return items
	}
	used := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		used[strings.ToLower(strings.TrimSpace(token.Value))] = true
	}
	filtered := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		if !used[strings.ToLower(strings.TrimSpace(item.Value))] {
			filtered = append(filtered, item)
		}
	}
	return filtered
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
// column ("Task", "Route", "Schedule", "Priority", "Tags"). Sized to the
// longest label plus a trailing space so the body column lines up.
const previewGroupLabelWidth = 9

// previewGroup is one labeled cluster of parsed attributes (Task, Route,
// Schedule, Priority, Tags). Both the full grouped preview and the compact
// two-row fallback are built from the same groups so they can never disagree
// about what the parser understood.
type previewGroup struct {
	label  string
	chunks []string
}

// previewGroups builds the populated preview groups from the parsed result, or
// reports ok=false when there is nothing to preview. It is the single source of
// the recognized task, route, schedule, state, priority, tag, project, and
// information shown by both preview renderers.
func (q *quickCaptureInput) previewGroups() (groups []previewGroup, ok bool) {
	if q.preview == nil || q.preview.title == "" {
		return nil, false
	}

	previewTask := &models.Task{
		Title:         q.preview.title,
		WorkflowState: models.StateInbox,
		Priority:      q.preview.priority,
		Tags:          q.preview.tags,
	}
	taskTitle := TaskTitleStyleFor(previewTask, TaskTitleStyleOptions{}).Render(q.preview.title)
	taskChunks := []string{taskTitle}
	groups = append(groups, previewGroup{label: "Task", chunks: taskChunks})
	destinationDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDestination)
	dueDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDue)
	priorityDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPriority)
	tagDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldTag)

	destination := app.CaptureDestinationInbox
	destinationLabel := destinationDefinition.Trigger + string(destination) + " (default)"
	if q.preview.parentID != nil {
		destination = app.CaptureDestinationBacklog
		destinationLabel = destinationDefinition.Trigger + string(destination) + " (default)"
	}
	if q.preview.destinationSet {
		destination = q.preview.destination
		destinationLabel = destinationDefinition.Trigger + string(destination)
	}
	routeChunks := []string{destination.WorkflowState().Icon() + " " + destinationLabel}
	if q.preview.targetProjectName != "" {
		routeChunks = append(routeChunks, "→ "+q.preview.targetProjectName)
	}
	groups = append(groups, previewGroup{label: "Route", chunks: routeChunks})

	var scheduleChunks []string
	if q.preview.dueDate != nil {
		scheduleChunks = append(scheduleChunks, dueDefinition.Trigger+q.preview.dueDate.Format("Jan 2"))
	}
	if len(scheduleChunks) > 0 {
		groups = append(groups, previewGroup{label: "Schedule", chunks: scheduleChunks})
	}

	if q.preview.priority != models.PriorityUnset {
		groups = append(groups, previewGroup{label: "Priority", chunks: []string{fmt.Sprintf("%s%d", priorityDefinition.Trigger, q.preview.priority)}})
	}
	if len(q.preview.tags) > 0 {
		tagStrs := make([]string, len(q.preview.tags))
		for i, t := range q.preview.tags {
			tagStrs[i] = tagDefinition.Trigger + t
		}
		groups = append(groups, previewGroup{label: "Tags", chunks: []string{strings.Join(tagStrs, " ")}})
	}

	return groups, true
}

// renderPreview formats the parsed preview for display. The output is chunked
// into labeled groups (Task / Route / Schedule / Priority / Tags) so the user can
// scan what the parser understood without parsing a separator-soup. Only
// populated groups are shown.
func (q *quickCaptureInput) renderPreview() string {
	if q.previewErr != "" {
		return ValidationErrorStyle.Render(q.previewErr)
	}
	groups, ok := q.previewGroups()
	if !ok {
		return ""
	}

	var lines []string
	for _, g := range groups {
		lines = append(lines, q.renderPreviewLine(g.label, strings.Join(g.chunks, " · ")))
	}
	return strings.Join(lines, "\n")
}

// renderCompactPreview renders the same recognized metadata as renderPreview but
// as a single unlabeled " · "-joined line wrapped to at most two rows. Quick
// Capture falls back to it when the full grouped preview would displace the
// scrollable syntax guide or the app-shell footer, keeping the composed view
// within its height budget without dropping what the parser understood.
func (q *quickCaptureInput) renderCompactPreview() string {
	if q.previewErr != "" {
		return q.clampToRows(ValidationErrorStyle.Render(q.previewErr), 2)
	}
	groups, ok := q.previewGroups()
	if !ok {
		return ""
	}
	var chunks []string
	for _, g := range groups {
		chunks = append(chunks, g.chunks...)
	}
	return q.clampToRows(strings.Join(chunks, " · "), 2)
}

// composePreviewBlock renders the live preview (or error) for a composed capture
// view, choosing the full grouped preview when it fits alongside the lower pane
// and falling back to the compact two-row form when it would otherwise displace
// the lower pane or the footer. It returns the rendered block, its row count, and
// whether a trailing blank separator should follow it (1 when non-empty). Shared
// by Quick Capture and the Linked Task Creator so both budget the preview identically.
func (q *quickCaptureInput) composePreviewBlock(remaining, lowerMin int) (block string, rows, blankAfter int) {
	renderFull := func() string {
		if q.err != nil {
			return ValidationErrorStyle.Render(q.err.Error())
		}
		return q.renderPreview()
	}
	renderCompact := func() string {
		if q.err != nil {
			return q.clampToRows(ValidationErrorStyle.Render(q.err.Error()), 2)
		}
		return q.renderCompactPreview()
	}

	block = renderFull()
	if block != "" {
		rows = lipgloss.Height(block)
		blankAfter = 1
	}

	// Fall back to compact when a lower pane needs room the full preview would eat.
	if lowerMin > 0 && rows+blankAfter+lowerMin > remaining {
		block = renderCompact()
		rows, blankAfter = 0, 0
		if block != "" {
			rows = lipgloss.Height(block)
			blankAfter = 1
		}
	}

	// Safety net when there is no lower pane: never let the preview overflow the
	// budget. Clip trailing rows rather than push the footer off screen.
	if lowerMin == 0 && rows+blankAfter > remaining {
		lines := strings.Split(block, "\n")
		if keep := max(remaining-blankAfter, 1); len(lines) > keep {
			block = strings.Join(lines[:keep], "\n")
			rows = keep
		}
	}
	return block, rows, blankAfter
}

// clampToRows wraps a styled string to q.width and keeps at most rows lines,
// silently dropping any overflow. Used for the best-effort compact preview,
// where losing a trailing detail is acceptable but a fixed row budget is not.
func (q *quickCaptureInput) clampToRows(styled string, rows int) string {
	if rows < 1 {
		rows = 1
	}
	var lines []string
	if q.width <= 0 {
		lines = strings.Split(styled, "\n")
	} else {
		lines = wrapStyled(styled, q.width)
	}
	if len(lines) > rows {
		lines = lines[:rows]
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

// syntaxGuideTitle is the neutral heading shared by both capture hosts (Quick
// Capture and the Linked Task Creator). It is intentionally host-agnostic so
// the same grammar reference reads correctly wherever it appears.
const syntaxGuideTitle = "Capture Syntax"

// syntaxGuideEntry documents one canonical sigil family: the accepted form, a
// short description, display examples, and a set of full capture inputs that
// must parse successfully. The display fields (forms/desc/examples) may carry
// presentation sugar such as arrows or "(repeatable)" hints; the validation
// inputs are kept separate so parser-backed tests exercise real, parseable
// capture strings rather than the rendered sugar.
type syntaxGuideEntry struct {
	forms      string
	desc       string
	examples   string
	validation []string
}

// syntaxGuideGroup clusters related capture sigils under a heading so the
// reference scans as intent groups rather than a flat sigil soup.
type syntaxGuideGroup struct {
	name    string
	entries []syntaxGuideEntry
}

// syntaxGuideGroups returns the grouped sigil reference. Project Routing is
// only included when %project capture is enabled, so the Linked Task Creator
// (which skips project syntax) never advertises it.
func (q *quickCaptureInput) syntaxGuideGroups() []syntaxGuideGroup {
	var groupOrder []string
	byGroup := make(map[string][]syntaxGuideEntry)
	for _, def := range textgrammar.Definitions(textgrammar.SurfaceCapture) {
		if def.Legacy || (q.skipProjectSyntax && def.Field == textgrammar.FieldProject) {
			continue
		}
		if _, exists := byGroup[def.Group]; !exists {
			groupOrder = append(groupOrder, def.Group)
		}
		entry := syntaxGuideEntry{
			forms:    strings.Join(def.Forms, " · "),
			desc:     def.Description,
			examples: strings.Join(def.Examples, "  "),
		}
		for _, example := range def.Examples {
			entry.validation = append(entry.validation, "Task "+example)
		}
		byGroup[def.Group] = append(byGroup[def.Group], entry)
	}

	groups := make([]syntaxGuideGroup, 0, len(groupOrder))
	for _, name := range groupOrder {
		if entries := byGroup[name]; len(entries) > 0 {
			groups = append(groups, syntaxGuideGroup{name: name, entries: entries})
		}
	}
	return groups
}

// Layout constants for the syntax guide's measured responsive rendering.
const (
	syntaxEntryIndent = 2 // Leading indent for an entry's first field.
	syntaxFieldIndent = 4 // Indent for wrapped/stacked continuation fields.
	syntaxColumnGap   = 2 // Gap between aligned columns.
)

// renderSyntaxGuide returns the grouped, measured, wrap-aware canonical sigil
// reference shown when the user toggles the gated syntax help. Shared by Quick
// Capture and the linked-task creator so both surfaces stay in lockstep.
//
// Each entry is laid out with a measured responsive strategy, tried in order:
//  1. Aligned row: forms, description, and examples share one line, with forms
//     and descriptions padded to consistent columns.
//  2. If the aligned row does not fit, keep forms and description on the first
//     row and wrap the examples underneath.
//  3. If forms and description cannot coexist on one row, stack every field.
//
// All measurement uses ANSI-aware visual width, so the guide never overflows
// q.width at narrow widths regardless of styling.
func (q *quickCaptureInput) renderSyntaxGuide() string {
	groups := q.syntaxGuideGroups()

	// Compute aligned column widths across every entry so aligned rows line up
	// their forms and descriptions into consistent columns.
	formsCol, descCol := 0, 0
	for _, g := range groups {
		for _, e := range g.entries {
			formsCol = max(formsCol, lipgloss.Width(e.forms))
			descCol = max(descCol, lipgloss.Width(e.desc))
		}
	}

	var lines []string

	// appendWrappedField wraps a single field to q.width at the given indent.
	appendWrappedField := func(indent int, style lipgloss.Style, text string) {
		pad := strings.Repeat(" ", indent)
		avail := q.width - indent
		if q.width <= 0 || avail <= 0 {
			lines = append(lines, pad+style.Render(text))
			return
		}
		for _, w := range wrapStyled(text, avail) {
			lines = append(lines, pad+style.Render(w))
		}
	}

	// padStyled pads an already-styled field with trailing spaces so its rendered
	// visual width reaches col. Trailing padding is unstyled, which is width-safe.
	padStyled := func(styled string, plainWidth, col int) string {
		if plainWidth >= col {
			return styled
		}
		return styled + strings.Repeat(" ", col-plainWidth)
	}

	appendEntry := func(e syntaxGuideEntry) {
		formsW := lipgloss.Width(e.forms)
		descW := lipgloss.Width(e.desc)
		examplesW := lipgloss.Width(e.examples)

		styledForms := MatchHighlightStyle.Render(e.forms)
		styledDesc := MutedStyle.Render(e.desc)
		styledExamples := MutedStyle.Render(e.examples)

		indentStr := strings.Repeat(" ", syntaxEntryIndent)
		gapStr := strings.Repeat(" ", syntaxColumnGap)
		formsDescRow := indentStr +
			padStyled(styledForms, formsW, formsCol) + gapStr +
			padStyled(styledDesc, descW, descCol)
		fdWidth := syntaxEntryIndent + formsCol + syntaxColumnGap + descCol

		switch {
		case q.width <= 0:
			// No width budget: fall back to a simple stacked form.
			appendWrappedField(syntaxEntryIndent, MatchHighlightStyle, e.forms)
			appendWrappedField(syntaxFieldIndent, MutedStyle, e.desc)
			if examplesW > 0 {
				appendWrappedField(syntaxFieldIndent, MutedStyle, e.examples)
			}
		case examplesW > 0 && fdWidth+syntaxColumnGap+examplesW <= q.width:
			// Mode 1: aligned single row.
			lines = append(lines, formsDescRow+gapStr+styledExamples)
		case fdWidth <= q.width:
			// Mode 2: forms + description on the first row, examples wrapped below.
			lines = append(lines, formsDescRow)
			if examplesW > 0 {
				appendWrappedField(syntaxFieldIndent, MutedStyle, e.examples)
			}
		default:
			// Mode 3: forms and description cannot coexist; stack every field.
			appendWrappedField(syntaxEntryIndent, MatchHighlightStyle, e.forms)
			appendWrappedField(syntaxFieldIndent, MutedStyle, e.desc)
			if examplesW > 0 {
				appendWrappedField(syntaxFieldIndent, MutedStyle, e.examples)
			}
		}
	}

	lines = append(lines, MutedStyle.Render(syntaxGuideTitle))
	for _, g := range groups {
		lines = append(lines, "")
		appendWrappedField(0, SectionHeaderStyle, g.name)
		for _, e := range g.entries {
			appendEntry(e)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// isAutocompleteActive returns whether autocomplete is currently showing.
func (q *quickCaptureInput) isAutocompleteActive() bool {
	return q.ac.IsActive()
}
