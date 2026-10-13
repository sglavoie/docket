package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// planningRepair carries the planning state a fix pass would commit, computed
// once by the check so the report and the repair cannot disagree.
type planningRepair struct {
	repaired *config.PlanningState
	changed  bool
}

// checkPlanningMembership audits Today and Next against the task states the
// projects actually hold, and returns the pruned membership a --fix pass commits.
//
// This is the skew two machines produce most often: one machine completes or
// deletes a task, the other machine's planning.json still names it. The task
// state is the authority — completing clears planning, and Today and Next are
// mutually exclusive — so a locator disagreeing with it is repairable without
// guessing, which is what makes this the third fixable bucket.
//
// A locator whose project is cataloged but failed to load is never pruned. The
// target may be perfectly addressable and only this run could not read it;
// dropping it would be exactly the silent data loss the check exists to catch.
func checkPlanningMembership(runtime *config.RuntimeState, loadedProjects []*models.Project, results *doctorResults) *planningRepair {
	planning := runtime.Planning()
	if planning == nil {
		results.planningIssues = append(results.planningIssues, doctorIssue{
			project: "Planning",
			detail:  "planning state is unavailable",
		})
		return &planningRepair{}
	}
	if err := planning.Validate(); err != nil {
		results.planningIssues = append(results.planningIssues, doctorIssue{
			project: "Planning",
			detail:  err.Error(),
		})
	}

	cataloged := make(map[string]string)
	for _, project := range runtime.AllProjects() {
		if project != nil {
			cataloged[project.ID] = project.Name
		}
	}
	loaded := make(map[string]map[string]*models.Task, len(loadedProjects))
	for _, project := range loadedProjects {
		if project == nil {
			continue
		}
		// No tasks.md loads as an empty project. That is a project this run
		// could not read, not one that owns nothing, so it stays out of loaded
		// and its entries take the unavailable branch below.
		if storage.ProjectTasksFileMissing(project) {
			continue
		}
		tasks := make(map[string]*models.Task, len(project.Tasks))
		for _, task := range project.Tasks {
			if task != nil && task.ID != "" {
				tasks[task.ID] = task
			}
		}
		loaded[project.ID] = tasks
	}

	repaired := &config.PlanningState{}
	claimed := make(map[config.ProjectTaskLocator]string)
	changed := false

	for _, collection := range []struct {
		name  string
		items []config.ProjectTaskLocator
		keep  *[]config.ProjectTaskLocator
	}{
		{name: "Today", items: planning.Today, keep: &repaired.Today},
		{name: "Next", items: planning.Next, keep: &repaired.Next},
	} {
		seen := make(map[config.ProjectTaskLocator]bool, len(collection.items))
		for _, item := range collection.items {
			detail, prune, unavailable := planningMembershipDefect(collection.name, item, cataloged, loaded, seen, claimed)
			seen[item] = true
			switch {
			case detail == "":
				*collection.keep = append(*collection.keep, item)
				claimed[item] = collection.name
			case unavailable:
				results.unavailablePlanningRefs = append(results.unavailablePlanningRefs, doctorIssue{
					project: "Planning",
					detail:  detail,
				})
				*collection.keep = append(*collection.keep, item)
				claimed[item] = collection.name
			default:
				results.planningIssues = append(results.planningIssues, doctorIssue{
					project: "Planning",
					detail:  detail,
				})
				if prune {
					changed = true
				} else {
					*collection.keep = append(*collection.keep, item)
					claimed[item] = collection.name
				}
			}
		}
	}
	return &planningRepair{repaired: repaired, changed: changed}
}

// planningMembershipDefect describes what is wrong with one planning locator.
// prune reports whether dropping it is the correct repair; unavailable marks the
// one finding that is informational because this run simply could not read the
// project.
func planningMembershipDefect(
	collection string,
	item config.ProjectTaskLocator,
	cataloged map[string]string,
	loaded map[string]map[string]*models.Task,
	seen map[config.ProjectTaskLocator]bool,
	claimed map[config.ProjectTaskLocator]string,
) (detail string, prune bool, unavailable bool) {
	if !item.Valid() {
		return fmt.Sprintf("%s holds an entry with incomplete identity", collection), true, false
	}
	if seen[item] {
		return fmt.Sprintf("%s repeats task %s in project %s", collection, item.TaskID, item.ProjectID), true, false
	}
	if other, exists := claimed[item]; exists {
		return fmt.Sprintf(
			"task %s in project %s is in both %s and %s, which are mutually exclusive; %s wins",
			item.TaskID, item.ProjectID, other, collection, other,
		), true, false
	}
	name, known := cataloged[item.ProjectID]
	if !known {
		return fmt.Sprintf("%s names unknown project %s", collection, item.ProjectID), true, false
	}
	tasks, ok := loaded[item.ProjectID]
	if !ok {
		return fmt.Sprintf("%s entry in project %s is unavailable: its project did not load", collection, name), false, true
	}
	task, exists := tasks[item.TaskID]
	if !exists {
		return fmt.Sprintf("%s names task %s which project %s does not own", collection, item.TaskID, name), true, false
	}
	if task.IsTerminalState() {
		return fmt.Sprintf("%s names terminal task #%d %q in project %s", collection, task.ShortID, task.Title, name), true, false
	}
	return "", false, false
}

// fixPlanningMembership commits the pruned membership through the runtime's
// checked planning commit, so a repair takes part in the same optimistic
// concurrency every other planning write does.
func fixPlanningMembership(runtime *config.RuntimeState, repair *planningRepair) (string, error) {
	if repair == nil || !repair.changed || repair.repaired == nil {
		return "", nil
	}
	before := runtime.Planning()
	removed := 0
	if before != nil {
		removed = len(before.Today) + len(before.Next) - len(repair.repaired.Today) - len(repair.repaired.Next)
	}
	if err := runtime.CommitPlanning(repair.repaired); err != nil {
		return "", fmt.Errorf("repair planning membership: %w", err)
	}
	return fmt.Sprintf("pruned %s from Today and Next", Pluralize(removed, "planning entry", "planning entries")), nil
}

// checkOrphanedAttachments compares each project's attachment metadata against
// the bytes under its .attachments tree.
//
// Metadata naming a file that is not there is an issue: the attachment is dead
// to every reader. Bytes no metadata names are informational, because they may be
// the only surviving copy of an attachment whose tasks.md lost the entry, and
// deleting them is never a repair a scan should choose.
