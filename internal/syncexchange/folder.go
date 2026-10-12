package syncexchange

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/util"
)

// The folder exchange is the desktop's own transport: one pass between a
// device's history and an exchange folder it can reach as an ordinary
// directory — iCloud Drive on a Mac, any shared folder on Linux. It speaks the
// same tt-sync/v1 layout apple/TTSync does, so a desktop and an iPhone can
// share one folder. It never deletes anything from the exchange, it never
// reads a file iCloud has not downloaded (a ".name.icloud" placeholder is
// reported unavailable, never as missing content), and it applies nothing
// except through ImportInbox, which verifies every byte first.

// Exchange identity and advertisement formats.
const (
	ExchangeRoot              = "tt-sync/v1"
	exchangeIdentityFormat    = "tt-sync-exchange"
	deviceAdvertisementFormat = "tt-sync-device"
	exchangeFormatVersion     = 1
	statusFormat              = "tt-sync-transport-status"
)

// ErrForeignExchange refuses an exchange folder whose identity names another
// workspace.
var ErrForeignExchange = errors.New("syncexchange: the exchange folder belongs to another workspace")

// FolderCommitter is the commit boundary a folder exchange publishes from and
// imports through.
type FolderCommitter interface {
	Committer
	CommitExternalChanges() (*storage.ExternalChanges, error)
	CommitSyncPublished(txnIDs []string) error
}

// FolderReport is one exchange pass, in the shape status surfaces read. Local
// commits, publication, delivery, and what is held are reported separately:
// a manifest written to the folder is published, not delivered.
type FolderReport struct {
	Format   string    `json:"format"`
	At       time.Time `json:"at"`
	Folder   string    `json:"folder"`
	DeviceID string    `json:"device_id"`
	// Succeeded reports a pass that reached the folder and finished; it is
	// what "last successful exchange" reads.
	Succeeded bool `json:"succeeded"`
	// Published are the outgoing transactions this pass wrote and verified.
	Published []string `json:"published"`
	// OutstandingUp is how many outgoing transactions are still unpublished.
	OutstandingUp int `json:"outstanding_up"`
	// Imported counts the incoming transactions this pass applied.
	Imported int `json:"imported"`
	// OutstandingDown counts incoming transactions found in the folder and not
	// yet applied: waiting for content or ancestors, held, or failing.
	OutstandingDown int `json:"outstanding_down"`
	// Held are incoming transactions this device cannot read yet, each with
	// the update it waits for. They are kept, never dropped.
	Held []Result `json:"held"`
	// Unavailable are exchange files present only as cloud placeholders.
	Unavailable []string `json:"unavailable"`
	// External is the observation the pass ran first, when it recorded or
	// held anything.
	External *storage.ExternalChanges `json:"external,omitempty"`
	// Peers are the other devices' advertisements found in the folder.
	Peers []DeviceAdvertisement `json:"peers"`
	// Results is the import pass in full.
	Results []Result `json:"results"`
	Errors  []string `json:"errors"`
}

