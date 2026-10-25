// Package synclog is a device's durable sync history: the immutable,
// content-addressed record of every transaction the device committed or
// imported, the blobs those records name, the device's identity, and the
// mutable tracking state — causal heads, the outbox of records awaiting
// publication, and the receipt high-water mark per remote device.
//
// Nothing here writes on its own. Every change to the history is planned as
// transaction.Op values that the caller stages in the same journal manifest as
// the workspace bytes the history describes, so crash recovery lands both or
// neither: a committed edit is never missing its outgoing record, and an
// incoming transaction is never acknowledged without being applied.
//
// The history lives in <data>/.docket/sync/. It is device-local: it is never
// exchanged as files, only published as the manifests and blobs it holds.
package synclog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Manifest format identifiers, version 1 of docs/mobile/sync-envelope.md.
const (
	ManifestFormat        = "tt-sync-txn"
	ManifestFormatVersion = 1
)

// Kind says why a transaction exists. A reader applies every kind the same way;
// the kind is provenance, never an instruction to restore an image.
type Kind string

const (
	KindUser       Kind = "user"
	KindUndo       Kind = "undo"
	KindRedo       Kind = "redo"
	KindMerge      Kind = "merge"
	KindResolution Kind = "resolution"
	KindRepair     Kind = "repair"
	// KindExternal imports a valid change made to the workspace outside a
	// commit — an editor, a script, or a checked domain save — observed against
	// the last indexed revision.
	KindExternal Kind = "external"
	// KindEnroll is the first transaction of an enrolled workspace: it writes
	// every synced file the working copy started from, so the history alone
	// can rebuild the workspace on a device that joins later.
	KindEnroll Kind = "enroll"
)

func (k Kind) valid() bool {
	switch k {
	case KindUser, KindUndo, KindRedo, KindMerge, KindResolution, KindRepair, KindExternal, KindEnroll:
		return true
	}
	return false
}

// EffectKind names the kind of artifact an effect changes.
type EffectKind string

const (
	EffectProjectDocument EffectKind = "project_document"
	EffectCatalog         EffectKind = "catalog"
	EffectPlanning        EffectKind = "planning"
	EffectLenses          EffectKind = "lenses"
	EffectNote            EffectKind = "note"
	EffectGlobalNote      EffectKind = "global_note"
	EffectAttachment      EffectKind = "attachment"
	EffectProjectTree     EffectKind = "project_tree"
)

// projectScoped reports whether an effect of this kind must name its project.
func (k EffectKind) projectScoped() bool {
	switch k {
	case EffectProjectDocument, EffectNote, EffectAttachment, EffectProjectTree:
		return true
	case EffectCatalog, EffectPlanning, EffectLenses, EffectGlobalNote:
		return false
	}
	return false
}

// EffectOp is what an effect does to its path.
type EffectOp string

const (
	OpPut    EffectOp = "put"
	OpDelete EffectOp = "delete"
	OpMove   EffectOp = "move"
)

// Effect is one durable file change. BaseBlob is the digest of the bytes the
// author replaced (nil when the path did not exist) and Blob the digest of the
// new bytes (nil for a delete). A move carries the unchanged digest in both.
type Effect struct {
	Kind       EffectKind `json:"kind"`
	ProjectID  string     `json:"project_id,omitempty"`
	Path       string     `json:"path"`
	Op         EffectOp   `json:"op"`
	TargetPath string     `json:"target_path,omitempty"`
	BaseBlob   *string    `json:"base_blob"`
	Blob       *string    `json:"blob"`
}

// Manifest is one immutable sync transaction. TxnID is the digest of the
// manifest's canonical JSON with txn_id omitted, so a manifest cannot change
// after it is minted.
type Manifest struct {
	Format        string         `json:"format"`
	FormatVersion int            `json:"format_version"`
	WorkspaceID   string         `json:"workspace_id"`
	TxnID         string         `json:"txn_id"`
	DeviceID      string         `json:"device_id"`
	DeviceSeq     uint64         `json:"device_seq"`
	Parents       []string       `json:"parents"`
	Kind          Kind           `json:"kind"`
	LocalTxnID    string         `json:"local_txn_id,omitempty"`
	CreatedAt     string         `json:"created_at"`
	Schemas       map[string]int `json:"schemas"`
	Requires      []string       `json:"requires"`
	Effects       []Effect       `json:"effects"`
}

