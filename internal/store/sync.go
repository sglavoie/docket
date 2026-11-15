package store

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/sglavoie/docket/internal/outline"
)

// Machines sync by exchanging operations, never the database. Each machine
// appends its own operations to one file in a shared folder and reads the
// others' files, so no file ever has two writers and any tool that copies
// files can carry them.
//
// A remote operation is applied by its stamp, per field: the newest write to
// a field wins, as does the newest move of a row. Applying is idempotent and
// independent of arrival order, so every machine that has seen the same
// operations holds the same state.
//
// A pass has three parts. SyncBegin reads what there is to write from the
// database; SyncJob.Run does all the work in the folder and touches no
// database, so it may run on another goroutine while the folder is slow;
// SyncEnd applies what Run read.

const syncSchema = `
CREATE TABLE IF NOT EXISTS pending(stamp TEXT PRIMARY KEY, op TEXT NOT NULL) WITHOUT ROWID;
`

// wireOp is an operation as written to a sync file, one JSON object a line.
type wireOp struct {
	Stamp  string `json:"s"`
	Kind   string `json:"k"`
	Row    string `json:"r"`
	Field  string `json:"f,omitempty"`
	Value  string `json:"v,omitempty"`
	Parent string `json:"p,omitempty"`
	Pos    string `json:"o,omitempty"`
}

func stampMS(stamp string) int64 {
	ms, _ := strconv.ParseInt(strings.SplitN(stamp, "-", 2)[0], 10, 64)
	return ms
}

// stuckAt is where a sync file stopped being readable, and for how many
// passes in a row.
type stuckAt struct {
	off    int64
	line   int
	passes int
}

// stuckAfter is how many passes must find the same unreadable line before
// it is reported. A file still being copied in can hold one for a moment.
const stuckAfter = 2

// Stuck names a sync file that holds a line this machine cannot read.
// Nothing after that line is applied until the file is whole again.
type Stuck struct {
	File string
	Line int
}

// SyncReport is the outcome of one pass.
type SyncReport struct {
	Changed  int     // remote operations that changed something here
	Machines int     // other machines that have written to the folder
	Waiting  int     // operations held back for a row that has not arrived
	Stuck    []Stuck // files that cannot be read past a line
	// Again asks for another pass at once: this machine's file was gone
	// from the folder, so all of it has to be written out.
	Again bool
}

// SyncJob is one pass's work in the folder.
type SyncJob struct {
	dir, device string
	out         []byte           // this machine's operations not yet in its file
	from, last  int64            // the log entries out covers
	at          map[string]int64 // how far into each file this machine has read
}

// SyncRead is what Run found in the folder.
type SyncRead struct {
	err      error
	dir      string
	last     int64            // the log entry this machine's file now reaches
	lost     bool             // this machine's file was gone from the folder
	ops      []wireOp         // the lines not read before
	at       map[string]int64 // how far into each file they reach
	bad      map[string]stuckAt
	machines int
}

// Sync does a whole pass at once: it writes this machine's new operations
// to dir and applies the other machines' new ones. It reports how many
// remote operations changed something here.
func (s *Store) Sync(dir string) (int, error) {
	changed := 0
	for pass := 0; pass < 2; pass++ {
		job, err := s.SyncBegin(dir)
		if err != nil {
			return changed, err
		}
		report, err := s.SyncEnd(job.Run())
		changed += report.Changed
		if err != nil || !report.Again {
			return changed, err
		}
	}
	return changed, nil
}

