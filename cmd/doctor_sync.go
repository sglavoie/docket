package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

// The checks in this file audit the state a synced workspace can arrive in that
// a single-machine workspace never produces: lens definitions written by another
// machine, transactions another machine left behind, conflict bundles waiting to
// be resolved, lock files that traveled through the sync client, planning
// membership recorded against a task state a second machine has since changed,
// and attachment bytes that arrived without their metadata or the reverse.
//
// None of them can be reproduced by driving one app: they need two machines and
// a file-copying third party. They are read-only except planning membership,
// which is the one skew a scan can repair without guessing, because the task
// state it disagrees with is the authority.

// checkLensDefinitions audits the portable lens home and the machine-local
// pointer into it.
//
// A dangling active-lens pointer is a note rather than an issue: failing closed
// to no lens, silently, is the contracted behavior, so a machine whose lens was
// deleted elsewhere is working exactly as designed. A lens whose scoped project
// is unknown to the catalog is an issue, because the lens is refused whole every
// time it is applied and nothing in the product will say why.
func checkLensDefinitions(runtime *config.RuntimeState, schema int, results *doctorResults) {
	dataDir := runtime.DataDir()
	path := config.LensDefinitionsPath(dataDir)
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		if schema >= config.V16ConfigSchemaVersion {
			results.lensIssues = append(results.lensIssues, doctorIssue{
				project: "Lenses",
				detail:  fmt.Sprintf("portable lens definitions are missing: %s", path),
			})
		}
	case err != nil:
		results.lensIssues = append(results.lensIssues, doctorIssue{
			project: "Lenses",
			detail:  fmt.Sprintf("cannot inspect portable lens definitions %s: %v", path, err),
		})
	case !info.Mode().IsRegular():
		results.lensIssues = append(results.lensIssues, doctorIssue{
			project: "Lenses",
			detail:  fmt.Sprintf("portable lens definitions are not a regular file: %s", path),
		})
	}

	machine := runtime.Machine()
	if machine == nil {
		results.lensIssues = append(results.lensIssues, doctorIssue{
			project: "Lenses",
			detail:  "machine-local state is unavailable, so no lens can be resolved",
		})
		return
	}
	lenses := machine.NamedLenses
	if err := lenses.Validate(); err != nil {
		results.lensIssues = append(results.lensIssues, doctorIssue{
			project: "Lenses",
			detail:  err.Error(),
		})
	}

	byID := make(map[string]bool, len(lenses))
	for _, lens := range lenses {
		byID[lens.ID] = true
	}
	if machine.ActiveLensID != "" && !byID[machine.ActiveLensID] {
		results.lensNotes = append(results.lensNotes, doctorIssue{
			project: "Lenses",
			detail:  fmt.Sprintf("active lens %q no longer exists; the Outliner opens with no lens", machine.ActiveLensID),
		})
	}

	catalogProjects := make(map[string]*models.Project)
	for _, project := range runtime.AllProjects() {
		if project != nil {
			catalogProjects[project.ID] = project
		}
	}
	for _, lens := range lenses {
		if lens.Scope == "" {
			continue
		}
		project := catalogProjects[lens.Scope]
		switch {
		case project == nil:
			results.lensIssues = append(results.lensIssues, doctorIssue{
				project: "Lenses",
				detail:  fmt.Sprintf("lens %q is scoped to unknown project %q and is refused whenever it is applied", lens.Name, lens.Scope),
			})
		case project.Archived:
			results.lensNotes = append(results.lensNotes, doctorIssue{
				project: "Lenses",
				detail:  fmt.Sprintf("lens %q is scoped to archived project %q and is refused until it is unarchived", lens.Name, project.Name),
			})
		case projectHasParkedTag(project):
			results.lensNotes = append(results.lensNotes, doctorIssue{
				project: "Lenses",
				detail:  fmt.Sprintf("lens %q is scoped to parked project %q and is refused until it is unparked", lens.Name, project.Name),
			})
		}
	}

	results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
		project: "Lenses",
		detail:  fmt.Sprintf("%s defined", Pluralize(len(lenses), "named lens", "named lenses")),
	})
}

func projectHasParkedTag(project *models.Project) bool {
	for _, tag := range project.Tags {
		if tag == models.ParkedProjectTag {
			return true
		}
	}
	return false
}

