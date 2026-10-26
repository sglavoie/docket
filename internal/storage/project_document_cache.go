package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/sglavoie/docket/internal/models"
)

// ProjectDocumentCache retains the parse of each tasks.md keyed by a digest of
// the exact bytes it was parsed from, so loading an unchanged file again skips
// both the parse and the canonical render its revision hashes. A load still
// reads the file every time: the bytes are the key, so a cached answer can never
// describe a version of the file other than the one just read, and the
// single-read guarantee LoadProjectWithCachedDocument gives is untouched.
//
// The retained document is never handed out. A ProjectSource holds it and
// copies it whenever it loads, because a document is the merge base a mutation
// keeps beside the project it edits, and rollback installs a document's own
// tasks on a project and rebuilds their tree. Two snapshots sharing one
// document would share those writes.
//
// A nil cache is valid and caches nothing. It is safe for concurrent use.
type ProjectDocumentCache struct {
	mu      sync.Mutex
	entries map[string]*cachedProjectDocument
}

// cachedProjectDocument is one retained parse and what is derived from it once
// per version of the file: the task identities a snapshot indexes without
// loading the project, and whether loading it succeeds at all, which a load's
// post-parse audit decides from the document alone.
type cachedProjectDocument struct {
	digest   [sha256.Size]byte
	document *ProjectDocument
	taskIDs  []string
	loadErr  error
}

// newCachedProjectDocument derives an entry's readings from a fresh parse. The
// load is tried on copies, so the retained document's tasks stay untouched.
func newCachedProjectDocument(digest [sha256.Size]byte, document *ProjectDocument) *cachedProjectDocument {
	entry := &cachedProjectDocument{digest: digest, document: document, taskIDs: make([]string, 0, len(document.Tasks))}
	for _, task := range document.Tasks {
		if task != nil && task.ID != "" {
			entry.taskIDs = append(entry.taskIDs, task.ID)
		}
	}
	entry.loadErr = loadProjectModels(&models.Project{}, cloneTasks(document.Tasks), models.CloneProjectComments(document.Comments))
	return entry
}

func cloneTasks(tasks []*models.Task) []*models.Task {
	cloned := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		cloned = append(cloned, models.CloneTaskDeep(task))
	}
	return cloned
}

// SharedProjectDocuments is the process-wide cache every workspace reader
// shares, so one launch parses each tasks.md once however many readers ask.
// Sharing is safe because an entry is a pure function of the path and the
// exact bytes read from it, and no reader ever holds the retained copy.
var SharedProjectDocuments = NewProjectDocumentCache()

// NewProjectDocumentCache returns an empty cache.
func NewProjectDocumentCache() *ProjectDocumentCache {
	return &ProjectDocumentCache{entries: make(map[string]*cachedProjectDocument)}
}

// retain is the cache entry for the content read from path, parsed on a miss.
// The entry itself is returned, so its document must never reach a caller that
// could write to it; ProjectSource is the one holder, and it hands out copies.
// A nil cache parses every time and retains nothing.
func (c *ProjectDocumentCache) retain(path string, content []byte) (*cachedProjectDocument, error) {
	digest := sha256.Sum256(content)
	if c == nil {
		document, err := ParseProjectDocument(string(content))
		if err != nil {
			return nil, err
		}
		return newCachedProjectDocument(digest, document), nil
	}
	c.mu.Lock()
	entry, hit := c.entries[path]
	c.mu.Unlock()
	if hit && entry.digest == digest {
		return entry, nil
	}
	document, err := ParseProjectDocument(string(content))
	if err != nil {
		return nil, err
	}
	entry = newCachedProjectDocument(digest, document)
	c.mu.Lock()
	c.entries[path] = entry
	c.mu.Unlock()
	return entry, nil
}

// NoteCommittedDocument retains document as the parse of content, which a
// commit just wrote to path as document's canonical rendering. The commit parsed
// those bytes' source to learn their revision, and a canonical rendering parses
// back to the document it renders, so the next reader of exactly these bytes is
// served that parse instead of making its own. The cache owns document from
// here on. Content whose digest is not document's revision is not its canonical
// rendering and retains nothing.
func (c *ProjectDocumentCache) NoteCommittedDocument(path string, content []byte, document *ProjectDocument) {
	if c == nil || document == nil {
		return
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != document.Revision {
		return
	}
	entry := newCachedProjectDocument(digest, document)
	c.mu.Lock()
	c.entries[path] = entry
	c.mu.Unlock()
}

// clone is a deep copy sharing nothing a caller may write through.
func (d *ProjectDocument) clone() *ProjectDocument {
	copied := *d
	copied.Tasks = make([]*models.Task, len(d.Tasks))
	for i, task := range d.Tasks {
		copied.Tasks[i] = models.CloneTaskDeep(task)
	}
	copied.Comments = models.CloneProjectComments(d.Comments)
	return &copied
}
