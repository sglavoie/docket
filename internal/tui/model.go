// Package tui is the terminal client of the outline. It holds view state
// only: the zoom, the folds, the cursor, and one row being edited.
package tui

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	SetFolds(changes map[string]bool) error
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
	metaNumbers    = "numbers"     // whether rows say how far they are from the cursor
	metaPrompts    = "prompts"     // lines kept on the filter, stamp and command prompts
	undoDepth      = 200
	promptDepth    = 50 // lines each prompt remembers
)

type tickMsg struct{}

// flashMsg ends the flash of copy number gen, unless a later copy has its own.
type flashMsg struct{ gen int }

// flashFor is how long a copy lights what it took. Tests set it to nothing,
// so a copy's timer does not hold them up.
var flashFor = 200 * time.Millisecond

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
	top        int // first drawn line of a tall editing row
	column     int // preferred column during consecutive vertical moves
	vertical   bool
	normal     bool   // the caret is on a character: keys are a grammar, not text
	inside     bool   // typing opened inside the row: Esc goes back to the caret
	sel        bool   // inside the row: a selection is being swept
	anchor     int    // inside the row: where that selection began
	pend       string // inside the row: a pending operator, and what it waits for
	count      int    // inside the row: a pending count
	opCount    int    // inside the row: the count before the operator
	past       []snapshot
	ahead      []snapshot
	cmd        []tea.KeyPressMsg // inside the row: the keys of the change being made
	how        string            // the key that opened the typing, while . can repeat it
	start      []rune            // the line as the typing found it
	at         int               // where in it the typing began
	fresh      bool              // the row was created by this edit
	merge      bool              // continue the undo step recorded by a split
	notes      bool              // the default the line was rendered against
	back       string            // where the cursor returns if a fresh row is abandoned
	body       bool              // the body of the row the pane opens, edited by lines
	side       bool              // a project's name typed in its sidebar entry: the line stays in its row
	sideC      bool              // opened by c in the sidebar: the typing it opens ends at Esc
	flash      [2]int            // the span y last took from the line, lit while it flashes
	// A walk through the colours a ~word could become: which is written,
	// zero being the word as typed, which walkDraft holds meanwhile.
	walk      int
	walkDraft string
	ops, inv  []outline.Op
}

// repeatAct holds the last change made to rows, for the dot key: a stamp, a
// named move, a link, a key with its count, an operator with what completed
// it, or text typed into a row. It is view state and is never saved with
// undo history.
type repeatAct struct {
	line   string
	to     entry
	link   entry
	key    string // an act on rows, or an operator
	motion string // what completed the operator: itself doubled, or a motion
	count  int
	how    string // the key that opened typing: o O A I cc
	typed  string // what was typed there
	old    string // text a substitution looks for
	new    string // and what it puts there
}

// mark notes how typing began, so that what it adds can be repeated.
func (e *editor) mark(how string) {
	e.how, e.start, e.at = how, slices.Clone(e.buf), e.caret
}

// inserted is what the typing added where it began, and whether adding
// there is all it did.
func (e *editor) inserted() (string, bool) {
	tail := len(e.start) - e.at
	if e.how == "" || len(e.buf) < len(e.start) || !slices.Equal(e.buf[:e.at], e.start[:e.at]) ||
		!slices.Equal(e.buf[len(e.buf)-tail:], e.start[e.at:]) {
		return "", false
	}
	return string(e.buf[e.at : len(e.buf)-tail]), true
}

