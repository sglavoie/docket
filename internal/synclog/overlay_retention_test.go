package synclog

import "testing"

func TestOverlayRetainsSharedContentOnlyOnce(t *testing.T) {
	h := bootstrapTestHistory(t)
	o := newOverlay(h)
	data := []byte("shared historical and current content")
	digest := Digest(data)
	o.retainBytes(digest, data)
	o.retain(&vfile{exists: true, digest: digest, data: data})
	o.retainBytes(digest, data)
	if len(o.blobOps) != 1 || o.blobOps[0].Target != h.BlobPath(digest) {
		t.Fatalf("one digest must have one retained write: %+v", o.blobOps)
	}
}
