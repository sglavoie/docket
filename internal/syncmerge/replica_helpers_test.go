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
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/syncmerge"
	"github.com/sglavoie/tt/internal/transaction"
)

// The replicas below start from one copy of the representative compatibility
// fixture: a real workspace whose catalog, planning, project documents, notes
// and attachments every live reader accepts.
const fixtureDir = "../compat/testdata/workspaces/representative"

const (
	atlasID   = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01"
	homeID    = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a02"
	shipBeta  = "0a7e5d10-2c3b-4f6a-8e9d-000000000101"
	notes     = "0a7e5d10-2c3b-4f6a-8e9d-000000000102"
	proofread = "0a7e5d10-2c3b-4f6a-8e9d-000000000103"
	awaitCert = "0a7e5d10-2c3b-4f6a-8e9d-000000000105"
	localize  = "0a7e5d10-2c3b-4f6a-8e9d-000000000106"
	darkIcons = "0a7e5d10-2c3b-4f6a-8e9d-000000000107"
	homeTask1 = "0a7e5d10-2c3b-4f6a-8e9d-000000000201"
	homeTask2 = "0a7e5d10-2c3b-4f6a-8e9d-000000000202"
)

var replicaSchemas = map[string]int{
	synclog.SchemaDomainEnvelope: config.CurrentDomainSchemaVersion,
	synclog.SchemaCatalog:        config.CurrentConfigVersion,
	synclog.SchemaTaskMarkdown:   storage.CurrentTaskSchemaVersion,
}

// replica is one device: a workspace, its sync history, and a commit service
// that records every local commit and reconciles every incoming one.
type replica struct {
	t       *testing.T
	device  string
	root    string
	history *synclog.History
	svc     *storage.CommitService
}

// cluster is a set of devices that started from one genesis copy, and the
// genesis they can clone further observers from.
type cluster struct {
	t       *testing.T
	genesis string
	devices map[string]*replica
}

func newCluster(t *testing.T, devices ...string) *cluster {
	t.Helper()
	genesis := filepath.Join(t.TempDir(), "genesis")
	copyTree(t, fixtureDir, genesis)
	c := &cluster{t: t, genesis: genesis, devices: map[string]*replica{}}
	for _, device := range devices {
		c.devices[device] = c.clone(device)
	}
	return c
}

// clone opens a fresh device on a copy of the genesis workspace.
func (c *cluster) clone(device string) *replica {
	c.t.Helper()
	root := filepath.Join(c.t.TempDir(), device)
	copyTree(c.t, c.genesis, root)
	history, err := synclog.New(synclog.Config{
		DataDir:  root,
		DeviceID: device,
		Schemas:  replicaSchemas,
		Now:      func() time.Time { return time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		c.t.Fatalf("synclog.New: %v", err)
	}
	ledger := storage.NewWorkspaceLedger(filepath.Join(root, ".tt", "undo"), []string{root})
	svc := storage.NewCommitServiceWithLedger(filepath.Join(root, ".tt", "transactions"), []string{root}, ledger).
		WithSyncHistory(history).
		WithSyncMerger(syncmerge.Merger{})
	return &replica{t: c.t, device: device, root: root, history: history, svc: svc}
}

func (c *cluster) r(device string) *replica { return c.devices[device] }

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", from, err)
	}
}

func (r *replica) path(rel string) string { return filepath.Join(r.root, filepath.FromSlash(rel)) }

func (r *replica) read(rel string) []byte {
	r.t.Helper()
	data, err := os.ReadFile(r.path(rel))
	if err != nil {
		r.t.Fatalf("%s: read %s: %v", r.device, rel, err)
	}
	return data
}

func (r *replica) exists(rel string) bool {
	_, err := os.Stat(r.path(rel))
	return err == nil
}

func (r *replica) commit(artifacts ...storage.ArtifactChange) {
	r.t.Helper()
	if _, err := r.svc.CommitArtifacts(artifacts); err != nil {
		r.t.Fatalf("%s: commit: %v", r.device, err)
	}
}

func (r *replica) put(rel string, data []byte) storage.ArtifactChange {
	return storage.ArtifactChange{Path: r.path(rel), Data: data, Mode: storage.FilePermissions}
}

// doc reads and parses a project's tasks.md.
func (r *replica) doc(slug string) *storage.ProjectDocument {
	r.t.Helper()
	doc, err := storage.ParseProjectDocument(string(r.read("projects/" + slug + "/tasks.md")))
	if err != nil {
		r.t.Fatalf("%s: parse %s: %v", r.device, slug, err)
	}
	return doc
}

func (r *replica) docArtifact(slug string, doc *storage.ProjectDocument) storage.ArtifactChange {
	r.t.Helper()
	content, err := doc.WriteCurrent()
	if err != nil {
		r.t.Fatalf("%s: render %s: %v", r.device, slug, err)
	}
	return r.put("projects/"+slug+"/tasks.md", []byte(content))
}

// editDoc commits one change to a project document.
func (r *replica) editDoc(slug string, edit func(doc *storage.ProjectDocument)) {
	r.t.Helper()
	doc := r.doc(slug)
	edit(doc)
	r.commit(r.docArtifact(slug, doc))
}

