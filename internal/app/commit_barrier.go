package app

// TaskCommitBarrier is the optional coordination seam a cross-machine
// consistency test installs to hold a single-project task mutation at the one
// instant that makes a lost update possible: the snapshot is taken, the target
// task and the revision the commit will be guarded by are captured, and nothing
// has been written yet. Normal execution leaves it nil, nothing in production
// ever writes it, and it mints no command, flag, or CLI spelling.
//
// It exists because the property these mutations owe a synced workspace — that
// two machines editing one tasks.md at once lose neither edit — is only
// observable while both processes are holding that state at the same time. A
// barrier placed around a call cannot arrange it: every mutation loads its own
// snapshot, so a partner released before this line could read, commit, and exit
// before the other process read the document it is about to rewrite, and the
// pair would be two sequential writes wearing a race's clothes.
//
// One seam covers every single-project task mutation rather than one variable
// per use case, which is what keeps a resume note, a comment, an
// acknowledgement, and a fresh task provably racing at the same point rather
// than at four points that only look alike. undoAction is the mutation's own
// ledger action ("Add Comment", "Mark Reviewed", "Add Task", "Set Resume
// Note"), so an installed barrier holds the one mutation it came for and lets
// every other write through untouched.
var TaskCommitBarrier func(undoAction string, locator TaskLocator, expectedRevision string)

// awaitCommitBarrier hands the resolved target and the guarding revision to an
// installed barrier. The locator is rebuilt from the mutated ref rather than
// taken from the caller, so a creation — whose task did not exist when the call
// began — names the identity it is about to commit.
func awaitCommitBarrier(ref TaskRef, expectedRevision, undoAction string) {
	if TaskCommitBarrier == nil {
		return
	}
	locator := TaskLocator{}
	if ref.Project != nil {
		locator.ProjectID = ref.Project.ID
	}
	if ref.Task != nil {
		locator.TaskID = ref.Task.ID
	}
	TaskCommitBarrier(undoAction, locator, expectedRevision)
}