type Model struct {
	tree    *outline.Tree
	backend Backend
	now     func() int64
	newID   func() string
	openURL func(string) error

	lightPalette  bool // dark until the terminal answers the background query
	width, height int
	at            place
	zoom          []place  // where Esc returns to
	older, newer  []*visit // the places jumps left, for Ctrl+O and Ctrl+I
	sideFrom      *visit   // where this sidebar visit began
	folded        map[string]bool
	vis           []visRow
	cursor, top   int

	side          []entry // the sidebar's destinations, top to bottom
	sel           entry   // the one the outline pane was opened from
	sideFocus     bool
	lists         map[string][]string
	stats         map[string]entry // each active project's sidebar line, by row
	oldestProject int              // the greatest age among them, which their ages are drawn against
	oldestRow     int64            // the greatest age among the Waiting or Stale rows drawn, in ms
	colW          []int            // how wide each column at a listed row's right edge is, or none
	filters       []savedFilter
	syncDir       string
	syncNote      string // the sidebar's last word on sync
	syncFailed    bool
	// A pass works in the folder off the keys' path. syncIO is that work
	// while a pass is under way; syncSent says the program has been handed
	// it; syncHeld is a finished pass waiting for a line to be typed out.
	syncIO     func() tea.Msg
	syncSent   bool
	syncHeld   *syncDoneMsg
	syncLoud   bool   // the pass was asked for, so its outcome is said
	syncWant   bool   // another pass was asked for while one was under way
	syncAgain  bool   // this pass is the one retry a missing file is owed
	backedUp   string // the day of the newest snapshot
	scanned    scanKey
	tags       []string       // tags in use, most used first
	tagUse     map[string]int // how many live rows carry each tag
	contexts   []string       // contexts in use, most used first
	contextUse map[string]int // how many live rows carry each context
	headings   []entry        // live heading destinations, in tree order
	rowNames   []entry        // every other live row with text, for the pickers
	parked     int
	next       map[string]bool     // each active project's next action, derived in scan
	first      map[string]bool     // each parent's first workable child, derived in scan
	inToday    map[string]bool     // the rows in Today, derived in scan
	missed     map[string]bool     // picks from an earlier day still open, derived in scan
	pointers   map[string][]string // the rows linking to each row, in outline order, derived in scan

	totals    map[string]rowTotals // own and descendant counts, derived in scan
	paneTotal taskTotals           // held rows, refreshed in rebuild
	hides     map[string]int       // in a filter, the rows each of its folds hides directly

	outlineOrder map[string]int // traversal ranks, derived in scan for Today ties

	repeat       repeatAct
	undo, redo   [][]outline.Op
	count        int                 // pending outline count; zero means absent
	undoGroup    []outline.Op        // inverses collected by a counted reorder
	prefix       string              // a pending g or z, or i after an operator
	operator     string              // pending d, y, > or <, or c (cc only)
	replaying    bool                // the dot key is typing: what it types is not remembered again
	textRepeat   textAct             // the last change made inside a row
	textRegister string              // what y, d, c and x inside a row last took
	charFind     charFind            // the last f, F, t or T inside a row
	opCount      int                 // count before the operator
	register     string              // session-local serialized rows
	copied       string              // what y last took, which a delete does not replace
	flashed      map[string]bool     // the rows y last took, lit while they flash
	flashGen     int                 // which copy is flashing
	past         map[string][]string // lines kept on each recalling prompt, oldest first
	centre       bool                // the cursor has landed somewhere: show it mid-pane if off screen
	scroll       byte                // zt zz zb: draw the cursor's row at the top, middle or bottom
	find         string              // the last filter kept or dropped, for n and N
	changes      []string            // the rows changed this session, oldest first, for g; and g,
	changeAt     int                 // where g; and g, stand in changes
	findHere     string              // the row that filter's &. stands for
	picked       map[string]bool     // the selection
	sweep        string              // anchor row of a live sweep
	sweepBase    map[string]bool     // picks made before the sweep
	last         map[string]string   // each place's cursor row, for coming back
	hideClosed   bool
	numbers      bool // rows say how far they are from the cursor
	quit         bool
	edit         *editor
	ask          *prompt
	help         bool
	helpTop      int     // first visible help line
	helpG        bool    // a pending g in help
	helpFind     *editor // the help's search line while it is typed
	helpFrom     int     // where help stood when its search line opened
	helpQuery    string  // what help is searched for; n and N step through it
	helpHit      int     // the help line of the match stood on
	msg          string
	bodyDrawn    bodyCache // the body last drawn, wrapped
	pendingCmd   tea.Cmd   // command produced by normalKey for Update to deliver
	recorded     bool      // a key made an undo step, so its outcome is the user's
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
		openURL: openURL,
	}
	if v, err := backend.Meta(metaHideClosed); err == nil {
		m.hideClosed = v == "1"
	}
	if v, err := backend.Meta(metaNumbers); err == nil {
		m.numbers = v == "1"
	}
	if v, err := backend.Meta(metaHistory); err == nil && v != "" {
		var h history
		if json.Unmarshal([]byte(v), &h) == nil {
			m.undo, m.redo = h.Undo, h.Redo
		}
	}
	m.past = map[string][]string{}
	if v, err := backend.Meta(metaPrompts); err == nil && v != "" {
		json.Unmarshal([]byte(v), &m.past)
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

func (m *Model) Init() tea.Cmd { return tea.Batch(tick(), m.syncCmd(), tea.RequestBackgroundColor) }

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

// recalls reports a prompt whose Up and Down bring back lines kept on it
// before. The pickers walk their matches with those keys instead.
func recalls(kind string) bool {
	return kind == askFilter || kind == askStamp || kind == askCommand
}

// remember keeps a line a prompt was given, newest last and once only. It is
// this machine's, like the folds, and outlasts the session.
func (m *Model) remember(kind, line string) {
	line = strings.TrimSpace(line)
	if line == "" || m.replaying {
		return
	}
	lines := slices.DeleteFunc(m.past[kind], func(l string) bool { return l == line })
	lines = append(lines, line)
	if n := len(lines); n > promptDepth {
		lines = lines[n-promptDepth:]
	}
	m.past[kind] = lines
	if b, err := json.Marshal(m.past); err == nil {
		m.backend.SetMeta(metaPrompts, string(b))
	}
}

// recall steps a prompt through the lines kept on it that begin with what
// was typed: older with Up, newer with Down, and back to the typed line.
func (m *Model) recall(a *prompt, older bool) {
	if a.recalled == 0 {
		a.draft = string(a.ed.buf)
	}
	var lines []string // newest first
	for _, l := range slices.Backward(m.past[a.kind]) {
		if strings.HasPrefix(l, a.draft) && l != a.draft {
			lines = append(lines, l)
		}
	}
	at := a.recalled - 1
	if older {
		at = a.recalled + 1
	}
	if at < 0 || at > len(lines) {
		return
	}
	a.recalled = at
	line := a.draft
	if at > 0 {
		line = lines[at-1]
	}
	a.ed = editor{buf: []rune(line), caret: len([]rune(line))}
}

// walkCommands completes the name on the command line: Tab and its aliases
// write the next command the typed letters could become, the others the one
// before, and past either end the line as it was typed. A name only one
// command answers is finished at once. It reports false once the name is
// done, where the same keys recall and edit as on any line.
func (m *Model) walkCommands(a *prompt, k string) bool {
	by := 0
	switch k {
	case "tab", "down", "ctrl+n", "ctrl+j":
		by = 1
	case "shift+tab", "up", "ctrl+p", "ctrl+k":
		by = -1
	default:
		a.menu = nil
		return false
	}
	typed := string(a.ed.buf)
	if a.choice > 0 {
		typed = a.draft
	}
	t := strings.TrimLeft(typed, " ")
	if len(t) > len(commandName(t)) {
		a.menu = nil
		return false
	}
	uses, _ := commandMatches(typed)
	n := len(uses)
	if n == 0 {
		return true
	}
	line := commandFill(uses[0])
	if n > 1 {
		if a.choice == 0 {
			a.draft = typed
		}
		a.menu, a.menuAt = uses, len(typed)-len(t)
		a.choice = (a.choice + by + n + 1) % (n + 1)
		line = a.draft
		if a.choice > 0 {
			line = commandFill(uses[a.choice-1])
		}
	}
	a.ed = editor{buf: []rune(line), caret: len([]rune(line))}
	return true
}

// commandFill is what completing a command writes: its name, and the space
// or slash that what it takes is typed after.
func commandFill(use string) string {
	name := commandName(use)
	if len(use) > len(name) {
		if use[len(name)] == '/' {
			return name + "/"
		}
		return name + " "
	}
	return name
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
			{Kind: outline.Insert, Row: id, Parent: filtersID, Pos: m.tree.PosLast(filtersID, false)},
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
	if m.at.query != "" {
		m.cursor = m.firstMatch()
	}
	m.seek(m.last[placeKey(m.at)])
	m.land()
}

// picksLeft is Today's sidebar count: its picks not yet closed.
func (m *Model) picksLeft() int {
	for _, e := range m.side {
		if e.kind == kindToday {
			return e.count
		}
	}
	return 0
}

// sayDayDone marks the end of a day's work: when a change the user made
// closes the last open pick, the status line says so, once. An undo, a sync
// pass or the day turning over says nothing, and nor does a key that has
// something of its own to say.
func (m *Model) sayDayDone(left int) {
	if !m.recorded || left == 0 || m.picksLeft() > 0 || m.msg != "" {
		return
	}
	if len(m.lists[kindToday]) > 0 {
		m.msg = "today is done: every pick is closed"
	}
}

// Notice shows a message on the status line until the next key.
func (m *Model) Notice(msg string) { m.msg = msg }

// rebuild recomputes the visible rows and the sidebar, and keeps the cursor
// inside the rows.
func (m *Model) rebuild() {
	cur := m.current()
	m.scan()
	if m.at.list == kindToday {
		m.sortToday(m.at.rows, m.at.carried)
	}
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
	m.paneTotal = taskTotals{}
	if m.at.query != "" {
		// A filter shows every row it found but those under one of its own
		// folds. It counts every match, folded or not, as an outline does.
		m.hides = map[string]int{}
		fold, under := "", -1 // the fold being skipped and its depth
		for _, v := range m.at.hits {
			if !m.live(v.id) {
				continue
			}
			if m.at.matched[v.id] {
				m.paneTotal.add(m.totals[v.id].own)
			}
			if under >= 0 && v.depth > under {
				if v.depth == under+1 {
					m.hides[fold]++
				}
				continue
			}
			fold, under = "", -1
			m.vis = append(m.vis, v)
			if m.isFolded(v.id) {
				fold, under = v.id, v.depth
			}
		}
	} else if m.at.list != "" {
		for _, id := range m.at.rows {
			if m.live(id) {
				m.vis = append(m.vis, visRow{id, 0})
				m.paneTotal.add(m.totals[id].own)
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
		m.paneTotal = m.totals[m.at.root].under
	}
	m.cursor = max(0, min(m.cursor, len(m.vis)-1))
	for id := range m.picked {
		if !m.live(id) {
			delete(m.picked, id)
			delete(m.sweepBase, id)
		}
	}
	if m.at.list == kindToday {
		m.seek(cur)
	}
}

// flat refuses an act that needs the whole tree in view: while a list or a
// filter is showing, a row's siblings are not all on screen.
func (m *Model) flat() bool {
	switch {
	case m.at.query != "":
		m.msg = "clear the filter first (Esc)"
	case m.at.list != "":
		m.msg = "a list has no order: h shows the row where it lives, Enter works inside it"
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

// land asks for the cursor's row to be shown mid-pane when the pane was not
// showing it, so a row reached from afar is read with what is around it.
func (m *Model) land() { m.centre = true }

// firstMatch is where the first row a filter found is drawn, since in
// context the rows above it are only there to place it. With no filter it
// is the first row.
func (m *Model) firstMatch() int {
	for i, v := range m.at.hits {
		if m.at.matched[v.id] {
			return i
		}
	}
	return 0
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

// foldOutline changes only this pane's subtree, saving all local view state
// together. Levels start at one for the rows immediately below the root.
func (m *Model) foldOutline(fold bool, args []string) {
	level := 0
	for i, word := range args {
		n, err := strconv.Atoi(word)
		if err != nil || n <= 0 || i > 0 || !fold {
			m.msg = "invalid fold level: " + word + ": fold 2 shows two levels"
			return
		}
		level = n
	}
	if m.noFolds() {
		return
	}
	changes := map[string]bool{}
	var walk func(string, int)
	walk = func(parent string, depth int) {
		for _, id := range m.tree.Children(parent) {
			if hiddenTop(id) {
				continue
			}
			if len(m.tree.Children(id)) > 0 {
				if level == 0 || depth <= level {
					want := fold && (level == 0 || depth == level)
					if m.isFolded(id) != want {
						changes[id] = want
					}
				}
				walk(id, depth+1)
			}
		}
	}
	walk(m.at.root, 1)
	m.applyFolds(changes)
}

// foldBranches folds or unfolds rows all the way down: each row given and
// every row under it that has rows of its own.
func (m *Model) foldBranches(ids []string, fold bool) {
	if m.noFolds() {
		return
	}
	changes := map[string]bool{}
	parents := 0
	var walk func(id string)
	walk = func(id string) {
		kids := m.tree.Children(id)
		if len(kids) == 0 {
			return
		}
		parents++
		if m.isFolded(id) != fold {
			changes[id] = fold
		}
		for _, kid := range kids {
			walk(kid)
		}
	}
	for _, id := range ids {
		walk(id)
	}
	if parents == 0 {
		m.msg = "no rows under this one"
	}
	m.applyFolds(changes)
}

// noFolds refuses a fold where rows do not stand in their tree: a list or a
// flat filter.
func (m *Model) noFolds() bool {
	switch {
	case m.at.list != "":
		m.msg = "nothing to fold in a list"
	case m.at.query != "" && m.at.flat:
		m.msg = "nothing to fold in a flat filter"
	default:
		return false
	}
	return true
}

// applyFolds saves a set of fold changes together and shows them, leaving
// the cursor on its row or on the nearest row above it still shown. In a
// filter the folds are the filter's own, held with it and never saved.
func (m *Model) applyFolds(changes map[string]bool) {
	if len(changes) == 0 {
		return
	}
	folds := m.folded
	if m.at.query != "" {
		if m.at.folds == nil {
			m.at.folds = map[string]bool{}
		}
		m.at.foldsFor, folds = m.at.query, m.at.folds
	} else if err := m.backend.SetFolds(changes); err != nil {
		m.msg = "not saved, nothing changed: " + err.Error()
		return
	}
	cur := m.current()
	for id, folded := range changes {
		if folded {
			folds[id] = true
		} else {
			delete(folds, id)
		}
	}
	m.rebuild()
	visible := make(map[string]int, len(m.vis))
	for i, row := range m.vis {
		visible[row.id] = i
	}
	for cur != "" && cur != m.at.root {
		if i, ok := visible[cur]; ok {
			m.cursor = i
			break
		}
		cur = m.tree.Row(cur).Parent
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
	m.recorded = true
	m.noteChange(ops)
	if m.undoGroup != nil {
		m.undoGroup = append(inv, m.undoGroup...)
		m.redo = nil
		return true
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
func (m *Model) replay(from, to *[][]outline.Op, verb string) {
	if len(*from) == 0 {
		m.msg = "nothing to " + verb
		return
	}
	cur := m.current()
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
	m.msg = verb + ": " + describe(group, m.under)
	if m.at.list == kindToday {
		for _, op := range group {
			if op.Field == fieldTodayPos {
				m.seek(cur)
				break
			}
		}
	}
}

func (m *Model) indentOp(id string) (outline.Op, bool) {
	prev := m.prevSibling(id)
	if prev == "" {
		m.msg = "no row above to indent under"
		return outline.Op{}, false
	}
	return outline.Op{Kind: outline.Move, Row: id, Parent: prev, Pos: m.tree.PosLast(prev, m.pinned(id))}, true
}

func (m *Model) outdentOp(id string) (outline.Op, bool) {
	r := m.tree.Row(id)
	if r.Parent == m.at.root {
		m.msg = "already at the top level"
		return outline.Op{}, false
	}
	p := m.tree.Row(r.Parent)
	return outline.Op{Kind: outline.Move, Row: id, Parent: p.Parent, Pos: m.tree.PosAfter(p.Parent, p.ID, m.pinned(id))}, true
}

func (m *Model) pinned(id string) bool { return m.tree.Row(id).Pinned() }

// placeOps moves rows under parent to land directly after the sibling named,
// or first when it is "", keeping their order. Each keeps its pin, so it
// lands as near there as its group allows; the rows themselves are passed
// over as neighbours.
func (m *Model) placeOps(rows []string, parent, after string) []outline.Op {
	moving := make(map[string]bool, len(rows))
	for _, id := range rows {
		moving[id] = true
	}
	gaps := map[bool]*[2]string{}
	var ops []outline.Op
	for _, id := range rows {
		g := m.pinned(id)
		gap := gaps[g]
		if gap == nil {
			left, right := m.tree.Gap(parent, after, g, moving)
			gap = &[2]string{left, right}
			gaps[g] = gap
		}
		gap[0] = outline.Between(gap[0], gap[1])
		ops = append(ops, outline.Op{Kind: outline.Move, Row: id, Parent: parent, Pos: gap[0]})
	}
	return ops
}

// appendOps moves rows to be the last children of parent, in order, each
// after the last of its own group.
func (m *Model) appendOps(rows []string, parent string) []outline.Op {
	last := map[bool]string{}
	var ops []outline.Op
	for _, id := range rows {
		g := m.pinned(id)
		pos, ok := last[g]
		if ok {
			pos = outline.Between(pos, "")
		} else {
			pos = m.tree.PosLast(parent, g)
		}
		last[g] = pos
		ops = append(ops, outline.Op{Kind: outline.Move, Row: id, Parent: parent, Pos: pos})
	}
	return ops
}

// pinWall says why a row cannot be reordered past next: pinned rows stay
// above the rest. It is empty when the two are in one group.
func (m *Model) pinWall(rows []string, next string) string {
	for _, id := range rows {
		if m.pinned(id) != m.pinned(next) {
			if m.pinned(id) {
				return "pinned rows stay above the rest: ^ unpins it"
			}
			return "pinned rows stay above the rest: ^ pins it"
		}
	}
	return ""
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
	var ops []outline.Op
	switch key {
	case ">", "<":
		op, ok := m.outdentOp(first)
		if key == ">" {
			op, ok = m.indentOp(first)
		}
		if !ok {
			return
		}
		parent = op.Parent
		if key == ">" {
			ops = m.appendOps(block, parent)
		} else {
			ops = m.placeOps(block, parent, m.tree.Row(first).Parent)
		}
	case "J":
		next := m.tree.NextSibling(last)
		if next == "" {
			return
		}
		if wall := m.pinWall(block, next); wall != "" {
			m.msg = wall
			return
		}
		ops = m.placeOps(block, parent, next)
	case "K":
		prev := m.prevSibling(first)
		if prev == "" {
			return
		}
		if wall := m.pinWall(block, prev); wall != "" {
			m.msg = wall
			return
		}
		ops = m.placeOps(block, parent, m.tree.PrevSibling(prev))
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

// sortRows reorders siblings once, leaving their descendants attached.
func (m *Model) sortRows(keys []string) {
	for _, key := range keys {
		if key != "@" && key != "!" && key != "$" {
			m.msg = "unknown sort key: " + key + ": the keys are @ ! $, or none for text"
			return
		}
	}
	if m.flat() {
		return
	}
	cur := m.current()
	var block []string
	if len(m.picked) > 0 {
		block = m.selectedBlock(m.acting())
		if len(block) == 0 {
			return
		}
	} else if cur != "" {
		for _, id := range m.tree.Children(m.tree.Row(cur).Parent) {
			if !hiddenTop(id) {
				block = append(block, id)
			}
		}
	}
	if len(block) == 0 {
		m.msg = "nothing to sort"
		return
	}
	states := []string{sigil.Doing, sigil.Open, sigil.Waiting, "", sigil.Parked, sigil.Done, sigil.Canceled}
	ordered := slices.Clone(block)
	slices.SortStableFunc(ordered, func(a, b string) int {
		x, y := m.tree.Row(a).Fields, m.tree.Row(b).Fields
		// Each group is sorted on its own: pinned rows stay first.
		if xp, yp := m.pinned(a), m.pinned(b); xp != yp {
			if xp {
				return -1
			}
			return 1
		}
		if len(keys) == 0 {
			return strings.Compare(strings.ToLower(x[sigil.FieldText]), strings.ToLower(y[sigil.FieldText]))
		}
		for _, key := range keys {
			var order int
			switch key {
			case "@":
				xd, yd := x[sigil.FieldDue], y[sigil.FieldDue]
				switch {
				case xd == yd:
				case xd == "":
					order = 1
				case yd == "":
					order = -1
				default:
					order = strings.Compare(xd, yd)
				}
			case "!":
				xf, yf := x[sigil.FieldFlag] != "", y[sigil.FieldFlag] != ""
				if xf != yf {
					order = 1
					if xf {
						order = -1
					}
				}
			case "$":
				order = cmp.Compare(slices.Index(states, x[sigil.FieldState]), slices.Index(states, y[sigil.FieldState]))
			}
			if order != 0 {
				return order
			}
		}
		return 0
	})
	if slices.Equal(block, ordered) {
		m.msg = "already in order"
		return
	}
	parent := m.tree.Row(block[0]).Parent
	ops := m.placeOps(ordered, parent, m.tree.PrevSibling(block[0]))
	if m.do(ops...) {
		m.rebuild()
		m.seek(cur)
		m.msg = "sorted " + rowCount(len(block))
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
			m.msg = "already the last of its siblings: >> or m moves it elsewhere"
			return
		}
		if wall := m.pinWall([]string{id}, next); wall != "" {
			m.msg = wall
			return
		}
		pos = m.tree.PosAfter(r.Parent, next, r.Pinned())
	} else {
		prev := m.prevSibling(id)
		if prev == "" {
			m.msg = "already the first of its siblings: << or m moves it elsewhere"
			return
		}
		if wall := m.pinWall([]string{id}, prev); wall != "" {
			m.msg = wall
			return
		}
		pos = m.tree.PosBefore(r.Parent, prev, r.Pinned())
	}
	if m.do(outline.Op{Kind: outline.Move, Row: id, Parent: r.Parent, Pos: pos}) {
		m.rebuild()
		m.seek(id)
	}
}

// newRow opens a blank row for typing: above the cursor, as the first child
// of an open parent, or else directly below.
func (m *Model) newRow(above bool) {
	m.newRowAt(above, m.current())
}

// openSpot is where o opens a row below cur: as the first child of an open
// parent, else directly after it, and with no row last in the place. The row
// goes under parent, after the sibling named, or first when it is "".
func (m *Model) openSpot(cur string) (parent, after string) {
	switch {
	case cur == "":
		kids := m.tree.Children(m.at.root)
		if len(kids) == 0 {
			return m.at.root, ""
		}
		return m.at.root, kids[len(kids)-1]
	case len(m.tree.Children(cur)) > 0 && !m.folded[cur]:
		return cur, ""
	default:
		return m.tree.Row(cur).Parent, cur
	}
}

// moveHere moves the selected roots to where o would open a row, in screen
// order, as one undo step. The cursor stays on its row.
func (m *Model) moveHere(ids []string) {
	cur := m.current()
	roots := m.selectedRoots(ids)
	moving := make(map[string]bool, len(roots))
	for _, id := range roots {
		moving[id] = true
	}
	parent, after := m.openSpot(cur)
	var live []string
	for _, id := range roots {
		if m.live(id) {
			live = append(live, id)
		}
	}
	ops := m.placeOps(live, parent, after)
	if len(ops) > 0 && m.do(ops...) {
		m.rebuild()
		m.seek(cur)
		m.msg = "moved here"
	}
}

func (m *Model) newRowAt(above bool, cur string) {
	if m.at.list != "" && m.at.query == "" {
		m.msg = "add rows inside a project, or press a for the Inbox"
		return
	}
	if m.flat() {
		return
	}
	// The new row takes the pin of the sibling it opens beside, so it is
	// drawn where it was opened.
	var parent, pos string
	var pin bool
	if above && cur != "" {
		parent = m.tree.Row(cur).Parent
		pin = m.pinned(cur)
		pos = m.tree.PosBefore(parent, cur, pin)
	} else {
		var after string
		parent, after = m.openSpot(cur)
		if after != "" {
			pin = m.pinned(after)
		} else if kids := m.tree.Children(parent); len(kids) > 0 {
			pin = m.pinned(kids[0])
		}
		pos = m.tree.PosAfter(parent, after, pin)
	}
	ops := []outline.Op{{Kind: outline.Insert, Row: m.newID(), Parent: parent, Pos: pos}}
	if pin {
		ops = append(ops, outline.Op{Kind: outline.Set, Row: ops[0].Row, Field: sigil.FieldPin, Value: "1"})
	}
	op := ops[0]
	inv, err := m.apply(ops)
	if err != nil {
		m.msg = err.Error()
		return
	}
	m.rebuild()
	m.seek(op.Row)
	m.edit = &editor{row: op.Row, fresh: true, back: cur, ops: ops, inv: inv}
	m.edit.mark("o")
	if above {
		m.edit.mark("O")
	}
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

func (m *Model) retypeRow(id string) {
	m.editRow(id)
	line := sigil.FromFields(m.tree.Row(id).Fields)
	line.Text = ""
	text := sigil.Format(line, m.edit.notes, m.today())
	// Spell even an implicit state: the replacement text may gain or lose
	// a trailing colon, but cc keeps the row's fields in either direction.
	if line.State == sigil.DefaultState(m.edit.notes, "") {
		state := line.State
		if state == "" {
			state = "note"
		}
		text = strings.TrimSpace(text + " $" + state)
	}
	m.edit.buf, m.edit.caret = []rune(" "+text), 0
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
	for _, name := range []string{sigil.FieldText, fieldBody, sigil.FieldState, sigil.FieldFlag, sigil.FieldPin, sigil.FieldDue, sigil.FieldStart, sigil.FieldRepeat, sigil.FieldTags, sigil.FieldContext, sigil.FieldColor} {
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
			// Reset the checklist at every depth in this same undo step,
			// preserving each row's dates and every state other than done.
			changed := strconv.FormatInt(m.now(), 10)
			var reopen func(string)
			reopen = func(parent string) {
				for _, child := range m.tree.Children(parent) {
					if m.tree.Row(child).Fields[sigil.FieldState] == sigil.Done {
						ops = append(ops,
							outline.Op{Kind: outline.Set, Row: child, Field: sigil.FieldState, Value: sigil.Open},
							outline.Op{Kind: outline.Set, Row: child, Field: sigil.FieldChanged, Value: changed})
					}
					reopen(child)
				}
			}
			reopen(id)
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
	if e.body {
		return m.finishBody(e)
	}
	if e.side {
		// A project typed out of the sidebar's list, by $parked or $done,
		// gives the selection to the entry now in its place.
		at := m.sideAt()
		defer func() {
			msg := m.msg
			m.sideKeep(at)
			m.msg = msg
		}()
	}
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
	want := sigil.Parse(typed, notes, m.today()).Fields()
	if e.fresh && m.pinned(e.row) {
		// A fresh row was pinned by where it was opened, not by its line.
		want[sigil.FieldPin] = "1"
	}
	ops := m.setFields(e.row, want)
	if inv, err := m.apply(ops); err == nil {
		e.ops = append(e.ops, ops...)
		e.inv = append(inv, e.inv...)
	}
	if len(e.ops) > 0 {
		var saved bool
		if e.merge {
			saved = m.save(e.ops, e.inv)
			if saved {
				m.noteChange(e.ops)
				last := len(m.undo) - 1
				m.undo[last] = append(e.inv, m.undo[last]...)
				m.keepHistory()
			}
		} else {
			saved = m.record(e.ops, e.inv)
		}
		if !saved {
			if e.fresh {
				m.tree.Forget(e.row)
			}
			m.rebuild()
			m.seek(e.back)
			return false
		}
		e.merge = true
	}
	switch typed, only := e.inserted(); {
	case !only || m.replaying:
	case e.how == "text":
		// Typing opened inside the row completes the change made there.
		m.textRepeat = textAct{keys: e.cmd, typed: typed}
	case strings.TrimSpace(typed) != "":
		m.repeat = repeatAct{how: e.how, typed: typed}
	}
	m.rebuild()
	m.seek(e.row)
	return true
}

// retype types the remembered text again: into a new row for o and O, else
// into each row given where the key would have put the caret. It is one
// undo step.
func (m *Model) retype(act repeatAct, rows []string) {
	m.replaying = true
	defer func() { m.replaying = false }()
	if act.how == "o" || act.how == "O" {
		if !m.single() {
			return
		}
		if m.newRowAt(act.how == "O", m.current()); m.edit != nil {
			m.edit.insert([]rune(act.typed))
			m.finishEdit()
		}
		return
	}
	cur := m.current()
	m.undoGroup = []outline.Op{}
	for _, id := range rows {
		if !m.live(id) {
			continue
		}
		if act.how == "cc" {
			m.retypeRow(id)
		} else if m.editRow(id); act.how == "I" {
			m.edit.caret = 0
		}
		m.edit.insert([]rune(act.typed))
		if !m.finishEdit() {
			break
		}
	}
	if len(m.undoGroup) > 0 {
		m.undo = append(m.undo, m.undoGroup)
		m.keepHistory()
	}
	m.undoGroup = nil
	m.seek(cur)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.lightPalette = !msg.IsDark()
		return m, nil
	case tea.FocusMsg:
		// The terminal may have changed its colours while away, as Docket.app
		// does when macOS turns dark or light.
		return m, tea.RequestBackgroundColor
	case tickMsg:
		m.backup()
		m.refresh()
		m.syncSettle()
		m.syncStart(false)
		m.rebuild()
		return m, tea.Batch(tick(), m.syncCmd(), tea.RequestBackgroundColor)
	case flashMsg:
		if msg.gen == m.flashGen {
			m.unflash()
		}
		return m, nil
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
		outlineKey := !m.help && m.edit == nil && m.ask == nil && !m.sideFocus
		tree, gen := m.tree, m.tree.Gen()
		left := m.picksLeft()
		m.recorded = false
		switch {
		case m.help && m.helpFind != nil:
			m.helpFindKey(msg)
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
		// Outline navigation and pending grammar use the existing visible rows.
		// Place and fold changes rebuild explicitly in their handlers.
		if !outlineKey || m.tree != tree || m.tree.Gen() != gen {
			m.rebuild()
		} else {
			// The sidebar still follows a minute turning over between keys.
			m.scan()
		}
		m.sayDayDone(left)
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
	// A recalling prompt: how far back among its kept lines it stands, zero
	// being the line as typed, which draft holds meanwhile.
	recalled int
	draft    string
	// A walk through completions: what Tab steps through, listed above the
	// line while the walk lasts, and the rune of the line they are written at.
	menu   []string
	menuAt int
}

const (
	askCapture = "capture" // a line for the Inbox
	askJump    = "jump"    // a project or list to go to
	askMove    = "move"    // a project, Inbox or heading to move rows into
	askLink    = "link"    // a row for rows to point at
	askFilter  = "filter"  // a query narrowing the pane, live
	askStamp   = "stamp"   // sigils to set on rows
	askCommand = "command"
)

var askLabels = map[string]string{
	askCapture: "Inbox › ",
	askJump:    "Go to › ",
	askMove:    "Move to › ",
	askLink:    "Link to › ",
	askFilter:  "Filter › ",
	askStamp:   "Stamp › ",
	askCommand: ": ",
}

func (m *Model) askKey(msg tea.KeyPressMsg) {
	a := m.ask
	k := msg.String()
	if m.walkValues(&a.ed, a.kind, k) {
		a.recalled = 0
		return
	}
	if m.complete(&a.ed, a.kind, k) {
		k = "" // the line changed; a filter reads it again below
	}
	if a.kind == askCommand && m.walkCommands(a, k) {
		return
	}
	if a.kind == askFilter && m.completeFilter(a, k) {
		k = "" // the line changed; the filter reads it again below
	}
	text := strings.TrimSpace(string(a.ed.buf))

	if a.kind == askFilter {
		// The pane follows the query as it is typed. Enter keeps it and
		// returns to the rows; Esc drops it.
		switch k {
		case "enter":
			m.ask = nil
			if text != "" {
				m.find, m.findHere = text, m.at.here
				m.remember(askFilter, text)
			}
		case "esc":
			m.ask = nil
			m.at.query = ""
		case "tab":
			m.at.flat = !m.at.flat
		case "":
			m.at.query = text
		case "up", "ctrl+p", "down", "ctrl+n":
			m.recall(a, k == "up" || k == "ctrl+p")
			m.at.query = strings.TrimSpace(string(a.ed.buf))
		default:
			a.ed.key(msg)
			a.recalled = 0
			m.at.query = strings.TrimSpace(string(a.ed.buf))
		}
		m.refilter()
		// The cursor goes to what was found, not to a row kept above it.
		m.cursor, m.top = m.firstMatch(), 0
		// A selection made before the rows changed could act on rows no
		// longer on screen.
		m.clearSelection()
		if k == "enter" && len(m.at.matched) > 0 {
			// The count leaves with the line, so keeping it says what is
			// held and the two keys that are new while it is.
			m.msg = matchCount(len(m.at.matched)) + " kept · Esc drops the filter"
			if len(m.at.matched) > 1 {
				m.msg = matchCount(len(m.at.matched)) + " kept · n N step through them · Esc drops the filter"
			}
		}
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
			if m.applyStamp(a.rows, string(a.ed.buf)) {
				m.ask = nil
			}
		case askCommand:
			m.ask = nil
			m.remember(askCommand, text)
			m.command(text)
		default:
			hits := m.offers(a, text)
			if len(hits) == 0 {
				m.msg = "nothing matches " + text
				return
			}
			hit := hits[min(a.choice, len(hits)-1)]
			m.ask = nil
			if a.kind == askJump {
				m.jump(hit)
				return
			}
			switch {
			case a.kind == askLink:
				m.linkTo(a.rows, hit)
			case hit.kind == kindHere:
				m.moveHere(a.rows)
			default:
				m.moveTo(a.rows, hit)
			}
		}
	case "down", "ctrl+n", "up", "ctrl+p":
		// A picker's arrows walk its matches.
		if picker(a.kind) {
			n := len(m.offers(a, text))
			if k == "down" || k == "ctrl+n" {
				a.choice++
			} else {
				a.choice--
			}
			a.choice = max(0, min(a.choice, n-1))
		} else if recalls(a.kind) {
			m.recall(a, k == "up" || k == "ctrl+p")
		}
	default:
		a.ed.key(msg)
		a.choice, a.recalled = 0, 0
	}
}

// applyStamp reports whether the line was readable, so an unreadable prompt
// stays open. Only a successfully applied change replaces the remembered act.
func (m *Model) applyStamp(rows []string, line string) bool {
	c, bad := parseStamp(line, m.today())
	if bad != "" {
		m.msg = unreadStamp(bad)
		return false
	}
	var ops []outline.Op
	for _, id := range rows {
		if m.live(id) {
			ops = append(ops, m.stampOps(id, c)...)
		}
	}
	cur := m.current()
	if len(ops) > 0 && m.do(ops...) {
		// A pin moves the row, and the cursor goes with it.
		m.rebuild()
		m.seek(cur)
		m.repeat = repeatAct{line: line}
		m.remember(askStamp, line)
	} else if len(ops) == 0 && len(c.shift) > 0 && len(c.set)+len(c.add)+len(c.strip) == 0 {
		m.msg = "no day to move: ++ and -- count from the day a row already has"
	}
	return true
}

func (m *Model) moveTo(rows []string, to entry) {
	dest := to.rowID()
	if !m.live(dest) {
		m.msg = "destination is no longer live: " + to.label
		return
	}
	var live []string
	for _, id := range rows {
		if m.live(id) {
			live = append(live, id)
		}
	}
	// Each row is placed after the one before it.
	ops := m.appendOps(live, dest)
	if len(ops) > 0 && m.do(ops...) {
		m.msg = "moved to " + to.label
		m.repeat = repeatAct{to: to}
	}
}

// command runs a line typed after a colon.
func (m *Model) command(line string) {
	words := strings.Fields(line)
	if len(words) == 0 {
		return
	}
	if old, new, ok := readSwap(line); ok {
		if old == "" {
			m.msg = "write it as s/old/new: every old in the rows' text becomes new"
			return
		}
		m.swapText(m.acting(), old, new)
		return
	}
	name := commandName(words[0])
	words[0] = name + words[0][len(name):]
	if uses, _ := commandMatches(line); len(words) == 1 && len(uses) == 1 {
		// A name the line has narrowed to one command is that command.
		words[0] = commandName(uses[0])
	}
	switch words[0] {
	case "help", "h":
		m.openHelp()
	case "tutor":
		m.tutor()
	case "q", "quit":
		m.quit = true
	case "fold", "unfold":
		m.foldOutline(words[0] == "fold", words[1:])
	case "sort":
		m.sortRows(words[1:])
	case "export":
		m.export(strings.Join(words[1:], " "))
	case "import":
		m.importFile(strings.Join(words[1:], " "))
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
	case "all":
		// The pane's rows become the selection: v picks one, this the pane.
		// A filter in context dims ancestors it did not match; they stay out.
		picked := map[string]bool{}
		for _, v := range m.vis {
			if m.at.query == "" || m.at.matched[v.id] {
				picked[v.id] = true
			}
		}
		if len(picked) == 0 {
			m.msg = "nothing to select"
			return
		}
		m.picked = picked
	case "review":
		m.open(entry{kind: kindReview})
	case "archive":
		m.archive()
	case "save", "forget":
		m.keepFilter(words[0] == "save", strings.Join(words[1:], " "))
	case "numbers":
		// Each row says how many rows it is from the cursor, or stops.
		m.numbers = !m.numbers
		value, said := "", "row numbers hidden"
		if m.numbers {
			value, said = "1", "row numbers shown: each is a count from the cursor"
		}
		if err := m.backend.SetMeta(metaNumbers, value); err != nil {
			said = "not saved: " + err.Error()
		}
		m.msg = said
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
		if hint, ok := strayCommands[words[0]]; ok && len(words) == 1 {
			m.msg = hint
			return
		}
		var uses []string
		for _, c := range commandUses {
			uses = append(uses, c.use)
		}
		m.msg = "no command " + words[0] + ": " + strings.Join(uses, ", ")
	}
}

// readSwap reads s/old/new, where the mark after the s may be any character
// that is not a letter, a digit or a blank. A last mark may close the line,
// which is how new ends in a blank. old is empty when the line is not whole.
func readSwap(line string) (old, new string, ok bool) {
	r := []rune(line)
	if len(r) < 2 || r[0] != 's' || r[1] == ' ' || r[1] == '_' || wordClass(r[1], false) == 1 {
		return "", "", false
	}
	parts := strings.Split(string(r[2:]), string(r[1]))
	if len(parts) == 3 && parts[2] == "" {
		parts = parts[:2]
	}
	if len(parts) != 2 {
		return "", "", true
	}
	return parts[0], parts[1], true
}

// swapText puts new wherever old stands in the text of the rows given, as
// one undo step. Fields are left alone: a stamp changes those.
func (m *Model) swapText(rows []string, old, new string) {
	var ops []outline.Op
	for _, id := range rows {
		if !m.live(id) {
			continue
		}
		if text := m.tree.Row(id).Text(); strings.Contains(text, old) {
			ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: sigil.FieldText, Value: strings.ReplaceAll(text, old, new)})
		}
	}
	if len(ops) == 0 {
		m.msg = "no row here holds " + old
		return
	}
	if m.do(ops...) {
		m.repeat = repeatAct{old: old, new: new}
		m.msg = "changed " + rowCount(len(ops))
		if len(ops) == 1 {
			m.msg = "changed 1 row"
		}
	}
}

const archiveText = "Archive:"

// archive moves the closed work under the pane's root to the end of its
// project, under a heading named Archive, as one undo step. A closed row
// goes with everything under it, and stays where it is while anything under
// it is still to do.
func (m *Model) archive() {
	if m.at.list != "" || m.at.query != "" {
		m.msg = "archive works in an outline: open a project first"
		return
	}
	if m.at.root == inboxID {
		m.msg = "the Inbox has no archive: move the row to a project first (m)"
		return
	}
	var todo func(id string) bool
	todo = func(id string) bool {
		if l := sigil.FromFields(m.tree.Row(id).Fields); l.State != "" && !l.Closed() && l.State != sigil.Parked {
			return true
		}
		return slices.ContainsFunc(m.tree.Children(id), todo)
	}
	var ops []outline.Op
	moved := 0
	sweep := func(project, root string) {
		shelf := ""
		for _, id := range m.tree.Children(project) {
			if r := m.tree.Row(id); !r.Deleted() && r.Text() == archiveText {
				shelf = id
			}
		}
		var found []string
		var walk func(parent string)
		walk = func(parent string) {
			for _, id := range m.tree.Children(parent) {
				r := m.tree.Row(id)
				switch {
				case r.Deleted() || id == shelf:
				case sigil.FromFields(r.Fields).Closed() && !todo(id):
					found = append(found, id)
				default:
					walk(id)
				}
			}
		}
		// Zoomed inside the archive there is nothing left to shelve.
		if shelf == "" || (root != shelf && !m.tree.Contains(shelf, root)) {
			walk(root)
		}
		if len(found) == 0 {
			return
		}
		if shelf == "" {
			shelf = m.newID()
			last := m.tree.PosLast(project, false)
			if n := len(ops); n > 0 && ops[n-1].Parent == project {
				last = outline.Between(ops[n-1].Pos, "")
			}
			ops = append(ops,
				outline.Op{Kind: outline.Insert, Row: shelf, Parent: project, Pos: last},
				outline.Op{Kind: outline.Set, Row: shelf, Field: sigil.FieldText, Value: archiveText})
		}
		// A shelf made just now has no rows yet, so each group starts afresh.
		ops = append(ops, m.appendOps(found, shelf)...)
		moved += len(found)
	}
	if m.at.root == "" {
		for _, top := range m.tree.Children("") {
			if !hiddenTop(top) && !m.tree.Row(top).Deleted() {
				sweep(top, top)
			}
		}
	} else if p := m.projectOf(m.at.root); p != nil && p.ID != inboxID {
		sweep(p.ID, m.at.root)
	} else {
		m.msg = "the Inbox has no archive: move the row to a project first (m)"
		return
	}
	if moved == 0 {
		m.msg = "nothing closed to archive"
		return
	}
	cur := m.current()
	if m.do(ops...) {
		m.vis, m.at.kept = nil, nil
		m.rebuild()
		m.seek(cur)
		m.msg = "archived " + rowCount(moved) + " under " + archiveText
		if moved == 1 {
			m.msg = "archived 1 row under " + archiveText
		}
	}
}

// sideKey handles the sidebar while it has focus. Moving shows each
// destination at once; prompts can capture, jump or run a command.
func (m *Model) sideKey(k string) {
	g, d, c := m.prefix == "g", m.prefix == "d", m.prefix == "c"
	m.prefix = ""
	_, height := m.size()
	page := max(height-2, 1) // exclude the header and status line
	if c {
		m.sideChange(k)
		return
	}
	switch k {
	case "A", "I", "w", "b", "e", "W", "B", "E":
		m.sideType(k)
	case "c":
		if m.sideTyped() {
			m.prefix = "c"
		}
	case "d":
		if d {
			m.sideDelete()
		} else {
			m.prefix = "d"
		}
	case "u":
		m.sideReplay(&m.undo, &m.redo, "undo")
	case "ctrl+r":
		m.sideReplay(&m.redo, &m.undo, "redo")
	case "j", "down":
		m.sideStep(1)
	case "k", "up":
		m.sideStep(-1)
	case "ctrl+d":
		m.sideStep(max(page/2, 1))
	case "ctrl+u":
		m.sideStep(-max(page/2, 1))
	case "ctrl+f", "pgdown":
		m.sideStep(page)
	case "ctrl+b", "pgup":
		m.sideStep(-page)
	case "G", "end":
		m.sideStep(len(m.side))
	case "home":
		m.sideStep(-len(m.side))
	case "g":
		if g {
			m.sideStep(-len(m.side))
		} else {
			m.prefix = "g"
		}
	case "tab", "enter", "esc", "l", "right":
		m.leaveSidebar()
	case ":":
		m.leaveSidebar()
		m.ask = &prompt{kind: askCommand}
	case "f":
		m.ask = &prompt{kind: askJump}
	case "a":
		m.ask = &prompt{kind: askCapture}
	}
}

func (m *Model) normalKey(k string) {
	if k == "esc" && (m.count > 0 || m.operator != "") {
		m.endSweep()
		m.count, m.prefix, m.operator, m.opCount = 0, "", "", 0
		return
	}
	if m.operator == "c" {
		m.operator, m.opCount, m.count = "", 0, 0
		if k == "c" {
			m.normalAct("cc", 0)
		}
		return
	}
	if m.prefix == "" && len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
		// Saturate rather than overflow; every consumer stops at its boundary.
		digit, limit := int(k[0]-'0'), int(^uint(0)>>1)
		if m.count > (limit-digit)/10 {
			m.count = limit
		} else {
			m.count = m.count*10 + digit
		}
		return
	}
	if m.operator != "" {
		m.operatorKey(k)
		return
	}
	if k == "esc" && m.sweep != "" {
		m.prefix = ""
	}
	if m.prefix != "" {
		k, m.prefix = m.prefix+k, ""
	} else if k == "d" || k == "y" || k == "c" || k == ">" || k == "<" {
		m.endSweep()
		m.operator, m.opCount, m.count = k, m.count, 0
		if k == "c" {
			m.opCount = 0
		}
		return
	} else if k == "g" || k == "z" {
		m.prefix = k
		return
	}
	if k == "V" {
		m.count = 0
		if m.sweep != "" {
			m.endSweep()
		} else {
			m.startSweep()
		}
		return
	}
	if (k == "k" || k == "up") && m.cursor == 0 && m.count == 0 && m.sweep == "" && m.bodyRow() != "" {
		// Above the first row is the body of the row the pane opens.
		if len(m.picked) > 0 {
			m.msg = "a selection stays among the rows: Esc clears it, then k reaches the body"
			return
		}
		m.enterBody()
		return
	}
	if isMotion(k) {
		defer m.updateSweep()
	} else if k != "zz" && k != "zt" && k != "zb" {
		// Placing the row in the pane changes nothing a sweep reads.
		m.endSweep()
	}
	count := m.count
	m.count = 0
	m.act(k, count)
}

// act runs a completed key with its count. A key that changed rows is what
// the dot key repeats next.
func (m *Model) act(k string, count int) {
	tree, gen := m.tree, m.tree.Gen()
	defer func() {
		switch k {
		case "x", "!", "^", "t", "J", "K", "p", "P", "gp", "gP":
			if m.tree == tree && m.tree.Gen() != gen {
				m.repeat = repeatAct{key: k, count: count}
			}
		}
	}()
	if count > 0 {
		switch k {
		case "x", "!", "^", "t", "m", "L", ".", "#", "@", "$", "*", "-", "~", "v", "zc", "zo":
			if len(m.picked) > 0 {
				m.msg = "a count cannot act on a selection: act without one, or Esc clears the selection"
				return
			}
		}
		switch k {
		case "J", "K":
			m.undoGroup = []outline.Op{}
			for range count {
				before := len(m.undoGroup)
				m.normalAct(k, 0)
				if len(m.undoGroup) == before {
					break
				}
			}
			if len(m.undoGroup) > 0 {
				m.undo = append(m.undo, m.undoGroup)
				m.keepHistory()
			}
			m.undoGroup = nil
			return
		case "u", "ctrl+r":
			steps := 0
			for range count {
				before := len(m.undo)
				if k == "ctrl+r" {
					before = len(m.redo)
				}
				m.normalAct(k, 0)
				after := len(m.undo)
				if k == "ctrl+r" {
					after = len(m.redo)
				}
				if after == before {
					break
				}
				steps++
			}
			if steps > 1 {
				verb := "undo"
				if k == "ctrl+r" {
					verb = "redo"
				}
				m.msg = fmt.Sprintf("%s: %d steps", verb, steps)
			}
			return
		case "v":
			for range min(count, len(m.vis)-m.cursor) {
				m.normalAct(k, 0)
			}
			return
		}
	}
	m.normalAct(k, count)
}

// isMotion names the keys motionTarget reads.
func isMotion(k string) bool {
	switch k {
	case "j", "k", "down", "up", "gg", "G", "home", "end", "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b", "pgdown", "pgup",
		"}", "{", ")", "(", "H", "]", "[", "n", "N":
		return true
	}
	return false
}

// sibling is the shown row at i's depth next in that direction, before any
// shallower row; i itself when there is none.
func (m *Model) sibling(i, step int) int {
	for j := i + step; j >= 0 && j < len(m.vis) && m.vis[j].depth >= m.vis[i].depth; j += step {
		if m.vis[j].depth == m.vis[i].depth {
			return j
		}
	}
	return i
}

// finder reads a row against the filter on screen, else the one last kept
// or dropped, for n and N. It is nil when there is none.
func (m *Model) finder() func(id string) bool {
	if m.at.query != "" {
		return func(id string) bool { return m.at.matched[id] }
	}
	today := m.today()
	q := m.query(m.find, m.findHere, today)
	terms := q.terms
	if len(terms) == 0 {
		return nil
	}
	m.scan() // computed filter words read the scan's answers
	hasAbove := slices.ContainsFunc(terms, func(t term) bool { return t.above })
	above := make([]int, len(terms))
	under := m.underSets(terms, today)
	return func(id string) bool {
		r := m.tree.Row(id)
		clear(above)
		for p := r.Parent; hasAbove && p != ""; {
			pr := m.tree.Row(p)
			l := sigil.FromFields(pr.Fields)
			for i, t := range terms {
				if t.above && above[i] == 0 && m.holdsAny(pr, l, t, today) {
					above[i] = 1
				}
			}
			p = pr.Parent
		}
		return m.matches(r, q, today, above, under)
	}
}

// motionTarget reads only the visible rows; operators use it without moving.
func (m *Model) motionTarget(k string, count int) (int, bool) {
	target := m.cursor
	if len(m.vis) == 0 {
		return target, isMotion(k)
	}
	step := 1
	switch k {
	case "{", "(", "[", "N":
		step = -1
	}
	switch k {
	case "}", "{":
		// The next or previous sibling, passing over what is under each.
		for range max(count, 1) {
			next := m.sibling(target, step)
			if next == target {
				break
			}
			target = next
		}
	case ")", "(":
		for next := m.sibling(target, step); next != target; next = m.sibling(target, step) {
			target = next
		}
	case "H":
		// The parent, without folding anything; a count climbs that many.
		for range max(count, 1) {
			up := target - 1
			for up >= 0 && m.vis[up].depth >= m.vis[target].depth {
				up--
			}
			if up < 0 {
				break
			}
			target = up
		}
	case "]", "[", "n", "N":
		hit := func(id string) bool { return strings.HasSuffix(m.tree.Row(id).Text(), ":") }
		if k == "n" || k == "N" {
			if hit = m.finder(); hit == nil {
				m.msg = "nothing to find: keep a filter with / and Enter first"
				return target, true
			}
		}
		left := max(count, 1)
		for i := target + step; i >= 0 && i < len(m.vis) && left > 0; i += step {
			if hit(m.vis[i].id) {
				target, left = i, left-1
			}
		}
		if target == m.cursor && (k == "n" || k == "N") {
			m.msg = "no match further on"
		}
	case "gg", "home":
		target = 0
	case "G", "end":
		target = max(len(m.vis)-1, 0)
	case "j", "down":
		target += min(max(count, 1), max(len(m.vis)-1-target, 0))
	case "k", "up":
		target -= min(max(count, 1), target)
	case "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b", "pgdown", "pgup":
		_, height := m.size()
		step := max(height-2, 1)
		if k == "ctrl+d" || k == "ctrl+u" {
			step = max(step/2, 1)
		}
		step *= min(max(count, 1), len(m.vis)/step+1)
		if k == "ctrl+u" || k == "ctrl+b" || k == "pgup" {
			target -= min(step, target)
		} else {
			target += min(step, max(len(m.vis)-1-target, 0))
		}
	default:
		return target, false
	}
	return target, true
}

func (m *Model) operatorKey(k string) {
	if m.prefix != "" {
		k, m.prefix = m.prefix+k, ""
	} else if k == "g" || k == "i" {
		// g begins a motion, i a row object: ic and is.
		m.prefix = k
		return
	}
	op, count := m.operator, 0
	if m.opCount > 0 || m.count > 0 {
		a, b, limit := max(m.opCount, 1), max(m.count, 1), int(^uint(0)>>1)
		count = limit
		if a <= limit/b {
			count = a * b
		}
	}
	m.operator, m.opCount, m.count = "", 0, 0
	m.operate(op, k, count)
}

// operate completes an operator: doubled it takes rows, with a motion the
// rows from the cursor to where the motion goes. A change is what the dot
// key repeats next.
func (m *Model) operate(op, k string, count int) {
	tree, gen := m.tree, m.tree.Gen()
	if k == op {
		if count > 0 && len(m.picked) > 0 {
			m.msg = "a count cannot act on a selection: act without one, or Esc clears the selection"
			return
		}
		m.takeRows(op, m.countedRows(count))
	} else if target, ok := m.motionTarget(k, count); ok {
		if len(m.picked) > 0 {
			m.msg = "a motion cannot act on a selection: double the key instead, as in " + op + op
			return
		}
		m.takeRows(op, m.shownRange(m.cursor, target))
	} else if k == "ic" || k == "is" {
		m.takeRows(op, m.rowObject(k))
	}
	if m.tree == tree && m.tree.Gen() != gen {
		m.repeat = repeatAct{key: op, motion: k, count: count}
	}
}

// rowObject is the rows an operator takes by the tree and not by a motion:
// ic the rows under the cursor's row, is that row with all its siblings.
// Each goes with everything under it, shown or folded.
func (m *Model) rowObject(k string) []string {
	cur := m.current()
	switch {
	case cur == "":
		return nil
	case len(m.picked) > 0:
		m.msg = "a row object cannot act on a selection: press Esc to clear it"
		return nil
	case m.at.list != "" || m.at.query != "":
		m.msg = "a list or a filter does not show a row's branch: open the row first (h)"
		return nil
	}
	parent := cur
	if k == "is" {
		parent = m.tree.Row(cur).Parent
	}
	var ids []string
	for _, id := range m.tree.Children(parent) {
		if !hiddenTop(id) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		m.msg = "no rows under this one"
	}
	return ids
}

// under lists the rows beneath a row that go and come back with it.
func (m *Model) under(id string) []string {
	var out []string
	for _, kid := range m.tree.Children(id) {
		if r := m.tree.Row(kid); r != nil && !r.Deleted() {
			out = append(append(out, kid), m.under(kid)...)
		}
	}
	return out
}

// Capture live subtrees before deletion, using the same format as export.
func (m *Model) takeRows(op string, ids []string) {
	if len(ids) == 0 {
		return
	}
	if op == ">" || op == "<" {
		m.moveBlock(ids, op)
		return
	}
	text, n := m.writeRows(m.selectedRoots(ids))
	m.register = text
	m.pendingCmd = tea.SetClipboard(text)
	if op == "y" {
		m.copied = text
		m.msg = "copied " + rowCount(n)
		// What went is lit for a moment, its rows beneath included, so the
		// copy can be checked against the screen.
		lit := map[string]bool{}
		for _, id := range ids {
			lit[id] = true
			for _, kid := range m.under(id) {
				lit[kid] = true
			}
		}
		m.flashed = lit
		m.pendingCmd = tea.Batch(m.pendingCmd, m.startFlash())
		return
	}
	var ops []outline.Op
	for _, id := range ids {
		ops = append(ops, outline.Op{Kind: outline.Set, Row: id, Field: outline.FieldDeleted, Value: "1"})
	}
	if m.do(ops...) {
		m.rebuild()
		// Delete asks nothing, so it says what went: a folded row takes
		// rows with it that were never on screen.
		m.msg = "deleted " + rowCount(n)
	}
}

// normalAct dispatches a completed key, with an optional row or motion count.
func (m *Model) normalAct(k string, count int) {
	if target, ok := m.motionTarget(k, count); ok {
		m.cursor = target
		if k == "n" || k == "N" {
			m.land()
		}
		return
	}
	cur := m.current()
	switch k {
	case "o":
		if m.single() {
			m.newRow(false)
		}
	case "O":
		if m.single() {
			m.newRow(true)
		}
	case "/":
		// A fresh filter reads &. as the row the cursor is on now.
		if m.at.query == "" {
			m.at.here = cur
		}
		buf := []rune(m.at.query)
		m.ask = &prompt{kind: askFilter, ed: editor{buf: buf, caret: len(buf)}}
	case ":":
		m.ask = &prompt{kind: askCommand}
	case ".":
		// A count given now replaces the one the act was made with; rows
		// selected now are what it lands on.
		act, n := m.repeat, count
		if n == 0 && len(m.picked) == 0 {
			n = act.count
		}
		switch {
		case act.line != "":
			m.applyStamp(m.countedRows(count), act.line)
		case act.to.kind != "":
			m.moveTo(m.countedRows(count), act.to)
		case act.link.kind != "":
			m.linkTo(m.countedRows(count), act.link)
		case act.old != "":
			m.swapText(m.countedRows(count), act.old, act.new)
		case act.motion != "":
			m.operate(act.key, act.motion, n)
		case act.key != "":
			m.act(act.key, n)
		case act.how != "":
			m.retype(act, m.countedRows(count))
		default:
			m.msg = "nothing to repeat"
		}
		return
	case "u":
		m.replay(&m.undo, &m.redo, "undo")
	case "ctrl+r":
		m.replay(&m.redo, &m.undo, "redo")
	case "tab":
		m.sideFrom = m.departure()
		m.sideFocus = true
	case "p", "P":
		m.put(m.register, k == "P")
	case "gp", "gP":
		// The last copy, whatever has been deleted since.
		if m.copied == "" && !m.sideFocus {
			m.msg = "nothing copied to put: y copies"
		} else {
			m.put(m.copied, k == "gP")
		}
	case "f":
		m.ask = &prompt{kind: askJump}
	case "g;", "g,":
		m.walkChanges(k == "g;", count)
		return
	case "ctrl+o", "ctrl+i":
		// Back along the trail of jumps, and forward again.
		for range max(count, 1) {
			if !m.travel(k == "ctrl+o") {
				break
			}
		}
		return
	case "a":
		m.ask = &prompt{kind: askCapture}
	case "zt", "zz", "zb":
		// The cursor's row is drawn at the top, middle or bottom of the pane.
		m.scroll = k[1]
		return
	case "esc":
		// Esc undoes the most recent narrowing: the selection, then the
		// filter, then the zoom.
		if len(m.picked) > 0 {
			m.clearSelection()
		} else if m.at.query != "" {
			m.find, m.findHere, m.at.query = m.at.query, m.at.here, ""
			m.refilter()
			m.vis = nil
			m.rebuild()
			m.seek(cur)
			m.land()
		} else if n := len(m.zoom); n > 0 {
			was := m.at.root
			if m.at.returnRow != "" {
				was = m.at.returnRow
			}
			m.leave()
			m.at, m.zoom = m.zoom[n-1], m.zoom[:n-1]
			m.rebuild()
			m.seek(was)
			m.land()
		}
	}
	if cur == "" {
		return
	}

	ids := m.countedRows(count)
	switch k {
	case "A", "I", "cc":
		if m.single() {
			if k == "cc" {
				m.retypeRow(cur)
			} else {
				m.editRow(cur)
				if k == "I" {
					m.edit.caret = 0
				}
			}
			m.edit.mark(k)
		}
	case "w", "b", "e", "W", "B", "E":
		// A word motion steps into the row's text.
		if m.single() {
			m.enterText(cur, k, count)
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
	case "#", "@", "$", "-", "*", "~":
		// A sigil typed on the rows opens the stamp line already holding it.
		m.ask = &prompt{kind: askStamp, ed: editor{buf: []rune(k), caret: 1}, rows: ids}
	case "h", "left":
		if m.at.list != "" || (m.at.query != "" && m.at.flat) {
			if !m.single() {
				break
			}
			m.leave()
			m.zoom = append(m.zoom, m.at)
			m.at = place{root: m.tree.Row(cur).Parent, returnRow: cur}
			if m.hideClosed && sigil.FromFields(m.tree.Row(cur).Fields).Closed() {
				m.at.kept = map[string]bool{cur: true}
			}
			m.arrive()
			m.seek(cur)
		} else if m.at.query != "" && len(m.tree.Children(cur)) > 0 && !m.isFolded(cur) {
			m.applyFolds(map[string]bool{cur: true})
		} else if m.at.query != "" {
			// Folded or with nothing under it, h steps to the parent shown.
			for i := m.cursor - 1; i >= 0; i-- {
				if m.vis[i].depth < m.vis[m.cursor].depth {
					m.cursor = i
					break
				}
			}
		} else if len(m.tree.Children(cur)) > 0 && !m.folded[cur] {
			m.setFold(cur, true)
			m.rebuild()
		} else if p := m.tree.Row(cur).Parent; p != m.at.root {
			m.seek(p)
		}
	case "l", "right":
		if m.at.list != "" || (m.at.query != "" && m.at.flat) {
			break
		} else if m.at.query != "" && m.isFolded(cur) {
			m.applyFolds(map[string]bool{cur: false})
		} else if m.at.query != "" {
			if next := m.cursor + 1; next < len(m.vis) && m.vis[next].depth > m.vis[m.cursor].depth {
				m.cursor = next
			}
		} else if m.folded[cur] {
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
	case "^":
		// A pin is an order the user asks for: the row is drawn first among
		// its siblings, and the cursor goes with it.
		m.toggle(ids, sigil.FieldPin, "1")
		m.rebuild()
		m.seek(cur)
	case "t":
		// Today is a pick that lapses by itself: the field holds the day it
		// was made, and tomorrow that is no longer today.
		m.pick(ids)
	case "m":
		m.ask = &prompt{kind: askMove, rows: ids}
	case "L":
		m.ask = &prompt{kind: askLink, rows: ids}
	case "gl":
		if m.single() {
			m.follow()
		}
	case "gf":
		if m.single() {
			m.goProject(cur)
		}
	case "gn", "gN":
		if m.single() {
			m.enterMatch(k == "gn")
		}
	case "zc", "zo":
		m.foldBranches(ids, k == "zc")
	case "J", "K":
		if m.at.list == kindToday {
			if m.single() {
				m.reorderToday(cur, k == "J")
			}
		} else if len(m.picked) > 0 {
			m.moveBlock(ids, k)
		} else {
			m.reorder(cur, k == "J")
		}
	default:
		if hint, ok := strayKeys[k]; ok {
			m.msg = hint
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
	if e.normal {
		m.textKey(msg)
		if e.sideC && m.edit == e && !e.normal {
			e.inside = false
		}
		return
	}
	if e.body {
		m.bodyType(msg)
		return
	}
	if msg.String() != "up" && msg.String() != "down" {
		e.vertical = false
	}
	if m.walkValues(e, surfaceRow, msg.String()) {
		return
	}
	if m.complete(e, surfaceRow, msg.String()) {
		return
	}
	if e.side {
		// A project's name is one row: nothing splits it, joins it, indents
		// it or carries the typing on into another.
		switch msg.String() {
		case "enter", "esc":
			if msg.String() == "esc" && e.inside {
				m.textBack(e)
			} else {
				m.finishEdit()
			}
		case "up":
			m.editVertical(-1)
		case "down":
			m.editVertical(1)
		case "tab", "shift+tab":
		case "backspace":
			if e.caret > 0 {
				e.key(msg)
			}
		default:
			e.key(msg)
		}
		return
	}
	switch msg.String() {
	case "enter":
		if e.caret > 0 && e.caret < len(e.buf) && m.at.list == "" && m.at.query == "" {
			tail := append([]rune(nil), e.buf[e.caret:]...)
			e.buf, e.how = e.buf[:e.caret], ""
			if m.finishEdit() {
				m.newRowAt(false, e.row)
				if m.edit != nil {
					m.edit.buf = tail
					m.edit.merge = e.merge
					m.edit.mark("") // a split is not text to type again
				}
			}
			return
		}
		// A fresh row chains into the next one, so a list is typed without
		// leaving the keyboard's home row.
		if m.finishEdit() && e.fresh {
			m.newRow(false)
		}
	case "esc":
		// Typing ends where it was opened: inside the row, or in the outline.
		if e.inside {
			m.textBack(e)
		} else {
			m.finishEdit()
		}
	case "up":
		m.editVertical(-1)
	case "down":
		m.editVertical(1)
	case "tab":
		m.shift(e.row, true)
	case "shift+tab":
		m.shift(e.row, false)
	case "backspace":
		if len(e.buf) == 0 && e.fresh {
			m.finishEdit()
		} else if e.caret == 0 {
			m.joinEdit()
		} else {
			e.key(msg)
		}
	default:
		e.key(msg)
	}
}

// editNext keeps the row being typed and goes on typing in the row drawn
// above or below it, the caret in the same column. Each row is its own undo
// step. At either end the line stays as it is.
func (m *Model) editNext(by int) {
	e := m.edit
	next := m.cursor + by
	if e.side || next < 0 || next >= len(m.vis) {
		return
	}
	target := m.vis[next].id
	blank := e.fresh && strings.TrimSpace(string(e.buf)) == ""
	if !m.finishEdit() && !blank {
		return // the row could not be saved, and the status line says so
	}
	m.seek(target)
	if m.current() != target {
		return
	}
	m.editRow(target)
	m.edit.caret = min(e.caret, len(m.edit.buf))
}

// joinEdit keeps typing in the displayed predecessor, carrying the removal
// and any pending moves into that edit's undo step.
func (m *Model) joinEdit() {
	e := m.edit
	if m.at.list != "" || m.at.query != "" || m.cursor == 0 || len(m.tree.Children(e.row)) > 0 {
		return
	}
	previous := m.vis[m.cursor-1].id
	if e.fresh {
		m.tree.Forget(e.row)
		e.ops, e.inv = nil, nil
	} else {
		op := outline.Op{Kind: outline.Set, Row: e.row, Field: outline.FieldDeleted, Value: "1"}
		inv, err := m.apply([]outline.Op{op})
		if err != nil {
			m.msg = err.Error()
			return
		}
		e.ops = append(e.ops, op)
		e.inv = append(inv, e.inv...)
	}
	m.editRow(previous)
	m.edit.buf = append(append(m.edit.buf, ' '), e.buf...)
	m.edit.ops, m.edit.inv = e.ops, e.inv
	m.edit.merge = e.merge
	m.rebuild()
	m.seek(previous)
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

// startFlash lights a copy until its own timer ends it; a copy made sooner
// takes the light over.
func (m *Model) startFlash() tea.Cmd {
	m.flashGen++
	gen := m.flashGen
	return tea.Tick(flashFor, func(time.Time) tea.Msg { return flashMsg{gen} })
}

// unflash puts out whatever a copy lit, in the outline and on the line.
func (m *Model) unflash() {
	m.flashed = nil
	if m.edit != nil {
		m.edit.flash = [2]int{}
	}
	if m.ask != nil {
		m.ask.ed.flash = [2]int{}
	}
}