// SyncBegin gathers what a pass will write: the operations made here that
// this machine's file does not hold yet.
func (s *Store) SyncBegin(dir string) (*SyncJob, error) {
	if _, err := s.db.Exec(syncSchema); err != nil {
		return nil, err
	}
	job := &SyncJob{dir: dir, device: s.device, at: map[string]int64{}}
	// Another folder is read from the top and is owed the whole log.
	was, err := s.Meta("sync_folder")
	if err != nil {
		return nil, err
	}
	if was != dir {
		if _, err := s.db.Exec(`DELETE FROM meta WHERE key LIKE 'sync_at:%' OR key = 'sync_exported'`); err != nil {
			return nil, err
		}
		if err := s.SetMeta("sync_folder", dir); err != nil {
			return nil, err
		}
	}
	v, err := s.Meta("sync_exported")
	if err != nil {
		return nil, err
	}
	job.from, _ = strconv.ParseInt(v, 10, 64)
	job.last = job.from
	// A database that was copied here writes out all it held at the time,
	// whoever made it: the machine it came from may never sync.
	if v, err = s.Meta("sync_adopted"); err != nil {
		return nil, err
	}
	adopted, _ := strconv.ParseInt(v, 10, 64)
	rs, err := s.db.Query(`SELECT seq, stamp, kind, row, field, value, parent, pos FROM ops
		WHERE seq > ? AND (stamp LIKE ? OR seq <= ?) ORDER BY seq`, job.from, "%-"+s.device, adopted)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var op wireOp
		if err := rs.Scan(&job.last, &op.Stamp, &op.Kind, &op.Row, &op.Field, &op.Value, &op.Parent, &op.Pos); err != nil {
			rs.Close()
			return nil, err
		}
		line, _ := json.Marshal(op)
		job.out = append(append(job.out, line...), '\n')
	}
	if err := rs.Close(); err != nil {
		return nil, err
	}
	seen, err := s.db.Query(`SELECT key, value FROM meta WHERE key LIKE 'sync_at:%'`)
	if err != nil {
		return nil, err
	}
	defer seen.Close()
	for seen.Next() {
		var key, value string
		if err := seen.Scan(&key, &value); err != nil {
			return nil, err
		}
		job.at[strings.TrimPrefix(key, "sync_at:")], _ = strconv.ParseInt(value, 10, 64)
	}
	return job, seen.Err()
}

// folder lets one pass at a time work in the folder, so the pass made on
// quit waits for one still under way and no file gets two writers.
var folder sync.Mutex

// Run writes this machine's operations to its file and reads the lines of
// every file that have not been read yet. It uses the folder only.
func (j *SyncJob) Run() *SyncRead {
	folder.Lock()
	defer folder.Unlock()
	r := &SyncRead{dir: j.dir, last: j.from, at: map[string]int64{}, bad: map[string]stuckAt{}}
	if r.err = os.MkdirAll(j.dir, 0o700); r.err != nil {
		return r
	}
	if err := j.write(r); err != nil {
		r.err = fmt.Errorf("writing to %s: %w", j.dir, err)
		return r
	}
	files, err := filepath.Glob(filepath.Join(j.dir, "*.ops"))
	if err != nil {
		r.err = err
		return r
	}
	for _, path := range files {
		// This machine's own file is read too. Its operations are already
		// here, so they are passed over, unless the database was restored
		// from a backup: then the file gives back what the backup lacks.
		name := filepath.Base(path)
		if name != j.device+".ops" {
			r.machines++
		}
		if r.err = j.read(path, name, r); r.err != nil {
			return r
		}
	}
	return r
}

// write appends the operations made here to this machine's file.
func (j *SyncJob) write(r *SyncRead) error {
	path := filepath.Join(j.dir, j.device+".ops")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if j.from > 0 {
			// The file is gone: it is owed everything, not only what is
			// new. The next pass writes it.
			r.last, r.lost = 0, true
			return nil
		}
	} else if err != nil {
		return err
	}
	if len(j.out) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := dropTornLine(f); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(j.out); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	r.last = j.last
	return nil
}

// dropTornLine cuts a last line that was never finished, left by a write
// the machine did not live to complete. Written after, it would become a
// whole line no machine could read. What it held was never noted as
// written, so it is in what is appended next.
func dropTornLine(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	end := info.Size()
	buf := make([]byte, 4096)
	for end > 0 {
		from := max(0, end-int64(len(buf)))
		n, err := f.ReadAt(buf[:end-from], from)
		if err != nil && err != io.EOF {
			return err
		}
		if i := bytes.LastIndexByte(buf[:n], '\n'); i >= 0 {
			end = from + int64(i) + 1
			break
		}
		end = from
	}
	if end == info.Size() {
		return nil
	}
	return f.Truncate(end)
}

