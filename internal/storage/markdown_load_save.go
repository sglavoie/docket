package storage

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tterrors"
)

// LoadProject loads tasks from a project's tasks.md file. Malformed hierarchy
// data does not survive as a repaired tree: ParseProjectDocument rejects a
// missing project-local parent, a parent cycle, and a depth overflow before the
// tree is built, so a corrupt file is a load failure rather than a silent
// in-memory rewrite.
func LoadProject(project *models.Project) error {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		if err := DefaultRepository.Load(project); err != nil {
			return err
		}
		return auditLoadedProject(project)
	}

	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		return loadMissingProject(project)
	}
	if err != nil {
		return fmt.Errorf("failed to read tasks file: %w", err)
	}
	return loadProjectContent(project, content)
}

// LoadProjectCached is LoadProject answering the parse from the shared
// document cache when tasks.md still holds bytes it has already parsed. The
// file is read either way; a test repository loads through LoadProject.
func LoadProjectCached(project *models.Project) error {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		return LoadProject(project)
	}
	_, err := LoadProjectWithCachedDocument(project, SharedProjectDocuments)
	return err
}

// LoadProjectRevisionCached is LoadProjectCached returning the canonical
// revision of what it loaded, which the cached parse already holds. A test
// repository has no bytes to key on, so it loads through LoadProject and its
// revision is rendered from the loaded project, as an uncached reader's is.
func LoadProjectRevisionCached(project *models.Project) (string, error) {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		if err := LoadProject(project); err != nil {
			return "", err
		}
		return ToProjectDocument(project).Revision, nil
	}
	document, err := LoadProjectWithCachedDocument(project, SharedProjectDocuments)
	if err != nil {
		return "", err
	}
	return document.Revision, nil
}

// ProjectReadWindowProbe is the optional observation seam a consistency test
// installs to rewrite tasks.md at the exact instant LoadProjectWithCachedDocument
// holds its bytes and has derived nothing from them yet. Normal execution
// leaves it nil and nothing in production ever writes it. It exists because the
// property this file owes its callers — that the model, the base document, and
// the guarded revision all describe one version of the file — is only
// observable by replacing that file inside the window and proving the answer
// did not move.
var ProjectReadWindowProbe func(project *models.Project)

// readBuffers holds the buffers tasks.md reads land in. A snapshot reads every
// project on every mutation, and those bytes are only hashed, or copied into the
// string a parse reads, so a buffer is free again once its load returns rather
// than garbage the size of the whole workspace each time. It is a bounded free
// list rather than a sync.Pool because a pool is emptied by every collection,
// and on a large workspace a collection lands between most snapshots.
var readBuffers = &bufferFreeList{limit: 8}

type bufferFreeList struct {
	mu    sync.Mutex
	free  []*bytes.Buffer
	limit int
}

func (l *bufferFreeList) Get() *bytes.Buffer {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n := len(l.free); n > 0 {
		buffer := l.free[n-1]
		l.free = l.free[:n-1]
		return buffer
	}
	return new(bytes.Buffer)
}

func (l *bufferFreeList) Put(buffer *bytes.Buffer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.free) < l.limit {
		l.free = append(l.free, buffer)
	}
}

// readPooled reads path into a pooled buffer the caller puts back.
func readPooled(path string) (*bytes.Buffer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	buffer := readBuffers.Get()
	buffer.Reset()
	// A pooled buffer may be smaller than this file; sizing it first is one
	// growth rather than the doubling ReadFrom would do.
	if info, statErr := file.Stat(); statErr == nil {
		buffer.Grow(int(info.Size()) + bytes.MinRead)
	}
	if _, err := buffer.ReadFrom(file); err != nil {
		readBuffers.Put(buffer)
		return nil, err
	}
	return buffer, nil
}

// LoadProjectWithCachedDocument loads a project's tasks and returns the exact
// lossless on-disk document whose revision guards them, both derived from a
// single read of tasks.md. The parse is answered from cache when tasks.md still
// holds the bytes cache last parsed for it; the file is read either way, so the
// result describes the bytes on disk now, and a nil cache parses every time.
//
// Loading the model and then reading the document back through a second file
// read is what this replaces: another process saving version B between the two
// reads would pair version A's tasks with version B's revision, and a commit
// carrying that pair matches the revision on disk, skips the three-way merge,
// and writes A plus the caller's edit over B. One read cannot straddle a save,
// so the worst a concurrent write can now do is make the whole answer stale —
// which the revision guard catches at the commit boundary.
//
// The project's task pointers are the ones a mutation edits in place; the
// document's are the immutable base a merge is measured against, so the project
// takes deep copies and the two never share a task.
func LoadProjectWithCachedDocument(project *models.Project, cache *ProjectDocumentCache) (*ProjectDocument, error) {
	document, _, err := ReadProjectWithCachedDocument(project, cache)
	return document, err
}

