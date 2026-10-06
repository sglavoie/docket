package compat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workspace"
)

// fixtureManifestFormat names the compatibility-fixture manifest. The contract
// it implements is docs/mobile/fixture-contract.md.
const (
	fixtureManifestFormat  = "tt-compat-fixture"
	fixtureManifestVersion = 1
	fixturesDir            = "testdata/workspaces"
)

// requiredFixtureCoverage is what the compatibility baseline must exercise
// across the committed fixtures as a whole. A fixture may declare more; it may
// never declare a key without a probe.
var requiredFixtureCoverage = []string{
	"relationship.parent_child",
	"relationship.dependency_cross_project",
	"relationship.checklist_origin",
	"planning.today",
	"planning.next",
	"planning.parked_next_hidden",
	"checkin.repeating",
	"checkin.one_off",
	"checkin.waiting",
	"lifecycle.all_states",
	"rich.description",
	"rich.resume_note",
	"rich.task_comments",
	"rich.project_comments",
	"rich.checklist",
	"rich.attachment_file",
	"rich.project_note",
	"rich.global_note",
	"lens.scoped",
	"project.archived",
	"project.parked",
	"project.system_unfiled",
}

type fixtureManifest struct {
	Format        string         `json:"format"`
	FormatVersion int            `json:"format_version"`
	Name          string         `json:"name"`
	Workspace     string         `json:"workspace"`
	Description   string         `json:"description"`
	Schemas       fixtureSchemas `json:"schemas"`
	Expect        fixtureExpect  `json:"expect"`
	Coverage      []string       `json:"coverage"`
}

type fixtureSchemas struct {
	DomainEnvelope int `json:"domain_envelope"`
	Catalog        int `json:"catalog"`
	TaskMarkdown   int `json:"task_markdown"`
}

type fixtureExpect struct {
	Projects         int `json:"projects"`
	ArchivedProjects int `json:"archived_projects"`
	ParkedProjects   int `json:"parked_projects"`
	Tasks            int `json:"tasks"`
	Today            int `json:"today"`
	Next             int `json:"next"`
	Lenses           int `json:"lenses"`
	ProjectNotes     int `json:"project_notes"`
	GlobalNotes      int `json:"global_notes"`
	AttachmentFiles  int `json:"attachment_files"`
}

func loadFixtureManifests(t *testing.T) []fixtureManifest {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(fixturesDir, "*.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no compatibility fixture manifests under %s", fixturesDir)
	}
	manifests := make([]fixtureManifest, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var manifest fixtureManifest
		if err := decoder.Decode(&manifest); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if want := strings.TrimSuffix(filepath.Base(path), ".fixture.json"); manifest.Name != want {
			t.Fatalf("%s: name %q does not match its file name %q", path, manifest.Name, want)
		}
		manifests = append(manifests, manifest)
	}
	return manifests
}

// TestCompatFixtureManifestsDeclareLiveSchemas holds every manifest to the
// fixture contract and to the schemas this build reads and writes. A schema bump
// that leaves the fixtures behind fails here rather than in a mobile client.
func TestCompatFixtureManifestsDeclareLiveSchemas(t *testing.T) {
	declared := map[string]bool{}
	for _, manifest := range loadFixtureManifests(t) {
		if manifest.Format != fixtureManifestFormat || manifest.FormatVersion != fixtureManifestVersion {
			t.Errorf("%s: format %q v%d, want %q v%d", manifest.Name, manifest.Format, manifest.FormatVersion, fixtureManifestFormat, fixtureManifestVersion)
		}
		if strings.TrimSpace(manifest.Description) == "" {
			t.Errorf("%s: description is required", manifest.Name)
		}
		if info, err := os.Stat(filepath.Join(fixturesDir, manifest.Workspace, ".tt")); err != nil || !info.IsDir() {
			t.Errorf("%s: workspace %q has no .tt directory", manifest.Name, manifest.Workspace)
		}
		want := fixtureSchemas{
			DomainEnvelope: config.CurrentDomainSchemaVersion,
			Catalog:        config.CurrentConfigVersion,
			TaskMarkdown:   storage.CurrentTaskSchemaVersion,
		}
		if manifest.Schemas != want {
			t.Errorf("%s: schemas %+v, live build is %+v; regenerate the fixture", manifest.Name, manifest.Schemas, want)
		}
		if len(manifest.Coverage) == 0 {
			t.Errorf("%s: declares no coverage", manifest.Name)
		}
		for _, key := range manifest.Coverage {
			if _, ok := coverageProbes[key]; !ok {
				t.Errorf("%s: coverage key %q has no probe", manifest.Name, key)
			}
			if declared[manifest.Name+"\x00"+key] {
				t.Errorf("%s: coverage key %q repeats", manifest.Name, key)
			}
			declared[manifest.Name+"\x00"+key] = true
			declared[key] = true
		}
	}
	for _, key := range requiredFixtureCoverage {
		if !declared[key] {
			t.Errorf("required coverage %q is declared by no fixture", key)
		}
	}
}

