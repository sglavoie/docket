// Package tui is the terminal client of the outline. It holds view state
// only: the zoom, the folds, the cursor, and one row being edited.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
	"github.com/sglavoie/docket/internal/store"
)

// Backend is what the client needs from storage.
type Backend interface {
	Commit(ops []outline.Op, now int64) error
	SetFold(id string, folded bool) error
	Meta(key string) (string, error)
	SetMeta(key, value string) error
	Sync(dir string) (int, error)
	SyncBegin(dir string) (*store.SyncJob, error)
	SyncEnd(read *store.SyncRead) (store.SyncReport, error)
	DailyBackup(day string) error
	Stale() (bool, error)
	Load() (*outline.Tree, error)
}

const (
	metaHistory    = "history"     // the undo and redo stacks
	metaHideClosed = "hide_closed" // whether outlines leave closed rows out
	metaFilters    = "filters"     // saved filters from before they were rows
	metaSyncDir    = "sync_dir"    // the folder shared with other machines
	undoDepth      = 200
)

type tickMsg struct{}

// syncDoneMsg is a pass's work in the folder, finished and ready to apply.
type syncDoneMsg struct{ read *store.SyncRead }

// tick wakes the screen once a minute, so a day that turns over while the
// app is open is seen without a key press.
func tick() tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg { return tickMsg{} })
}

type visRow struct {
	id    string
	depth int
}

// editor is the one row being typed into. Its operations are already applied
// to the tree and are stored only when the edit ends, so one edit is one
// transaction and one undo step.
type editor struct {
	row        string
	buf        []rune
	caret, off int
	fresh      bool   // the row was created by this edit
	notes      bool   // the default the line was rendered against
	back       string // where the cursor returns if a fresh row is abandoned
	ops, inv   []outline.Op
}

type Model struct {
	tree    *outline.Tree
	backend Backend
	now     func() int64
	newID   func() string

	width, height int
	at            place
	zoom          []place // where Esc returns to
	folded        map[string]bool
	vis           []visRow
	cursor, top   int

	side       []entry // the sidebar's destinations, top to bottom
	sel        entry   // the one the outline pane was opened from
	sideFocus  bool
	lists      map[string][]string
	stats      map[string]entry // each active project's sidebar line, by row
	filters    []savedFilter
	syncDir    string
	syncNote   string // the sidebar's last word on sync
	syncFailed bool
	// A pass works in the folder off the keys' path. syncIO is that work
	// while a pass is under way; syncSent says the program has been handed
	// it; syncHeld is a finished pass waiting for a line to be typed out.
	syncIO    func() tea.Msg
	syncSent  bool
	syncHeld  *syncDoneMsg
	syncLoud  bool   // the pass was asked for, so its outcome is said
	syncWant  bool   // another pass was asked for while one was under way
	syncAgain bool   // this pass is the one retry a missing file is owed
	backedUp  string // the day of the newest snapshot
	scanned   scanKey
	tags      []string // tags in use, most used first
	parked    int

	undo, redo [][]outline.Op
	prefix     string            // a pending g or d
	picked     map[string]bool   // the selection
	last       map[string]string // each place's cursor row, for coming back
	hideClosed bool
	quit       bool
	edit       *editor
	ask        *prompt
	help       bool
	helpTop    int  // first visible help line
	helpG      bool // a pending g in help
	msg        string
	pendingCmd tea.Cmd // command produced by normalKey for Update to deliver
}

func New(tree *outline.Tree, backend Backend, folds map[string]bool) *Model {
	return newModel(tree, backend, folds, func() int64 { return time.Now().UnixMilli() })
}

func newModel(tree *outline.Tree, backend Backend, folds map[string]bool, now func() int64) *Model {
	if folds == nil {
		folds = map[string]bool{}
	}
	m := &Model{
		tree:    tree,
		backend: backend,
		folded:  folds,
		picked:  map[string]bool{},
		last:    map[string]string{},
		now:     now,
		newID:   store.NewID,
	}
	if v, err := backend.Meta(metaHideClosed); err == nil {
		m.hideClosed = v == "1"
	}
	if v, err := backend.Meta(metaHistory); err == nil && v != "" {
		var h history
		if json.Unmarshal([]byte(v), &h) == nil {
			m.undo, m.redo = h.Undo, h.Redo
		}
	}
	m.syncDir, _ = backend.Meta(metaSyncDir)
	m.ensureFixed(inboxID, "Inbox")
	m.ensureFixed(filtersID, "Filters")
	m.adoptOldFilters()
	m.backedUp = m.today().Format("2006-01-02")
	m.syncStart(false)
	m.rebuild()
	// Launch where the day's work is, or among the projects when nothing
	// has been picked yet.
	if len(m.lists[kindToday]) > 0 {
		m.open(entry{kind: kindToday})
	} else {
		m.open(entry{kind: kindProjects})
	}
	return m
}

func (m *Model) Init() tea.Cmd { return tea.Batch(tick(), m.syncCmd()) }

// history is the undo and redo stacks as they are kept between sessions.
// They belong to this machine: each step undoes a change made here.
type history struct {
	Undo, Redo [][]outline.Op
}

func (m *Model) keepHistory() {
	if n := len(m.undo); n > undoDepth {
		m.undo = m.undo[n-undoDepth:]
	}
	if b, err := json.Marshal(history{m.undo, m.redo}); err == nil {
		m.backend.SetMeta(metaHistory, string(b))
	}
}

