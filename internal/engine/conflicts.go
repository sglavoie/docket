package engine

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/syncmerge"
)

// Sync conflict refusals. Callers classify with errors.Is.
var (
	// ErrConflictStale marks a resolution of a conflict that changed since it
	// was read: another device's resolution closed it, or an edit gave it new
	// bytes and a new id. Nothing was written; the racing edit and every
	// competing version stay as they are, so re-read and choose again.
	ErrConflictStale = errors.New("engine: the sync conflict changed since it was read")
	// ErrInvalidResolution marks a resolution the engine cannot write: an
	// unknown choice or version, replacement text for a kind that takes none,
	// or text the workspace's readers would refuse.
	ErrInvalidResolution = errors.New("engine: invalid sync conflict resolution")
)

// maxConflictText bounds the content a conflict view spells inline; larger or
// binary versions are described by size alone.
const maxConflictText = 1 << 20

// Resolution choices.
const (
	// ResolveVersion keeps one competing version, and with it the same
	// transactions' versions of every other conflict they left, in one
	// resolution transaction.
	ResolveVersion = "version"
	// ResolveCurrent keeps what the workspace holds now.
	ResolveCurrent = "current"
	// ResolveReplace writes replacement text a person composed.
	ResolveReplace = "replace"
)

// ConflictView is one open sync conflict as a person reads it: what the file
// is, the version both sides started from, each competing version with the
// devices and changes that wrote it, what the workspace holds meanwhile, and
// what else a choice would settle.
type ConflictView struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	ProjectID string `json:"project_id,omitempty"`
	// Label names the file the way the app does: the project's tasks, a
	// note's name, the planning lists.
	Label string `json:"label"`
	// Base is the version every competing change started from, or nil when
	// the file did not exist then.
	Base *ConflictContent `json:"base"`
	// Current is what the workspace holds now.
	Current  *ConflictContent  `json:"current"`
	Versions []ConflictVersion `json:"versions"`
	// Details are the parts the merge could not reconcile on its own.
	Details []synclog.ConflictDetail `json:"details"`
	// Replaceable reports that replacement text may be written for this kind.
	Replaceable bool `json:"replaceable"`
}

// ConflictContent is one version's bytes as far as a reader can show them.
type ConflictContent struct {
	Digest string `json:"digest,omitempty"`
	// Deleted marks a version that removed the file.
	Deleted bool `json:"deleted"`
	Size    int  `json:"size"`
	// Text is the content when it is text of a readable size.
	Text *string `json:"text,omitempty"`
	// Available is false when the bytes are not retained on this device.
	Available bool `json:"available"`
}

// ConflictVersion is one competing version and who left it.
type ConflictVersion struct {
	Index int `json:"index"`
	ConflictContent
	// Current marks the version the workspace holds meanwhile.
	Current bool             `json:"current"`
	Writers []ConflictWriter `json:"writers"`
	// AlsoResolves are the other open conflicts the same transactions left;
	// choosing this version keeps their versions too, in one transaction.
	AlsoResolves []ConflictRef `json:"also_resolves"`
}

// ConflictWriter is one transaction that wrote a competing version.
type ConflictWriter struct {
	TxnID    string `json:"txn_id"`
	DeviceID string `json:"device_id"`
	// DeviceName is the name the device advertises, when this device knows it.
	DeviceName string `json:"device_name,omitempty"`
	// ThisDevice marks a change made here.
	ThisDevice bool   `json:"this_device"`
	Change     string `json:"change"`
	CreatedAt  string `json:"created_at"`
	// Files counts every file the transaction changed.
	Files int `json:"files"`
}

// ConflictRef names an open conflict.
type ConflictRef struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Label string `json:"label"`
}

// ResolveRequest chooses how one open conflict, read with ConflictID, ends.
type ResolveRequest struct {
	ConflictID string `json:"conflict_id"`
	Choice     string `json:"choice"`
	// Version is the index of the competing version ResolveVersion keeps.
	Version int `json:"version"`
	// Text is the replacement ResolveReplace writes.
	Text string `json:"text"`
}

// ResolveResult is a committed resolution.
type ResolveResult struct {
	TxnID string `json:"txn_id"`
	// Resolved are the conflicts the resolution closed: the one chosen and
	// every one coupled to it.
	Resolved []ConflictRef `json:"resolved"`
	// Remaining counts the conflicts still open.
	Remaining int `json:"remaining"`
}

