package outliner

import (
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// inlinePeekOwnerKind names what kind of thing a remembered offset belongs to.
// Peek draws two documents — a task's and a project header's — and each kind's
// own identity is what tells one document of that kind from another. The kind
// travels beside the identity so the two populations cannot collide: a task
// owner and a project owner are never the same owner however their identity
// fields happen to compare.
type inlinePeekOwnerKind uint8

const (
	inlinePeekOwnerNone inlinePeekOwnerKind = iota
	inlinePeekOwnerTask
	inlinePeekOwnerProject
	inlinePeekOwnerChecklist
)

// inlinePeekOwner is the typed identity half of the key, and the extension point
// a further document kind takes: it adds its kind and its own identity field
// here, teaches peekCursorDocument to build its document, and every reset rule
// below keeps working with nothing else changed.
type inlinePeekOwner struct {
	kind inlinePeekOwnerKind
	task taskref.TaskIdentity
	// project is the catalog identity rather than the row's collapse key, which
	// is what makes the several headers one project can hold at once — All emits
	// a group per state section — one owner: a page turn under one of them is the
	// same page under the others, because they are the same project's detail.
	project string
	// Checklist items have no durable ID. Index and text prevent an unrelated
	// item at a reused position from inheriting this item's reading position.
	checklistIndex int
	checklistText  string
}

func taskPeekOwner(identity taskref.TaskIdentity) inlinePeekOwner {
	return inlinePeekOwner{kind: inlinePeekOwnerTask, task: identity}
}

// projectPeekOwner names the project a header row stands for. A header with no
// identity resolves no project and therefore draws no document, so it takes the
// unwritten owner rather than an owner every other anonymous header would agree
// with.
func projectPeekOwner(projectID string) inlinePeekOwner {
	if projectID == "" {
		return inlinePeekOwner{}
	}
	return inlinePeekOwner{kind: inlinePeekOwnerProject, project: projectID}
}

// sameAs answers whether two owners are the same document's. An owner naming no
// kind matches nothing, itself included: that is the unwritten slot, and a
// terminal that has not been sized yet would otherwise hand it a key of zeroes
// to agree with.
func (owner inlinePeekOwner) sameAs(other inlinePeekOwner) bool {
	if owner.kind == inlinePeekOwnerNone || other.kind == inlinePeekOwnerNone {
		return false
	}
	return owner.kind == other.kind && owner.task == other.task && owner.project == other.project &&
		owner.checklistIndex == other.checklistIndex && owner.checklistText == other.checklistText
}

// rowSupportsPeek names the rows that carry a detail document. Every reader of a
// Peek — the render, the measurement, the offset key, and the four scrolling
// keys — asks here, so none of them can admit a row another one refuses.
func rowSupportsPeek(row OutlinerRow) bool {
	return row.IsTaskRow() || row.IsProjectHeader || checklistPeekItemValid(row)
}

// inlinePeekOffsetKey is what a remembered offset is remembered for. Peek's
// scrolling is session-only. Matching content and geometry reuse a row offset;
// changed content or geometry resolves the saved reading anchor instead.
type inlinePeekOffsetKey struct {
	owner       inlinePeekOwner
	fingerprint string
	query       string
	width       int
	height      int
}

// sameAs answers whether a remembered offset still belongs to what is on screen.
// It names each axis rather than comparing the two structs with `==`, because a
// field nobody spells here is a field the reset does not consult: the
// declaration audit reports an unread key field, so forgetting to extend this
// comparison is a red gate rather than an offset that silently outlives its
// document.
func (key inlinePeekOffsetKey) sameAs(other inlinePeekOffsetKey) bool {
	return key.owner.sameAs(other.owner) &&
		key.fingerprint == other.fingerprint &&
		key.query == other.query &&
		key.width == other.width &&
		key.height == other.height
}

// peekOffsetState is the one offset Peek remembers, counted in content rows from
// the top of the body. There is exactly one slot because Peek shows exactly one
// row's detail; every other owner reads as a key mismatch and therefore as the
// top of its own document.
type peekOffsetState struct {
	key    inlinePeekOffsetKey
	offset int
	anchor peekReadingAnchor
}

// inlinePeekDocumentForRow builds the complete detail document for one row.
// Every reader — the renderer, the viewport measurement, and the two paging
// keys — comes through here, so none of them can be looking at a different
// document than the others. The row's kind picks the builder, and the project
// header takes the branch first: it is not a task row, and a task row is never a
// header.
func (m *OutlinerModel) inlinePeekDocumentForRow(
	row OutlinerRow,
	indent string,
	rowWidth int,
	geometry peekDockGeometry,
) inlinePeekDocument {
	if m == nil {
		return inlinePeekDocument{}
	}
	if row.IsChecklistItem {
		return inlinePeekChecklistDocumentFor(row, indent, rowWidth, geometry)
	}
	if row.IsProjectHeader {
		// The row's own project identity, not the view's: a cross-project group
		// header names a project the scoped model may not be, and resolving through
		// the scope would draw the wrong project's comments under it.
		return inlinePeekProjectDocumentFor(inlinePeekProjectOptions{
			Project: m.projectByGroupID(row.ProjectID),
			Indent:  indent,
			Width:   rowWidth,
			Window:  geometry,
		})
	}
	if !row.IsTaskRow() {
		return inlinePeekDocument{}
	}
	index := m.dependencyIndex()
	excerpt := m.peekSearchExcerpt(row)
	projectSlug, _ := m.projectSlugForRow(row)
	return inlinePeekDocumentFor(inlinePeekOptions{
		Task:             row.Task,
		Indent:           indent,
		Width:            rowWidth,
		Window:           geometry,
		Compact:          !m.PeekExpanded,
		Breadcrumb:       row.Breadcrumb,
		ProjectSlug:      projectSlug,
		SearchExcerpt:    excerpt,
		SearchExcerptKey: m.searchTextQuery() + "\x1e" + excerpt,
		Planning:         m.peekPlanningMembership(row),
		Dependencies:     outgoingDependencyDetails(row.Task, index),
		Dependents:       incomingDependencyDetails(m.rowIdentity(row), index),
		DependencyCensus: DependencyCensusFor(row.Task, index),
	})
}

// peekPlanningMembership names the planning collection a task belongs to, or
// nothing. Membership is neither a lifecycle state nor a field on the task: it is
// an explicit project-qualified entry in the planning state, and Today and Next
// are mutually exclusive, so one name is the whole answer. This is the impure
// half of the document — the renderer below it reads only what it is handed — and
// it is resolved here for the same reason the reference lists are.
func (m *OutlinerModel) peekPlanningMembership(row OutlinerRow) string {
	if m == nil || m.Runtime == nil || row.Task == nil {
		return ""
	}
	identity := m.rowIdentity(row)
	if identity.ProjectID == "" || identity.TaskID == "" {
		return ""
	}
	locator := config.ProjectTaskLocator{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
	planning := m.Runtime.Planning()
	switch {
	case slices.Contains(planning.GetToday(), locator):
		return chrome.PlanStatusValue("today")
	case slices.Contains(planning.GetNext(), locator):
		return chrome.PlanStatusValue("next")
	}
	return ""
}

func (m *OutlinerModel) inlinePeekOffsetKey(row OutlinerRow, doc inlinePeekDocument) inlinePeekOffsetKey {
	return inlinePeekOffsetKey{
		owner:       m.peekOwnerForRow(row),
		fingerprint: doc.Fingerprint,
		query:       m.searchTextQuery(),
		width:       m.Width(),
		height:      m.Height(),
	}
}

// peekOwnerForRow types the row the offset belongs to, on the same split
// inlinePeekDocumentForRow builds it on.
func (m *OutlinerModel) peekOwnerForRow(row OutlinerRow) inlinePeekOwner {
	if row.IsProjectHeader {
		return projectPeekOwner(row.ProjectID)
	}
	if checklistPeekItemValid(row) {
		return inlinePeekOwner{
			kind: inlinePeekOwnerChecklist, task: m.rowIdentity(row),
			checklistIndex: row.ChecklistIndex, checklistText: row.Task.Checklist[row.ChecklistIndex].Text,
		}
	}
	return taskPeekOwner(m.rowIdentity(row))
}

// peekOffsetForRow is pure: unchanged frames use the stored row, while reflow
// and edits resolve the saved content anchor in the current document. Only a
// scroll writes the anchor, so temporary shrinking cannot erase reading position.
func (m *OutlinerModel) peekOffsetForRow(row OutlinerRow, doc inlinePeekDocument) int {
	if m == nil || !m.peek.key.owner.sameAs(m.peekOwnerForRow(row)) || m.peek.key.query != m.searchTextQuery() {
		return 0
	}
	if m.peek.key.sameAs(m.inlinePeekOffsetKey(row, doc)) {
		return doc.ClampOffset(m.peek.offset)
	}
	return doc.offsetForAnchor(m.peek.anchor)
}

// peekCursorDocument is the document the dock draws and the scrolling keys act
// on, or false when there is none: no dock on the frame, a cursor off the rows,
// or a row that owns no document. An open Peek alone is not enough — it survives
// the cursor moving onto a section header or a note row, where the dock draws its
// placeholder and there is nothing to page.
//
// The frame's geometry is stamped onto the document here, which is the one place
// content and window meet: every reader below takes the window off the document
// it was handed rather than measuring the frame again.
func (m *OutlinerModel) peekCursorDocument() (OutlinerRow, inlinePeekDocument, bool) {
	geometry := m.peekDockGeometry()
	if m == nil || !geometry.Displayed() || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
	row := m.Rows[m.Cursor]
	if !rowSupportsPeek(row) {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
	width := m.peekDockWidth()
	doc := m.inlinePeekDocumentForRow(row, peekDockIndent, width, geometry)
	// A header naming a project the population no longer holds draws nothing, and
	// nothing on screen is nothing to page: the four keys belong to the list again
	// rather than being swallowed by a pane that is not there.
	if doc.Empty() {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
	return row, doc, true
}

// CanScrollPeek reports whether the window can move at all in a direction, +1
// down and -1 up. One answer covers all four keys, because a half page and a
// full page clamp against the same two ends: where the shorter step cannot
// move, neither can the longer one. It decides what the footer offers and
// nothing about what the keys consume — a Peek on screen consumes them at both
// boundaries rather than letting the press fall through to the list.
func (m *OutlinerModel) CanScrollPeek(direction int) bool {
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return false
	}
	current := m.peekOffsetForRow(row, doc)
	return doc.ClampOffset(current+direction) != current
}

// scrollPeek moves the window by a number of content rows and reports whether it
// moved. It is the only writer of the offset and content anchor. Both distances
// come through here, so a half page and a full page are one
// movement measured twice rather than two paging models.
func (m *OutlinerModel) scrollPeek(rows int) bool {
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return false
	}
	current := m.peekOffsetForRow(row, doc)
	next := doc.ClampOffset(current + rows)
	if next == current {
		return false
	}
	m.peek = peekOffsetState{key: m.inlinePeekOffsetKey(row, doc), offset: next, anchor: doc.readingAnchor(next)}
	return true
}

// HalfPagePeek moves Peek a half page, +1 down and -1 up, and reports whether it
// moved. It is Ctrl+D and Ctrl+U over an open Peek: the same distance those keys
// move a list cursor, spent on the pane that is on screen instead. The distance
// is the window's half rather than a constant, so the keys keep meaning "half of
// what I can see" at every dock height.
func (m *OutlinerModel) HalfPagePeek(direction int) bool {
	return m.scrollPeek(direction * m.peekDockGeometry().HalfPageRows())
}

// PagePeek moves Peek a full page, +1 down and -1 up, and reports whether it
// moved. Ctrl+F and Ctrl+B keep the one-row overlap every other pageable body on
// the product gives them, measured against the window that is actually drawn.
func (m *OutlinerModel) PagePeek(direction int) bool {
	return m.scrollPeek(direction * m.peekDockGeometry().FullPageRows())
}

// peekScrollHints names the movements that can actually happen. Only the half
// page is offered: it is the smaller commitment and the one a reader reaches for,
// and advertising Ctrl+F and Ctrl+B beside it spent a footer entry restating the
// same direction. The full-page pair still answers and `?` help still names it.
// The Peek footer is a hand-written hint list rather than the strategy's ranks,
// so these read their keys off the strategy instead of spelling them: the hint
// cannot drift from the binding it teaches.
func (m *OutlinerModel) peekScrollHints() []chrome.GuidanceHint {
	peek := m.PeekCommandContext()
	if !peek.Visible {
		return nil
	}
	hints := make([]chrome.GuidanceHint, 0, 2)
	if peek.CanPageDown {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekHalfPageDown, "Ctrl+D")+" more", peekHalfPageHintPriority, false))
	}
	if peek.CanPageUp {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekHalfPageUp, "Ctrl+U")+" back", peekHalfPageHintPriority, false))
	}
	return hints
}