// reload reads the outline again from the database, keeping the cursor on
// its row.
func (m *Model) reload() bool {
	tree, err := m.backend.Load()
	if err != nil {
		m.msg = "could not read the outline: " + err.Error()
		return false
	}
	cur := m.current()
	m.tree = tree
	m.refilter()
	m.rebuild()
	m.seek(cur)
	return true
}

// refresh reads the outline again when another window on this database has
// changed it. It waits while a row is being typed, since that row exists
// only here.
func (m *Model) refresh() {
	if m.edit != nil {
		return
	}
	if stale, err := m.backend.Stale(); err == nil && stale {
		m.reload()
	}
}

// adoptOldFilters turns saved filters from before they were rows into rows.
func (m *Model) adoptOldFilters() {
	v, err := m.backend.Meta(metaFilters)
	if err != nil || v == "" {
		return
	}
	var old []savedFilter
	if json.Unmarshal([]byte(v), &old) != nil {
		return
	}
	for _, f := range old {
		flat := ""
		if f.Flat {
			flat = "1"
		}
		id := m.newID()
		ops := []outline.Op{
			{Kind: outline.Insert, Row: id, Parent: filtersID, Pos: m.tree.PosLast(filtersID)},
			{Kind: outline.Set, Row: id, Field: sigil.FieldText, Value: f.Name},
			{Kind: outline.Set, Row: id, Field: fieldQuery, Value: f.Query},
			{Kind: outline.Set, Row: id, Field: fieldFlat, Value: flat},
			{Kind: outline.Set, Row: id, Field: fieldRoot, Value: f.Root},
		}
		if _, err := m.apply(ops); err != nil || m.backend.Commit(ops, m.now()) != nil {
			return
		}
	}
	m.backend.SetMeta(metaFilters, "")
}

// syncStart begins exchanging operations with the other machines. What is
// to be written is read here; the folder, which may be slow, is worked in
// away from the keys, and syncDone applies what was found there. It waits
// while a line is being typed, since that line is not stored yet.
func (m *Model) syncStart(loud bool) {
	if m.syncDir == "" || m.edit != nil || m.ask != nil {
		return
	}
	m.syncLoud = m.syncLoud || loud
	if m.syncIO != nil {
		m.syncWant = m.syncWant || loud
		return
	}
	job, err := m.backend.SyncBegin(m.syncDir)
	if err != nil {
		m.syncFail(err)
		return
	}
	m.syncIO = func() tea.Msg { return syncDoneMsg{job.Run()} }
}

// syncCmd hands the program a pass that has begun, once.
func (m *Model) syncCmd() tea.Cmd {
	if m.syncIO == nil || m.syncSent {
		return nil
	}
	m.syncSent = true
	return m.syncIO
}

func (m *Model) syncFail(err error) {
	m.syncNote, m.syncFailed = "sync failed", true
	m.msg = "sync failed: " + err.Error()
	m.syncLoud, m.syncAgain = false, false
}

// syncSettle applies a finished pass once nothing is being typed: what it
// brings may redraw the outline, and a typed line exists only here.
func (m *Model) syncSettle() {
	if m.syncHeld == nil || m.edit != nil || m.ask != nil {
		return
	}
	done := *m.syncHeld
	m.syncHeld, m.syncIO, m.syncSent = nil, nil, false
	if m.syncDir == "" {
		m.syncLoud, m.syncWant, m.syncAgain = false, false, false
		return
	}
	report, err := m.backend.SyncEnd(done.read)
	if err != nil {
		m.syncFail(err)
		return
	}
	// This machine's file was gone from the folder: it is written whole.
	if report.Again && !m.syncAgain {
		m.syncAgain = true
		m.syncStart(false)
		return
	}
	loud := m.syncLoud
	m.syncLoud, m.syncAgain = false, false
	m.syncNote, m.syncFailed = "synced "+m.today().Format("15:04"), false
	if report.Waiting > 0 {
		// Changes that name a row which has not arrived yet.
		m.syncNote += fmt.Sprintf(" · %d waiting", report.Waiting)
	}
	switch {
	case report.Changed > 0:
		if m.reload() {
			m.msg = fmt.Sprintf("synced: %d changes from other machines", report.Changed)
		}
	case loud && report.Machines == 1:
		m.msg = "synced with 1 other machine"
	case loud:
		m.msg = fmt.Sprintf("synced with %d other machines", report.Machines)
	}
	// A file that cannot be read past a line holds back everything after
	// it, so it is said for as long as it lasts.
	if len(report.Stuck) > 0 {
		at := report.Stuck[0]
		m.syncNote, m.syncFailed = "sync stuck", true
		m.msg = fmt.Sprintf("sync stuck: line %d of %s cannot be read, and nothing after it is applied", at.Line, at.File)
		if more := len(report.Stuck) - 1; more > 0 {
			m.msg += fmt.Sprintf(" (and %d more)", more)
		}
	}
	if m.syncWant {
		m.syncWant = false
		m.syncStart(true)
	}
}

// syncQuit is the last pass, done whole before the program ends.
func (m *Model) syncQuit() {
	if m.syncDir != "" {
		m.backend.Sync(m.syncDir)
	}
}

// backup takes the day's snapshot when the day has turned over while the
// app stayed open.
func (m *Model) backup() {
	day := m.today().Format("2006-01-02")
	if day == m.backedUp {
		return
	}
	m.backedUp = day
	if err := m.backend.DailyBackup(day); err != nil {
		m.msg = "no backup today: " + err.Error()
	}
}

