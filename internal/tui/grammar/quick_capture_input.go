package grammar

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/cursor"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// SetAllProjects replaces the projects a line may route to, for a composer
// that stays open across a commit which created one, and rebuilds the caches
// that read them.
func (q *QuickCaptureInput) SetAllProjects(projects []*models.Project) {
	q.allProjects = projects
	q.InitCaches()
}

// InitCaches builds tag, context, and project caches for autocomplete. Parent
// and child candidates are deliberately absent: they follow the draft's effective
// %project, which changes while the user types, so caching them here is what
// made parent completion offer the source project's tasks after a route.
func (q *QuickCaptureInput) InitCaches() {
	// Cache non-archived projects for cross-project capture
	q.projectCache = make([]*models.Project, 0)
	if q.allProjects != nil {
		for _, p := range q.allProjects {
			if !p.Archived {
				q.projectCache = append(q.projectCache, p)
			}
		}
	}

	// Tags and contexts come from every project a line may route to, not only
	// the source one: a global capture starts in Unfiled, which would otherwise offer
	// almost nothing. The source project is included even when no list was
	// handed in, so a host without one still completes locally.
	projects := append([]*models.Project{q.Project}, q.projectCache...)
	q.TagCache = collectTagNames(projects...)
	q.contextCache = collectContextNames(projects...)
}

// ensureSyntaxViewport lazily constructs the syntax help viewport so callers can
// scroll or measure it before it has been sized by a render pass.
func (q *QuickCaptureInput) ensureSyntaxViewport() {
	if !q.syntaxReady {
		q.SyntaxVP = viewport.New(viewport.WithWidth(max(q.Width, 1)), viewport.WithHeight(1))
		q.syntaxReady = true
	}
}

// SetSyntaxViewportWidth resizes the guide viewport to the composer width and,
// when help is open, re-renders the width-measured guide and clamps the offset
// so a resize can never leave the viewport scrolled past its (possibly shorter)
// new content.
func (q *QuickCaptureInput) SetSyntaxViewportWidth(width int) {
	q.ensureSyntaxViewport()
	q.SyntaxVP.SetWidth(max(width, 1))
	if q.HelpExpanded {
		q.refreshSyntaxGuide()
	}
}

// SetSyntaxViewportHeight sets the guide viewport height (the bounded lower-pane
// budget) and re-clamps the offset against the new visible height.
func (q *QuickCaptureInput) SetSyntaxViewportHeight(height int) {
	q.ensureSyntaxViewport()
	q.SyntaxVP.SetHeight(max(height, 1))
	q.positionSyntaxAnchor()
}

// clampSyntaxOffset re-applies the current offset, which the viewport clamps to
// its valid range. Used after resize so a shrunk guide or pane cannot leave a
// stale offset scrolled past the end.
func (q *QuickCaptureInput) clampSyntaxOffset() {
	q.SyntaxVP.SetYOffset(q.SyntaxVP.YOffset())
}

// openHelp opens the complete guide at the current token's group, or its top.
func (q *QuickCaptureInput) openHelp() {
	q.HelpExpanded = true
	q.ensureSyntaxViewport()
	q.syntaxAnchor = q.syntaxGroupAtCaret()
	q.SyntaxVP.GotoTop()
	q.refreshSyntaxGuide()
}

// CloseHelp collapses the syntax guide. The offset is retained on the viewport
// so re-opening via ToggleHelp can choose a fresh context,
// while autocomplete-driven hide/show preserves the mid-scroll position.
func (q *QuickCaptureInput) CloseHelp() {
	q.HelpExpanded = false
}

// ToggleHelp opens contextual syntax help when closed, or closes it when open.
func (q *QuickCaptureInput) ToggleHelp() {
	if q.HelpExpanded {
		q.CloseHelp()
		return
	}
	q.openHelp()
}

// SyntaxScrollLine scrolls the guide by delta lines (negative = up).
func (q *QuickCaptureInput) SyntaxScrollLine(delta int) {
	q.syntaxAnchor = ""
	q.ensureSyntaxViewport()
	if delta < 0 {
		q.SyntaxVP.ScrollUp(-delta)
	} else {
		q.SyntaxVP.ScrollDown(delta)
	}
}