// The priority the Peek guidance spends on movement: under the row of ordinary
// optional hints.
const peekHalfPageHintPriority = 9

// peekDisplayKey is the key Peek answers to when the registry cannot be asked.
// The guidance and the header projection both spell it through here rather than
// inline, so the day it moves again there is one literal to change.
const peekDisplayKey = "b"

func (m *OutlinerModel) peekOperationKey(id operation.OperationID, fallback string) string {
	if m == nil || m.OperationRegistry == nil {
		return fallback
	}
	if key := m.OperationRegistry.InputShortcutForOperation(operation.InputStrategySimplified, id); key != "" {
		return key
	}
	return fallback
}

// PeekCommandContext is Peek's contribution to the operation grammar's view of
// the world, and it answers two different questions with two different fields.
// Visible is consumption, and it is the dock's own open-or-closed state rather
// than the cursor document's: the four scrolling descriptors are applicable
// under it alone, so a dock on screen swallows every one of those keys — at
// either boundary, over a document too short to move, and over a row showing the
// placeholder — rather than letting the press reach the list cursor underneath.
// The delegated handler is where a row that names no document refuses by name.
// CanPageDown and CanPageUp are availability: they say whether the window has
// anywhere to go, and only the footer hints read them, because an offer that
// cannot move is worse than no offer while a key that silently moves the wrong
// thing is worse than both.
func (m *OutlinerModel) PeekCommandContext() operation.PeekContext {
	if !m.PeekDockVisible() {
		return operation.PeekContext{}
	}
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return operation.PeekContext{Visible: true}
	}
	current := m.peekOffsetForRow(row, doc)
	return operation.PeekContext{
		Visible:     true,
		CanPageDown: current < doc.MaxOffset(),
		CanPageUp:   current > 0,
	}
}