// placeKey names a place for remembering its cursor.
func placeKey(p place) string { return p.list + "|" + p.root }

// leave remembers where the cursor was, and forgets the rows on screen so
// the next place is read fresh.
func (m *Model) leave() {
	if cur := m.current(); cur != "" {
		m.last[placeKey(m.at)] = cur
	}
	m.vis = nil
}

// arrive puts the cursor back where it was the last time this place showed.
func (m *Model) arrive() {
	m.rebuild()
	m.seek(m.last[placeKey(m.at)])
}

// Notice shows a message on the status line until the next key.
func (m *Model) Notice(msg string) { m.msg = msg }

// rebuild recomputes the visible rows and the sidebar, and keeps the cursor
// inside the rows.
func (m *Model) rebuild() {
	// A closed row that was on screen stays until the place is left, so
	// closing a row never pulls it from under the cursor.
	onScreen := map[string]bool{}
	for _, v := range m.vis {
		onScreen[v.id] = true
	}
	hidden := func(id string) bool {
		if !m.hideClosed || !sigil.FromFields(m.tree.Row(id).Fields).Closed() {
			return false
		}
		if !onScreen[id] && !m.at.kept[id] {
			return true
		}
		if m.at.kept == nil {
			m.at.kept = map[string]bool{}
		}
		m.at.kept[id] = true
		return false
	}
	m.vis = m.vis[:0]
	if m.at.query != "" {
		for _, v := range m.at.hits {
			if m.live(v.id) {
				m.vis = append(m.vis, v)
			}
		}
	} else if m.at.list != "" {
		for _, id := range m.at.rows {
			if m.live(id) {
				m.vis = append(m.vis, visRow{id, 0})
			}
		}
	} else {
		var walk func(parent string, depth int)
		walk = func(parent string, depth int) {
			for _, id := range m.tree.Children(parent) {
				if hiddenTop(id) || hidden(id) {
					continue
				}
				m.vis = append(m.vis, visRow{id, depth})
				if !m.folded[id] {
					walk(id, depth+1)
				}
			}
		}
		walk(m.at.root, 0)
	}
	m.cursor = max(0, min(m.cursor, len(m.vis)-1))
	for id := range m.picked {
		if !m.live(id) {
			delete(m.picked, id)
		}
	}
	m.scan()
}

// flat refuses an act that needs the whole tree in view: while a list or a
// filter is showing, a row's siblings are not all on screen.
func (m *Model) flat() bool {
	switch {
	case m.at.query != "":
		m.msg = "clear the filter first (Esc)"
	case m.at.list != "":
		m.msg = "a list has no order: press Enter on a row to work inside it"
	default:
		return false
	}
	return true
}

// single refuses an act that works on one row while several are selected.
func (m *Model) single() bool {
	if len(m.picked) > 0 {
		m.msg = "that works on one row: press Esc to clear the selection"
		return false
	}
	return true
}

// prevSibling is the row drawn above id among its siblings.
func (m *Model) prevSibling(id string) string {
	if prev := m.tree.PrevSibling(id); !hiddenTop(prev) {
		return prev
	}
	return ""
}

func (m *Model) current() string {
	if m.cursor < len(m.vis) {
		return m.vis[m.cursor].id
	}
	return ""
}

// seek puts the cursor on id when it is visible.
func (m *Model) seek(id string) {
	for i, v := range m.vis {
		if v.id == id {
			m.cursor = i
			return
		}
	}
}

func (m *Model) setFold(id string, folded bool) {
	if m.folded[id] == folded {
		return
	}
	if folded {
		m.folded[id] = true
	} else {
		delete(m.folded, id)
	}
	if err := m.backend.SetFold(id, folded); err != nil {
		m.msg = "not saved: " + err.Error()
	}
}

// apply runs ops against the tree and returns the group that undoes them,
// newest first. A failure leaves the tree as it was.
func (m *Model) apply(ops []outline.Op) ([]outline.Op, error) {
	now := m.now()
	inv := make([]outline.Op, 0, len(ops))
	for _, op := range ops {
		undo, err := m.tree.Apply(op, now)
		if err != nil {
			for _, u := range inv {
				m.tree.Apply(u, now)
			}
			return nil, err
		}
		inv = append([]outline.Op{undo}, inv...)
	}
	return inv, nil
}

// save stores an applied group. If it cannot be stored the group is taken
// back out of the tree, so memory never holds what the disk does not.
func (m *Model) save(ops, inv []outline.Op) bool {
	err := m.backend.Commit(ops, m.now())
	if err == nil {
		// The write may have found another window's changes.
		m.refresh()
		return true
	}
	now := m.now()
	for _, u := range inv {
		m.tree.Apply(u, now)
	}
	m.msg = "not saved, nothing changed: " + err.Error()
	return false
}

// record stores an applied group and makes it the next undo step.
func (m *Model) record(ops, inv []outline.Op) bool {
	if !m.save(ops, inv) {
		return false
	}
	m.undo = append(m.undo, inv)
	m.redo = nil
	m.keepHistory()
	return true
}

func (m *Model) do(ops ...outline.Op) bool {
	inv, err := m.apply(ops)
	if err != nil {
		m.msg = err.Error()
		return false
	}
	return m.record(ops, inv)
}

