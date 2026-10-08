package synclog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WorkspaceDigest digests every synced regular file under dataDir, path and
// content, so two devices holding the same workspace report the same value and
// any lost, extra, or differing synced file changes it. Device-local paths, as
// Classify names them, and paths it cannot classify are left out. The value is
// "sha256:" over sorted "path\tsha256:hex\n" lines, the same construction the
// Apple contract's seed digest uses.
func WorkspaceDigest(dataDir string) (string, error) {
	var lines []string
	err := filepath.WalkDir(dataDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(dataDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if !synced(rel) {
			return nil
		}
		digest, digestErr := fileDigest(path)
		if digestErr != nil {
			return digestErr
		}
		lines = append(lines, rel+"\t"+digest+"\n")
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("digest workspace: %w", err)
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "")))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func fileDigest(path string) (digest string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
