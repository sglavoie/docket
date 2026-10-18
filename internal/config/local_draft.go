package config

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/util"
)

// LocalDraft is an opaque, machine-local recovery document. Compare-and-swap
// under a file lock prevents two running terminals from overwriting each other.
// Views own its format; it is never a workspace artifact or a sync input.
type LocalDraft struct {
	path     string
	observed []byte
}

func (rs *RuntimeState) CaptureDraft(key []byte) (*LocalDraft, error) {
	if rs == nil || rs.DataDir() == "" {
		return nil, errors.New("workspace is unavailable")
	}
	path, err := machineStatePathOn(rs.coord.store.MachineHome(), rs.Catalog().WorkspaceID)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(key)
	return &LocalDraft{path: filepath.Join(filepath.Dir(path), "capture-drafts", fmt.Sprintf("%x.json", sum))}, nil
}

func (d *LocalDraft) Read() ([]byte, error) {
	data, err := os.ReadFile(d.path)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	d.observed = bytes.Clone(data)
	return data, nil
}

// Replace deletes on nil; an unchanged document needs no write. The caller
// serializes access to one LocalDraft while the lock covers other processes.
func (d *LocalDraft) Replace(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(d.path), 0700); err != nil {
		return err
	}
	return util.WithFileLock(d.path+".lock", true, func() error {
		current, err := os.ReadFile(d.path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if bytes.Equal(data, current) {
			// A previous atomic replacement may have reached its rename but
			// failed directory sync. Retrying the same bytes must be possible.
			if err = util.SyncDir(filepath.Dir(d.path)); err != nil {
				return err
			}
			d.observed = bytes.Clone(data)
			return nil
		}
		if !bytes.Equal(current, d.observed) {
			return errors.New("another terminal changed this capture draft; keep this editor open")
		}
		if data == nil {
			err = os.Remove(d.path)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			if err == nil {
				err = util.SyncDir(filepath.Dir(d.path))
			}
		} else {
			err = util.AtomicWriteFile(d.path, data, 0600)
		}
		if err == nil {
			d.observed = bytes.Clone(data)
		}
		return err
	})
}
