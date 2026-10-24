package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Stamps is the stat identity of every file a workspace reload reads, by
// path: the `.tt/*.json` domains under the data directory and each project's
// tasks.md, plus the sync tracking state. The latter catches imports of notes
// and attachments already applied by another process. It uses the same stat
// identity as Loader, reads no file content, and records a missing file as part
// of the identity rather than as an error.
type Stamps map[string]storage.FileStamp

// StampWorkspace stamps the files a reload of dataDir and projects reads. An
// empty dataDir has no stamps.
func StampWorkspace(dataDir string, projects []*models.Project) Stamps {
	if dataDir == "" {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(dataDir, ".tt", "*.json"))
	syncStates, _ := filepath.Glob(filepath.Join(dataDir, ".tt", "sync", "devices", "*", "state.json"))
	paths = append(paths, syncStates...)
	for _, project := range projects {
		if project != nil && project.Path != "" {
			paths = append(paths, storage.TasksFilePath(project.Path))
		}
	}
	stamps := make(Stamps, len(paths))
	for _, path := range paths {
		stamps[filepath.Clean(path)] = storage.StampFile(path)
	}
	return stamps
}

// Fingerprint digests the stamps into one comparable value. No stamps have
// no fingerprint.
func (s Stamps) Fingerprint() string {
	if s == nil {
		return ""
	}
	paths := make([]string, 0, len(s))
	for path := range s {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		stamp := s[path]
		if stamp.Missing {
			_, _ = fmt.Fprintf(h, "%s\x00missing\n", path)
			continue
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, stamp.Size, stamp.ModTime)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ChangedElsewhere reports whether current differs from s at a path no commit
// recorded in own accounts for. A path that differs but still carries the stamp
// the latest recorded commit there left behind changed by that commit; one a
// listing no longer names is read as absent. Comparing against no stamps is
// always a change, and a nil own accounts for nothing.
func (s Stamps) ChangedElsewhere(current Stamps, own *storage.OwnWrites) bool {
	if s == nil || current == nil {
		return s != nil || current != nil
	}
	stampAt := func(stamps Stamps, path string) storage.FileStamp {
		if stamp, ok := stamps[path]; ok {
			return stamp
		}
		return storage.FileStamp{Missing: true}
	}
	changed := func(path string) bool {
		now := stampAt(current, path)
		if stampAt(s, path) == now {
			return false
		}
		recorded, ok := own.Stamp(path)
		return !ok || recorded != now
	}
	for path := range current {
		if changed(path) {
			return true
		}
	}
	for path := range s {
		if _, listed := current[path]; !listed && changed(path) {
			return true
		}
	}
	return false
}
