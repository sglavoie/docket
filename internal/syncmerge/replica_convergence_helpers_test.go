package syncmerge_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/synclog"
)

// outgoing is every transaction a device authored, oldest first.
func (r *replica) outgoing() []*synclog.Manifest {
	r.t.Helper()
	pending, err := r.history.Pending()
	if err != nil {
		r.t.Fatalf("%s: pending: %v", r.device, err)
	}
	return pending
}

// receive applies every given transaction this device has not seen, retrying
// any whose parents or earlier sequence numbers have not arrived yet, so the
// order given is the delivery order wherever causality allows it.
func (r *replica) receive(source synclog.BlobSource, manifests ...*synclog.Manifest) {
	r.t.Helper()
	pending := append([]*synclog.Manifest{}, manifests...)
	for len(pending) > 0 {
		var waiting []*synclog.Manifest
		for _, m := range pending {
			_, err := r.svc.CommitIncoming(m, source)
			switch {
			case err == nil,
				errors.Is(err, synclog.ErrOwnTransaction),
				errors.Is(err, synclog.ErrAlreadyApplied):
			case errors.Is(err, synclog.ErrIncomplete):
				waiting = append(waiting, m)
			default:
				r.t.Fatalf("%s: receive %s seq %d: %v", r.device, m.DeviceID, m.DeviceSeq, err)
			}
		}
		if len(waiting) == len(pending) {
			r.t.Fatalf("%s: %d transactions can never be applied", r.device, len(waiting))
		}
		pending = waiting
	}
}

// pull applies everything the other devices authored.
func (r *replica) pull(sources ...*replica) {
	r.t.Helper()
	for _, source := range sources {
		r.receive(source.history, source.outgoing()...)
	}
}

// pool is a blob source over every device's history: the exchange folder.
type pool []*replica

func (p pool) Blob(digest string) ([]byte, error) {
	for _, r := range p {
		if data, err := r.history.Blob(digest); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("blob %s is not in the exchange", digest)
}

// everything is every transaction the cluster's devices authored.
func (c *cluster) everything() ([]*synclog.Manifest, pool) {
	var all []*synclog.Manifest
	var sources pool
	names := make([]string, 0, len(c.devices))
	for name := range c.devices {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		all = append(all, c.devices[name].outgoing()...)
		sources = append(sources, c.devices[name])
	}
	return all, sources
}

// settle delivers every authored transaction to every device, then to
// observers that start from genesis and receive them in several seeded
// orders. It returns every replica, so the caller can assert they agree.
func (c *cluster) settle(orders int) []*replica {
	c.t.Helper()
	all, sources := c.everything()
	var replicas []*replica
	names := make([]string, 0, len(c.devices))
	for name := range c.devices {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c.devices[name].receive(sources, all...)
		replicas = append(replicas, c.devices[name])
	}
	for seed := 0; seed < orders; seed++ {
		observer := c.clone(fmt.Sprintf("observer-%d", seed))
		order := append([]*synclog.Manifest{}, all...)
		rng := rand.New(rand.NewSource(int64(seed)))
		if seed == 1 {
			for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
				order[i], order[j] = order[j], order[i]
			}
		} else if seed > 1 {
			rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		}
		observer.receive(sources, order...)
		replicas = append(replicas, observer)
	}
	return replicas
}

// syncedFiles is the digest of every synced file, by workspace path.
func syncedFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if !isSyncedPath(rel) {
			// A file outside the synced layout is not workspace content.
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		out[rel] = synclog.Digest(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// isSyncedPath reports whether rel is synced workspace content.
func isSyncedPath(rel string) bool {
	_, _, local, err := synclog.Classify(rel)
	return err == nil && !local
}

// requireConverged holds every replica to identical synced content, identical
// conflict records, a valid workspace, and recoverable competing versions. It
// returns the agreed conflicts.
func requireConverged(t *testing.T, replicas []*replica) []synclog.Conflict {
	t.Helper()
	first := replicas[0]
	want := syncedFiles(t, first.root)
	wantConflicts := conflictsJSON(t, first)
	for _, r := range replicas[1:] {
		got := syncedFiles(t, r.root)
		for rel, digest := range want {
			if got[rel] != digest {
				t.Fatalf("%s and %s disagree on %s:\n%s\n---\n%s", first.device, r.device, rel, readOr(first, rel), readOr(r, rel))
			}
		}
		for rel := range got {
			if _, ok := want[rel]; !ok {
				t.Fatalf("%s holds %s, %s does not", r.device, rel, first.device)
			}
		}
		if gotConflicts := conflictsJSON(t, r); gotConflicts != wantConflicts {
			t.Fatalf("%s and %s derive different conflicts:\n%s\n---\n%s", first.device, r.device, wantConflicts, gotConflicts)
		}
	}
	for _, r := range replicas {
		requireValidWorkspace(t, r)
	}
	conflicts, err := first.svc.SyncConflicts()
	if err != nil {
		t.Fatal(err)
	}
	for _, conflict := range conflicts {
		for _, candidate := range conflict.Candidates {
			if candidate.Blob == nil {
				continue
			}
			for _, r := range replicas {
				if _, blobErr := r.history.Blob(*candidate.Blob); blobErr != nil {
					t.Fatalf("%s cannot recover competing version %s of %s: %v", r.device, *candidate.Blob, conflict.Key, blobErr)
				}
			}
		}
	}
	return conflicts
}

func readOr(r *replica, rel string) string {
	data, err := os.ReadFile(r.path(rel))
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func conflictsJSON(t *testing.T, r *replica) string {
	t.Helper()
	conflicts, err := r.svc.SyncConflicts()
	if err != nil {
		t.Fatalf("%s: conflicts: %v", r.device, err)
	}
	data, err := json.MarshalIndent(conflicts, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// requireValidWorkspace reads the replica through the live readers: the
// catalog and planning validate, and every project the catalog names has a
// tasks.md that parses and passes the project audit with unique short ids.
func requireValidWorkspace(t *testing.T, r *replica) {
	t.Helper()
	catalog, _ := r.catalog()
	if err := catalog.Validate(); err != nil {
		t.Fatalf("%s: catalog invalid: %v", r.device, err)
	}
	if err := r.planning().Validate(); err != nil {
		t.Fatalf("%s: planning invalid: %v", r.device, err)
	}
	for _, project := range catalog.Projects {
		rel := "projects/" + project.Slug + "/tasks.md"
		if !r.exists(rel) {
			continue
		}
		doc := r.doc(project.Slug)
		if err := models.AuditProject(&models.Project{Name: doc.ProjectName, Tasks: doc.Tasks, Comments: doc.Comments}); err != nil {
			t.Fatalf("%s: %s fails the audit: %v", r.device, rel, err)
		}
		seen := map[int]string{}
		for _, task := range doc.Tasks {
			if other, clash := seen[task.ShortID]; clash {
				t.Fatalf("%s: %s gives short id %d to %s and %s", r.device, rel, task.ShortID, other, task.ID)
			}
			seen[task.ShortID] = task.ID
		}
	}
}

// conflictFor returns the conflicts whose key or details mention needle.
func conflictsAbout(conflicts []synclog.Conflict, needle string) []synclog.Conflict {
	var out []synclog.Conflict
	for _, conflict := range conflicts {
		data, _ := json.Marshal(conflict)
		if strings.Contains(string(data), needle) {
			out = append(out, conflict)
		}
	}
	return out
}
