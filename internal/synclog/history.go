package synclog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/util"
)

// State and record format identifiers.
const (
	StateFormat          = "tt-sync-history"
	StateFormatVersion   = 1
	DeviceIdentityFormat = "tt-sync-device-identity"
	ReceiptFormat        = "tt-sync-receipt"
	IndexFormat          = "tt-sync-index"
)

// Schema keys a manifest's schemas object carries.
const (
	SchemaDomainEnvelope = "domain_envelope"
	SchemaCatalog        = "catalog"
	SchemaTaskMarkdown   = "task_markdown"
)

// Config names the device a history speaks for and the schemas it writes.
type Config struct {
	// DataDir is the workspace root holding .tt/ and projects/.
	DataDir string
	// DeviceID is this device's stable sync identity.
	DeviceID string
	// Schemas are the live artifact schemas this device writes; they must name
	// the domain envelope, catalog, and task Markdown schemas.
	Schemas map[string]int
	// Capabilities are what this device can apply. Nil derives the full set
	// from Schemas.
	Capabilities []string
	// Now is the clock stamped on records. Nil is the system clock.
	Now func() time.Time
}

// History plans durable sync-history appends for one device in one workspace.
// It holds no state of its own: every plan reads the history from disk under
// the caller's workspace commit lock, so recovery is the only reconciliation it
// ever needs.
type History struct {
	dataDir      string
	deviceID     string
	schemas      map[string]int
	capabilities map[string]bool
	now          func() time.Time
}

// New validates cfg and returns the device's history. It writes nothing.
func New(cfg Config) (*History, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("synclog: a data directory is required")
	}
	if !ValidDeviceID(cfg.DeviceID) {
		return nil, fmt.Errorf("synclog: device identity %q must be 1-64 letters, digits, '.', '_' or '-'", cfg.DeviceID)
	}
	for _, key := range []string{SchemaDomainEnvelope, SchemaCatalog, SchemaTaskMarkdown} {
		if cfg.Schemas[key] < 1 {
			return nil, fmt.Errorf("synclog: schema %q is required", key)
		}
	}
	schemas := make(map[string]int, len(cfg.Schemas))
	for key, value := range cfg.Schemas {
		schemas[key] = value
	}
	capabilities := cfg.Capabilities
	if capabilities == nil {
		capabilities = DefaultCapabilities(schemas)
	}
	capabilitySet := make(map[string]bool, len(capabilities))
	for _, capability := range capabilities {
		capabilitySet[capability] = true
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &History{
		dataDir:      filepath.Clean(cfg.DataDir),
		deviceID:     cfg.DeviceID,
		schemas:      schemas,
		capabilities: capabilitySet,
		now:          now,
	}, nil
}

// DefaultCapabilities is everything a device writing schemas can read.
func DefaultCapabilities(schemas map[string]int) []string {
	return []string{
		fmt.Sprintf("catalog.v%d", schemas[SchemaCatalog]),
		"planning.v1",
		"lenses.v1",
		fmt.Sprintf("project-document.v%d", schemas[SchemaTaskMarkdown]),
		"project-comments.v1",
		"check-ins.v1",
		"notes.v1",
		"attachments.v1",
		"project-tree.v1",
	}
}

// requirement is the capability a reader needs to apply an effect of kind.
func (h *History) requirement(kind EffectKind) string {
	switch kind {
	case EffectProjectDocument:
		return fmt.Sprintf("project-document.v%d", h.schemas[SchemaTaskMarkdown])
	case EffectCatalog:
		return fmt.Sprintf("catalog.v%d", h.schemas[SchemaCatalog])
	case EffectPlanning:
		return "planning.v1"
	case EffectLenses:
		return "lenses.v1"
	case EffectNote, EffectGlobalNote:
		return "notes.v1"
	case EffectAttachment:
		return "attachments.v1"
	default:
		return "project-tree.v1"
	}
}

// DeviceID is the identity this history records as.
func (h *History) DeviceID() string { return h.deviceID }

