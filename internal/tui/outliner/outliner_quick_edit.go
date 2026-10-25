package outliner

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// handleQuickEditMode handles input while in quick edit mode.
func (m *OutlinerModel) HandleQuickEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.QuickEdit.SetContext(m.QuickEdit.GetProject(), m.quickEditProjects())
	transition, result, cmd := m.QuickEdit.Handle(msg)
	if transition != grammar.QuickEditCommit || result == nil {
		return m, cmd
	}
	project := m.QuickEdit.GetProject()
	task := m.QuickEdit.GetTask()
	if project == nil || task == nil || project.GetTaskByID(task.ID) != task {
		m.QuickEdit.RejectCommit("task edit context is unavailable")
		return m, nil
	}
	source := taskref.For(task, project)
	relocation := m.CaptureOutlinerRelocation([]taskref.TaskIdentity{source})
	m.QuickEdit.BeginCommit()
	return m, m.taskItem.EditTaskLine(m.TaskLineEditRequest(source, result), relocation)
}

func (m *OutlinerModel) quickEditProjects() []*models.Project {
	if m.Project != nil {
		return []*models.Project{m.Project}
	}
	return m.Population.Projects
}

func TaskLineEditRequest(source taskref.TaskIdentity, result *grammar.QuickEditResult) app.TaskLineEditRequest {
	req := app.TaskLineEditRequest{
		Source:               source.Locator(),
		Title:                result.Title,
		DueSet:               result.DueDateSet,
		DeferSet:             result.DeferSet,
		TagsToAdd:            append([]string(nil), result.TagsToAdd...),
		TagsToRemove:         append([]string(nil), result.TagsToRemove...),
		DestinationProjectID: source.ProjectID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentPreserve},
	}
	if result.PrioritySet {
		priority := result.Priority
		req.Priority = &priority
	}
	if result.ContextSet {
		context := result.ContextValue
		if result.ContextClear {
			context = ""
		}
		req.Context = &context
	}
	if result.DueDateSet && !result.DueDateClear {
		req.Due = result.DueDate
	}
	if result.DeferSet && !result.DeferClear {
		req.Defer = result.DeferUntil
	}
	if result.DestinationSet {
		destination := result.Destination
		req.Destination = &destination
	}
	if result.PinnedSet {
		pinned := result.Pinned
		req.Pinned = &pinned
	}
	if result.ReviewSet {
		review := result.Review
		req.Review = &review
	}
	if result.TargetProject != nil {
		req.DestinationProjectID = result.TargetProject.ID
	}
	switch result.ParentMode {
	case grammar.QuickEditParentPreserve:
		// Preserve is already the request default.
	case grammar.QuickEditParentRoot:
		req.Parent.Mode = app.TaskLineParentRoot
	case grammar.QuickEditParentSet:
		req.Parent.Mode = app.TaskLineParentSet
		if result.Parent != nil {
			req.Parent.Parent = app.TaskLocator{
				ProjectID: req.DestinationProjectID,
				TaskID:    result.Parent.ID,
			}
		}
	}
	// A dependency locator is fully qualified at parse time and is independent of
	// where the edited task lands, so a cross-project move carries it unchanged.
	req.Dependencies = result.DependencyPatch()
	// Children resolve in the effective project, which is the destination
	// whenever the edit also crosses a boundary.
	for _, child := range result.Children {
		req.Children = append(req.Children, app.TaskLocator{
			ProjectID: req.DestinationProjectID,
			TaskID:    child.ID,
		})
	}
	return req
}

// TaskLineEditRequest binds every live inline edit to an explicit planning
// builder instead of inheriting whichever default a prior service call left
// installed. Cross-project edits carry a complete planning-move delta, so they
// use the structural builder that reconciles against the newest on-disk state.
func (m *OutlinerModel) TaskLineEditRequest(source taskref.TaskIdentity, result *grammar.QuickEditResult) app.TaskLineEditRequest {
	req := TaskLineEditRequest(source, result)
	req.PlanningArtifacts = coordinator.PlanningArtifacts(m.Runtime, m.configManager)
	if req.DestinationProjectID != source.ProjectID {
		req.PlanningArtifacts = coordinator.StructuralPlanningArtifacts(m.Runtime, m.configManager)
	}
	return req
}

// quickEditFooterEntries is the inline editor's footer spelling of each field
// it teaches: a short positive token and a plain noun that never repeats it.
// Clears and negations are absent on purpose; the `?` syntax guide states
// them, and a footer naming both halves of every field restated itself.
var quickEditFooterEntries = map[textgrammar.Field]string{
	textgrammar.FieldPriority:    "!1-3 priority",
	textgrammar.FieldTag:         "#tag tag",
	textgrammar.FieldContext:     "##ctx context",
	textgrammar.FieldDue:         "@date due",
	textgrammar.FieldDefer:       "@@date defer",
	textgrammar.FieldDestination: "$state status",
	textgrammar.FieldPinned:      "*pin pin",
	textgrammar.FieldReview:      "=review review",
	textgrammar.FieldProject:     "%slug project",
	textgrammar.FieldParent:      ">task parent",
	textgrammar.FieldChild:       "<task adopt",
	textgrammar.FieldDependency:  "~task depends",
}

// quickEditGrammarGuidance lists one entry per field the edit surface accepts,
// in teach order, so the footer reads in the order the syntax guide does and
// the two-row packer sheds from the end of what a user meets last.
func quickEditGrammarGuidance() []string {
	definitions := textgrammar.GuideDefinitions(textgrammar.SurfaceEdit)
	entries := make([]string, 0, len(quickEditFooterEntries))
	for _, definition := range definitions {
		if entry, ok := quickEditFooterEntries[definition.Field]; ok {
			entries = append(entries, entry)
		}
	}
	return entries
}
