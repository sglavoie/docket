package syncexchange

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sglavoie/docket/internal/synclog"
)

// CheckpointInterval is how many transactions a device lets its history grow
// past its newest checkpoint before it authors another. A device with no
// checkpoint at its current schemas authors one on its next pass.
const CheckpointInterval = 500

var checkpointPathPattern = regexp.MustCompile(`^checkpoints/([0-9a-f]{64})\.json$`)

// CheckpointPath is where the checkpoint record with id lives under the
// exchange's tt-sync/v1 root.
func CheckpointPath(id string) string {
	return "checkpoints/" + strings.TrimPrefix(id, "sha256:") + ".json"
}

// ParseCheckpointPath reads the checkpoint id a record path promises.
func ParseCheckpointPath(rel string) (string, bool) {
	match := checkpointPathPattern.FindStringSubmatch(rel)
	if match == nil {
		return "", false
	}
	return "sha256:" + match[1], true
}

// checkpointInstaller can build an empty working copy from a checkpoint in one
// checked transaction; it reports false where a checkpoint has no place.
type checkpointInstaller interface {
	InstallCheckpoint(*synclog.Checkpoint, synclog.BlobSource) (string, bool, error)
}

// checkpointAuthor records this device's checkpoint of its history.
type checkpointAuthor interface {
	CommitSyncCheckpoint() (*synclog.Checkpoint, error)
}

// CheckpointFiles are the files publishing c writes: every blob of its image,
// then the record last, so a peer never reads a record whose content is not
// already on its way.
func CheckpointFiles(history *synclog.History, c *synclog.Checkpoint) ([]File, error) {
	files := make([]File, 0, len(c.Files)+1)
	for _, digest := range c.Blobs() {
		blob, err := history.Blob(digest)
		if err != nil {
			return nil, fmt.Errorf("syncexchange: checkpoint %s names blob %s: %w", c.CheckpointID, digest, err)
		}
		files = append(files, File{Path: BlobPath(digest), Source: history.BlobPath(digest), Digest: digest, Size: int64(len(blob))})
	}
	source := history.CheckpointPath(c.CheckpointID)
	record, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("reading checkpoint record: %w", err)
	}
	return append(files, File{Path: CheckpointPath(c.CheckpointID), Source: source, Digest: synclog.Digest(record), Size: int64(len(record))}), nil
}

// OwnCheckpoints are the checkpoints this device authored, most summarized
// first.
func OwnCheckpoints(history *synclog.History) ([]*synclog.Checkpoint, error) {
	records, err := history.Checkpoints()
	if err != nil {
		return nil, err
	}
	var out []*synclog.Checkpoint
	for _, c := range records {
		if c.DeviceID == history.DeviceID() {
			out = append(out, c)
		}
	}
	return out, nil
}

// stagedCheckpoint is one checkpoint record a transport staged in an inbox.
type stagedCheckpoint struct {
	result     Result
	checkpoint *synclog.Checkpoint
}

