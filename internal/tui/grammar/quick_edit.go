package grammar

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

type QuickEditParentMode uint8

const (
	QuickEditParentPreserve QuickEditParentMode = iota
	QuickEditParentRoot
	QuickEditParentSet
)

// QuickEditResult is a validated patch plus its pure, overlaid preview. Omitted
// fields preserve the current task. Structural references are resolved against
// the effective project, but neither project nor task is mutated by parsing.
type QuickEditResult struct {
	Title string

	Priority    int
	PrioritySet bool

	TagsToAdd    []string
	TagsToRemove []string

	ContextValue string
	ContextSet   bool
	ContextClear bool

	DueDate      *time.Time
	DueDateSet   bool
	DueDateClear bool

	DeferUntil *time.Time
	DeferSet   bool
	DeferClear bool

	Destination    textgrammar.Destination
	DestinationSet bool
	State          models.WorkflowState
	StateSet       bool

	Pinned    bool
	PinnedSet bool

	Review    bool
	ReviewSet bool

	TargetProject *models.Project
	ParentMode    QuickEditParentMode
	Parent        *models.Task
	// children is the additive adoption patch in first-occurrence order. An
	// empty slice means the task keeps exactly the children it already has;
	// there is no clear form, so nil and empty say the same thing.
	Children []*models.Task

	// dependencyAdds and dependencyRemovals are the additive and subtractive
	// halves of the soft-reference patch, in first-occurrence order.
	// dependencyClear replaces the whole stored set with nothing and cannot be
	// combined with either, exactly as app.DependencyPatch requires. All three
	// empty means the edit never mentioned a dependency, which preserves the
	// stored set.
	dependencyAdds     []CapturedDependencyRef
	dependencyRemovals []CapturedDependencyRef
	dependencyClear    bool

	// preview is a detached task value reflecting every accepted root patch.
	// Descendants and workspace collections are deliberately never rewritten.
	Preview models.Task
}

// ParseQuickEdit parses and validates inline task edits. The optional project
// slice supplies exact-slug cross-project resolution without widening legacy
// callers that only edit within one project.
func ParseQuickEdit(input string, task *models.Task, project *models.Project, projectSets ...[]*models.Project) (*QuickEditResult, error) {
	result, errMsg := parseQuickEditShared(input, task, project, flattenProjectSets(project, projectSets), true)
	if errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}
	return result, nil
}

// parseQuickEditForPreview returns the same pure overlay as strict parsing while
// preserving the first validation error for live feedback.
func parseQuickEditForPreview(input string, task *models.Task, project *models.Project, projectSets ...[]*models.Project) (*QuickEditResult, string) {
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

func parseQuickEditShared(input string, task *models.Task, project *models.Project, allProjects []*models.Project, strict bool) (*QuickEditResult, string) {
	parser := quickEditParser{
		parsed:           textgrammar.Parse(input, textgrammar.SurfaceEdit),
		result:           &QuickEditResult{ParentMode: QuickEditParentPreserve},
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
	parser.applyReview()
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
	p.result.ContextSet = true
	if tokens[0].Value == "0" {
		p.result.ContextClear = true
		return false
	}
	context, err := models.NormalizeTaskContext(tokens[0].Value)
	if err != nil {
		definition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, textgrammar.FieldContext)
		return p.setError(fmt.Sprintf("invalid context %s%s: %s", definition.Trigger, tokens[0].Value, err))
	}
	p.result.ContextValue = context
	return false
}
