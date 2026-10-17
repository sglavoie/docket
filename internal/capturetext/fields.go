package capturetext

import (
	"errors"
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

	p.parseDuplicateSingletons(parsed)
	if p.err != nil {
		return
	}
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
	p.parseReviewTokens(parsed.TokensFor(textgrammar.FieldReview))
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
		p.result.Attachments = append(p.result.Attachments, token.Value)
	}
}

// parseCommentTokens collects every ;comment; span in input order. The grammar
// already guarantees a nonblank span — the pattern requires a non-space first
// character and a closing delimiter — and the application layer runs the same
// normalization an interactively typed comment goes through, so this stage only
// carries the text.
func (p *quickCaptureParser) parseCommentTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		p.result.Comments = append(p.result.Comments, token.Value)
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
				p.failTokenf(token, "omit %s to clear the context here", definition.Forms[0])
				return
			}
			p.failTokenf(token, "%s is not available when capturing; a new task starts without a context", definition.Trigger+token.Value)
			return
		}
		context, err := models.NormalizeTaskContext(token.Value)
		if err != nil {
			if p.failTokenf(token, "invalid context %s%s: %s", definition.Trigger, token.Value, err) {
				return
			}
			continue
		}
		p.result.Context = context
	}
}

// parsePinTokens reads the pin token both surfaces of this parser tokenize. The
// clear value reaches here on purpose: neither surface advertises it, so the
// refusal has to name the surface rather than let the word fold into the title.
func (p *quickCaptureParser) parsePinTokens(tokens []textgrammar.Token) {
	setValues := PinSetValues()
	for _, token := range tokens {
		if slices.Contains(setValues, token.Value) {
			p.result.Pinned = true
			continue
		}
		if p.surface == textgrammar.SurfaceOutline {
			if p.failTokenf(token, "omit *pin to unpin here") {
				return
			}
			continue
		}
		if p.failTokenf(token, "*unpin is not available when capturing; a new task starts unpinned") {
			return
		}
	}
}

// PinSetValues is the advertised pin vocabulary, read from the registry so the
// accepted value stays owned by the definition instead of spelled out here.
func PinSetValues() []string {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPinned)
	if !ok {
		return nil
	}
	return definition.Values
}

// parseReviewTokens reads the review token, refusing the clear value on the same
// two surfaces and for the same reasons the pin one does: the outline buffer
// clears by omission, and a captured task starts unflagged.
func (p *quickCaptureParser) parseReviewTokens(tokens []textgrammar.Token) {
	setValues := ReviewSetValues()
	for _, token := range tokens {
		if slices.Contains(setValues, token.Value) {
			p.result.Review = true
			continue
		}
		if p.surface == textgrammar.SurfaceOutline {
			if p.failTokenf(token, "omit =review to clear the review flag here") {
				return
			}
			continue
		}
		if p.failTokenf(token, "=unreview is not available when capturing; a new task starts unflagged") {
			return
		}
	}
}

// ReviewSetValues is the advertised review vocabulary, read from the registry
// for the same reason PinSetValues is.
func ReviewSetValues() []string {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldReview)
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
	p.failTokenf(token, "task type ^%s was removed (%s)", token.Value, replacement)
}

func (p *quickCaptureParser) parsePriorityTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		values, ok := textgrammar.ParsePriority(token.Value, false)
		if !ok {
			if p.failTokenf(token, "%s", textgrammar.InvalidPriorityError(token.Value)) {
				return
			}
			continue
		}
		p.result.Priority = values[0]
		p.result.PrioritySet = true
	}
}

func (p *quickCaptureParser) parseTagTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		p.result.Tags = append(p.result.Tags, token.Value)
	}
}

func (p *quickCaptureParser) normalizeTagTokens() {
	tags, err := models.NormalizeTaskTags(p.result.Tags)
	if err != nil {
		p.failf("invalid tags: %s", err)
		return
	}
	p.result.Tags = tags
}

func (p *quickCaptureParser) parseDueTokens(tokens []textgrammar.Token) {
	if len(tokens) > 0 {
		p.parseDueToken(tokens[0], ParseDueDateValue)
	}
}