// importCheckpoints decides what the checkpoints staged under inbox are for.
// Only a device that has recorded no history uses one: it installs the most
// summarized checkpoint it can read, once every blob of its image is staged,
// and every other staged record is then spent. A device with history spends
// them all unread. workspaceID, when set, refuses a checkpoint of another
// workspace.
func importCheckpoints(committer Committer, history *synclog.History, inbox, workspaceID string, blobs inboxBlobs) ([]Result, error) {
	staged, err := readStagedCheckpoints(inbox, workspaceID)
	if err != nil || len(staged) == 0 {
		return nil, err
	}
	results := func() []Result {
		out := make([]Result, 0, len(staged))
		for _, item := range staged {
			out = append(out, item.result)
		}
		return out
	}
	state, err := history.LoadState()
	if err != nil {
		return nil, err
	}
	installer, canInstall := committer.(checkpointInstaller)
	if state != nil || !canInstall {
		for _, item := range staged {
			if item.result.Outcome == "" {
				item.result.Outcome, item.result.Reason = Known, "this device already holds its history"
			}
		}
		return results(), nil
	}
	var candidates []*stagedCheckpoint
	for _, item := range staged {
		if item.result.Outcome != "" {
			continue
		}
		if readErr := history.CheckpointReadable(item.checkpoint); readErr != nil {
			item.result.Outcome, item.result.Reason = Held, readErr.Error()
			item.result.Upgrade = synclog.RequiredUpgrade(readErr, item.checkpoint.DeviceID)
			continue
		}
		candidates = append(candidates, item)
	}
	if len(candidates) == 0 {
		return results(), nil
	}
	list := make([]*synclog.Checkpoint, 0, len(candidates))
	byID := make(map[string]*stagedCheckpoint, len(candidates))
	for _, item := range candidates {
		list = append(list, item.checkpoint)
		byID[item.checkpoint.CheckpointID] = item
	}
	synclog.SortCheckpoints(list)
	best := byID[list[0].CheckpointID]
	// The most summarized checkpoint is waited for rather than passed over
	// for an older one whose content happens to be here: every device then
	// starts from the same record, and from the least history to import.
	for _, digest := range best.checkpoint.Blobs() {
		if !blobs.has(digest) {
			best.result.Missing = append(best.result.Missing, digest)
		}
	}
	if len(best.result.Missing) > 0 {
		best.result.Outcome = Incomplete
		best.result.Reason = fmt.Sprintf("%d required blob(s) not yet delivered", len(best.result.Missing))
		for _, item := range candidates {
			if item != best {
				item.result.Outcome, item.result.Reason = Incomplete, "waiting for checkpoint "+best.checkpoint.CheckpointID
			}
		}
		return results(), nil
	}
	id, accepted, err := installer.InstallCheckpoint(best.checkpoint, blobs)
	if err != nil {
		return nil, fmt.Errorf("installing sync checkpoint %s: %w", best.checkpoint.CheckpointID, err)
	}
	for _, item := range candidates {
		switch {
		case !accepted:
			item.result.Outcome, item.result.Reason = Known, "not used: this working copy already holds files"
		case item == best:
			item.result.Outcome, item.result.LocalTxnID = Imported, id
		default:
			item.result.Outcome, item.result.Reason = Known, "superseded by checkpoint "+best.checkpoint.CheckpointID
		}
	}
	return results(), nil
}

// readStagedCheckpoints decodes every record staged under inbox/checkpoints.
// A record that is damaged or named for other content is invalid, one of
// another workspace foreign, and one of a newer format held; the rest are
// left undecided.
func readStagedCheckpoints(inbox, workspaceID string) ([]*stagedCheckpoint, error) {
	dir := filepath.Join(inbox, "checkpoints")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading staged checkpoints: %w", err)
	}
	var out []*stagedCheckpoint
	for _, entry := range entries {
		rel := "checkpoints/" + entry.Name()
		id, ok := ParseCheckpointPath(rel)
		if entry.IsDir() || !ok {
			continue
		}
		item := &stagedCheckpoint{result: Result{Path: rel, TxnID: id}}
		out = append(out, item)
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		c, decodeErr := synclog.DecodeCheckpoint(data)
		switch {
		case errors.Is(decodeErr, synclog.ErrHeld):
			item.result.Outcome, item.result.Reason = Held, decodeErr.Error()
			item.result.Upgrade = synclog.RequiredUpgrade(decodeErr, "")
		case decodeErr != nil:
			item.result.Outcome, item.result.Reason = Invalid, decodeErr.Error()
		case c.CheckpointID != id:
			item.result.Outcome, item.result.Reason = Invalid, fmt.Sprintf("staged as %s but carries %s", rel, c.CheckpointID)
		case workspaceID != "" && c.WorkspaceID != workspaceID:
			item.result.DeviceID = c.DeviceID
			item.result.Outcome, item.result.Reason = Foreign, fmt.Sprintf("checkpoint of workspace %s, not %s", c.WorkspaceID, workspaceID)
		default:
			item.result.DeviceID = c.DeviceID
			item.checkpoint = c
		}
	}
	return out, nil
}