// SyncConflicts reads every open sync conflict with its versions and their
// context. names maps device ids to the names their advertisements give;
// on a desktop the last exchange pass supplies them when names is nil.
func (r *Runtime) SyncConflicts(names map[string]string) ([]ConflictView, error) {
	commits := r.repo.svc
	history := commits.SyncHistory()
	if history == nil {
		return []ConflictView{}, nil
	}
	conflicts, err := commits.SyncConflicts()
	if err != nil {
		return nil, err
	}
	if names == nil {
		names = r.peerNames()
	}
	reader := conflictReader{runtime: r, history: history, names: names, writers: map[string]ConflictWriter{}, all: conflicts}
	views := make([]ConflictView, 0, len(conflicts))
	for _, conflict := range conflicts {
		view, err := reader.view(conflict)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].Path != views[j].Path {
			return views[i].Path < views[j].Path
		}
		return views[i].ID < views[j].ID
	})
	return views, nil
}

// ResolveSyncConflict commits a resolution. Keeping a competing version also
// keeps that version's transactions' side of every coupled conflict, so a
// change that spanned files is resolved whole. The conflict must still be
// open with the bytes it was read with; otherwise nothing is written and the
// refusal wraps ErrConflictStale.
func (r *Runtime) ResolveSyncConflict(req ResolveRequest) (*ResolveResult, error) {
	commits := r.repo.svc
	history := commits.SyncHistory()
	if history == nil {
		return nil, fmt.Errorf("%w: this workspace records no sync history", ErrInvalidResolution)
	}
	conflicts, err := commits.SyncConflicts()
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(conflicts, func(c synclog.Conflict) bool { return c.ID == req.ConflictID })
	if index < 0 {
		return nil, fmt.Errorf("%w: conflict %s is no longer open", ErrConflictStale, shortDigest(req.ConflictID))
	}
	chosen := conflicts[index]
	reader := conflictReader{runtime: r, history: history, all: conflicts}
	var resolutions []storage.SyncResolution
	resolved := []ConflictRef{reader.ref(chosen)}
	switch req.Choice {
	case ResolveCurrent:
		data, readErr := reader.bytes(chosen.ResultBlob, chosen.Path)
		if readErr != nil {
			return nil, readErr
		}
		resolutions = append(resolutions, storage.SyncResolution{ConflictID: chosen.ID, Data: data})
	case ResolveReplace:
		if !replaceable(chosen.Kind) {
			return nil, fmt.Errorf("%w: a %s takes a version, not replacement text", ErrInvalidResolution, chosen.Kind)
		}
		data := []byte(req.Text)
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("%w: replacement text is not UTF-8", ErrInvalidResolution)
		}
		if invalid := (syncmerge.Merger{}).ValidateArtifact(chosen.Kind, data); invalid != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidResolution, invalid)
		}
		resolutions = append(resolutions, storage.SyncResolution{ConflictID: chosen.ID, Data: data})
	case ResolveVersion:
		if req.Version < 0 || req.Version >= len(chosen.Candidates) {
			return nil, fmt.Errorf("%w: conflict %s has no version %d", ErrInvalidResolution, shortDigest(chosen.ID), req.Version)
		}
		candidate := chosen.Candidates[req.Version]
		data, keepErr := reader.kept(chosen, candidate)
		if keepErr != nil {
			return nil, keepErr
		}
		resolutions = append(resolutions, storage.SyncResolution{ConflictID: chosen.ID, Data: data})
		for _, coupled := range reader.coupled(chosen, candidate) {
			coupledData, coupledErr := reader.kept(coupled.conflict, coupled.candidate)
			if coupledErr != nil {
				return nil, coupledErr
			}
			resolutions = append(resolutions, storage.SyncResolution{ConflictID: coupled.conflict.ID, Data: coupledData})
			resolved = append(resolved, reader.ref(coupled.conflict))
		}
	default:
		return nil, fmt.Errorf("%w: choice %q is not version, current, or replace", ErrInvalidResolution, req.Choice)
	}
	txnID, err := commits.CommitSyncResolutions(resolutions)
	switch {
	case errors.Is(err, storage.ErrSyncConflictClosed), errors.Is(err, storage.ErrStaleArtifact):
		return nil, fmt.Errorf("%w: %w", ErrConflictStale, err)
	case err != nil:
		return nil, err
	}
	if err = r.Refresh(); err != nil {
		return nil, err
	}
	remaining, err := commits.SyncConflicts()
	if err != nil {
		return nil, err
	}
	return &ResolveResult{TxnID: txnID, Resolved: resolved, Remaining: len(remaining)}, nil
}

