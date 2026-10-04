package grammar

import (
	"fmt"
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

func appendUnique(values []string, value string) []string {
	if !slices.Contains(values, value) {
		return append(values, value)
	}
	return values
}

func ProjectLabel(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.Slug != "" {
		return project.Slug
	}
	return project.Name
}

// ValidQuickEditParents is also the dynamic completion source for the row
// editor: it derives candidates from the effective project and applies the same
// cycle and depth checks as strict parsing.
func ValidQuickEditParents(task *models.Task, source, effective *models.Project) []*models.Task {
	if effective == nil {
		return nil
	}
	result := make([]*models.Task, 0, len(effective.Tasks))
	for _, candidate := range effective.Tasks {
		if candidate == nil || candidate.IsComplete() {
			continue
		}
		if InvalidQuickEditParent(task, source, effective, candidate) == "" {
			result = append(result, candidate)
		}
	}
	return result
}

// QuickEditParentCandidates resolves the effective project from the current
// draft before producing parent completion candidates. It intentionally ignores
// the draft's parent token: completion is about the choices that may replace it.
func QuickEditParentCandidates(input string, task *models.Task, source *models.Project, allProjects []*models.Project) ([]*models.Task, string) {
	parsed := textgrammar.Parse(input, textgrammar.SurfaceEdit)
	for _, diagnostic := range parsed.Diagnostics {
		if diagnostic.Field == textgrammar.FieldProject {
			return nil, diagnostic.Message
		}
	}
	effective := source
	if tokens := parsed.TokensFor(textgrammar.FieldProject); len(tokens) > 0 {
		projects := flattenProjectSets(source, [][]*models.Project{allProjects})
		effective = resolveProject(tokens[0].Value, projects)
		if effective == nil {
			return nil, fmt.Sprintf("project not found: %q (%s)", tokens[0].Value, projectSlugHint(tokens[0].Value, projects))
		}
	}
	return ValidQuickEditParents(task, source, effective), ""
}

// QuickEditChildCandidates offers the tasks the draft can still adopt. It reads
// the whole line — the %project route, the parent patch, and every <child token
// already accepted — because an adoption is legal only against the hierarchy the
// finished line produces. It withholds exactly what strict parsing would refuse:
// the edited task itself, an alias of a task the draft already names, a
// projected ancestor (which covers the task named by >parent), and a candidate
// whose subtree would overflow the depth limit.
func QuickEditChildCandidates(input string, task *models.Task, source *models.Project, allProjects []*models.Project) ([]*models.Task, string) {
	parsed := textgrammar.Parse(input, textgrammar.SurfaceEdit)
	for _, diagnostic := range parsed.Diagnostics {
		if diagnostic.Field == textgrammar.FieldProject {
			return nil, diagnostic.Message
		}
	}
	effective := source
	draft := &QuickEditResult{ParentMode: QuickEditParentPreserve}
	if tokens := parsed.TokensFor(textgrammar.FieldProject); len(tokens) > 0 {
		projects := flattenProjectSets(source, [][]*models.Project{allProjects})
		effective = resolveProject(tokens[0].Value, projects)
		if effective == nil {
			return nil, fmt.Sprintf("project not found: %q (%s)", tokens[0].Value, projectSlugHint(tokens[0].Value, projects))
		}
		draft.ParentMode = QuickEditParentRoot
	}
	if task == nil || source == nil || effective == nil {
		return nil, ""
	}
	if tokens := parsed.TokensFor(textgrammar.FieldParent); len(tokens) > 0 {
		draft.ParentMode, draft.Parent = draftedQuickEditParent(tokens[0].Value, task, source, effective, draft.ParentMode)
	}
	adopted := map[string]bool{}
	for _, token := range parsed.TokensFor(textgrammar.FieldChild) {
		child, err := resolveChild(token.Value, effective)
		if err != nil || child == nil || adopted[child.ID] {
			continue
		}
		adopted[child.ID] = true
		draft.Children = append(draft.Children, child)
	}
	hierarchy := newQuickEditHierarchy(task, source, effective, draft)
	candidates := make([]*models.Task, 0, len(effective.Tasks))
	for _, candidate := range StructuralCandidates(effective) {
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
		return QuickEditParentRoot, nil
	}
	parent, err := resolveParent(value, effective)
	if err != nil || parent == nil || InvalidQuickEditParent(task, source, effective, parent) != "" {
		return mode, nil
	}
	return QuickEditParentSet, parent
}

func InvalidQuickEditParent(task *models.Task, source, effective *models.Project, parent *models.Task) string {
	if task == nil || source == nil || effective == nil || parent == nil {
		return "task and parent must belong to loaded projects"
	}
	if effective.GetTaskByID(parent.ID) != parent {
		return fmt.Sprintf("parent #%d is stale or not in project %q", parent.ShortID, ProjectLabel(effective))
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
			return 0, fmt.Errorf("task %q is missing from project %q", taskID, ProjectLabel(project))
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
func invalidQuickEditHierarchy(task *models.Task, source, effective *models.Project, result *QuickEditResult) string {
	if len(result.Children) == 0 {
		return ""
	}
	if task == nil || source == nil || effective == nil {
		return "task and children must belong to loaded projects"
	}
	hierarchy := newQuickEditHierarchy(task, source, effective, result)
	for _, child := range result.Children {
		if hierarchy.childIsAncestor(task, child) {
			return fmt.Sprintf("child #%d is an ancestor of the edited task", child.ShortID)
		}
	}
	if hierarchy.depthErr != nil {
		return hierarchy.depthErr.Error()
	}
	for _, child := range result.Children {
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

func newQuickEditHierarchy(task *models.Task, source, effective *models.Project, result *QuickEditResult) quickEditHierarchy {
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
func projectedQuickEditParents(task *models.Task, source, effective *models.Project, result *QuickEditResult) map[string]*string {
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
	switch result.ParentMode {
	case QuickEditParentRoot:
		parents[task.ID] = nil
	case QuickEditParentSet:
		parentID := result.Parent.ID
		parents[task.ID] = &parentID
	case QuickEditParentPreserve:
	}
	for _, child := range result.Children {
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
