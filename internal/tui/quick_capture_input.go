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
	acModeChild              // Completing a <child
	acModePriority           // Completing a !priority
	acModeDate               // Completing a @date
	acModeDestination        // Completing a $destination
	acModeProject            // Completing a %project
	acModePin                // Completing a *pin
	acModeContext            // Completing a ##context
	acModeDefer              // Completing a @@defer
	acModeAttachment         // Completing an &link
	acModeComment            // Completing a ;comment;
	// acModeRemoveTag completes a -#tag. It is reached only from the search
	// surface: capture does not carry the field at all, and quick edit peels the
	// two-character trigger itself so it can reuse the tag mode's candidate list.
	acModeRemoveTag
	// acModeDependency completes a ~task and acModeRemoveDependency a -~task.
	// Only the edit surface reaches the removal mode: capture has no stored set to
	// subtract from, and it peels the two-character trigger itself.
	acModeDependency
	acModeRemoveDependency
	// The remaining negations are search-only, so none of them is ever reached
	// while capturing or editing. Each still needs a mode of its own because the
	// field-to-mode projection has to stay one-to-one for a trigger to be resolved
	// back to the definition it came from; the suggestions themselves come from the
	// positive twin's vocabulary.
	acModeRemovePriority
	acModeRemoveContext
	acModeRemoveDue
	acModeRemoveDefer
	acModeRemoveDestination
	acModeRemovePin
	acModeRemoveProject
	// acModeDerived completes a ?predicate and acModeRemoveDerived a -?predicate.
	// Both are search-only: a derived value is computed from the task rather than
	// stored on it, so there is nothing for capture or edit to write.
	acModeDerived
	acModeRemoveDerived
)

// acVisibleCount is the number of suggestions visible at once (scrollable window size)
const acVisibleCount = 7

// capturedChildRef is one resolved <child reference. The identifying fields are
// copied out of the task the reference named rather than kept as a pointer, so a
// parse result can be previewed and mapped into a request without holding a
// handle on a view-owned task.
type capturedChildRef struct {
	id       string
	shortID  int
	title    string
	terminal bool
}

// quickCaptureResult holds parsed metadata from quick capture input.
type quickCaptureResult struct {
	title             string
	priority          int
	prioritySet       bool
	tags              []string
	context           string
	dueDate           *time.Time
	deferUntil        *time.Time
	deferSet          bool
	parentID          *string
	children          []capturedChildRef
	destination       app.CaptureDestination
	destinationSet    bool
	pinned            bool
	dependencies      []capturedDependencyRef
	attachments       []string
	comments          []string
	targetProjectID   *string // Target project ID for cross-project capture
	targetProjectName string  // Target project name (for preview display)
}

// childIDs projects the resolved children into the stable IDs a CaptureRequest
// carries, in reference order.
func (r *quickCaptureResult) childIDs() []string {
	if len(r.children) == 0 {
		return nil
	}
	ids := make([]string, len(r.children))
	for i, child := range r.children {
		ids[i] = child.id
	}
	return ids
}

// dependencyLocators projects the resolved references into the locators a
// CaptureRequest carries, in reference order.
func (r *quickCaptureResult) dependencyLocators() []app.TaskLocator {
	return dependencyLocators(r.dependencies)
}

// quickCaptureInput encapsulates the compact Task Composer's autocomplete,
// preview, and parsing infrastructure.
type quickCaptureInput struct {
	input textarea.Model
	err   error

	// Autocomplete state
	ac           AutocompleteState
	tagCache     []string
	contextCache []string
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
		if mode == acModeNone {
			continue
		}
		triggers = append(triggers, ACTrigger{Text: definition.Trigger, Mode: mode})
	}
	return triggers
}

