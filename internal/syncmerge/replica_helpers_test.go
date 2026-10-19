package syncmerge_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
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
	ledger  *storage.WorkspaceLedger
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
	return c.open(device, root)
}

// open opens device over the workspace at root.
func (c *cluster) open(device, root string) *replica {
	c.t.Helper()
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
	return &replica{t: c.t, device: device, root: root, history: history, svc: svc, ledger: ledger}
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

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func removeFile(path string) error { return os.Remove(path) }

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
