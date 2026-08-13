package tui

import (
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
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
	p.parseDueTokens(parsed.TokensFor(textgrammar.FieldDue))
	if p.err != nil {
		return
	}
	p.parseRetiredRecurrenceTokens(parsed.TokensFor(textgrammar.FieldRecurrence))
	if p.err != nil {
		return
	}
	p.parseProjectTokens(parsed.TokensFor(textgrammar.FieldProject))
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
	p.parseTerminalAdoption()
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
		p.parseDueToken(tokens[0], ParseDate)
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

func (p *quickCaptureParser) parseRetiredRecurrenceTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		p.failf("recurrence ~%s was removed (use @date for a due date)", token.Value)
		return
	}
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
