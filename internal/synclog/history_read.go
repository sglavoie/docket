package synclog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RecordedDeviceID is the sync identity a working copy already records as, or
// "" when it has recorded nothing. A working copy belongs to one device, so the
// identity it holds is that device's whatever the machine is called today: an
// identity derived afresh from a renamed host would start a second history
// beside the first, with its own genesis and nothing received.
//
// A working copy that already holds several — the product of exactly that
// rename — answers with the one that has recorded the most, which only widens
// its lead once chosen, so the answer holds still without being stored.
func RecordedDeviceID(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	devices := filepath.Join(dataDir, internalDir, "sync", "devices")
	entries, err := os.ReadDir(devices)
	if err != nil {
		return ""
	}
	best, bestSeq := "", uint64(0)
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || !ValidDeviceID(id) {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(devices, id, "state.json"))
		if readErr != nil {
			continue
		}
		var state State
		if json.Unmarshal(data, &state) != nil || state.Format != StateFormat || state.DeviceID != id {
			continue
		}
		// ReadDir is sorted, so a tie keeps the first name.
		if best == "" || state.NextSeq > bestSeq {
			best, bestSeq = id, state.NextSeq
		}
	}
	return best
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
	// A state written before schema 24 tracks .tt/ paths; the files it tracked
	// are the same ones at their .docket/ paths now.
	for rel, digest := range state.Tracked {
		if canonical := CanonicalPath(rel); canonical != rel {
			delete(state.Tracked, rel)
			state.Tracked[canonical] = digest
		}
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

// Pending returns outgoing transactions awaiting publication, with parents
// before descendants, each verified against its content address. A resolution
// may also queue retained local ancestors authored by an older device identity.
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

// Genesis is the enrollment genesis this device recorded, or nil when its
// first transaction is not one (or it has recorded none).
func (h *History) Genesis() (*Manifest, error) {
	matches, err := filepath.Glob(filepath.Join(h.deviceDir(), "txns", fmt.Sprintf("%010d-*.json", 1)))
	if err != nil || len(matches) == 0 {
		return nil, err
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("reading the first outgoing record: %w", err)
	}
	m, err := DecodeManifest(data)
	if err != nil {
		return nil, err
	}
	if m.Kind != KindEnroll {
		return nil, nil
	}
	return m, nil
}