// Sentinel errors. Callers classify with errors.Is.
var (
	// ErrInvalidManifest marks a manifest that is malformed or whose txn_id does
	// not match its content. It is never applied or acknowledged.
	ErrInvalidManifest = errors.New("synclog: invalid sync manifest")
	// ErrForeignWorkspace marks a manifest addressed to another workspace.
	ErrForeignWorkspace = errors.New("synclog: manifest belongs to another workspace")
	// ErrHeld marks a manifest this device cannot apply yet because its format,
	// schemas, or required capabilities are not this device's. It is held, never
	// acknowledged or dropped.
	ErrHeld = errors.New("synclog: manifest held until this device can read it")
	// ErrIncomplete marks a manifest whose parents, earlier device sequence
	// numbers, or blobs have not all been delivered yet.
	ErrIncomplete = errors.New("synclog: manifest is not yet complete")
	// ErrNeedsMerge marks a manifest whose base bytes are not this device's
	// current bytes, so applying it is reconciliation rather than fast-forward.
	ErrNeedsMerge = errors.New("synclog: manifest base differs from local state")
	// ErrAlreadyApplied marks a manifest this device has already received.
	ErrAlreadyApplied = errors.New("synclog: manifest already received")
	// ErrOwnTransaction marks a manifest this device authored itself.
	ErrOwnTransaction = errors.New("synclog: manifest was authored by this device")
	// ErrUnclassifiedPath marks a committed path the history has no effect kind
	// for. A commit refuses rather than publish an incomplete transaction.
	ErrUnclassifiedPath = errors.New("synclog: path has no sync effect kind")
)

var (
	digestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	deviceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// Digest returns the content address of data: "sha256:" and its hex digest.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ValidDigest reports whether s is a well-formed content address.
func ValidDigest(s string) bool { return digestPattern.MatchString(s) }

// ValidDeviceID reports whether id is usable as a device identity: a short
// token safe in a manifest, a file name, and a log line.
func ValidDeviceID(id string) bool { return deviceIDPattern.MatchString(id) }

func digestHex(digest string) string { return strings.TrimPrefix(digest, "sha256:") }

// CanonicalJSON is the manifest's canonical form with txn_id omitted: sorted
// keys, no insignificant whitespace, UTF-8 without HTML escaping.
func (m Manifest) CanonicalJSON() ([]byte, error) {
	m.TxnID = ""
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encoding manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic map[string]any
	if err := decoder.Decode(&generic); err != nil {
		return nil, fmt.Errorf("canonicalizing manifest: %w", err)
	}
	delete(generic, "txn_id")
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(generic); err != nil {
		return nil, fmt.Errorf("canonicalizing manifest: %w", err)
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

// ComputeTxnID returns the content address the manifest's txn_id must equal.
func (m Manifest) ComputeTxnID() (string, error) {
	canonical, err := m.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return Digest(canonical), nil
}

// seal stamps the manifest's content address.
func (m *Manifest) seal() error {
	id, err := m.ComputeTxnID()
	if err != nil {
		return err
	}
	m.TxnID = id
	return nil
}

// Validate checks structure and the content address. It does not judge whether
// this device can apply the manifest; PlanIncoming does that.
func (m *Manifest) Validate() error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, args...))
	}
	if m.Format != ManifestFormat {
		return invalid("format %q", m.Format)
	}
	if m.FormatVersion < 1 {
		return invalid("format_version %d", m.FormatVersion)
	}
	if m.WorkspaceID == "" {
		return invalid("missing workspace_id")
	}
	if !ValidDigest(m.TxnID) {
		return invalid("txn_id %q", m.TxnID)
	}
	if !ValidDeviceID(m.DeviceID) {
		return invalid("device_id %q", m.DeviceID)
	}
	if m.DeviceSeq < 1 {
		return invalid("device_seq %d", m.DeviceSeq)
	}
	if !m.Kind.valid() {
		return invalid("kind %q", m.Kind)
	}
	if !sort.StringsAreSorted(m.Parents) {
		return invalid("parents are not sorted")
	}
	for i, parent := range m.Parents {
		if !ValidDigest(parent) {
			return invalid("parent %q", parent)
		}
		if i > 0 && m.Parents[i-1] == parent {
			return invalid("duplicate parent %q", parent)
		}
		if parent == m.TxnID {
			return invalid("names itself as a parent")
		}
	}
	if len(m.Effects) == 0 {
		return invalid("no effects")
	}
	for i := range m.Effects {
		if err := m.Effects[i].validate(); err != nil {
			return invalid("effect %d: %v", i, err)
		}
	}
	want, err := m.ComputeTxnID()
	if err != nil {
		return invalid("%v", err)
	}
	if want != m.TxnID {
		return invalid("txn_id %s does not match content %s", m.TxnID, want)
	}
	return nil
}