// replay moves one step between the undo and redo stacks.
func (m *Model) replay(from, to *[][]outline.Op, empty string) {
	if len(*from) == 0 {
		m.msg = empty
		return
	}
	group := (*from)[len(*from)-1]
	inv, err := m.apply(group)
	if err != nil {
		m.msg = err.Error()
		return
	}
	if !m.save(group, inv) {
		m.rebuild()
		return
	}
	*from = (*from)[:len(*from)-1]
	*to = append(*to, inv)
	m.keepHistory()
	m.rebuild()
	m.seek(group[len(group)-1].Row)
}

func (m *Model) indentOp(id string) (outline.Op, bool) {
	prev := m.prevSibling(id)
	if prev == "" {
		m.msg = "no row above to indent under"
		return outline.Op{}, false
	}
	return outline.Op{Kind: outline.Move, Row: id, Parent: prev, Pos: m.tree.PosLast(prev)}, true
}

func (m *Model) outdentOp(id string) (outline.Op, bool) {
	r := m.tree.Row(id)
	if r.Parent == m.at.root {
		m.msg = "already at the top level"
		return outline.Op{}, false
	}
	p := m.tree.Row(r.Parent)
	return outline.Op{Kind: outline.Move, Row: id, Parent: p.Parent, Pos: m.tree.PosAfter(p.Parent, p.ID)}, true
}

// shift indents or outdents id, inside the open edit when there is one.
func (m *Model) shift(id string, indent bool) {
	if m.flat() {
		return
	}
	op, ok := m.outdentOp(id)
	if indent {
		op, ok = m.indentOp(id)
	}
	if !ok {
		return
	}
	if m.edit != nil {
		inv, err := m.apply([]outline.Op{op})
		if err != nil {
			m.msg = err.Error()
			return
		}
		m.edit.ops = append(m.edit.ops, op)
		m.edit.inv = append(inv, m.edit.inv...)
	} else if !m.do(op) {
		return
	}
	if indent {
		m.setFold(op.Parent, false)
	}
	m.rebuild()
	m.seek(id)
}

// selectedRoots drops descendants of selected rows, preserving acting order.
func (m *Model) selectedRoots(ids []string) []string {
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	var roots []string
	for _, id := range ids {
		p := m.tree.Row(id).Parent
		for p != "" && !selected[p] {
			p = m.tree.Row(p).Parent
		}
		if p == "" {
			roots = append(roots, id)
		}
	}
	return roots
}

// selectedBlock requires adjacent siblings and orders them by the tree.
// Folds can hide selections, so acting order need not be sibling order.
func (m *Model) selectedBlock(ids []string) []string {
	roots := m.selectedRoots(ids)
	selected := make(map[string]bool, len(roots))
	for _, id := range roots {
		selected[id] = true
	}
	if len(roots) == 0 {
		return nil
	}
	parent := m.tree.Row(roots[0]).Parent
	for _, id := range roots {
		if m.tree.Row(id).Parent != parent {
			m.msg = "select rows that sit together to move them"
			return nil
		}
	}
	var block []string
	for _, id := range m.tree.Children(parent) {
		if selected[id] {
			block = append(block, id)
		} else if len(block) > 0 {
			break
		}
	}
	if len(block) != len(roots) {
		m.msg = "select rows that sit together to move them"
		return nil
	}
	return block
}

func (m *Model) moveBlock(ids []string, key string) {
	if m.flat() {
		return
	}
	block := m.selectedBlock(ids)
	if len(block) == 0 {
		return
	}
	first, last := block[0], block[len(block)-1]
	parent := m.tree.Row(first).Parent
	var pos, right string
	switch key {
	case ">", "<":
		var op outline.Op
		var ok bool
		if key == ">" {
			op, ok = m.indentOp(first)
		} else {
			op, ok = m.outdentOp(first)
			if next := m.tree.NextSibling(parent); next != "" {
				right = m.tree.Row(next).Pos
			}
		}
		if !ok {
			return
		}
		parent, pos = op.Parent, op.Pos
	case "J":
		next := m.tree.NextSibling(last)
		if next == "" {
			return
		}
		pos = m.tree.PosAfter(parent, next)
		if after := m.tree.NextSibling(next); after != "" {
			right = m.tree.Row(after).Pos
		}
	case "K":
		prev := m.prevSibling(first)
		if prev == "" {
			return
		}
		pos, right = m.tree.PosBefore(parent, prev), m.tree.Row(prev).Pos
	}
	var ops []outline.Op
	for _, id := range block {
		ops = append(ops, outline.Op{Kind: outline.Move, Row: id, Parent: parent, Pos: pos})
		pos = outline.Between(pos, right)
	}
	cur := m.current()
	if m.do(ops...) {
		if key == ">" {
			m.setFold(parent, false)
		}
		m.rebuild()
		m.seek(cur)
	}
}

func (m *Model) reorder(id string, down bool) {
	if m.flat() {
		return
	}
	r := m.tree.Row(id)
	var pos string
	if down {
		next := m.tree.NextSibling(id)
		if next == "" {
			return
		}
		pos = m.tree.PosAfter(r.Parent, next)
	} else {
		prev := m.prevSibling(id)
		if prev == "" {
			return
		}
		pos = m.tree.PosBefore(r.Parent, prev)
	}
	if m.do(outline.Op{Kind: outline.Move, Row: id, Parent: r.Parent, Pos: pos}) {
		m.rebuild()
		m.seek(id)
	}
}