// ReadProjectWithCachedDocument is LoadProjectWithCachedDocument also reporting
// whether the read found no tasks.md, which loads as an empty project. It is the
// answer ProjectTasksFileMissing gives, taken from the same read rather than from
// a second look at the file that could disagree with it. A test repository has
// no file to miss.
func ReadProjectWithCachedDocument(project *models.Project, cache *ProjectDocumentCache) (*ProjectDocument, bool, error) {
	source, err := ReadProjectSource(project, cache)
	if err != nil {
		return nil, false, err
	}
	document, err := source.Load(project)
	if err != nil {
		return nil, false, err
	}
	return document, source.Missing(), nil
}

// ProjectSource is one read of a project's tasks.md, held so the project can be
// loaded from exactly those bytes later, or never. A snapshot reads every
// project to learn its revision, but a mutation reaches one or two of them, and
// loading — copying every task out of the parse into the project — is the part
// worth deferring. Whatever loads it later loads the version this read saw, so
// the single-read guarantee holds however late that is.
type ProjectSource struct {
	entry   *cachedProjectDocument
	missing bool
	// loaded is a test repository's document, already loaded into the project
	// when the source was read, since that repository has no bytes to keep.
	loaded *ProjectDocument
}

// ReadProjectSource reads a project's tasks.md once and returns it unloaded. A
// file that does not parse, or whose tasks a load's audit would refuse, fails
// here rather than at Load, so a reader can leave the project out up front.
func ReadProjectSource(project *models.Project, cache *ProjectDocumentCache) (*ProjectSource, error) {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		// Test doubles keep projects in memory with no tasks.md, so there is no
		// read window to close: the canonical in-memory document is the only
		// answer available and is derived from the same load.
		if err := LoadProject(project); err != nil {
			return nil, err
		}
		document, err := CanonicalProjectDocument(project, nil)
		if err != nil {
			return nil, err
		}
		return &ProjectSource{loaded: document}, nil
	}

	path := tasksFilePath(project.Path)
	buffer, err := readPooled(path)
	if os.IsNotExist(err) {
		document := &ProjectDocument{ProjectName: project.Name, Tasks: []*models.Task{}, SchemaVersion: CurrentTaskSchemaVersion}
		document.Revision = document.ComputeRevision()
		return &ProjectSource{entry: &cachedProjectDocument{document: document}, missing: true}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}
	defer readBuffers.Put(buffer)
	if ProjectReadWindowProbe != nil {
		ProjectReadWindowProbe(project)
	}
	entry, err := cache.retain(path, buffer.Bytes())
	if err != nil {
		return nil, loadParseError(project, err)
	}
	if entry.loadErr != nil {
		return nil, entry.loadErr
	}
	return &ProjectSource{entry: entry}, nil
}

// Missing reports whether the read found no tasks.md.
func (s *ProjectSource) Missing() bool { return s.missing }

// Revision is the revision of the version read.
func (s *ProjectSource) Revision() string {
	if s.loaded != nil {
		return s.loaded.Revision
	}
	return s.entry.document.Revision
}

// TaskIDs names the tasks the version read holds, without loading them. The
// slice is shared and must not be written.
func (s *ProjectSource) TaskIDs() []string {
	if s.loaded != nil {
		ids := make([]string, 0, len(s.loaded.Tasks))
		for _, task := range s.loaded.Tasks {
			if task != nil && task.ID != "" {
				ids = append(ids, task.ID)
			}
		}
		return ids
	}
	return s.entry.taskIDs
}

// Load fills project with the version read and returns a document of it that
// the caller owns. The project takes deep copies rather than the document's own
// tasks: the document is the guarded base a commit is measured against, and the
// project is what the caller goes on to mutate.
func (s *ProjectSource) Load(project *models.Project) (*ProjectDocument, error) {
	if s.loaded != nil {
		return s.loaded, nil
	}
	if s.missing {
		if err := loadMissingProject(project); err != nil {
			return nil, err
		}
		document := *s.entry.document
		document.Tasks = []*models.Task{}
		return &document, nil
	}
	document := s.entry.document.clone()
	if err := loadProjectModels(project, cloneTasks(document.Tasks), models.CloneProjectComments(document.Comments)); err != nil {
		return nil, err
	}
	return document, nil
}

