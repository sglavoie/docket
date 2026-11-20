// Command stats prints what the outline's own log says about the last 28
// days: what was finished, what was picked for Today and what came of it,
// what was carried, what was added, and whether the outline was touched at
// all. It opens the database read-only, writes nothing, and prints counts
// only, never a row's text.
//
//	go run ./tools/stats              # DOCKET_DB, else the usual place
//	go run ./tools/stats -db dev.db
//
// Each day's figures are read from the ops log by the stamp's day in local
// time, replayed in stamp order so a set that lost to a newer one on another
// machine never counts. Rows deleted, or under a deleted row, are left out.
// A day of more than 500 inserts is an import: it is marked, and left out of
// the medians.
package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sglavoie/docket/internal/outline"
	"github.com/sglavoie/docket/internal/sigil"
)

const (
	days        = 28
	importDay   = 500 // more inserts than this in a day is a bulk import
	inboxID     = "inbox"
	filtersID   = "filters"
	dayFormat   = "2006-01-02"
	dayMillis   = int64(24 * time.Hour / time.Millisecond)
	maxAncestry = 1 << 12 // a walk up the tree longer than this is a cycle
)

func main() {
	path := flag.String("db", "", "database to read (default: $DOCKET_DB, else ~/.local/share/docket-v2/docket.db)")
	flag.Parse()
	if err := run(*path, time.Now(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "stats:", err)
		os.Exit(1)
	}
}

func run(path string, now time.Time, w io.Writer) error {
	if path == "" {
		var err error
		if path, err = dbPath(); err != nil {
			return err
		}
	}
	db, err := openReadOnly(path)
	if err != nil {
		return err
	}
	defer db.Close()
	r, err := collect(db, now)
	if err != nil {
		return err
	}
	return r.write(w)
}

// dbPath is where docket itself looks.
func dbPath() (string, error) {
	if p := os.Getenv("DOCKET_DB"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "docket-v2", "docket.db"), nil
}