// DeviceAdvertisement is the tt-sync-device record each device keeps at
// devices/<device-id>.json and only it rewrites.
type DeviceAdvertisement struct {
	Format             string         `json:"format"`
	FormatVersion      int            `json:"format_version"`
	DeviceID           string         `json:"device_id"`
	DeviceName         string         `json:"device_name"`
	Platform           string         `json:"platform"`
	AppVersion         string         `json:"app_version"`
	BridgeVersions     []int          `json:"bridge_versions"`
	SyncFormatVersions []int          `json:"sync_format_versions"`
	Schemas            map[string]int `json:"schemas"`
	Capabilities       []string       `json:"capabilities"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type exchangeIdentity struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	WorkspaceID   string `json:"workspace_id"`
}

// FolderOptions configures one pass.
type FolderOptions struct {
	// Folder holds the tt-sync/ tree.
	Folder string
	// WorkspaceID is the workspace the folder must belong to.
	WorkspaceID string
	// DeviceName and AppVersion fill this device's advertisement.
	DeviceName string
	AppVersion string
	// RequestDownloads asks the folder provider to deliver unavailable absolute
	// paths. Nil uses the platform provider (iCloud on macOS).
	RequestDownloads func([]string) error
	// Now is the pass clock; nil is the system clock.
	Now func() time.Time
}

// TransportDir is where the folder exchange keeps its inbox and status, in
// the device-local sync directory.
func TransportDir(history *synclog.History) string {
	return filepath.Join(history.Dir(), "transport")
}

// StatusPath is where the last pass's report is kept.
func StatusPath(history *synclog.History) string {
	return filepath.Join(TransportDir(history), "status.json")
}

// LoadFolderStatus reads the last pass's report and the last successful
// pass's time. A device that never exchanged reports (nil, zero, nil).
func LoadFolderStatus(history *synclog.History) (*FolderReport, time.Time, error) {
	data, err := os.ReadFile(StatusPath(history))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("reading sync transport status: %w", err)
	}
	var status struct {
		Last        FolderReport `json:"last"`
		LastSuccess time.Time    `json:"last_success"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, time.Time{}, fmt.Errorf("decoding sync transport status: %w", err)
	}
	return &status.Last, status.LastSuccess, nil
}

// ExchangeFolder runs one pass: observe external changes, publish every
// outgoing transaction and its content, advertise this device, fetch what
// other devices published, and import it. The report is also kept as this
// device's transport status. An error is returned only for a pass that could
// not reach or trust the folder; per-file trouble is in the report.
func ExchangeFolder(committer FolderCommitter, opts FolderOptions) (*FolderReport, error) {
	history := committer.SyncHistory()
	if history == nil {
		return nil, fmt.Errorf("syncexchange: exchange requires a sync history")
	}
	// Automatic and manual passes share the inbox and status file. Serialize
	// the entire transport pass with a separate device-local lock; commits keep
	// using their own workspace lock and remain possible during folder I/O.
	dir := TransportDir(history)
	if err := os.MkdirAll(dir, util.DirPermissions); err != nil {
		return nil, err
	}
	var report *FolderReport
	err := util.WithFileLock(filepath.Join(dir, "exchange.lock"), true, func() error {
		var passErr error
		report, passErr = exchangeFolderLocked(committer, opts)
		return passErr
	})
	return report, err
}

func exchangeFolderLocked(committer FolderCommitter, opts FolderOptions) (*FolderReport, error) {
	history := committer.SyncHistory()
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	report := &FolderReport{
		Format:      statusFormat,
		At:          now().UTC(),
		Folder:      opts.Folder,
		DeviceID:    history.DeviceID(),
		Published:   []string{},
		Held:        []Result{},
		Unavailable: []string{},
		Peers:       []DeviceAdvertisement{},
		Results:     []Result{},
		Errors:      []string{},
	}
	err := exchangePass(committer, history, opts, report, now)
	requestUnavailableDownloads(opts, report)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
	}
	report.Succeeded = err == nil
	if pending, pendingErr := history.Pending(); pendingErr == nil {
		report.OutstandingUp = len(pending)
	}
	if saveErr := saveFolderStatus(history, report); saveErr != nil && err == nil {
		err = saveErr
	}
	return report, err
}

