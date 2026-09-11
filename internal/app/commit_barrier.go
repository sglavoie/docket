package app

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
// write through untouched. projectID owns the document that commit will rewrite;
// taskID is the task the mutation resolved, or "" for a mutation that names a
// project and no task — a project comment is the live example — because an
// absent task is the honest answer there and a minted identity would make the
// barrier claim a task nobody resolved.
//
// The target is two parameters rather than a struct on purpose. A struct here
// would declare fields that only an installed barrier — which lives in a test —
// can ever read, and `scripts/dead_code.sh` rejects a production declaration
// whose only consumer is a `_test.go` file. Parameters carry the same two values
// without minting one.
var CommitBarrier func(undoAction, projectID, taskID, expectedRevision string)

// awaitCommitBarrier is the single call into an installed barrier. Both spellings
// below funnel through it so a project mutation and a task mutation are held at
// the same seam rather than at two that drifted apart.
func awaitCommitBarrier(projectID, taskID, expectedRevision, undoAction string) {
	if CommitBarrier == nil {
		return
	}
	CommitBarrier(undoAction, projectID, taskID, expectedRevision)
}

// awaitTaskCommitBarrier hands the resolved task and the guarding revision to an
// installed barrier. The target is rebuilt from the mutated ref rather than taken
// from the caller, so a creation — whose task did not exist when the call began —
// names the identity it is about to commit.
func awaitTaskCommitBarrier(ref TaskRef, expectedRevision, undoAction string) {
	var projectID, taskID string
	if ref.Project != nil {
		projectID = ref.Project.ID
	}
	if ref.Task != nil {
		taskID = ref.Task.ID
	}
	awaitCommitBarrier(projectID, taskID, expectedRevision, undoAction)
}

// awaitProjectCommitBarrier is the project-level spelling, for a mutation whose
// whole target is the project document and which therefore names no task.
func awaitProjectCommitBarrier(projectID, expectedRevision, undoAction string) {
	awaitCommitBarrier(projectID, "", expectedRevision, undoAction)
}

// awaitBulkCommitBarrier is the multi-target spelling, for a mutation that
// resolved a whole selection against one snapshot and is about to commit every
// project it touched as one transaction. It names each resolved target in turn,
// guarded by that target's own project revision, because an installed barrier
// filters on project and task: a multi-project request must be able to say which
// document it is holding, and a single seam naming only the first one would let
// the second project's guard slip past unobserved. The nil check is kept here so
// ordinary execution walks no selection at all.
func awaitBulkCommitBarrier(refs []TaskRef, snap Snapshot, undoAction string) {
	if CommitBarrier == nil || snap == nil {
		return
	}
	for _, ref := range refs {
		if ref.Project == nil {
			continue
		}
		awaitTaskCommitBarrier(ref, snap.Revision(ref.Project.ID), undoAction)
	}
}