// newRow opens a blank row for typing: above the cursor, as the first child
// of an open parent, or else directly below.
func (m *Model) newRow(above bool) {
	if m.at.list != "" && m.at.query == "" {
		m.msg = "add rows inside a project, or press a for the Inbox"
		return
	}
	if m.flat() {
		return
	}
	cur := m.current()
	parent, pos := m.at.root, ""
	switch {
	case cur == "":
		pos = m.tree.PosLast(m.at.root)
	case above:
		parent = m.tree.Row(cur).Parent
		pos = m.tree.PosBefore(parent, cur)
	case len(m.tree.Children(cur)) > 0 && !m.folded[cur]:
		parent = cur
		pos = m.tree.PosAfter(cur, "")
	default:
		parent = m.tree.Row(cur).Parent
		pos = m.tree.PosAfter(parent, cur)
	}
	op := outline.Op{Kind: outline.Insert, Row: m.newID(), Parent: parent, Pos: pos}
	inv, err := m.apply([]outline.Op{op})
	if err != nil {
		m.msg = err.Error()
		return
	}
	m.rebuild()
	m.seek(op.Row)
	m.edit = &editor{row: op.Row, fresh: true, back: cur, ops: []outline.Op{op}, inv: inv}
}

func (m *Model) today() time.Time { return time.UnixMilli(m.now()).Local() }

// defaultsToNotes reports whether children of parent default to notes.
func (m *Model) defaultsToNotes(parent string) bool {
	return parent == "" || m.tree.Row(parent).Fields[sigil.FieldState] != ""
}

func (m *Model) editRow(id string) {
	row := m.tree.Row(id)
	line := sigil.FromFields(row.Fields)
	notes := m.defaultsToNotes(row.Parent)
	buf := []rune(sigil.Format(line, notes, m.today()))
	m.edit = &editor{row: id, buf: buf, caret: len(buf), notes: notes}
}

// setFields returns the operations that bring a row to want, skipping the
// fields that already agree. A change of state also restarts its clock.
func (m *Model) setFields(id string, want map[string]string) []outline.Op {
	have := m.tree.Row(id).Fields
	var ops []outline.Op
	field := func(name string) string {
		if v, ok := want[name]; ok {
			return v
		}
		return have[name]
	}
	for _, name := range []string{sigil.FieldText, sigil.FieldState, sigil.FieldFlag, sigil.FieldDue, sigil.FieldStart, sigil.FieldRepeat, sigil.FieldTags} {
		value, ok := want[name]
		if !ok || value == have[name] {
			continue
		}
		if every := field(sigil.FieldRepeat); name == sigil.FieldState && value == sigil.Done && every != "" {
			// A repeating row is never done: closing it moves it on to its
			// next day and leaves it open. With a deadline, that is the
			// next deadline; without one, the day it comes back.
			moves := sigil.FieldStart
			if field(sigil.FieldDue) != "" {
				moves = sigil.FieldDue
			}
			next := sigil.NextDate(field(moves), every, m.today())
			ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: moves, Value: next})
			if moves == sigil.FieldDue && field(sigil.FieldStart) != "" {
				ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: sigil.FieldStart,
					Value: sigil.NextDate(field(sigil.FieldStart), every, m.today())})
			}
			if have[sigil.FieldState] != sigil.Open {
				ops = append(ops,
					outline.Op{Kind: outline.Set, Row: id, Field: sigil.FieldState, Value: sigil.Open},
					outline.Op{Kind: outline.Set, Row: id, Field: sigil.FieldChanged, Value: strconv.FormatInt(m.now(), 10)})
			}
			label, _ := sigil.DueLabel(next, m.today())
			m.msg = "repeats: next " + strings.TrimPrefix(label, "@")
			continue
		}
		ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: name, Value: value})
		if name == sigil.FieldState {
			ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: sigil.FieldChanged, Value: strconv.FormatInt(m.now(), 10)})
		}
	}
	return ops
}

// finishEdit ends the open edit and reports whether its row was kept. There
// is no cancel: the line stands, undo takes it back, and only a fresh row
// left blank disappears.
func (m *Model) finishEdit() bool {
	e := m.edit
	m.edit = nil
	typed := strings.TrimSpace(string(e.buf))
	if e.fresh && typed == "" {
		m.tree.Forget(e.row)
		m.rebuild()
		m.seek(e.back)
		return false
	}
	// A fresh row takes its default from where it ended up; an existing one
	// keeps the default its line was rendered against, so indenting it while
	// typing never changes a state nobody touched.
	notes := e.notes
	if e.fresh {
		notes = m.defaultsToNotes(m.tree.Row(e.row).Parent)
	}
	ops := m.setFields(e.row, sigil.Parse(typed, notes, m.today()).Fields())
	if inv, err := m.apply(ops); err == nil {
		e.ops = append(e.ops, ops...)
		e.inv = append(inv, e.inv...)
	}
	if len(e.ops) > 0 && !m.record(e.ops, e.inv) {
		if e.fresh {
			m.tree.Forget(e.row)
		}
		m.rebuild()
		m.seek(e.back)
		return false
	}
	m.rebuild()
	m.seek(e.row)
	return true
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.backup()
		m.refresh()
		m.syncSettle()
		m.syncStart(false)
		m.rebuild()
		return m, tea.Batch(tick(), m.syncCmd())
	case syncDoneMsg:
		m.syncHeld = &msg
		m.syncSettle()
		m.rebuild()
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.PasteMsg:
		m.msg = ""
		m.paste(msg.Content)
		m.rebuild()
	case tea.KeyPressMsg:
		m.msg = ""
		m.refresh()
		if msg.String() == "ctrl+c" {
			if m.edit != nil {
				m.finishEdit()
			}
			if m.ask != nil && m.ask.kind == askCapture {
				m.capture(string(m.ask.ed.buf))
			}
			m.ask = nil
			m.syncQuit()
			return m, tea.Quit
		}
		switch {
		case m.help:
			m.helpKey(msg.String())
		case m.edit != nil:
			m.editKey(msg)
		case m.ask != nil:
			m.askKey(msg)
		case m.sideFocus:
			m.sideKey(msg.String())
		default:
			m.normalKey(msg.String())
		}
		// Every key may have changed what the sidebar counts.
		m.rebuild()
		if m.quit {
			m.syncQuit()
			return m, tea.Quit
		}
		m.syncSettle()
	}
	cmd := tea.Batch(m.pendingCmd, m.syncCmd())
	m.pendingCmd = nil
	return m, cmd
}

