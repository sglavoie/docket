package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type enrollFixture struct {
	root        string
	home        string
	original    string
	workingCopy string
	exchange    string
	snapshots   string
}

func newEnrollFixture(t *testing.T) enrollFixture {
	t.Helper()
	root := t.TempDir()
	fixture := enrollFixture{
		root:        root,
		home:        filepath.Join(root, "home"),
		original:    filepath.Join(root, "original"),
		workingCopy: filepath.Join(root, "local", "synced"),
		exchange:    filepath.Join(root, "exchange"),
		snapshots:   filepath.Join(root, "snapshots"),
	}
	t.Setenv("HOME", fixture.home)
	if _, err := NewManager(ConfigPathForDataDir(fixture.original)).LoadOrCreateDomains(); err != nil {
		t.Fatalf("minting the original: %v", err)
	}
	return fixture
}

func (f enrollFixture) options() EnrollOptions {
	return EnrollOptions{
		Original:     f.original,
		WorkingCopy:  f.workingCopy,
		Exchange:     f.exchange,
		SnapshotRoot: f.snapshots,
		MachineHome:  f.home,
		Now:          func() time.Time { return time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC) },
	}
}

func digestTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(data)
		rel, _ := filepath.Rel(dir, path)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func requireSameDigests(t *testing.T, label string, want, got map[string]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: %d files, want %d", label, len(got), len(want))
	}
	for rel, digest := range want {
		if got[rel] != digest {
			t.Fatalf("%s: %s changed or is missing", label, rel)
		}
	}
}

// TestSyncEnrollInterruptedMigration stops enrollment at each of its durable
// steps and runs it again: the original is never changed, a stale staging
// directory or partial snapshot is discarded, a published working copy is
// resumed rather than copied twice, and the result is one complete enrolled
// working copy that is the active workspace.
func TestSyncEnrollInterruptedMigration(t *testing.T) {
	for _, stage := range []string{"snapshot", "staged", "published"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newEnrollFixture(t)
			before := digestTree(t, fixture.original)
			stale := filepath.Join(fixture.snapshots, "left-by-a-crash"+snapshotPartialExt)
			if err := os.MkdirAll(stale, DirPermissions); err != nil {
				t.Fatal(err)
			}

			interrupted := errors.New("interrupted at " + stage)
			enrollHook = func(s string) error {
				if s == stage {
					return interrupted
				}
				return nil
			}
			_, err := EnrollWorkspace(fixture.options())
			enrollHook = nil
			if !errors.Is(err, interrupted) {
				t.Fatalf("EnrollWorkspace = %v, want the interruption", err)
			}
			requireSameDigests(t, "the original after the interruption", before, digestTree(t, fixture.original))
			if stage != "published" {
				if _, statErr := os.Stat(fixture.workingCopy); !errors.Is(statErr, fs.ErrNotExist) {
					t.Fatalf("an interruption before publication left a working copy: %v", statErr)
				}
			}

			report, err := EnrollWorkspace(fixture.options())
			if err != nil {
				t.Fatalf("re-running enrollment: %v", err)
			}
			if report.Resumed != (stage == "published") {
				t.Fatalf("Resumed = %v after an interruption at %s", report.Resumed, stage)
			}
			requireSameDigests(t, "the original after resuming", before, digestTree(t, fixture.original))
			if _, statErr := os.Stat(fixture.workingCopy + enrollStagingSuffix); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("a staging directory survived: %v", statErr)
			}
			partials, _ := filepath.Glob(filepath.Join(fixture.snapshots, "*", "*"+snapshotPartialExt))
			if len(partials) > 0 {
				t.Fatalf("partial snapshots survived: %v", partials)
			}
			if _, statErr := os.Stat(filepath.Join(report.Snapshot, snapshotManifest)); statErr != nil {
				t.Fatalf("the recorded snapshot %s is not complete: %v", report.Snapshot, statErr)
			}
			catalog, err := NewDomainFileStore(fixture.workingCopy).LoadCatalogReadOnly()
			if err != nil || !catalog.Enrolled() || catalog.SchemaVersion != SyncEnrolledConfigVersion {
				t.Fatalf("working copy catalog = %+v, %v", catalog, err)
			}
			record, err := LoadEnrollmentRecord(fixture.workingCopy)
			if err != nil || record == nil || record.ExchangeFolder != fixture.exchange || record.Original != fixture.original {
				t.Fatalf("enrollment record = %+v, %v", record, err)
			}
			if pointer, ok := ReadActiveWorkspacePointer(); !ok || pointer != fixture.workingCopy {
				t.Fatalf("active workspace = %q, want the working copy", pointer)
			}
			if _, statErr := os.Stat(WorkspaceCatalogPath(fixture.workingCopy) + ".bak"); statErr == nil {
				t.Fatal("the working copy carries a catalog sidecar")
			}
		})
	}
}

