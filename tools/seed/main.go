// Command seed fills a fresh database with a large, plausible outline for
// trying Docket under load: dozens of projects, thousands of tasks, a year of
// history. Everything goes through store.Commit, so the log, the stamps and
// the ages the sidebar reads are what real use would leave.
//
//	go run ./tools/seed -db dev.db -tasks 6000
//
// The Inbox opens with a row for each kind of address `gl` opens, and one
// linked to another. It refuses a database that already holds rows. The same -seed gives the
// same outline, with dates relative to today.
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
	"github.com/sglavoie/docket/internal/store"
)

const day = 24 * time.Hour

// The fixed rows the outline expects, as the TUI would create them.
const (
	inboxID   = "inbox"
	filtersID = "filters"
	fixedPos  = "0"
)

var projectNames = []string{
	"docket-rewrite", "blog", "tax-2026", "homelab", "kitchen-reno", "garden",
	"japan-trip", "thesis", "bike-maintenance", "learn-rust", "photo-archive",
	"newsletter", "side-hustle", "wedding-gifts", "car-lease", "home-insurance",
	"podcast", "dotfiles", "health", "running-plan", "piano", "book-club",
	"client-acme", "client-globex", "client-initech", "conference-talk",
	"open-source", "kids-school", "moving-out", "budget-2027", "reading-list",
	"woodworking", "nas-upgrade", "website-redesign", "job-search", "spanish",
	"family-reunion", "camping-gear", "music-production", "board-games",
	"aquarium", "cookbook", "volunteering", "chess", "3d-printing",
	"retirement-plan", "attic-cleanup", "solar-panels", "keyboard-build",
	"mentoring", "grant-application", "research-paper", "game-jam",
}

// projectColors are given to every fifth project, in turn.
var projectColors = []string{
	"teal", "orange", "purple", "green", "pink", "#3fa7c4", "yellow", "red", "blue", "brown",
}

var headings = []string{
	"Backlog:", "Next up:", "Design:", "Research:", "Admin:", "Errands:",
	"Ideas:", "Bugs:", "Writing:", "Purchases:", "Follow-ups:", "Someday:",
	"Planning:", "Infrastructure:", "Notes:", "Review:",
}

var verbs = []string{
	"Write", "Review", "Call", "Email", "Fix", "Draft", "Order", "Book",
	"Compare", "Research", "Schedule", "Clean", "Update", "Refactor", "Test",
	"Plan", "Read", "Sketch", "Measure", "Replace", "Back up", "Pay",
	"Renew", "Ask about", "Sort out", "Prototype", "Document", "Benchmark",
	"Cancel", "Pick up", "Return", "Configure", "Migrate", "Outline",
	"Finish", "Check", "Install", "Print", "Submit", "Archive",
}

var objects = []string{
	"the quarterly report", "dentist appointment", "flight options",
	"router firmware", "invoice #4417", "garage shelves", "the intro chapter",
	"sync conflict handling", "backup script", "insurance renewal",
	"birthday present", "grocery list", "landlord about the heater",
	"slides for Thursday", "kitchen faucet", "database migration",
	"cover letter", "tyre pressure", "passport photos", "fence quote",
	"CI pipeline", "the onboarding doc", "the budget spreadsheet",
	"library books", "SSL certificates", "the sidebar layout",
	"recipe for Sunday", "fertiliser for the lawn", "meeting notes",
	"the API contract", "chimney inspection", "hotel in Kyoto",
	"camera lens", "the release checklist", "parking permit",
	"bank statement", "the grant budget", "keyboard switches",
	"tax receipts", "school forms", "monitor arm", "the test fixtures",
	"water filter", "the reading notes", "logo variants", "smoke detectors",
	"vacation photos", "the conference abstract", "pull request #212",
	"old hard drives", "the pricing page", "gym membership",
}

var details = []string{
	"before the deadline", "with Sam", "for next week", "again",
	"properly this time", "v2", "and the spare", "for mum", "draft 3",
	"if time allows", "from scratch", "per the spec", "after lunch",
	"in the shed", "by phone", "online", "with receipts", "",
	"", "", "", "", "", "",
}

