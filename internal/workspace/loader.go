package workspace

import (
	"errors"
	"os"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tterrors"
)

// sourceIdentity captures the fast staleness signal for a project's tasks.md:
// its modification time and size. Two loads with the same identity are treated
// as the same source revision, so the cached parse can be reused without
// re-reading and re-parsing the file.
type sourceIdentity struct {
	modTime time.Time
	size    int64
	exists  bool
}

func (a sourceIdentity) equal(b sourceIdentity) bool {
	return a.exists == b.exists && a.size == b.size && a.modTime.Equal(b.modTime)
}

type cacheEntry struct {
	identity sourceIdentity
	revision string
	project  *models.Project
}

// Loader builds workspace snapshots while caching parsed projects keyed by
// their source revision. When a project's tasks.md is unchanged since it was
// last loaded (same mod time and size), the cached parse is reused and the file
// is not read or parsed again. The cache is not safe for concurrent use; each
// goroutine (or render pass) should own its Loader.
type Loader struct {
	// loadProject parses a project's tasks.md into the project in place and
	// returns the canonical content revision of what it parsed. Overridable for
	// tests; defaults to loadThroughSharedDocuments.
	loadProject func(*models.Project) (string, error)
	// statSource returns the source identity of a project's tasks.md.
	// Overridable for tests; defaults to stat of storage.TasksFilePath.
	statSource func(*models.Project) sourceIdentity

	cache      map[string]cacheEntry
	parseCount int
	cacheHits  int
}

// NewLoader returns a Loader backed by the real storage layer.
func NewLoader() *Loader {
	return &Loader{
		loadProject: loadThroughSharedDocuments,
		statSource:  defaultStatSource,
		cache:       make(map[string]cacheEntry),
	}
}

// loadThroughSharedDocuments loads a project through the process-wide document
// cache, so a file another loader or the repository already parsed in exactly
// these bytes costs a read and a copy rather than a parse and a render, and the
// revision comes from that one parse instead of a second rendering.
func loadThroughSharedDocuments(project *models.Project) (string, error) {
	return storage.LoadProjectRevisionCached(project)
}

func defaultStatSource(project *models.Project) sourceIdentity {
	info, err := os.Stat(storage.TasksFilePath(project.Path))
	if err != nil {
		// A missing file is a valid empty project, not a failure. Any other
		// stat error simply disables caching for this project (identity stays
		// zero/exists=false), which is safe: the project is loaded fresh.
		return sourceIdentity{exists: false}
	}
	return sourceIdentity{modTime: info.ModTime(), size: info.Size(), exists: true}
}

// Build loads each requested project once and returns an immutable snapshot.
// Projects that fail to load are recorded as diagnostics and excluded rather
// than aborting the whole build, so a single corrupt project cannot blank out
// every other project's tasks.
func (l *Loader) Build(projects []*models.Project) *Snapshot {
	snap := newSnapshot()
	for _, project := range projects {
		if project == nil {
			continue
		}
		loaded, revision, diag, ok := l.load(project)
		if !ok {
			snap.addDiagnostic(diag)
			continue
		}
		snap.addProject(loaded, revision)
	}
	return snap
}

// load resolves one project, reusing the cached parse when the source revision
// is unchanged. On a cache hit the cached project pointer is returned and no
// parse is performed. On a miss the caller's project is loaded in place, cached,
// and returned — so a fresh per-call Loader always hands back the caller's own
// (now populated) project, preserving mutation semantics for mutable consumers.
func (l *Loader) load(project *models.Project) (*models.Project, string, Diagnostic, bool) {
	identity := l.statSource(project)
	if identity.exists && project.ID != "" {
		if entry, hit := l.cache[project.ID]; hit && entry.identity.equal(identity) {
			l.cacheHits++
			return entry.project, entry.revision, Diagnostic{}, true
		}
	}

	revision, err := l.loadProject(project)
	if err != nil {
		return nil, "", Diagnostic{
			ProjectID:   project.ID,
			ProjectName: project.Name,
			Kind:        classifyLoadError(err),
			Detail:      err.Error(),
		}, false
	}
	l.parseCount++

	if identity.exists && project.ID != "" {
		l.cache[project.ID] = cacheEntry{identity: identity, revision: revision, project: project}
	}
	return project, revision, Diagnostic{}, true
}

func classifyLoadError(err error) DiagnosticKind {
	if errors.Is(err, tterrors.ErrParseError) {
		return DiagnosticCorrupt
	}
	return DiagnosticSkipped
}

// ParseCount reports how many projects this Loader has actually read and parsed
// (cache misses). Instrumentation for proving unchanged projects are not
// reparsed across successive Build calls.
func (l *Loader) ParseCount() int { return l.parseCount }

// CacheHits reports how many project loads were served from the revision cache
// without re-reading or re-parsing the source file.
func (l *Loader) CacheHits() int { return l.cacheHits }