// Dir is <data>/.tt/sync.
func (h *History) Dir() string { return filepath.Join(h.dataDir, ".tt", "sync") }

func (h *History) deviceDir() string { return filepath.Join(h.Dir(), "devices", h.deviceID) }

// StatePath is the device's mutable tracking state.
func (h *History) StatePath() string { return filepath.Join(h.deviceDir(), "state.json") }

// IdentityPath is the device's immutable identity record.
func (h *History) IdentityPath() string { return filepath.Join(h.deviceDir(), "device.json") }

// BlobPath is where the blob with digest is retained.
func (h *History) BlobPath(digest string) string {
	hex := digestHex(digest)
	return filepath.Join(h.Dir(), "blobs", "sha256", hex[:2], hex)
}

func (h *History) indexPath(txnID string) string {
	hex := digestHex(txnID)
	return filepath.Join(h.Dir(), "index", hex[:2], hex+".json")
}

func (h *History) outgoingRecordPath(seq uint64, txnID string) string {
	return filepath.Join(h.deviceDir(), "txns", fmt.Sprintf("%010d-%s.json", seq, digestHex(txnID)))
}

func (h *History) receiptPath(author string, seq uint64, txnID string) string {
	return filepath.Join(h.deviceDir(), "incoming", author, fmt.Sprintf("%010d-%s.json", seq, digestHex(txnID)))
}

func (h *History) rel(abs string) string {
	rel, err := filepath.Rel(h.dataDir, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

func (h *History) abs(rel string) string {
	return filepath.Join(h.dataDir, filepath.FromSlash(rel))
}

// State is a device's mutable sync tracking. It is rewritten only inside the
// journal transaction whose records it accounts for.
type State struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	WorkspaceID   string `json:"workspace_id"`
	DeviceID      string `json:"device_id"`
	// NextSeq is the device_seq the next outgoing transaction takes.
	NextSeq uint64 `json:"next_seq"`
	// Heads are the transactions no other known transaction descends from: the
	// parents of the next local transaction.
	Heads []string `json:"heads"`
	// Outbox lists outgoing transactions not yet confirmed published, in
	// device_seq order.
	Outbox []OutboxEntry `json:"outbox"`
	// Received is the highest device_seq applied from each remote device.
	// Delivery is gap-free, so every lower sequence number is applied too.
	Received map[string]uint64 `json:"received"`
	// Tracked is the digest of each synced path as of the last transaction the
	// history recorded or applied: the last indexed revision of the workspace.
	Tracked map[string]string `json:"tracked"`
	// Indexed reports that Tracked covers every synced file, not only the ones
	// a transaction named: the device indexed the workspace it started from
	// (see baseline), so any difference from Tracked is an external change.
	Indexed bool `json:"indexed,omitempty"`
}

// OutboxEntry is one outgoing transaction awaiting publication.
type OutboxEntry struct {
	TxnID     string `json:"txn_id"`
	DeviceSeq uint64 `json:"device_seq"`
	Record    string `json:"record"`
}

type deviceIdentity struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	DeviceID      string `json:"device_id"`
	WorkspaceID   string `json:"workspace_id"`
	CreatedAt     string `json:"created_at"`
}

// IndexEntry locates one known transaction by its content address.
type IndexEntry struct {
	Format    string `json:"format"`
	TxnID     string `json:"txn_id"`
	DeviceID  string `json:"device_id"`
	DeviceSeq uint64 `json:"device_seq"`
	// Origin is "local" for a transaction this device authored and "incoming"
	// for one it received.
	Origin string `json:"origin"`
	// Record is the workspace-relative path of the stored manifest or receipt.
	Record string `json:"record"`
}

// Receipt is the immutable record that an incoming transaction was applied.
// It is written in the same journal transaction as the bytes it applied.
type Receipt struct {
	Format     string   `json:"format"`
	TxnID      string   `json:"txn_id"`
	LocalTxnID string   `json:"local_txn_id"`
	ReceivedAt string   `json:"received_at"`
	Manifest   Manifest `json:"manifest"`
	// Conflicts are the ids of the conflicts reconciliation left open once
	// this transaction was applied. A fast-forward names none.
	Conflicts []string `json:"conflicts,omitempty"`
}