func (p *quickCaptureParser) parseDueToken(token textgrammar.Token, parse func(string) (time.Time, error)) {
	date, err := parse(token.Value)
	if err != nil {
		p.failTokenf(token, "%s", textgrammar.InvalidScheduleError(textgrammar.FieldDue, token.Value))
		return
	}
	p.result.DueDate = &date
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
			p.failTokenf(token, "%s%s is not available when capturing; a new task starts without a deferral", definition.Trigger, token.Value)
			return
		}
		instant, err := util.ParseDeferInstant(token.Value)
		if err != nil {
			if p.failTokenf(token, "%s", textgrammar.InvalidScheduleError(textgrammar.FieldDefer, token.Value)) {
				return
			}
			continue
		}
		p.result.DeferSet = true
		p.result.DeferUntil = &instant
	}
}

func (p *quickCaptureParser) parseProjectTokens(tokens []textgrammar.Token) {
	if p.skipProject {
		return
	}
	for _, token := range tokens {
		target := ResolveProject(token.Value, p.allProjects)
		if target == nil {
			hint := ProjectSlugHint(token.Value, p.allProjects)
			if p.failTokenf(token, "project not found: '%s' (%s)", token.Value, hint) {
				var tokenErr *CaptureTokenError
				if errors.As(p.err, &tokenErr) {
					tokenErr.UnknownProject = token.Value
				}
				return
			}
			continue
		}
		p.result.TargetProjectID = &target.ID
		p.result.TargetProjectName = target.Name
		p.result.TargetProject = target
		p.project = target
	}
}

func (p *quickCaptureParser) parseParentTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		parent, err := ResolveParent(token.Value, p.project)
		if err != nil {
			if p.failTokenf(token, "%s", err) {
				return
			}
			continue
		}
		switch parent {
		case nil:
			if p.failTokenf(token, "parent not found: '%s' (use >short-id or title prefix)", token.Value) {
				return
			}
		default:
			p.result.ParentID = &parent.ID
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
		child, err := ResolveChild(token.Value, p.project)
		if err != nil {
			if p.failTokenf(token, "%s", err) {
				return
			}
			continue
		}
		if child == nil {
			if p.failTokenf(token, "child not found: '%s' (use <short-id or title prefix)", token.Value) {
				return
			}
			continue
		}
		if p.result.ParentID != nil && *p.result.ParentID == child.ID {
			if p.failTokenf(token, "#%d cannot be both the parent and a child", child.ShortID) {
				return
			}
			continue
		}
		if seen[child.ID] {
			continue
		}
		seen[child.ID] = true
		p.result.Children = append(p.result.Children, CapturedChildRef{
			Id:       child.ID,
			ShortID:  child.ShortID,
			Title:    child.Title,
			Terminal: child.IsTerminalState(),
		})
	}
}

// parseTerminalAdoption applies the terminal-adoption policy to the draft, so the
// composer refuses the line the application layer would refuse instead of
// previewing a Done task above open work. It runs after the destination is
// settled because the destination is what makes an adoption illegal.
func (p *quickCaptureParser) parseTerminalAdoption() {
	if !p.result.DestinationSet || !p.result.Destination.WorkflowState().IsTerminal() {
		return
	}
	for _, child := range p.result.Children {
		if child.Terminal {
			continue
		}
		if p.failf("cannot capture as %s: adopted #%d is incomplete", p.result.Destination, child.ShortID) {
			return
		}
	}
}

func (p *quickCaptureParser) parseDestinationTokens(tokens []textgrammar.Token) {
	for _, token := range tokens {
		destination, ok := app.ParseCaptureDestination(token.Value)
		if !ok {
			err := app.InvalidCaptureDestinationError(token.Value)
			p.failTokenf(token, "%s", err)
			return
		}
		p.result.Destination = destination
		p.result.DestinationSet = true
	}
}

// parseDuplicateSingletons refuses a repeated singleton such as `@fri @mon` or
// `!1 !2`. The grammar already diagnoses it; without this stage the field
// parsers would each keep one of the values and the other would vanish from the
// draft, so the refusal names the later token, where the conflict begins.
func (p *quickCaptureParser) parseDuplicateSingletons(parsed textgrammar.Result) {
	for _, diagnostic := range parsed.Diagnostics {
		tokens := parsed.TokensFor(diagnostic.Field)
		if len(tokens) < 2 {
			if p.failf("%s", diagnostic.Message) {
				return
			}
			continue
		}
		if p.failTokenf(tokens[1], "%s", diagnostic.Message) {
			return
		}
	}
}
