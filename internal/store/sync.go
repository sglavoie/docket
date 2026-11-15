package store

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

const syncSchema = `
CREATE TABLE IF NOT EXISTS pending(stamp TEXT PRIMARY KEY, op TEXT NOT NULL) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS ops_stamp ON ops(stamp);
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

// Sync writes this machine's new operations to dir and applies the other
// machines' new ones. It reports how many remote operations changed
// something here.
func (s *Store) Sync(dir string) (int, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(syncSchema); err != nil {
		return 0, err
	}
	if err := s.export(dir); err != nil {
		return 0, fmt.Errorf("writing to %s: %w", dir, err)
	}
	return s.absorb(dir)
}

// export appends the operations made here that the file does not hold yet.
func (s *Store) export(dir string) error {
	path := filepath.Join(dir, s.device+".ops")
	from := int64(0)
	if _, err := os.Stat(path); err == nil {
		v, err := s.Meta("sync_exported")
		if err != nil {
			return err
		}
		from, _ = strconv.ParseInt(v, 10, 64)
	} else if !os.IsNotExist(err) {
		return err
	}
	rs, err := s.db.Query(`SELECT seq, stamp, kind, row, field, value, parent, pos FROM ops
		WHERE seq > ? AND stamp LIKE ? ORDER BY seq`, from, "%-"+s.device)
	if err != nil {
		return err
	}
	var out []byte
	last := from
	for rs.Next() {
		var op wireOp
		if err := rs.Scan(&last, &op.Stamp, &op.Kind, &op.Row, &op.Field, &op.Value, &op.Parent, &op.Pos); err != nil {
			rs.Close()
			return err
		}
		line, _ := json.Marshal(op)
		out = append(append(out, line...), '\n')
	}
	if err := rs.Close(); err != nil {
		return err
	}
	if len(out) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(out); err != nil {
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
	return s.SetMeta("sync_exported", strconv.FormatInt(last, 10))
}

// absorb applies the operations in the other machines' files that have not
// been read yet, along with any held back for want of a row.
func (s *Store) absorb(dir string) (int, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.ops"))
	if err != nil {
		return 0, err
	}
	var ops []wireOp
	seen := map[string]int{} // lines read from each file after this pass
	for _, path := range files {
		// This machine's own file is read too. Its operations are already
		// here, so they are passed over, unless the database was restored
		// from a backup: then the file gives back what the backup lacks.
		name := filepath.Base(path)
		v, err := s.Meta("sync_seen:" + name)
		if err != nil {
			return 0, err
		}
		skip, _ := strconv.Atoi(v)
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		r := bufio.NewReader(f)
		n := 0
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				break // the end, or a last line still being copied in
			}
			n++
			if n <= skip {
				continue
			}
			var op wireOp
			if json.Unmarshal([]byte(line), &op) != nil || op.Stamp == "" {
				n--
				break
			}
			ops = append(ops, op)
		}
		f.Close()
		seen[name] = n
	}

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
	for name, n := range seen {
		if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, "sync_seen:"+name, strconv.Itoa(n)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	// Whatever is written here next is newer than everything just seen:
	// the clock takes the newest stamp's time and its counter.
	if parts := strings.Split(newest, "-"); len(parts) == 3 {
		ms, _ := strconv.ParseInt(parts[0], 10, 64)
		c, _ := strconv.ParseInt(parts[1], 16, 32)
		if ms > s.lastMS || (ms == s.lastMS && int(c) > s.counter) {
			s.lastMS, s.counter = ms, int(c)
		}
	}
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
