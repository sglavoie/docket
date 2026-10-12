package synclog

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// statFixture is a workspace of one catalog and many notes, indexed as it
// stands, whose synced reads are counted.
type statFixture struct {
	t       *testing.T
	h       *History
	tracked map[string]string
	reads   []string
	settled time.Time
}

const statFixtureNotes = 300

func newStatFixture(t *testing.T) *statFixture {
	t.Helper()
	h, err := New(Config{
		DataDir:  t.TempDir(),
		DeviceID: "desk",
		Schemas:  map[string]int{SchemaCatalog: 1, SchemaDomainEnvelope: 1, SchemaTaskMarkdown: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := &statFixture{t: t, h: h, tracked: map[string]string{}, settled: time.Now().Add(-time.Hour)}
	if err := os.MkdirAll(h.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	f.write(catalogRel, `{"workspace_id":"ws","data":{"projects":[{"id":"p1","slug":"work"}]}}`, f.settled)
	for i := 0; i < statFixtureNotes; i++ {
		f.write(f.note(i), fmt.Sprintf("note %04d", i), f.settled)
	}
	for rel := range f.tracked {
		data, readErr := os.ReadFile(h.abs(rel))
		if readErr != nil {
			t.Fatal(readErr)
		}
		f.tracked[rel] = Digest(data)
	}
	previous := readSyncedFile
	readSyncedFile = func(abs string) ([]byte, error) {
		f.reads = append(f.reads, h.rel(abs))
		return previous(abs)
	}
	t.Cleanup(func() { readSyncedFile = previous })
	return f
}

func (f *statFixture) note(i int) string { return fmt.Sprintf("projects/work/notes/%04d.md", i) }

// write puts content at rel with the given modification time.
func (f *statFixture) write(rel, content string, mtime time.Time) {
	f.t.Helper()
	abs := f.h.abs(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chtimes(abs, mtime, mtime); err != nil {
		f.t.Fatal(err)
	}
	if _, known := f.tracked[rel]; !known {
		f.tracked[rel] = ""
	}
}

// observe runs one observation against the index the fixture started from and
// returns the paths it recorded as changed and the synced files it read.
func (f *statFixture) observe() (changed map[string]string, reads []string) {
	f.t.Helper()
	state := f.h.newState("ws")
	state.Indexed = true
	for rel, digest := range f.tracked {
		state.Tracked[rel] = digest
	}
	f.reads = nil
	plan, err := f.h.observe(state, newOverlay(f.h), nil, KindExternal, "local-txn")
	if err != nil {
		f.t.Fatal(err)
	}
	if len(plan.Held) > 0 {
		f.t.Fatalf("observation held %v", plan.Held)
	}
	changed = map[string]string{}
	if plan.Manifest != nil {
		for _, effect := range plan.Manifest.Effects {
			changed[effect.Path] = *effect.Blob
		}
	}
	reads = append([]string{}, f.reads...)
	sort.Strings(reads)
	return changed, reads
}

func TestObservationReadsNoSyncedFileOnAnIdleWalk(t *testing.T) {
	f := newStatFixture(t)
	changed, reads := f.observe()
	if len(changed) != 0 || len(reads) != statFixtureNotes+1 {
		t.Fatalf("first walk changed %d and read %d files, want 0 and %d", len(changed), len(reads), statFixtureNotes+1)
	}
	changed, reads = f.observe()
	if len(changed) != 0 || len(reads) != 0 {
		t.Fatalf("idle walk changed %d and read %v, want nothing", len(changed), reads)
	}
}

func TestObservationDetectsChangesTheStatCacheCouldHide(t *testing.T) {
	cases := []struct {
		name   string
		change func(f *statFixture, rel string)
	}{
		{"content of the same size", func(f *statFixture, rel string) {
			f.write(rel, "NOTE 0007", time.Now())
		}},
		{"size under the same mtime", func(f *statFixture, rel string) {
			f.write(rel, "note 0007, grown", f.settled)
		}},
		{"rewrite inside the racy window", func(f *statFixture, rel string) {
			// Digested in the tick it was written in, then rewritten in that
			// same tick: size and mtime both still match the cache.
			racy := time.Now()
			f.write(rel, "note 0007", racy)
			if changed, _ := f.observe(); len(changed) != 0 {
				f.t.Fatalf("an unchanged rewrite was recorded: %v", changed)
			}
			f.write(rel, "NOTE 0007", racy)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStatFixture(t)
			f.observe()
			rel := f.note(7)
			tc.change(f, rel)
			data, err := os.ReadFile(f.h.abs(rel))
			if err != nil {
				t.Fatal(err)
			}
			changed, reads := f.observe()
			if want := map[string]string{rel: Digest(data)}; !reflect.DeepEqual(changed, want) {
				t.Fatalf("observation recorded %v, want %v", changed, want)
			}
			if !reflect.DeepEqual(reads, []string{rel}) {
				t.Fatalf("observation read %v, want only %s", reads, rel)
			}
		})
	}
}

func TestDeletingTheStatCacheChangesNoObservation(t *testing.T) {
	f := newStatFixture(t)
	f.observe()
	f.write(f.note(3), "note three, rewritten", time.Now())
	f.write("projects/work/notes/added.md", "added", time.Now())
	if err := os.Remove(f.h.abs(f.note(9))); err != nil {
		t.Fatal(err)
	}
	observeDeleted := func() (map[string]string, []string) {
		t.Helper()
		state := f.h.newState("ws")
		state.Indexed = true
		for rel, digest := range f.tracked {
			if digest != "" {
				state.Tracked[rel] = digest
			}
		}
		plan, err := f.h.observe(state, newOverlay(f.h), nil, KindExternal, "local-txn")
		if err != nil || plan.Manifest == nil {
			t.Fatalf("observe: %v, manifest %v", err, plan.Manifest)
		}
		puts, deletes := map[string]string{}, []string{}
		for _, effect := range plan.Manifest.Effects {
			if effect.Op == OpDelete {
				deletes = append(deletes, effect.Path)
				continue
			}
			puts[effect.Path] = *effect.Blob
		}
		return puts, deletes
	}
	cachedPuts, cachedDeletes := observeDeleted()
	if len(cachedPuts) != 2 || !reflect.DeepEqual(cachedDeletes, []string{f.note(9)}) {
		t.Fatalf("observation recorded puts %v and deletes %v", cachedPuts, cachedDeletes)
	}
	for _, damage := range []func() error{
		func() error { return os.Remove(f.h.StatCachePath()) },
		func() error { return os.WriteFile(f.h.StatCachePath(), []byte("{not json"), 0o644) },
	} {
		if err := damage(); err != nil {
			t.Fatal(err)
		}
		f.reads = nil
		puts, deletes := observeDeleted()
		if !reflect.DeepEqual(puts, cachedPuts) || !reflect.DeepEqual(deletes, cachedDeletes) {
			t.Fatalf("without a cache the observation recorded %v and %v, want %v and %v", puts, deletes, cachedPuts, cachedDeletes)
		}
		if len(f.reads) != statFixtureNotes+1 {
			t.Fatalf("a walk without a cache read %d files, want all %d", len(f.reads), statFixtureNotes+1)
		}
		if _, err := os.Stat(f.h.StatCachePath()); err != nil {
			t.Fatalf("the walk did not rewrite its cache: %v", err)
		}
	}
}