// fixtureWorld is one fixture opened through the live readers.
type fixtureWorld struct {
	dataDir     string
	home        string
	runtime     *config.RuntimeState
	projects    []*models.Project
	documents   map[string]*storage.ProjectDocument
	tasks       map[string]*models.Task
	owner       map[string]*models.Project
	lenses      config.NamedLensList
	notes       map[string][]*models.Note
	globalNotes []*models.Note
	snapshot    *workspace.Snapshot
}

// copyFixture copies a committed fixture into a fresh directory, so no test can
// write through to testdata.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join(fixturesDir, name)
	target := filepath.Join(t.TempDir(), "workspace")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func openFixture(t *testing.T, name string) *fixtureWorld {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := copyFixture(t, name)
	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("load domains: %v", err)
	}
	world := &fixtureWorld{
		dataDir:   dataDir,
		home:      home,
		runtime:   runtime,
		documents: map[string]*storage.ProjectDocument{},
		tasks:     map[string]*models.Task{},
		owner:     map[string]*models.Project{},
		notes:     map[string][]*models.Note{},
	}
	for _, project := range runtime.AllProjects() {
		document, loadErr := storage.LoadProjectWithDocument(project)
		if loadErr != nil {
			t.Fatalf("load %s: %v", project.Slug, loadErr)
		}
		world.projects = append(world.projects, project)
		world.documents[project.ID] = document
		for _, task := range project.Tasks {
			if _, dup := world.tasks[task.ID]; dup {
				t.Fatalf("task id %s appears twice in the workspace", task.ID)
			}
			world.tasks[task.ID] = task
			world.owner[task.ID] = project
		}
		notes, notesErr := storage.ListNotes(project)
		if notesErr != nil {
			t.Fatalf("notes %s: %v", project.Slug, notesErr)
		}
		world.notes[project.ID] = notes
	}
	globalNotes, err := storage.ListNotesFrom(storage.GlobalNotesDir(dataDir))
	if err != nil {
		t.Fatalf("global notes: %v", err)
	}
	world.globalNotes = globalNotes
	lenses, _, err := config.NewDomainFileStoreOnMachine(dataDir, home).LoadLensDefinitionsChecked()
	if err != nil {
		t.Fatalf("lenses: %v", err)
	}
	if err := lenses.Validate(); err != nil {
		t.Fatalf("lenses: %v", err)
	}
	world.lenses = lenses.Lenses
	// The snapshot is the one every frontend reads through; it reloads from disk
	// with fresh project values so its diagnostics judge the files, not the
	// already-loaded models above.
	world.snapshot = workspace.NewLoader().Build(runtime.AllProjects())
	return world
}

// TestCompatRepresentativeWorkspaceLoadsThroughLiveReaders opens each committed
// fixture with the readers the desktop uses and holds it to the strict ones:
// every project audits clean, re-renders to the exact bytes on disk, and the
// shared snapshot reports no diagnostics.
func TestCompatRepresentativeWorkspaceLoadsThroughLiveReaders(t *testing.T) {
	for _, manifest := range loadFixtureManifests(t) {
		t.Run(manifest.Name, func(t *testing.T) {
			world := openFixture(t, manifest.Workspace)
			if world.snapshot.HasDiagnostics() {
				t.Fatalf("snapshot diagnostics: %v", world.snapshot.Diagnostics())
			}
			for _, project := range world.projects {
				if err := models.AuditProject(project); err != nil {
					t.Errorf("%s: audit: %v", project.Slug, err)
				}
				onDisk, err := os.ReadFile(storage.TasksFilePath(project.Path))
				if err != nil {
					t.Fatal(err)
				}
				rendered, err := storage.WriteProjectMarkdown(project)
				if err != nil {
					t.Fatalf("%s: render: %v", project.Slug, err)
				}
				if rendered != string(onDisk) {
					t.Errorf("%s: canonical writer does not reproduce the committed bytes; regenerate the fixture", project.Slug)
				}
				if got := world.documents[project.ID].SchemaVersion; got != storage.CurrentTaskSchemaVersion {
					t.Errorf("%s: task schema %d, want %d", project.Slug, got, storage.CurrentTaskSchemaVersion)
				}
			}
			for id, task := range world.tasks {
				for _, reference := range task.Dependencies {
					location, status := world.snapshot.LookupTaskID(reference.TaskID)
					if status != workspace.TaskLookupUnique || location.Project.ID != reference.ProjectID {
						t.Errorf("task %s depends on unresolvable %s:%s", id, reference.ProjectID, reference.TaskID)
					}
				}
			}
			for _, list := range [][]config.ProjectTaskLocator{world.runtime.Planning().GetToday(), world.runtime.Planning().GetNext()} {
				for _, locator := range list {
					if owner := world.owner[locator.TaskID]; owner == nil || owner.ID != locator.ProjectID {
						t.Errorf("planning locator %s:%s names no task", locator.ProjectID, locator.TaskID)
					}
				}
			}
			checkFixtureExpectations(t, world, manifest.Expect)
		})
	}
}

