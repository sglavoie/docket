package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/syncmerge"
	"github.com/sglavoie/tt/internal/transaction"
)

// ErrNotEnrolled refuses a sync operation on a workspace that was never
// enrolled. Such a workspace keeps the behavior it always had.
var ErrNotEnrolled = errors.New("engine: this workspace is not enrolled in sync")

// EnrollConfig is one desktop enrollment: the workspace to enroll, the local
// working copy that becomes the enrolled workspace, and the exchange folder.
type EnrollConfig struct {
	Original    string
	WorkingCopy string
	Exchange    string
	// SnapshotRoot overrides where the original's snapshot is kept.
	SnapshotRoot string
	// Machine is the device enrolling; its home also decides what counts as
	// iCloud Drive.
	Machine Machine
	Clock   app.Clock
	// AppVersion is advertised to the other devices.
	AppVersion string
}

// EnrollResult is what an enrollment did, step by step.
type EnrollResult struct {
	Enrollment *config.EnrollReport
	// Genesis is the working copy's first sync transaction.
	Genesis string
	// Exchange is the pass that published the initial history.
	Exchange *syncexchange.FolderReport
}

// enrollHook is the seam tests interrupt an enrollment at after the working
// copy is published; production leaves it nil.
var enrollHook func(stage string) error

// Enroll enrolls a desktop workspace in sync: it snapshots the original,
// builds the enrolled working copy outside iCloud and selects it, records the
// working copy's genesis, and publishes that initial history to the exchange
// folder. Running it again after an interruption finishes what is left: a
// published working copy is not copied twice, a recorded genesis is not
// recorded twice, and publication only sends what is still outstanding.
func Enroll(cfg EnrollConfig) (*EnrollResult, error) {
	if err := cfg.Machine.validate(); err != nil {
		return nil, err
	}
	clock := cfg.Clock
	if clock == nil {
		clock = app.SystemClock()
	}
	enrollment, err := config.EnrollWorkspace(config.EnrollOptions{
		Original:     cfg.Original,
		WorkingCopy:  cfg.WorkingCopy,
		Exchange:     cfg.Exchange,
		SnapshotRoot: cfg.SnapshotRoot,
		MachineHome:  cfg.Machine.Home,
		Now:          clock.Now,
	})
	if err != nil {
		return nil, err
	}
	result := &EnrollResult{Enrollment: enrollment}
	if enrollHook != nil {
		if hookErr := enrollHook("genesis"); hookErr != nil {
			return result, hookErr
		}
	}
	rt, err := Open(Config{DataDir: enrollment.WorkingCopy, Machine: cfg.Machine, Clock: clock})
	if err != nil {
		return result, fmt.Errorf("opening the working copy: %w", err)
	}
	// Open records the genesis; asking again surfaces a refusal Open could
	// only log, and is a no-op once it is recorded.
	if genesisErr := rt.recordGenesis(); genesisErr != nil {
		return result, genesisErr
	}
	if genesis, genesisErr := rt.repo.svc.SyncHistory().Genesis(); genesisErr == nil && genesis != nil {
		result.Genesis = genesis.TxnID
	}
	if enrollHook != nil {
		if hookErr := enrollHook("exchange"); hookErr != nil {
			return result, hookErr
		}
	}
	report, err := rt.SyncExchange(SyncExchangeOptions{Folder: enrollment.Exchange, AppVersion: cfg.AppVersion})
	result.Exchange = report
	if err != nil {
		return result, fmt.Errorf("publishing the initial history: %w", err)
	}
	return result, nil
}

// recordGenesis records an enrolled working copy's genesis when this device
// has recorded nothing yet, and does nothing otherwise.
func (r *Runtime) recordGenesis() error {
	history := r.repo.svc.SyncHistory()
	if history == nil || !r.state.Catalog().Enrolled() {
		return nil
	}
	state, err := history.LoadState()
	if err != nil {
		return err
	}
	if state != nil {
		return nil
	}
	_, err = r.repo.svc.CommitSyncGenesis()
	return err
}

// SyncExchangeOptions configures one exchange pass.
type SyncExchangeOptions struct {
	// Folder overrides the exchange folder the enrollment record names.
	Folder string
	// AppVersion is advertised to the other devices.
	AppVersion string
}

