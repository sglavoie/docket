package synclog

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// The stat cache is what lets an observation pass over a workspace that did
// not change without reading it. It remembers, for each synced file, the size
// and modification time it was last digested at; a file still showing both is
// not read again. It is device-local, derived entirely from the files it
// names, and written on its own rather than through the journal: it accounts
// for no transaction, so losing or deleting it costs one full walk and changes
// no result.

// Stat cache format identifiers.
const (
	StatCacheFormat        = "tt-sync-stat-cache"
	StatCacheFormatVersion = 1
)

// statCacheRacyWindow widens git's racy rule. A file modified in the same
// timestamp tick it was digested in keeps its size and mtime, so an entry is
// trusted only when its mtime is older than the walk that recorded it by more
// than the coarsest timestamp a filesystem keeps.
const statCacheRacyWindow = 2 * time.Second

// readSyncedFile reads one synced file for observation.
var readSyncedFile = os.ReadFile

type statCache struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	// WalkedAt is when the walk that wrote the cache began, in Unix
	// nanoseconds: every entry was digested at or after it.
	WalkedAt int64 `json:"walked_at"`
	// Files maps each synced path to the identity it was digested at.
	Files map[string]statEntry `json:"files"`
}

type statEntry struct {
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	Digest  string `json:"digest"`
}

// StatCachePath is the device-local observation stat cache.
func (h *History) StatCachePath() string { return filepath.Join(h.Dir(), "stat-cache.json") }

// loadStatCache reads the stat cache. A missing, unreadable, foreign, or
// future-dated cache is nil, which is a full walk.
func (h *History) loadStatCache(now time.Time) *statCache {
	data, err := os.ReadFile(h.StatCachePath())
	if err != nil {
		return nil
	}
	var cache statCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	if cache.Format != StatCacheFormat || cache.FormatVersion != StatCacheFormatVersion {
		return nil
	}
	if cache.WalkedAt > now.UnixNano() {
		return nil
	}
	return &cache
}

// digest returns the digest rel was last read at, when info still shows the
// identity it was read under and that identity is safely older than the walk.
func (c *statCache) digest(rel string, info fs.FileInfo) (string, bool) {
	if c == nil {
		return "", false
	}
	entry, ok := c.Files[rel]
	if !ok || entry.Size != info.Size() || entry.ModTime != info.ModTime().UnixNano() {
		return "", false
	}
	if entry.ModTime+int64(statCacheRacyWindow) >= c.WalkedAt || !ValidDigest(entry.Digest) {
		return "", false
	}
	return entry.Digest, true
}

// saveStatCache replaces the stat cache with the walk begun at walkedAt. It is
// best effort and not made durable: a cache that was not written, or that a
// crash tore, is a full walk next time.
func (h *History) saveStatCache(walkedAt time.Time, files map[string]statEntry) {
	data, err := encodeJSON(statCache{
		Format:        StatCacheFormat,
		FormatVersion: StatCacheFormatVersion,
		WalkedAt:      walkedAt.UnixNano(),
		Files:         files,
	})
	if err != nil {
		return
	}
	path := h.StatCachePath()
	staged, err := os.CreateTemp(filepath.Dir(path), ".tt-stage-stat-cache-*")
	if err != nil {
		return
	}
	_, writeErr := staged.Write(data)
	closeErr := staged.Close()
	if writeErr == nil && closeErr == nil && os.Rename(staged.Name(), path) == nil {
		return
	}
	_ = os.Remove(staged.Name())
}

// learned reports whether the walk begun at walkedAt knows anything the cache
// it started from does not: a file added, removed, or read at a new identity,
// or one read again only because it was racy that this walk is late enough to
// vouch for. A walk that learned nothing writes nothing.
func (c *statCache) learned(walkedAt time.Time, files map[string]statEntry, read map[string][]byte) bool {
	if c == nil || len(files) != len(c.Files) {
		return true
	}
	for rel := range read {
		entry := files[rel]
		if c.Files[rel] != entry || entry.ModTime+int64(statCacheRacyWindow) < walkedAt.UnixNano() {
			return true
		}
	}
	return false
}

// syncedDigests is one walk of the synced workspace: every file's digest, and
// the bytes of the ones the walk had to read.
type syncedDigests struct {
	digests  map[string]string
	contents map[string][]byte
}

// digestSynced digests every synced file, reading only the ones the stat
// cache cannot vouch for, and rewrites the cache when the walk learned
// anything.
func (h *History) digestSynced() (*syncedDigests, error) {
	walkedAt := time.Now()
	cache := h.loadStatCache(walkedAt)
	walk := &syncedDigests{digests: map[string]string{}, contents: map[string][]byte{}}
	files := map[string]statEntry{}
	err := h.walkSyncedFiles(func(rel, abs string, info fs.FileInfo) error {
		digest, ok := cache.digest(rel, info)
		if !ok {
			data, err := h.readSynced(rel, abs)
			if err != nil {
				return err
			}
			digest = Digest(data)
			walk.contents[rel] = data
		}
		walk.digests[rel] = digest
		files[rel] = statEntry{Size: info.Size(), ModTime: info.ModTime().UnixNano(), Digest: digest}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if cache.learned(walkedAt, files, walk.contents) {
		h.saveStatCache(walkedAt, files)
	}
	return walk, nil
}

// bytes returns the content of rel, reading it when the walk reused its
// digest. The digest is recomputed from what is read, so a cache entry is
// never what a change is judged by. A file gone since the walk reports false.
func (w *syncedDigests) bytes(h *History, rel string) ([]byte, bool, error) {
	if data, read := w.contents[rel]; read {
		return data, true, nil
	}
	data, err := h.readSynced(rel, h.abs(rel))
	if isNotExist(err) {
		delete(w.digests, rel)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	w.digests[rel] = Digest(data)
	w.contents[rel] = data
	return data, true, nil
}