func checkFixtureExpectations(t *testing.T, world *fixtureWorld, expect fixtureExpect) {
	t.Helper()
	archived, parked, projectNotes, files := 0, 0, 0, 0
	for _, project := range world.projects {
		if project.Archived {
			archived++
		}
		if models.ProjectTagsParked(project.Tags) {
			parked++
		}
		projectNotes += len(world.notes[project.ID])
		for _, task := range project.Tasks {
			for _, attachment := range task.Attachments {
				if !attachment.IsURL && !attachment.IsCodeRef {
					files++
				}
			}
		}
	}
	got := fixtureExpect{
		Projects:         len(world.projects),
		ArchivedProjects: archived,
		ParkedProjects:   parked,
		Tasks:            len(world.tasks),
		Today:            len(world.runtime.Planning().GetToday()),
		Next:             len(world.runtime.Planning().GetNext()),
		Lenses:           len(world.lenses),
		ProjectNotes:     projectNotes,
		GlobalNotes:      len(world.globalNotes),
		AttachmentFiles:  files,
	}
	if got != expect {
		t.Errorf("fixture census %+v, manifest expects %+v", got, expect)
	}
}

// coverageProbes answers each coverage key a manifest may declare. A probe
// inspects only what the live readers produced, so a key holds only if the
// feature survived parsing, not merely if its bytes are present.
var coverageProbes = map[string]func(*fixtureWorld) bool{
	"relationship.parent_child": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.ParentID != nil && task.Parent != nil })
	},
	"relationship.nested_depth_3": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.Level >= 2 })
	},
	"relationship.dependency_cross_project": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			for _, reference := range task.Dependencies {
				if reference.ProjectID != w.owner[task.ID].ID {
					return true
				}
			}
			return false
		})
	},
	"relationship.dependency_reciprocal": func(w *fixtureWorld) bool {
		edges := map[[2]string]bool{}
		for _, task := range w.tasks {
			for _, reference := range task.Dependencies {
				edges[[2]string{w.owner[task.ID].ID, reference.ProjectID}] = true
			}
		}
		for edge := range edges {
			if edge[0] != edge[1] && edges[[2]string{edge[1], edge[0]}] {
				return true
			}
		}
		return false
	},
	"relationship.checklist_origin": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.ChecklistOrigin != nil && task.ParentID != nil && task.ChecklistOrigin.TaskID == *task.ParentID
		})
	},
	"planning.today": func(w *fixtureWorld) bool { return len(w.runtime.Planning().GetToday()) > 0 },
	"planning.next":  func(w *fixtureWorld) bool { return len(w.runtime.NextOrder()) > 0 },
	"planning.parked_next_hidden": func(w *fixtureWorld) bool {
		return len(w.runtime.Planning().GetNext()) > len(w.runtime.NextOrder())
	},
	"checkin.repeating": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.NextCheckIn != nil && task.CheckInRepeatDays > 0 })
	},
	"checkin.one_off": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.NextCheckIn != nil && task.CheckInRepeatDays == 0 })
	},
	"checkin.waiting": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.WorkflowState == models.StateWaiting && task.NextCheckIn != nil
		})
	},
	"checkin.reviewed_terminal": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.WorkflowState.IsTerminal() && task.LastReviewedAt != nil && task.NextCheckIn == nil
		})
	},
	"lifecycle.all_states": func(w *fixtureWorld) bool {
		seen := map[models.WorkflowState]bool{}
		for _, task := range w.tasks {
			seen[task.WorkflowState] = true
		}
		for _, state := range []models.WorkflowState{
			models.StateInbox, models.StateBacklog, models.StateDoing, models.StateWaiting,
			models.StatePaused, models.StateSomeday, models.StateDone, models.StateCanceled,
		} {
			if !seen[state] {
				return false
			}
		}
		return true
	},
	"schedule.due_date": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.DueDate != nil })
	},
	"schedule.due_time": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			return task.DueDate != nil && (task.DueDate.Hour() != 0 || task.DueDate.Minute() != 0)
		})
	},
	"schedule.deferred": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.DeferredUntil != nil && task.DeferCount > 0 })
	},
	"rich.description": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return strings.Contains(task.Description, "\n") })
	},
	"rich.resume_note": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.ResumeNote != "" })
	},
	"rich.task_comments": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return len(task.Comments) > 1 })
	},
	"rich.project_comments": func(w *fixtureWorld) bool {
		for _, project := range w.projects {
			for _, comment := range project.Comments {
				if strings.Contains(comment.Text, "\n") {
					return true
				}
			}
		}
		return false
	},
	"rich.checklist": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			done, open := false, false
			for _, item := range task.Checklist {
				done = done || item.Done
				open = open || !item.Done
			}
			return done && open
		})
	},
	"rich.attachment_file": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool {
			for _, attachment := range task.Attachments {
				if !attachment.IsURL && !attachment.IsCodeRef &&
					storage.AttachmentExists(w.owner[task.ID], task, attachment.Filename) &&
					storage.GetAttachmentSize(w.owner[task.ID], task, &attachment) > 0 {
					return true
				}
			}
			return false
		})
	},
	"rich.attachment_url": func(w *fixtureWorld) bool {
		return w.anyAttachment(func(attachment models.Attachment) bool { return attachment.IsURL })
	},
	"rich.attachment_code_ref": func(w *fixtureWorld) bool {
		return w.anyAttachment(func(attachment models.Attachment) bool {
			return attachment.IsCodeRef && attachment.LineNumber > 0
		})
	},
	"rich.project_note": func(w *fixtureWorld) bool {
		for _, notes := range w.notes {
			for _, note := range notes {
				if note.ParseError == "" && note.Body != "" {
					return true
				}
			}
		}
		return false
	},
	"rich.global_note": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.globalNotes, func(note *models.Note) bool { return note.ParseError == "" && note.Body != "" })
	},
	"rich.tags_and_context": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return len(task.Tags) > 1 && task.Context != "" })
	},
	"rich.pinned": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.Pinned && task.PinnedAt != nil })
	},
	"rich.review_flag": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return task.Review })
	},
	"lens.scoped": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.lenses, func(lens config.NamedLens) bool {
			return lens.Scope != "" && w.projectByID(lens.Scope) != nil && lens.Query != ""
		})
	},
	"lens.unscoped": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.lenses, func(lens config.NamedLens) bool { return lens.Scope == "" })
	},
	"project.archived": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.projects, func(project *models.Project) bool {
			return project.Archived && project.ArchivedAt != nil && len(project.Tasks) > 0
		})
	},
	"project.archived_check_in": func(w *fixtureWorld) bool {
		return w.anyTask(func(task *models.Task) bool { return w.owner[task.ID].Archived && task.NextCheckIn != nil })
	},
	"project.parked": func(w *fixtureWorld) bool {
		for _, project := range w.projects {
			if !models.ProjectTagsParked(project.Tags) {
				continue
			}
			hidden := !slices.ContainsFunc(w.runtime.TaskWorkspaceProjects(), func(p *models.Project) bool { return p.ID == project.ID })
			available := slices.ContainsFunc(w.runtime.AvailableProjects(), func(p *models.Project) bool { return p.ID == project.ID })
			if hidden && available {
				return true
			}
		}
		return false
	},
	"project.system_unfiled": func(w *fixtureWorld) bool {
		unfiled := w.runtime.SystemUnfiledProject()
		return unfiled != nil && w.projectByID(unfiled.ID) != nil && len(w.projectByID(unfiled.ID).Tasks) > 0
	},
	"project.tags": func(w *fixtureWorld) bool {
		return slices.ContainsFunc(w.projects, func(project *models.Project) bool {
			return len(project.Tags) > 1 && !models.ProjectTagsParked(project.Tags)
		})
	},
}