// LoadProjectForRewrite loads the exact tasks.md bytes that a caller is about to
// render as a full-file replacement and returns their raw optimistic-concurrency
// revision. A missing file is represented by a non-nil pointer to the empty
// revision, which means the eventual write must still observe the file as absent.
//
// Filesystem-backed loads parse the same byte slice whose hash is returned, so a
// lifecycle planner can never render one version while carrying the revision of
// another. Test repositories still load through their injected implementation;
// their revision contract reflects any physical shadow tasks.md when present and
// otherwise requires absence.
func LoadProjectForRewrite(project *models.Project) (*string, error) {
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		if err := LoadProject(project); err != nil {
			return nil, err
		}
		content, err := os.ReadFile(tasksFilePath(project.Path))
		if os.IsNotExist(err) {
			expected := ""
			return &expected, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tasks file for rewrite: %w", err)
		}
		expected := RawArtifactRevision(content)
		return &expected, nil
	}

	content, err := os.ReadFile(tasksFilePath(project.Path))
	if os.IsNotExist(err) {
		if loadErr := loadMissingProject(project); loadErr != nil {
			return nil, loadErr
		}
		expected := ""
		return &expected, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file for rewrite: %w", err)
	}
	if err := loadProjectContent(project, content); err != nil {
		return nil, err
	}
	expected := RawArtifactRevision(content)
	return &expected, nil
}

// loadMissingProject treats an absent tasks file as a successful load of an
// empty project. Comments are cleared with the tasks: they live in the same
// absent file, so keeping whatever the in-memory project happened to carry
// would resurrect a collection nothing on disk holds.
func loadMissingProject(project *models.Project) error {
	project.Tasks = []*models.Task{}
	project.Comments = nil
	project.NextShortID = 1
	return auditLoadedProject(project)
}

func loadProjectContent(project *models.Project, content []byte) error {
	// The project keeps the tasks and comments alone, so the revision a full
	// ParseProjectDocument would compute is never read here.
	document, err := parseProjectDocumentContent(string(content))
	if err != nil {
		return loadParseError(project, err)
	}
	return loadProjectModels(project, document.Tasks, document.Comments)
}

func loadParseError(project *models.Project, err error) error {
	return fmt.Errorf("%w: %s is not a valid schema-%d task document: %w",
		tterrors.ErrParseError, loadedTasksFileLabel(project), CurrentTaskSchemaVersion, err)
}

// loadProjectModels installs parsed tasks and comments on a project and
// derives what a load owes beside them: the next ShortID, the task tree, and
// the post-load audit.
func loadProjectModels(project *models.Project, tasks []*models.Task, comments []*models.ProjectComment) error {
	project.Tasks = tasks
	project.Comments = comments
	maxShortID := 0
	for _, task := range project.Tasks {
		if task.ShortID > maxShortID {
			maxShortID = task.ShortID
		}
	}
	project.NextShortID = maxShortID + 1
	project.BuildTaskTree()
	return auditLoadedProject(project)
}

// loadedTasksFileLabel names the file a load failure came from, so the reader
// can open it; a project with no directory still gets a noun.
func loadedTasksFileLabel(project *models.Project) string {
	if project == nil || strings.TrimSpace(project.Path) == "" {
		return tasksFileName
	}
	return tasksFilePath(project.Path)
}

func auditLoadedProject(project *models.Project) error {
	if err := models.AuditProject(project); err != nil {
		return fmt.Errorf("%w: project audit failed after load normalization: %w", tterrors.ErrParseError, err)
	}

	return nil
}

func auditProjectBeforeSave(project *models.Project) error {
	if err := models.AuditProject(project); err != nil {
		projectName := "<unknown>"
		if project != nil && project.Name != "" {
			projectName = project.Name
		}
		return tterrors.NewSaveProjectError(projectName, fmt.Errorf("project audit failed before save: %w", err))
	}

	return nil
}

// ErrNoProjectPath reports a filesystem save of a project rooted at no
// directory. An empty path would otherwise resolve tasks.md and its lock beneath
// the process working directory, so the save is refused by name instead.
var ErrNoProjectPath = errors.New("storage: project has no directory path")

// SaveProject saves a project's tasks to its tasks.md file.
func SaveProject(project *models.Project) error {
	if err := auditProjectBeforeSave(project); err != nil {
		return err
	}
	if _, ok := DefaultRepository.(*FileSystemRepository); !ok {
		return DefaultRepository.Save(project)
	}
	if strings.TrimSpace(project.Path) == "" {
		return tterrors.NewSaveProjectError(project.Name, ErrNoProjectPath)
	}
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	tasksPath := tasksFilePath(project.Path)
	document, err := CanonicalProjectDocument(project, nil)
	if err != nil {
		return fmt.Errorf("failed to render tasks file: %w", err)
	}
	content, err := document.WriteCurrent()
	if err != nil {
		return fmt.Errorf("failed to render tasks file: %w", err)
	}
	if err := writeFileAtomic(tasksPath, []byte(content), FilePermissions); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

// projectExists checks if a project directory and tasks file exist.
func projectExists(projectPath string) bool {
	_, err := os.Stat(tasksFilePath(projectPath))
	return err == nil
}

// CreateProjectDir creates the project directory structure.
func CreateProjectDir(project *models.Project) error {
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	return SaveProject(project)
}

// deleteProjectDir removes a project's directory.
func deleteProjectDir(projectPath string) error {
	return os.RemoveAll(projectPath)
}