// read collects the whole lines of one file past where it was last read.
func (j *SyncJob) read(path, name string, r *SyncRead) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// The place kept must still be the start of a line. If the file was
	// replaced by another it is read from the top; nothing is applied twice.
	off := j.at[name]
	if off > info.Size() {
		off = 0
	}
	if off > 0 {
		var b [1]byte
		if _, err := f.ReadAt(b[:], off-1); err != nil || b[0] != '\n' {
			off = 0
		}
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return err
	}
	in := bufio.NewReader(f)
	for {
		line, err := in.ReadBytes('\n')
		if err != nil {
			break // the end, or a last line still being copied in
		}
		var op wireOp
		if json.Unmarshal(line, &op) != nil || op.Stamp == "" {
			// A whole line that cannot be read. What follows waits for it,
			// since a file being copied in pieces may yet fill it in.
			r.bad[name] = stuckAt{off: off, line: lineAt(f, off)}
			break
		}
		r.ops = append(r.ops, op)
		off += int64(len(line))
	}
	r.at[name] = off
	return nil
}

// lineAt is the number of the line that starts at off.
func lineAt(f *os.File, off int64) int {
	n := 1
	buf := make([]byte, 64<<10)
	for at := int64(0); at < off; {
		want := min(int64(len(buf)), off-at)
		got, err := f.ReadAt(buf[:want], at)
		n += bytes.Count(buf[:got], []byte{'\n'})
		at += int64(got)
		if err != nil {
			break
		}
	}
	return n
}