// SyncExchange runs one pass between this enrolled workspace and its exchange
// folder (see syncexchange.ExchangeFolder) and refreshes the snapshot when it
// imported anything. The report is returned even when the pass failed, so a
// caller can show what was reached.
func (r *Runtime) SyncExchange(opts SyncExchangeOptions) (*syncexchange.FolderReport, error) {
	catalog := r.state.Catalog()
	if !catalog.Enrolled() {
		return nil, ErrNotEnrolled
	}
	if err := r.recordGenesis(); err != nil {
		return nil, err
	}
	folder := opts.Folder
	if folder == "" {
		record, err := config.LoadEnrollmentRecord(r.DataDir())
		if err != nil {
			return nil, err
		}
		if record == nil || record.ExchangeFolder == "" {
			return nil, fmt.Errorf("this device has no exchange folder configured; run 'tt sync exchange --folder DIR'")
		}
		folder = record.ExchangeFolder
	}
	deviceName, _ := os.Hostname()
	report, err := syncexchange.ExchangeFolder(r.repo.svc, syncexchange.FolderOptions{
		Folder:      folder,
		WorkspaceID: catalog.WorkspaceID,
		DeviceName:  deviceName,
		AppVersion:  opts.AppVersion,
		Now:         r.clock.Now,
	})
	if report != nil && report.Imported > 0 {
		if refreshErr := r.Refresh(); refreshErr != nil && err == nil {
			err = refreshErr
		}
	}
	return report, err
}

// SyncStatus is everything a status surface shows about an enrolled
// workspace, kept apart the way the plan requires: what is committed locally,
// what is still outstanding in each direction, when an exchange last
// succeeded, what is held and which device must update, and the open
// conflicts.
type SyncStatus struct {
	Enrolled    bool   `json:"enrolled"`
	WorkspaceID string `json:"workspace_id"`
	DeviceID    string `json:"device_id"`
	// Schemas are the schemas this device records and reads at.
	Schemas    map[string]int           `json:"schemas"`
	Enrollment *config.EnrollmentRecord `json:"enrollment,omitempty"`
	// Genesis reports that the initial history is recorded.
	Genesis bool `json:"genesis"`
	// LocalTransactions counts the transactions this device authored.
	LocalTransactions uint64 `json:"local_transactions"`
	// OutstandingUp counts committed transactions not yet published.
	OutstandingUp int `json:"outstanding_up"`
	// Received is the highest sequence applied from each other device.
	Received map[string]uint64 `json:"received"`
	// LastExchange is the most recent pass, successful or not.
	LastExchange *syncexchange.FolderReport `json:"last_exchange,omitempty"`
	// LastSuccess is when a pass last succeeded; zero if never.
	LastSuccess time.Time `json:"last_success,omitempty"`
	// OutstandingDown counts incoming transactions found and not yet applied,
	// as of the last pass.
	OutstandingDown int `json:"outstanding_down"`
	// Held are incoming transactions waiting for an update, as of the last
	// pass; they are kept, never dropped.
	Held []syncexchange.Result `json:"held"`
	// Upgrades name every update the held transactions and the peers'
	// advertisements call for, deduplicated.
	Upgrades []string `json:"upgrades"`
	// Conflicts are the open sync conflicts.
	Conflicts []synclog.Conflict `json:"conflicts"`
}