func (w *fixtureWorld) anyTask(predicate func(*models.Task) bool) bool {
	for _, task := range w.tasks {
		if predicate(task) {
			return true
		}
	}
	return false
}

func (w *fixtureWorld) anyAttachment(predicate func(models.Attachment) bool) bool {
	return w.anyTask(func(task *models.Task) bool { return slices.ContainsFunc(task.Attachments, predicate) })
}

func (w *fixtureWorld) projectByID(id string) *models.Project {
	for _, project := range w.projects {
		if project.ID == id {
			return project
		}
	}
	return nil
}

// TestCompatFixtureCoverageSurvivesLiveParsing runs the probe for every key each
// manifest declares, one subtest per key so a lost feature is named.
func TestCompatFixtureCoverageSurvivesLiveParsing(t *testing.T) {
	for _, manifest := range loadFixtureManifests(t) {
		world := openFixture(t, manifest.Workspace)
		for _, key := range manifest.Coverage {
			t.Run(manifest.Name+"/"+key, func(t *testing.T) {
				probe, ok := coverageProbes[key]
				if !ok {
					t.Fatalf("no probe for %q", key)
				}
				if !probe(world) {
					t.Fatalf("%s declares %q but the live readers do not exhibit it", manifest.Name, key)
				}
			})
		}
	}
}