func findTask(t *testing.T, doc *storage.ProjectDocument, id string) *models.Task {
	t.Helper()
	for _, task := range doc.Tasks {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("task %s is not in %q", id, doc.ProjectName)
	return nil
}

func hasTask(doc *storage.ProjectDocument, id string) bool {
	for _, task := range doc.Tasks {
		if task.ID == id {
			return true
		}
	}
	return false
}

func dropTask(doc *storage.ProjectDocument, id string) {
	kept := doc.Tasks[:0]
	for _, task := range doc.Tasks {
		if task.ID != id {
			kept = append(kept, task)
		}
	}
	doc.Tasks = kept
}

// newTask appends a root task minted the way a device mints one: the next
// free short id against the document it has.
func newTask(doc *storage.ProjectDocument, id, title string, parent *string) *models.Task {
	template := *doc.Tasks[len(doc.Tasks)-1]
	task := &template
	task.ID = id
	task.Title = title
	task.ParentID = parent
	task.Dependencies = nil
	task.Pinned = false
	task.PinnedAt = nil
	maxShort := 0
	for _, existing := range doc.Tasks {
		if existing.ShortID > maxShort {
			maxShort = existing.ShortID
		}
	}
	task.ShortID = maxShort + 1
	doc.Tasks = append(doc.Tasks, task)
	return task
}

type envelope struct {
	SchemaVersion int             `json:"schema_version"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data"`
}

func decodeDomain(t *testing.T, data []byte, payload any) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if err := json.Unmarshal(env.Data, payload); err != nil {
		t.Fatalf("decode %s payload: %v", env.Kind, err)
	}
	return env
}

func encodeDomain(t *testing.T, env envelope, payload any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	env.Data = raw
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (r *replica) catalog() (*config.WorkspaceCatalog, envelope) {
	r.t.Helper()
	catalog := &config.WorkspaceCatalog{}
	env := decodeDomain(r.t, r.read(".tt/workspace.json"), catalog)
	return catalog, env
}

func (r *replica) catalogArtifact(edit func(*config.WorkspaceCatalog)) storage.ArtifactChange {
	r.t.Helper()
	catalog, env := r.catalog()
	edit(catalog)
	return r.put(".tt/workspace.json", encodeDomain(r.t, env, catalog))
}

func (r *replica) planning() *config.PlanningState {
	r.t.Helper()
	planning := &config.PlanningState{}
	decodeDomain(r.t, r.read(".tt/planning.json"), planning)
	return planning
}

func (r *replica) planningArtifact(edit func(*config.PlanningState)) storage.ArtifactChange {
	r.t.Helper()
	planning := &config.PlanningState{}
	env := decodeDomain(r.t, r.read(".tt/planning.json"), planning)
	edit(planning)
	return r.put(".tt/planning.json", encodeDomain(r.t, env, planning))
}

// renameProject moves a project's directory and rewrites its catalog slug in
// one transaction, as the project editor does.
func (r *replica) renameProject(oldSlug, newSlug string) {
	r.t.Helper()
	catalog := r.catalogArtifact(func(c *config.WorkspaceCatalog) {
		for i := range c.Projects {
			if c.Projects[i].Slug == oldSlug {
				c.Projects[i].Slug = newSlug
			}
		}
	})
	_, err := r.svc.CommitOpsTransaction(storage.OpsCommitRequest{
		Ops: []transaction.Op{{
			Kind:   transaction.OpMove,
			Source: r.path("projects/" + oldSlug),
			Target: r.path("projects/" + newSlug),
		}},
		Artifacts:  []storage.ArtifactChange{catalog},
		UndoAction: "Edit Project",
	})
	if err != nil {
		r.t.Fatalf("%s: rename %s: %v", r.device, oldSlug, err)
	}
}

// deleteProject removes every file of a project and its catalog and planning
// entries in one transaction, as project deletion does.
func (r *replica) deleteProject(slug, projectID string) {
	r.t.Helper()
	var artifacts []storage.ArtifactChange
	err := filepath.WalkDir(r.path("projects/"+slug), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, ".lock") {
			return err
		}
		artifacts = append(artifacts, storage.ArtifactChange{Path: path, Delete: true})
		return nil
	})
	if err != nil {
		r.t.Fatal(err)
	}
	artifacts = append(artifacts,
		r.catalogArtifact(func(c *config.WorkspaceCatalog) {
			kept := c.Projects[:0]
			for _, p := range c.Projects {
				if p.ID != projectID {
					kept = append(kept, p)
				}
			}
			c.Projects = kept
		}),
		r.planningArtifact(func(p *config.PlanningState) {
			p.Today = withoutProject(p.Today, projectID)
			p.Next = withoutProject(p.Next, projectID)
		}),
	)
	r.commit(artifacts...)
}

func withoutProject(items []config.ProjectTaskLocator, projectID string) []config.ProjectTaskLocator {
	var out []config.ProjectTaskLocator
	for _, item := range items {
		if item.ProjectID != projectID {
			out = append(out, item)
		}
	}
	return out
}

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
		if _, _, local, classifyErr := synclog.Classify(rel); classifyErr != nil || local {
			return classifyErr
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