// LoadState reads the device's tracking state. A device that has recorded
// nothing yet has none, reported as (nil, nil).
func (h *History) LoadState() (*State, error) {
	data, err := os.ReadFile(h.StatePath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading sync state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decoding sync state: %w", err)
	}
	if state.Format != StateFormat || state.FormatVersion != StateFormatVersion {
		return nil, fmt.Errorf("sync state %s has format %q version %d", h.StatePath(), state.Format, state.FormatVersion)
	}
	if state.DeviceID != h.deviceID {
		return nil, fmt.Errorf("sync state %s belongs to device %q", h.StatePath(), state.DeviceID)
	}
	if state.Received == nil {
		state.Received = map[string]uint64{}
	}
	if state.Tracked == nil {
		state.Tracked = map[string]string{}
	}
	return &state, nil
}

// Lookup returns the index entry of a known transaction, or false.
func (h *History) Lookup(txnID string) (IndexEntry, bool, error) {
	if !ValidDigest(txnID) {
		return IndexEntry{}, false, nil
	}
	data, err := os.ReadFile(h.indexPath(txnID))
	if os.IsNotExist(err) {
		return IndexEntry{}, false, nil
	}
	if err != nil {
		return IndexEntry{}, false, fmt.Errorf("reading sync index: %w", err)
	}
	var entry IndexEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return IndexEntry{}, false, fmt.Errorf("decoding sync index %s: %w", txnID, err)
	}
	return entry, true, nil
}

// Transaction loads and verifies a known transaction's manifest.
func (h *History) Transaction(txnID string) (*Manifest, error) {
	entry, ok, err := h.Lookup(txnID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("synclog: unknown transaction %s", txnID)
	}
	data, err := os.ReadFile(h.abs(entry.Record))
	if err != nil {
		return nil, fmt.Errorf("reading sync record: %w", err)
	}
	if entry.Origin == "incoming" {
		var receipt Receipt
		if err := json.Unmarshal(data, &receipt); err != nil {
			return nil, fmt.Errorf("decoding receipt %s: %w", txnID, err)
		}
		if err := receipt.Manifest.Validate(); err != nil {
			return nil, err
		}
		return &receipt.Manifest, nil
	}
	return DecodeManifest(data)
}