// openReadOnly opens the database so that nothing can be written: the file
// is opened read-only, and the connection refuses writes besides. A missing
// file is an error, never a new database.
func openReadOnly(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	query := "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)"
	// With no write-ahead log beside it, no docket has the database open,
	// and a read-only open would still leave a log and its index behind.
	// Opened immutable, nothing is created; with a log there, it is read.
	if _, err := os.Stat(abs + "-wal"); os.IsNotExist(err) {
		query += "&immutable=1"
	}
	dsn := (&url.URL{Scheme: "file", OmitHost: true, Path: abs, RawQuery: query}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// day is one line of the table. Each set holds row identities, counted
// once the replay knows which rows survive.
type day struct {
	start   time.Time
	done    map[string]bool // went done that day, and still done at its end
	picks   map[string]bool // picked that day, and still picked at its end
	closed  map[string]bool // of picks, done at the day's end
	carried map[string]bool // picked on an earlier day and still to do at its start
	added   map[string]bool
	inbox   map[string]bool // of added, put in the Inbox
	inserts int             // every insert, for telling an import
	ops     int
}

func (d *day) imported() bool { return d.inserts > importDay }

type report struct {
	days []*day
	live func(id string) bool

	inboxLeft   int
	inboxOldest int // days; -1 when the Inbox is empty
	projects    int
	doing       int
	estimated   int
}

type row struct {
	parent string
	fields map[string]string
}

// replay is the outline as the log has built it so far.
type replay map[string]*row

func (t replay) apply(kind, id, field, value, parent string) {
	r := t[id]
	switch outline.Kind(kind) {
	case outline.Insert:
		if r == nil {
			t[id] = &row{parent: parent, fields: map[string]string{}}
		}
	case outline.Set:
		if r == nil {
			r = &row{fields: map[string]string{}}
			t[id] = r
		}
		r.fields[field] = value
	case outline.Move:
		if r != nil {
			r.parent = parent
		}
	}
}

// live reports a row that exists with no deleted row above it, and not
// under the saved filters, which are not part of the outline.
func (t replay) live(id string) bool {
	for i := 0; id != ""; i++ {
		r := t[id]
		if r == nil || r.fields[outline.FieldDeleted] != "" || id == filtersID || i > maxAncestry {
			return false
		}
		id = r.parent
	}
	return true
}

func (t replay) state(id string) string {
	if r := t[id]; r != nil {
		return r.fields[sigil.FieldState]
	}
	return ""
}

func (t replay) picked(id string) string {
	if r := t[id]; r != nil {
		return r.fields[sigil.FieldToday]
	}
	return ""
}

func stampMillis(stamp string) int64 {
	ms, _, _ := strings.Cut(stamp, "-")
	n, _ := strconv.ParseInt(ms, 10, 64)
	return n
}

func collect(db *sql.DB, now time.Time) (*report, error) {
	loc := now.Location()
	y, m, d := now.Date()
	r := &report{}
	for i := range days {
		r.days = append(r.days, &day{
			start:   time.Date(y, m, d-(days-1)+i, 0, 0, 0, 0, loc),
			done:    map[string]bool{},
			picks:   map[string]bool{},
			closed:  map[string]bool{},
			carried: map[string]bool{},
			added:   map[string]bool{},
			inbox:   map[string]bool{},
		})
	}
	end := time.Date(y, m, d+1, 0, 0, 0, 0, loc).UnixMilli()

	t := replay{}
	// cur is the day being read; -1 before the window.
	cur := -1
	// enter begins day i: what is carried into it is read from the outline
	// as the day before left it.
	enter := func(i int) {
		day := r.days[i]
		name := day.start.Format(dayFormat)
		for id, row := range t {
			pick := row.fields[sigil.FieldToday]
			s := row.fields[sigil.FieldState]
			if pick != "" && pick < name && (s == sigil.Open || s == sigil.Doing || s == sigil.Waiting) && t.live(id) {
				day.carried[id] = true
			}
		}
	}
	// leave ends day i: what was done or picked that day counts if it still
	// stands at the day's end, so an undo takes it back.
	leave := func(i int) {
		day := r.days[i]
		for id := range day.done {
			if t.state(id) != sigil.Done {
				delete(day.done, id)
			}
		}
		for id := range day.picks {
			if t.picked(id) == "" {
				delete(day.picks, id)
			} else if t.state(id) == sigil.Done {
				day.closed[id] = true
			}
		}
	}
	advance := func(ms int64) {
		for cur+1 < days && ms >= r.days[cur+1].start.UnixMilli() {
			if cur >= 0 {
				leave(cur)
			}
			cur++
			enter(cur)
		}
	}

	rs, err := db.Query(`SELECT stamp, kind, row, field, value, parent FROM ops ORDER BY stamp, seq`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var stamp, kind, id, field, value, parent string
		if err := rs.Scan(&stamp, &kind, &id, &field, &value, &parent); err != nil {
			return nil, err
		}
		ms := stampMillis(stamp)
		advance(ms)
		var today *day
		if cur >= 0 && ms < end {
			today = r.days[cur]
		}
		if today != nil {
			today.ops++
			switch outline.Kind(kind) {
			case outline.Insert:
				today.inserts++
				if _, seen := t[id]; !seen && id != inboxID && id != filtersID {
					today.added[id] = true
					if parent == inboxID {
						today.inbox[id] = true
					}
				}
			case outline.Set:
				switch field {
				case sigil.FieldState:
					if value == sigil.Done && t.state(id) != sigil.Done {
						today.done[id] = true
					}
				case sigil.FieldToday:
					if value != "" && t.picked(id) != value {
						today.picks[id] = true
					}
				}
			}
		}
		t.apply(kind, id, field, value, parent)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	advance(now.UnixMilli())
	if cur >= 0 {
		leave(cur)
	}
	r.live = t.live

	if err := r.current(db, now); err != nil {
		return nil, err
	}
	return r, nil
}

// current reads the lines below the table from the outline as it stands.
func (r *report) current(db *sql.DB, now time.Time) error {
	rows := map[string]*row{}
	created := map[string]int64{}
	rs, err := db.Query(`SELECT id, parent, created FROM rows`)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var id, parent string
		var c int64
		if err := rs.Scan(&id, &parent, &c); err != nil {
			return err
		}
		rows[id] = &row{parent: parent, fields: map[string]string{}}
		created[id] = c
	}
	if err := rs.Err(); err != nil {
		return err
	}
	fs, err := db.Query(`SELECT row, name, value FROM fields WHERE value != ''`)
	if err != nil {
		return err
	}
	defer fs.Close()
	for fs.Next() {
		var id, name, value string
		if err := fs.Scan(&id, &name, &value); err != nil {
			return err
		}
		if row := rows[id]; row != nil {
			row.fields[name] = value
		}
	}
	if err := fs.Err(); err != nil {
		return err
	}

	t := replay(rows)
	oldest := int64(-1)
	for id, row := range t {
		if !t.live(id) {
			continue
		}
		l := sigil.FromFields(row.fields)
		switch {
		case row.parent == inboxID:
			if !l.Closed() {
				r.inboxLeft++
				if oldest < 0 || created[id] < oldest {
					oldest = created[id]
				}
			}
		case row.parent == "" && id != inboxID:
			if l.State != sigil.Parked && !l.Closed() {
				r.projects++
			}
		}
		if l.State == sigil.Doing {
			r.doing++
		}
		if l.State != "" && l.State != sigil.Parked && !l.Closed() && slices.ContainsFunc(l.Tags, isEst) {
			r.estimated++
		}
	}
	r.inboxOldest = -1
	if oldest >= 0 {
		r.inboxOldest = int(max(0, now.UnixMilli()-oldest) / dayMillis)
	}
	return nil
}

func isEst(tag string) bool {
	name, _, _ := strings.Cut(tag, "=")
	return name == "est"
}

// count is how many of a day's rows survive.
func (r *report) count(set map[string]bool) int {
	n := 0
	for id := range set {
		if r.live(id) {
			n++
		}
	}
	return n
}

// columns are the table's figures, in order.
var columns = []string{"done", "picks", "closed", "carried", "added", "inbox"}

func (r *report) figures(d *day) []int {
	return []int{r.count(d.done), r.count(d.picks), r.count(d.closed), r.count(d.carried), r.count(d.added), r.count(d.inbox)}
}

func median(xs []int) string {
	if len(xs) == 0 {
		return "-"
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return strconv.Itoa(s[n/2])
	}
	return strconv.FormatFloat(float64(s[n/2-1]+s[n/2])/2, 'f', -1, 64)
}

func (r *report) write(out io.Writer) error {
	w := &bytes.Buffer{}
	fmt.Fprintf(w, "%-14s", "day")
	for _, c := range columns {
		fmt.Fprintf(w, " %7s", c)
	}
	fmt.Fprintf(w, "  %s\n", "changed")
	cols := make([][]int, len(columns))
	changed, counted := 0, 0
	for _, d := range r.days {
		fs := r.figures(d)
		fmt.Fprintf(w, "%-14s", d.start.Format("2006-01-02 Mon"))
		for _, f := range fs {
			fmt.Fprintf(w, " %7d", f)
		}
		mark := "no"
		if d.ops > 0 {
			mark = "yes"
		}
		if d.imported() {
			mark += "  import, not in the medians"
		} else {
			counted++
			if d.ops > 0 {
				changed++
			}
			for i, f := range fs {
				cols[i] = append(cols[i], f)
			}
		}
		fmt.Fprintf(w, "  %s\n", mark)
	}
	fmt.Fprintf(w, "%-14s", "median")
	for _, c := range cols {
		fmt.Fprintf(w, " %7s", median(c))
	}
	fmt.Fprintf(w, "  %d of %d days\n\n", changed, counted)

	switch r.inboxOldest {
	case -1:
		fmt.Fprintln(w, "Inbox: empty")
	default:
		fmt.Fprintf(w, "Inbox: %d not yet filed, the oldest %d days old\n", r.inboxLeft, r.inboxOldest)
	}
	fmt.Fprintf(w, "Active projects: %d\n", r.projects)
	fmt.Fprintf(w, "Doing: %d\n", r.doing)
	fmt.Fprintf(w, "Open tasks with #est: %d\n", r.estimated)
	_, err := out.Write(w.Bytes())
	return err
}
