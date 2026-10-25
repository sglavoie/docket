package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/grammar"
)

// The outline editor is the bulk-restructuring escape hatch: one project's task
// tree is dumped to $EDITOR, edited as text, and applied as a single
// transaction with one undo entry.
//
// Scope is defined by the project, not by the viewport. Dumping a complete model
// subtree is what makes indentation safe to reinterpret as parentage: there is
// no folded or filtered child off screen to orphan.

// outlineEditorModeRefusal is what a fixed-order mode answers `O` with. Today,
// Next, Due, and Recent promote rows across section boundaries, so indentation
// there is not parentage and reading it as parentage would corrupt hierarchy.
const outlineEditorModeRefusal = "Outline edit needs All, Inbox, or Backlog"

// outlineBufferPrefix and outlineBufferSuffix name the temp file the round trip
// writes. The suffix gives editors a Markdown mode, which is close enough to the
// buffer's shape to get useful highlighting.
const (
	outlineBufferPrefix = "tt-outline-"
	outlineBufferSuffix = ".md"
)

// outlineEditorRequest resolves what `O` would act on, or the refusal that stops
// it. An empty refusal with a nil project means there is simply nothing under
// the cursor to outline.
func (m *AppModel) outlineEditorRequest() (*models.Project, string, string) {
	out := m.activeOutliner()
	if out == nil {
		return nil, "", ""
	}
	mode, ok := chrome.OutlinerModeByID(out.PrimaryMode)
	if !ok || mode.FixedOrder() {
		return nil, "", outlineEditorModeRefusal
	}
	return out.CursorProject(), mode.Label, ""
}

// dumpOutlineBuffer renders a project's complete task tree and names every task
// it printed.
//
// The walk follows project.Tasks order rather than the built child lists,
// because that is the order the application layer reads sibling sequence back
// from; a dump that reordered siblings would report a change nobody made.
//
// The printed IDs are returned from the same walk rather than read off
// project.Tasks, because absence is a deletion now. A task whose ParentID names
// a task that no longer exists is neither a root nor anyone's child, so it never
// reaches the buffer — and a scope built from project.Tasks would let a line the
// user never saw be deleted by a line they never removed.
// projects is the loaded workspace population the dumped dependencies are
// addressed against. It is passed in rather than re-read here so the aliases the
// buffer prints and the ones the returned buffer resolves come from one view.
func dumpOutlineBuffer(project *models.Project, projects []*models.Project, modeLabel string, now time.Time) (string, []string, error) {
	if project == nil {
		return "", nil, fmt.Errorf("no project to outline")
	}
	if err := assertOutlineMarkersUnique(project); err != nil {
		return "", nil, err
	}

	var builder strings.Builder
	builder.WriteString(outlineBufferHeader(project.Name, modeLabel, now))

	children := make(map[string][]*models.Task, len(project.Tasks))
	var roots []*models.Task
	for _, task := range project.Tasks {
		switch {
		case task == nil:
		case task.ParentID == nil:
			roots = append(roots, task)
		default:
			children[*task.ParentID] = append(children[*task.ParentID], task)
		}
	}

	dumped := make([]string, 0, len(project.Tasks))
	var walk func(task *models.Task, depth int, insideHidden bool)
	walk = func(task *models.Task, depth int, insideHidden bool) {
		// Hidden work stays in the dump, since leaving it out would let a
		// removed parent orphan it, but every view hides it, so a comment the
		// parser skips names the subtree before anyone edits it unawares.
		if task.Hidden && !insideHidden {
			builder.WriteString(outlineHiddenSubtreeComment)
			builder.WriteString("\n")
		}
		builder.WriteString(outlineDumpLine(task, depth, project, projects))
		builder.WriteString("\n")
		dumped = append(dumped, task.ID)
		for _, child := range children[task.ID] {
			walk(child, depth+1, insideHidden || task.Hidden)
		}
	}
	for _, root := range roots {
		walk(root, 0, false)
	}
	return builder.String(), dumped, nil
}

// outlineHiddenSubtreeComment stands above each hidden subtree in a dump. It
// sits in the first column, which is what makes the buffer parser skip it.
const outlineHiddenSubtreeComment = "# hidden: the next line and its subtasks (wh unhides)"

// outlineBufferHeader is the comment block every dump opens with. It states the
// two rules a text buffer cannot show on its own: indentation is parentage, and
// a line is a complete declaration rather than a patch.
func outlineBufferHeader(projectName, modeLabel string, now time.Time) string {
	scope := "project: " + projectName
	if modeLabel != "" {
		scope += " · mode: " + modeLabel
	}
	return strings.Join([]string{
		fmt.Sprintf("# tt outline · %s · %s", scope, now.Format("2006-01-02 15:04")),
		fmt.Sprintf("# Indent = parentage (%d spaces). Sigils: %s", grammar.OutlineIndentWidth, outlineGrammarForms()),
		"# Lines are declarative: an omitted sigil clears that value.",
		fmt.Sprintf("# Keep %sxxxxxxxx%s to edit a task. No marker = new task.", grammar.OutlineMarkerPrefix, grammar.OutlineMarkerSuffix),
		"# Removing a line deletes that task; you confirm the list on return.",
		"",
		"",
	}, "\n")
}

func outlineGrammarForms() string {
	var forms []string
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceOutline) {
		forms = append(forms, definition.Forms...)
	}
	return strings.Join(forms, " ")
}