// prompt is the bottom line while it is asking for something. Its label
// names which of its acts it is doing.
type prompt struct {
	kind   string
	ed     editor
	rows   []string // the rows a move or a stamp lands on
	choice int      // which match a picker takes
}

const (
	askCapture = "capture" // a line for the Inbox
	askJump    = "jump"    // a project or list to go to
	askMove    = "move"    // a project to move rows into
	askFilter  = "filter"  // a query narrowing the pane, live
	askStamp   = "stamp"   // sigils to set on rows
	askCommand = "command"
)

var askLabels = map[string]string{
	askCapture: "Inbox › ",
	askJump:    "Go to › ",
	askMove:    "Move to › ",
	askFilter:  "Filter › ",
	askStamp:   "Stamp › ",
	askCommand: ": ",
}

func (m *Model) askKey(msg tea.KeyPressMsg) {
	a := m.ask
	k := msg.String()
	if m.complete(&a.ed, a.kind, k) {
		k = "" // the line changed; a filter reads it again below
	}
	text := strings.TrimSpace(string(a.ed.buf))

	if a.kind == askFilter {
		// The pane follows the query as it is typed. Enter keeps it and
		// returns to the rows; Esc drops it.
		switch k {
		case "enter":
			m.ask = nil
		case "esc":
			m.ask = nil
			m.at.query = ""
		case "tab":
			m.at.flat = !m.at.flat
		case "":
			m.at.query = text
		default:
			a.ed.key(msg)
			m.at.query = strings.TrimSpace(string(a.ed.buf))
		}
		m.refilter()
		m.cursor, m.top = 0, 0
		// A selection made before the rows changed could act on rows no
		// longer on screen.
		m.picked = map[string]bool{}
		return
	}

	switch k {
	case "":
	case "esc":
		m.ask = nil
		if a.kind == askCapture {
			m.capture(text)
		}
	case "enter":
		switch a.kind {
		case askCapture:
			// The line stays open for the next thought; a blank one closes it.
			m.capture(text)
			a.ed = editor{}
			if text == "" {
				m.ask = nil
			}
		case askStamp:
			c, bad := parseStamp(text, m.today())
			if bad != "" {
				m.msg = "cannot read " + bad
				return
			}
			m.ask = nil
			var ops []outline.Op
			for _, id := range a.rows {
				if m.live(id) {
					ops = append(ops, m.stampOps(id, c)...)
				}
			}
			if len(ops) > 0 {
				m.do(ops...)
			}
		case askCommand:
			m.ask = nil
			m.command(text)
		default:
			hits := match(m.targets(a.kind == askJump), text, 3)
			if len(hits) == 0 {
				m.msg = "nothing matches " + text
				return
			}
			hit := hits[min(a.choice, len(hits)-1)]
			m.ask = nil
			if a.kind == askJump {
				m.sideFocus = false
				m.open(hit)
				return
			}
			to := hit.rowID()
			var ops []outline.Op
			for _, id := range a.rows {
				if m.live(id) {
					// Each row is placed after the one before it.
					pos := m.tree.PosLast(to)
					if n := len(ops); n > 0 {
						pos = outline.Between(ops[n-1].Pos, "")
					}
					ops = append(ops, outline.Op{Kind: outline.Move, Row: id, Parent: to, Pos: pos})
				}
			}
			if len(ops) > 0 && m.do(ops...) {
				m.msg = "moved to " + hit.label
			}
		}
	case "down", "ctrl+n", "up", "ctrl+p":
		// A picker's arrows walk its matches.
		if a.kind == askJump || a.kind == askMove {
			n := len(match(m.targets(a.kind == askJump), text, 3))
			if k == "down" || k == "ctrl+n" {
				a.choice++
			} else {
				a.choice--
			}
			a.choice = max(0, min(a.choice, n-1))
		}
	default:
		a.ed.key(msg)
		a.choice = 0
	}
}