// SyncEnd applies what a pass read: the operations in the other machines'
// files that are new here, along with any held back for want of a row.
func (s *Store) SyncEnd(r *SyncRead) (SyncReport, error) {
	if r.err != nil {
		return SyncReport{}, r.err
	}
	// A pass that was under way when the folder was changed is dropped.
	if dir, err := s.Meta("sync_folder"); err != nil || dir != r.dir {
		return SyncReport{}, err
	}
	if err := s.SetMeta("sync_exported", strconv.FormatInt(r.last, 10)); err != nil {
		return SyncReport{}, err
	}
	report := SyncReport{Machines: r.machines, Again: r.lost}
	changed, err := s.absorb(r.ops, r.at)
	if err != nil {
		return report, err
	}
	report.Changed = changed
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pending`).Scan(&report.Waiting); err != nil {
		return report, err
	}
	// A line is reported once it has been unreadable at the same place for
	// more than one pass.
	for name := range s.stuck {
		if _, still := r.bad[name]; !still {
			delete(s.stuck, name)
		}
	}
	for name, bad := range r.bad {
		if was := s.stuck[name]; was.off == bad.off {
			bad.passes = was.passes
		}
		bad.passes++
		s.stuck[name] = bad
		if bad.passes >= stuckAfter {
			report.Stuck = append(report.Stuck, Stuck{File: name, Line: bad.line})
		}
	}
	sort.Slice(report.Stuck, func(i, j int) bool { return report.Stuck[i].File < report.Stuck[j].File })
	return report, nil
}

// absorb applies ops read from the folder and notes how far each file was
// read, in one transaction.
func (s *Store) absorb(ops []wireOp, at map[string]int64) (int, error) {
	held, err := s.db.Query(`SELECT op FROM pending`)
	if err != nil {
		return 0, err
	}
	for held.Next() {
		var raw string
		var op wireOp
		if err := held.Scan(&raw); err != nil {
			held.Close()
			return 0, err
		}
		if json.Unmarshal([]byte(raw), &op) == nil {
			ops = append(ops, op)
		}
	}
	if err := held.Close(); err != nil {
		return 0, err
	}
	if len(ops) == 0 {
		return 0, nil
	}
	// Stamp order puts a row's insert before anything done to it.
	sort.SliceStable(ops, func(i, j int) bool { return ops[i].Stamp < ops[j].Stamp })

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM pending`); err != nil {
		return 0, err
	}
	var top int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM ops`).Scan(&top); err != nil {
		return 0, err
	}
	foreign := top != s.seq
	exists := func(id string) (bool, error) {
		var one int
		err := tx.QueryRow(`SELECT 1 FROM rows WHERE id = ?`, id).Scan(&one)
		if err == sql.ErrNoRows {
			return false, nil
		}
		return err == nil, err
	}
	changed, newest := 0, ""
	for _, op := range ops {
		newest = max(newest, op.Stamp)
		var known int
		if err := tx.QueryRow(`SELECT 1 FROM ops WHERE stamp = ?`, op.Stamp).Scan(&known); err == nil {
			continue // already applied
		} else if err != sql.ErrNoRows {
			return 0, err
		}

		// An operation on a row, or under a parent, that has not arrived
		// yet waits for it.
		hasRow, err := exists(op.Row)
		if err != nil {
			return 0, err
		}
		hasParent := op.Parent == ""
		if !hasParent {
			if hasParent, err = exists(op.Parent); err != nil {
				return 0, err
			}
		}
		kind := outline.Kind(op.Kind)
		if (kind != outline.Insert && !hasRow) || (kind != outline.Set && !hasParent) {
			raw, _ := json.Marshal(op)
			if _, err := tx.Exec(`INSERT OR IGNORE INTO pending(stamp, op) VALUES(?, ?)`, op.Stamp, string(raw)); err != nil {
				return 0, err
			}
			continue
		}

		ms := stampMS(op.Stamp)
		var res sql.Result
		switch kind {
		case outline.Insert:
			if hasRow {
				break // the Inbox, which every machine creates
			}
			res, err = tx.Exec(`INSERT INTO rows(id, parent, pos, moved, created, modified)
				VALUES(?, ?, ?, ?, ?, ?)`, op.Row, op.Parent, op.Pos, op.Stamp, ms, ms)
		case outline.Set:
			res, err = tx.Exec(`INSERT INTO fields(row, name, value, stamp) VALUES(?, ?, ?, ?)
				ON CONFLICT(row, name) DO UPDATE SET value = excluded.value, stamp = excluded.stamp
				WHERE excluded.stamp > fields.stamp`, op.Row, op.Field, op.Value, op.Stamp)
			if err == nil {
				_, err = tx.Exec(`UPDATE rows SET modified = max(modified, ?) WHERE id = ?`, ms, op.Row)
			}
		case outline.Move:
			res, err = tx.Exec(`UPDATE rows SET parent = ?, pos = ?, moved = ?, modified = max(modified, ?)
				WHERE id = ? AND moved < ?`, op.Parent, op.Pos, op.Stamp, ms, op.Row, op.Stamp)
		default:
			continue // written by a newer build; it stays in its file
		}
		if err != nil {
			return 0, err
		}
		if res != nil {
			if n, _ := res.RowsAffected(); n > 0 {
				changed++
			}
		}
		if _, err := tx.Exec(`INSERT INTO ops(stamp, kind, row, field, value, parent, pos)
			VALUES(?, ?, ?, ?, ?, ?, ?)`, op.Stamp, op.Kind, op.Row, op.Field, op.Value, op.Parent, op.Pos); err != nil {
			return 0, err
		}
	}
	for name, off := range at {
		if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, "sync_at:"+name, strconv.FormatInt(off, 10)); err != nil {
			return 0, err
		}
	}
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM ops`).Scan(&top); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	// The entries just written are this Store's own; one from another
	// window, found on the way, still calls for a fresh read.
	s.seq, s.foreign = top, s.foreign || foreign
	// Whatever is written here next is newer than everything just seen:
	// the clock takes the newest stamp's time and its counter.
	s.observe(newest)
	return changed, nil
}

// breakCycles makes the loaded rows a tree. Two machines can each move a
// row under the other's; every machine then holds the same loop, and breaks
// it the same way: the row in it that moved last is shown at the top level.
func breakCycles(byID map[string]*outline.Row, moved map[string]string) {
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var path []string
		at := map[string]int{}
		for cur := byID[id]; cur != nil; cur = byID[cur.Parent] {
			if from, loop := at[cur.ID]; loop {
				last := path[from]
				for _, member := range path[from:] {
					if moved[member] > moved[last] {
						last = member
					}
				}
				byID[last].Parent = ""
				break
			}
			at[cur.ID] = len(path)
			path = append(path, cur.ID)
		}
	}
}