func autocompleteModeForField(field textgrammar.Field) int {
	switch field {
	case textgrammar.FieldTag:
		return acModeTag
	case textgrammar.FieldRemoveTag:
		return acModeRemoveTag
	case textgrammar.FieldContext:
		return acModeContext
	case textgrammar.FieldParent:
		return acModeParent
	case textgrammar.FieldChild:
		return acModeChild
	case textgrammar.FieldPriority:
		return acModePriority
	case textgrammar.FieldDue:
		return acModeDate
	case textgrammar.FieldDefer:
		return acModeDefer
	case textgrammar.FieldDestination:
		return acModeDestination
	case textgrammar.FieldProject:
		return acModeProject
	case textgrammar.FieldPinned:
		return acModePin
	case textgrammar.FieldDependency:
		return acModeDependency
	case textgrammar.FieldRemoveDependency:
		return acModeRemoveDependency
	case textgrammar.FieldRemovePriority:
		return acModeRemovePriority
	case textgrammar.FieldRemoveContext:
		return acModeRemoveContext
	case textgrammar.FieldRemoveDue:
		return acModeRemoveDue
	case textgrammar.FieldRemoveDefer:
		return acModeRemoveDefer
	case textgrammar.FieldRemoveDestination:
		return acModeRemoveDestination
	case textgrammar.FieldRemovePinned:
		return acModeRemovePin
	case textgrammar.FieldRemoveProject:
		return acModeRemoveProject
	case textgrammar.FieldDerived:
		return acModeDerived
	case textgrammar.FieldRemoveDerived:
		return acModeRemoveDerived
	case textgrammar.FieldAttachment:
		// An attachment value is a pasted URL or path, so there is nothing to
		// suggest. The mode exists because quickCaptureTriggers drops a field
		// returning acModeNone, and the registry projection must stay one-to-one;
		// refreshAutocomplete has no default arm, so the mode simply shows nothing.
		return acModeAttachment
	case textgrammar.FieldComment:
		// A comment is free prose, so there is nothing to suggest either; the mode
		// exists for the same one-to-one reason acModeAttachment does.
		return acModeComment
	default:
		return acModeNone
	}
}

// initCaches builds tag and project caches for autocomplete. Parent and child
// candidates are deliberately absent: they follow the draft's effective
// %project, which changes while the user types, so caching them here is what
// made parent completion offer the source project's tasks after a route.
func (q *quickCaptureInput) initCaches() {
	// Cache all tags from the project
	q.tagCache = CollectTagNamesForProject(q.project)
	q.contextCache = CollectContextNamesForProject(q.project)

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
	case acModeTag, acModeContext:
		field := textgrammar.FieldTag
		if mode == acModeContext {
			field = textgrammar.FieldContext
		}
		definition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, field)
		parsed := textgrammar.Parse(definition.Trigger+partial, textgrammar.SurfaceCapture)
		return parsed.Text == "" && len(parsed.TokensFor(field)) == 1
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
	case acModeContext:
		items = q.getContextSuggestions(partial)
	case acModeParent:
		items = q.getParentSuggestions(partial, structuralCandidates(q.effectiveProject(parsedLine)))
	case acModeChild:
		effective := q.effectiveProject(parsedLine)
		candidates := filterAdoptedChildCandidates(
			structuralCandidates(effective), effective, parsedLine.TokensFor(textgrammar.FieldChild),
		)
		items = q.getChildSuggestions(partial, candidates)
	case acModePriority:
		items = q.getPrioritySuggestions(partial)
	case acModeDate:
		items = q.getDateSuggestions(partial)
	case acModeDefer:
		items = q.getDeferSuggestions(partial)
	case acModeDestination:
		items = q.getDestinationSuggestions(partial)
	case acModeProject:
		items = q.getProjectSuggestions(partial)
	case acModePin:
		items = q.getPinSuggestions(partial)
	case acModeDependency:
		items = q.getDependencySuggestions(partial, parsedLine)
	}
	q.ac.UpdateSuggestions(mode, pos, items)
}

