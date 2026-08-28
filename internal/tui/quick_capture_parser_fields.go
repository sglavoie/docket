package tui

import (
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/util"
)

func (p *quickCaptureParser) applySharedTokens(parsed textgrammar.Result) {
	p.input = parsed.Text

	p.parseRetiredTaskTypeTokens(parsed.TokensFor(textgrammar.FieldTaskType))
	if p.err != nil {
		return
	}
	p.parsePriorityTokens(parsed.TokensFor(textgrammar.FieldPriority))
	if p.err != nil {
		return
	}
	p.parseTagTokens(parsed.TokensFor(textgrammar.FieldTag))
	p.normalizeTagTokens()
	if p.err != nil {
		return
	}
	p.parseContextTokens(parsed.TokensFor(textgrammar.FieldContext))
	if p.err != nil {
		return
	}
	p.parseDueTokens(parsed.TokensFor(textgrammar.FieldDue))
	if p.err != nil {
		return
	}
	p.parseDeferTokens(parsed.TokensFor(textgrammar.FieldDefer))
	if p.err != nil {
		return
	}
	p.parseProjectTokens(parsed.TokensFor(textgrammar.FieldProject))
	if p.err != nil {
		return
	}
	// Dependencies resolve after the route because a bare alias is read in the
	// effective project, which %project is what settles.
	p.parseDependencyTokens(
		parsed.TokensFor(textgrammar.FieldDependency),
		parsed.TokensFor(textgrammar.FieldRemoveDependency),
	)
	if p.err != nil {
		return
	}
	p.parseParentTokens(parsed.TokensFor(textgrammar.FieldParent))
	if p.err != nil {
		return
	}
	p.parseChildTokens(parsed.TokensFor(textgrammar.FieldChild))
	if p.err != nil {
		return
	}
	p.parseDestinationTokens(parsed.TokensFor(textgrammar.FieldDestination))
	if p.err != nil {
		return
	}
	p.parsePinTokens(parsed.TokensFor(textgrammar.FieldPinned))
	if p.err != nil {
		return
	}
	p.parseAttachmentTokens(parsed.TokensFor(textgrammar.FieldAttachment))
	if p.err != nil {
		return
	}
	p.parseCommentTokens(parsed.TokensFor(textgrammar.FieldComment))
	if p.err != nil {
		return
	}
	p.parseTerminalAdoption()
}

// parseAttachmentTokens collects every &link reference in input order. The token
// is already lexically valid — the grammar leaves an unattachable value in the
// title rather than tokenizing it — and whether the value names a reachable file
// is the application layer's question, asked once through
// app.ResolveAttachmentInput so every surface classifies identically.
func (p *quickCaptureParser) parseAttachmentTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		p.result.attachments = append(p.result.attachments, token.Value)
	}
}

// parseCommentTokens collects every ;comment; span in input order. The grammar
// already guarantees a nonblank span — the pattern requires a non-space first
// character and a closing delimiter — and the application layer runs the same
// normalization an interactively typed comment goes through, so this stage only
// carries the text.
func (p *quickCaptureParser) parseCommentTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		p.result.comments = append(p.result.comments, token.Value)
	}
}

func (p *quickCaptureParser) parseContextTokens(tokens []textgrammar.Token) {
	definition, ok := textgrammar.DefinitionFor(p.surface, textgrammar.FieldContext)
	if !ok {
		p.failf("context grammar is unavailable")
		return
	}
	for _, token := range tokens {
		if token.Value == "0" {
			if p.surface == textgrammar.SurfaceOutline {
				p.failf("omit %s to clear the context here", definition.Forms[0])
				return
			}
			p.failf("%s is not available when capturing; a new task starts without a context", definition.Trigger+token.Value)
			return
		}
		context, err := models.NormalizeTaskContext(token.Value)
		if err != nil {
			if p.failf("invalid context %s%s: %s", definition.Trigger, token.Value, err) {
				return
			}
			continue
		}
		p.result.context = context
	}
}

// parsePinTokens reads the pin token both surfaces of this parser tokenize. The
// clear value reaches here on purpose: neither surface advertises it, so the
// refusal has to name the surface rather than let the word fold into the title.
func (p *quickCaptureParser) parsePinTokens(tokens []textgrammar.Token) {
	setValues := pinSetValues()
	for _, token := range tokens {
		if slices.Contains(setValues, token.Value) {
			p.result.pinned = true
			continue
		}
		if p.surface == textgrammar.SurfaceOutline {
			if p.failf("omit *pin to unpin here") {
				return
			}
			continue
		}
		if p.failf("*unpin is not available when capturing; a new task starts unpinned") {
			return
		}
	}
}

// pinSetValues is the advertised pin vocabulary, read from the registry so the
// accepted value stays owned by the definition instead of spelled out here.
func pinSetValues() []string {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPinned)
	if !ok {
		return nil
	}
	return definition.Values
}

