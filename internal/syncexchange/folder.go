package syncexchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/util"
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
	var status folderStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, time.Time{}, fmt.Errorf("decoding sync transport status: %w", err)
	}
	return &status.Last, status.LastSuccess, nil
}

// folderStatus is the shape of status.json.
type folderStatus struct {
	Last        FolderReport `json:"last"`
	LastSuccess time.Time    `json:"last_success"`
}

// statusRefreshInterval bounds how far behind status.json's times may fall
// while passes find nothing new. Automatic passes run every few seconds and
// each rewrite costs two full flushes (the file and its directory), so an
// unchanged report is only rewritten once its times are this old. The
// Outliner's strip reads the age as "<1m" anyway, and the shell keeps its own
// copy current from each pass's report.
const statusRefreshInterval = time.Minute

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
	if err := checkAuthorSequences(root); err != nil {
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
	if len(pending) > 0 {
		paths := make([]string, 0, len(pending))
		for _, outgoing := range pending {
			paths = append(paths, outgoing.Manifest.Path)
		}
		if err := checkAuthorSequences(root, paths...); err != nil {
			return err
		}
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
// that succeeded, as this device's transport status. A report that says what
// the stored one says, apart from its times, is not rewritten until those
// times are statusRefreshInterval old.
func saveFolderStatus(history *synclog.History, report *FolderReport) error {
	path := StatusPath(history)
	previous, previousErr := readFolderStatus(path)
	var lastSuccess time.Time
	if previousErr == nil && previous != nil {
		lastSuccess = previous.LastSuccess
	}
	if report.Succeeded {
		lastSuccess = report.At
	}
	status := folderStatus{Last: *report, LastSuccess: lastSuccess}
	if previousErr == nil && previous != nil && statusCurrent(*previous, status) {
		return nil
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), util.DirPermissions); err != nil {
		return fmt.Errorf("creating the sync transport directory: %w", err)
	}
	if err := util.AtomicWriteFile(path, append(data, '\n'), util.FilePermissions); err != nil {
		return fmt.Errorf("writing sync transport status: %w", err)
	}
	return nil
}

// readFolderStatus reads status.json; a device that never exchanged reports
// (nil, nil).
func readFolderStatus(path string) (*folderStatus, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var status folderStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// statusCurrent reports whether stored already says what next says, apart
// from times younger than statusRefreshInterval. A clock that moved backwards
// counts as stale, so the file never keeps a time from the future.
func statusCurrent(stored, next folderStatus) bool {
	age := next.Last.At.Sub(stored.Last.At)
	if age < 0 || age >= statusRefreshInterval {
		return false
	}
	if stored.LastSuccess.IsZero() != next.LastSuccess.IsZero() {
		return false
	}
	stored.Last.At, stored.LastSuccess = next.Last.At, next.LastSuccess
	return sameJSON(stored, next)
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
