package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check project data for integrity issues",
	Long:  `Scans all projects for data integrity issues like orphaned references, duplicate IDs, and broken links.`,
	RunE:  runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Automatically fix issues where possible")
	rootCmd.AddCommand(doctorCmd)
}

type doctorIssue struct {
	project string
	task    string
	detail  string
}

type doctorResults struct {
	orphanedCrossRefs []doctorIssue
	duplicateIDs      []doctorIssue
	deadDependencies  []doctorIssue
	missingTaskFiles  []doctorIssue
	brokenParentRefs  []doctorIssue
	emptyProjects     []doctorIssue
}

func (r *doctorResults) totalIssues() int {
	return len(r.orphanedCrossRefs) + len(r.duplicateIDs) + len(r.deadDependencies) +
		len(r.missingTaskFiles) + len(r.brokenParentRefs)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	fix, _ := cmd.Flags().GetBool("fix")

	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	projects := cfg.GetAllLoadableProjects()

	// Load all projects (including archived) so cross-project ref checks
	// don't report false positives for refs pointing to archived projects.
	allProjects := cfg.Projects
	var loadedProjects []*models.Project
	for _, project := range allProjects {
		if err := storage.LoadProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", project.Name, err)
			continue
		}
		project.BuildTaskTree()
		loadedProjects = append(loadedProjects, project)
	}

	results := &doctorResults{}

	checkMissingTaskFiles(projects, results)
	checkEmptyProjects(loadedProjects, results)
	checkDuplicateIDs(loadedProjects, results)
	checkBrokenParentRefs(loadedProjects, results)
	checkDeadDependencies(loadedProjects, results)
	checkOrphanedCrossRefs(loadedProjects, cfg, results)

	if fix {
		fixIssues(loadedProjects, cfg)
	}

	printResults(results)

	if results.totalIssues() > 0 {
		if fix {
			fmt.Println("\nSome issues were fixed. Re-run 'tt doctor' to verify.")
		} else {
			fmt.Println("\nRun 'tt doctor --fix' to automatically fix fixable issues.")
		}
		return fmt.Errorf("found %d issue(s)", results.totalIssues())
	}

	if len(results.emptyProjects) > 0 {
		fmt.Println("\nAll checks passed (with informational notes above).")
	} else {
		fmt.Println("\nAll checks passed.")
	}
	return nil
}

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
					task:    task.ID,
					detail:  fmt.Sprintf("duplicate within project: %s", task.Title),
				})
			}
			localIDs[task.ID] = true

			// Check across projects
			if otherProject, exists := globalIDs[task.ID]; exists {
				results.duplicateIDs = append(results.duplicateIDs, doctorIssue{
					project: project.Name,
					task:    task.ID,
					detail:  fmt.Sprintf("also exists in project %s", otherProject),
				})
			} else {
				globalIDs[task.ID] = project.Name
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
						task:    task.ID,
						detail:  fmt.Sprintf("parent %s not found (task: %s)", *task.ParentID, task.Title),
					})
				}
			}
		}
	}
}

func checkDeadDependencies(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		taskMap := buildTaskMap(project)
		for _, task := range project.Tasks {
			for _, dep := range task.DependsOn {
				ref := models.ParseTaskRef(dep)
				if ref.IsCrossProject() {
					continue // handled by checkOrphanedCrossRefs
				}
				if _, exists := taskMap[ref.TaskID]; !exists {
					results.deadDependencies = append(results.deadDependencies, doctorIssue{
						project: project.Name,
						task:    task.ID,
						detail:  fmt.Sprintf("depends on nonexistent task %s (task: %s)", ref.TaskID, task.Title),
					})
				}
			}
			for _, rel := range task.RelatedTo {
				ref := models.ParseTaskRef(rel)
				if ref.IsCrossProject() {
					continue // handled by checkOrphanedCrossRefs
				}
				if _, exists := taskMap[ref.TaskID]; !exists {
					results.deadDependencies = append(results.deadDependencies, doctorIssue{
						project: project.Name,
						task:    task.ID,
						detail:  fmt.Sprintf("related to nonexistent task %s (task: %s)", ref.TaskID, task.Title),
					})
				}
			}
		}
	}
}

func checkOrphanedCrossRefs(projects []*models.Project, cfg interface{ GetProject(string) *models.Project }, results *doctorResults) {
	// Build a map of all loaded projects for task lookup
	projectTaskMaps := make(map[string]map[string]bool)
	for _, project := range projects {
		projectTaskMaps[project.ID] = buildTaskIDSet(project)
	}

	for _, project := range projects {
		for _, task := range project.Tasks {
			checkCrossRefs(project.Name, task, task.DependsOn, "DependsOn", cfg, projectTaskMaps, results)
			checkCrossRefs(project.Name, task, task.RelatedTo, "RelatedTo", cfg, projectTaskMaps, results)
		}
	}
}

