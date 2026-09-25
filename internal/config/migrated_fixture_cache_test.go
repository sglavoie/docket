package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// migrateFixtureTo names the public entry point each cached schema is reached
// through. Every one of them runs its predecessors first, so reaching schema N
// from a cached schema N-1 runs exactly the step that boundary adds.
var migrateFixtureTo = map[int]func(*Manager) error{
	7:  (*Manager).MigrateWorkspaceToV7,
	8:  (*Manager).MigrateWorkspaceToV8,
	9:  (*Manager).MigrateWorkspaceToV9,
	10: (*Manager).MigrateWorkspaceToV10,
	11: (*Manager).MigrateWorkspaceToV11,
	12: (*Manager).MigrateWorkspaceToV12,
	13: (*Manager).MigrateWorkspaceToV13,
	14: (*Manager).MigrateWorkspaceToV14,
	15: (*Manager).MigrateWorkspaceToV15,
	16: (*Manager).MigrateWorkspaceToV16,
	17: (*Manager).MigrateWorkspaceToV17,
	18: (*Manager).MigrateWorkspaceToV18,
	19: (*Manager).MigrateWorkspaceToV19,
	20: (*Manager).MigrateWorkspaceToV20,
	21: (*Manager).MigrateWorkspaceToV21,
}

// preScopeV7FixtureSchema is the schema copyPreScopeV7Fixture leaves behind.
const preScopeV7FixtureSchema = 6

// migratedFixture is one published workspace and the HOME holding its
// machine-local state, frozen right after a migration returned.
type migratedFixture struct {
	root string
	home string
}

// migratedFixtures holds every schema this test binary has already migrated the
// pre-scope fixture to. A migration test's setup otherwise replays the whole
// chain from schema 6 for every case, which is most of this package's time.
// Nothing in either tree names its own path, so a copy is the same workspace
// the chain would have produced at a new location.
var migratedFixtures struct {
	sync.Mutex
	dir     string
	byKey   map[string]migratedFixture
	counter int
}

// copyMigratedFixture returns a private copy of the pre-scope v7 fixture already
// migrated to schema, with HOME pointed at a private copy of its machine-local
// state, exactly as copyPreScopeV7Fixture followed by MigrateWorkspaceToV<schema>
// would leave them.
func copyMigratedFixture(t *testing.T, schema int) string {
	t.Helper()
	migratedFixtures.Lock()
	fixture := cachedMigratedFixture(t, schema)
	migratedFixtures.Unlock()
	return restoreMigratedFixture(t, fixture)
}

// cachedMigratedFixture reaches schema from the cached predecessor, one step at
// a time, so each boundary runs once per binary and time zone. The v11 step
// reads the local zone, which some cases pin, so the zone is part of the key.
// The caller holds migratedFixtures' lock.
func cachedMigratedFixture(t *testing.T, schema int) migratedFixture {
	t.Helper()
	key := fmt.Sprintf("%s/%d", time.Local, schema)
	if fixture, ok := migratedFixtures.byKey[key]; ok {
		return fixture
	}
	var root string
	switch migrate, ok := migrateFixtureTo[schema]; {
	case schema == preScopeV7FixtureSchema:
		root = copyPreScopeV7Fixture(t).DataDir
	case !ok:
		t.Fatalf("no cached fixture migration reaches schema %d", schema)
	default:
		root = restoreMigratedFixture(t, cachedMigratedFixture(t, schema-1))
		if err := migrate(NewManager(ConfigPathForDataDir(root))); err != nil {
			t.Fatalf("publish schema-%d fixture: %v", schema, err)
		}
	}

	if migratedFixtures.dir == "" {
		dir, err := os.MkdirTemp("", "tt-config-fixtures-*")
		if err != nil {
			t.Fatal(err)
		}
		migratedFixtures.dir = dir
		migratedFixtures.byKey = make(map[string]migratedFixture)
	}
	migratedFixtures.counter++
	frozen := filepath.Join(migratedFixtures.dir, fmt.Sprint(migratedFixtures.counter))
	fixture := migratedFixture{root: filepath.Join(frozen, "workspace"), home: filepath.Join(frozen, "home")}
	copyFixtureTree(t, root, fixture.root)
	copyFixtureTree(t, os.Getenv("HOME"), fixture.home)
	migratedFixtures.byKey[key] = fixture
	return fixture
}

func restoreMigratedFixture(t *testing.T, fixture migratedFixture) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	copyFixtureTree(t, fixture.home, home)
	root := filepath.Join(t.TempDir(), "workspace")
	copyFixtureTree(t, fixture.root, root)
	return root
}

// removeMigratedFixtures deletes the frozen trees once the binary is done.
func removeMigratedFixtures() {
	migratedFixtures.Lock()
	defer migratedFixtures.Unlock()
	if migratedFixtures.dir != "" {
		_ = os.RemoveAll(migratedFixtures.dir)
	}
}

// copyFixtureTree copies src onto dst keeping every permission bit and every
// empty directory, since both are part of what a migration published.
func copyFixtureTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
				return err
			}
			return os.Chmod(target, info.Mode().Perm())
		case entry.Type().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
				return err
			}
			return os.Chmod(target, info.Mode().Perm())
		default:
			return fmt.Errorf("%s: fixture trees hold only files and directories", path)
		}
	})
	if err != nil {
		t.Fatalf("copy fixture %s: %v", src, err)
	}
}