func autocompleteSingletonField(mode int) (textgrammar.Field, bool) {
	switch mode {
	case acModeParent:
		return textgrammar.FieldParent, true
	case acModeContext:
		return textgrammar.FieldContext, true
	case acModePriority:
		return textgrammar.FieldPriority, true
	case acModeDate:
		return textgrammar.FieldDue, true
	case acModeDefer:
		return textgrammar.FieldDefer, true
	case acModeDestination:
		return textgrammar.FieldDestination, true
	case acModeProject:
		return textgrammar.FieldProject, true
	case acModePin:
		return textgrammar.FieldPinned, true
	default:
		return "", false
	}
}

// effectiveProject resolves the project the drafted line's >parent and <child
// references address, so completion offers the tasks the parse will actually
// match instead of the composer's source project. The scope is the drafted line
// rather than the whole buffer because every nonblank line becomes its own
// request with its own route.
func (q *quickCaptureInput) effectiveProject(draftedLine textgrammar.Result) *models.Project {
	if q.skipProjectSyntax {
		return q.project
	}
	tokens := draftedLine.TokensFor(textgrammar.FieldProject)
	if len(tokens) == 0 {
		return q.project
	}
	// The parser lets the last %project token win, so completion must too.
	if target := resolveProject(tokens[len(tokens)-1].Value, q.allProjects); target != nil {
		return target
	}
	return q.project
}

// structuralCandidates returns the tasks a >parent or <child completion offers.
// Completed tasks are withheld from the suggestion list only; a manually typed
// reference still resolves against every task in the project, so an intentional
// reference to finished work is never blocked by this policy.
func structuralCandidates(project *models.Project) []*models.Task {
	if project == nil {
		return nil
	}
	candidates := make([]*models.Task, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil && !task.IsComplete() {
			candidates = append(candidates, task)
		}
	}
	return candidates
}

// filterAdoptedChildCandidates drops candidates an earlier <child token already
// names. Tokens are resolved before comparison, so a short ID and a title prefix
// naming one task remove it once rather than letting the alias reappear.
func filterAdoptedChildCandidates(candidates []*models.Task, project *models.Project, tokens []textgrammar.Token) []*models.Task {
	if len(tokens) == 0 {
		return candidates
	}
	adopted := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		task, err := resolveChild(token.Value, project)
		if err != nil || task == nil {
			continue
		}
		adopted[task.ID] = true
	}
	if len(adopted) == 0 {
		return candidates
	}
	filtered := make([]*models.Task, 0, len(candidates))
	for _, task := range candidates {
		if !adopted[task.ID] {
			filtered = append(filtered, task)
		}
	}
	return filtered
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
// column ("Task", "Route", "Children", "Schedule", "Priority", "Tags"). Sized to
// the longest label plus a trailing space so the body column lines up.
const previewGroupLabelWidth = 9

// previewGroup is one labeled cluster of parsed attributes (Task, Route,
// Children, Schedule, Priority, Tags). Both the full grouped preview and the
// compact two-row fallback are built from the same groups so they can never
// disagree about what the parser understood.
type previewGroup struct {
	label  string
	chunks []string
}

