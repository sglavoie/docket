package app

// CommitBarrierTarget names what one held commit is about to write. TaskID is
// empty for a mutation that names a project and no task — a project comment is
// the live example — because an absent task is the honest answer there and a
// minted identity would make the barrier claim a task nobody resolved.
type CommitBarrierTarget struct {
	// ProjectID owns the document the held commit will rewrite.
	ProjectID string
	// TaskID is the task the mutation resolved, or "" when it resolved none.
	TaskID string
}

// CommitBarrier is the optional coordination seam a cross-machine consistency
// test installs to hold a single-project mutation at the one instant that makes
// a lost update possible: the snapshot is taken, the target and the revision the
// commit will be guarded by are captured, and nothing has been written yet.
// Normal execution leaves it nil, nothing in production ever writes it, and it
// mints no command, flag, or CLI spelling.
//
// It exists because the property these mutations owe a synced workspace — that
// two machines editing one tasks.md at once lose neither edit — is only
// observable while both processes are holding that state at the same time. A
// barrier placed around a call cannot arrange it: every mutation loads its own
// snapshot, so a partner released before this line could read, commit, and exit
// before the other process read the document it is about to rewrite, and the
// pair would be two sequential writes wearing a race's clothes.
//
// One seam covers every single-project mutation rather than one variable per use
// case, which is what keeps a resume note, a task comment, an acknowledgement, a
// fresh task, and a project comment provably racing at the same point rather
// than at five points that only look alike. undoAction is the mutation's own
// ledger action ("Add Comment", "Mark Reviewed", "Add Task", "Set Resume Note",
// "Add Project Comment", "Edit Project Comment", "Remove Project Comment"), so
// an installed barrier holds the one mutation it came for and lets every other
// write through untouched.
var CommitBarrier func(undoAction string, target CommitBarrierTarget, expectedRevision string)

// awaitCommitBarrier is the single call into an installed barrier. Both spellings
// below funnel through it so a project mutation and a task mutation are held at
// the same seam rather than at two that drifted apart.
func awaitCommitBarrier(target CommitBarrierTarget, expectedRevision, undoAction string) {
	if CommitBarrier == nil {
		return
	}
	CommitBarrier(undoAction, target, expectedRevision)
}

// awaitTaskCommitBarrier hands the resolved task and the guarding revision to an
// installed barrier. The target is rebuilt from the mutated ref rather than taken
// from the caller, so a creation — whose task did not exist when the call began —
// names the identity it is about to commit.
func awaitTaskCommitBarrier(ref TaskRef, expectedRevision, undoAction string) {
	target := CommitBarrierTarget{}
	if ref.Project != nil {
		target.ProjectID = ref.Project.ID
	}
	if ref.Task != nil {
		target.TaskID = ref.Task.ID
	}
	awaitCommitBarrier(target, expectedRevision, undoAction)
}

// awaitProjectCommitBarrier is the project-level spelling, for a mutation whose
// whole target is the project document.
func awaitProjectCommitBarrier(projectID, expectedRevision, undoAction string) {
	awaitCommitBarrier(CommitBarrierTarget{ProjectID: projectID}, expectedRevision, undoAction)
}