// SyntaxScrollPage scrolls the guide by delta full pages (negative = up).
func (q *QuickCaptureInput) SyntaxScrollPage(delta int) {
	q.syntaxAnchor = ""
	q.ensureSyntaxViewport()
	if delta < 0 {
		q.SyntaxVP.PageUp()
	} else {
		q.SyntaxVP.PageDown()
	}
}

// SyntaxOverflows reports whether the rendered guide is taller than its visible
// viewport, i.e. scrolling is meaningful. Footer scroll hints and the scroll
// percentage are shown only in this case.
func (q *QuickCaptureInput) SyntaxOverflows() bool {
	return q.syntaxReady && q.SyntaxVP.TotalLineCount() > q.SyntaxVP.Height()
}

// SyntaxScrollPercent returns the scroll position as a 0-100 percentage, clamped
// so a non-overflowing guide reports fully scrolled (100%).
func (q *QuickCaptureInput) SyntaxScrollPercent() int {
	if !q.SyntaxOverflows() {
		return 100
	}
	pct := int(q.SyntaxVP.ScrollPercent() * 100)
	return max(0, min(pct, 100))
}

// HandleAutocompleteKey handles keys when autocomplete is active.
// Returns true if the key was consumed.
func (q *QuickCaptureInput) HandleAutocompleteKey(key string) bool {
	return q.AC.HandleKey(key, func() {
		if q.acceptSuggestion() {
			q.UpdatePreview()
		}
	})
}

// AutocompleteLiteralComplete reports whether the active token is already
// complete as typed: a tag or context literal the grammar accepts, or any token
// that spells one of the offered values exactly. The composer uses this to let
// Enter submit a complete token while Tab remains the explicit completion key.
func (q *QuickCaptureInput) AutocompleteLiteralComplete() bool {
	if !q.AC.IsActive() {
		return false
	}
	mode, _, partial := DetectTrigger(q.Input.Value()[:q.cursorByteOffset()], q.Triggers)
	if mode != q.AC.Mode || partial == "" {
		return false
	}
	return q.AC.OffersExactly(partial) || literalTokenComplete(mode, partial, textgrammar.SurfaceCapture)
}

// literalTokenComplete reports whether a tag or context token parses whole as
// typed. Both mint on the fly, so a literal the grammar accepts needs no
// suggestion to be complete; every other field completes only to an offered
// value.
func literalTokenComplete(mode int, partial string, surface textgrammar.Surface) bool {
	field := textgrammar.FieldTag
	switch mode {
	case ACModeTag:
	case ACModeContext:
		field = textgrammar.FieldContext
	default:
		return false
	}
	definition, ok := textgrammar.DefinitionFor(surface, field)
	if !ok {
		return false
	}
	parsed := textgrammar.Parse(definition.Trigger+partial, surface)
	return parsed.Text == "" && len(parsed.TokensFor(field)) == 1
}

// UpdateInput updates the textarea and refreshes autocomplete/preview state.
// Suggestions are re-derived only when the draft text or caret changed: the textarea
// also receives cursor blinks and other non-editing messages, and refreshing
// on those would reopen a dropdown Esc just closed over the same token.
func (q *QuickCaptureInput) UpdateInput(msg tea.Msg) tea.Cmd {
	before := q.Input.Value()
	beforeCursor := q.cursorByteOffset()
	var cmd tea.Cmd
	q.Input, cmd = q.Input.Update(msg)

	changed := q.Input.Value() != before || q.cursorByteOffset() != beforeCursor
	if changed {
		q.err = nil
	}
	// Blink messages do not change the draft or workspace. Still refresh on a
	// new minute for relative dates; other messages may carry refreshed data.
	_, blink := msg.(cursor.BlinkMsg)
	if changed || !blink || !q.previewMinute.Equal(time.Now().Truncate(time.Minute)) {
		q.UpdatePreview()
	}
	if changed {
		q.RefreshAutocomplete()
	}

	return cmd
}

