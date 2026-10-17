package grammar

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/util"
)

type quickEditParser struct {
	parsed           textgrammar.Result
	result           *QuickEditResult
	task             *models.Task
	project          *models.Project
	allProjects      []*models.Project
	effectiveProject *models.Project
	strict           bool
	errMsg           string
}

func (p *quickEditParser) setError(message string) bool {
	if p.errMsg == "" {
		p.errMsg = message
	}
	return p.strict
}

func (p *quickEditParser) applyDiagnostics() bool {
	for _, diagnostic := range p.parsed.Diagnostics {
		if p.setError(diagnostic.Message) {
			return true
		}
	}
	return false
}

func (p *quickEditParser) applyPriority() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldPriority)
	if len(tokens) == 0 {
		return false
	}
	token := tokens[0]
	if token.Value == "0" {
		p.result.Priority = models.PriorityUnset
		p.result.PrioritySet = true
		return false
	}
	values, ok := textgrammar.ParsePriority(token.Value, false)
	if !ok {
		return p.setError(textgrammar.InvalidPriorityError(token.Value).Error())
	}
	p.result.Priority = values[0]
	p.result.PrioritySet = true
	return false
}

func (p *quickEditParser) applyTags() bool {
	for _, token := range p.parsed.TokensFor(textgrammar.FieldRemoveTag) {
		tag, err := models.NormalizeTaskTag(token.Value)
		if err != nil {
			if p.setError(fmt.Sprintf("invalid tag -#%s: %s", token.Value, err)) {
				return true
			}
			continue
		}
		p.result.TagsToRemove = appendUnique(p.result.TagsToRemove, tag)
	}
	for _, token := range p.parsed.TokensFor(textgrammar.FieldTag) {
		tag, err := models.NormalizeTaskTag(token.Value)
		if err != nil {
			if p.setError(fmt.Sprintf("invalid tag #%s: %s", token.Value, err)) {
				return true
			}
			continue
		}
		if slices.Contains(p.result.TagsToRemove, tag) {
			if p.setError(fmt.Sprintf("conflicting tag patch: #%s and -#%s", tag, tag)) {
				return true
			}
			continue
		}
		p.result.TagsToAdd = appendUnique(p.result.TagsToAdd, tag)
	}
	return false
}

func (p *quickEditParser) applyDueDate() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldDue)
	if len(tokens) == 0 {
		return false
	}
	token := tokens[0]
	p.result.DueDateSet = true
	if token.Value == "0" {
		p.result.DueDateClear = true
		return false
	}
	date, err := capturetext.ParseDueDateValue(token.Value)
	if err != nil {
		return p.setError(textgrammar.InvalidScheduleError(textgrammar.FieldDue, token.Value).Error())
	}
	p.result.DueDate = &date
	return false
}

// applyDefer runs after applyDestination because a deferral carries a lifecycle
// effect of its own: it refuses a terminal task and otherwise claims Backlog,
// which is the same route the defer picker and the application layer take. The
// state it judges is the one this edit produces, so a line that reopens and
// defers together is measured on its result.
func (p *quickEditParser) applyDefer() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldDefer)
	if len(tokens) == 0 {
		return false
	}
	token := tokens[0]
	p.result.DeferSet = true
	if token.Value == "0" {
		p.result.DeferClear = true
		return false
	}
	instant, err := util.ParseDeferInstant(token.Value)
	if err != nil {
		return p.setError(textgrammar.InvalidScheduleError(textgrammar.FieldDefer, token.Value).Error())
	}
	p.result.DeferUntil = &instant

	state := models.StateBacklog
	switch {
	case p.result.DestinationSet:
		state = p.result.Destination.WorkflowState()
	case p.task != nil:
		state = p.task.State()
	}
	if refusal := app.DeferralRefusesState(state); refusal != "" {
		return p.setError(refusal)
	}
	p.result.Destination = textgrammar.DestinationBacklog
	p.result.DestinationSet = true
	p.result.State = models.StateBacklog
	p.result.StateSet = true
	return false
}

func (p *quickEditParser) applyDestination() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldDestination)
	if len(tokens) == 0 {
		return false
	}
	destination, ok := textgrammar.ParseDestination(tokens[0].Value)
	if !ok {
		return p.setError(textgrammar.InvalidDestinationError(tokens[0].Value).Error())
	}
	p.result.Destination = destination
	p.result.DestinationSet = true
	p.result.State = destination.WorkflowState()
	p.result.StateSet = true
	return false
}