func (p *quickCaptureParser) parseRetiredTaskTypeTokens(tokens []textgrammar.Token) {
	if len(tokens) == 0 {
		return
	}
	token := tokens[0]
	replacement := "use #tag instead"
	switch strings.ToLower(token.Value) {
	case "bug", "feature", "idea":
		replacement = "use #" + strings.ToLower(token.Value)
	}
	p.failf("task type ^%s was removed (%s)", token.Value, replacement)
}

func (p *quickCaptureParser) parsePriorityTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		values, ok := textgrammar.ParsePriority(token.Value, false)
		if !ok {
			if p.failf("%s", textgrammar.InvalidPriorityError(token.Value)) {
				return
			}
			continue
		}
		p.result.priority = values[0]
		p.result.prioritySet = true
	}
}

func (p *quickCaptureParser) parseTagTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		p.result.tags = append(p.result.tags, token.Value)
	}
}

func (p *quickCaptureParser) normalizeTagTokens() {
	tags, err := models.NormalizeTaskTags(p.result.tags)
	if err != nil {
		p.failf("invalid tags: %s", err)
		return
	}
	p.result.tags = tags
}

func (p *quickCaptureParser) parseDueTokens(tokens []textgrammar.Token) {
	if len(tokens) > 0 {
		p.parseDueToken(tokens[0], ParseDueDateValue)
	}
}

func (p *quickCaptureParser) parseDueToken(token textgrammar.Token, parse func(string) (time.Time, error)) {
	date, err := parse(token.Value)
	if err != nil {
		p.failf("invalid date: %s", token.Value)
		return
	}
	p.result.dueDate = &date
}

// parseDeferTokens reads the deferral token. The clear value reaches here on
// purpose: capture does not advertise it, so the refusal names the surface
// rather than letting @@0 fold into the title.
func (p *quickCaptureParser) parseDeferTokens(tokens []textgrammar.Token) {
	definition, ok := textgrammar.DefinitionFor(p.surface, textgrammar.FieldDefer)
	if !ok {
		return
	}
	for _, token := range tokens {
		if token.Value == "0" {
			p.failf("%s%s is not available when capturing; a new task starts without a deferral", definition.Trigger, token.Value)
			return
		}
		instant, err := util.ParseDeferInstant(token.Value)
		if err != nil {
			if p.failf("invalid defer: %s", token.Value) {
				return
			}
			continue
		}
		p.result.deferSet = true
		p.result.deferUntil = &instant
	}
}

// parseDependencyTokens collects every ~task reference in input order. Removal
// and clearing reach this parser on purpose: capture tokenizes both so the
// refusal can name the operation instead of letting -~12 or ~0 fold into the
// title, and a task that does not exist yet has nothing to remove or clear.
//
// A dependency is a soft reference, so an addition asks only that the target be
// a real, still-open task the workspace loaded; nothing about the new task's
// readiness or ordering follows from it.
func (p *quickCaptureParser) parseDependencyTokens(add, remove []textgrammar.Token) {
	definition, ok := textgrammar.DefinitionFor(p.surface, textgrammar.FieldDependency)
	if !ok {
		p.failf("dependency grammar is unavailable")
		return
	}
	// The removal form is read from the edit surface because capture tokenizes it
	// without teaching it, so capture advertises no definition to name it with.
	removeDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldRemoveDependency)
	if p.surface == textgrammar.SurfaceOutline {
		p.parseOutlineDependencyTokens(add, remove, definition, removeDefinition)
		return
	}
	if len(remove) > 0 {
		p.failf("%stask is not available when capturing; a new task has no dependencies to remove", removeDefinition.Trigger)
		return
	}
	for _, token := range add {
		if token.Value == dependencyClearValue {
			p.failf("%s%s is not available when capturing; a new task starts without dependencies", definition.Trigger, token.Value)
			return
		}
		ref, err := resolveDependencyReference(token.Value, p.project, p.allProjects)
		if err != nil {
			if p.failf("%s", err) {
				return
			}
			continue
		}
		if !ref.resolved {
			if p.failf("%s", dependencyNotFoundMessage(definition.Trigger, token.Value)) {
				return
			}
			continue
		}
		if ref.terminal {
			if p.failf("cannot depend on %s: it is already closed", ref.label()) {
				return
			}
			continue
		}
		p.result.dependencies = appendDependencyRef(p.result.dependencies, ref)
	}
}

