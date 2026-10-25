package storage_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

// --- local edits ---

// commitLocal commits one user edit on d, possibly crashing, and returns the
// sync transaction it recorded, or "" when the crash lost the edit.
func (s *faultSim) commitLocal(d *simDevice, what string, artifacts ...storage.ArtifactChange) string {
	changed := false
	for _, artifact := range artifacts {
		current, err := os.ReadFile(artifact.Path)
		if err != nil || !bytes.Equal(current, artifact.Data) {
			changed = true
		}
	}
	if !changed {
		s.logf("%s skip %s: it changes nothing", d.name, what)
		return ""
	}
	before, err := d.history.LoadState()
	if err != nil {
		s.fatalf("%s: state: %v", d.name, err)
	}
	var nextSeq uint64
	if before != nil {
		nextSeq = before.NextSeq
	}
	crashed := s.withCrash(d, what, func() error {
		_, commitErr := d.svc.CommitArtifacts(artifacts)
		return commitErr
	})
	state, err := d.history.LoadState()
	if err != nil {
		s.fatalf("%s: state after %s: %v", d.name, what, err)
	}
	if state == nil || state.NextSeq == nextSeq {
		if !crashed {
			s.fatalf("%s: %s recorded no transaction", d.name, what)
		}
		s.logf("%s lost %s to the crash", d.name, what)
		return ""
	}
	if len(state.Heads) != 1 {
		s.fatalf("%s: after %s heads are %v", d.name, what, state.Heads)
	}
	s.count("local_commits")
	s.logf("%s commit %s: %s", d.name, short(state.Heads[0]), what)
	return state.Heads[0]
}

func (s *faultSim) read(d *simDevice, rel string) []byte {
	data, err := os.ReadFile(d.path(rel))
	if err != nil {
		s.fatalf("%s: read %s: %v", d.name, rel, err)
	}
	return data
}

func (s *faultSim) doc(d *simDevice, slug string) *storage.ProjectDocument {
	doc, err := storage.ParseProjectDocument(string(s.read(d, "projects/"+slug+"/tasks.md")))
	if err != nil {
		s.fatalf("%s: parse %s: %v", d.name, slug, err)
	}
	return doc
}

func (s *faultSim) put(d *simDevice, rel string, data []byte) storage.ArtifactChange {
	return storage.ArtifactChange{Path: d.path(rel), Data: data, Mode: storage.FilePermissions}
}

func (s *faultSim) docArtifact(d *simDevice, slug string, doc *storage.ProjectDocument) storage.ArtifactChange {
	content, err := doc.WriteCurrent()
	if err != nil {
		s.fatalf("%s: render %s: %v", d.name, slug, err)
	}
	return s.put(d, "projects/"+slug+"/tasks.md", []byte(content))
}

func (s *faultSim) marker(d *simDevice) string {
	s.minted++
	return fmt.Sprintf("mk%dx%04d%s", s.seed, s.minted, strings.TrimPrefix(d.name, "device-"))
}

func (s *faultSim) record(txn string, d *simDevice, path, subject, marker, kind string) {
	if txn != "" {
		s.writes = append(s.writes, userWrite{txn: txn, device: d.name, path: path, subject: subject, marker: marker, kind: kind})
	}
}

// retitle gives a task a fresh unique title.
func (s *faultSim) retitle(d *simDevice, slug, taskID string) {
	doc := s.doc(d, slug)
	task := findDocTask(doc, taskID)
	if task == nil {
		s.logf("%s cannot retitle missing task %s", d.name, taskID)
		return
	}
	marker := s.marker(d)
	task.Title = "Title " + marker
	txn := s.commitLocal(d, "retitle "+taskID[len(taskID)-3:]+" "+marker, s.docArtifact(d, slug, doc))
	s.record(txn, d, "projects/"+slug+"/tasks.md", "task:"+taskID, marker, "title")
}

// retitleRandom retitles a seeded task of a seeded project.
func (s *faultSim) retitleRandom(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	s.retitle(d, slug, doc.Tasks[s.rng.Intn(len(doc.Tasks))].ID)
}