// peekSearchExcerpt reads the source attributed by the live projection and its
// actual case mode. A predicate-only query and a title hit need no explanation.
func (m *OutlinerModel) peekSearchExcerpt(row OutlinerRow) string {
	if !m.HasSearchProjection() || row.Task == nil || m.searchTextQuery() == "" {
		return ""
	}
	label := matchSourceLabels[row.MatchSource]
	if label == "" {
		return ""
	}
	text := searchMatchText(row.Task, m.projectByGroupID(row.ProjectID), row.MatchSource)
	_, positions := fuzzy.NewMatcher(m.searchCaseSensitive()).ScoreTermsWithPositions(text, m.searchTextQuery())
	if len(positions) == 0 {
		return ""
	}
	return label + " · " + searchMatchExcerpt(text, positions)
}

// A reading position belongs to content within a named section, independent
// of terminal cells, wrapping, palette, and the sections above it.
type peekReadingAnchor struct {
	section   string
	text      []rune
	character int
}

func peekReadingText(line string) []rune {
	return []rune(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, ansi.Strip(line)))
}

func (doc inlinePeekDocument) readingAnchor(offset int) peekReadingAnchor {
	for _, section := range doc.sections {
		if offset >= len(section.lines) {
			offset -= len(section.lines)
			continue
		}
		name, _, _ := strings.Cut(section.key, ":")
		anchor := peekReadingAnchor{section: name}
		for i, line := range section.lines {
			text := peekReadingText(line)
			if i < offset {
				anchor.character += len(text)
			}
			anchor.text = append(anchor.text, text...)
		}
		return anchor
	}
	return peekReadingAnchor{}
}