// TestCompatDesktopOpenLeavesFixtureUntouched opens the fixture the way the
// desktop does — recovery, migration classification, domain bootstrap — and
// proves the current build treats it as already current: no portable byte
// changes and no portable file appears or vanishes.
func TestCompatDesktopOpenLeavesFixtureUntouched(t *testing.T) {
	for _, manifest := range loadFixtureManifests(t) {
		t.Run(manifest.Name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			dataDir := copyFixture(t, manifest.Workspace)
			before := hashPortableTree(t, dataDir)
			manager := config.NewManagerOnMachine(config.ConfigPathForDataDir(dataDir), home)
			if _, err := manager.LoadRuntimeState(); err != nil {
				t.Fatalf("desktop open: %v", err)
			}
			after := hashPortableTree(t, dataDir)
			if diff := diffTrees(before, after); diff != "" {
				t.Fatalf("desktop open rewrote portable artifacts:\n%s", diff)
			}
		})
	}
}

// hashPortableTree hashes every portable file. Local-only directories the
// desktop may create on open — journals, undo, backups, conflicts — are not
// portable state and are excluded, as are lock files and the .bak sidecars a
// domain write leaves beside its artifact. A machine joining a workspace adopts
// its lenses on first open and republishes lenses.json byte-identically, which
// leaves exactly such a sidecar; sync must never carry one.
func hashPortableTree(t *testing.T, root string) map[string]string {
	t.Helper()
	local := map[string]bool{
		filepath.Join(".tt", "transactions"): true,
		filepath.Join(".tt", "undo"):         true,
		filepath.Join(".tt", "backups"):      true,
		filepath.Join(".tt", "conflicts"):    true,
	}
	hashes := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if local[relative] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(relative, ".lock") || strings.HasSuffix(relative, ".bak") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hashes
}

func diffTrees(before, after map[string]string) string {
	var lines []string
	for path, hash := range before {
		switch other, ok := after[path]; {
		case !ok:
			lines = append(lines, "removed "+path)
		case other != hash:
			lines = append(lines, "changed "+path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			lines = append(lines, "added "+path)
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestCompatExternalMarkdownEditIsObservable covers the one durable write tt
// does not perform: a person editing tasks.md in their own editor. Sync has no
// hook inside that editor, so its integration point is observation — a valid
// external edit must change the canonical revision the next load computes, and
// an invalid one must be refused by the strict reader rather than read as a
// smaller project.
func TestCompatExternalMarkdownEditIsObservable(t *testing.T) {
	world := openFixture(t, "representative")
	project := world.projectByID(fixtureAtlasID)
	if project == nil {
		t.Fatal("fixture lost the Atlas project")
	}
	path := storage.TasksFilePath(project.Path)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	baseRevision := world.documents[project.ID].Revision

	edited := bytes.Replace(original, []byte("## Task: Ship beta build"), []byte("## Task: Ship beta build today"), 1)
	if bytes.Equal(edited, original) {
		t.Fatal("fixture no longer contains the edited heading")
	}
	if writeErr := os.WriteFile(path, edited, 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	reloaded := &models.Project{ID: project.ID, Slug: project.Slug, Name: project.Name, Path: project.Path}
	document, err := storage.LoadProjectWithDocument(reloaded)
	if err != nil {
		t.Fatalf("valid external edit refused: %v", err)
	}
	if document.Revision == baseRevision {
		t.Fatal("external edit left the canonical revision unchanged; sync could not observe it")
	}
	if storage.RawArtifactRevision(edited) == storage.RawArtifactRevision(original) {
		t.Fatal("raw revision did not move with the bytes")
	}

	corrupt := bytes.Replace(original, []byte("state: doing"), []byte("state: started"), 1)
	if err := os.WriteFile(path, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.LoadProjectWithDocument(&models.Project{ID: project.ID, Slug: project.Slug, Name: project.Name, Path: project.Path}); err == nil {
		t.Fatal("strict reader accepted an external edit naming an unknown state")
	}
}