// capture creates a root task with a minted identity and a unique title.
func (s *faultSim) capture(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	marker := s.marker(d)
	id := fmt.Sprintf("0a7e5d10-2c3b-4f6a-8e9d-%012x", 0xf0000000+s.seed*0x10000+int64(s.minted))
	template := *doc.Tasks[len(doc.Tasks)-1]
	task := &template
	task.ID = id
	task.Title = "Capture " + marker
	task.ParentID = nil
	task.Dependencies = nil
	task.Pinned = false
	task.PinnedAt = nil
	maxShort := 0
	for _, existing := range doc.Tasks {
		maxShort = max(maxShort, existing.ShortID)
	}
	task.ShortID = maxShort + 1
	doc.Tasks = append(doc.Tasks, task)
	txn := s.commitLocal(d, "capture "+marker, s.docArtifact(d, slug, doc))
	s.record(txn, d, "projects/"+slug+"/tasks.md", "task:"+id, marker, "title")
}

// deleteLeaf deletes a seeded task that has no children and no planning slot.
func (s *faultSim) deleteLeaf(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	if len(doc.Tasks) < 3 {
		return
	}
	task := doc.Tasks[s.rng.Intn(len(doc.Tasks))]
	planned := string(s.read(d, ".tt/planning.json"))
	for _, other := range doc.Tasks {
		if other.ParentID != nil && *other.ParentID == task.ID {
			return
		}
		for _, dep := range other.Dependencies {
			if strings.Contains(fmt.Sprint(dep), task.ID) {
				return
			}
		}
	}
	if strings.Contains(planned, task.ID) {
		return
	}
	kept := doc.Tasks[:0]
	for _, other := range doc.Tasks {
		if other.ID != task.ID {
			kept = append(kept, other)
		}
	}
	doc.Tasks = kept
	txn := s.commitLocal(d, "delete "+task.ID[len(task.ID)-3:], s.docArtifact(d, slug, doc))
	s.record(txn, d, "projects/"+slug+"/tasks.md", "task:"+task.ID, "", "delete")
}

// reprioritize sets a seeded task's priority.
func (s *faultSim) reprioritize(d *simDevice) {
	slug := faultSlugs[s.rng.Intn(len(faultSlugs))]
	doc := s.doc(d, slug)
	task := doc.Tasks[s.rng.Intn(len(doc.Tasks))]
	task.Priority = s.rng.Intn(4)
	s.commitLocal(d, fmt.Sprintf("priority %s %d", task.ID[len(task.ID)-3:], task.Priority), s.docArtifact(d, slug, doc))
}

// appendNote appends a unique line to the shared runbook note.
func (s *faultSim) appendNote(d *simDevice) {
	marker := s.marker(d)
	data := append(append([]byte{}, s.read(d, faultNotePath)...), []byte("- "+marker+"\n")...)
	txn := s.commitLocal(d, "note "+marker, s.put(d, faultNotePath, data))
	s.record(txn, d, faultNotePath, "note", marker, "note")
}

// plan moves a seeded open atlas task into Today or Next, or reverses Today.
func (s *faultSim) plan(d *simDevice) {
	raw := s.read(d, ".tt/planning.json")
	var env faultEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		s.fatalf("%s: planning envelope: %v", d.name, err)
	}
	planning := &config.PlanningState{}
	if err := json.Unmarshal(env.Data, planning); err != nil {
		s.fatalf("%s: planning: %v", d.name, err)
	}
	doc := s.doc(d, "atlas-launch")
	task := doc.Tasks[s.rng.Intn(len(doc.Tasks))]
	if task.IsTerminalState() || task.State() == models.StateBacklog {
		today := append([]config.ProjectTaskLocator{}, planning.Today...)
		for i, j := 0, len(today)-1; i < j; i, j = i+1, j-1 {
			today[i], today[j] = today[j], today[i]
		}
		planning.Today = today
	} else {
		item := config.ProjectTaskLocator{ProjectID: faultAtlasID, TaskID: task.ID}
		planning.Today = dropLocator(planning.Today, item)
		planning.Next = dropLocator(planning.Next, item)
		if s.rng.Intn(2) == 0 {
			planning.Today = append(planning.Today, item)
		} else {
			planning.Next = append(planning.Next, item)
		}
	}
	data, err := json.Marshal(planning)
	if err != nil {
		s.fatalf("encode planning: %v", err)
	}
	env.Data = data
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		s.fatalf("encode planning envelope: %v", err)
	}
	s.commitLocal(d, "plan "+task.ID[len(task.ID)-3:], s.put(d, ".tt/planning.json", out))
}

type faultEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data"`
}

func dropLocator(items []config.ProjectTaskLocator, drop config.ProjectTaskLocator) []config.ProjectTaskLocator {
	var out []config.ProjectTaskLocator
	for _, item := range items {
		if item != drop {
			out = append(out, item)
		}
	}
	return out
}

func findDocTask(doc *storage.ProjectDocument, id string) *models.Task {
	for _, task := range doc.Tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

// conflicts are the sync conflicts d derives.
func (s *faultSim) conflicts(d *simDevice) []synclog.Conflict {
	conflicts, err := d.svc.SyncConflicts()
	if err != nil {
		s.fatalf("%s: conflicts: %v", d.name, err)
	}
	return conflicts
}

// resolve resolves one of d's open conflicts by choosing one present
// competing version, or the current bytes, as the user would in a picker.
// choice < 0 is seeded.
func (s *faultSim) resolve(d *simDevice, conflict synclog.Conflict, choice int) {
	var options [][]byte
	for _, candidate := range conflict.Candidates {
		if candidate.Blob == nil {
			continue
		}
		data, err := d.history.Blob(*candidate.Blob)
		if err != nil {
			s.fatalf("%s: competing version %s of %s is not recoverable: %v", d.name, *candidate.Blob, conflict.Key, err)
		}
		options = append(options, data)
	}
	if current, err := os.ReadFile(d.path(conflict.Path)); err == nil {
		options = append(options, current)
	}
	if len(options) == 0 {
		return
	}
	if choice < 0 || choice >= len(options) {
		choice = s.rng.Intn(len(options))
	}
	s.commitResolution(d, conflict, options[choice], fmt.Sprintf("option %d of %d", choice, len(options)))
}

// resolveRetitling resolves a conflict on a project document by writing the
// current document with one task given a fresh title: a resolution that is
// none of the competing versions.
func (s *faultSim) resolveRetitling(d *simDevice, conflict synclog.Conflict, slug, taskID string) {
	doc := s.doc(d, slug)
	task := findDocTask(doc, taskID)
	if task == nil {
		s.fatalf("%s: task %s is not in %s", d.name, taskID, slug)
	}
	marker := s.marker(d)
	task.Title = "Resolved " + marker
	content, err := doc.WriteCurrent()
	if err != nil {
		s.fatalf("%s: render %s: %v", d.name, slug, err)
	}
	if txn := s.commitResolution(d, conflict, []byte(content), "retitle "+marker); txn != "" {
		s.record(txn, d, conflict.Path, "task:"+taskID, marker, "title")
	}
}

// commitResolution commits one resolution, possibly crashing, and returns
// its transaction, or "" when the crash lost it.
func (s *faultSim) commitResolution(d *simDevice, conflict synclog.Conflict, data []byte, how string) string {
	var txn string
	crashed := s.withCrash(d, "resolve "+short(conflict.ID), func() error {
		_, err := d.svc.CommitSyncResolution(conflict.ID, data)
		return err
	})
	if state, err := d.history.LoadState(); err == nil && state != nil && len(state.Heads) == 1 {
		if m, txErr := d.history.Transaction(state.Heads[0]); txErr == nil && m.Kind == synclog.KindResolution && m.DeviceID == d.name {
			txn = m.TxnID
		}
	}
	if txn == "" {
		if !crashed {
			s.fatalf("%s: resolving %s recorded no resolution", d.name, short(conflict.ID))
		}
		s.logf("%s lost a resolution of %s to the crash", d.name, short(conflict.ID))
		return ""
	}
	s.count("resolutions")
	s.logf("%s resolve %s at %s with %s as %s", d.name, short(conflict.ID), conflict.Path, how, short(txn))
	s.record(txn, d, conflict.Path, "", "", "resolution")
	return txn
}

// resolveRandom resolves a seeded open conflict on d, if it has one.
func (s *faultSim) resolveRandom(d *simDevice) bool {
	open := s.conflicts(d)
	if len(open) == 0 {
		return false
	}
	conflict := open[s.rng.Intn(len(open))]
	if conflict.Kind == synclog.EffectCatalog {
		return false
	}
	s.resolve(d, conflict, -1)
	return true
}