// applyPin reads the one pin token this surface tokenizes. The vocabulary is
// closed and already validated, so the advertised set value is pinning and the
// only other possibility is the clear value; there is nothing left to reject,
// which is why it is the one apply step that cannot fail the parse.
func (p *quickEditParser) applyPin() {
	tokens := p.parsed.TokensFor(textgrammar.FieldPinned)
	if len(tokens) == 0 {
		return
	}
	p.result.Pinned = slices.Contains(capturetext.PinSetValues(), tokens[0].Value)
	p.result.PinnedSet = true
}

// applyReview reads the one review token this surface tokenizes. Like the pin
// step, the vocabulary is closed and already validated, so the advertised set
// value flags the task and the only other possibility is the clear value.
func (p *quickEditParser) applyReview() {
	tokens := p.parsed.TokensFor(textgrammar.FieldReview)
	if len(tokens) == 0 {
		return
	}
	p.result.Review = slices.Contains(capturetext.ReviewSetValues(), tokens[0].Value)
	p.result.ReviewSet = true
}

func (p *quickEditParser) applyProject() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldProject)
	if len(tokens) == 0 {
		return false
	}
	target := capturetext.ResolveProject(tokens[0].Value, p.allProjects)
	if target == nil {
		return p.setError(fmt.Sprintf("project not found: %q (%s)", tokens[0].Value, capturetext.ProjectSlugHint(tokens[0].Value, p.allProjects)))
	}
	p.result.TargetProject = target
	p.effectiveProject = target
	// A project move without an explicit parent always lands at root.
	p.result.ParentMode = QuickEditParentRoot
	return false
}

func (p *quickEditParser) applyParent() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldParent)
	if len(tokens) == 0 {
		return false
	}
	token := tokens[0]
	if token.Value == "0" {
		p.result.ParentMode = QuickEditParentRoot
		return false
	}
	parent, err := capturetext.ResolveParent(token.Value, p.effectiveProject)
	if err != nil {
		return p.setError(err.Error())
	}
	if parent == nil {
		return p.setError(fmt.Sprintf("parent not found: %q in project %q", token.Value, ProjectLabel(p.effectiveProject)))
	}
	if reason := InvalidQuickEditParent(p.task, p.project, p.effectiveProject, parent); reason != "" {
		return p.setError(reason)
	}
	p.result.ParentMode = QuickEditParentSet
	p.result.Parent = parent
	return false
}

// applyChildren resolves every <child reference against the effective project,
// which applyProject has already settled, and against the parent patch
// applyParent has already settled. Children are additive and repeatable, so
// aliases naming one task collapse to their first occurrence. Cycles and depth
// are left to the combined graph check, which measures them on the hierarchy the
// whole draft produces rather than on any intermediate one.
func (p *quickEditParser) applyChildren() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldChild)
	if len(tokens) == 0 || p.task == nil {
		return false
	}
	seen := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		child, err := capturetext.ResolveChild(token.Value, p.effectiveProject)
		if err != nil {
			if p.setError(err.Error()) {
				return true
			}
			continue
		}
		if child == nil {
			if p.setError(fmt.Sprintf("child not found: %q in project %q", token.Value, ProjectLabel(p.effectiveProject))) {
				return true
			}
			continue
		}
		if child.ID == p.task.ID {
			if p.setError("task cannot be its own child") {
				return true
			}
			continue
		}
		if p.result.ParentMode == QuickEditParentSet && p.result.Parent != nil && p.result.Parent.ID == child.ID {
			if p.setError(fmt.Sprintf("#%d cannot be both the parent and a child", child.ShortID)) {
				return true
			}
			continue
		}
		if seen[child.ID] {
			continue
		}
		seen[child.ID] = true
		p.result.Children = append(p.result.Children, child)
	}
	if reason := invalidQuickEditHierarchy(p.task, p.project, p.effectiveProject, p.result); reason != "" {
		return p.setError(reason)
	}
	return false
}

