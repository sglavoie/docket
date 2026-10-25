package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/docket/internal/util"
)

// Names the product used before it became Docket. They are inputs only: the
// adoption below reads them once and nothing writes them again.
const (
	// legacyLocalRootName is the machine-local root before ~/.docket.
	legacyLocalRootName = ".tt"

	// legacyICloudDataDirName is the iCloud Drive workspace folder before
	// iCloud Drive/Docket.
	legacyICloudDataDirName = "tt"
)

// AdoptLegacyRoots moves the machine-local root and the iCloud Drive workspace
// from their tt-era names to the Docket ones. It must run before the
// active-workspace pointer is read. See adoptLegacyRootsOn.
func AdoptLegacyRoots() error {
	return adoptLegacyRootsOn("")
}

// adoptLegacyRootsOn is AdoptLegacyRoots resolved against an explicit machine
// home.
//
// It does nothing once ~/.docket exists, so it runs at most once per home. Until
// then it renames ~/.tt to ~/.docket in one step: runtime state, the pointer,
// logs, and the default local workspace when ~/.tt is one all move together.
// An iCloud Drive/tt workspace moves to iCloud Drive/Docket in the same pass,
// and the pointer is rewritten when it named either old location. Any other
// pointer target is left alone.
//
// Nothing moves when iCloud Drive/Docket already exists beside a tt workspace.
// A failure part way puts back what had already moved, so the old names stay
// intact and the next start tries again.
func adoptLegacyRootsOn(machineHome string) error {
	home, ok := resolveMachineHome(machineHome)
	if !ok {
		return nil
	}
	newLocal := filepath.Join(home, localRootName)
	if present, err := pathPresent(newLocal); err != nil || present {
		return err
	}
	oldLocal := filepath.Join(home, legacyLocalRootName)
	localPresent, err := pathPresent(oldLocal)
	if err != nil {
		return err
	}

	oldCloud, newCloud, cloudMove, err := legacyICloudMove(home)
	if err != nil {
		return err
	}
	if !localPresent && !cloudMove {
		return nil
	}

	if cloudMove {
		if err := os.Rename(oldCloud, newCloud); err != nil {
			return fmt.Errorf("could not move %s to %s; both were left untouched: %w", DisplayPath(oldCloud), DisplayPath(newCloud), err)
		}
	}
	undoCloud := func() {
		if cloudMove {
			_ = os.Rename(newCloud, oldCloud)
		}
	}
	if !localPresent {
		return nil
	}
	if err := os.Rename(oldLocal, newLocal); err != nil {
		undoCloud()
		return fmt.Errorf("could not move %s to %s; %s was left untouched: %w", DisplayPath(oldLocal), DisplayPath(newLocal), DisplayPath(oldLocal), err)
	}

	moves := map[string]string{oldLocal: newLocal}
	if oldCloud != "" && !pathExists(oldCloud) && pathExists(newCloud) {
		// Also covers a workspace another machine already moved through iCloud.
		moves[oldCloud] = newCloud
	}
	if err := rewriteAdoptedPointer(filepath.Join(newLocal, runtimeDirName, activeWorkspacePointerName), moves); err != nil {
		_ = os.Rename(newLocal, oldLocal)
		undoCloud()
		return fmt.Errorf("could not rewrite the active-workspace pointer, so %s was left untouched: %w", DisplayPath(oldLocal), err)
	}
	return nil
}

// legacyICloudMove reports the old and new iCloud Drive workspace paths and
// whether the old one is a workspace that should move. The paths are empty when
// iCloud Drive is unavailable. A tt workspace beside an existing Docket folder
// is refused by name.
func legacyICloudMove(home string) (oldCloud, newCloud string, move bool, err error) {
	drive := filepath.Join(home, iCloudDriveRelPath)
	if !isDirectory(drive) {
		return "", "", false, nil
	}
	oldCloud = filepath.Join(drive, legacyICloudDataDirName)
	newCloud = filepath.Join(drive, iCloudDataDirName)
	if !isLegacyWorkspace(oldCloud) {
		return oldCloud, newCloud, false, nil
	}
	present, err := pathPresent(newCloud)
	if err != nil {
		return "", "", false, err
	}
	if present {
		return "", "", false, fmt.Errorf("cannot move the iCloud Drive workspace %s to %s: %s already exists; both were left untouched, so move or merge one of them and start Docket again", DisplayPath(oldCloud), DisplayPath(newCloud), DisplayPath(newCloud))
	}
	return oldCloud, newCloud, true, nil
}

// isLegacyWorkspace reports whether dir holds a workspace by the same test the
// default-workspace resolution applies, so an unrelated folder named tt stays.
func isLegacyWorkspace(dir string) bool {
	if hasSplitDomainMarker(dir) {
		return true
	}
	_, err := os.Stat(filepath.Join(dir, configFileName))
	return err == nil
}

// rewriteAdoptedPointer points the active-workspace pointer at the new location
// when its data dir is, or lies beneath, one of the moved roots. A missing or
// malformed pointer is left as it is: reading it already falls through.
func rewriteAdoptedPointer(path string, moves map[string]string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	fields, dataDir, ok := decodePointerFields(data)
	if !ok {
		return nil
	}
	rewritten, ok := movedPath(filepath.Clean(dataDir), moves)
	if !ok {
		return nil
	}
	encoded, err := json.Marshal(rewritten)
	if err != nil {
		return err
	}
	fields["data_dir"] = encoded
	out, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	return util.AtomicWriteFile(path, out, FilePermissions)
}

// decodePointerFields decodes a pointer into its raw fields, so a rewrite keeps
// every other field byte for byte, and reports its data dir. A malformed
// pointer or one without a data dir reports false.
func decodePointerFields(data []byte) (map[string]json.RawMessage, string, bool) {
	var fields map[string]json.RawMessage
	var dataDir string
	if json.Unmarshal(data, &fields) == nil && json.Unmarshal(fields["data_dir"], &dataDir) == nil && dataDir != "" {
		return fields, dataDir, true
	}
	return nil, "", false
}

// isDirectory reports whether path is a directory, following symlinks.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// movedPath maps path through the first move whose source is path or one of
// its ancestors.
func movedPath(path string, moves map[string]string) (string, bool) {
	for from, to := range moves {
		if path == from {
			return to, true
		}
		if rest, ok := strings.CutPrefix(path, from+string(os.PathSeparator)); ok {
			return filepath.Join(to, rest), true
		}
	}
	return "", false
}

// pathPresent reports whether anything, a dangling symlink included, sits at
// path. Only absence counts as absent: any other error is returned so the
// adoption never mistakes an unreadable root for a missing one.
func pathPresent(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("could not inspect %s: %w", DisplayPath(path), err)
}

// pathExists is pathPresent with errors read as absent.
func pathExists(path string) bool {
	present, err := pathPresent(path)
	return err == nil && present
}
