package tui

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/util"
)

type quickEditParentMode uint8

const (
	quickEditParentPreserve quickEditParentMode = iota
	quickEditParentRoot
	quickEditParentSet
)

// quickEditResult is a validated patch plus its pure, overlaid preview. Omitted
// fields preserve the current task. Structural references are resolved against
// the effective project, but neither project nor task is mutated by parsing.
type quickEditResult struct {
	title string

	priority    int
	prioritySet bool

	tagsToAdd    []string
	tagsToRemove []string

	contextValue string
	contextSet   bool
	contextClear bool

	dueDate      *time.Time
	dueDateSet   bool
	dueDateClear bool

	deferUntil *time.Time
	deferSet   bool
	deferClear bool

	destination    textgrammar.Destination
	destinationSet bool
	state          models.WorkflowState
	stateSet       bool

	pinned    bool
	pinnedSet bool

	targetProject *models.Project
	parentMode    quickEditParentMode
	parent        *models.Task
	// children is the additive adoption patch in first-occurrence order. An
	// empty slice means the task keeps exactly the children it already has;
	// there is no clear form, so nil and empty say the same thing.
	children []*models.Task

	// dependencyAdds and dependencyRemovals are the additive and subtractive
	// halves of the soft-reference patch, in first-occurrence order.
	// dependencyClear replaces the whole stored set with nothing and cannot be
	// combined with either, exactly as app.DependencyPatch requires. All three
	// empty means the edit never mentioned a dependency, which preserves the
	// stored set.
	dependencyAdds     []capturedDependencyRef
	dependencyRemovals []capturedDependencyRef
	dependencyClear    bool

	// preview is a detached task value reflecting every accepted root patch.
	// Descendants and workspace collections are deliberately never rewritten.
	preview models.Task
}

// parseQuickEdit parses and validates inline task edits. The optional project
// slice supplies exact-slug cross-project resolution without widening legacy
// callers that only edit within one project.
func parseQuickEdit(input string, task *models.Task, project *models.Project, projectSets ...[]*models.Project) (*quickEditResult, error) {
	result, errMsg := parseQuickEditShared(input, task, project, flattenProjectSets(project, projectSets), true)
	if errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}
	return result, nil
}

// parseQuickEditForPreview returns the same pure overlay as strict parsing while
// preserving the first validation error for live feedback.
func parseQuickEditForPreview(input string, task *models.Task, project *models.Project, projectSets ...[]*models.Project) (*quickEditResult, string) {
	return parseQuickEditShared(input, task, project, flattenProjectSets(project, projectSets), false)
}

func flattenProjectSets(current *models.Project, sets [][]*models.Project) []*models.Project {
	projects := []*models.Project{}
	if len(sets) > 0 {
		projects = append(projects, sets[0]...)
	}
	if current != nil && !slices.ContainsFunc(projects, func(candidate *models.Project) bool {
		return candidate != nil && candidate.ID == current.ID
	}) {
		projects = append(projects, current)
	}
	return projects
}

func parseQuickEditShared(input string, task *models.Task, project *models.Project, allProjects []*models.Project, strict bool) (*quickEditResult, string) {
	parser := quickEditParser{
		parsed:           textgrammar.Parse(input, textgrammar.SurfaceEdit),
		result:           &quickEditResult{parentMode: quickEditParentPreserve},
		task:             task,
		project:          project,
		allProjects:      allProjects,
		effectiveProject: project,
		strict:           strict,
	}
	if parser.applyDiagnostics() || parser.applyPriority() || parser.applyTags() || parser.applyContext() ||
		parser.applyDueDate() || parser.applyDestination() || parser.applyDefer() {
		return parser.result, parser.errMsg
	}
	parser.applyPin()
	if parser.applyProject() || parser.applyParent() || parser.applyChildren() || parser.applyDependencies() {
		return parser.result, parser.errMsg
	}
	return parser.finish()
}

