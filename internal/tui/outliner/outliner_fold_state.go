package outliner

import (
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// sectionToggleRestoreAnchor identifies a task temporarily hidden by an
// s-driven all-section collapse. The next s-driven expansion restores focus to
// it when the task remains visible in the active view.
type sectionToggleRestoreAnchor struct {
	task       taskref.TaskIdentity
	sectionKey string
}

// foldState is the whole of what the Outliner has folded shut: the task
// subtrees `t` writes, the section and project-group keys `s` and `p` share,
// the checklists their owners open, and the one anchor an s-driven collapse
// leaves behind so the next expansion can put the cursor back.
//
// Three maps rather than one, and that is the point of the type rather than an
// accident it inherited. Their polarities disagree deliberately —
// expandedNodes defaults expanded so an absent key shows a subtree,
// collapsedSections defaults expanded so an absent key shows a section, and
// expandedChecklists defaults collapsed so an absent key hides its items — and
// their key spaces disagree too: two are project-qualified task identities and
// one is a string namespaced per grouping surface. Merging them would need a
// polarity per key. Naming them together is what makes clearAllFolds a single
// act over three axes without pretending they are one map.
//
// The anchor belongs here rather than beside the cursor because it names a
// section key: it exists only for the window in which those keys are collapsed,
// and clearSections below is what makes that literally true.
//
// The zero value is inert. Every map allocates on first write and reads as
// empty until then, which is exactly the all-open state a fresh Outliner opens
// in, so a struct-literal test model needs no constructor.
type foldState struct {
	// expandedNodes is the project-qualified task fold state: true=expanded, and
	// an absent key reads as expanded too, so the empty map is the canonical
	// all-expanded state rather than merely the initial one.
	ExpandedNodes map[taskref.TaskIdentity]bool

	// collapsedSections is the section and project-group fold state. Its string
	// keys are namespaced per grouping surface and are never cleared on a mode
	// change, so a fold written in one mode is still folded on return to it.
	CollapsedSections map[string]bool

	// expandedChecklists is the project-qualified checklist fold state, and it
	// defaults to collapsed: an absent key hides the owner's items, which is the
	// projection that existed before checklist rows.
	ExpandedChecklists map[taskref.TaskIdentity]bool

	// sectionToggleRestore preserves a task focus across one s-driven
	// collapse/expand cycle. It is deliberately session-only.
	SectionToggleRestore *sectionToggleRestoreAnchor

	// groupFocus remembers, per project-group collapse key, the task the fold that
	// closed that group was standing on, so expanding the group again puts the
	// cursor back inside it instead of on the row that replaced it. Its lifetime is
	// exactly the fold's: a key is written when the group closes and dropped when it
	// opens, which is why a stale identity can never outlive the summary that was
	// covering it.
	GroupFocus map[string]taskref.TaskIdentity
}

// setNodes adopts a captured task-fold map. Deep-copying is the caller's,
// because a caller restoring a frame that may be restored again owes the copy
// and one restoring a map it just built does not.
func (s *foldState) setNodes(nodes map[taskref.TaskIdentity]bool) {
	if s == nil {
		return
	}
	s.ExpandedNodes = nodes
}

// resetNodes returns every task subtree to expanded by dropping the map rather
// than writing true over each node: an absent key already reads as expanded, so
// the empty map is that state exactly and carries no stale identities with it.
func (s *foldState) resetNodes() {
	s.setNodes(make(map[taskref.TaskIdentity]bool))
}

// setNode writes one task's fold state, allocating the map on first write so no
// caller has to ask whether it exists yet.
func (s *foldState) setNode(identity taskref.TaskIdentity, expanded bool) {
	if s == nil {
		return
	}
	if s.ExpandedNodes == nil {
		s.ExpandedNodes = make(map[taskref.TaskIdentity]bool)
	}
	s.ExpandedNodes[identity] = expanded
}

// setSections adopts a captured section-fold map on the same terms as setNodes.
func (s *foldState) setSections(sections map[string]bool) {
	if s == nil {
		return
	}
	s.CollapsedSections = sections
}

// setSection writes one section or project-group key.
func (s *foldState) setSection(key string, collapsed bool) {
	if s == nil {
		return
	}
	if s.CollapsedSections == nil {
		s.CollapsedSections = make(map[string]bool)
	}
	s.CollapsedSections[key] = collapsed
}

// clearSection forgets one key entirely rather than writing false over it, so a
// reveal leaves no entry behind for a later namespace collision to read.
func (s *foldState) clearSection(key string) {
	if s == nil {
		return
	}
	delete(s.CollapsedSections, key)
}

// clearSections drops every section and project-group fold, and the two cursor
// memories with them. Both name a key to return the cursor to, so neither can
// outlive the folds that made it necessary.
func (s *foldState) clearSections() {
	if s == nil {
		return
	}
	s.setSections(make(map[string]bool))
	s.SectionToggleRestore = nil
	s.GroupFocus = nil
}

// rememberGroupFocus records the task one project group's fold has just hidden.
// An invalid identity forgets instead of writing: a fold taken from a header row
// stands on no task, and remembering an empty identity would make the next
// expansion fall back to the group's first task twice over.
func (s *foldState) rememberGroupFocus(key string, identity taskref.TaskIdentity) {
	if s == nil || key == "" {
		return
	}
	if !identity.Valid() {
		s.forgetGroupFocus(key)
		return
	}
	if s.GroupFocus == nil {
		s.GroupFocus = make(map[string]taskref.TaskIdentity)
	}
	s.GroupFocus[key] = identity
}

// groupFocus reads the task a folded project group is covering.
func (s *foldState) groupFocus(key string) (taskref.TaskIdentity, bool) {
	if s == nil || key == "" {
		return taskref.TaskIdentity{}, false
	}
	identity, ok := s.GroupFocus[key]
	return identity, ok && identity.Valid()
}

// forgetGroupFocus drops one group's memory. Every write of an expanded fold
// goes through it, so the memory exists exactly while the group is closed.
func (s *foldState) forgetGroupFocus(key string) {
	if s == nil {
		return
	}
	delete(s.GroupFocus, key)
}

// setChecklists adopts a captured checklist-fold map on the same terms as
// setNodes.
func (s *foldState) setChecklists(checklists map[taskref.TaskIdentity]bool) {
	if s == nil {
		return
	}
	s.ExpandedChecklists = checklists
}

// setChecklist opens or closes one owner's checklist. Closing deletes rather
// than writing false, because absent is what collapsed means on this axis.
func (s *foldState) SetChecklist(identity taskref.TaskIdentity, expanded bool) {
	if s == nil {
		return
	}
	if !expanded {
		delete(s.ExpandedChecklists, identity)
		return
	}
	if s.ExpandedChecklists == nil {
		s.ExpandedChecklists = make(map[taskref.TaskIdentity]bool)
	}
	s.ExpandedChecklists[identity] = true
}

// setSectionToggleAnchor records, or drops, the task an s-driven collapse hid.
func (s *foldState) setSectionToggleAnchor(anchor *sectionToggleRestoreAnchor) {
	if s == nil {
		return
	}
	s.SectionToggleRestore = anchor
}

// takeSectionToggleAnchor consumes the anchor. It is one-shot by construction:
// the anchor describes one collapse/expand cycle, and reading it without
// clearing it would let the next expansion move a cursor the user has since
// placed somewhere else.
func (s *foldState) takeSectionToggleAnchor() *sectionToggleRestoreAnchor {
	if s == nil {
		return nil
	}
	anchor := s.SectionToggleRestore
	s.SectionToggleRestore = nil
	return anchor
}

// resetAll opens everything on all three axes and drops the anchor. It is the
// whole-projection reset, so it replaces each map rather than emptying it in
// place: a frame captured before the reset must not alias the state after it.
func (s *foldState) ResetAll() {
	if s == nil {
		return
	}
	s.resetNodes()
	s.clearSections()
	s.setChecklists(make(map[taskref.TaskIdentity]bool))
}