var noteLines = []string{
	"Ref: see the thread from last month", "Budget is about 300",
	"Ask for the corporate rate", "Blocked until the parts arrive",
	"Two options: rent or buy", "Model number is on the back",
	"Remember to keep the receipt", "Last done in spring",
	"Prefer the morning slot", "Link is in the shared folder",
}

var tags = []string{
	"home", "work", "errands", "phone", "computer", "deep", "quick",
	"money", "health", "family", "waiting-on", "someday", "admin", "outside",
}

var contexts = []string{"home", "office", "phone", "town", "computer"}

// row is a row to write: its insert at created, and later its state change.
type row struct {
	id, parent, pos string
	fields          map[string]string // written with the insert
	created         int64
	state           string // written at changed, when set
	changed         int64
}

type gen struct {
	r     *rand.Rand
	now   time.Time
	start int64 // when the history begins
	end   int64 // the last moment the project being built was touched
	rows  []*row
	n     int
	tasks int
}

func (g *gen) id() string {
	g.n++
	return fmt.Sprintf("seed%06d", g.n)
}

func (g *gen) pick(xs []string) string { return xs[g.r.IntN(len(xs))] }

func (g *gen) chance(p float64) bool { return g.r.Float64() < p }

// between returns a moment uniformly between a and the end of the project
// being built.
func (g *gen) between(a int64) int64 {
	if a >= g.end {
		return g.end
	}
	return a + g.r.Int64N(g.end-a)
}

func (g *gen) dayOffset(d int) string {
	return g.now.Add(time.Duration(d) * day).Format("2006-01-02")
}

func (g *gen) title() string {
	t := g.pick(verbs) + " " + g.pick(objects)
	if d := g.pick(details); d != "" {
		t += " " + d
	}
	return t
}

func (g *gen) add(parent, pos string, created int64, fields map[string]string) *row {
	r := &row{id: g.id(), parent: parent, pos: pos, fields: fields, created: created}
	g.rows = append(g.rows, r)
	return r
}

// task adds one task under parent, sometimes with subtasks and a note.
func (g *gen) task(parent, pos string, after int64, depth int) {
	created := g.between(after)
	l := sigil.Line{Text: g.title()}
	// The older a task, the likelier it is closed by now.
	age := float64(g.now.UnixMilli()-created) / float64(g.now.UnixMilli()-g.start)
	if g.chance(0.25 + 0.7*age) {
		l.State = sigil.Done
		if g.chance(0.15) {
			l.State = sigil.Canceled
		}
	} else {
		switch x := g.r.Float64(); {
		case x < 0.72:
			l.State = sigil.Open
		case x < 0.82:
			l.State = sigil.Doing
		case x < 0.92:
			l.State = sigil.Waiting
		default:
			l.State = sigil.Parked
		}
	}
	if g.chance(0.05) {
		l.Flag = true
	}
	if g.chance(0.06) {
		// Mostly ahead; now and then one slips past.
		d := 2 + g.r.IntN(120)
		if g.chance(0.04) {
			d = -g.r.IntN(20)
		}
		l.Due = g.dayOffset(d)
	}
	if g.chance(0.04) && !l.Closed() {
		l.Start = g.dayOffset(1 + g.r.IntN(40))
	}
	if g.chance(0.025) {
		l.Every = g.pick([]string{"1w", "2w", "1m", "3m", "1y"})
		if l.Due == "" {
			l.Due = g.dayOffset(g.r.IntN(30))
		}
	}
	if g.chance(0.35) {
		n := 1 + g.r.IntN(2)
		for range n {
			l.Tags = append(l.Tags, g.pick(tags))
		}
		sort.Strings(l.Tags)
		l.Tags = compact(l.Tags)
	}
	if g.chance(0.2) {
		l.Context = g.pick(contexts)
	}
	f := l.Fields()
	f[sigil.FieldState] = ""
	r := g.add(parent, pos, created, f)
	r.state = l.State
	// Work still to do has mostly been looked at lately; closed work
	// changed state some time after it was made.
	r.changed = g.between(created)
	if !l.Closed() && g.chance(0.75) {
		r.changed = max(r.changed, g.between(created), g.between(created), g.between(created))
	}
	g.tasks++

	kid := ""
	if depth < 2 && g.chance(0.12) {
		for range 1 + g.r.IntN(5) {
			kid = outline.Between(kid, "")
			g.task(r.id, kid, created, depth+1)
		}
	}
	if g.chance(0.05) {
		kid = outline.Between(kid, "")
		g.add(r.id, kid, g.between(created), map[string]string{sigil.FieldText: g.pick(noteLines)})
	}
}