// Pending returns the outgoing transactions awaiting publication, in
// device_seq order, each verified against its content address.
func (h *History) Pending() ([]*Manifest, error) {
	state, err := h.LoadState()
	if err != nil || state == nil {
		return nil, err
	}
	out := make([]*Manifest, 0, len(state.Outbox))
	for _, entry := range state.Outbox {
		m, err := h.Transaction(entry.TxnID)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// Blob returns retained bytes by digest, verified against it.
func (h *History) Blob(digest string) ([]byte, error) {
	if !ValidDigest(digest) {
		return nil, fmt.Errorf("synclog: invalid digest %q", digest)
	}
	data, err := os.ReadFile(h.BlobPath(digest))
	if err != nil {
		return nil, err
	}
	if Digest(data) != digest {
		return nil, fmt.Errorf("synclog: blob %s is corrupt", digest)
	}
	return data, nil
}

// BlobSource supplies the bytes an incoming manifest names.
type BlobSource interface {
	Blob(digest string) ([]byte, error)
}

// Plan is a sync-history append ready to be staged into a journal manifest.
type Plan struct {
	// Ops are the journal operations, in order. For an incoming plan the
	// workspace effects come first; the history's own records follow and the
	// tracking state is always last.
	Ops []transaction.Op
	// MoveTargets are directories that must exist before the ops apply.
	MoveTargets []string
}

// newState starts a device's history in workspaceID.
func (h *History) newState(workspaceID string) *State {
	return &State{
		Format:        StateFormat,
		FormatVersion: StateFormatVersion,
		WorkspaceID:   workspaceID,
		DeviceID:      h.deviceID,
		NextSeq:       1,
		Heads:         []string{},
		Outbox:        []OutboxEntry{},
		Received:      map[string]uint64{},
		Tracked:       map[string]string{},
	}
}

func encodeJSON(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeOp(path string, data []byte) transaction.Op {
	return transaction.Op{Kind: transaction.OpWrite, Target: path, Data: data, Mode: util.FilePermissions}
}

// immutableOp stages a write of a record that may never be replaced. An
// existing record under the same name is a corrupted history, never a file to
// overwrite.
func immutableOp(path string, data []byte) (transaction.Op, error) {
	if _, err := os.Lstat(path); err == nil {
		return transaction.Op{}, fmt.Errorf("synclog: immutable record %s already exists", path)
	} else if !os.IsNotExist(err) {
		return transaction.Op{}, err
	}
	return writeOp(path, data), nil
}

// appendRecords stages the shared tail of every plan: the device identity on
// first use, the index entry, and the state rewrite last.
func (h *History) appendRecords(ops []transaction.Op, state *State, index *IndexEntry) ([]transaction.Op, error) {
	if _, err := os.Lstat(h.IdentityPath()); os.IsNotExist(err) {
		identity, encodeErr := encodeJSON(deviceIdentity{
			Format:        DeviceIdentityFormat,
			FormatVersion: 1,
			DeviceID:      h.deviceID,
			WorkspaceID:   state.WorkspaceID,
			CreatedAt:     h.now().UTC().Format(time.RFC3339Nano),
		})
		if encodeErr != nil {
			return nil, encodeErr
		}
		ops = append(ops, writeOp(h.IdentityPath(), identity))
	} else if err != nil {
		return nil, err
	}
	if index != nil {
		data, err := encodeJSON(index)
		if err != nil {
			return nil, err
		}
		op, err := immutableOp(h.indexPath(index.TxnID), data)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	sort.Strings(state.Heads)
	data, err := encodeJSON(state)
	if err != nil {
		return nil, err
	}
	return append(ops, writeOp(h.StatePath(), data)), nil
}

// catalogIdentity is the slice of the catalog the history needs: the
// workspace identity and which project id owns each slug.
type catalogIdentity struct {
	workspaceID string
	slugs       map[string]string
}

func parseCatalog(data []byte) (*catalogIdentity, error) {
	var envelope struct {
		WorkspaceID string `json:"workspace_id"`
		Data        struct {
			WorkspaceID string `json:"workspace_id"`
			Projects    []struct {
				ID   string `json:"id"`
				Slug string `json:"slug"`
			} `json:"projects"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decoding workspace catalog: %w", err)
	}
	identity := &catalogIdentity{workspaceID: envelope.WorkspaceID, slugs: map[string]string{}}
	if identity.workspaceID == "" {
		identity.workspaceID = envelope.Data.WorkspaceID
	}
	for _, project := range envelope.Data.Projects {
		identity.slugs[project.Slug] = project.ID
	}
	return identity, nil
}

const catalogRel = ".tt/workspace.json"

// mergeHeads drops parents from heads and adds txnID.
func mergeHeads(heads, parents []string, txnID string) []string {
	drop := make(map[string]bool, len(parents))
	for _, parent := range parents {
		drop[parent] = true
	}
	out := make([]string, 0, len(heads)+1)
	for _, head := range heads {
		if !drop[head] && head != txnID {
			out = append(out, head)
		}
	}
	out = append(out, txnID)
	sort.Strings(out)
	return out
}

func isNotExist(err error) bool { return errors.Is(err, os.ErrNotExist) }

func hasPathPrefix(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+string(filepath.Separator))
}

// WorkspacePath resolves a manifest's workspace-relative path to its absolute
// location in this workspace.
func (h *History) WorkspacePath(rel string) string { return h.abs(rel) }
