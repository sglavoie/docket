package engine

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sglavoie/docket/internal/syncexchange"
	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/syncmerge"
)

// maxConflictText bounds the content a conflict view spells inline; larger or
// binary versions are described by size alone.
const maxConflictText = 1 << 20

// peerNames are the device names the last exchange pass read from the
// folder's advertisements.
func (r *Runtime) peerNames() map[string]string {
	names := map[string]string{}
	history := r.repo.svc.SyncHistory()
	if history == nil {
		return names
	}
	last, _, err := syncexchange.LoadFolderStatus(history)
	if err != nil || last == nil {
		return names
	}
	for _, peer := range last.Peers {
		if peer.DeviceName != "" {
			names[peer.DeviceID] = peer.DeviceName
		}
	}
	return names
}

type conflictReader struct {
	runtime *Runtime
	history *synclog.History
	names   map[string]string
	writers map[string]ConflictWriter
	all     []synclog.Conflict
}

type coupledChoice struct {
	conflict  synclog.Conflict
	candidate synclog.ConflictCandidate
}

func (c *conflictReader) view(conflict synclog.Conflict) (ConflictView, error) {
	view := ConflictView{
		ID:          conflict.ID,
		Path:        conflict.Path,
		Kind:        string(conflict.Kind),
		ProjectID:   conflict.ProjectID,
		Label:       c.label(conflict),
		Versions:    []ConflictVersion{},
		Details:     append([]synclog.ConflictDetail{}, conflict.Details...),
		Replaceable: replaceable(conflict.Kind),
	}
	if conflict.BaseBlob != nil {
		base := c.content(conflict.Kind, conflict.BaseBlob, conflict.Path)
		view.Base = &base
	}
	current := c.content(conflict.Kind, conflict.ResultBlob, conflict.Path)
	view.Current = &current
	for i, candidate := range conflict.Candidates {
		version := ConflictVersion{
			Index:           i,
			ConflictContent: c.content(conflict.Kind, candidate.Blob, conflict.Path),
			Current:         sameBlob(candidate.Blob, conflict.ResultBlob),
			Writers:         []ConflictWriter{},
			AlsoResolves:    []ConflictRef{},
		}
		for _, txnID := range candidate.Writers {
			writer, err := c.writer(txnID)
			if err != nil {
				return ConflictView{}, err
			}
			version.Writers = append(version.Writers, writer)
		}
		sort.SliceStable(version.Writers, func(i, j int) bool { return version.Writers[i].CreatedAt < version.Writers[j].CreatedAt })
		for _, coupled := range c.coupled(conflict, candidate) {
			version.AlsoResolves = append(version.AlsoResolves, c.ref(coupled.conflict))
		}
		view.Versions = append(view.Versions, version)
	}
	return view, nil
}

// coupled are the other open conflicts the candidate's writers also left,
// each with the one version those writers wrote there. A conflict where the
// writers left more than one version is not coupled unambiguously and is
// left for its own choice.
func (c *conflictReader) coupled(chosen synclog.Conflict, candidate synclog.ConflictCandidate) []coupledChoice {
	var out []coupledChoice
	for _, other := range c.all {
		if other.ID == chosen.ID || other.Path == chosen.Path {
			continue
		}
		var matches []synclog.ConflictCandidate
		for _, theirs := range other.Candidates {
			if sharesWriter(theirs.Writers, candidate.Writers) {
				matches = append(matches, theirs)
			}
		}
		if len(matches) == 1 {
			out = append(out, coupledChoice{conflict: other, candidate: matches[0]})
		}
	}
	return out
}

func sharesWriter(a, b []string) bool {
	for _, id := range a {
		if slices.Contains(b, id) {
			return true
		}
	}
	return false
}

func (c *conflictReader) writer(txnID string) (ConflictWriter, error) {
	if writer, ok := c.writers[txnID]; ok {
		return writer, nil
	}
	manifest, err := c.history.Transaction(txnID)
	if err != nil {
		return ConflictWriter{}, err
	}
	writer := ConflictWriter{
		TxnID:      txnID,
		DeviceID:   manifest.DeviceID,
		DeviceName: c.names[manifest.DeviceID],
		ThisDevice: manifest.DeviceID == c.history.DeviceID(),
		Change:     changeLabel(manifest.Kind),
		CreatedAt:  manifest.CreatedAt,
		Files:      len(manifest.Effects),
	}
	c.writers[txnID] = writer
	return writer, nil
}

func changeLabel(kind synclog.Kind) string {
	switch kind {
	case synclog.KindUser:
		return "edit"
	case synclog.KindExternal:
		return "edit outside tt"
	case synclog.KindResolution:
		return "conflict resolution"
	case synclog.KindUndo:
		return "undo"
	case synclog.KindRedo:
		return "redo"
	case synclog.KindEnroll:
		return "enrollment"
	default:
		return string(kind)
	}
}

func (c *conflictReader) content(kind synclog.EffectKind, blob *string, path string) ConflictContent {
	if blob == nil {
		return ConflictContent{Deleted: true, Available: true}
	}
	content := ConflictContent{Digest: *blob}
	data, err := c.bytes(blob, path)
	if err != nil {
		return content
	}
	content.Available = true
	content.Size = len(data)
	if kind != synclog.EffectAttachment && len(data) <= maxConflictText && utf8.Valid(data) {
		text := string(data)
		content.Text = &text
	}
	return content
}