// checkTransactionJournal describes the transactions directory without touching
// it, and names the manual recovery step when something is quarantined.
//
// Quarantine blocks every workspace mutation and there is no command that clears
// it, so the only useful thing a diagnostic can do is say exactly where the
// manifests are and what moving them aside means. A transaction another machine
// still appears to be driving is a note, not an issue: it is the ordinary state
// of a synced workspace whose other machine is mid-commit.
func checkTransactionJournal(runtime *config.RuntimeState, results *doctorResults) {
	dataDir := runtime.DataDir()
	dir := config.TransactionsDir(dataDir)
	inventory, err := transaction.Inspect(dir, []string{dataDir, config.LocalDataRoot()}, "")
	if err != nil {
		results.transactionIssues = append(results.transactionIssues, doctorIssue{
			project: "Transactions",
			detail:  err.Error(),
		})
		return
	}

	var live, foreign int
	for _, pending := range inventory.Pending {
		switch {
		case pending.LoadError != "":
			results.transactionIssues = append(results.transactionIssues, doctorIssue{
				project: "Transactions",
				detail:  fmt.Sprintf("unreadable manifest %s at %s: %s", pending.ID, pending.Path, pending.LoadError),
			})
		case pending.ForeignLive():
			foreign++
			results.transactionNotes = append(results.transactionNotes, doctorIssue{
				project: "Transactions",
				detail: fmt.Sprintf(
					"transaction %s is %s on machine %s as of %s; this machine leaves it alone",
					pending.ID, pending.State, pending.Machine, pending.Heartbeat,
				),
			})
		case pending.Live() && pending.EscapesRoots:
			results.transactionIssues = append(results.transactionIssues, doctorIssue{
				project: "Transactions",
				detail: fmt.Sprintf(
					"transaction %s (prepared %s) names paths outside this machine's workspace roots; the workspace was prepared at a different absolute path",
					pending.ID, pending.CreatedAt,
				),
			})
		case pending.Live():
			live++
		}
	}
	if live > 0 {
		results.transactionNotes = append(results.transactionNotes, doctorIssue{
			project: "Transactions",
			detail:  fmt.Sprintf("%s pending for this machine and recoverable on next launch", Pluralize(live, "transaction", "transactions")),
		})
	}

	if !inventory.Blocked() {
		return
	}
	for _, quarantined := range inventory.Quarantined {
		detail := fmt.Sprintf("quarantined transaction %s at %s", quarantined.ID, quarantined.Path)
		if quarantined.State != "" {
			detail += fmt.Sprintf(" in state %s", quarantined.State)
		}
		if quarantined.Machine != "" {
			detail += fmt.Sprintf(" from machine %s", quarantined.Machine)
		}
		if quarantined.LoadError != "" {
			detail += fmt.Sprintf(" (unreadable: %s)", quarantined.LoadError)
		}
		results.transactionIssues = append(results.transactionIssues, doctorIssue{
			project: "Transactions",
			detail:  detail,
		})
	}
	results.transactionIssues = append(results.transactionIssues, doctorIssue{
		project: "Transactions",
		detail:  quarantineRecoveryStep(inventory.QuarantineDir()),
	})
}

// quarantineRecoveryStep is the one recovery instruction for a quarantine. It is
// a single string so the blocked-startup path and the doctor bucket say the same
// thing.
func quarantineRecoveryStep(quarantineDir string) string {
	return fmt.Sprintf(
		"recovery step: every workspace mutation is blocked while %s holds manifests. "+
			"Inspect them, restore any file they name from .tt/backups if its contents are wrong, "+
			"then move the directory aside (for example to %s.bak) to unblock mutations.",
		quarantineDir, quarantineDir,
	)
}