func exchangePass(committer FolderCommitter, history *synclog.History, opts FolderOptions, report *FolderReport, now func() time.Time) error {
	root := filepath.Join(opts.Folder, filepath.FromSlash(ExchangeRoot))
	if info, err := os.Stat(opts.Folder); err != nil || !info.IsDir() {
		return fmt.Errorf("the exchange folder %s is not reachable", opts.Folder)
	}
	for _, path := range []string{".", "workspace.json"} {
		if exchangeFileUnavailable(filepath.Join(root, path)) {
			report.Unavailable = append(report.Unavailable, path)
			return fmt.Errorf("the exchange %s is not downloaded yet", path)
		}
	}
	if err := ensureIdentity(root, opts.WorkspaceID); err != nil {
		return err
	}
	external, err := committer.CommitExternalChanges()
	if err != nil {
		return fmt.Errorf("observing external changes: %w", err)
	}
	if external.TransactionID != "" || len(external.Held) > 0 {
		report.External = external
	}
	if publishErr := publish(committer, history, root, report); publishErr != nil {
		return publishErr
	}
	if advertiseErr := advertise(history, root, opts, now()); advertiseErr != nil {
		return advertiseErr
	}
	peers, err := readPeers(root, history.DeviceID())
	if err != nil {
		return err
	}
	report.Peers = peers
	inbox := filepath.Join(TransportDir(history), "inbox")
	if fetchErr := fetch(history, root, inbox, report); fetchErr != nil {
		return fetchErr
	}
	imported, err := ImportInbox(committer, inbox)
	if err != nil {
		return err
	}
	report.Results = imported.Results
	for _, result := range imported.Results {
		switch result.Outcome {
		case Imported:
			report.Imported++
		case Held:
			report.Held = append(report.Held, result)
			report.OutstandingDown++
		case Failed:
			report.OutstandingDown++
			report.Errors = append(report.Errors, fmt.Sprintf("applying %s from %s seq %d: %s",
				result.TxnID, result.DeviceID, result.DeviceSeq, result.Reason))
		case Incomplete:
			report.OutstandingDown++
		default:
			// Known, Invalid, and Foreign are neither waiting nor applied.
		}
	}
	return pruneInbox(history, inbox, imported)
}

// ensureIdentity creates the exchange identity or confirms the folder is this
// workspace's, refusing a foreign one before anything is written to it.
func ensureIdentity(root, workspaceID string) error {
	path := filepath.Join(root, "workspace.json")
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		var identity exchangeIdentity
		if decodeErr := json.Unmarshal(data, &identity); decodeErr != nil {
			return fmt.Errorf("the exchange identity %s is unreadable: %w", path, decodeErr)
		}
		if identity.WorkspaceID != workspaceID {
			return fmt.Errorf("%w: %s names workspace %s, not %s", ErrForeignExchange, path, identity.WorkspaceID, workspaceID)
		}
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		if placeholderExists(path) {
			return fmt.Errorf("the exchange identity %s is not downloaded yet", path)
		}
		return fmt.Errorf("reading the exchange identity: %w", err)
	case placeholderExists(path):
		return fmt.Errorf("the exchange identity %s is not downloaded yet", path)
	}
	encoded, err := json.MarshalIndent(exchangeIdentity{Format: exchangeIdentityFormat, FormatVersion: exchangeFormatVersion, WorkspaceID: workspaceID}, "", "  ")
	if err != nil {
		return err
	}
	return writeExchangeFile(path, append(encoded, '\n'))
}

// publish copies every outgoing transaction's blobs and then its manifest,
// verifies each copy by reading it back, and acknowledges what was verified.
func publish(committer FolderCommitter, history *synclog.History, root string, report *FolderReport) error {
	pending, err := Pending(history)
	if err != nil {
		return err
	}
	var published []string
	for _, outgoing := range pending {
		files := append(append([]File{}, outgoing.Blobs...), outgoing.Manifest)
		ok := true
		for _, file := range files {
			if err := publishFile(root, file); err != nil {
				report.Errors = append(report.Errors, err.Error())
				ok = false
				break
			}
		}
		if !ok {
			// Later transactions descend from this one; publishing them first
			// would hand peers manifests that must wait anyway.
			break
		}
		published = append(published, outgoing.TxnID)
	}
	if len(published) == 0 {
		return nil
	}
	if err := committer.CommitSyncPublished(published); err != nil {
		return fmt.Errorf("acknowledging publication: %w", err)
	}
	report.Published = published
	return nil
}

