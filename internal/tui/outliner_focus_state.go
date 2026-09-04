package tui

// FocusEntry is the retained internal focus-frame representation. It records
// the complete viewport state needed to restore the parent view exactly.
type FocusEntry struct {
	TaskID       string // UUID of the task that was focused
	CursorIndex  int    // Cursor position at the moment of focus
	ScrollOffset int    // Persistent viewport offset at the moment of focus
}

// CrossProjectFocusEntry records the state at a single focus level for the
// cross-project outliner. It extends FocusEntry with a ProjectID field to
// scope focus resolution to the correct project, preventing UUID collisions
// across projects from causing incorrect focus resolution.
type CrossProjectFocusEntry struct {
	TaskID       string // UUID of the task that was focused
	ProjectID    string // ID of the project owning the task
	CursorIndex  int    // Cursor position at the moment of focus
	ScrollOffset int    // Persistent viewport offset at the moment of focus
}

// sectionZoom narrows the Outliner to one section of the active mode, plus the
// viewport Esc restores. A zero value means no zoom.
type sectionZoom struct {
	key    string // Collapse key of the zoomed section
	name   string // Display name, for chrome only
	cursor int    // Cursor row the zoom was entered from
	scroll int    // Persistent viewport offset the zoom was entered from
}

func (z sectionZoom) active() bool { return z.key != "" }

// focusState is everything the Outliner has narrowed to and can hand back: the
// two focus stacks, the section zoom, and the one-shot notice a frame that
// stopped resolving leaves behind.
//
// Esc is the argument for one owner. The unwind ladder walks task focus and
// then the zoom before it reaches project scope, and every wholesale reset
// takes all of them down together, so a caller that put one of them away and
// forgot another would leave the view narrowed by something no Esc can name.
// The notice belongs here for the same reason from the other end: only
// validateFocusStack and validateCrossFocusStack write it, and they write it
// exactly when they shorten a stack.
//
// Two stacks rather than one, because a cross-project frame carries the
// owning project and a single-project frame cannot: which stack is live is
// m.project's answer, never a field here. Whether a cross-stack write also owes
// m.population.invalidate() stays the call site's, not this type's — the
// memoization guard is the population's, this state deliberately cannot see it,
// and validateCrossFocusStack truncates mid-rebuild without claiming one.
//
// The zero value is inert: no frames, no zoom, no notice.
type focusState struct {
	// stack is the single-project focus stack and crossStack the cross-project
	// one. Each frame retains the cursor and scroll offset Esc restores, so
	// nested focus levels unwind without moving the viewport.
	stack      []FocusEntry
	crossStack []CrossProjectFocusEntry

	// notice is the transient message shown when a focus target left its branch.
	// ViewStatusStrip consumes it, so it survives exactly one frame.
	notice string

	// zoom is the section the projection is narrowed to; the zero value is none.
	zoom sectionZoom
}

// active reports whether either stack holds a frame. It is the one definition
// of "the view is focused", shared by the empty-state hint, the launch reset's
// axis inventory, and the search projection's own focus question.
func (s *focusState) active() bool {
	if s == nil {
		return false
	}
	return len(s.stack) > 0 || len(s.crossStack) > 0
}

// push records one single-project focus frame.
func (s *focusState) push(entry FocusEntry) {
	if s == nil {
		return
	}
	s.stack = append(s.stack, entry)
}

// pushCross records one cross-project focus frame.
func (s *focusState) pushCross(entry CrossProjectFocusEntry) {
	if s == nil {
		return
	}
	s.crossStack = append(s.crossStack, entry)
}

// pop removes the top single-project frame and returns it.
func (s *focusState) pop() (FocusEntry, bool) {
	if s == nil || len(s.stack) == 0 {
		return FocusEntry{}, false
	}
	top := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return top, true
}

// popCross removes the top cross-project frame and returns it.
func (s *focusState) popCross() (CrossProjectFocusEntry, bool) {
	if s == nil || len(s.crossStack) == 0 {
		return CrossProjectFocusEntry{}, false
	}
	top := s.crossStack[len(s.crossStack)-1]
	s.crossStack = s.crossStack[:len(s.crossStack)-1]
	return top, true
}

// truncate keeps the first depth single-project frames, dropping the rest.
func (s *focusState) truncate(depth int) {
	if s == nil || depth < 0 || depth > len(s.stack) {
		return
	}
	s.stack = s.stack[:depth]
}

// truncateCross keeps the first depth cross-project frames, dropping the rest.
func (s *focusState) truncateCross(depth int) {
	if s == nil || depth < 0 || depth > len(s.crossStack) {
		return
	}
	s.crossStack = s.crossStack[:depth]
}

// clear drops the single-project stack.
func (s *focusState) clear() {
	if s == nil {
		return
	}
	s.stack = nil
}

// clearCross drops the cross-project stack.
func (s *focusState) clearCross() {
	if s == nil {
		return
	}
	s.crossStack = nil
}

// setStack adopts a captured single-project stack. Deep-copying is the
// caller's, on the same terms as foldState's adopters.
func (s *focusState) setStack(stack []FocusEntry) {
	if s == nil {
		return
	}
	s.stack = stack
}

// restore writes a captured frame's three narrowings back at once. The scope
// frame and the presentation snapshot both capture the trio together, because
// unwinding one of them without the others would leave the view narrowed by a
// remnant of the frame it thought it had left.
func (s *focusState) restore(stack []FocusEntry, crossStack []CrossProjectFocusEntry, zoom sectionZoom) {
	if s == nil {
		return
	}
	s.stack = stack
	s.crossStack = crossStack
	s.zoom = zoom
}

// enterZoom narrows to one section, retaining the viewport Esc restores.
func (s *focusState) enterZoom(key, name string, cursor, scroll int) {
	if s == nil {
		return
	}
	s.zoom = sectionZoom{key: key, name: name, cursor: cursor, scroll: scroll}
}

// exitZoom widens back and returns the viewport the zoom was entered from.
func (s *focusState) exitZoom() (cursor, scroll int) {
	if s == nil {
		return 0, 0
	}
	cursor, scroll = s.zoom.cursor, s.zoom.scroll
	s.zoom = sectionZoom{}
	return cursor, scroll
}

// clearZoom drops the zoom and the viewport it retained, for a transition that
// reshapes the projection on its own terms.
func (s *focusState) clearZoom() {
	if s == nil {
		return
	}
	s.zoom = sectionZoom{}
}

// setNotice records the one-shot message a shortened stack owes the user.
func (s *focusState) setNotice(notice string) {
	if s == nil {
		return
	}
	s.notice = notice
}

// clearNotice consumes it. The strip is rendered once per frame and reserves
// its own height from the same body, so consuming here is what keeps the
// notice from outliving the frame that reported it.
func (s *focusState) clearNotice() {
	if s == nil {
		return
	}
	s.notice = ""
}
