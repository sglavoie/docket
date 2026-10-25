package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// checkTaskReferences audits the stored soft task references of every loaded
// project against the whole catalog. It reports malformed locators, repeated
// locators on one task, a task naming itself, a locator whose project the
// catalog does not know, and a locator whose project is loaded but does not own
// the task.
//
// A catalog-known project this run could not load is reported as an
// informational note, never a broken reference: the target may well exist and be
// perfectly addressable, and calling it dead would invite exactly the silent
// pruning this check exists to prevent. Nothing here is fixable — an externally
// corrupted reference is repaired by the user, not deleted by a scan.
func checkTaskReferences(everyProject, loadedProjects []*models.Project, results *doctorResults) {
	cataloged := make(map[string]bool, len(everyProject))
	for _, project := range everyProject {
		if project != nil {
			cataloged[project.ID] = true
		}
	}
	loadedTasks := make(map[string]map[string]bool, len(loadedProjects))
	for _, project := range loadedProjects {
		if project == nil {
			continue
		}
		loadedTasks[project.ID] = buildTaskMapPresence(project)
	}

	for _, project := range loadedProjects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || len(task.Dependencies) == 0 {
				continue
			}
			seen := make(map[models.TaskReference]bool, len(task.Dependencies))
			for _, reference := range task.Dependencies {
				detail := taskReferenceDefect(project, task, reference, seen, cataloged, loadedTasks)
				switch {
				case detail == "":
				case cataloged[reference.ProjectID] && loadedTasks[reference.ProjectID] == nil:
					results.unavailableTaskRefs = append(results.unavailableTaskRefs, doctorIssue{
						project: project.Name,
						detail:  detail,
					})
				default:
					results.brokenTaskRefs = append(results.brokenTaskRefs, doctorIssue{
						project: project.Name,
						detail:  detail,
					})
				}
				seen[reference] = true
			}
		}
	}
}

// taskReferenceDefect describes what is wrong with one stored reference, or ""
// when it resolves cleanly. The order of the tests is the order of severity: a
// locator that cannot be parsed is never also measured against the workspace.
func taskReferenceDefect(
	project *models.Project,
	task *models.Task,
	reference models.TaskReference,
	seen map[models.TaskReference]bool,
	cataloged map[string]bool,
	loadedTasks map[string]map[string]bool,
) string {
	if err := reference.Validate(); err != nil {
		return fmt.Sprintf("malformed reference on task %q: %v", task.Title, err)
	}
	if seen[reference] {
		return fmt.Sprintf("duplicate reference %s on task %q", reference.Locator(), task.Title)
	}
	if reference.ProjectID == project.ID && reference.TaskID == task.ID {
		return fmt.Sprintf("task %q references itself", task.Title)
	}
	if !cataloged[reference.ProjectID] {
		return fmt.Sprintf("reference %s on task %q names an unknown project", reference.Locator(), task.Title)
	}
	tasks, loaded := loadedTasks[reference.ProjectID]
	if !loaded {
		return fmt.Sprintf("reference %s on task %q is unavailable: its project did not load", reference.Locator(), task.Title)
	}
	if !tasks[reference.TaskID] {
		return fmt.Sprintf("reference %s on task %q names a task its project does not own", reference.Locator(), task.Title)
	}
	return ""
}

// checkDependencyCycles reports rings of soft references as an informational
// census, never an issue. A ring is legal: a soft reference gates nothing, so
// readiness, ordering, ranking and completion cannot deadlock on one, and
// refusing it would hand a reference an authority the grammar denies it. Naming
// it is still worth doing, because no single row can show one — a row names one
// hop, never a transitive walk — so a ring drawn by accident is otherwise
// invisible.
//
// The population is the widest one this run holds: every retained project,
// archived included, already loaded above.
func checkDependencyCycles(loadedProjects []*models.Project, results *doctorResults) {
	names := make(map[string]string, len(loadedProjects))
	for _, project := range loadedProjects {
		if project != nil {
			names[project.ID] = project.Name
		}
	}
	for _, cycle := range app.NewDependencyIndex(loadedProjects).Cycles() {
		locators := cycle.Locators()
		if len(locators) == 0 {
			continue
		}
		project := names[cycle[0].ProjectID]
		if project == "" {
			project = cycle[0].ProjectID
		}
		results.dependencyCycles = append(results.dependencyCycles, doctorIssue{
			project: project,
			detail:  "reference cycle: " + strings.Join(append(locators, locators[0]), " -> "),
		})
	}
}

func buildTaskMapPresence(project *models.Project) map[string]bool {
	present := make(map[string]bool, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil && task.ID != "" {
			present[task.ID] = true
		}
	}
	return present
}
