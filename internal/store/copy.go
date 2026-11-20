package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// CopyJob is a snapshot of the database on its way to a folder off this
// machine, such as one iCloud Drive carries. The snapshot is taken on this
// machine first, so the database is held only as long as that takes, and the
// slow folder is worked in afterwards.
type CopyJob struct {
	s    *Store
	dir  string // this machine's folder among the copies
	day  string
	keep int
	last int64 // the newest log entry the copy holds
}

// CopyDone is what a finished copy did.
type CopyDone struct {
	Err  error
	dir  string
	last int64
}

// copying lets one copy at a time work in the folder, so the copy made on
// quit waits for one still under way.
var copying sync.Mutex

// CopyBegin readies a copy of the database into dir, under a folder named
// for this database's device, as docket-day.db. It returns no job when
// nothing has changed since the last copy into dir, unless force is set.
func (s *Store) CopyBegin(dir, day string, keep int, force bool) (*CopyJob, error) {
	var last int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM ops`).Scan(&last); err != nil {
		return nil, err
	}
	if !force {
		was, err := s.Meta("copy_last")
		if err != nil {
			return nil, err
		}
		if seq, at, ok := strings.Cut(was, " "); ok && at == dir && seq == strconv.FormatInt(last, 10) {
			return nil, nil
		}
	}
	return &CopyJob{s: s, dir: filepath.Join(dir, s.device), day: day, keep: keep, last: last}, nil
}

// Run takes the snapshot and writes it into the folder, replacing the day's
// copy, and keeps the newest keep copies there. It may run away from the
// goroutine the store is used from: it reads the database through its own
// statement and touches nothing else the store holds.
func (j *CopyJob) Run() *CopyDone {
	copying.Lock()
	defer copying.Unlock()
	done := &CopyDone{dir: filepath.Dir(j.dir), last: j.last}
	if done.Err = j.run(); done.Err != nil {
		done.Err = fmt.Errorf("copying to %s: %w", j.dir, done.Err)
	}
	return done
}

func (j *CopyJob) run() error {
	// The snapshot is made beside the database, where writing is quick.
	snap, err := os.CreateTemp(filepath.Dir(j.s.path), ".docket-copy-*.db")
	if err != nil {
		return err
	}
	name := snap.Name()
	snap.Close()
	defer os.Remove(name)
	if err := os.Remove(name); err != nil {
		return err
	}
	if _, err := j.s.db.Exec(`VACUUM INTO ?`, name); err != nil {
		return err
	}
	// The folder is made if its parent is there, and the device's folder
	// in it, but nothing further up: a folder misnamed, or a drive gone,
	// fails here rather than becoming a local tree no one looks in.
	for _, d := range []string{filepath.Dir(j.dir), j.dir} {
		if err := os.Mkdir(d, 0o700); err != nil && !os.IsExist(err) {
			return err
		}
	}
	// Written beside its final name and renamed, so a copy that exists is
	// always whole and a syncing tool never carries half of one.
	out, err := os.CreateTemp(j.dir, ".docket-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	in, err := os.Open(name)
	if err != nil {
		out.Close()
		return err
	}
	_, err = io.Copy(out, in)
	in.Close()
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(out.Name(), filepath.Join(j.dir, "docket-"+j.day+".db")); err != nil {
		return err
	}
	old, err := filepath.Glob(filepath.Join(j.dir, "docket-*.db"))
	if err != nil {
		return err
	}
	sort.Strings(old)
	for i := 0; i < len(old)-j.keep; i++ {
		if err := os.Remove(old[i]); err != nil {
			return err
		}
	}
	return nil
}

// CopyEnd records a copy that was written, so the next one is skipped until
// something changes.
func (s *Store) CopyEnd(done *CopyDone) error {
	if done.Err != nil {
		return done.Err
	}
	return s.SetMeta("copy_last", strconv.FormatInt(done.last, 10)+" "+done.dir)
}

// Copy writes a copy whole, waiting for the folder: the copy made on quit.
func (s *Store) Copy(dir, day string, keep int) error {
	job, err := s.CopyBegin(dir, day, keep, false)
	if err != nil || job == nil {
		return err
	}
	return s.CopyEnd(job.Run())
}
