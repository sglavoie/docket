package synclog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/transaction"
)

// A checkpoint is where a device joining an enrolled workspace starts instead
// of the enrollment genesis. The genesis and every transaction after it are
// sealed at the schemas their author wrote, so once the workspace migrates, a
// device built at the new schemas can no longer replay its history from the
// beginning: every early transaction is held. A checkpoint is a complete,
// current-schema image of the synced workspace together with the exact set of
// transactions it summarizes, so a joiner installs the image, treats the
// summarized transactions as received, and imports only what came after.
//
// A checkpoint is never a transaction. It lives beside the history, under
// checkpoints/ in the exchange and in .docket/sync/checkpoints/ on a device,
// so a device that already holds the history never reads one, and an older
// build never sees one.

// Checkpoint format identifiers.
const (
	CheckpointFormat        = "tt-sync-checkpoint"
	CheckpointFormatVersion = 1
)

// KindCheckpoint is the kind of the one graph node an installed checkpoint
// contributes. It is not a manifest kind: no manifest may carry it, so a
// checkpoint can never be mistaken for, or published as, a transaction.
const KindCheckpoint Kind = "checkpoint"

// originCheckpoint is the index origin of a transaction an installed
// checkpoint summarizes: known, its content part of the checkpoint, its own
// manifest never held by this device.
const originCheckpoint = "checkpoint"

var (
	// ErrCheckpointRefused marks a checkpoint this device cannot author now,
	// or cannot install: the reason says which and why.
	ErrCheckpointRefused = errors.New("synclog: checkpoint refused")
	// ErrInvalidCheckpoint marks a checkpoint record that is malformed or whose
	// id does not match its content.
	ErrInvalidCheckpoint = errors.New("synclog: invalid sync checkpoint")
	// ErrSummarized marks a transaction this device knows only through the
	// checkpoint it joined from: its manifest was never held here.
	ErrSummarized = errors.New("synclog: transaction is summarized by a checkpoint")
)

// CoveredTxn is one transaction a checkpoint summarizes, with the parents
// that place it in the history. Only its identity and ancestry are kept: its
// effects are part of the checkpoint's image.
type CoveredTxn struct {
	TxnID     string   `json:"txn_id"`
	DeviceID  string   `json:"device_id"`
	DeviceSeq uint64   `json:"device_seq"`
	Parents   []string `json:"parents"`
}

// Checkpoint is an immutable, content-addressed image of a synced workspace at
// one point of its history. Covered is ancestor-closed and gap-free per
// device, Heads are its tips, and Files are puts of every synced file (no
// base, since the image replaces nothing).
type Checkpoint struct {
	Format        string         `json:"format"`
	FormatVersion int            `json:"format_version"`
	WorkspaceID   string         `json:"workspace_id"`
	CheckpointID  string         `json:"checkpoint_id"`
	DeviceID      string         `json:"device_id"`
	CreatedAt     string         `json:"created_at"`
	Schemas       map[string]int `json:"schemas"`
	Requires      []string       `json:"requires"`
	Heads         []string       `json:"heads"`
	Covered       []CoveredTxn   `json:"covered"`
	Files         []Effect       `json:"files"`

	covered map[string]bool
}

// ComputeID is the content address the checkpoint_id must equal: the digest
// of the record's canonical JSON with checkpoint_id omitted.
func (c Checkpoint) ComputeID() (string, error) {
	c.CheckpointID = ""
	raw, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encoding checkpoint: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic map[string]any
	if err := decoder.Decode(&generic); err != nil {
		return "", fmt.Errorf("canonicalizing checkpoint: %w", err)
	}
	delete(generic, "checkpoint_id")
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(generic); err != nil {
		return "", fmt.Errorf("canonicalizing checkpoint: %w", err)
	}
	return Digest(bytes.TrimSuffix(out.Bytes(), []byte("\n"))), nil
}

// Received is the highest sequence the checkpoint summarizes from each device.
func (c *Checkpoint) Received() map[string]uint64 {
	out := map[string]uint64{}
	for _, txn := range c.Covered {
		if txn.DeviceSeq > out[txn.DeviceID] {
			out[txn.DeviceID] = txn.DeviceSeq
		}
	}
	return out
}

// Blobs are the content addresses of every file in the image, sorted.
func (c *Checkpoint) Blobs() []string {
	seen := map[string]bool{}
	for _, file := range c.Files {
		if file.Blob != nil {
			seen[*file.Blob] = true
		}
	}
	return sortedKeys(seen)
}