// publishFile writes one content-addressed file unless an intact copy is
// already there, then reads it back. A damaged copy is replaced: its name
// says what its content must be.
func publishFile(root string, file File) error {
	target := filepath.Join(root, filepath.FromSlash(file.Path))
	if existing, err := os.ReadFile(target); err == nil && synclog.Digest(existing) == file.Digest {
		return nil
	}
	data, err := os.ReadFile(file.Source)
	if err != nil {
		return fmt.Errorf("reading %s to publish: %w", file.Source, err)
	}
	if synclog.Digest(data) != file.Digest {
		return fmt.Errorf("the local copy of %s does not match its digest", file.Path)
	}
	if writeErr := writeExchangeFile(target, data); writeErr != nil {
		return writeErr
	}
	written, err := os.ReadFile(target)
	if err != nil || synclog.Digest(written) != file.Digest {
		return fmt.Errorf("the published copy of %s did not verify", file.Path)
	}
	return nil
}

// advertise rewrites this device's advertisement when what it says changed.
func advertise(history *synclog.History, root string, opts FolderOptions, at time.Time) error {
	path := filepath.Join(root, "devices", history.DeviceID()+".json")
	ad := DeviceAdvertisement{
		Format:             deviceAdvertisementFormat,
		FormatVersion:      exchangeFormatVersion,
		DeviceID:           history.DeviceID(),
		DeviceName:         opts.DeviceName,
		Platform:           platform(),
		AppVersion:         opts.AppVersion,
		BridgeVersions:     []int{},
		SyncFormatVersions: []int{synclog.ManifestFormatVersion},
		Schemas:            history.Schemas(),
		Capabilities:       history.Capabilities(),
		UpdatedAt:          at.UTC(),
	}
	if data, err := os.ReadFile(path); err == nil {
		var existing DeviceAdvertisement
		if json.Unmarshal(data, &existing) == nil {
			existing.UpdatedAt = ad.UpdatedAt
			if sameJSON(existing, ad) {
				return nil
			}
		}
	}
	encoded, err := json.MarshalIndent(ad, "", "  ")
	if err != nil {
		return err
	}
	return writeExchangeFile(path, append(encoded, '\n'))
}

func platform() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

func sameJSON(a, b any) bool {
	left, errA := json.Marshal(a)
	right, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(left, right)
}

// readPeers lists the other devices' advertisements. One that is unreadable
// or not downloaded is skipped: advertisements inform and never gate.
func readPeers(root, self string) ([]DeviceAdvertisement, error) {
	if exchangeFileUnavailable(filepath.Join(root, "devices")) {
		return []DeviceAdvertisement{}, nil
	}
	entries, err := os.ReadDir(filepath.Join(root, "devices"))
	if errors.Is(err, fs.ErrNotExist) {
		return []DeviceAdvertisement{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing device advertisements: %w", err)
	}
	peers := []DeviceAdvertisement{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") || strings.TrimSuffix(name, ".json") == self {
			continue
		}
		data, readable := readExchangeFile(filepath.Join(root, "devices", name))
		if !readable {
			continue
		}
		var ad DeviceAdvertisement
		if json.Unmarshal(data, &ad) == nil && ad.Format == deviceAdvertisementFormat {
			peers = append(peers, ad)
		}
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].DeviceID < peers[j].DeviceID })
	return peers, nil
}

// fetchHistory is what a fetch reads of this device's history.
type fetchHistory interface {
	DeviceID() string
	LoadState() (*synclog.State, error)
	Lookup(txnID string) (synclog.IndexEntry, bool, error)
	Blob(digest string) ([]byte, error)
}

