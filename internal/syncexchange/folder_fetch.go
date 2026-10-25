package syncexchange

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/util"
)

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