// SyncStatus reads the workspace's sync status. It writes nothing. A
// workspace that is not enrolled reports Enrolled false and nothing else.
func (r *Runtime) SyncStatus() (*SyncStatus, error) {
	catalog := r.state.Catalog()
	status := &SyncStatus{
		Received:  map[string]uint64{},
		Held:      []syncexchange.Result{},
		Upgrades:  []string{},
		Conflicts: []synclog.Conflict{},
	}
	history := r.repo.svc.SyncHistory()
	if !catalog.Enrolled() || history == nil {
		return status, nil
	}
	status.Enrolled = true
	status.WorkspaceID = catalog.WorkspaceID
	status.DeviceID = history.DeviceID()
	status.Schemas = history.Schemas()
	record, err := config.LoadEnrollmentRecord(r.DataDir())
	if err != nil {
		return nil, err
	}
	status.Enrollment = record
	state, err := history.LoadState()
	if err != nil {
		return nil, err
	}
	if genesis, genesisErr := history.Genesis(); genesisErr != nil {
		return nil, genesisErr
	} else if genesis != nil {
		status.Genesis = true
	} else if state != nil && len(state.Received) > 0 {
		// A device that joined starts from another device's genesis.
		status.Genesis = true
	}
	if state != nil {
		status.LocalTransactions = state.NextSeq - 1
		status.OutstandingUp = len(state.Outbox)
		for device, seq := range state.Received {
			status.Received[device] = seq
		}
	}
	last, lastSuccess, err := syncexchange.LoadFolderStatus(history)
	if err != nil {
		return nil, err
	}
	status.LastExchange, status.LastSuccess = last, lastSuccess
	upgrades := map[string]bool{}
	if last != nil {
		status.OutstandingDown = last.OutstandingDown
		status.Held = append(status.Held, last.Held...)
		for _, held := range last.Held {
			if held.Upgrade != "" {
				upgrades[held.Upgrade] = true
			}
		}
		for _, peer := range last.Peers {
			if upgrade := peerUpgrade(status.Schemas, peer); upgrade != "" {
				upgrades[upgrade] = true
			}
		}
	}
	for upgrade := range upgrades {
		status.Upgrades = append(status.Upgrades, upgrade)
	}
	sort.Strings(status.Upgrades)
	conflicts, err := r.repo.svc.SyncConflicts()
	if err != nil {
		return nil, err
	}
	status.Conflicts = append(status.Conflicts, conflicts...)
	return status, nil
}

// peerUpgrade compares a peer's advertised schemas with this device's and
// names the update that would let the two read each other, or "". An
// advertisement only informs: it gates nothing.
func peerUpgrade(own map[string]int, peer syncexchange.DeviceAdvertisement) string {
	keys := make([]string, 0, len(own))
	for key := range own {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		theirs, ok := peer.Schemas[key]
		switch {
		case !ok || theirs == own[key]:
			continue
		case theirs < own[key]:
			return fmt.Sprintf("update tt on device %s: it reads %s schema %d, older than this device's %d", peer.DeviceID, key, theirs, own[key])
		default:
			return fmt.Sprintf("update tt on this device: device %s writes %s schema %d, newer than this device's %d", peer.DeviceID, key, theirs, own[key])
		}
	}
	return ""
}

// JoinConfig is a desktop joining a workspace another device enrolled.
type JoinConfig struct {
	// DataDir is the new local working copy; it must not exist yet, or be
	// empty, and it may not be inside iCloud Drive.
	DataDir string
	// Exchange is the folder the enrolling device published to.
	Exchange   string
	Machine    Machine
	Clock      app.Clock
	AppVersion string
}

// Join builds this device's working copy of an enrolled workspace from the
// history published in the exchange folder alone: it starts an empty history,
// imports the genesis and everything after it through the verified import
// path, records this device's exchange folder, selects the working copy, and
// opens it. Nothing is copied from another device's files, so the working copy
// is exactly what the history says.
func Join(cfg JoinConfig) (*Runtime, *syncexchange.FolderReport, error) {
	if err := cfg.Machine.validate(); err != nil {
		return nil, nil, err
	}
	clock := cfg.Clock
	if clock == nil {
		clock = app.SystemClock()
	}
	workspaceID, err := syncexchange.ReadExchangeIdentity(cfg.Exchange)
	if err != nil {
		return nil, nil, err
	}
	owner := newManager(filepath.Clean(cfg.DataDir), cfg.Machine).TransactionOwner()
	deviceID := owner
	if deviceID == "" {
		deviceID = transaction.DefaultMachineID()
	}
	if _, statErr := os.Stat(filepath.Join(cfg.Exchange, filepath.FromSlash(syncexchange.ExchangeRoot), "txns", deviceID)); statErr == nil {
		return nil, nil, fmt.Errorf("join: device %s already publishes to this exchange folder; each device keeps one working copy, so use that one", deviceID)
	}
	dataDir, err := config.PrepareJoinedWorkingCopy(cfg.DataDir, cfg.Exchange, cfg.Machine.Home)
	if err != nil {
		return nil, nil, err
	}
	svc, err := joiningCommits(dataDir, owner, clock)
	if err != nil {
		return nil, nil, err
	}
	deviceName, _ := os.Hostname()
	report, err := syncexchange.ExchangeFolder(svc, syncexchange.FolderOptions{
		Folder:      cfg.Exchange,
		WorkspaceID: workspaceID,
		DeviceName:  deviceName,
		AppVersion:  cfg.AppVersion,
		Now:         clock.Now,
	})
	if err != nil {
		return nil, report, fmt.Errorf("join: %w", err)
	}
	if recordErr := config.RecordJoinedWorkingCopy(dataDir, workspaceID, cfg.Exchange); recordErr != nil {
		return nil, report, recordErr
	}
	rt, err := Open(Config{DataDir: dataDir, Machine: cfg.Machine, Clock: clock})
	if err != nil {
		return nil, report, fmt.Errorf("join: opening the joined working copy: %w", err)
	}
	return rt, report, nil
}

