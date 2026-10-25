package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check workspace data for integrity issues",
	Long: `Validate the retained workspace domains, protected Unfiled project,
named lenses, transaction journal and quarantine, conflict bundles, lock
files, migration snapshots, planning membership, attachments, checklist and
comment structure, state clocks, project files, task identities, and
hierarchy references. Diagnostics are read-only unless --fix is explicitly
supplied; immutable migration snapshots are never changed.

--fix commits each repair as a journaled transaction checked against the
revision it read, then runs every check again, so the verdict describes the
workspace the fixes left behind. --json prints one JSON report on stdout and
nothing else.

Exit status: 0 when every check passes, 2 when doctor ran and found issues,
and 1 when doctor could not run or a repair failed.`,
	Example: `  docket doctor
  docket doctor --fix
  docket doctor --json`,
	Args:              noArgs,
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE:              runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "automatically fix issues where possible")
	doctorCmd.Flags().Bool("json", false, "print the result as JSON")
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

// doctorIssuesError reports that doctor ran to completion and found integrity
// issues. It exits 2 so a script can tell "the workspace needs attention" apart
// from "doctor could not run", which exits 1. quiet keeps the message off stderr
// when --json already carried the count.
type doctorIssuesError struct {
	count int
	quiet bool
}

func (e *doctorIssuesError) Error() string {
	if e.quiet {
		return ""
	}
	return "found " + Pluralize(e.count, "issue", "issues")
}

// ExitCode is the process status main reports for this error.
func (e *doctorIssuesError) ExitCode() int { return 2 }

// doctorState is what one check pass loaded, which is exactly what a --fix
// pass repairs.
type doctorState struct {
	runtime        *config.RuntimeState
	everyProject   []*models.Project
	loadedProjects []*models.Project
	planning       *planningRepair
}

// collectDoctorResults loads the workspace from disk and runs every check
// against it. It is called once for the verdict and, under --fix, again after
// repairing, so the verdict always describes the state the fixes left behind.
func collectDoctorResults() (*doctorResults, *doctorState, error) {
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
		return nil, nil, err
	}

	state := &doctorState{
		runtime:      workspace.Runtime,
		everyProject: workspace.Runtime.AllProjects(),
	}
	results := &doctorResults{}
	checkWorkspaceHealth(workspace, results)

	// Load every retained project, including archived projects, so terminal work
	// and hierarchy integrity remain visible. The repair reader admits the defects
	// --fix repairs, so they are reported by their own checks below rather than
	// as a load failure --fix could never reach.
	for _, project := range state.everyProject {
		if _, loadErr := storage.LoadProjectForRepair(project); loadErr != nil {
			results.loadFailures = append(results.loadFailures, doctorIssue{
				project: project.Name,
				detail:  loadErr.Error(),
			})
			continue
		}
		state.loadedProjects = append(state.loadedProjects, project)
	}
	loadedProjects := state.loadedProjects

	// The four synced-state buckets that need loaded tasks continue the order
	// checkWorkspaceHealth started: planning membership, then attachments, then
	// checklist and comment structure, then the state clocks.
	state.planning = checkPlanningMembership(workspace.Runtime, loadedProjects, results)
	checkOrphanedAttachments(loadedProjects, results)
	checkChecklistAndComments(loadedProjects, results)
	checkStateClocks(loadedProjects, results)

	checkMissingTaskFiles(state.everyProject, results)
	checkEmptyProjects(loadedProjects, results)
	checkDuplicateIDs(loadedProjects, results)
	checkDuplicateShortIDs(loadedProjects, results)
	checkBrokenParentRefs(loadedProjects, results)
	checkCompletionTimestamps(loadedProjects, results)
	checkTaskReferences(state.everyProject, loadedProjects, results)
	checkDependencyCycles(loadedProjects, results)

	return results, state, nil
}

// doctorApplyFixes is the --fix step, a seam so a test can land a concurrent
// write between the check pass and the repairs.
var doctorApplyFixes = applyDoctorFixes

// applyDoctorFixes runs every supported repair and keeps going past a failure,
// so one stale project does not hide what the others fixed.
func applyDoctorFixes(state *doctorState) ([]string, error) {
	fixed, taskErr := fixIssues(state.runtime.DataDir(), state.loadedProjects)
	planningFixed, planningErr := fixPlanningMembership(state.runtime, state.planning)
	if planningFixed != "" {
		fixed = append(fixed, planningFixed)
	}
	return fixed, errors.Join(taskErr, planningErr)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	fix, _ := cmd.Flags().GetBool("fix")
	asJSON, _ := cmd.Flags().GetBool("json")

	if err := requireExistingWorkspace(); err != nil {
		return err
	}
	results, state, err := collectDoctorResults()
	if err != nil {
		return err
	}

	var fixed []string
	var fixErr error
	if fix {
		fixed, fixErr = doctorApplyFixes(state)
		if fixErr != nil {
			fixErr = fmt.Errorf("doctor --fix could not apply every repair: %w", fixErr)
		}
		results, _, err = collectDoctorResults()
		if err != nil {
			return errors.Join(fixErr, fmt.Errorf("re-checking the workspace after --fix: %w", err))
		}
	}

	// verdictGap sets the closing verdict off from whatever printed above it,
	// and is empty when nothing did.
	verdictGap := ""
	if asJSON {
		if err := printJSON(newDoctorReport(results, fixed)); err != nil {
			return err
		}
	} else {
		for _, line := range fixed {
			fmt.Printf("  Fixed: %s\n", line)
		}
		if printResults(results) || len(fixed) > 0 {
			verdictGap = "\n"
		}
	}

	if fixErr != nil {
		return fixErr
	}
	if results.totalIssues() > 0 {
		if !asJSON {
			if !fix && results.fixableIssues() > 0 {
				fmt.Println(verdictGap + "Run 'docket doctor --fix' to automatically fix fixable issues.")
			} else {
				fmt.Println(verdictGap + "Resolve the reported integrity issues, then re-run 'docket doctor'.")
			}
		}
		return &doctorIssuesError{count: results.totalIssues(), quiet: asJSON}
	}

	if !asJSON {
		if results.informationalNotes() > 0 {
			fmt.Println(verdictGap + "All checks passed (with informational notes above).")
		} else {
			fmt.Println(verdictGap + "All checks passed.")
		}
	}
	return nil
}