func compact(xs []string) []string {
	out := xs[:0]
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

// build makes the outline. dir is the folder the database is in, which a
// seeded link opens.
func (g *gen) build(projects, target int, dir string) {
	start := g.now.Add(-400 * day).UnixMilli()
	g.start, g.end = start, g.now.UnixMilli()
	g.add("", fixedPos, start, map[string]string{sigil.FieldText: "Inbox"})
	g.add("", fixedPos, start, map[string]string{sigil.FieldText: "Filters"})
	g.rows[0].id, g.rows[1].id = inboxID, filtersID

	names := append([]string(nil), projectNames...)
	g.r.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	for len(names) < projects {
		names = append(names, fmt.Sprintf("project-%d", len(names)+1))
	}
	names = names[:projects]

	perProject := target * 6 / (projects * 5) // quiet projects are a third the size
	pos := fixedPos
	for i, name := range names {
		pos = outline.Between(pos, "")
		// A quarter of the projects have gone quiet: nothing in them has
		// changed for days or months, and they are smaller.
		g.end = g.now.UnixMilli()
		quiet := g.chance(0.25)
		if quiet {
			g.end = g.now.Add(-time.Duration(3+g.r.IntN(120)) * day).UnixMilli()
		}
		created := g.between(start)
		p := g.add("", pos, created, map[string]string{sigil.FieldText: name})
		switch {
		case i%9 == 4:
			p.state, p.changed = sigil.Parked, g.between(created)
		case i%13 == 6:
			p.state, p.changed = sigil.Done, g.between(created)
		case i%17 == 11:
			p.state, p.changed = sigil.Canceled, g.between(created)
		}
		// Every fifth project has a colour, a hex value among the names. It
		// draws on no randomness, so the rest of the outline is unchanged.
		if i%5 == 0 {
			p.fields[sigil.FieldColor] = projectColors[i/5%len(projectColors)]
		}
		if g.chance(0.1) {
			p.fields[sigil.FieldDue] = g.dayOffset(g.r.IntN(30) - 5)
		}
		// Some projects are big, some tiny.
		want := perProject/4 + g.r.IntN(perProject*7/4+1)
		if quiet {
			want /= 3
		}
		goal := g.tasks + want
		hpos := ""
		hs := append([]string(nil), headings...)
		g.r.Shuffle(len(hs), func(i, j int) { hs[i], hs[j] = hs[j], hs[i] })
		hs = hs[:2+g.r.IntN(5)]
		for i, h := range hs {
			hpos = outline.Between(hpos, "")
			hrow := g.add(p.id, hpos, g.between(created), map[string]string{sigil.FieldText: h})
			// Each heading takes an uneven share of what is left.
			share := g.tasks + (goal-g.tasks)*(1+g.r.IntN(3))/(2*(len(hs)-i))
			if i == len(hs)-1 {
				share = goal
			}
			tpos := ""
			for g.tasks < share {
				tpos = outline.Between(tpos, "")
				g.task(hrow.id, tpos, hrow.created, 1)
			}
		}
		// A few loose tasks straight under the project.
		for range g.r.IntN(6) {
			hpos = outline.Between(hpos, "")
			g.task(p.id, hpos, created, 1)
		}
	}

	// The Inbox: recent captures, not yet filed.
	g.end = g.now.UnixMilli()
	ipos, first := "", ""
	for range 30 + g.r.IntN(20) {
		ipos = outline.Between(ipos, "")
		if first == "" {
			first = ipos
		}
		g.task(inboxID, ipos, g.now.Add(-10*day).UnixMilli(), 2)
	}

	g.pickToday()
	// After the picks, which read every row before them.
	g.links(first, dir)
}

// links heads the Inbox with a row for each address `gl` opens: a web page,
// a folder, a file at a line and one from the home folder, then a row linked
// to another, which `gl` follows. A Mail link is left out: one made up names
// no message, and Mail says it cannot open it. They draw on no randomness,
// so the rest of the outline is unchanged.
func (g *gen) links(before, dir string) {
	texts := []string{
		"Read the Go release notes https://go.dev/doc/devel/release",
		"Tidy the folder this database is in " + dir,
		"Fix the first rule in " + filepath.Join(dir, "SPEC.md") + ":10",
		"Sort out ~/Downloads",
	}
	at := g.now.Add(-time.Hour).UnixMilli()
	pos := ""
	var target string
	for i, text := range texts {
		pos = outline.Between(pos, before)
		r := g.add(inboxID, pos, at, map[string]string{sigil.FieldText: text})
		r.state, r.changed = sigil.Open, at
		if i == 0 {
			target = r.id
		}
	}
	pos = outline.Between(pos, before)
	r := g.add(inboxID, pos, at, map[string]string{
		sigil.FieldText: "Follow the link to the release notes task",
		"link":          target,
	})
	r.state, r.changed = sigil.Open, at
}

// pickToday picks a dozen open tasks for today, in order.
func (g *gen) pickToday() {
	today := g.now.Format("2006-01-02")
	tpos := ""
	picked := 0
	for _, i := range g.r.Perm(len(g.rows)) {
		r := g.rows[i]
		if r.state != sigil.Open && r.state != sigil.Doing {
			continue
		}
		r.fields[sigil.FieldToday] = today
		tpos = outline.Between(tpos, "")
		r.fields["todaypos"] = tpos
		if picked++; picked == 12 {
			return
		}
	}
}

// commit writes the rows oldest moment first, one transaction per day, so the
// clock only ever moves forward and each row's ages are what they claim.
func commit(st *store.Store, rows []*row) (int, error) {
	type event struct {
		at  int64
		ops []outline.Op
	}
	var events []event
	for _, r := range rows {
		ops := []outline.Op{{Kind: outline.Insert, Row: r.id, Parent: r.parent, Pos: r.pos}}
		keys := make([]string, 0, len(r.fields))
		for k, v := range r.fields {
			if v != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			ops = append(ops, outline.Op{Kind: outline.Set, Row: r.id, Field: k, Value: r.fields[k]})
		}
		events = append(events, event{r.created, ops})
		if r.state != "" {
			events = append(events, event{r.changed, []outline.Op{
				{Kind: outline.Set, Row: r.id, Field: sigil.FieldState, Value: r.state},
				{Kind: outline.Set, Row: r.id, Field: sigil.FieldChanged, Value: strconv.FormatInt(r.changed, 10)},
			}})
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at < events[j].at })

	total := 0
	for i := 0; i < len(events); {
		bucket := events[i].at / day.Milliseconds()
		at := events[i].at
		var ops []outline.Op
		for ; i < len(events) && events[i].at/day.Milliseconds() == bucket; i++ {
			ops = append(ops, events[i].ops...)
			at = events[i].at
		}
		if err := st.Commit(ops, at); err != nil {
			return total, err
		}
		total += len(ops)
	}
	return total, nil
}

func main() {
	path := flag.String("db", "dev.db", "database to fill; must hold no rows")
	tasks := flag.Int("tasks", 6000, "roughly how many tasks to make")
	projects := flag.Int("projects", 48, "how many projects to make")
	seed := flag.Uint64("seed", 1, "random seed")
	flag.Parse()

	st, err := store.Open(*path)
	if err != nil {
		fail(err)
	}
	defer st.Close()
	tree, err := st.Load()
	if err != nil {
		fail(err)
	}
	if len(tree.Children("")) > 0 {
		fail(fmt.Errorf("%s already holds an outline; remove it first", *path))
	}

	g := &gen{r: rand.New(rand.NewPCG(*seed, 0)), now: time.Now()}
	abs, err := filepath.Abs(*path)
	if err != nil {
		fail(err)
	}
	g.build(*projects, *tasks, filepath.Dir(abs))
	n, err := commit(st, g.rows)
	if err != nil {
		fail(err)
	}
	fmt.Printf("seeded %s: %d rows, %d tasks, %d projects, %d operations\n",
		*path, len(g.rows), g.tasks, *projects, n)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "seed:", strings.TrimSpace(err.Error()))
	os.Exit(1)
}
