package syncexchange

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/sglavoie/docket/internal/synclog"
)

// countingHistory is a device's history as fetch reads it, counting the index
// lookups a pass makes.
type countingHistory struct {
	received map[string]uint64
	loads    int
	lookups  []string
}

func (h *countingHistory) DeviceID() string { return "desk" }

func (h *countingHistory) LoadState() (*synclog.State, error) {
	h.loads++
	if h.received == nil {
		return nil, nil
	}
	return &synclog.State{Received: h.received}, nil
}

func (h *countingHistory) Lookup(txnID string) (synclog.IndexEntry, bool, error) {
	h.lookups = append(h.lookups, txnID)
	return synclog.IndexEntry{}, false, nil
}

func (h *countingHistory) Blob(digest string) ([]byte, error) {
	return nil, errors.New("not retained: " + digest)
}

// publishNote writes the manifest of one note written by device at seq, and
// its blob, under an exchange root. It returns their paths under that root.
func publishNote(t *testing.T, root, device string, seq uint64) (manifestRel, blobRel string) {
	t.Helper()
	content := []byte(fmt.Sprintf("note %s %d", device, seq))
	blob := synclog.Digest(content)
	manifest := &synclog.Manifest{
		Format:        synclog.ManifestFormat,
		FormatVersion: synclog.ManifestFormatVersion,
		WorkspaceID:   "ws",
		DeviceID:      device,
		DeviceSeq:     seq,
		Parents:       []string{},
		Kind:          synclog.KindUser,
		CreatedAt:     "2026-10-12T12:00:00Z",
		Schemas:       map[string]int{synclog.SchemaCatalog: 1, synclog.SchemaDomainEnvelope: 1, synclog.SchemaTaskMarkdown: 1},
		Requires:      []string{},
		Effects: []synclog.Effect{{
			Kind: synclog.EffectGlobalNote, Path: fmt.Sprintf(".docket/global-notes/%s-%d.md", device, seq),
			Op: synclog.OpPut, Blob: &blob,
		}},
	}
	txnID, err := manifest.ComputeTxnID()
	if err != nil {
		t.Fatal(err)
	}
	manifest.TxnID = txnID
	data, err := manifest.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, decodeErr := synclog.DecodeManifest(data); decodeErr != nil {
		t.Fatalf("fixture manifest does not decode: %v", decodeErr)
	}
	manifestRel, blobRel = ManifestPath(device, seq, txnID), BlobPath(blob)
	for rel, bytes := range map[string][]byte{manifestRel: data, blobRel: content} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, bytes, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return manifestRel, blobRel
}

func TestFetchReadsNothingItAlreadyApplied(t *testing.T) {
	root, inbox := t.TempDir(), t.TempDir()
	const applied = 40
	for seq := uint64(1); seq <= applied; seq++ {
		publishNote(t, root, "phone", seq)
		publishNote(t, root, "desk", seq) // This device's own outgoing history.
	}
	var reads []string
	previous := readFolderFile
	readFolderFile = func(abs string) ([]byte, error) {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			t.Fatal(err)
		}
		reads = append(reads, filepath.ToSlash(rel))
		return previous(abs)
	}
	t.Cleanup(func() { readFolderFile = previous })

	history := &countingHistory{received: map[string]uint64{"phone": applied}}
	if err := fetch(history, root, inbox, &FolderReport{}); err != nil {
		t.Fatal(err)
	}
	if history.loads != 1 || len(history.lookups) != 0 || len(reads) != 0 {
		t.Fatalf("an idle fetch loaded state %d times, looked up %d transactions and read %v",
			history.loads, len(history.lookups), reads)
	}

	manifestRel, blobRel := publishNote(t, root, "phone", applied+1)
	history.loads = 0
	if err := fetch(history, root, inbox, &FolderReport{}); err != nil {
		t.Fatal(err)
	}
	want := []string{blobRel, manifestRel}
	sort.Strings(want)
	sort.Strings(reads)
	if history.loads != 1 || len(history.lookups) != 1 || !reflect.DeepEqual(reads, want) {
		t.Fatalf("fetching one new manifest loaded state %d times, looked up %d transactions and read %v, want %v",
			history.loads, len(history.lookups), reads, want)
	}
	for _, rel := range want {
		if _, err := os.Stat(filepath.Join(inbox, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s was not staged: %v", rel, err)
		}
	}
}

func TestFetchStagesEverythingForADeviceWithNoState(t *testing.T) {
	root, inbox := t.TempDir(), t.TempDir()
	manifestRel, _ := publishNote(t, root, "phone", 1)
	history := &countingHistory{}
	if err := fetch(history, root, inbox, &FolderReport{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(inbox, filepath.FromSlash(manifestRel))); err != nil {
		t.Fatalf("a device with no state staged nothing: %v", err)
	}
}
