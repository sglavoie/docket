package storage

import (
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/sglavoie/tt/internal/transaction"
)

// FileStamp is the stat identity a reload signal compares: size and
// modification time, or the file's absence. It reads no content.
type FileStamp struct {
	Size    int64
	ModTime int64
	Missing bool
}

// StampFile stamps path as it stands now. Any stat failure is absence, as the
// workspace fingerprint has always read it.
func StampFile(path string) FileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return FileStamp{Missing: true}
	}
	return FileStamp{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
}

// ownWriteLimit bounds the registry. Content-addressed sync and undo files
// make the set of committed paths grow with every commit; forgetting them all
// at once only costs the next poll a reload it would have taken anyway.
const ownWriteLimit = 4096

// OwnWrites remembers, for every path the commit services sharing it wrote,
// moved, or deleted, the stamp that commit left behind. A frontend polling the
// workspace for changes made elsewhere reads it to recognize its own commits,
// and those of a sync worker it shares the registry with, without rereading the
// workspace. It is safe for concurrent use; a nil registry records nothing.
type OwnWrites struct {
	mu     sync.Mutex
	stamps map[string]FileStamp
}

// NewOwnWrites returns an empty registry.
func NewOwnWrites() *OwnWrites { return &OwnWrites{stamps: map[string]FileStamp{}} }

// record stamps every path a committed manifest touched. It runs after the
// commit returns, so a write landing between the journal's rename and this stat
// is attributed to the commit; another tt process cannot land one there, since
// it waits on the same workspace commit lock.
func (w *OwnWrites) record(manifest *transaction.Manifest) {
	if w == nil || manifest == nil {
		return
	}
	stamps := make(map[string]FileStamp, len(manifest.Ops))
	for _, op := range manifest.Ops {
		target := filepath.Clean(op.Target)
		stamps[target] = StampFile(target)
		if op.Kind == transaction.OpMove && op.Source != "" {
			source := filepath.Clean(op.Source)
			stamps[source] = StampFile(source)
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.stamps)+len(stamps) > ownWriteLimit {
		clear(w.stamps)
	}
	maps.Copy(w.stamps, stamps)
}

// Stamp reports the stamp the latest recorded commit left at path, and whether
// one did. A path whose current stamp equals it has not changed since.
func (w *OwnWrites) Stamp(path string) (FileStamp, bool) {
	if w == nil {
		return FileStamp{}, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	stamp, ok := w.stamps[filepath.Clean(path)]
	return stamp, ok
}

// WithOwnWrites records every transaction this service commits into w, and
// returns the same service. The attached ledger's undo and redo record there
// too.
func (s *CommitService) WithOwnWrites(w *OwnWrites) *CommitService {
	s.ownWrites = w
	if s.ledger != nil {
		s.ledger.ownWrites = w
	}
	return s
}

// OwnWrites is the registry this service records into, or nil.
func (s *CommitService) OwnWrites() *OwnWrites { return s.ownWrites }

// commitJournal commits a prepared manifest through the package seam and
// records what it wrote.
func (s *CommitService) commitJournal(journal *transaction.Journal, manifest *transaction.Manifest) error {
	if err := commitJournal(journal, manifest); err != nil {
		return err
	}
	s.ownWrites.record(manifest)
	return nil
}