// joiningCommits is the commit boundary a join imports through: an empty
// history that an enroll genesis may start, at the enrolled catalog schema.
func joiningCommits(dataDir, owner string, clock app.Clock) (*storage.CommitService, error) {
	schemas := LiveSchemas()
	schemas[synclog.SchemaCatalog] = config.SyncEnrolledConfigVersion
	history := syncHistory(dataDir, owner, clock, schemas)
	if history == nil {
		return nil, fmt.Errorf("join: this device has no sync identity")
	}
	return NewRepository(dataDir, nil, storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir}), owner).svc.
		WithSyncHistory(history).WithSyncMerger(syncmerge.Merger{}), nil
}

// JoinInboxConfig is a device joining an enrolled workspace from history its
// own transport already staged, as the iPhone does: Swift owns the coordinated
// exchange-folder reads, so the engine sees only the verified inbox.
type JoinInboxConfig struct {
	// DataDir is the new local working copy; it must not exist yet, or be
	// empty.
	DataDir string
	// Exchange names the exchange folder the transport read, recorded in the
	// device-local enrollment record. The engine never reads it.
	Exchange string
	// WorkspaceID is the identity the exchange folder declares.
	WorkspaceID string
	// InboxDir is where the transport staged the exchange's manifests and
	// blobs, laid out as the exchange is.
	InboxDir string
	Machine  Machine
	Clock    app.Clock
	IDs      app.IDGenerator
}

// JoinInbox builds this device's working copy of an enrolled workspace from a
// staged inbox, exactly as Join does from a folder: an empty history imports
// the genesis and everything after it through the verified import path, then
// the working copy is recorded and opened. It refuses a device that already
// publishes to the exchange and an inbox that never delivered the workspace,
// leaving the working copy empty-handed rather than half-selected.
func JoinInbox(cfg JoinInboxConfig) (*Runtime, *syncexchange.Report, error) {
	if err := cfg.Machine.validate(); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(cfg.WorkspaceID) == "" || !filepath.IsAbs(cfg.InboxDir) {
		return nil, nil, invalidConfig("join: a workspace id and an absolute inbox are required")
	}
	clock := cfg.Clock
	if clock == nil {
		clock = app.SystemClock()
	}
	owner := newManager(filepath.Clean(cfg.DataDir), cfg.Machine).TransactionOwner()
	deviceID := owner
	if deviceID == "" {
		deviceID = transaction.DefaultMachineID()
	}
	if _, statErr := os.Stat(filepath.Join(cfg.InboxDir, "txns", deviceID)); statErr == nil {
		return nil, nil, fmt.Errorf("join: device %s already publishes to this exchange folder; each device keeps one working copy, so use that one", deviceID)
	}
	dataDir, err := config.PrepareJoinedWorkingCopy(cfg.DataDir, cfg.Exchange, cfg.Machine.Home)
	if err != nil {
		return nil, nil, err
	}
	svc, err := joiningCommits(dataDir, owner, clock)
	if err != nil {
		return nil, nil, err
	}
	report, err := syncexchange.ImportInbox(svc, filepath.Clean(cfg.InboxDir))
	if err != nil {
		return nil, report, fmt.Errorf("join: %w", err)
	}
	if recordErr := config.RecordJoinedEnrollment(dataDir, cfg.WorkspaceID, cfg.Exchange); recordErr != nil {
		return nil, report, recordErr
	}
	rt, err := Open(Config{DataDir: dataDir, Machine: cfg.Machine, Clock: clock, IDs: cfg.IDs, RefuseMigration: true})
	if err != nil {
		return nil, report, fmt.Errorf("join: opening the joined working copy: %w", err)
	}
	return rt, report, nil
}
