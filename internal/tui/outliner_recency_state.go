package tui

import (
	"maps"
	"time"

	"github.com/sglavoie/tt/internal/tui/taskref"
)

// recencyState is Recent mode's frozen ordering, held against the projection it
// was taken in. Two members, one owner, because neither means anything without
// the other: the timestamps are only trustworthy as the answer some particular
// projection gave, and the projection fingerprint is only worth storing because
// timestamps are hanging off it. Writing one without the other is the bug this
// type exists to make unspellable — a stale fingerprint would serve a frozen
// order for a view that no longer exists, and a stale map would reorder the list
// the user is triaging out from under them.
//
// The freeze is what makes Recent usable: a mutation performed from a Recent row
// updates that task's UpdatedAt, and reading it live would move the row the user
// just acted on. It is deliberately narrower than a cache, since it is rebased
// rather than invalidated: a scope, filter, search, sort, or fold change gives a
// new fingerprint and re-reads the world, while an ordinary mutation leaves the
// list in place.
//
// The zero value is thawed, which is what makes the first pass in any projection
// freeze rather than serve nothing.
type recencyState struct {
	keys       map[taskref.TaskIdentity]time.Time
	projection string
}

// rebase opens a fresh freeze whenever the projection on screen is no longer the
// one the keys were taken in, and leaves an existing freeze alone otherwise. A
// nil map counts as a different projection so a thawed state re-freezes without
// the caller having to notice it was thawed.
func (r *recencyState) rebase(projection string) {
	if r == nil {
		return
	}
	if r.projection == projection && r.keys != nil {
		return
	}
	r.keys = make(map[taskref.TaskIdentity]time.Time)
	r.projection = projection
}

// freeze records a task's activity the first time this projection sees it. First
// write wins, which is the whole of the guarantee: a later pass over the same
// projection cannot move a row by re-reading a timestamp the mutation just moved.
func (r *recencyState) freeze(identity taskref.TaskIdentity, at time.Time) {
	if r == nil || r.keys == nil {
		return
	}
	if _, frozen := r.keys[identity]; frozen {
		return
	}
	r.keys[identity] = at
}

// frozenKeys hands the whole freeze to the ordering pass that sorts by it.
func (r *recencyState) frozenKeys() map[taskref.TaskIdentity]time.Time {
	if r == nil {
		return nil
	}
	return r.keys
}

// frozenAt answers for one row, so the chip a render draws agrees with the
// bucket the ordering pass put that row in. A task the freeze does not name is
// the caller's to fall back on.
func (r *recencyState) frozenAt(identity taskref.TaskIdentity) (time.Time, bool) {
	if r == nil {
		return time.Time{}, false
	}
	frozen, ok := r.keys[identity]
	return frozen, ok
}

// thaw drops the freeze and the projection together, so the next pass re-reads
// activity from the world. Entering Recent over an already-open Recent is the
// manual refresh, and this is what makes it one.
func (r *recencyState) thaw() {
	if r == nil {
		return
	}
	r.keys = nil
	r.projection = ""
}

// recencySnapshot is the freeze as a restoration point. It carries both members
// for the same reason the state holds both: a frame that restored the timestamps
// without the fingerprint they were taken under would serve them to the first
// projection that asked, whatever it was.
type recencySnapshot struct {
	keys       map[taskref.TaskIdentity]time.Time
	projection string
}

// capture copies the freeze out, so a retained frame can never alias the map a
// later pass goes on to extend.
func (r *recencyState) capture() recencySnapshot {
	if r == nil {
		return recencySnapshot{}
	}
	return recencySnapshot{keys: maps.Clone(r.keys), projection: r.projection}
}

// restore writes a captured freeze back. A frame taken outside Recent carries no
// keys, and restoring that is a thaw: the view it returns to had none either.
func (r *recencyState) restore(snapshot recencySnapshot) {
	if r == nil {
		return
	}
	r.keys = maps.Clone(snapshot.keys)
	r.projection = snapshot.projection
}