func (doc inlinePeekDocument) offsetForAnchor(anchor peekReadingAnchor) int {
	start := 0
	for _, section := range doc.sections {
		name, _, _ := strings.Cut(section.key, ":")
		if name != anchor.section {
			start += len(section.lines)
			continue
		}
		var text []rune
		ends := make([]int, len(section.lines))
		for i, line := range section.lines {
			text = append(text, peekReadingText(line)...)
			ends[i] = len(text)
		}
		character := relocatedPeekCharacter(anchor.text, text, anchor.character)
		for i, end := range ends {
			if character < end {
				return doc.ClampOffset(start + i)
			}
		}
		return doc.ClampOffset(start + max(len(section.lines)-1, 0))
	}
	// If the section itself was removed, there is no honest place to resume.
	return 0
}

// Preserve a point in the unchanged prefix or suffix of an edit. This covers
// appending text, inserting before the passage, and changes in other sections.
// If the passage itself was replaced, resume at the start of that section.
func relocatedPeekCharacter(before, after []rune, character int) int {
	prefix := 0
	for prefix < min(len(before), len(after)) && before[prefix] == after[prefix] {
		prefix++
	}
	if character < prefix || prefix == len(before) {
		return character
	}
	suffix := 0
	for suffix < min(len(before), len(after))-prefix && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	if character >= len(before)-suffix {
		return max(character+len(after)-len(before), 0)
	}
	return 0
}
