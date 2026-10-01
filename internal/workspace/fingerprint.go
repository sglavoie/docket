package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Fingerprint digests the stat identity — path, size, modification time — of
// the files a workspace reload reads: the `.tt/*.json` domains under dataDir
// and each project's tasks.md. It is the signal Loader already trusts, so a
// file Loader would not reparse is a file that changes no fingerprint. It reads
// no file content, and a missing file is part of the identity rather than an
// error. An empty dataDir has no fingerprint.
func Fingerprint(dataDir string, projects []*models.Project) string {
	if dataDir == "" {
		return ""
	}
	paths, _ := filepath.Glob(filepath.Join(dataDir, ".tt", "*.json"))
	for _, project := range projects {
		if project != nil && project.Path != "" {
			paths = append(paths, storage.TasksFilePath(project.Path))
		}
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			_, _ = fmt.Fprintf(h, "%s\x00missing\n", path)
			continue
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, info.Size(), info.ModTime().UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil))
}
