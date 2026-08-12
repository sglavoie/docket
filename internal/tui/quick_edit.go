package tui

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
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

	dueDate      *time.Time
	dueDateSet   bool
	dueDateClear bool

	destination    textgrammar.Destination
	destinationSet bool
	state          models.WorkflowState
	stateSet       bool

	targetProject *models.Project
	parentMode    quickEditParentMode
	parent        *models.Task

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
	if parser.applyDiagnostics() || parser.applyPriority() || parser.applyTags() ||
		parser.applyDueDate() || parser.applyDestination() || parser.applyProject() ||
		parser.applyParent() {
		return parser.result, parser.errMsg
	}
	return parser.finish()
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
	date, err := ParseDate(token.Value)
	if err != nil {
		return p.setError(fmt.Sprintf("invalid date: %s", token.Value))
	}
	p.result.dueDate = &date
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

func buildQuickEditPreview(result *quickEditResult, task *models.Task, project *models.Project) error {
	preview := *task
	preview.Tags = slices.Clone(task.Tags)
	preview.Children = slices.Clone(task.Children)
	preview.Comments = slices.Clone(task.Comments)
	preview.Attachments = slices.Clone(task.Attachments)
	preview.Checklist = slices.Clone(task.Checklist)
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
	if result.dueDateSet {
		preview.DueDate = result.dueDate
		if result.dueDateClear {
			preview.DueDate = nil
		}
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
	if result.destinationSet {
		switch result.destination {
		case textgrammar.DestinationToday:
			_, _, err := app.ApplyPlanningActionToTask(&preview, project.ID, app.PlanningAddToday, time.Now())
			if err != nil {
				return err
			}
		case textgrammar.DestinationNext:
			_, _, err := app.ApplyPlanningActionToTask(&preview, project.ID, app.PlanningAddNext, time.Now())
			if err != nil {
				return err
			}
		case textgrammar.DestinationBacklog:
			_, _, err := app.ApplyPlanningActionToTask(&preview, project.ID, app.PlanningBacklog, time.Now())
			if err != nil {
				return err
			}
		default:
			if preview.State() != result.state {
				if reason := models.CanTransitionState(preview.State(), result.state); reason != "" {
					return fmt.Errorf("%s", reason)
				}
				if result.state.IsTerminal() && hasIncompleteDirectChildren(task, project) {
					return fmt.Errorf("task #%d has incomplete children and cannot move to %s", task.ShortID, result.state)
				}
				if err := preview.ApplyStateTransition(result.state); err != nil {
					return err
				}
			}
		}
	} else if result.stateSet && preview.State() != result.state {
		if reason := models.CanTransitionState(preview.State(), result.state); reason != "" {
			return fmt.Errorf("%s", reason)
		}
		if err := preview.ApplyStateTransition(result.state); err != nil {
			return err
		}
	}
	result.state = preview.State()
	result.preview = preview
	return nil
}

func hasIncompleteDirectChildren(task *models.Task, project *models.Project) bool {
	for _, candidate := range project.Tasks {
		if candidate != nil && candidate.ParentID != nil && *candidate.ParentID == task.ID && !candidate.IsTerminalState() {
			return true
		}
	}
	return false
}