func (e Effect) validate() error {
	if err := validRelativePath(e.Path); err != nil {
		return err
	}
	kind, _, local, err := Classify(CanonicalPath(e.Path))
	if err != nil {
		return err
	}
	if local {
		return fmt.Errorf("path %q is device-local", e.Path)
	}
	if kind != e.Kind {
		return fmt.Errorf("path %q is a %s, not a %s", e.Path, kind, e.Kind)
	}
	if e.Kind.projectScoped() != (e.ProjectID != "") {
		return fmt.Errorf("project_id %q does not suit a %s", e.ProjectID, e.Kind)
	}
	for _, digest := range []*string{e.BaseBlob, e.Blob} {
		if digest != nil && !ValidDigest(*digest) {
			return fmt.Errorf("blob %q", *digest)
		}
	}
	switch e.Op {
	case OpPut:
		if e.Blob == nil || e.TargetPath != "" {
			return fmt.Errorf("put requires a blob and no target_path")
		}
	case OpDelete:
		if e.BaseBlob == nil || e.Blob != nil || e.TargetPath != "" {
			return fmt.Errorf("delete requires a base_blob and no blob")
		}
	case OpMove:
		if e.BaseBlob == nil || e.Blob == nil || *e.BaseBlob != *e.Blob {
			return fmt.Errorf("move requires one unchanged blob")
		}
		if err := validRelativePath(e.TargetPath); err != nil {
			return fmt.Errorf("target_path: %w", err)
		}
		targetKind, _, targetLocal, err := Classify(CanonicalPath(e.TargetPath))
		if err != nil {
			return err
		}
		if targetLocal || targetKind != e.Kind {
			return fmt.Errorf("move changes %q's kind", e.Path)
		}
	default:
		return fmt.Errorf("op %q", e.Op)
	}
	return nil
}

// validRelativePath refuses anything but a clean, relative, '/'-separated path
// inside the workspace.
func validRelativePath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || path.Clean(p) != p {
		return fmt.Errorf("path %q is not a clean relative path", p)
	}
	if p == ".." || strings.HasPrefix(p, "../") {
		return fmt.Errorf("path %q escapes the workspace", p)
	}
	return nil
}

// Paths returns every workspace-relative path the manifest writes, moves, or
// deletes, sources and targets alike.
func (m *Manifest) Paths() []string {
	paths := make([]string, 0, len(m.Effects))
	for _, effect := range m.Effects {
		paths = append(paths, effect.Path)
		if effect.TargetPath != "" {
			paths = append(paths, effect.TargetPath)
		}
	}
	return paths
}

// Encode renders a manifest as the indented record stored and published.
func (m *Manifest) Encode() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeManifest parses and validates a published manifest strictly. A
// manifest of a newer format version is held rather than refused, since its
// unknown fields are the newer version's and not corruption.
func DecodeManifest(data []byte) (*Manifest, error) {
	var header struct {
		Format        string `json:"format"`
		FormatVersion int    `json:"format_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidManifest, err)
	}
	if header.Format == ManifestFormat && header.FormatVersion > ManifestFormatVersion {
		return nil, &HeldError{FormatVersion: header.FormatVersion}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var m Manifest
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidManifest, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}
