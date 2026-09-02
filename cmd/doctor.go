package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check workspace data for integrity issues",
	Long: `Validate the retained workspace domains, protected Unfiled project,
named lenses, transaction journal and quarantine, conflict bundles, lock
files, migration snapshots, planning membership, attachments, checklist and
comment structure, state clocks, project files, task identities, and
hierarchy references. Diagnostics are read-only unless --fix is explicitly
supplied; immutable migration snapshots are never changed.`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Automatically fix issues where possible")
	rootCmd.AddCommand(doctorCmd)
}

type doctorIssue struct {
	project string
	detail  string
}

type doctorResults struct {
	workspaceIssues             []doctorIssue
	workspaceNotes              []doctorIssue
	lensIssues                  []doctorIssue
	lensNotes                   []doctorIssue
	transactionIssues           []doctorIssue
	transactionNotes            []doctorIssue
	conflictIssues              []doctorIssue
	conflictNotes               []doctorIssue
	lockIssues                  []doctorIssue
	lockNotes                   []doctorIssue
	planningIssues              []doctorIssue
	unavailablePlanningRefs     []doctorIssue
	attachmentIssues            []doctorIssue
	attachmentNotes             []doctorIssue
	structureIssues             []doctorIssue
	stateClockIssues            []doctorIssue
	clockSkewNotes              []doctorIssue
	loadFailures                []doctorIssue
	duplicateIDs                []doctorIssue
	duplicateShortIDs           []doctorIssue
	missingTaskFiles            []doctorIssue
	brokenParentRefs            []doctorIssue
	invalidCompletionTimestamps []doctorIssue
	brokenTaskRefs              []doctorIssue
	unavailableTaskRefs         []doctorIssue
	dependencyCycles            []doctorIssue
	emptyProjects               []doctorIssue
}

func (r *doctorResults) totalIssues() int {
	return len(r.workspaceIssues) + len(r.loadFailures) +
		len(r.lensIssues) + len(r.transactionIssues) +
		len(r.conflictIssues) + len(r.lockIssues) +
		len(r.planningIssues) + len(r.attachmentIssues) +
		len(r.structureIssues) + len(r.stateClockIssues) +
		len(r.duplicateIDs) + len(r.duplicateShortIDs) +
		len(r.missingTaskFiles) + len(r.brokenParentRefs) +
		len(r.invalidCompletionTimestamps) + len(r.brokenTaskRefs)
}

func (r *doctorResults) fixableIssues() int {
	return len(r.duplicateShortIDs) + len(r.brokenParentRefs) + len(r.planningIssues)
}

// informationalNotes reports whether anything was printed that is worth a human
// glance without being an integrity failure, so a clean run can say so honestly.
func (r *doctorResults) informationalNotes() int {
	return len(r.emptyProjects) + len(r.workspaceNotes) +
		len(r.lensNotes) + len(r.transactionNotes) +
		len(r.conflictNotes) + len(r.lockNotes) +
		len(r.unavailablePlanningRefs) + len(r.attachmentNotes) +
		len(r.clockSkewNotes) + len(r.unavailableTaskRefs) +
		len(r.dependencyCycles)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	fix, _ := cmd.Flags().GetBool("fix")

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		// A quarantine refuses every workspace mutation, and loading the workspace
		// runs recovery, so the one bucket that most needs to be read is also the one
		// that stops the workspace from opening. Print the recovery step here rather
		// than let the diagnostic be unreachable exactly when it matters.
		if errors.Is(err, transaction.ErrRecoveryQuarantine) {
			dataDir := config.WorkspaceRootFromConfigPath(resolveConfigPath())
			fmt.Fprintf(os.Stderr, "\n%s\n", quarantineRecoveryStep(
				transaction.QuarantineDirFor(config.TransactionsDir(dataDir)),
			))
		}
		return err
	}

	everyProject := workspace.Runtime.AllProjects()
	var loadedProjects []*models.Project
	results := &doctorResults{}
	checkWorkspaceHealth(workspace, results)

	// Load every retained project, including archived projects, so terminal work
	// and hierarchy integrity remain visible.
	for _, project := range everyProject {
		if loadErr := storage.LoadProject(project); loadErr != nil {
			results.loadFailures = append(results.loadFailures, doctorIssue{
				project: project.Name,
				detail:  loadErr.Error(),
			})
			continue
		}
		project.BuildTaskTree()
		loadedProjects = append(loadedProjects, project)
	}

	// The four synced-state buckets that need loaded tasks continue the order
	// checkWorkspaceHealth started: planning membership, then attachments, then
	// checklist and comment structure, then the state clocks.
	planning := checkPlanningMembership(workspace.Runtime, loadedProjects, results)
	checkOrphanedAttachments(loadedProjects, results)
	checkChecklistAndComments(loadedProjects, results)
	checkStateClocks(loadedProjects, results)

	checkMissingTaskFiles(everyProject, results)
	checkEmptyProjects(loadedProjects, results)
	checkDuplicateIDs(loadedProjects, results)
	checkDuplicateShortIDs(loadedProjects, results)
	checkBrokenParentRefs(loadedProjects, results)
	checkCompletionTimestamps(loadedProjects, results)
	checkTaskReferences(everyProject, loadedProjects, results)
	checkDependencyCycles(loadedProjects, results)

	if fix {
		fixIssues(loadedProjects)
		fixPlanningMembership(workspace.Runtime, planning)
	}

	printResults(results)

	if results.totalIssues() > 0 {
		switch {
		case fix && results.fixableIssues() > 0:
			fmt.Println("\nSome issues were fixed. Re-run 'tt doctor' to verify.")
		case !fix && results.fixableIssues() > 0:
			fmt.Println("\nRun 'tt doctor --fix' to automatically fix fixable issues.")
		default:
			fmt.Println("\nResolve the reported integrity issues, then re-run 'tt doctor'.")
		}
		return fmt.Errorf("found %d issue(s)", results.totalIssues())
	}

	if results.informationalNotes() > 0 {
		fmt.Println("\nAll checks passed (with informational notes above).")
	} else {
		fmt.Println("\nAll checks passed.")
	}
	return nil
}