// TestSyncEnrolledCatalogSchema holds the enrolled schema and the sync block
// to each other: the legacy enrolled schema 22 requires the block, the block
// requires schema 22 or later, and from schema 23 the block alone marks
// enrollment, so the current schema reads with or without it.
func TestSyncEnrolledCatalogSchema(t *testing.T) {
	enrolledAt := time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		catalog WorkspaceCatalog
		wantErr string
	}{
		{"current_without_sync", WorkspaceCatalog{SchemaVersion: CurrentConfigVersion}, ""},
		{"enrolled_with_sync", WorkspaceCatalog{SchemaVersion: SyncEnrolledConfigVersion, Sync: &CatalogSync{EnrolledAt: enrolledAt, Exchange: SyncExchangeFormat}}, ""},
		{"legacy_enrolled_with_sync", WorkspaceCatalog{SchemaVersion: V22ConfigSchemaVersion, Sync: &CatalogSync{EnrolledAt: enrolledAt, Exchange: SyncExchangeFormat}}, ""},
		{"legacy_enrolled_without_sync", WorkspaceCatalog{SchemaVersion: V22ConfigSchemaVersion}, "reserved for workspaces enrolled"},
		{"sync_before_enrolled_schema", WorkspaceCatalog{SchemaVersion: V21ConfigSchemaVersion, Sync: &CatalogSync{EnrolledAt: enrolledAt, Exchange: SyncExchangeFormat}}, "requires schema version"},
		{"unknown_exchange", WorkspaceCatalog{SchemaVersion: SyncEnrolledConfigVersion, Sync: &CatalogSync{EnrolledAt: enrolledAt, Exchange: "tt-sync/v9"}}, "not supported"},
		{"newer_than_supported", WorkspaceCatalog{SchemaVersion: SupportedConfigVersion + 1}, "newer than supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			catalog := tc.catalog
			catalog.Projects = []CatalogProject{}
			err := catalog.Validate()
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("Validate: %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("Validate = %v, want %q", err, tc.wantErr)
			}
		})
	}

	fixture := newEnrollFixture(t)
	if _, err := NewManager(ConfigPathForDataDir(fixture.original)).LoadOrCreateDomains(); err != nil {
		t.Fatal(err)
	}
	if schema, _, err := WorkspaceCatalogSchema(fixture.original); err != nil || schema != CurrentConfigVersion {
		t.Fatalf("a fresh workspace is at schema %d (%v), want %d", schema, err, CurrentConfigVersion)
	}
	if _, err := EnrollWorkspace(fixture.options()); err != nil {
		t.Fatal(err)
	}
	if schema, _, err := WorkspaceCatalogSchema(fixture.workingCopy); err != nil || schema != SyncEnrolledConfigVersion {
		t.Fatalf("the working copy reports schema %d (%v)", schema, err)
	}
	// Loading through the manager recovers and runs the activation chain; an
	// enrolled working copy passes through it unchanged.
	before := digestTree(t, fixture.workingCopy)
	if _, err := NewManager(ConfigPathForDataDir(fixture.workingCopy)).LoadOrCreateDomains(); err != nil {
		t.Fatalf("loading the working copy: %v", err)
	}
	after := digestTree(t, fixture.workingCopy)
	for rel, digest := range before {
		if !strings.HasSuffix(rel, ".lock") && after[rel] != digest {
			t.Fatalf("loading the working copy rewrote %s", rel)
		}
	}
}
