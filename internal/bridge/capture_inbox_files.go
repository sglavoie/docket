package bridge

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
)

// stagedCaptureFile is one verified file of a capture.
type stagedCaptureFile struct {
	path, name, digest string
}

// captureFiles resolves each shared file inside its own entry and checks its
// bytes against the envelope, so a damaged copy imports nothing.
func captureFiles(dir string, files []captureFile) ([]stagedCaptureFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	out := make([]stagedCaptureFile, 0, len(files))
	for _, file := range files {
		if file.Path == "" || filepath.IsAbs(file.Path) {
			return nil, newError(CodeValidation, "the shared file %q is not inside its capture", file.Path).with("field", "files")
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.Clean(file.Path)))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, newError(CodeValidation, "the shared file %s is missing", file.Filename).with("field", "files")
		}
		if err != nil {
			return nil, err
		}
		if relative, relErr := filepath.Rel(root, resolved); relErr != nil || relative == "." || strings.HasPrefix(relative, "..") {
			return nil, newError(CodeValidation, "the shared file %q is not inside its capture", file.Path).with("field", "files")
		}
		digest, err := fileDigest(resolved)
		if err != nil {
			return nil, err
		}
		if digest != file.Digest {
			return nil, newError(CodeValidation, "the shared file %s is damaged: its digest is %s, not %s", file.Filename, digest, file.Digest).
				with("field", "files")
		}
		out = append(out, stagedCaptureFile{path: resolved, name: attachmentFilename(file.Filename, resolved), digest: digest})
	}
	return out, nil
}

// attachCapturedFile attaches one shared file unless an earlier attempt
// already did, answering the transaction it committed or "" for none.
func (b *Bridge) attachCapturedFile(target app.TaskLocator, file stagedCaptureFile) (string, error) {
	ref, err := b.lookupTask(target)
	if err != nil {
		return "", err
	}
	if _, present := attachedBytes(ref.Project, ref.Task, file.name, file.digest); present {
		return "", nil
	}
	name := freeAttachmentName(ref.Project, ref.Task, file.name)
	mutation, err := b.commitTask(func(service *app.Service) (app.TaskResult, error) {
		result, importErr := service.ImportAttachment(target, file.path, name)
		return result.Task, importErr
	})
	if err != nil {
		return "", err
	}
	return mutation.Committed.TransactionID, nil
}

func (b *Bridge) captureReceiptPath(id string) string {
	return filepath.Join(b.workspace.runtimeDir, captureReceiptsDir, id+".json")
}

func (b *Bridge) readCaptureReceipt(id string) (captureReceipt, bool) {
	data, err := os.ReadFile(b.captureReceiptPath(id))
	if err != nil {
		return captureReceipt{}, false
	}
	var receipt captureReceipt
	if json.Unmarshal(data, &receipt) != nil || receipt.ID != id {
		return captureReceipt{}, false
	}
	return receipt, true
}

func (b *Bridge) writeCaptureReceipt(receipt captureReceipt) error {
	path := b.captureReceiptPath(receipt.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(path, append(data, '\n'))
}

// writeFileAtomically replaces path with data through a synced sibling, so a
// termination leaves the old file or the new one and never a torn one.
func writeFileAtomically(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// removeCaptureEntry deletes an imported capture. It is first renamed out of
// the inbox in one step, so a termination never leaves a half-deleted entry
// that reads as a capture still being written; sweepCaptureStaging finishes
// what it left behind. Its receipt already names it either way.
func removeCaptureEntry(dir string) error {
	removed := filepath.Join(filepath.Dir(dir), captureStagingDir, capturedRemovalPrefix+filepath.Base(dir))
	if err := os.MkdirAll(filepath.Dir(removed), 0o755); err != nil {
		return err
	}
	_ = os.RemoveAll(removed)
	if err := os.Rename(dir, removed); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.RemoveAll(removed)
}

// capturedRemovalPrefix marks an imported entry on its way out of the inbox.
const capturedRemovalPrefix = "imported-"

// abandonedStagingAge is how long a writer's unfinished entry is kept before
// the importer sweeps it: far longer than any extension lives.
const abandonedStagingAge = 24 * time.Hour

// sweepCaptureStaging removes imported entries a termination left on their
// way out, and writes no extension can still be finishing.
func (b *Bridge) sweepCaptureStaging(capturesDir string) {
	staging := filepath.Join(capturesDir, captureStagingDir)
	entries, err := os.ReadDir(staging)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(staging, entry.Name())
		if strings.HasPrefix(entry.Name(), capturedRemovalPrefix) {
			_ = os.RemoveAll(path)
			continue
		}
		if info, err := entry.Info(); err == nil && b.now().Sub(info.ModTime()) > abandonedStagingAge {
			_ = os.RemoveAll(path)
		}
	}
}
