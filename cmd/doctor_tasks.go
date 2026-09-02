package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func checkMissingTaskFiles(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		taskFile := filepath.Join(project.Path, "tasks.md")
		if _, err := os.Stat(taskFile); os.IsNotExist(err) {
			results.missingTaskFiles = append(results.missingTaskFiles, doctorIssue{
				project: project.Name,
				detail:  taskFile,
			})
		}
	}
}

func checkEmptyProjects(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		if len(project.Tasks) == 0 {
			results.emptyProjects = append(results.emptyProjects, doctorIssue{
				project: project.Name,
			})
		}
	}
}

func checkDuplicateIDs(projects []*models.Project, results *doctorResults) {
	globalIDs := make(map[string]string) // taskID -> project name

	for _, project := range projects {
		localIDs := make(map[string]bool)
		for _, task := range project.Tasks {
			// Check within project
			if localIDs[task.ID] {
				results.duplicateIDs = append(results.duplicateIDs, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("duplicate within project: %s", task.Title),
				})
			}
			localIDs[task.ID] = true

			// Check across projects
			if otherProject, exists := globalIDs[task.ID]; exists {
				results.duplicateIDs = append(results.duplicateIDs, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("also exists in project %s", otherProject),
				})
			} else {
				globalIDs[task.ID] = project.Name
			}
		}
	}
}

func checkDuplicateShortIDs(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		seen := make(map[int]string) // shortID -> first task title
		for _, task := range project.Tasks {
			if first, exists := seen[task.ShortID]; exists {
				results.duplicateShortIDs = append(results.duplicateShortIDs, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("short ID #%d used by %q and %q", task.ShortID, first, task.Title),
				})
			} else {
				seen[task.ShortID] = task.Title
			}
		}
	}
}

func checkBrokenParentRefs(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		taskMap := buildTaskMap(project)
		for _, task := range project.Tasks {
			if task.ParentID != nil && *task.ParentID != "" {
				if _, exists := taskMap[*task.ParentID]; !exists {
					results.brokenParentRefs = append(results.brokenParentRefs, doctorIssue{
						project: project.Name,
						detail:  fmt.Sprintf("parent %s not found (task: %s)", *task.ParentID, task.Title),
					})
				}
			}
		}
	}
}

// checkCompletionTimestamps surfaces records that completion-age cleanup must
// exclude. Terminal tasks require a non-zero CompletedAt, while nonterminal
// tasks must not retain one; doctor reports both cases rather than inventing an
// age or silently repairing lifecycle history.
func checkCompletionTimestamps(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		for _, task := range project.Tasks {
			terminal := task.IsTerminalState()
			switch {
			case terminal && (task.CompletedAt == nil || task.CompletedAt.IsZero()):
				results.invalidCompletionTimestamps = append(results.invalidCompletionTimestamps, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("terminal task #%d %q has no valid completed_at", task.ShortID, task.Title),
				})
			case !terminal && task.CompletedAt != nil:
				results.invalidCompletionTimestamps = append(results.invalidCompletionTimestamps, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("nonterminal task #%d %q retains completed_at", task.ShortID, task.Title),
				})
			}
		}
	}
}

func fixIssues(projects []*models.Project) {
	dirtyProjects := make(map[string]bool)

	for _, project := range projects {
		// Fix duplicate short IDs by reassigning duplicates
		seenShortIDs := make(map[int]bool)
		for _, task := range project.Tasks {
			if seenShortIDs[task.ShortID] {
				newID := project.GetNextShortID()
				fmt.Printf("  Fixed: reassigned duplicate short ID #%d to #%d for task %s in %s\n", task.ShortID, newID, task.Title, project.Name)
				task.ShortID = newID
				dirtyProjects[project.ID] = true
			}
			seenShortIDs[task.ShortID] = true
		}

		taskMap := buildTaskMap(project)
		for _, task := range project.Tasks {
			// Fix broken parent refs
			if task.ParentID != nil && *task.ParentID != "" {
				if _, exists := taskMap[*task.ParentID]; !exists {
					fmt.Printf("  Fixed: cleared broken parent ref %s from task %s in %s\n", *task.ParentID, task.Title, project.Name)
					task.ParentID = nil
					dirtyProjects[project.ID] = true
				}
			}
		}

		if dirtyProjects[project.ID] {
			if err := storage.SaveProjectLocked(project); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to save project %s: %v\n", project.Name, err)
			}
		}
	}
}

func buildTaskMap(project *models.Project) map[string]*models.Task {
	m := make(map[string]*models.Task, len(project.Tasks))
	for _, task := range project.Tasks {
		m[task.ID] = task
	}
	return m
}