func (p *quickEditParser) applyContext() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldContext)
	if len(tokens) == 0 {
		return false
	}
	p.result.contextSet = true
	if tokens[0].Value == "0" {
		p.result.contextClear = true
		return false
	}
	context, err := models.NormalizeTaskContext(tokens[0].Value)
	if err != nil {
		definition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldContext)
		return p.setError(fmt.Sprintf("invalid context %s%s: %s", definition.Trigger, tokens[0].Value, err))
	}
	p.result.contextValue = context
	return false
}

type quickEditParser struct {
	parsed           textgrammar.Result
	result           *quickEditResult
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
		p.result.priority = models.PriorityUnset
		p.result.prioritySet = true
		return false
	}
	values, ok := textgrammar.ParsePriority(token.Value, false)
	if !ok {
		return p.setError(textgrammar.InvalidPriorityError(token.Value).Error())
	}
	p.result.priority = values[0]
	p.result.prioritySet = true
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
		p.result.tagsToRemove = appendUnique(p.result.tagsToRemove, tag)
	}
	for _, token := range p.parsed.TokensFor(textgrammar.FieldTag) {
		tag, err := models.NormalizeTaskTag(token.Value)
		if err != nil {
			if p.setError(fmt.Sprintf("invalid tag #%s: %s", token.Value, err)) {
				return true
			}
			continue
		}
		if slices.Contains(p.result.tagsToRemove, tag) {
			if p.setError(fmt.Sprintf("conflicting tag patch: #%s and -#%s", tag, tag)) {
				return true
			}
			continue
		}
		p.result.tagsToAdd = appendUnique(p.result.tagsToAdd, tag)
	}
	return false
}

func (p *quickEditParser) applyDueDate() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldDue)
	if len(tokens) == 0 {
		return false
	}
	token := tokens[0]
	p.result.dueDateSet = true
	if token.Value == "0" {
		p.result.dueDateClear = true
		return false
	}
	date, err := ParseDueDateValue(token.Value)
	if err != nil {
		return p.setError(fmt.Sprintf("invalid date: %s", token.Value))
	}
	p.result.dueDate = &date
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
	p.result.deferSet = true
	if token.Value == "0" {
		p.result.deferClear = true
		return false
	}
	instant, err := util.ParseDeferInstant(token.Value)
	if err != nil {
		return p.setError(fmt.Sprintf("invalid defer: %s", token.Value))
	}
	p.result.deferUntil = &instant

	state := models.StateBacklog
	switch {
	case p.result.destinationSet:
		state = p.result.destination.WorkflowState()
	case p.task != nil:
		state = p.task.State()
	}
	if refusal := app.DeferralRefusesState(state); refusal != "" {
		return p.setError(refusal)
	}
	p.result.destination = textgrammar.DestinationBacklog
	p.result.destinationSet = true
	p.result.state = models.StateBacklog
	p.result.stateSet = true
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
	p.result.destination = destination
	p.result.destinationSet = true
	p.result.state = destination.WorkflowState()
	p.result.stateSet = true
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
	p.result.pinned = slices.Contains(pinSetValues(), tokens[0].Value)
	p.result.pinnedSet = true
}

func (p *quickEditParser) applyProject() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldProject)
	if len(tokens) == 0 {
		return false
	}
	target := resolveProject(tokens[0].Value, p.allProjects)
	if target == nil {
		return p.setError(fmt.Sprintf("project not found: %q (use an exact %%project-slug)", tokens[0].Value))
	}
	p.result.targetProject = target
	p.effectiveProject = target
	// A project move without an explicit parent always lands at root.
	p.result.parentMode = quickEditParentRoot
	return false
}