// SyncDiagnostics is what a person sends when sync misbehaves: the device's
// sync status, the conflicts' structure, and what the transport reported. It
// names files, devices, and transactions, never file content.
type SyncDiagnostics struct {
	Format      string    `json:"format"`
	GeneratedAt time.Time `json:"generated_at"`
	DeviceID    string    `json:"device_id"`
	// WorkspaceDigest digests the synced files (synclog.WorkspaceDigest):
	// two devices that converged report the same value.
	WorkspaceDigest string                   `json:"workspace_digest,omitempty"`
	Status          *SyncStatus              `json:"status"`
	Outbox          []string                 `json:"outbox"`
	Conflicts       []DiagnosticConflict     `json:"conflicts"`
	External        *storage.ExternalChanges `json:"external,omitempty"`
}

// DiagnosticConflict is a conflict without content.
type DiagnosticConflict struct {
	ID       string                   `json:"id"`
	Path     string                   `json:"path"`
	Kind     string                   `json:"kind"`
	Base     string                   `json:"base,omitempty"`
	Current  string                   `json:"current,omitempty"`
	Versions []DiagnosticVersion      `json:"versions"`
	Details  []synclog.ConflictDetail `json:"details"`
}

// DiagnosticVersion is one competing version's digest and writers.
type DiagnosticVersion struct {
	Digest    string           `json:"digest,omitempty"`
	Available bool             `json:"available"`
	Writers   []ConflictWriter `json:"writers"`
}

// SyncDiagnosticsFormat names the diagnostics document.
const SyncDiagnosticsFormat = "tt-sync-diagnostics/v1"

// SyncDiagnostics gathers diagnostics without writing anything; the status
// read never observes, publishes, or imports.
func (r *Runtime) SyncDiagnostics(names map[string]string) (*SyncDiagnostics, error) {
	status, err := r.SyncStatus()
	if err != nil {
		return nil, err
	}
	status.Conflicts = nil
	report := &SyncDiagnostics{Format: SyncDiagnosticsFormat, GeneratedAt: r.Now().UTC(), Status: status, Outbox: []string{}, Conflicts: []DiagnosticConflict{}}
	history := r.repo.svc.SyncHistory()
	if history == nil {
		return report, nil
	}
	pending, err := history.Pending()
	if err != nil {
		return nil, err
	}
	report.DeviceID = history.DeviceID()
	if report.WorkspaceDigest, err = synclog.WorkspaceDigest(r.DataDir()); err != nil {
		return nil, err
	}
	for _, manifest := range pending {
		report.Outbox = append(report.Outbox, manifest.TxnID)
	}
	views, err := r.SyncConflicts(names)
	if err != nil {
		return nil, err
	}
	for _, view := range views {
		entry := DiagnosticConflict{ID: view.ID, Path: view.Path, Kind: view.Kind, Versions: []DiagnosticVersion{}, Details: view.Details}
		if view.Base != nil {
			entry.Base = view.Base.Digest
		}
		if view.Current != nil {
			entry.Current = view.Current.Digest
		}
		for _, version := range view.Versions {
			entry.Versions = append(entry.Versions, DiagnosticVersion{Digest: version.Digest, Available: version.Available, Writers: version.Writers})
		}
		report.Conflicts = append(report.Conflicts, entry)
	}
	return report, nil
}

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

// replaceable reports whether a person may write replacement text for a
// kind: prose they can read and edit. Structured domain files and binary
// content take a version instead.
func replaceable(kind synclog.EffectKind) bool {
	switch kind {
	case synclog.EffectProjectDocument, synclog.EffectNote, synclog.EffectGlobalNote:
		return true
	default:
		return false
	}
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
	return nil, fmt.Errorf("%w: version %s is not on this device: %w", ErrInvalidResolution, shortDigest(*blob), err)
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
