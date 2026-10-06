package syncexchange

import (
	"testing"

	"github.com/sglavoie/tt/internal/synclog"
)

func TestManifestPathsRoundTripAndRefuseOtherNames(t *testing.T) {
	txn := "sha256:" + "ab" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"
	path := ManifestPath("mac-1", 42, txn)
	if path != "txns/mac-1/0000000042-ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd.json" {
		t.Fatalf("path %s", path)
	}
	device, seq, id, ok := ParseManifestPath(path)
	if !ok || device != "mac-1" || seq != 42 || id != txn {
		t.Fatalf("parsed %s %d %s %v", device, seq, id, ok)
	}
	for _, other := range []string{
		"txns/mac-1/0000000000-ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd.json",
		"txns/mac-1/42-ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd.json",
		"txns/../0000000042-ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd.json",
		"blobs/sha256/ab/ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd",
	} {
		if _, _, _, ok := ParseManifestPath(other); ok {
			t.Fatalf("%s parsed as a manifest path", other)
		}
	}
	if got := BlobPath(txn); got != "blobs/sha256/ab/ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd" {
		t.Fatalf("blob path %s", got)
	}
}

func TestRequiredBlobsNamesEveryBaseAndNewDigestOnce(t *testing.T) {
	a, b, c := synclog.Digest([]byte("a")), synclog.Digest([]byte("b")), synclog.Digest([]byte("c"))
	m := &synclog.Manifest{Effects: []synclog.Effect{
		{Path: "projects/x/tasks.md", BaseBlob: &a, Blob: &b},
		{Path: "projects/x/.attachments/f", Blob: &c},
		{Path: "projects/x/notes/n.md", BaseBlob: &b},
	}}
	got := RequiredBlobs(m)
	if len(got) != 3 || got[0] > got[1] || got[1] > got[2] {
		t.Fatalf("required %v", got)
	}
}