// RefreshAutocomplete detects triggers and updates suggestion list.
func (q *QuickCaptureInput) RefreshAutocomplete() {
	value := q.Input.Value()
	mode, pos, partial := DetectTrigger(value[:q.cursorByteOffset()], q.Triggers)
	if mode == ACModeNone {
		q.AC.Clear()
		return
	}

	lineStart := strings.LastIndex(value[:pos], "\n") + 1
	lineEnd := len(value)
	if end := strings.IndexByte(value[pos:], '\n'); end >= 0 {
		lineEnd = pos + end
	}
	// Read the complete logical line, excluding the token being completed.
	// Routing and already-used attributes may appear on either side of it.
	parsedLine := textgrammar.Parse(value[lineStart:pos]+value[q.captureTokenEnd():lineEnd], textgrammar.SurfaceCapture)
	if field, singleton := autocompleteSingletonField(mode); singleton && len(parsedLine.TokensFor(field)) > 0 {
		q.AC.Clear()
		return
	}

	var items []ACSuggestion
	switch mode {
	case ACModeTag:
		items = q.getTagSuggestions(partial)
		items = FilterUsedTagSuggestions(items, parsedLine.TokensFor(textgrammar.FieldTag))
	case ACModeContext:
		items = q.GetContextSuggestions(partial)
	case acModeParent:
		items = q.getParentSuggestions(partial, StructuralCandidates(q.effectiveProject(parsedLine)))
	case ACModeChild:
		effective := q.effectiveProject(parsedLine)
		candidates := filterAdoptedChildCandidates(
			StructuralCandidates(effective), effective, parsedLine.TokensFor(textgrammar.FieldChild),
		)
		items = q.getChildSuggestions(partial, candidates)
	case acModePriority:
		items = q.getPrioritySuggestions(partial)
	case acModeDate:
		items = q.getDateSuggestions(partial)
	case acModeDefer:
		items = q.getDeferSuggestions(partial)
	case ACModeDestination:
		items = q.GetDestinationSuggestions(partial)
	case acModeProject:
		items = q.getProjectSuggestions(partial)
	case acModePin:
		items = q.getPinSuggestions(partial)
	case acModeReview:
		items = q.getReviewSuggestions(partial)
	case ACModeDependency:
		items = q.getDependencySuggestions(partial, parsedLine)
	}
	q.AC.UpdateSuggestions(mode, pos, items)
}

func autocompleteSingletonField(mode int) (textgrammar.Field, bool) {
	switch mode {
	case acModeParent:
		return textgrammar.FieldParent, true
	case ACModeContext:
		return textgrammar.FieldContext, true
	case acModePriority:
		return textgrammar.FieldPriority, true
	case acModeDate:
		return textgrammar.FieldDue, true
	case acModeDefer:
		return textgrammar.FieldDefer, true
	case ACModeDestination:
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
func (q *QuickCaptureInput) effectiveProject(draftedLine textgrammar.Result) *models.Project {
	if q.skipProjectSyntax {
		return q.Project
	}
	tokens := draftedLine.TokensFor(textgrammar.FieldProject)
	if len(tokens) == 0 {
		return q.Project
	}
	// The parser lets the last %project token win, so completion must too.
	if target := capturetext.ResolveProject(tokens[len(tokens)-1].Value, q.allProjects); target != nil {
		return target
	}
	return q.Project
}

// StructuralCandidates returns the tasks a >parent or <child completion offers.
// Completed and effectively hidden tasks are withheld from the suggestion list
// only; a manually typed reference still resolves against every task in the
// project, so an intentional reference to finished or hidden work is never
// blocked by this policy.
func StructuralCandidates(project *models.Project) []*models.Task {
	if project == nil {
		return nil
	}
	candidates := make([]*models.Task, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil && !task.IsComplete() && !models.TaskEffectivelyHidden(project, task) {
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
		task, err := capturetext.ResolveChild(token.Value, project)
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

func FilterUsedTagSuggestions(items []ACSuggestion, tokens []textgrammar.Token) []ACSuggestion {
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