// previewGroups builds the populated preview groups from the parsed result, or
// reports ok=false when there is nothing to preview. It is the single source of
// the recognized task, route, child, schedule, state, priority, tag, and project
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
	childDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldChild)
	pinnedDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPinned)

	destination := app.CaptureDestinationInbox
	if q.preview.parentID != nil {
		destination = app.CaptureDestinationBacklog
	}
	if q.preview.destinationSet {
		destination = q.preview.destination
	}
	// The deferral route runs through the same helper the submitted request uses,
	// so the preview never promises a landing place the capture will not honor.
	deferred := false
	if routed := composerDeferredDestination(q.preview, destination); routed != destination {
		destination, deferred = routed, true
	}
	destinationLabel := destinationDefinition.Trigger + string(destination)
	switch {
	case deferred:
		destinationLabel += " (deferred)"
	case !q.preview.destinationSet:
		destinationLabel += " (default)"
	}
	routeChunks := []string{destination.WorkflowState().Icon() + " " + destinationLabel}
	if q.preview.targetProjectName != "" {
		routeChunks = append(routeChunks, "→ "+q.preview.targetProjectName)
	}
	groups = append(groups, previewGroup{label: "Route", chunks: routeChunks})

	// Children sits directly after Route because both answer where the task lands.
	// The parser already deduped aliases in first-reference order, so one chunk per
	// entry both survives the compact fallback's flattening and states the count.
	if len(q.preview.children) > 0 {
		childChunks := make([]string, len(q.preview.children))
		for i, child := range q.preview.children {
			childChunks[i] = fmt.Sprintf("%s%d %s", childDefinition.Trigger, child.shortID, child.title)
		}
		groups = append(groups, previewGroup{label: "Children", chunks: childChunks})
	}

	// Depends follows Children because both name other tasks, and it states one
	// chunk per reference for the same reason: the parser already deduped aliases
	// in first-reference order, so the group is also the count.
	if len(q.preview.dependencies) > 0 {
		dependencyDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDependency)
		dependencyChunks := make([]string, len(q.preview.dependencies))
		for i, ref := range q.preview.dependencies {
			dependencyChunks[i] = dependencyDefinition.Trigger + ref.label()
		}
		groups = append(groups, previewGroup{label: "Depends", chunks: dependencyChunks})
	}

	var scheduleChunks []string
	if q.preview.dueDate != nil {
		scheduleChunks = append(scheduleChunks, dueDefinition.Trigger+q.preview.dueDate.Format("Jan 2"))
	}
	if q.preview.deferUntil != nil {
		deferDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDefer)
		scheduleChunks = append(scheduleChunks, deferDefinition.Trigger+formatDeferDate(*q.preview.deferUntil, time.Now()))
	}
	if len(scheduleChunks) > 0 {
		groups = append(groups, previewGroup{label: "Schedule", chunks: scheduleChunks})
	}

	if q.preview.priority != models.PriorityUnset {
		groups = append(groups, previewGroup{label: "Priority", chunks: []string{fmt.Sprintf("%s%d", priorityDefinition.Trigger, q.preview.priority)}})
	}
	if q.preview.pinned {
		groups = append(groups, previewGroup{label: "Pinned", chunks: []string{pinnedDefinition.Trigger + "pin"}})
	}
	// Attachments sit below the metadata groups and one chunk per entry, so the
	// preview states how many the line would create. Without this group a pasted
	// URL would vanish from the title with nothing said in its place.
	if len(q.preview.attachments) > 0 {
		attachmentDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldAttachment)
		attachmentChunks := make([]string, len(q.preview.attachments))
		for i, value := range q.preview.attachments {
			attachmentChunks[i] = attachmentDefinition.Trigger + value
		}
		groups = append(groups, previewGroup{label: "Attach", chunks: attachmentChunks})
	}
	// Comments sit beside attachments for the same reason: the span is peeled out
	// of the title, so without this group a comment whose closing delimiter is
	// still missing would be indistinguishable from one already recognized.
	if len(q.preview.comments) > 0 {
		commentDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldComment)
		commentChunks := make([]string, len(q.preview.comments))
		for i, text := range q.preview.comments {
			commentChunks[i] = commentDefinition.Trigger + text + commentDefinition.Trigger
		}
		groups = append(groups, previewGroup{label: "Comments", chunks: commentChunks})
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
// into labeled groups (Task / Route / Children / Schedule / Priority / Tags) so
// the user can scan what the parser understood without parsing a
// separator-soup. Only populated groups are shown.
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

// syntaxGuideGroups returns the grouped sigil reference in teach order, which is
// why it reads GuideDefinitions rather than the parse-order projection: the
// attachment sigil has to be peeled before #tag and would otherwise lead the
// guide. Project Routing is only included when %project capture is enabled, so
// the Linked Task Creator (which skips project syntax) never advertises it.
func (q *quickCaptureInput) syntaxGuideGroups() []syntaxGuideGroup {
	var groupOrder []string
	byGroup := make(map[string][]syntaxGuideEntry)
	for _, def := range textgrammar.GuideDefinitions(textgrammar.SurfaceCapture) {
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
