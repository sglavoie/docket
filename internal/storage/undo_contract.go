// Package storage persists project task data and undo snapshots.
//
// Undo/redo contract:
//
// The project undo model is intentionally linear, matching Vim's common
// u / ctrl+r workflow rather than Vim's full branching undo tree. Every
// user-visible task/project mutation that participates in undo records a
// pre-mutation snapshot on the undo stack. Undo consumes the newest undo
// entry, stores the current file as a redo snapshot, and restores the consumed
// snapshot. Redo performs the same operation in the opposite direction.
//
// Any new mutation after an undo clears the redo stack before the new undo
// entry is persisted. Undo entries with TaskIDs restore only those explicitly
// changed tasks when possible; nil TaskIDs mean full-file restore.
//
// Grouped transactions extend the same linear model across projects: all
// entries in the group share one group id and project set, a group undo/redo
// restores every touched project as one logical operation, and divergence or
// missing snapshots abort the group instead of partially restoring it. A new
// mutation after a grouped undo clears redo for the affected project history.
package storage
