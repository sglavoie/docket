package storage_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/synclog"
)

// --- invariants ---

// syncedPath reports a path synclog.Classify names and does not call
// device-local; an unclassifiable path is not synced.
func syncedPath(rel string) bool {
	_, _, local, err := synclog.Classify(rel)
	return err == nil && !local
}

// syncedDir reports whether a directory can hold a synced path. Classify syncs
// nothing outside projects/ and .tt/ and, inside .tt/, only its top-level files
// and global-notes/, so the walk skips the history, journal, undo, backup, and
// conflict stores that every delivery grows. Fingerprinting each delivery
// spent most of this suite's time listing those directories on macOS.
func syncedDir(rel string) bool {
	parts := strings.Split(rel, "/")
	switch parts[0] {
	case ".":
		return true
	case "projects":
		return true
	case ".tt":
		return len(parts) == 1 || parts[1] == "global-notes"
	}
	return false
}

func (s *faultSim) syncedFiles(d *simDevice) map[string]string {
	out := map[string]string{}
	err := filepath.WalkDir(d.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(d.root, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if !syncedDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !syncedPath(rel) {
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
		s.fatalf("%s: walk: %v", d.name, err)
	}
	return out
}

// requireConverged holds every replica, authors and genesis observers alike,
// to identical synced bytes, identical derived conflicts, a workspace every
// live reader accepts, and competing versions every replica can recover.
func (s *faultSim) requireConverged() []synclog.Conflict {
	first := s.devices[0]
	want := s.syncedFiles(first)
	wantConflicts := s.conflictsJSON(first)
	for _, d := range s.devices[1:] {
		got := s.syncedFiles(d)
		for _, rel := range sortedKeys(want) {
			if got[rel] != want[rel] {
				s.fatalf("%s and %s disagree on %s:\n%s\n---\n%s", first.name, d.name, rel, readOrErr(first, rel), readOrErr(d, rel))
			}
		}
		for _, rel := range sortedKeys(got) {
			if _, ok := want[rel]; !ok {
				s.fatalf("%s holds %s and %s does not", d.name, rel, first.name)
			}
		}
		if gotConflicts := s.conflictsJSON(d); gotConflicts != wantConflicts {
			s.fatalf("%s and %s derive different conflicts:\n%s\n---\n%s", first.name, d.name, wantConflicts, gotConflicts)
		}
	}
	for _, d := range s.devices {
		s.requireValidWorkspace(d)
	}
	conflicts := s.conflicts(first)
	for _, conflict := range conflicts {
		for _, candidate := range conflict.Candidates {
			if candidate.Blob == nil {
				continue
			}
			for _, d := range s.devices {
				if _, err := d.history.Blob(*candidate.Blob); err != nil {
					s.fatalf("%s cannot recover competing version %s of %s: %v", d.name, *candidate.Blob, conflict.Key, err)
				}
			}
		}
	}
	s.logf("converged: %d replicas, %d synced files, %d open conflicts", len(s.devices), len(want), len(conflicts))
	return conflicts
}

func readOrErr(d *simDevice, rel string) string {
	data, err := os.ReadFile(d.path(rel))
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (s *faultSim) conflictsJSON(d *simDevice) string {
	data, err := json.MarshalIndent(s.conflicts(d), "", "  ")
	if err != nil {
		s.fatalf("encode conflicts: %v", err)
	}
	return string(data)
}

// requireValidWorkspace reads a replica through the live readers.
func (s *faultSim) requireValidWorkspace(d *simDevice) {
	var env faultEnvelope
	if err := json.Unmarshal(s.read(d, ".tt/workspace.json"), &env); err != nil {
		s.fatalf("%s: catalog envelope: %v", d.name, err)
	}
	catalog := &config.WorkspaceCatalog{}
	if err := json.Unmarshal(env.Data, catalog); err != nil {
		s.fatalf("%s: catalog: %v", d.name, err)
	}
	if err := catalog.Validate(); err != nil {
		s.fatalf("%s: catalog invalid: %v", d.name, err)
	}
	if err := json.Unmarshal(s.read(d, ".tt/planning.json"), &env); err != nil {
		s.fatalf("%s: planning envelope: %v", d.name, err)
	}
	planning := &config.PlanningState{}
	if err := json.Unmarshal(env.Data, planning); err != nil {
		s.fatalf("%s: planning: %v", d.name, err)
	}
	if err := planning.Validate(); err != nil {
		s.fatalf("%s: planning invalid: %v", d.name, err)
	}
	for _, project := range catalog.Projects {
		rel := "projects/" + project.Slug + "/tasks.md"
		if _, err := os.Stat(d.path(rel)); err != nil {
			continue
		}
		doc := s.doc(d, project.Slug)
		if err := models.AuditProject(&models.Project{Name: doc.ProjectName, Tasks: doc.Tasks, Comments: doc.Comments}); err != nil {
			s.fatalf("%s: %s fails the audit: %v", d.name, rel, err)
		}
		shortIDs := map[int]string{}
		for _, task := range doc.Tasks {
			if other, clash := shortIDs[task.ShortID]; clash {
				s.fatalf("%s: %s gives short id %d to %s and %s", d.name, rel, task.ShortID, other, task.ID)
			}
			shortIDs[task.ShortID] = task.ID
		}
	}
}

// ancestry is every authored transaction's strict ancestors, read from the
// exchange.
func (s *faultSim) ancestry() map[string]map[string]bool {
	parents := map[string][]string{}
	for _, txnID := range s.ex.order {
		parents[txnID] = s.decode(txnID).Parents
	}
	memo := map[string]map[string]bool{}
	var visit func(string) map[string]bool
	visit = func(id string) map[string]bool {
		if out, ok := memo[id]; ok {
			return out
		}
		out := map[string]bool{}
		for _, parent := range parents[id] {
			out[parent] = true
			for ancestor := range visit(parent) {
				out[ancestor] = true
			}
		}
		memo[id] = out
		return out
	}
	for _, txnID := range s.ex.order {
		visit(txnID)
	}
	return memo
}

// requirePreserved holds the converged workspace to the user's work: every
// marker a user wrote that no later edit of theirs replaced — a later title
// for the same task, a deletion of it, or a resolution of the file it sits in,
// each made after seeing it — is still in the workspace or in a competing
// version of a conflict that is still open.
func (s *faultSim) requirePreserved(conflicts []synclog.Conflict) {
	ancestors := s.ancestry()
	d := s.devices[0]
	var corpus strings.Builder
	files := s.syncedFiles(d)
	for _, rel := range sortedKeys(files) {
		corpus.Write(s.read(d, rel))
	}
	workspace := corpus.String()
	var competing strings.Builder
	for _, conflict := range conflicts {
		for _, candidate := range conflict.Candidates {
			if candidate.Blob == nil {
				continue
			}
			data, err := d.history.Blob(*candidate.Blob)
			if err != nil {
				s.fatalf("competing version %s is not recoverable: %v", *candidate.Blob, err)
			}
			competing.Write(data)
		}
	}
	live, kept := 0, 0
	for _, w := range s.writes {
		if w.marker == "" {
			continue
		}
		if _, published := s.ex.position[w.txn]; !published {
			s.fatalf("%s's write %s in %s was never published", w.device, w.marker, short(w.txn))
		}
		if s.superseded(w, ancestors) {
			continue
		}
		live++
		switch {
		case strings.Contains(workspace, w.marker):
			kept++
		case strings.Contains(competing.String(), w.marker):
			s.count("preserved_in_conflict")
		default:
			s.fatalf("%s's %s %s (%s in %s) is in neither the workspace nor any open conflict", w.device, w.kind, w.marker, w.subject, short(w.txn))
		}
	}
	s.stats["live_user_writes"] += live
	s.logf("preserved: %d live user writes, %d in the workspace", live, kept)
}

func (s *faultSim) superseded(w userWrite, ancestors map[string]map[string]bool) bool {
	for _, later := range s.writes {
		if !ancestors[later.txn][w.txn] {
			continue
		}
		switch {
		case later.kind == "resolution" && later.path == w.path:
			return true
		case w.kind == "title" && later.subject == w.subject && (later.kind == "title" || later.kind == "delete"):
			return true
		}
	}
	return false
}

// requireIdempotent redelivers the whole exchange to every replica, damaged,
// duplicated and reordered, and requires that nothing changes anywhere.
func (s *faultSim) requireIdempotent() {
	before := map[string]string{}
	for _, d := range s.devices {
		before[d.name] = s.fingerprint(d)
		for _, txnID := range s.ex.order {
			s.enqueue(d, s.decode(txnID), s.ex.manifests[txnID])
		}
	}
	s.logf("redeliver everything to every replica")
	s.settle()
	for _, d := range s.devices {
		if after := s.fingerprint(d); after != before[d.name] {
			s.fatalf("%s: redelivering the exchange changed the replica:\n%s\n---\n%s", d.name, before[d.name], after)
		}
	}
}