// parseOutlineDependencyTokens reads the outline buffer's declarative dependency
// set. The line is a complete statement, so the positive tokens are the final
// set: deleting a dumped ~task is what removes that dependency, -~task subtracts
// from the same line's positives for anyone who would rather strike a reference
// than delete it, and ~0 states the empty set.
//
// Nothing here judges whether a target is closed or loaded. The dump prints
// terminal and unavailable references so they can round-trip, and only the
// application layer knows which locators the task already stored — so it, not
// this parser, decides that an existing reference may stay while a newly typed
// one must name a loaded open task.
func (p *quickCaptureParser) parseOutlineDependencyTokens(add, remove []textgrammar.Token, definition, removeDefinition textgrammar.Definition) {
	clears := 0
	for _, token := range add {
		if token.Value == dependencyClearValue {
			clears++
		}
	}
	if clears > 0 {
		if clears != len(add) || len(remove) > 0 {
			p.failf("%s%s is the empty set and cannot be combined with adding or removing dependencies", definition.Trigger, dependencyClearValue)
		}
		return
	}

	removed := make([]app.TaskLocator, 0, len(remove))
	for _, token := range remove {
		if token.Value == dependencyClearValue {
			p.failf("%s%s is not a clear form; use %s%s to clear every dependency", removeDefinition.Trigger, token.Value, definition.Trigger, dependencyClearValue)
			return
		}
		ref, ok := p.resolveOutlineDependency(token, removeDefinition.Trigger)
		if !ok {
			return
		}
		removed = append(removed, ref.locator)
	}
	for _, token := range add {
		ref, ok := p.resolveOutlineDependency(token, definition.Trigger)
		if !ok {
			return
		}
		if slices.Contains(removed, ref.locator) {
			continue
		}
		p.result.dependencies = appendDependencyRef(p.result.dependencies, ref)
	}
}

// resolveOutlineDependency addresses one outline token. A locator the workspace
// cannot resolve is still accepted when it is well-shaped, which is what lets the
// project-id:task-id fallback the dump printed for an unavailable target come
// back in unchanged.
func (p *quickCaptureParser) resolveOutlineDependency(token textgrammar.Token, trigger string) (capturedDependencyRef, bool) {
	ref, err := resolveDependencyReference(token.Value, p.project, p.allProjects)
	if err != nil {
		p.failf("%s", err)
		return capturedDependencyRef{}, false
	}
	if ref.locator.TaskID == "" {
		p.failf("%s", dependencyNotFoundMessage(trigger, token.Value))
		return capturedDependencyRef{}, false
	}
	return ref, true
}

func (p *quickCaptureParser) parseProjectTokens(tokens []textgrammar.Token) {
	if p.skipProject {
		return
	}
	for _, token := range tokens {
		targetProject := resolveProject(token.Value, p.allProjects)
		if targetProject == nil {
			if p.failf("project not found: '%s' (use an exact %%project-slug)", token.Value) {
				return
			}
			continue
		}
		p.result.targetProjectID = &targetProject.ID
		p.result.targetProjectName = targetProject.Name
		p.project = targetProject
	}
}

func (p *quickCaptureParser) parseParentTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		parent, err := resolveParent(token.Value, p.project)
		if err != nil {
			if p.failf("%s", err) {
				return
			}
			continue
		}
		switch parent {
		case nil:
			if p.failf("parent not found: '%s' (use >short-id or title prefix)", token.Value) {
				return
			}
		default:
			p.result.parentID = &parent.ID
		}
	}
}

// parseChildTokens resolves every <child reference against the effective project,
// which parseProjectTokens has already settled, and against the parent
// parseParentTokens has already settled. Children are additive and repeatable, so
// aliases naming one task collapse to their first occurrence instead of failing.
func (p *quickCaptureParser) parseChildTokens(tokens []textgrammar.Token) {
	seen := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		child, err := resolveChild(token.Value, p.project)
		if err != nil {
			if p.failf("%s", err) {
				return
			}
			continue
		}
		if child == nil {
			if p.failf("child not found: '%s' (use <short-id or title prefix)", token.Value) {
				return
			}
			continue
		}
		if p.result.parentID != nil && *p.result.parentID == child.ID {
			if p.failf("#%d cannot be both the parent and a child", child.ShortID) {
				return
			}
			continue
		}
		if seen[child.ID] {
			continue
		}
		seen[child.ID] = true
		p.result.children = append(p.result.children, capturedChildRef{
			id:       child.ID,
			shortID:  child.ShortID,
			title:    child.Title,
			terminal: child.IsTerminalState(),
		})
	}
}

// parseTerminalAdoption applies the terminal-adoption policy to the draft, so the
// composer refuses the line the application layer would refuse instead of
// previewing a Done task above open work. It runs after the destination is
// settled because the destination is what makes an adoption illegal.
func (p *quickCaptureParser) parseTerminalAdoption() {
	if !p.result.destinationSet || !p.result.destination.WorkflowState().IsTerminal() {
		return
	}
	for _, child := range p.result.children {
		if child.terminal {
			continue
		}
		if p.failf("cannot capture as %s: adopted #%d is incomplete", p.result.destination, child.shortID) {
			return
		}
	}
}

func (p *quickCaptureParser) parseDestinationTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		destination, ok := app.ParseCaptureDestination(token.Value)
		if !ok {
			err := app.InvalidCaptureDestinationError(token.Value)
			p.failf("%s", err)
			return
		}
		p.result.destination = destination
		p.result.destinationSet = true
	}
}