// Validate checks the record's structure, its history's closure, and its
// content address. Whether this device can install it is decided apart.
func (c *Checkpoint) Validate() error {
	for _, check := range []func() error{c.validateHeader, c.validateCovered, c.validateFiles} {
		if err := check(); err != nil {
			return err
		}
	}
	want, err := c.ComputeID()
	if err != nil {
		return invalidCheckpoint("%v", err)
	}
	if want != c.CheckpointID {
		return invalidCheckpoint("checkpoint_id %s does not match content %s", c.CheckpointID, want)
	}
	return nil
}

func invalidCheckpoint(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCheckpoint, fmt.Sprintf(format, args...))
}

func (c *Checkpoint) validateHeader() error {
	switch {
	case c.Format != CheckpointFormat:
		return invalidCheckpoint("format %q", c.Format)
	case c.FormatVersion < 1:
		return invalidCheckpoint("format_version %d", c.FormatVersion)
	case c.WorkspaceID == "":
		return invalidCheckpoint("missing workspace_id")
	case !ValidDigest(c.CheckpointID):
		return invalidCheckpoint("checkpoint_id %q", c.CheckpointID)
	case !ValidDeviceID(c.DeviceID):
		return invalidCheckpoint("device_id %q", c.DeviceID)
	case len(c.Covered) == 0:
		return invalidCheckpoint("summarizes no transaction")
	case len(c.Heads) == 0 || !sort.StringsAreSorted(c.Heads):
		return invalidCheckpoint("heads are missing or unsorted")
	case len(c.Files) == 0:
		return invalidCheckpoint("holds no file")
	}
	for _, key := range []string{SchemaDomainEnvelope, SchemaCatalog, SchemaTaskMarkdown} {
		if c.Schemas[key] < 1 {
			return invalidCheckpoint("schema %q is missing", key)
		}
	}
	return nil
}

// validateCovered checks that the summarized history is gap-free per device,
// ancestor-closed, and that heads are exactly its tips.
func (c *Checkpoint) validateCovered() error {
	covered := make(map[string]bool, len(c.Covered))
	sequences := map[string][]uint64{}
	for _, txn := range c.Covered {
		if !ValidDigest(txn.TxnID) || !ValidDeviceID(txn.DeviceID) || txn.DeviceSeq < 1 {
			return invalidCheckpoint("covered transaction %q from %q seq %d", txn.TxnID, txn.DeviceID, txn.DeviceSeq)
		}
		if covered[txn.TxnID] || txn.TxnID == c.CheckpointID {
			return invalidCheckpoint("covered transaction %s appears twice", txn.TxnID)
		}
		covered[txn.TxnID] = true
		sequences[txn.DeviceID] = append(sequences[txn.DeviceID], txn.DeviceSeq)
	}
	for device, seqs := range sequences {
		sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
		for i, seq := range seqs {
			if seq != uint64(i+1) {
				return invalidCheckpoint("device %s has a gap or reused sequence", device)
			}
		}
	}
	hasChild := map[string]bool{}
	for _, txn := range c.Covered {
		if !sort.StringsAreSorted(txn.Parents) {
			return invalidCheckpoint("parents of %s are unsorted", txn.TxnID)
		}
		for _, parent := range txn.Parents {
			if !covered[parent] {
				return invalidCheckpoint("parent %s of %s is not summarized", parent, txn.TxnID)
			}
			hasChild[parent] = true
		}
	}
	for i, head := range c.Heads {
		if !covered[head] || hasChild[head] || (i > 0 && c.Heads[i-1] == head) {
			return invalidCheckpoint("head %s is not a tip of the summarized history", head)
		}
	}
	if tips := len(c.Covered) - len(hasChild); tips != len(c.Heads) {
		return invalidCheckpoint("heads name %d of the summarized history's %d tips", len(c.Heads), tips)
	}
	return nil
}

// validateFiles checks that the image is puts over nothing, one per path, at
// current paths, and includes the workspace catalog.
func (c *Checkpoint) validateFiles() error {
	paths := make(map[string]bool, len(c.Files))
	for i, file := range c.Files {
		if file.Op != OpPut || file.BaseBlob != nil {
			return invalidCheckpoint("file %d is not a put over nothing", i)
		}
		if CanonicalPath(file.Path) != file.Path {
			return invalidCheckpoint("file %d names the pre-v24 path %s", i, file.Path)
		}
		if err := file.validate(); err != nil {
			return invalidCheckpoint("file %d: %v", i, err)
		}
		if paths[file.Path] {
			return invalidCheckpoint("file %s appears twice", file.Path)
		}
		paths[file.Path] = true
	}
	if !paths[catalogRel] {
		return invalidCheckpoint("holds no workspace catalog")
	}
	return nil
}