// applyDependencies reads the ~task, -~task, and ~0 halves of the soft-reference
// patch. It runs after applyProject so a bare alias resolves in the project the
// edit ends in, and it refuses exactly what the application layer refuses, at the
// same wording, so a line the preview accepted is never rejected on commit.
//
// A removal is validated for shape alone: a stored reference whose target has
// since closed, moved out of view, or been deleted must stay removable, which is
// why only additions ask the workspace anything.
func (p *quickEditParser) applyDependencies() bool {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldDependency)
	if !ok {
		return p.setError("dependency grammar is unavailable")
	}
	removeDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldRemoveDependency)
	additions := p.parsed.TokensFor(textgrammar.FieldDependency)
	removals := p.parsed.TokensFor(textgrammar.FieldRemoveDependency)
	if len(additions) == 0 && len(removals) == 0 {
		return false
	}

	clears := 0
	for _, token := range additions {
		if token.Value == DependencyClearValue {
			clears++
		}
	}
	if clears > 0 {
		p.result.dependencyClear = true
		if clears != len(additions) || len(removals) > 0 {
			return p.setError("clearing dependencies cannot be combined with adding or removing them")
		}
		return false
	}

	for _, token := range removals {
		if !p.applyDependencyToken(token, removeDefinition.Trigger, false) {
			return true
		}
	}
	for _, token := range additions {
		if !p.applyDependencyToken(token, definition.Trigger, true) {
			return true
		}
	}
	for _, addition := range p.result.dependencyAdds {
		if slices.ContainsFunc(p.result.dependencyRemovals, func(removal CapturedDependencyRef) bool {
			return removal.Locator == addition.Locator
		}) {
			return p.setError(fmt.Sprintf("conflicting dependency patch: %s%s and %s%s", definition.Trigger, addition.Label(), removeDefinition.Trigger, addition.Label()))
		}
	}
	return false
}

// applyDependencyToken resolves one reference and files it on the matching half
// of the patch. It reports false when the parse must stop, matching the loop
// convention every other repeatable field uses.
func (p *quickEditParser) applyDependencyToken(token textgrammar.Token, trigger string, addition bool) bool {
	if token.Value == DependencyClearValue && !addition {
		return !p.setError(fmt.Sprintf("%s%s is not a clear form; use ~%s to clear every dependency", trigger, token.Value, DependencyClearValue))
	}
	ref, err := capturetext.ResolveDependencyReference(token.Value, p.effectiveProject, p.allProjects)
	if err != nil {
		return !p.setError(err.Error())
	}
	if ref.Locator.TaskID == "" {
		return !p.setError(capturetext.DependencyNotFoundMessage(trigger, token.Value))
	}
	if p.task != nil && ref.Locator.TaskID == p.task.ID {
		return !p.setError("a task cannot depend on itself")
	}
	if addition {
		// A locator the task already stores round-trips on shape alone, matching the
		// application layer's existing-versus-new distinction: the workspace keeps a
		// reference after its target closes or its project stops being loaded, so
		// naming it again must stay the no-op it was a moment before.
		if !p.storesDependency(ref.Locator) {
			if !ref.Resolved {
				return !p.setError(capturetext.DependencyNotFoundMessage(trigger, token.Value))
			}
			if ref.Terminal {
				return !p.setError(fmt.Sprintf("cannot depend on %s: it is already closed", ref.Label()))
			}
		}
		p.result.dependencyAdds = capturetext.AppendDependencyRef(p.result.dependencyAdds, ref)
		return true
	}
	p.result.dependencyRemovals = capturetext.AppendDependencyRef(p.result.dependencyRemovals, ref)
	return true
}

// storesDependency reports whether the edited task already carries this exact
// locator. It is the parser's half of the existing-versus-new rule, so a line
// the preview accepted is one the commit accepts for the same reason.
func (p *quickEditParser) storesDependency(locator app.TaskLocator) bool {
	if p.task == nil {
		return false
	}
	return slices.Contains(p.task.Dependencies, locator.Reference())
}

// DependencyPatch projects the parsed halves into the application contract.
func (r *QuickEditResult) DependencyPatch() app.DependencyPatch {
	return app.DependencyPatch{
		Add:    capturetext.DependencyLocatorsOf(r.dependencyAdds),
		Remove: capturetext.DependencyLocatorsOf(r.dependencyRemovals),
		Clear:  r.dependencyClear,
	}
}

func (p *quickEditParser) finish() (*QuickEditResult, string) {
	p.result.Title = p.parsed.Text
	if err := models.ValidateTaskTitle(p.result.Title); err != nil {
		p.setError(err.Error())
	}
	if p.task == nil {
		p.setError("task is required for inline edit")
	}
	if p.project == nil || p.task == nil || (p.project.Tasks != nil && p.project.GetTaskByID(p.task.ID) != p.task) {
		p.setError("edited task is stale or is not in the source project")
	}
	if p.errMsg == "" {
		if err := buildQuickEditPreview(p.result, p.task, p.project); err != nil {
			p.setError(err.Error())
		}
	}
	return p.result, p.errMsg
}