// checkConflictBundles counts the conflict bundles waiting to be resolved and
// reports unreadable ones.
//
// An outstanding bundle is a note: it is the expected outcome of two machines
// editing one project, the status strip already leads with the count, and
// resolving stays in the CLI. Only a bundle that cannot be read is an issue,
// because then the count is a lie.
func checkConflictBundles(runtime *config.RuntimeState, results *doctorResults) {
	store := storage.NewConflictStore(config.ConflictsDir(runtime.DataDir()))
	records, err := store.List()
	if err != nil {
		results.conflictIssues = append(results.conflictIssues, doctorIssue{
			project: "Conflicts",
			detail:  err.Error(),
		})
		return
	}
	for _, record := range records {
		if _, loadErr := store.Load(record.ID); loadErr != nil {
			results.conflictIssues = append(results.conflictIssues, doctorIssue{
				project: "Conflicts",
				detail:  fmt.Sprintf("conflict bundle %s cannot be read: %v", record.ID, loadErr),
			})
		}
	}
	if len(records) == 0 {
		return
	}
	results.conflictNotes = append(results.conflictNotes, doctorIssue{
		project: "Conflicts",
		detail: fmt.Sprintf(
			"%s outstanding in %s; resolve with 'tt conflict'",
			Pluralize(len(records), "conflict bundle", "conflict bundles"),
			store.Dir(),
		),
	})
}

// staleLockAge is how long a lock file must have gone untouched before it is
// worth naming. Advisory locks are held for the duration of a single write, so
// anything older than this was left by a process that is gone or by a sync client
// copying the file in.
const staleLockAge = time.Hour

// checkLockFiles reports the advisory lock files present in the workspace.
//
// A lock file existing is not a defect: flock-based locking never removes the
// file it locks, so every project that has ever been written owns one forever.
// What is worth naming is a lock file a sync client has copied between machines,
// because the second machine then holds a file whose modification time belongs to
// the first. These are informational: the lock is advisory and unlocked, so it
// wedges nothing, and reporting it as an issue would make every ordinary
// workspace exit non-zero. Only a lock path that is not a regular file is an
// issue, since that one really can refuse a write.
func checkLockFiles(runtime *config.RuntimeState, projects []*models.Project, results *doctorResults) {
	paths := []string{filepath.Join(config.TransactionsDir(runtime.DataDir()), storage.WorkspaceCommitLockName)}
	for _, project := range projects {
		if project != nil && project.Path != "" {
			paths = append(paths, filepath.Join(project.Path, storage.LockFileName))
		}
	}
	sort.Strings(paths)

	var stale int
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case os.IsNotExist(err):
			continue
		case err != nil:
			results.lockIssues = append(results.lockIssues, doctorIssue{
				project: "Locks",
				detail:  fmt.Sprintf("cannot inspect lock file %s: %v", path, err),
			})
			continue
		case !info.Mode().IsRegular():
			results.lockIssues = append(results.lockIssues, doctorIssue{
				project: "Locks",
				detail:  fmt.Sprintf("lock path is not a regular file and will refuse writes: %s", path),
			})
			continue
		}
		if time.Since(info.ModTime()) > staleLockAge {
			stale++
		}
	}
	if stale > 0 {
		results.lockNotes = append(results.lockNotes, doctorIssue{
			project: "Locks",
			detail: fmt.Sprintf(
				"%s untouched for over %s; advisory locks are never removed, so this is expected and wedges nothing",
				Pluralize(stale, "lock file", "lock files"),
				staleLockAge,
			),
		})
	}
}

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
func fixPlanningMembership(runtime *config.RuntimeState, repair *planningRepair) {
	if repair == nil || !repair.changed || repair.repaired == nil {
		return
	}
	before := runtime.Planning()
	removed := 0
	if before != nil {
		removed = len(before.Today) + len(before.Next) - len(repair.repaired.Today) - len(repair.repaired.Next)
	}
	if err := runtime.CommitPlanning(repair.repaired); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to repair planning membership: %v\n", err)
		return
	}
	fmt.Printf("  Fixed: pruned %s from Today and Next\n", Pluralize(removed, "planning entry", "planning entries"))
}

// checkOrphanedAttachments compares each project's attachment metadata against
// the bytes under its .attachments tree.
//
// Metadata naming a file that is not there is an issue: the attachment is dead
// to every reader. Bytes no metadata names are informational, because they may be
// the only surviving copy of an attachment whose tasks.md lost the entry, and
// deleting them is never a repair a scan should choose.
func checkOrphanedAttachments(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		if project == nil {
			continue
		}
		findings, err := storage.InspectProjectAttachments(project)
		if err != nil {
			results.attachmentIssues = append(results.attachmentIssues, doctorIssue{
				project: project.Name,
				detail:  err.Error(),
			})
			continue
		}
		for _, finding := range findings {
			issue := doctorIssue{project: project.Name, detail: finding.Detail}
			if finding.Orphaned {
				results.attachmentNotes = append(results.attachmentNotes, issue)
			} else {
				results.attachmentIssues = append(results.attachmentIssues, issue)
			}
		}
	}
}