// fetch stages, into the device-local inbox, every manifest another device
// published that this one has not applied, with whatever of its content is
// in the folder and intact. Missing and placeholder content is left for a
// later pass; ImportInbox decides what is complete.
func fetch(history fetchHistory, root, inbox string, report *FolderReport) error {
	state, stateErr := history.LoadState()
	if stateErr != nil {
		return stateErr
	}
	// Delivery is gap-free, so a sequence at or below the highest one applied
	// from its author is already here: its name alone says so, and neither its
	// index entry nor its manifest is read. A device that has recorded nothing
	// has received nothing.
	var received map[string]uint64
	if state != nil {
		received = state.Received
	}
	txns := filepath.Join(root, "txns")
	walkErr := filepath.WalkDir(txns, func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && abs == txns {
				return filepath.SkipDir
			}
			return err
		}
		if entry.IsDir() {
			if abs != txns && filepath.Base(abs) == history.DeviceID() {
				return filepath.SkipDir
			}
			if cloudDataless(abs) {
				rel, relErr := filepath.Rel(root, abs)
				if relErr != nil {
					return relErr
				}
				report.Unavailable = append(report.Unavailable, filepath.ToSlash(rel))
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, abs)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if placeholder, ok := placeholderTarget(rel); ok {
			report.Unavailable = append(report.Unavailable, placeholder)
			return nil
		}
		deviceID, seq, txnID, ok := ParseManifestPath(rel)
		if !ok || deviceID == history.DeviceID() || seq <= received[deviceID] {
			return nil
		}
		if _, known, lookupErr := history.Lookup(txnID); lookupErr != nil || known {
			return lookupErr
		}
		data, readable := readExchangeFile(abs)
		if !readable {
			// Listed but not delivered yet: fetched again on a later pass.
			report.Unavailable = append(report.Unavailable, rel)
			return nil
		}
		if err := stageInbox(inbox, rel, data); err != nil {
			return err
		}
		manifest, decodable := decodeStagedManifest(data)
		if !decodable {
			// Staged as it is: the import reports it invalid.
			return nil
		}
		for _, digest := range RequiredBlobs(manifest) {
			if _, err := history.Blob(digest); err == nil {
				continue
			}
			blobRel := BlobPath(digest)
			blobAbs := filepath.Join(root, filepath.FromSlash(blobRel))
			blob, readable := readExchangeFile(blobAbs)
			if !readable {
				if exchangeFileUnavailable(blobAbs) {
					report.Unavailable = append(report.Unavailable, blobRel)
				}
				continue
			}
			if synclog.Digest(blob) != digest {
				continue
			}
			if err := stageInbox(inbox, blobRel, blob); err != nil {
				return err
			}
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("reading the exchange folder: %w", walkErr)
	}
	sort.Strings(report.Unavailable)
	return nil
}

// stageInbox copies one exchange file into the inbox unless an identical copy
// is already staged.
func stageInbox(inbox, rel string, data []byte) error {
	target := filepath.Join(inbox, filepath.FromSlash(rel))
	if existing, err := os.ReadFile(target); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), util.DirPermissions); err != nil {
		return fmt.Errorf("creating the sync inbox: %w", err)
	}
	if err := util.AtomicWriteFile(target, data, util.FilePermissions); err != nil {
		return fmt.Errorf("staging %s: %w", rel, err)
	}
	return nil
}

// readFolderFile reads one file of the exchange folder.
var readFolderFile = os.ReadFile

// readExchangeFile reads one listed exchange file; false means it cannot be
// read yet, which the caller reports as unavailable rather than as an error.
func readExchangeFile(abs string) ([]byte, bool) {
	if exchangeFileUnavailable(abs) {
		return nil, false
	}
	data, err := readFolderFile(abs)
	return data, err == nil
}

// decodeStagedManifest decodes a staged manifest; false means it is damaged
// and is left for the import to report.
func decodeStagedManifest(data []byte) (*synclog.Manifest, bool) {
	manifest, err := synclog.DecodeManifest(data)
	return manifest, err == nil
}

// pruneInbox removes the staged copies a pass spent: manifests applied, known,
// or invalid (an invalid copy is fetched again), and blobs the history now
// retains. Held, incomplete, and failing manifests stay for the next pass.
func pruneInbox(history *synclog.History, inbox string, report *Report) error {
	for _, result := range report.Results {
		switch result.Outcome {
		case Imported, Known, Invalid, Foreign:
			if err := os.Remove(filepath.Join(inbox, filepath.FromSlash(result.Path))); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("clearing the sync inbox: %w", err)
			}
		default:
			// Held, incomplete, and failing manifests stay for the next pass.
		}
	}
	blobs := filepath.Join(inbox, "blobs")
	return filepath.WalkDir(blobs, func(abs string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		digest := "sha256:" + entry.Name()
		if _, retained := history.Blob(digest); retained == nil {
			if err := os.Remove(abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("clearing the sync inbox: %w", err)
			}
		}
		return nil
	})
}