func (p *quickEditParser) applyParent() bool {
	tokens := p.parsed.TokensFor(textgrammar.FieldParent)
	if len(tokens) == 0 {
		return false
	}
	token := tokens[0]
	if token.Value == "0" {
		p.result.parentMode = quickEditParentRoot
		return false
	}
	parent, err := resolveParent(token.Value, p.effectiveProject)
	if err != nil {
		return p.setError(err.Error())
	}
	if parent == nil {
		return p.setError(fmt.Sprintf("parent not found: %q in project %q", token.Value, projectLabel(p.effectiveProject)))
	}
	if reason := invalidQuickEditParent(p.task, p.project, p.effectiveProject, parent); reason != "" {
		return p.setError(reason)
	}
	p.result.parentMode = quickEditParentSet
	p.result.parent = parent
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
		child, err := resolveChild(token.Value, p.effectiveProject)
		if err != nil {
			if p.setError(err.Error()) {
				return true
			}
			continue
		}
		if child == nil {
			if p.setError(fmt.Sprintf("child not found: %q in project %q", token.Value, projectLabel(p.effectiveProject))) {
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
		if p.result.parentMode == quickEditParentSet && p.result.parent != nil && p.result.parent.ID == child.ID {
			if p.setError(fmt.Sprintf("#%d cannot be both the parent and a child", child.ShortID)) {
				return true
			}
			continue
		}
		if seen[child.ID] {
			continue
		}
		seen[child.ID] = true
		p.result.children = append(p.result.children, child)
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
		if token.Value == dependencyClearValue {
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
		if slices.ContainsFunc(p.result.dependencyRemovals, func(removal capturedDependencyRef) bool {
			return removal.locator == addition.locator
		}) {
			return p.setError(fmt.Sprintf("conflicting dependency patch: %s%s and %s%s", definition.Trigger, addition.label(), removeDefinition.Trigger, addition.label()))
		}
	}
	return false
}

// applyDependencyToken resolves one reference and files it on the matching half
// of the patch. It reports false when the parse must stop, matching the loop
// convention every other repeatable field uses.
func (p *quickEditParser) applyDependencyToken(token textgrammar.Token, trigger string, addition bool) bool {
	if token.Value == dependencyClearValue && !addition {
		return !p.setError(fmt.Sprintf("%s%s is not a clear form; use ~%s to clear every dependency", trigger, token.Value, dependencyClearValue))
	}
	ref, err := resolveDependencyReference(token.Value, p.effectiveProject, p.allProjects)
	if err != nil {
		return !p.setError(err.Error())
	}
	if ref.locator.TaskID == "" {
		return !p.setError(dependencyNotFoundMessage(trigger, token.Value))
	}
	if p.task != nil && ref.locator.TaskID == p.task.ID {
		return !p.setError("a task cannot depend on itself")
	}
	if addition {
		if !ref.resolved {
			return !p.setError(dependencyNotFoundMessage(trigger, token.Value))
		}
		if ref.terminal {
			return !p.setError(fmt.Sprintf("cannot depend on %s: it is already closed", ref.label()))
		}
		p.result.dependencyAdds = appendDependencyRef(p.result.dependencyAdds, ref)
		return true
	}
	p.result.dependencyRemovals = appendDependencyRef(p.result.dependencyRemovals, ref)
	return true
}

// dependencyPatch projects the parsed halves into the application contract.
func (r *quickEditResult) dependencyPatch() app.DependencyPatch {
	return app.DependencyPatch{
		Add:    dependencyLocators(r.dependencyAdds),
		Remove: dependencyLocators(r.dependencyRemovals),
		Clear:  r.dependencyClear,
	}
}

func (p *quickEditParser) finish() (*quickEditResult, string) {
	p.result.title = p.parsed.Text
	if err := models.ValidateTaskTitle(p.result.title); err != nil {
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

func appendUnique(values []string, value string) []string {
	if !slices.Contains(values, value) {
		return append(values, value)
	}
	return values
}

func projectLabel(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.Slug != "" {
		return project.Slug
	}
	return project.Name
}

// validQuickEditParents is also the dynamic completion source for the row
// editor: it derives candidates from the effective project and applies the same
// cycle and depth checks as strict parsing.
func validQuickEditParents(task *models.Task, source, effective *models.Project) []*models.Task {
	if effective == nil {
		return nil
	}
	result := make([]*models.Task, 0, len(effective.Tasks))
	for _, candidate := range effective.Tasks {
		if candidate == nil || candidate.IsComplete() {
			continue
		}
		if invalidQuickEditParent(task, source, effective, candidate) == "" {
			result = append(result, candidate)
		}
	}
	return result
}

// quickEditParentCandidates resolves the effective project from the current
// draft before producing parent completion candidates. It intentionally ignores
// the draft's parent token: completion is about the choices that may replace it.
func quickEditParentCandidates(input string, task *models.Task, source *models.Project, allProjects []*models.Project) ([]*models.Task, string) {
	parsed := textgrammar.Parse(input, textgrammar.SurfaceEdit)
	for _, diagnostic := range parsed.Diagnostics {
		if diagnostic.Field == textgrammar.FieldProject {
			return nil, diagnostic.Message
		}
	}
	effective := source
	if tokens := parsed.TokensFor(textgrammar.FieldProject); len(tokens) > 0 {
		effective = resolveProject(tokens[0].Value, flattenProjectSets(source, [][]*models.Project{allProjects}))
		if effective == nil {
			return nil, fmt.Sprintf("project not found: %q (use an exact %%project-slug)", tokens[0].Value)
		}
	}
	return validQuickEditParents(task, source, effective), ""
}

// quickEditChildCandidates offers the tasks the draft can still adopt. It reads
// the whole line — the %project route, the parent patch, and every <child token
// already accepted — because an adoption is legal only against the hierarchy the
// finished line produces. It withholds exactly what strict parsing would refuse:
// the edited task itself, an alias of a task the draft already names, a
// projected ancestor (which covers the task named by >parent), and a candidate
// whose subtree would overflow the depth limit.
func quickEditChildCandidates(input string, task *models.Task, source *models.Project, allProjects []*models.Project) ([]*models.Task, string) {
	parsed := textgrammar.Parse(input, textgrammar.SurfaceEdit)
	for _, diagnostic := range parsed.Diagnostics {
		if diagnostic.Field == textgrammar.FieldProject {
			return nil, diagnostic.Message
		}
	}
	effective := source
	draft := &quickEditResult{parentMode: quickEditParentPreserve}
	if tokens := parsed.TokensFor(textgrammar.FieldProject); len(tokens) > 0 {
		effective = resolveProject(tokens[0].Value, flattenProjectSets(source, [][]*models.Project{allProjects}))
		if effective == nil {
			return nil, fmt.Sprintf("project not found: %q (use an exact %%project-slug)", tokens[0].Value)
		}
		draft.parentMode = quickEditParentRoot
	}
	if task == nil || source == nil || effective == nil {
		return nil, ""
	}
	if tokens := parsed.TokensFor(textgrammar.FieldParent); len(tokens) > 0 {
		draft.parentMode, draft.parent = draftedQuickEditParent(tokens[0].Value, task, source, effective, draft.parentMode)
	}
	adopted := map[string]bool{}
	for _, token := range parsed.TokensFor(textgrammar.FieldChild) {
		child, err := resolveChild(token.Value, effective)
		if err != nil || child == nil || adopted[child.ID] {
			continue
		}
		adopted[child.ID] = true
		draft.children = append(draft.children, child)
	}
	hierarchy := newQuickEditHierarchy(task, source, effective, draft)
	candidates := make([]*models.Task, 0, len(effective.Tasks))
	for _, candidate := range structuralCandidates(effective) {
		if candidate.ID == task.ID || adopted[candidate.ID] || hierarchy.rejectsChild(task, candidate) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates, ""
}

// draftedQuickEditParent reads the draft's parent token the way applyParent
// does, but keeps the current mode whenever the token names nothing valid: a
// broken parent already fails the draft, and completion must still describe the
// hierarchy the rest of the line produces.
func draftedQuickEditParent(value string, task *models.Task, source, effective *models.Project, mode quickEditParentMode) (quickEditParentMode, *models.Task) {
	if value == "0" {
		return quickEditParentRoot, nil
	}
	parent, err := resolveParent(value, effective)
	if err != nil || parent == nil || invalidQuickEditParent(task, source, effective, parent) != "" {
		return mode, nil
	}
	return quickEditParentSet, parent
}

func invalidQuickEditParent(task *models.Task, source, effective *models.Project, parent *models.Task) string {
	if task == nil || source == nil || effective == nil || parent == nil {
		return "task and parent must belong to loaded projects"
	}
	if effective.GetTaskByID(parent.ID) != parent {
		return fmt.Sprintf("parent #%d is stale or not in project %q", parent.ShortID, projectLabel(effective))
	}
	if parent.ID == task.ID {
		return "task cannot be its own parent"
	}
	if descendantIDs(task, source)[parent.ID] {
		return fmt.Sprintf("parent #%d is inside the edited task's subtree", parent.ShortID)
	}
	parentDepth, err := hierarchyDepth(effective, parent.ID)
	if err != nil {
		return err.Error()
	}
	height, err := subtreeHeight(source, task.ID)
	if err != nil {
		return err.Error()
	}
	if parentDepth+1+height > models.MaxNestingDepth {
		return fmt.Sprintf("parent #%d would make the subtree exceed maximum nesting depth of %d levels", parent.ShortID, models.MaxNestingDepth)
	}
	return ""
}

func descendantIDs(task *models.Task, project *models.Project) map[string]bool {
	result := map[string]bool{}
	if task == nil || project == nil {
		return result
	}
	children := make(map[string][]string)
	for _, candidate := range project.Tasks {
		if candidate != nil && candidate.ParentID != nil {
			children[*candidate.ParentID] = append(children[*candidate.ParentID], candidate.ID)
		}
	}
	var walk func(string)
	walk = func(id string) {
		for _, child := range children[id] {
			if result[child] {
				continue
			}
			result[child] = true
			walk(child)
		}
	}
	walk(task.ID)
	return result
}

func hierarchyDepth(project *models.Project, taskID string) (int, error) {
	depth := 0
	seen := map[string]bool{}
	for taskID != "" {
		if seen[taskID] {
			return 0, fmt.Errorf("task %q has a circular parent reference", taskID)
		}
		seen[taskID] = true
		task := project.GetTaskByID(taskID)
		if task == nil {
			return 0, fmt.Errorf("task %q is missing from project %q", taskID, projectLabel(project))
		}
		if task.ParentID == nil {
			return depth, nil
		}
		depth++
		taskID = *task.ParentID
	}
	return depth, nil
}

func subtreeHeight(project *models.Project, taskID string) (int, error) {
	children := make(map[string][]string)
	for _, task := range project.Tasks {
		if task != nil && task.ParentID != nil {
			children[*task.ParentID] = append(children[*task.ParentID], task.ID)
		}
	}
	var walk func(string, map[string]bool) (int, error)
	walk = func(id string, visiting map[string]bool) (int, error) {
		if visiting[id] {
			return 0, fmt.Errorf("task %q has a circular child reference", id)
		}
		visiting[id] = true
		defer delete(visiting, id)
		height := 0
		for _, child := range children[id] {
			childHeight, err := walk(child, visiting)
			if err != nil {
				return 0, err
			}
			height = max(height, childHeight+1)
		}
		return height, nil
	}
	return walk(taskID, map[string]bool{})
}

// invalidQuickEditHierarchy validates the parent patch and every adoption as one
// projected graph, exactly as the application layer submits them as one checked
// batch. Measuring the combination is what accepts a hierarchy inversion whose
// individual edges are illegal on their own, and what refuses an adoption that is
// only a cycle once the draft's own parent move lands.
func invalidQuickEditHierarchy(task *models.Task, source, effective *models.Project, result *quickEditResult) string {
	if len(result.children) == 0 {
		return ""
	}
	if task == nil || source == nil || effective == nil {
		return "task and children must belong to loaded projects"
	}
	hierarchy := newQuickEditHierarchy(task, source, effective, result)
	for _, child := range result.children {
		if hierarchy.childIsAncestor(task, child) {
			return fmt.Sprintf("child #%d is an ancestor of the edited task", child.ShortID)
		}
	}
	if hierarchy.depthErr != nil {
		return hierarchy.depthErr.Error()
	}
	for _, child := range result.children {
		overflows, err := hierarchy.childOverflowsDepth(child)
		if err != nil {
			return err.Error()
		}
		if overflows {
			return fmt.Sprintf("child #%d would make the subtree exceed maximum nesting depth of %d levels", child.ShortID, models.MaxNestingDepth)
		}
	}
	return ""
}

// quickEditHierarchy is the graph the draft produces plus the edited task's
// depth in it. Building it once is what lets completion rule on every candidate
// with the same two measurements strict validation applies to an accepted child.
type quickEditHierarchy struct {
	parents     map[string]*string
	descendants map[string][]string
	depth       int
	depthErr    error
}

func newQuickEditHierarchy(task *models.Task, source, effective *models.Project, result *quickEditResult) quickEditHierarchy {
	parents := projectedQuickEditParents(task, source, effective, result)
	descendants := make(map[string][]string, len(parents))
	for id, parentID := range parents {
		if parentID != nil {
			descendants[*parentID] = append(descendants[*parentID], id)
		}
	}
	hierarchy := quickEditHierarchy{parents: parents, descendants: descendants}
	hierarchy.depth, hierarchy.depthErr = projectedHierarchyDepth(parents, task.ID)
	return hierarchy
}

// childIsAncestor answers the cycle question. The candidate's own adoption edge
// is deliberately absent from the projection: it can only change the walk up
// from the edited task by closing the very cycle this reports.
func (h quickEditHierarchy) childIsAncestor(task, child *models.Task) bool {
	return projectedAncestorReaches(h.parents, task.ID, child.ID)
}

func (h quickEditHierarchy) childOverflowsDepth(child *models.Task) (bool, error) {
	height, err := projectedSubtreeHeight(h.descendants, child.ID, map[string]bool{})
	if err != nil {
		return false, err
	}
	return h.depth+1+height > models.MaxNestingDepth, nil
}

// rejectsChild collapses both measurements into the one predicate completion
// needs, and treats an unmeasurable graph as a refusal so a broken draft offers
// nothing rather than offering a candidate it cannot rule on.
func (h quickEditHierarchy) rejectsChild(task, child *models.Task) bool {
	if h.childIsAncestor(task, child) || h.depthErr != nil {
		return true
	}
	overflows, err := h.childOverflowsDepth(child)
	return err != nil || overflows
}

// projectedQuickEditParents is the parent map the draft would produce. It holds
// the effective project's tasks plus, on a cross-project edit, only the moving
// subtree, because every other source task stays behind.
func projectedQuickEditParents(task *models.Task, source, effective *models.Project, result *quickEditResult) map[string]*string {
	parents := make(map[string]*string, len(effective.Tasks))
	record := func(candidate *models.Task) {
		if candidate == nil {
			return
		}
		if candidate.ParentID == nil {
			parents[candidate.ID] = nil
			return
		}
		parentID := *candidate.ParentID
		parents[candidate.ID] = &parentID
	}
	for _, candidate := range effective.Tasks {
		record(candidate)
	}
	if effective != source {
		moving := descendantIDs(task, source)
		for _, candidate := range source.Tasks {
			if candidate != nil && (candidate.ID == task.ID || moving[candidate.ID]) {
				record(candidate)
			}
		}
	}
	switch result.parentMode {
	case quickEditParentRoot:
		parents[task.ID] = nil
	case quickEditParentSet:
		parentID := result.parent.ID
		parents[task.ID] = &parentID
	case quickEditParentPreserve:
	}
	for _, child := range result.children {
		adopter := task.ID
		parents[child.ID] = &adopter
	}
	return parents
}

func projectedAncestorReaches(parents map[string]*string, fromID, targetID string) bool {
	seen := map[string]bool{}
	for current := fromID; !seen[current]; {
		seen[current] = true
		parentID, known := parents[current]
		if !known || parentID == nil {
			return false
		}
		if *parentID == targetID {
			return true
		}
		current = *parentID
	}
	return false
}

func projectedHierarchyDepth(parents map[string]*string, taskID string) (int, error) {
	depth := 0
	seen := map[string]bool{}
	for {
		if seen[taskID] {
			return 0, fmt.Errorf("task %q has a circular parent reference", taskID)
		}
		seen[taskID] = true
		parentID, known := parents[taskID]
		if !known {
			return 0, fmt.Errorf("task %q is missing from the edited hierarchy", taskID)
		}
		if parentID == nil {
			return depth, nil
		}
		depth++
		taskID = *parentID
	}
}

func projectedSubtreeHeight(descendants map[string][]string, taskID string, visiting map[string]bool) (int, error) {
	if visiting[taskID] {
		return 0, fmt.Errorf("task %q has a circular child reference", taskID)
	}
	visiting[taskID] = true
	defer delete(visiting, taskID)
	height := 0
	for _, childID := range descendants[taskID] {
		childHeight, err := projectedSubtreeHeight(descendants, childID, visiting)
		if err != nil {
			return 0, err
		}
		height = max(height, childHeight+1)
	}
	return height, nil
}

func buildQuickEditPreview(result *quickEditResult, task *models.Task, project *models.Project) error {
	preview := *task
	preview.Tags = slices.Clone(task.Tags)
	preview.Children = slices.Clone(task.Children)
	preview.Comments = slices.Clone(task.Comments)
	preview.Attachments = slices.Clone(task.Attachments)
	preview.Checklist = slices.Clone(task.Checklist)
	preview.Dependencies = applyQuickEditPreviewDependencies(result, task)
	preview.Title = result.title
	if result.prioritySet {
		preview.Priority = result.priority
	}
	for _, tag := range result.tagsToRemove {
		preview.Tags = slices.DeleteFunc(preview.Tags, func(existing string) bool {
			normalized, err := models.NormalizeTaskTag(existing)
			return err == nil && normalized == tag
		})
	}
	for _, tag := range result.tagsToAdd {
		if !preview.HasTag(tag) {
			preview.Tags = append(preview.Tags, tag)
		}
	}
	if result.contextSet {
		preview.Context = result.contextValue
		if result.contextClear {
			preview.Context = ""
		}
	}
	if result.dueDateSet {
		preview.DueDate = result.dueDate
		if result.dueDateClear {
			preview.DueDate = nil
		}
	}
	if result.deferSet {
		preview.DeferredUntil = result.deferUntil
		if result.deferClear {
			preview.DeferredUntil = nil
		}
	}
	if result.pinnedSet {
		preview.Pinned = result.pinned
	}
	switch result.parentMode {
	case quickEditParentRoot:
		preview.ParentID = nil
		preview.Parent = nil
	case quickEditParentSet:
		parentID := result.parent.ID
		preview.ParentID = &parentID
		preview.Parent = result.parent
	case quickEditParentPreserve:
	}
	for _, child := range result.children {
		if !slices.ContainsFunc(preview.Children, func(existing *models.Task) bool {
			return existing != nil && existing.ID == child.ID
		}) {
			preview.Children = append(preview.Children, child)
		}
	}
	if err := applyQuickEditPreviewLifecycle(result, task, project, &preview); err != nil {
		return err
	}
	// An adoption is the one patch that can leave a task terminal above an open
	// child, which the application layer's post-mutation reconciliation reopens.
	if len(result.children) > 0 && preview.IsTerminalState() && hasIncompleteFinalChildren(result, task, project) {
		preview.Uncomplete()
	}
	result.state = preview.State()
	result.preview = preview
	return nil
}

// applyQuickEditPreviewLifecycle moves the preview to the state the edit names.
// The three planning destinations route through the planning action and settle
// the state themselves; every other destination, and a bare state edit, is an
// ordinary transition. Only a destination-driven terminal move consults the
// task's children, because a bare state edit is refused later by the same rule
// in the application layer.
func applyQuickEditPreviewLifecycle(result *quickEditResult, task *models.Task, project *models.Project, preview *models.Task) error {
	if result.destinationSet {
		switch result.destination {
		case textgrammar.DestinationToday:
			_, _, err := app.ApplyPlanningActionToTask(preview, project.ID, app.PlanningAddToday, time.Now())
			return err
		case textgrammar.DestinationNext:
			_, _, err := app.ApplyPlanningActionToTask(preview, project.ID, app.PlanningAddNext, time.Now())
			return err
		case textgrammar.DestinationBacklog:
			_, _, err := app.ApplyPlanningActionToTask(preview, project.ID, app.PlanningBacklog, time.Now())
			return err
		default:
			// Every other destination settles below as a state transition.
		}
	} else if !result.stateSet {
		return nil
	}
	// The children check precedes the equal-state return because a destination the
	// task already holds is still an explicit terminal claim, and the application
	// layer's guard refuses it on the same terms.
	if result.destinationSet && result.state.IsTerminal() && hasIncompleteFinalChildren(result, task, project) {
		return fmt.Errorf("task #%d has incomplete children and cannot move to %s", task.ShortID, result.state)
	}
	if preview.State() == result.state {
		return nil
	}
	if reason := models.CanTransitionState(preview.State(), result.state); reason != "" {
		return fmt.Errorf("%s", reason)
	}
	return preview.ApplyStateTransition(result.state)
}

// applyQuickEditPreviewDependencies overlays the soft-reference patch on the
// stored set with the same rules resolveDependencyPatch applies at commit:
// removals drop first, additions append in first-occurrence order, and a clear
// replaces everything with nothing. Keeping the two in step is what stops the
// preview from promising a set the commit would not produce.
func applyQuickEditPreviewDependencies(result *quickEditResult, task *models.Task) []models.TaskReference {
	if result.dependencyClear {
		return nil
	}
	next := make([]models.TaskReference, 0, len(task.Dependencies)+len(result.dependencyAdds))
	for _, existing := range task.Dependencies {
		if slices.ContainsFunc(result.dependencyRemovals, func(removal capturedDependencyRef) bool {
			return removal.locator.Reference() == existing
		}) {
			continue
		}
		if !slices.Contains(next, existing) {
			next = append(next, existing)
		}
	}
	for _, addition := range result.dependencyAdds {
		reference := addition.locator.Reference()
		if !slices.Contains(next, reference) {
			next = append(next, reference)
		}
	}
	return models.CloneTaskReferences(next)
}

func hasIncompleteDirectChildren(task *models.Task, project *models.Project) bool {
	for _, candidate := range project.Tasks {
		if candidate != nil && candidate.ParentID != nil && *candidate.ParentID == task.ID && !candidate.IsTerminalState() {
			return true
		}
	}
	return false
}

// hasIncompleteFinalChildren reads the children the edit ends with: the ones the
// task already owns plus the ones it adopts. Adoption is additive, so nothing
// leaves the set and the two sources simply combine.
func hasIncompleteFinalChildren(result *quickEditResult, task *models.Task, project *models.Project) bool {
	if hasIncompleteDirectChildren(task, project) {
		return true
	}
	return slices.ContainsFunc(result.children, func(child *models.Task) bool {
		return child != nil && !child.IsTerminalState()
	})
}
