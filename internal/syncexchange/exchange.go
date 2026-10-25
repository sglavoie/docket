// Package syncexchange is the engine's half of the exchange-folder contract
// (docs/mobile/sync-envelope.md). It names where each manifest and blob lives
// in the exchange, lists what a device still has to publish, and imports the
// manifests a transport delivered into a local inbox. It never touches the
// exchange folder itself: moving files to and from it — coordinated access,
// downloads, retries, and the durable transfer queues — is the transport's
// (apple/TTSync on Apple platforms), so the engine judges only bytes that are
// already local and verified.
package syncexchange

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/synclog"
)

var manifestPathPattern = regexp.MustCompile(`^txns/([A-Za-z0-9][A-Za-z0-9._-]{0,63})/([0-9]{10})-([0-9a-f]{64})\.json$`)

// ManifestPath is where the manifest of one transaction lives under the
// exchange's tt-sync/v1 root.
func ManifestPath(deviceID string, seq uint64, txnID string) string {
	return fmt.Sprintf("txns/%s/%010d-%s.json", deviceID, seq, strings.TrimPrefix(txnID, "sha256:"))
}

// BlobPath is where the blob with digest lives under the exchange root.
func BlobPath(digest string) string {
	hex := strings.TrimPrefix(digest, "sha256:")
	return path.Join("blobs", "sha256", hex[:2], hex)
}

// ParseManifestPath reads the author, sequence, and transaction id a manifest
// path promises. A path that is not a manifest path reports ok false.
func ParseManifestPath(rel string) (deviceID string, seq uint64, txnID string, ok bool) {
	match := manifestPathPattern.FindStringSubmatch(rel)
	if match == nil {
		return "", 0, "", false
	}
	seq, err := strconv.ParseUint(match[2], 10, 64)
	if err != nil || seq == 0 {
		return "", 0, "", false
	}
	return match[1], seq, "sha256:" + match[3], true
}

// File is one immutable file a device publishes: its place under the root, the
// local file holding its bytes, and their digest and size, which is what the
// transport verifies a copy against.
type File struct {
	Path   string `json:"path"`
	Source string `json:"source"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

// Outgoing is one transaction awaiting publication: the blobs it names, which
// a transport writes first, and its manifest, which it writes last so a peer
// never sees a manifest whose content is not already on its way.
type Outgoing struct {
	TxnID     string `json:"txn_id"`
	DeviceSeq uint64 `json:"device_seq"`
	Manifest  File   `json:"manifest"`
	Blobs     []File `json:"blobs"`
}

// Pending lists the device's outbox in dependency order, each transaction
// with the stored manifest record and every blob it names, all verified
// against their content addresses before they are offered.
func Pending(history *synclog.History) ([]Outgoing, error) {
	pending, err := history.Pending()
	if err != nil {
		return nil, err
	}
	out := make([]Outgoing, 0, len(pending))
	for _, manifest := range pending {
		entry, ok, err := history.Lookup(manifest.TxnID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("syncexchange: outbox names unknown transaction %s", manifest.TxnID)
		}
		source := history.WorkspacePath(entry.Record)
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("reading outgoing record: %w", err)
		}
		item := Outgoing{
			TxnID:     manifest.TxnID,
			DeviceSeq: manifest.DeviceSeq,
			Manifest: File{
				Path:   ManifestPath(manifest.DeviceID, manifest.DeviceSeq, manifest.TxnID),
				Source: source,
				Digest: synclog.Digest(data),
				Size:   int64(len(data)),
			},
			Blobs: []File{},
		}
		for _, digest := range RequiredBlobs(manifest) {
			blob, err := history.Blob(digest)
			if err != nil {
				return nil, fmt.Errorf("syncexchange: outgoing %s names blob %s: %w", manifest.TxnID, digest, err)
			}
			item.Blobs = append(item.Blobs, File{
				Path:   BlobPath(digest),
				Source: history.BlobPath(digest),
				Digest: digest,
				Size:   int64(len(blob)),
			})
		}
		out = append(out, item)
	}
	return out, nil
}

// RequiredBlobs is every content address a manifest names — the bytes each
// effect replaced and the bytes it wrote — sorted and deduplicated. A
// transaction is complete only when all of them are present and verified:
// the new bytes are what a fast-forward writes, and the replaced bytes are the
// merge base a reconciliation reads.
func RequiredBlobs(m *synclog.Manifest) []string {
	seen := make(map[string]bool)
	var out []string
	for _, effect := range m.Effects {
		for _, digest := range []*string{effect.BaseBlob, effect.Blob} {
			if digest != nil && !seen[*digest] {
				seen[*digest] = true
				out = append(out, *digest)
			}
		}
	}
	sort.Strings(out)
	return out
}

// inboxBlobs reads blobs a transport staged into an inbox, falling back to
// the device's own retained blobs. Every read is verified against its digest,
// so a truncated or damaged copy is never content.
type inboxBlobs struct {
	dir     string
	history *synclog.History
}

func (s inboxBlobs) Blob(digest string) ([]byte, error) {
	if !synclog.ValidDigest(digest) {
		return nil, fmt.Errorf("syncexchange: invalid digest %q", digest)
	}
	data, err := os.ReadFile(filepath.Join(s.dir, filepath.FromSlash(BlobPath(digest))))
	if err == nil && synclog.Digest(data) == digest {
		return data, nil
	}
	return s.history.Blob(digest)
}

// has reports that digest is available and intact.
func (s inboxBlobs) has(digest string) bool {
	_, err := s.Blob(digest)
	return err == nil
}