// command runs a line typed after a colon.
func (m *Model) command(line string) {
	words := strings.Fields(line)
	if len(words) == 0 {
		return
	}
	switch words[0] {
	case "q", "quit":
		m.quit = true
	case "export":
		m.export(strings.Join(words[1:], " "))
	case "sync":
		dir := strings.Join(words[1:], " ")
		switch {
		case dir == "off":
			m.syncDir, m.syncNote, m.syncFailed = "", "", false
			m.msg = "sync is off"
		case dir != "":
			if rest, ok := strings.CutPrefix(dir, "~/"); ok {
				if home, err := os.UserHomeDir(); err == nil {
					dir = filepath.Join(home, rest)
				}
			}
			m.syncDir = dir
		case m.syncDir == "":
			m.msg = "name a folder the machines share: sync ~/folder"
			return
		}
		if err := m.backend.SetMeta(metaSyncDir, m.syncDir); err != nil {
			m.msg = "not saved: " + err.Error()
			return
		}
		if m.syncDir != "" {
			m.syncStart(true)
		}
	case "review":
		m.open(entry{kind: kindReview})
	case "save", "forget":
		m.keepFilter(words[0] == "save", strings.Join(words[1:], " "))
	case "closed":
		// Outlines leave done and canceled rows out, or show them again.
		m.hideClosed = !m.hideClosed
		value, said := "", "closed rows shown"
		if m.hideClosed {
			value, said = "1", "closed rows hidden"
		}
		if err := m.backend.SetMeta(metaHideClosed, value); err != nil {
			said = "not saved: " + err.Error()
		}
		m.vis, m.at.kept = nil, nil
		m.msg = said
	default:
		m.msg = "commands: q, export [file], closed, review, save name, forget name, sync [folder|off]"
	}
}

// sideKey handles the sidebar while it has focus. Moving shows each
// destination at once, so leaving is the only other act.
func (m *Model) sideKey(k string) {
	switch k {
	case "j", "down":
		m.sideStep(1)
	case "k", "up":
		m.sideStep(-1)
	case "tab", "enter", "esc", "l", "right":
		m.sideFocus = false
	case "p":
		m.ask = &prompt{kind: askJump}
	case "a":
		m.ask = &prompt{kind: askCapture}
	case "?":
		m.openHelp()
	}
}

func (m *Model) normalKey(k string) {
	if m.prefix != "" {
		seq := m.prefix + k
		m.prefix = ""
		switch seq {
		case "gg":
			m.cursor = 0
		case "dd":
			var ops []outline.Op
			for _, id := range m.acting() {
				ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: outline.FieldDeleted, Value: "1"})
			}
			if len(ops) > 0 && m.do(ops...) {
				m.rebuild()
			}
		}
		return
	}

	cur := m.current()
	switch k {
	case "g", "d":
		m.prefix = k
	case "j", "down":
		m.cursor = min(m.cursor+1, max(len(m.vis)-1, 0))
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
	case "G":
		m.cursor = max(len(m.vis)-1, 0)
	case "o":
		if m.single() {
			m.newRow(false)
		}
	case "O":
		if m.single() {
			m.newRow(true)
		}
	case "/":
		buf := []rune(m.at.query)
		m.ask = &prompt{kind: askFilter, ed: editor{buf: buf, caret: len(buf)}}
	case ":":
		m.ask = &prompt{kind: askCommand}
	case "u":
		m.replay(&m.undo, &m.redo, "nothing to undo")
	case "ctrl+r":
		m.replay(&m.redo, &m.undo, "nothing to redo")
	case "?":
		m.openHelp()
	case "tab":
		m.sideFocus = true
	case "p":
		m.ask = &prompt{kind: askJump}
	case "a":
		m.ask = &prompt{kind: askCapture}
	case "esc":
		// Esc undoes the most recent narrowing: the selection, then the
		// filter, then the zoom.
		if len(m.picked) > 0 {
			m.picked = map[string]bool{}
		} else if m.at.query != "" {
			m.at.query = ""
			m.refilter()
			m.vis = nil
			m.rebuild()
			m.seek(cur)
		} else if n := len(m.zoom); n > 0 {
			was := m.at.root
			m.leave()
			m.at, m.zoom = m.zoom[n-1], m.zoom[:n-1]
			m.rebuild()
			m.seek(was)
		}
	}
	if cur == "" {
		return
	}

	ids := m.acting()
	switch k {
	case "y":
		text, n := m.writeRows(m.selectedRoots(ids))
		m.pendingCmd = tea.SetClipboard(text)
		m.msg = fmt.Sprintf("copied %d rows", n)
	case "e":
		if m.single() {
			m.editRow(cur)
		}
	case "enter":
		if m.single() {
			m.leave()
			m.zoom = append(m.zoom, m.at)
			m.at, m.cursor = place{root: cur}, 0
			m.arrive()
		}
	case "v":
		if m.picked[cur] {
			delete(m.picked, cur)
		} else {
			m.picked[cur] = true
		}
		m.cursor = min(m.cursor+1, len(m.vis)-1)
	case "#", "@", "$", "-", "*":
		// A sigil typed on the rows opens the stamp line already holding it.
		m.ask = &prompt{kind: askStamp, ed: editor{buf: []rune(k), caret: 1}, rows: ids}
	case "h", "left":
		if len(m.tree.Children(cur)) > 0 && !m.folded[cur] {
			m.setFold(cur, true)
			m.rebuild()
		} else if p := m.tree.Row(cur).Parent; p != m.at.root {
			m.seek(p)
		}
	case "l", "right":
		if m.folded[cur] {
			m.setFold(cur, false)
			m.rebuild()
		} else if kids := m.tree.Children(cur); len(kids) > 0 {
			m.seek(kids[0])
		}
	case "x":
		// Done and back. Any other state is stamped, since it is rarer.
		var tasks []string
		done := true
		for _, id := range ids {
			if state := m.tree.Row(id).Fields[sigil.FieldState]; state != "" {
				tasks = append(tasks, id)
				done = done && state == sigil.Done
			}
		}
		if len(tasks) == 0 {
			m.msg = "a note has no state: stamp it with $open"
			break
		}
		to := sigil.Done
		if done {
			to = sigil.Open
		}
		var ops []outline.Op
		for _, id := range tasks {
			ops = append(ops, m.setFields(id, map[string]string{sigil.FieldState: to})...)
		}
		if len(ops) > 0 {
			m.do(ops...)
		}
	case "!":
		m.toggle(ids, sigil.FieldFlag, "1")
	case "t":
		// Today is a pick that lapses by itself: the field holds the day it
		// was made, and tomorrow that is no longer today.
		m.toggle(ids, sigil.FieldToday, m.today().Format("2006-01-02"))
	case "m":
		m.ask = &prompt{kind: askMove, rows: ids}
	case ">", "<":
		if len(m.picked) > 0 {
			m.moveBlock(ids, k)
		} else {
			m.shift(cur, k == ">")
		}
	case "J", "K":
		if len(m.picked) > 0 {
			m.moveBlock(ids, k)
		} else {
			m.reorder(cur, k == "J")
		}
	}
}

