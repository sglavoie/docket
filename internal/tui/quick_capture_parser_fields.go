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
	p.parseDestinationTokens(parsed.TokensFor(textgrammar.FieldDestination))
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
			if p.failf("priority !%s was removed (use !1, !2, or !3)", token.Value) {
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
			if p.failf("project not found: '%s' (use %%project-slug or name)", token.Value) {
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
		parent := resolveParent(token.Value, p.project)
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

func aliasFirst(tokens []textgrammar.Token, primaryForm string) []textgrammar.Token {
	result := make([]textgrammar.Token, 0, len(tokens))
	for _, token := range tokens {
		if token.Form != primaryForm {
			result = append(result, token)
		}
	}
	for _, token := range tokens {
		if token.Form == primaryForm {
			result = append(result, token)
		}
	}
	return result
}
