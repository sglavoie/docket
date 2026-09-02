package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

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
	case acModeReview:
		items = q.getReviewSuggestions(partial)
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
	case acModeReview:
		return textgrammar.FieldReview, true
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
