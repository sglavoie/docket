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
// The retained document is never handed out. Every hit returns a deep copy,
// because a document is the merge base a mutation keeps beside the project it
// edits, and rollback installs a document's own tasks on a project and rebuilds
// their tree. Two snapshots sharing one document would share those writes.
//
// A nil cache is valid and caches nothing. It is safe for concurrent use.
type ProjectDocumentCache struct {
	mu      sync.Mutex
	entries map[string]cachedProjectDocument
	// canonical holds, per path, the digest of the bytes this process last
	// wrote there as a canonical rendering. Those bytes are their own canonical
	// content, so their revision is their digest and a parse of them need not
	// render the document again to learn it.
	canonical map[string][sha256.Size]byte
}

type cachedProjectDocument struct {
	digest   [sha256.Size]byte
	document *ProjectDocument
}

// SharedProjectDocuments is the process-wide cache every workspace reader
// shares, so one launch parses each tasks.md once however many readers ask.
// Sharing is safe because an entry is a pure function of the path and the
// exact bytes read from it, and no reader ever holds the retained copy.
var SharedProjectDocuments = NewProjectDocumentCache()

// NewProjectDocumentCache returns an empty cache.
func NewProjectDocumentCache() *ProjectDocumentCache {
	return &ProjectDocumentCache{
		entries:   make(map[string]cachedProjectDocument),
		canonical: make(map[string][sha256.Size]byte),
	}
}

// parse is ParseProjectDocument for the content read from path, answered from
// the cache when path last held exactly these bytes.
func (c *ProjectDocumentCache) parse(path string, content []byte) (*ProjectDocument, error) {
	if c == nil {
		return ParseProjectDocument(string(content))
	}
	digest := sha256.Sum256(content)
	c.mu.Lock()
	entry, hit := c.entries[path]
	c.mu.Unlock()
	if hit && entry.digest == digest {
		return entry.document.clone(), nil
	}
	c.mu.Lock()
	wrote, known := c.canonical[path]
	c.mu.Unlock()
	var document *ProjectDocument
	var err error
	if known && wrote == digest {
		document, err = parseProjectDocumentContent(string(content))
		if err == nil {
			document.Revision = hex.EncodeToString(digest[:])
		}
	} else {
		document, err = ParseProjectDocument(string(content))
	}
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.entries[path] = cachedProjectDocument{digest: digest, document: document.clone()}
	c.mu.Unlock()
	return document, nil
}

// NoteCanonicalWrite records that content, whose revision is revision, was just
// written to path as a canonical rendering. A revision is the digest of a
// document's canonical rendering, so the next parse of exactly these bytes takes
// their digest as its revision instead of rendering the document to compute it.
// Content whose digest is not that revision is not canonical and records nothing.
func (c *ProjectDocumentCache) NoteCanonicalWrite(path string, content []byte, revision string) {
	if c == nil {
		return
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != revision {
		return
	}
	c.mu.Lock()
	c.canonical[path] = digest
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
