package tui

import "github.com/sglavoie/tt/internal/tui/notes"

// noteIndexController is the notes cluster's shell state: the workspace-wide
// note snapshot the Outliner's search tier ranks, the generation that discards
// a superseded read, and the owner-catalog fingerprint that decides whether an
// availability change owes a reload at all.
//
// It is shell-owned because NotesAtLocations reads disk and the tier is scored
// on every keystroke: the read happens once per invalidation, asynchronously,
// and the Outliner only ever reads the retained result through an injected
// accessor.
//
// It owns the retained answer and the generation discipline, not the read and
// not the write. The disk read is a plain tea.Cmd built beside it in
// note_search_index.go; the notes surface handlers and the note commands stay
// AppModel methods, because the application service every note mutation commits
// through lives there. Nothing here touches disk, so the boundary can add no
// note write of its own — and because the owner catalog it fingerprints is the
// same NoteOwnerCatalog the Notes manager uses, the index stays
// location-qualified and archived projects stay out of it.
type noteIndexController struct {
	// rows is the retained snapshot. A failed refresh leaves it live, which
	// degrades the tier to a stale answer rather than an empty one.
	rows []notes.LocatedNoteRow

	// generation is the read the shell is still waiting on. A result carrying
	// any other generation is a slow read that has already been superseded.
	generation int

	// owners is the fingerprint of the owner catalog the current read was
	// started against.
	owners string
}

// beginLoad claims the next generation for a fresh read and records the owner
// catalog it was started against, superseding whatever read is still in flight.
func (c *noteIndexController) beginLoad(fingerprint string) int {
	c.owners = fingerprint
	c.generation++
	return c.generation
}

// ownersMoved reports whether the owner catalog changed since the last read was
// started. A cross-project load lands after every committed task mutation, so
// this is what keeps a full notes read off an ordinary keystroke.
func (c *noteIndexController) ownersMoved(fingerprint string) bool {
	return fingerprint != c.owners
}

// awaits reports whether a completed read is the one still being waited on.
func (c *noteIndexController) awaits(generation int) bool {
	return generation == c.generation
}

// adopt publishes an accepted read.
func (c *noteIndexController) adopt(rows []notes.LocatedNoteRow) {
	c.rows = rows
}
