package capturetext

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// DependencyRelationship names the sigil family in every reference error, the
// way "parent" and "child" do for the structural sigils.
const DependencyRelationship = "dependency"

// DependencyClearValue is the edit-only clear form's value. It is the same
// literal every other clear operation uses; the grammar owns whether a surface
// advertises it, and the parsers refuse it by name where it is not offered.
const DependencyClearValue = "0"

// CapturedDependencyRef is one resolved ~task or -~task reference. The display
// fields are copied out of the task the reference named rather than kept as a
// pointer, so a parse result can be previewed and mapped into a request without
// holding a handle on a view-owned task.
//
// resolved reports whether a loaded task backed the reference. A removal may
// name a target this workspace never loaded — that is exactly the reference a
// user most wants to drop — so an unresolved but well-shaped stable locator is a
// legitimate value here, carrying no display fields.
type CapturedDependencyRef struct {
	Locator     app.TaskLocator
	shortID     int
	title       string
	projectName string
	Terminal    bool
	Resolved    bool
}

// Label renders the reference for a preview or an error. A resolved reference
// reads as the project-qualified short ID a user typed; an unresolved one falls
// back to the durable locator, which is all that is known about it.
func (r CapturedDependencyRef) Label() string {
	if !r.Resolved {
		return r.Locator.Reference().Locator()
	}
	if r.projectName == "" {
		return fmt.Sprintf("#%d %s", r.shortID, r.title)
	}
	return fmt.Sprintf("%s #%d %s", r.projectName, r.shortID, r.title)
}

// ResolveDependencyReference resolves one ~ or -~ value against the workspace.
//
// A bare alias is an exact short ID or a unique case-insensitive title prefix in
// the effective project — the same rule >parent and <child use, so a dependency
// is addressed the way every other task reference is. A qualified alias splits
// on the first colon and resolves the head as an exact project slug, which is
// what lets a reference cross a project boundary that <child deliberately never
// does. A locator whose head names no loaded slug is tried as a stable project
// ID, and finally accepted as a shape-only locator so a stored reference stays
// removable after its target has been archived or deleted.
//
// It reports an ambiguous alias as an error and an unresolvable one as a zero
// reference, leaving the caller to word the refusal for its own surface.
func ResolveDependencyReference(ref string, effective *models.Project, allProjects []*models.Project) (CapturedDependencyRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return CapturedDependencyRef{}, nil
	}
	head, tail, qualified := strings.Cut(ref, models.TaskReferenceSeparator)
	if !qualified {
		task, err := ResolveStructuralReference(ref, effective, DependencyRelationship)
		if err != nil || task == nil {
			return CapturedDependencyRef{}, err
		}
		return DependencyRefFor(effective, task), nil
	}
	if project := ResolveProject(head, allProjects); project != nil {
		task, err := ResolveStructuralReference(tail, project, DependencyRelationship)
		if err != nil || task == nil {
			return CapturedDependencyRef{}, err
		}
		return DependencyRefFor(project, task), nil
	}
	if project := ProjectByStableID(head, allProjects); project != nil {
		if task := project.GetTaskByID(tail); task != nil {
			return DependencyRefFor(project, task), nil
		}
	}
	// Shape-only fallback. Validation is the domain's, so a locator this parser
	// accepts is exactly one storage would.
	if models.ValidateTaskReference(head, tail) == nil {
		return CapturedDependencyRef{Locator: app.TaskLocator{ProjectID: head, TaskID: tail}}, nil
	}
	return CapturedDependencyRef{}, nil
}

func ProjectByStableID(id string, allProjects []*models.Project) *models.Project {
	for _, project := range allProjects {
		if project != nil && project.ID == id {
			return project
		}
	}
	return nil
}

func DependencyRefFor(project *models.Project, task *models.Task) CapturedDependencyRef {
	ref := CapturedDependencyRef{
		Locator:  app.TaskLocator{TaskID: task.ID},
		shortID:  task.ShortID,
		title:    task.Title,
		Terminal: task.IsTerminalState(),
		Resolved: true,
	}
	if project != nil {
		ref.Locator.ProjectID = project.ID
		ref.projectName = project.Name
	}
	return ref
}

// DependencyAlias is the token text a completion inserts for one candidate: a
// bare short ID when the target already lives in the effective project, and a
// slug-qualified one otherwise. Keeping the local form short is what makes the
// common same-project reference the shortest thing to type, while the qualified
// form stays resolvable from any project.
func DependencyAlias(project *models.Project, task *models.Task, effective *models.Project) string {
	local := fmt.Sprintf("%d", task.ShortID)
	if project == nil || (effective != nil && project.ID == effective.ID) {
		return local
	}
	if project.Slug == "" {
		return project.ID + models.TaskReferenceSeparator + task.ID
	}
	return project.Slug + models.TaskReferenceSeparator + local
}

// DependencyNotFoundMessage is the one wording both surfaces use for a value the
// grammar accepted but the workspace cannot address.
func DependencyNotFoundMessage(trigger, ref string) string {
	return fmt.Sprintf("dependency not found: %q (use %sshort-id, %stitle-prefix, or %sproject-slug:short-id)", ref, trigger, trigger, trigger)
}

// DependencyLocatorsOf projects resolved references into the locators an
// application request carries, in reference order.
func DependencyLocatorsOf(refs []CapturedDependencyRef) []app.TaskLocator {
	if len(refs) == 0 {
		return nil
	}
	locators := make([]app.TaskLocator, len(refs))
	for i, ref := range refs {
		locators[i] = ref.Locator
	}
	return locators
}

// AppendDependencyRef collapses repeated locators to their first occurrence,
// which is the order the set is stored in. It mirrors the application layer's
// addition dedupe so preview and commit can never disagree about order.
func AppendDependencyRef(refs []CapturedDependencyRef, ref CapturedDependencyRef) []CapturedDependencyRef {
	for _, existing := range refs {
		if existing.Locator == ref.Locator {
			return refs
		}
	}
	return append(refs, ref)
}
