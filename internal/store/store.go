// Package store persists the outline in SQLite. Every change is appended to
// the ops table and folded into rows and fields in the same transaction, so
// the log is complete from the first write and the tables are a cache of it.
package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/sglavoie/docket/internal/outline"
)

const schema = `
CREATE TABLE IF NOT EXISTS meta(
	key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS rows(
	id TEXT PRIMARY KEY, parent TEXT NOT NULL, pos TEXT NOT NULL,
	moved TEXT NOT NULL, created INTEGER NOT NULL, modified INTEGER NOT NULL) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS fields(
	row TEXT NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, stamp TEXT NOT NULL,
	PRIMARY KEY(row, name)) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS ops(
	seq INTEGER PRIMARY KEY AUTOINCREMENT, stamp TEXT NOT NULL, kind TEXT NOT NULL,
	row TEXT NOT NULL, field TEXT NOT NULL DEFAULT '', value TEXT NOT NULL DEFAULT '',
	parent TEXT NOT NULL DEFAULT '', pos TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS folds(row TEXT PRIMARY KEY) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS ops_stamp ON ops(stamp);
`

const schemaVersion = "1"

// Store is one open database. It is used from a single goroutine.
type Store struct {
	db      *sql.DB
	path    string
	device  string
	renamed bool // this database was found somewhere new and took a new name
	lastMS  int64
	counter int

	// seq is the newest log entry this Store has read or written. An entry
	// past it was written by another window on the same database.
	seq     int64
	foreign bool

	// stuck is each sync file that holds a line this machine cannot read,
	// and how many passes in a row found it at the same place.
	stuck map[string]stuckAt
}

// keepBackups is how many daily snapshots are kept.
const keepBackups = 14

// NewID returns a fresh row identifier.
func NewID() string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Open opens the database at path, creating it and its directory if needed.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path, stuck: map[string]stuckAt{}}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	version, err := s.Meta("schema")
	if err != nil {
		return err
	}
	switch version {
	case "":
		if err := s.SetMeta("schema", schemaVersion); err != nil {
			return err
		}
	case schemaVersion:
	default:
		return fmt.Errorf("database schema %s is newer than this build understands (%s)", version, schemaVersion)
	}
	if err := s.identify(); err != nil {
		return err
	}
	// Resume the clock past the last stamp, so a wall clock that stepped
	// backwards never mints a stamp older than one already written.
	var last string
	// The newest stamp may be one received from another machine.
	var newest sql.NullString
	if err := s.db.QueryRow(`SELECT MAX(stamp) FROM ops`).Scan(&newest); err != nil {
		return err
	}
	last = newest.String
	s.observe(last)
	return s.db.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM ops`).Scan(&s.seq)
}

// home names where this database lives: this machine and this path. It is
// kept beside the device name, so a database that turns up somewhere else,
// copied or restored onto another machine, is known for a copy.
func (s *Store) home() string {
	path := s.path
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	sum := sha256.Sum256([]byte(machineID() + "\x00" + path))
	return hex.EncodeToString(sum[:8])
}

// identify gives this database the name it syncs under. Two databases must
// never share one: each would write the same sync file and pass over the
// other's lines as its own. So a database found away from where it was
// named, a copy, takes a new name. Everything it held is then written out
// again under that name, since the machine it came from may never sync.
func (s *Store) identify() error {
	var err error
	if s.device, err = s.Meta("device"); err != nil {
		return err
	}
	was, err := s.Meta("home")
	if err != nil {
		return err
	}
	home := s.home()
	switch {
	case s.device == "":
		s.device = NewID()[:8]
		if err := s.SetMeta("device", s.device); err != nil {
			return err
		}
		return s.SetMeta("home", home)
	case was == "": // named by a build that did not note where
		return s.SetMeta("home", home)
	case was == home:
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	device := NewID()[:8]
	var top int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM ops`).Scan(&top); err != nil {
		return err
	}
	for key, value := range map[string]string{
		"device":        device,
		"home":          home,
		"sync_adopted":  strconv.FormatInt(top, 10),
		"sync_exported": "0",
	} {
		if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
			return err
		}
	}
	// What the copy had read of each sync file is read again: applying is
	// idempotent, and the folder here may not be the one it knew.
	if _, err := tx.Exec(`DELETE FROM meta WHERE key LIKE 'sync_at:%' OR key LIKE 'sync_seen:%'`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.device, s.renamed = device, true
	return nil
}

// Renamed reports that this database was found on another machine, or at
// another path, than the one it was named on, and took a new sync name.
func (s *Store) Renamed() bool { return s.renamed }

// observe moves the clock up to a stamp, time and counter, so the next
// stamp issued here is newer than it.
func (s *Store) observe(stamp string) {
	parts := strings.Split(stamp, "-")
	if len(parts) != 3 {
		return
	}
	ms, _ := strconv.ParseInt(parts[0], 10, 64)
	c, _ := strconv.ParseInt(parts[1], 16, 32)
	if ms > s.lastMS || (ms == s.lastMS && int(c) > s.counter) {
		s.lastMS, s.counter = ms, int(c)
	}
}