// outlineDumpLine renders one task as its complete current value, so an omitted
// sigil on the way back in is a deliberate clear rather than an absent
// instruction.
func outlineDumpLine(task *models.Task, depth int, scope *models.Project, projects []*models.Project) string {
	segments := []string{task.Title}
	priorityDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldPriority)
	dueDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldDue)
	tagDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldTag)
	contextDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldContext)
	destinationDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldDestination)
	pinnedDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldPinned)
	reviewDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldReview)
	dependencyDefinition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceOutline, textgrammar.FieldDependency)
	if task.Priority != models.PriorityUnset {
		segments = append(segments, fmt.Sprintf("%s%d", priorityDefinition.Trigger, task.Priority))
	}
	if task.DueDate != nil {
		segments = append(segments, dueDefinition.Trigger+chrome.DueGrammarValue(task))
	}
	for _, tag := range task.Tags {
		segments = append(segments, tagDefinition.Trigger+tag)
	}
	if task.Context != "" {
		segments = append(segments, contextDefinition.Trigger+task.Context)
	}
	// Every stored reference is printed, closed and unloaded targets included.
	// The line is a complete declaration, so a reference the dump withheld would
	// be silently deleted by a user who never saw it.
	for _, reference := range task.Dependencies {
		segments = append(segments, dependencyDefinition.Trigger+outlineDependencyAlias(reference, scope, projects))
	}
	// The lifecycle state is dumped, but Today and Next membership is not: an
	// omitted $today cannot remove a membership (the application layer preserves
	// planning on omission), so printing one would promise a symmetry the buffer
	// does not have. Planning survives the round trip untouched.
	segments = append(segments, destinationDefinition.Trigger+string(task.State()))
	if task.Pinned {
		segments = append(segments, pinnedDefinition.Trigger+"pin")
	}
	if task.Review {
		segments = append(segments, reviewDefinition.Trigger+"review")
	}

	line := strings.Repeat(" ", depth*grammar.OutlineIndentWidth) + strings.Join(segments, " ")
	return line + "  " + grammar.OutlineMarkerPrefix + grammar.OutlineMarkerFor(task.ID) + grammar.OutlineMarkerSuffix
}

// outlineDependencyAlias renders one stored reference as the token the buffer
// prints. A target in the dumped project reads as a bare short ID, a target in
// another loaded project as its slug-qualified short ID, and anything this
// workspace cannot address falls back to the durable project-id:task-id locator
// — which is exactly the form the parser accepts back unchanged, so a reference
// whose target is archived or deleted survives the round trip.
func outlineDependencyAlias(reference models.TaskReference, scope *models.Project, projects []*models.Project) string {
	target := scope
	if target == nil || target.ID != reference.ProjectID {
		target = capturetext.ProjectByStableID(reference.ProjectID, projects)
	}
	if target != nil {
		if task := target.GetTaskByID(reference.TaskID); task != nil {
			return capturetext.DependencyAlias(target, task, scope)
		}
	}
	return reference.Locator()
}

// assertOutlineMarkersUnique refuses to open the editor over a buffer whose
// short markers collide. A collision would make two lines name the same task,
// and discovering that after the user has edited for ten minutes is worse than
// never opening.
func assertOutlineMarkersUnique(project *models.Project) error {
	seen := make(map[string]string, len(project.Tasks))
	for _, task := range project.Tasks {
		if task == nil || task.ID == "" {
			continue
		}
		marker := grammar.OutlineMarkerFor(task.ID)
		if other, clash := seen[marker]; clash {
			return fmt.Errorf("tasks %s and %s share the marker %s%s%s", other, task.ID, grammar.OutlineMarkerPrefix, marker, grammar.OutlineMarkerSuffix)
		}
		seen[marker] = task.ID
	}
	return nil
}

// outlineDumpSource renders the buffer to edit from one application snapshot.
//
// The cursor only names which project to outline. The tree that reaches the
// editor and the revision the apply is bound to are read together, so a cursor
// holding a project the shell loaded minutes ago cannot dump a stale tree under
// a current revision.
func (m *AppModel) outlineDumpSource(projectID, modeLabel string) (*models.Project, string, string, []string, error) {
	project, revision, err := m.features.Outline.Project(projectID)
	if err != nil {
		return nil, "", "", nil, err
	}
	buffer, scopeTaskIDs, err := dumpOutlineBuffer(project, m.outlineDependencyProjects(project), modeLabel, time.Now())
	if err != nil {
		return nil, "", "", nil, err
	}
	return project, revision, buffer, scopeTaskIDs, nil
}

// outlineDependencyProjects is the population a dumped or parsed dependency is
// addressed against: the same non-archived list the inline edit surface resolves
// against, with the scope project substituted for the shell's copy of it.
//
// The substitution matters because the outline round trip loads its project from
// its own snapshot. Without it the buffer would resolve a same-project reference
// against a second, older tree, and a task the dump printed could fail to
// resolve on the way back in.
func (m *AppModel) outlineDependencyProjects(scope *models.Project) []*models.Project {
	available := m.availableProjects()
	projects := make([]*models.Project, 0, len(available)+1)
	replaced := false
	for _, project := range available {
		if scope != nil && project != nil && project.ID == scope.ID {
			projects = append(projects, scope)
			replaced = true
			continue
		}
		projects = append(projects, project)
	}
	if scope != nil && !replaced {
		projects = append(projects, scope)
	}
	return projects
}