// fetchCheckpoints stages, for a device that has recorded no history, every
// checkpoint record the exchange holds, and the image of the one it will
// install: the most summarized it can read. A device with history needs none.
func fetchCheckpoints(history fetchHistory, root, inbox string, report *FolderReport) error {
	dir := filepath.Join(root, "checkpoints")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the exchange checkpoints: %w", err)
	}
	var readable []*synclog.Checkpoint
	for _, entry := range entries {
		rel := "checkpoints/" + entry.Name()
		if placeholder, ok := placeholderTarget(rel); ok {
			report.Unavailable = append(report.Unavailable, placeholder)
			continue
		}
		id, ok := ParseCheckpointPath(rel)
		if entry.IsDir() || !ok {
			continue
		}
		data, available := readExchangeFile(filepath.Join(dir, entry.Name()))
		if !available {
			report.Unavailable = append(report.Unavailable, rel)
			continue
		}
		if err := stageInbox(inbox, rel, data); err != nil {
			return err
		}
		c, decodeErr := synclog.DecodeCheckpoint(data)
		if decodeErr != nil || c.CheckpointID != id || history.CheckpointReadable(c) != nil {
			continue
		}
		readable = append(readable, c)
	}
	if len(readable) == 0 {
		return nil
	}
	synclog.SortCheckpoints(readable)
	for _, digest := range readable[0].Blobs() {
		if _, err := history.Blob(digest); err == nil {
			continue
		}
		if err := stageExchangeBlob(root, inbox, digest, report); err != nil {
			return err
		}
	}
	return nil
}

// stageExchangeBlob stages one blob from the exchange when it is delivered
// and intact, and reports it unavailable when it is a cloud placeholder.
func stageExchangeBlob(root, inbox, digest string, report *FolderReport) error {
	blobRel := BlobPath(digest)
	blobAbs := filepath.Join(root, filepath.FromSlash(blobRel))
	blob, readable := readExchangeFile(blobAbs)
	if !readable {
		if exchangeFileUnavailable(blobAbs) {
			report.Unavailable = append(report.Unavailable, blobRel)
		}
		return nil
	}
	if synclog.Digest(blob) != digest {
		return nil
	}
	return stageInbox(inbox, blobRel, blob)
}

// publishCheckpoints copies every checkpoint this device authored that the
// exchange does not hold yet: its image's blobs first, the record last, each
// verified by reading it back. A record already in the exchange at its size
// was published whole, since it is only ever written after its blobs.
func publishCheckpoints(history *synclog.History, root string, report *FolderReport) error {
	own, err := OwnCheckpoints(history)
	if err != nil {
		return err
	}
	for _, c := range own {
		record := filepath.Join(root, filepath.FromSlash(CheckpointPath(c.CheckpointID)))
		local, statErr := os.Stat(history.CheckpointPath(c.CheckpointID))
		if statErr != nil {
			return statErr
		}
		if published, err := os.Stat(record); err == nil && published.Size() == local.Size() {
			continue
		}
		files, err := CheckpointFiles(history, c)
		if err != nil {
			return err
		}
		for _, file := range files {
			if err := publishFile(root, file); err != nil {
				report.Errors = append(report.Errors, err.Error())
				return nil
			}
		}
		report.Checkpoints = append(report.Checkpoints, c.CheckpointID)
	}
	return nil
}

// checkpointIfDue authors this device's checkpoint when its history has grown
// past its newest one (see synclog.History.CheckpointDue). A refusal — open
// conflicts, unpublished work, an unreadable change — is not a failed pass:
// it is reported and asked again on the next one.
func checkpointIfDue(committer FolderCommitter, history *synclog.History, report *FolderReport) error {
	author, ok := committer.(checkpointAuthor)
	if !ok {
		return nil
	}
	due, err := history.CheckpointDue(CheckpointInterval)
	if err != nil || !due {
		return err
	}
	if _, err := author.CommitSyncCheckpoint(); err != nil {
		if errors.Is(err, synclog.ErrCheckpointRefused) {
			report.CheckpointDeferred = err.Error()
			history.DeferCheckpoint()
			return nil
		}
		return fmt.Errorf("recording a sync checkpoint: %w", err)
	}
	return nil
}