// Stale reports that the log holds entries this Store did not write or
// read: another window on the same database has made changes.
func (s *Store) Stale() (bool, error) {
	var top int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM ops`).Scan(&top); err != nil {
		return false, err
	}
	return s.foreign || top != s.seq, nil
}

// Meta reads a setting kept on this machine; an unset one is empty. Settings
// are never logged and never synced.
func (s *Store) Meta(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// stamp returns the next hybrid logical clock value: wall milliseconds, a
// counter for ties, and this device. Stamps sort bytewise in issue order.
func (s *Store) stamp(now int64) string {
	if now > s.lastMS {
		s.lastMS, s.counter = now, 0
	} else {
		s.counter++
	}
	return fmt.Sprintf("%013d-%04x-%s", s.lastMS, s.counter, s.device)
}

// Commit records ops as one transaction.
func (s *Store) Commit(ops []outline.Op, now int64) error {
	if len(ops) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Another window may have written since this one last looked. Its
	// stamps are passed, so no two are alike and this write is the newest;
	// and its entries are noted, so the outline here is read again.
	var newest sql.NullString
	var top sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(stamp), MAX(seq) FROM ops`).Scan(&newest, &top); err != nil {
		return err
	}
	s.observe(newest.String)
	foreign := top.Int64 != s.seq
	last := top.Int64
	for _, op := range ops {
		stamp := s.stamp(now)
		res, err := tx.Exec(`INSERT INTO ops(stamp, kind, row, field, value, parent, pos)
			VALUES(?, ?, ?, ?, ?, ?, ?)`, stamp, string(op.Kind), op.Row, op.Field, op.Value, op.Parent, op.Pos)
		if err != nil {
			return err
		}
		if last, err = res.LastInsertId(); err != nil {
			return err
		}
		switch op.Kind {
		case outline.Insert:
			_, err = tx.Exec(`INSERT INTO rows(id, parent, pos, moved, created, modified)
				VALUES(?, ?, ?, ?, ?, ?)`, op.Row, op.Parent, op.Pos, stamp, now, now)
		case outline.Set:
			if _, err = tx.Exec(`INSERT INTO fields(row, name, value, stamp) VALUES(?, ?, ?, ?)
				ON CONFLICT(row, name) DO UPDATE SET value = excluded.value, stamp = excluded.stamp`,
				op.Row, op.Field, op.Value, stamp); err == nil {
				_, err = tx.Exec(`UPDATE rows SET modified = ? WHERE id = ?`, now, op.Row)
			}
		case outline.Move:
			_, err = tx.Exec(`UPDATE rows SET parent = ?, pos = ?, moved = ?, modified = ? WHERE id = ?`,
				op.Parent, op.Pos, stamp, now, op.Row)
		default:
			err = fmt.Errorf("unknown operation %q", op.Kind)
		}
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.seq, s.foreign = last, s.foreign || foreign
	return nil
}

// Load reads the whole outline into memory.
func (s *Store) Load() (*outline.Tree, error) {
	// Noted first: anything written while the rows are read is seen as new.
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM ops`).Scan(&s.seq); err != nil {
		return nil, err
	}
	s.foreign = false
	byID := map[string]*outline.Row{}
	var rows []*outline.Row
	moved := map[string]string{}
	rs, err := s.db.Query(`SELECT id, parent, pos, moved, created, modified FROM rows`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		r := &outline.Row{Fields: map[string]string{}}
		var stamp string
		if err := rs.Scan(&r.ID, &r.Parent, &r.Pos, &stamp, &r.Created, &r.Modified); err != nil {
			return nil, err
		}
		moved[r.ID] = stamp
		byID[r.ID] = r
		rows = append(rows, r)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	fs, err := s.db.Query(`SELECT row, name, value FROM fields WHERE value != ''`)
	if err != nil {
		return nil, err
	}
	defer fs.Close()
	for fs.Next() {
		var id, name, value string
		if err := fs.Scan(&id, &name, &value); err != nil {
			return nil, err
		}
		if r := byID[id]; r != nil {
			r.Fields[name] = value
		}
	}
	if err := fs.Err(); err != nil {
		return nil, err
	}
	breakCycles(byID, moved)
	return outline.NewTree(rows), nil
}

// Folds returns the rows folded on this machine. Folding is view state: it is
// never logged and never synced.
func (s *Store) Folds() (map[string]bool, error) {
	out := map[string]bool{}
	rs, err := s.db.Query(`SELECT row FROM folds`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rs.Err()
}

func (s *Store) SetFold(id string, folded bool) error {
	var err error
	if folded {
		_, err = s.db.Exec(`INSERT OR IGNORE INTO folds(row) VALUES(?)`, id)
	} else {
		_, err = s.db.Exec(`DELETE FROM folds WHERE row = ?`, id)
	}
	return err
}

// Backup writes a snapshot of the database for day into dir, unless that
// day already has one, and keeps the newest keep snapshots. A snapshot is a
// complete database: restoring is copying it over the live file.
func (s *Store) Backup(dir, day string, keep int) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, "docket-"+day+".db")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Written beside its final name and renamed, so a snapshot that
		// exists is always whole.
		tmp := path + ".tmp"
		os.Remove(tmp)
		if _, err := s.db.Exec(`VACUUM INTO ?`, tmp); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	old, err := filepath.Glob(filepath.Join(dir, "docket-*.db"))
	if err != nil {
		return err
	}
	sort.Strings(old)
	for i := 0; i < len(old)-keep; i++ {
		if err := os.Remove(old[i]); err != nil {
			return err
		}
	}
	return nil
}

// DailyBackup writes day's snapshot beside the database, in backups/.
func (s *Store) DailyBackup(day string) error {
	return s.Backup(filepath.Join(filepath.Dir(s.path), "backups"), day, keepBackups)
}

func (s *Store) Close() error { return s.db.Close() }