// bytes are a version's retained bytes, nil for a deletion. The workspace's
// own copy stands in for a version the history did not retain.
func (c *conflictReader) bytes(blob *string, path string) ([]byte, error) {
	if blob == nil {
		return nil, nil
	}
	data, err := c.history.Blob(*blob)
	if err == nil {
		return data, nil
	}
	if disk, diskErr := os.ReadFile(c.history.WorkspacePath(path)); diskErr == nil && synclog.Digest(disk) == *blob {
		return disk, nil
	}
	return nil, fmt.Errorf("%w: version %s is unavailable on this device; sync and reopen the conflict", ErrInvalidResolution, shortDigest(*blob))
}

// kept is what keeping candidate writes. A structured file keeps the
// candidate's side of everything that conflicted and the rest of what the
// workspace holds now — the other side's independent changes and edits made
// since — by merging the current bytes into the candidate against the
// conflict's base with the candidate as the primary side. Opaque content and
// deletions are kept whole.
func (c *conflictReader) kept(conflict synclog.Conflict, candidate synclog.ConflictCandidate) ([]byte, error) {
	data, err := c.bytes(candidate.Blob, conflict.Path)
	if err != nil || data == nil || conflict.ResultBlob == nil || sameBlob(candidate.Blob, conflict.ResultBlob) {
		return data, err
	}
	switch conflict.Kind {
	case synclog.EffectProjectDocument, synclog.EffectCatalog, synclog.EffectPlanning:
	default:
		return data, nil
	}
	current, err := c.bytes(conflict.ResultBlob, conflict.Path)
	if err != nil {
		return nil, err
	}
	base := synclog.Version{}
	if conflict.BaseBlob != nil {
		if base.Data, err = c.bytes(conflict.BaseBlob, conflict.Path); err != nil {
			return nil, err
		}
		base.Exists = true
	}
	merger := syncmerge.Merger{}
	merged, mergeErr := merger.Merge(synclog.MergeRequest{
		Kind:  conflict.Kind,
		Key:   conflict.Key,
		Base:  base,
		Sides: []synclog.Version{{Exists: true, Data: data}, {Exists: true, Data: current}},
	})
	if mergeErr == nil && merger.ValidateArtifact(conflict.Kind, merged.Data) == nil {
		return merged.Data, nil
	}
	// A merge that cannot be made or read keeps the candidate whole, which is
	// what the person saw and chose.
	return data, nil
}

func (c *conflictReader) ref(conflict synclog.Conflict) ConflictRef {
	return ConflictRef{ID: conflict.ID, Path: conflict.Path, Label: c.label(conflict)}
}

// label names a conflicted file as the app does.
func (c *conflictReader) label(conflict synclog.Conflict) string {
	project := c.projectName(conflict.ProjectID)
	base := path.Base(conflict.Path)
	switch conflict.Kind {
	case synclog.EffectProjectDocument:
		return "Tasks in " + project
	case synclog.EffectNote:
		return "Note “" + c.noteTitle(conflict, base) + "” in " + project
	case synclog.EffectGlobalNote:
		return "Note “" + c.noteTitle(conflict, base) + "”"
	case synclog.EffectAttachment:
		return "Attachment " + base + " in " + project
	case synclog.EffectCatalog:
		return "Projects list"
	case synclog.EffectPlanning:
		return "Today and Next"
	case synclog.EffectLenses:
		return "Saved views"
	default:
		return conflict.Path
	}
}

// noteTitle is the title a conflicted note's current version declares in its
// frontmatter, else its file name without the creation stamp.
func (c *conflictReader) noteTitle(conflict synclog.Conflict, base string) string {
	for _, blob := range append([]*string{conflict.ResultBlob}, candidateBlobs(conflict)...) {
		data, err := c.bytes(blob, conflict.Path)
		if err != nil || data == nil {
			continue
		}
		if title := frontmatterTitle(string(data)); title != "" {
			return title
		}
	}
	name := strings.TrimSuffix(base, ".md")
	if match := noteStamp.FindStringSubmatch(name); match != nil {
		return match[1]
	}
	return name
}

var noteStamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d{6}_[0-9a-f]{8}-(.+)$`)

func candidateBlobs(conflict synclog.Conflict) []*string {
	blobs := make([]*string, 0, len(conflict.Candidates))
	for _, candidate := range conflict.Candidates {
		blobs = append(blobs, candidate.Blob)
	}
	return blobs
}

// frontmatterTitle reads `title:` from a note's leading frontmatter block.
func frontmatterTitle(text string) string {
	if !strings.HasPrefix(text, "---\n") {
		return ""
	}
	for _, line := range strings.Split(text[4:], "\n") {
		if line == "---" {
			break
		}
		if value, ok := strings.CutPrefix(line, "title:"); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}

func (c *conflictReader) projectName(projectID string) string {
	if catalog := c.runtime.state.Catalog(); catalog != nil {
		for _, project := range catalog.Projects {
			if project.ID == projectID {
				return project.Name
			}
		}
	}
	if projectID == "" {
		return "the workspace"
	}
	return "project " + shortDigest(projectID)
}

func sameBlob(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func shortDigest(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