// requestUnavailableDownloads submits a bounded, deduplicated batch each pass.
// A request is not delivery: entries stay unavailable until a later pass can
// read and verify them. Provider failures remain visible in the saved report.
func requestUnavailableDownloads(opts FolderOptions, report *FolderReport) {
	slices.Sort(report.Unavailable)
	report.Unavailable = slices.Compact(report.Unavailable)
	if len(report.Unavailable) == 0 {
		return
	}
	root, err := filepath.Abs(filepath.Join(opts.Folder, filepath.FromSlash(ExchangeRoot)))
	if err != nil {
		report.Errors = append(report.Errors, "locating cloud downloads: "+err.Error())
		return
	}
	paths := make([]string, 0, min(len(report.Unavailable), 128))
	for _, rel := range report.Unavailable[:min(len(report.Unavailable), 128)] {
		paths = append(paths, filepath.Join(root, filepath.FromSlash(rel)))
	}
	request := opts.RequestDownloads
	if request == nil {
		request = requestCloudDownloads
	}
	if err := request(paths); err != nil {
		report.Errors = append(report.Errors, err.Error())
	}
}

func exchangeFileUnavailable(path string) bool {
	return placeholderExists(path) || cloudDataless(path)
}

// placeholderTarget maps an iCloud placeholder (".name.icloud") to the path
// of the file it stands for.
func placeholderTarget(rel string) (string, bool) {
	dir, base := filepath.Split(filepath.FromSlash(rel))
	if !strings.HasPrefix(base, ".") || !strings.HasSuffix(base, ".icloud") || len(base) <= len("..icloud") {
		return "", false
	}
	return filepath.ToSlash(filepath.Join(dir, strings.TrimSuffix(strings.TrimPrefix(base, "."), ".icloud"))), true
}

// placeholderExists reports that iCloud holds path only as a placeholder.
func placeholderExists(path string) bool {
	_, err := os.Stat(filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".icloud"))
	return err == nil
}

// writeExchangeFile writes one file into the exchange through a temporary
// sibling and a rename, so a peer never reads half of it.
func writeExchangeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), util.DirPermissions); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := util.AtomicWriteFile(path, data, util.FilePermissions); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// saveFolderStatus keeps the pass's report, and the time of the last pass
// that succeeded, as this device's transport status.
func saveFolderStatus(history *synclog.History, report *FolderReport) error {
	_, lastSuccess, err := LoadFolderStatus(history)
	if err != nil {
		lastSuccess = time.Time{}
	}
	if report.Succeeded {
		lastSuccess = report.At
	}
	status := struct {
		Last        *FolderReport `json:"last"`
		LastSuccess time.Time     `json:"last_success"`
	}{Last: report, LastSuccess: lastSuccess}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	path := StatusPath(history)
	if err := os.MkdirAll(filepath.Dir(path), util.DirPermissions); err != nil {
		return fmt.Errorf("creating the sync transport directory: %w", err)
	}
	if err := util.AtomicWriteFile(path, append(data, '\n'), util.FilePermissions); err != nil {
		return fmt.Errorf("writing sync transport status: %w", err)
	}
	return nil
}

// ReadExchangeIdentity reads the workspace an exchange folder belongs to.
func ReadExchangeIdentity(folder string) (string, error) {
	path := filepath.Join(folder, filepath.FromSlash(ExchangeRoot), "workspace.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if placeholderExists(path) {
			return "", fmt.Errorf("the exchange identity %s is not downloaded yet", path)
		}
		return "", fmt.Errorf("%s holds no tt exchange: %w", folder, err)
	}
	var identity exchangeIdentity
	if err := json.Unmarshal(data, &identity); err != nil || identity.Format != exchangeIdentityFormat || identity.WorkspaceID == "" {
		return "", fmt.Errorf("the exchange identity %s is unreadable", path)
	}
	return identity.WorkspaceID, nil
}