func checkCrossRefs(projectName string, task *models.Task, refs []string, refType string, cfg interface{ GetProject(string) *models.Project }, projectTaskMaps map[string]map[string]bool, results *doctorResults) {
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if !parsed.IsCrossProject() {
			continue
		}

		targetProject := cfg.GetProject(parsed.ProjectID)
		if targetProject == nil {
			results.orphanedCrossRefs = append(results.orphanedCrossRefs, doctorIssue{
				project: projectName,
				task:    task.ID,
				detail:  fmt.Sprintf("%s ref %s: project %s not found (task: %s)", refType, ref, parsed.ProjectID, task.Title),
			})
			continue
		}

		taskIDs, loaded := projectTaskMaps[targetProject.ID]
		if !loaded {
			results.orphanedCrossRefs = append(results.orphanedCrossRefs, doctorIssue{
				project: projectName,
				task:    task.ID,
				detail:  fmt.Sprintf("%s ref %s: project %s could not be loaded (task: %s)", refType, ref, parsed.ProjectID, task.Title),
			})
			continue
		}

		if !taskIDs[parsed.TaskID] {
			results.orphanedCrossRefs = append(results.orphanedCrossRefs, doctorIssue{
				project: projectName,
				task:    task.ID,
				detail:  fmt.Sprintf("%s ref %s: task not found in project %s (task: %s)", refType, ref, targetProject.Name, task.Title),
			})
		}
	}
}

func fixIssues(projects []*models.Project, cfg interface{ GetProject(string) *models.Project }) {
	projectTaskMaps := make(map[string]map[string]bool)
	for _, project := range projects {
		projectTaskMaps[project.ID] = buildTaskIDSet(project)
	}

	dirtyProjects := make(map[string]bool)

	for _, project := range projects {
		taskMap := buildTaskMap(project)
		for _, task := range project.Tasks {
			// Fix orphaned cross-project refs and dead dependencies in DependsOn
			if cleaned := cleanRefs(task.DependsOn, cfg, projectTaskMaps, taskMap); len(cleaned) != len(task.DependsOn) {
				removed := len(task.DependsOn) - len(cleaned)
				fmt.Printf("  Fixed: removed %d broken DependsOn ref(s) from task %s in %s\n", removed, task.Title, project.Name)
				task.DependsOn = cleaned
				dirtyProjects[project.ID] = true
			}

			// Fix orphaned cross-project refs and dead same-project refs in RelatedTo
			if cleaned := cleanRefs(task.RelatedTo, cfg, projectTaskMaps, taskMap); len(cleaned) != len(task.RelatedTo) {
				removed := len(task.RelatedTo) - len(cleaned)
				fmt.Printf("  Fixed: removed %d broken RelatedTo ref(s) from task %s in %s\n", removed, task.Title, project.Name)
				task.RelatedTo = cleaned
				dirtyProjects[project.ID] = true
			}

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

// cleanRefs filters DependsOn refs, removing broken same-project and cross-project refs.
func cleanRefs(refs []string, cfg interface{ GetProject(string) *models.Project }, projectTaskMaps map[string]map[string]bool, taskMap map[string]*models.Task) []string {
	var valid []string
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if parsed.IsCrossProject() {
			if isValidCrossRef(parsed, cfg, projectTaskMaps) {
				valid = append(valid, ref)
			}
		} else {
			if _, exists := taskMap[parsed.TaskID]; exists {
				valid = append(valid, ref)
			}
		}
	}
	return valid
}

// cleanCrossRefs filters only cross-project refs, keeping same-project refs untouched.
func cleanCrossRefs(refs []string, cfg interface{ GetProject(string) *models.Project }, projectTaskMaps map[string]map[string]bool) []string {
	var valid []string
	for _, ref := range refs {
		parsed := models.ParseTaskRef(ref)
		if parsed.IsCrossProject() {
			if isValidCrossRef(parsed, cfg, projectTaskMaps) {
				valid = append(valid, ref)
			}
		} else {
			valid = append(valid, ref)
		}
	}
	return valid
}

func isValidCrossRef(ref models.TaskRef, cfg interface{ GetProject(string) *models.Project }, projectTaskMaps map[string]map[string]bool) bool {
	targetProject := cfg.GetProject(ref.ProjectID)
	if targetProject == nil {
		return false
	}
	taskIDs, loaded := projectTaskMaps[targetProject.ID]
	if !loaded {
		return false
	}
	return taskIDs[ref.TaskID]
}

func buildTaskMap(project *models.Project) map[string]*models.Task {
	m := make(map[string]*models.Task, len(project.Tasks))
	for _, task := range project.Tasks {
		m[task.ID] = task
	}
	return m
}

func buildTaskIDSet(project *models.Project) map[string]bool {
	m := make(map[string]bool, len(project.Tasks))
	for _, task := range project.Tasks {
		m[task.ID] = true
	}
	return m
}

func printResults(results *doctorResults) {
	printSection("Missing task files", results.missingTaskFiles)
	printSection("Duplicate task IDs", results.duplicateIDs)
	printSection("Broken parent refs", results.brokenParentRefs)
	printSection("Dead dependency links", results.deadDependencies)
	printSection("Orphaned cross-project refs", results.orphanedCrossRefs)
	printInfoSection("Empty projects", results.emptyProjects)
}

func printSection(name string, issues []doctorIssue) {
	fmt.Printf("\n%s: %d\n", name, len(issues))
	for _, issue := range issues {
		fmt.Printf("  [%s] %s\n", issue.project, issue.detail)
	}
}

func printInfoSection(name string, issues []doctorIssue) {
	fmt.Printf("\n%s (info): %d\n", name, len(issues))
	for _, issue := range issues {
		fmt.Printf("  %s\n", issue.project)
	}
}