// Encode renders the checkpoint as the record stored and published.
func (c *Checkpoint) Encode() ([]byte, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeCheckpoint parses and validates a checkpoint record strictly. A record
// of a newer format version is held, as a newer manifest is.
func DecodeCheckpoint(data []byte) (*Checkpoint, error) {
	var header struct {
		Format        string `json:"format"`
		FormatVersion int    `json:"format_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCheckpoint, err)
	}
	if header.Format == CheckpointFormat && header.FormatVersion > CheckpointFormatVersion {
		return nil, &HeldError{FormatVersion: header.FormatVersion}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var c Checkpoint
	if err := decoder.Decode(&c); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCheckpoint, err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// summarizes reports whether txnID is one of the transactions c stands for.
func (c *Checkpoint) summarizes(txnID string) bool {
	if c.covered == nil {
		c.covered = make(map[string]bool, len(c.Covered))
		for _, txn := range c.Covered {
			c.covered[txn.TxnID] = true
		}
	}
	return c.covered[txnID]
}

// node is the checkpoint as the one graph transaction that wrote every file of
// its image. It stands for every transaction the checkpoint summarizes.
func (c *Checkpoint) node() *Manifest {
	return &Manifest{
		Format:        ManifestFormat,
		FormatVersion: ManifestFormatVersion,
		WorkspaceID:   c.WorkspaceID,
		TxnID:         c.CheckpointID,
		DeviceID:      c.DeviceID,
		Parents:       []string{},
		Kind:          KindCheckpoint,
		CreatedAt:     c.CreatedAt,
		Schemas:       c.Schemas,
		Requires:      c.Requires,
		Effects:       c.Files,
	}
}

// CheckpointPath is where this device keeps the checkpoint record with id:
// the ones it authored and the one it joined from alike.
func (h *History) CheckpointPath(id string) string {
	return filepath.Join(h.Dir(), "checkpoints", digestHex(id)+".json")
}

// Checkpoints lists the checkpoint records this device keeps, most
// summarized first, then by id.
func (h *History) Checkpoints() ([]*Checkpoint, error) {
	entries, err := os.ReadDir(filepath.Join(h.Dir(), "checkpoints"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing sync checkpoints: %w", err)
	}
	var out []*Checkpoint
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		cache := h.manifests
		cache.cpMu.Lock()
		known := cache.records[entry.Name()]
		cache.cpMu.Unlock()
		if known != nil {
			out = append(out, known)
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(h.Dir(), "checkpoints", entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("reading sync checkpoint: %w", readErr)
		}
		c, decodeErr := DecodeCheckpoint(data)
		if decodeErr != nil {
			return nil, fmt.Errorf("sync checkpoint %s: %w", entry.Name(), decodeErr)
		}
		// A record is content-addressed and never rewritten, so its decoding
		// stands for as long as its name does.
		cache.cpMu.Lock()
		if cache.records == nil {
			cache.records = map[string]*Checkpoint{}
		}
		cache.records[entry.Name()] = c
		cache.cpMu.Unlock()
		out = append(out, c)
	}
	SortCheckpoints(out)
	return out, nil
}

// SortCheckpoints orders checkpoints most summarized first, ties by id, so
// every device choosing among the same records chooses the same one.
func SortCheckpoints(list []*Checkpoint) {
	sort.SliceStable(list, func(i, j int) bool {
		if len(list[i].Covered) != len(list[j].Covered) {
			return len(list[i].Covered) > len(list[j].Covered)
		}
		return list[i].CheckpointID < list[j].CheckpointID
	})
}

// installedCheckpoint is the checkpoint this device joined from, or nil.
func (h *History) installedCheckpoint() (*Checkpoint, error) {
	state, err := h.LoadState()
	if err != nil || state == nil || state.Checkpoint == "" {
		return nil, err
	}
	cache := h.manifests
	cache.cpMu.Lock()
	cached := cache.installed
	cache.cpMu.Unlock()
	if cached != nil && cached.CheckpointID == state.Checkpoint {
		return cached, nil
	}
	data, err := os.ReadFile(h.CheckpointPath(state.Checkpoint))
	if err != nil {
		return nil, fmt.Errorf("reading the installed sync checkpoint: %w", err)
	}
	c, err := DecodeCheckpoint(data)
	if err != nil {
		return nil, err
	}
	if c.CheckpointID != state.Checkpoint {
		return nil, fmt.Errorf("%w: the installed checkpoint record holds %s, not %s", ErrInvalidCheckpoint, c.CheckpointID, state.Checkpoint)
	}
	c.summarizes("")
	cache.cpMu.Lock()
	cache.installed = c
	cache.cpMu.Unlock()
	return c, nil
}

// CheckpointReadable reports why this device cannot install c, or nil: the
// image is written at its author's schemas, all of which govern it, so they
// must be this device's own, and its requirements must be ones it meets.
func (h *History) CheckpointReadable(c *Checkpoint) error {
	if c.FormatVersion != CheckpointFormatVersion {
		return &HeldError{FormatVersion: c.FormatVersion}
	}
	held := &HeldError{}
	for _, key := range []string{SchemaCatalog, SchemaDomainEnvelope, SchemaTaskMarkdown} {
		if c.Schemas[key] != h.schemas[key] {
			held.Schemas = append(held.Schemas, SchemaDifference{Key: key, Manifest: c.Schemas[key], Device: h.schemas[key]})
		}
	}
	if len(held.Schemas) > 0 {
		sort.Slice(held.Schemas, func(i, j int) bool { return held.Schemas[i].Key < held.Schemas[j].Key })
		return held
	}
	for _, requirement := range c.Requires {
		if !h.capabilities[requirement] {
			held.Missing = append(held.Missing, requirement)
		}
	}
	for _, file := range c.Files {
		if requirement := h.requirement(file.Kind); !h.capabilities[requirement] {
			held.Missing = append(held.Missing, requirement)
		}
	}
	if len(held.Missing) > 0 {
		sort.Strings(held.Missing)
		held.Missing = compactSorted(held.Missing)
		return held
	}
	return nil
}

func compactSorted(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i == 0 || in[i-1] != s {
			out = append(out, s)
		}
	}
	return out
}

// CheckpointDue reports whether this device should author a checkpoint now.
// With a checkpoint at its current schemas already kept, one is due once the
// history has grown every transactions past the newest. With none, one is due
// when a device joining at these schemas could not replay the history from its
// beginning — a root transaction is written at another schema, which is what a
// migration leaves behind — or when the history is already that long. The
// checkpoint a device joined from counts as kept, so joining never makes a
// device publish a copy of what it just installed. It reads the tracking
// state, the records this device keeps (decoded once each), and the cached
// graph only while no checkpoint is kept, so an exchange pass can ask on every
// run.
func (h *History) CheckpointDue(every int) (bool, error) {
	// A refusal stands until the history changes: what refused it — open
	// conflicts, unpublished work, an unreadable change — changes only then.
	cache := h.manifests
	cache.cpMu.Lock()
	deferredAt := cache.deferredAt
	cache.cpMu.Unlock()
	if deferredAt != nil {
		if current, statErr := os.Stat(h.StatePath()); statErr == nil && sameFile(deferredAt, current) {
			return false, nil
		}
	}
	state, err := h.LoadState()
	if err != nil || state == nil || !state.Indexed {
		return false, err
	}
	// Every sequence this device minted or received is a known transaction.
	known := int(state.NextSeq) - 1
	for _, seq := range state.Received {
		known += int(seq)
	}
	if known <= 0 {
		return false, nil
	}
	records, err := h.Checkpoints()
	if err != nil {
		return false, err
	}
	for _, c := range records {
		if h.CheckpointReadable(c) != nil {
			continue
		}
		return known-len(c.Covered) >= every, nil
	}
	if known >= every {
		return true, nil
	}
	g, err := h.loadGraph(nil)
	if err != nil {
		return false, err
	}
	for _, m := range g.txns {
		if len(m.Parents) > 0 {
			continue
		}
		for _, key := range []string{SchemaCatalog, SchemaDomainEnvelope, SchemaTaskMarkdown} {
			if m.Schemas[key] != h.schemas[key] {
				return true, nil
			}
		}
	}
	return false, nil
}

// DeferCheckpoint records that a due checkpoint was refused, so CheckpointDue
// answers false until the history changes.
func (h *History) DeferCheckpoint() {
	info, err := os.Stat(h.StatePath())
	if err != nil {
		return
	}
	cache := h.manifests
	cache.cpMu.Lock()
	cache.deferredAt = info
	cache.cpMu.Unlock()
}

// PlanCheckpoint plans this device's checkpoint of everything its history
// holds: the record, written beside the history, whose image is the tracked
// state of every synced file and whose summarized set is every known
// transaction. The image is the history's own last indexed revision rather
// than a fresh read of the disk, so it is exactly the state of the history it
// summarizes; the caller observes external changes first so that revision is
// the workspace's. It refuses (ErrCheckpointRefused) a device that has not
// indexed its workspace, holds unpublished transactions, or tracks a file
// whose bytes it no longer retains. Open conflicts are the caller's to refuse:
// judging them needs the merger.
func (h *History) PlanCheckpoint() (*Plan, *Checkpoint, error) {
	state, err := h.LoadState()
	if err != nil {
		return nil, nil, err
	}
	if state == nil || !state.Indexed {
		return nil, nil, fmt.Errorf("%w: this device has not indexed its workspace yet", ErrCheckpointRefused)
	}
	if len(state.Outbox) > 0 {
		return nil, nil, fmt.Errorf("%w: %d transaction(s) are not published yet; exchange first", ErrCheckpointRefused, len(state.Outbox))
	}
	g, err := h.loadGraph(nil)
	if err != nil {
		return nil, nil, err
	}
	if len(g.txns) == 0 {
		return nil, nil, fmt.Errorf("%w: the history holds no transaction", ErrCheckpointRefused)
	}
	var covered []CoveredTxn
	if g.checkpoint != nil {
		covered = append(covered, g.checkpoint.record.Covered...)
	}
	for _, m := range g.txns {
		if m.Kind == KindCheckpoint {
			continue
		}
		// The graph holds parents rewritten onto the checkpoint node; the
		// record keeps each transaction's own.
		original := m
		if o, rebased := g.original[m.TxnID]; rebased {
			original = o
		}
		covered = append(covered, CoveredTxn{
			TxnID: m.TxnID, DeviceID: m.DeviceID, DeviceSeq: m.DeviceSeq,
			Parents: append([]string{}, original.Parents...),
		})
	}
	sort.Slice(covered, func(i, j int) bool {
		if covered[i].DeviceID != covered[j].DeviceID {
			return covered[i].DeviceID < covered[j].DeviceID
		}
		return covered[i].DeviceSeq < covered[j].DeviceSeq
	})

	catalogDigest, ok := state.Tracked[catalogRel]
	if !ok {
		return nil, nil, fmt.Errorf("%w: the history tracks no workspace catalog", ErrCheckpointRefused)
	}
	catalogData, err := h.Blob(catalogDigest)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: the tracked workspace catalog is not retained: %w", ErrCheckpointRefused, err)
	}
	catalog, err := parseCatalog(catalogData)
	if err != nil {
		return nil, nil, err
	}
	if catalog.workspaceID != state.WorkspaceID {
		return nil, nil, fmt.Errorf("%w: the tracked catalog names workspace %s, not %s", ErrCheckpointRefused, catalog.workspaceID, state.WorkspaceID)
	}
	paths := make([]string, 0, len(state.Tracked))
	for rel := range state.Tracked {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	files := make([]Effect, 0, len(paths))
	requires := map[string]bool{}
	for _, rel := range paths {
		digest := state.Tracked[rel]
		kind, slug, local, classifyErr := Classify(rel)
		if classifyErr != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrCheckpointRefused, classifyErr)
		}
		if local {
			continue
		}
		if _, blobErr := h.Blob(digest); blobErr != nil {
			return nil, nil, fmt.Errorf("%w: the tracked bytes of %s are not retained: %w", ErrCheckpointRefused, rel, blobErr)
		}
		file := Effect{Kind: kind, Path: rel, Op: OpPut, Blob: digestPtr(digest)}
		if kind.projectScoped() {
			id, named := catalog.slugs[slug]
			if !named {
				return nil, nil, fmt.Errorf("%w: project directory %q is in no catalog", ErrCheckpointRefused, slug)
			}
			file.ProjectID = id
		}
		files = append(files, file)
		requires[h.requirement(kind)] = true
	}
	c := &Checkpoint{
		Format:        CheckpointFormat,
		FormatVersion: CheckpointFormatVersion,
		WorkspaceID:   state.WorkspaceID,
		DeviceID:      h.deviceID,
		CreatedAt:     h.now().UTC().Format(time.RFC3339Nano),
		Schemas:       h.Schemas(),
		Requires:      sortedKeys(requires),
		Heads:         g.heads(),
		Covered:       covered,
		Files:         files,
	}
	if c.CheckpointID, err = c.ComputeID(); err != nil {
		return nil, nil, err
	}
	if validErr := c.Validate(); validErr != nil {
		return nil, nil, fmt.Errorf("synclog: planned an invalid checkpoint: %w", validErr)
	}
	record, err := c.Encode()
	if err != nil {
		return nil, nil, err
	}
	op, err := immutableOp(h.CheckpointPath(c.CheckpointID), record)
	if err != nil {
		return nil, nil, err
	}
	return &Plan{Ops: []transaction.Op{op}}, c, nil
}

// PlanCheckpointInstall plans building an empty working copy from c: every
// file of its image, the record itself, an index entry for every transaction
// it summarizes, and a tracking state that has received exactly those, all in
// one journal transaction. Nil means this is not an empty, history-less
// working copy, where a checkpoint has no place; the caller imports item by
// item instead. A checkpoint this device cannot read is held (ErrHeld), one
// that names this device's own transactions is refused (ErrOwnTransaction),
// and one whose content is missing is incomplete (ErrIncomplete).
func (h *History) PlanCheckpointInstall(c *Checkpoint, blobs BlobSource, v Validator, localTxnID string) (*ReconcilePlan, error) {
	state, err := h.LoadState()
	if err != nil || state != nil {
		return nil, err
	}
	if _, err = os.Lstat(filepath.Join(h.Dir(), "index")); err == nil {
		return nil, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	occupied := false
	if err = h.walkSyncedFiles(func(_, _ string, _ fs.FileInfo) error {
		occupied = true
		return nil
	}); err != nil {
		return nil, err
	}
	if occupied {
		return nil, nil
	}
	if validErr := c.Validate(); validErr != nil {
		return nil, validErr
	}
	if readErr := h.CheckpointReadable(c); readErr != nil {
		return nil, readErr
	}
	for _, txn := range c.Covered {
		if txn.DeviceID == h.deviceID {
			return nil, fmt.Errorf("%w: checkpoint %s summarizes %s", ErrOwnTransaction, c.CheckpointID, txn.TxnID)
		}
	}
	o := newOverlay(h)
	plan := &ReconcilePlan{Expect: map[string]*string{}}
	state = h.newState(c.WorkspaceID)
	state.Indexed = true
	state.Checkpoint = c.CheckpointID
	state.Heads = append([]string{}, c.Heads...)
	state.Received = c.Received()
	var ops []transaction.Op
	for _, file := range c.Files {
		data, fetchErr := fetchBlob(blobs, *file.Blob)
		if fetchErr != nil {
			return nil, fetchErr
		}
		if v != nil && file.Kind.structured() {
			if validErr := v.ValidateArtifact(file.Kind, data); validErr != nil {
				return nil, fmt.Errorf("%w: %s in checkpoint %s: %w", ErrInvalidCheckpoint, file.Path, c.CheckpointID, validErr)
			}
		}
		abs := h.abs(file.Path)
		ops = append(ops, writeOp(abs, data))
		o.retainBytes(*file.Blob, data)
		plan.Expect[abs] = nil
		plan.MoveTargets = append(plan.MoveTargets, filepath.Dir(abs))
		state.Tracked[file.Path] = *file.Blob
	}
	ops = append(ops, o.blobOps...)
	record, err := c.Encode()
	if err != nil {
		return nil, err
	}
	recordPath := h.CheckpointPath(c.CheckpointID)
	if existing, readErr := os.ReadFile(recordPath); readErr == nil {
		if !bytes.Equal(existing, record) {
			return nil, fmt.Errorf("synclog: a different record is kept as checkpoint %s", c.CheckpointID)
		}
	} else {
		ops = append(ops, writeOp(recordPath, record))
	}
	for _, txn := range c.Covered {
		entry, encodeErr := encodeJSON(IndexEntry{
			Format: IndexFormat, TxnID: txn.TxnID, DeviceID: txn.DeviceID,
			DeviceSeq: txn.DeviceSeq, Origin: originCheckpoint, Record: h.rel(recordPath),
		})
		if encodeErr != nil {
			return nil, encodeErr
		}
		indexOp, indexErr := immutableOp(h.indexPath(txn.TxnID), entry)
		if indexErr != nil {
			return nil, indexErr
		}
		ops = append(ops, indexOp)
	}
	plan.Ops, err = h.appendRecords(ops, state, nil)
	return plan, err
}

// graphCheckpoint is an installed checkpoint as the graph sees it: one node
// standing for every transaction the record summarizes.
type graphCheckpoint struct {
	record  *Checkpoint
	covered map[string]bool
}

func newGraphCheckpoint(c *Checkpoint) *graphCheckpoint {
	covered := make(map[string]bool, len(c.Covered))
	for _, txn := range c.Covered {
		covered[txn.TxnID] = true
	}
	return &graphCheckpoint{record: c, covered: covered}
}

// rebase returns m with every summarized parent replaced by the checkpoint
// node, or m itself when it names none. The copy is the graph's alone: m's
// content address still names its own parents.
func (gc *graphCheckpoint) rebase(m *Manifest) *Manifest {
	if gc == nil {
		return m
	}
	rewritten := false
	parents := make([]string, 0, len(m.Parents))
	for _, parent := range m.Parents {
		if gc.covered[parent] {
			parent = gc.record.CheckpointID
			rewritten = true
		}
		parents = append(parents, parent)
	}
	if !rewritten {
		return m
	}
	sort.Strings(parents)
	copied := *m
	copied.Parents = compactSorted(parents)
	return &copied
}

// CheckpointGapError holds a transaction that descends from only part of the
// history the installed checkpoint summarizes: it was written concurrently
// with summarized work it never saw, so the checkpoint's image cannot stand in
// for its ancestry. Applying it as though it could would let it overwrite that
// work rather than merge with it, so it waits instead.
type CheckpointGapError struct {
	TxnID      string
	Checkpoint string
}

func (e *CheckpointGapError) Error() string {
	return fmt.Sprintf("%v: %s was written concurrently with history checkpoint %s summarizes", ErrHeld, e.TxnID, e.Checkpoint)
}

func (e *CheckpointGapError) Unwrap() error { return ErrHeld }

// checkpointGap holds m when this device joined from a checkpoint and m does
// not descend from every transaction it summarizes. The summarized history is
// ancestor-closed with the checkpoint's heads as its tips, so m descends from
// all of it exactly when its ancestry holds every head. Ancestry is walked
// through the transactions received since, down to the summarized ones; a
// transaction found to descend from all of it is remembered, since that never
// changes, and short-cuts every later walk through it.
func (h *History) checkpointGap(m *Manifest) error {
	c, err := h.installedCheckpoint()
	if err != nil || c == nil {
		return err
	}
	cache := h.manifests
	cache.cpMu.Lock()
	if cache.whole == nil || cache.wholeFor != c.CheckpointID {
		cache.whole, cache.wholeFor = map[string]bool{}, c.CheckpointID
	}
	whole := cache.whole
	cache.cpMu.Unlock()
	heads := make(map[string]bool, len(c.Heads))
	for _, head := range c.Heads {
		heads[head] = true
	}
	reached := map[string]bool{}
	seen := map[string]bool{}
	queue := append([]string{}, m.Parents...)
	for len(queue) > 0 && len(reached) < len(heads) {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		cache.cpMu.Lock()
		descends := whole[id]
		cache.cpMu.Unlock()
		if descends {
			return h.rememberWhole(m.TxnID)
		}
		entry, known, lookupErr := h.Lookup(id)
		if lookupErr != nil {
			return lookupErr
		}
		if !known {
			continue
		}
		if entry.Origin == originCheckpoint {
			if heads[id] {
				reached[id] = true
			}
			continue
		}
		parent, loadErr := h.Transaction(id)
		if loadErr != nil {
			return loadErr
		}
		queue = append(queue, parent.Parents...)
	}
	if len(reached) == len(heads) {
		return h.rememberWhole(m.TxnID)
	}
	return &CheckpointGapError{TxnID: m.TxnID, Checkpoint: c.CheckpointID}
}

func (h *History) rememberWhole(txnID string) error {
	cache := h.manifests
	cache.cpMu.Lock()
	if cache.whole != nil {
		cache.whole[txnID] = true
	}
	cache.cpMu.Unlock()
	return nil
}