// checkChecklistAndComments audits the two repeated task structures a Markdown
// round trip can damage: checklist items and comments.
//
// Both are stored as lists under a task heading, so a conflicted copy merged by
// hand, or a sync client stitching two versions of tasks.md together, can leave a
// blank item, a duplicated item, or a comment with no timestamp. None of these is
// repaired: the text is user content, and deciding which of two near-identical
// items to drop is a judgement a scan does not have.
func checkChecklistAndComments(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			seenItems := make(map[string]bool, len(task.Checklist))
			for index, item := range task.Checklist {
				text := strings.TrimSpace(item.Text)
				switch {
				case text == "":
					results.structureIssues = append(results.structureIssues, doctorIssue{
						project: project.Name,
						detail:  fmt.Sprintf("task #%d %q has an empty checklist item at position %d", task.ShortID, task.Title, index+1),
					})
				case seenItems[text]:
					results.structureIssues = append(results.structureIssues, doctorIssue{
						project: project.Name,
						detail:  fmt.Sprintf("task #%d %q repeats checklist item %q", task.ShortID, task.Title, text),
					})
				}
				seenItems[text] = true
			}
			for index, comment := range task.Comments {
				switch {
				case comment == nil:
					results.structureIssues = append(results.structureIssues, doctorIssue{
						project: project.Name,
						detail:  fmt.Sprintf("task #%d %q holds an absent comment at position %d", task.ShortID, task.Title, index+1),
					})
				case strings.TrimSpace(comment.Text) == "":
					results.structureIssues = append(results.structureIssues, doctorIssue{
						project: project.Name,
						detail:  fmt.Sprintf("task #%d %q has an empty comment at position %d", task.ShortID, task.Title, index+1),
					})
				case comment.CreatedAt.IsZero():
					results.structureIssues = append(results.structureIssues, doctorIssue{
						project: project.Name,
						detail:  fmt.Sprintf("task #%d %q has a comment with no timestamp at position %d", task.ShortID, task.Title, index+1),
					})
				}
			}
		}
	}
}

// clockSkewTolerance is how far a stored timestamp may sit in the future before
// it is worth naming. Two synced machines never agree exactly, so a small lead is
// ordinary; a large one means one machine's clock is wrong, which is what makes
// state ages and stale-lane membership read wrong on that machine.
const clockSkewTolerance = 24 * time.Hour

// checkStateClocks audits the state clock against the touch time.
//
// StateChangedAt answers "in this state since" and UpdatedAt answers "last
// touched", so a task whose state clock is newer than its touch time has been
// written by something that moved one and not the other. A missing clock is an
// issue rather than a thing to backfill: the storage layer rejects a task that
// reaches it without one, so a task holding a zero clock arrived by some other
// route. A timestamp far in the future is a clock-skew note, since the wrong
// clock is the other machine's, not this workspace's data.
func checkStateClocks(projects []*models.Project, results *doctorResults) {
	now := time.Now()
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			switch {
			case task.StateChangedAt.IsZero():
				results.stateClockIssues = append(results.stateClockIssues, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("task #%d %q has no state clock", task.ShortID, task.Title),
				})
			case task.UpdatedAt.IsZero():
				results.stateClockIssues = append(results.stateClockIssues, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("task #%d %q has no touch time", task.ShortID, task.Title),
				})
			case task.StateChangedAt.After(task.UpdatedAt):
				results.stateClockIssues = append(results.stateClockIssues, doctorIssue{
					project: project.Name,
					detail: fmt.Sprintf(
						"task #%d %q changed state at %s but was last touched at %s",
						task.ShortID, task.Title,
						task.StateChangedAt.Format(time.RFC3339),
						task.UpdatedAt.Format(time.RFC3339),
					),
				})
			}
			if task.StateChangedAt.Sub(now) > clockSkewTolerance || task.UpdatedAt.Sub(now) > clockSkewTolerance {
				results.clockSkewNotes = append(results.clockSkewNotes, doctorIssue{
					project: project.Name,
					detail: fmt.Sprintf(
						"task #%d %q carries timestamps in the future; another machine's clock is ahead",
						task.ShortID, task.Title,
					),
				})
			}
		}
	}
}