// complete takes the suggestion showing at the end of the line.
func (m *Model) complete(e *editor, surface string, k string) bool {
	rest := m.suggest(e, surface)
	if k != "right" || rest == "" {
		return false
	}
	e.insert([]rune(rest))
	return true
}

func (m *Model) editKey(msg tea.KeyPressMsg) {
	e := m.edit
	if m.complete(e, surfaceRow, msg.String()) {
		return
	}
	switch msg.String() {
	case "enter":
		// A fresh row chains into the next one, so a list is typed without
		// leaving the keyboard's home row.
		if m.finishEdit() && e.fresh {
			m.newRow(false)
		}
	case "esc":
		m.finishEdit()
	case "tab":
		m.shift(e.row, true)
	case "shift+tab":
		m.shift(e.row, false)
	case "backspace":
		if len(e.buf) == 0 && e.fresh {
			m.finishEdit()
		} else {
			e.key(msg)
		}
	default:
		e.key(msg)
	}
}

// key applies a text-editing key to the buffer.
func (e *editor) key(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "backspace":
		if e.caret > 0 {
			e.buf = append(e.buf[:e.caret-1], e.buf[e.caret:]...)
			e.caret--
		}
	case "delete":
		if e.caret < len(e.buf) {
			e.buf = append(e.buf[:e.caret], e.buf[e.caret+1:]...)
		}
	case "left":
		e.caret = max(e.caret-1, 0)
	case "right":
		e.caret = min(e.caret+1, len(e.buf))
	case "alt+left", "ctrl+left", "alt+b":
		e.caret = e.wordStart()
	case "alt+right", "ctrl+right", "alt+f":
		for e.caret < len(e.buf) && e.buf[e.caret] == ' ' {
			e.caret++
		}
		for e.caret < len(e.buf) && e.buf[e.caret] != ' ' {
			e.caret++
		}
	case "home", "ctrl+a":
		e.caret = 0
	case "end", "ctrl+e":
		e.caret = len(e.buf)
	case "ctrl+u":
		e.buf = e.buf[e.caret:]
		e.caret = 0
	case "ctrl+k":
		e.buf = e.buf[:e.caret]
	case "ctrl+w", "alt+backspace":
		i := e.wordStart()
		e.buf = append(e.buf[:i], e.buf[e.caret:]...)
		e.caret = i
	default:
		if msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			e.insert([]rune(msg.Text))
		}
	}
}

// wordStart is where the word at or before the caret begins.
func (e *editor) wordStart() int {
	i := e.caret
	for i > 0 && e.buf[i-1] == ' ' {
		i--
	}
	for i > 0 && e.buf[i-1] != ' ' {
		i--
	}
	return i
}

func (e *editor) insert(text []rune) {
	buf := make([]rune, 0, len(e.buf)+len(text))
	buf = append(buf, e.buf[:e.caret]...)
	buf = append(buf, text...)
	e.buf = append(buf, e.buf[e.caret:]...)
	e.caret += len(text)
}

func (m *Model) openHelp() {
	m.help, m.helpTop, m.helpG = true, 0, false
}

// helpPage is the number of help lines visible at once.
func (m *Model) helpPage() int {
	_, h := m.size()
	return max(h-1, 1)
}

// helpKey scrolls the help; any key it does not know closes it.
func (m *Model) helpKey(k string) {
	g := m.helpG
	m.helpG = false
	page := m.helpPage()
	switch k {
	case "j", "down", "ctrl+n":
		m.helpTop++
	case "k", "up", "ctrl+p":
		m.helpTop--
	case "ctrl+d":
		m.helpTop += page / 2
	case "ctrl+u":
		m.helpTop -= page / 2
	case "pgdown", "ctrl+f", "space":
		m.helpTop += page
	case "pgup", "ctrl+b":
		m.helpTop -= page
	case "G", "end":
		m.helpTop = 1 << 30
	case "home":
		m.helpTop = 0
	case "g":
		if g {
			m.helpTop = 0
		} else {
			m.helpG = true
		}
	default:
		m.help = false
	}
	m.clampHelp()
}

func (m *Model) clampHelp() {
	n := strings.Count(helpText, "\n") + 1
	m.helpTop = max(min(m.helpTop, n-m.helpPage()), 0)
}
