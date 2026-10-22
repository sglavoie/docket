package util

import (
	"os"
	"syscall"
)

// Darwin's fsync drains a file to the device; F_FULLFSYNC then drains the
// device's entire write cache. Grouping the latter is safe only AFTER every
// file and directory in the batch has completed fsync. Go's File.Sync issues
// F_FULLFSYNC, so it remains the final barrier, once per distinct filesystem.
// https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fsync.2.html
func syncBatchFile(f *os.File) error {
	if fsyncSkipped() {
		return nil
	}
	for {
		err := syscall.Fsync(int(f.Fd()))
		if err != syscall.EINTR {
			return err
		}
	}
}

func syncBatchDirectories(dirs []string) error {
	if fsyncSkipped() {
		return nil
	}
	return syncBatchDirectoriesWith(dirs, syncBatchFile, SyncFile)
}

func syncBatchDirectoriesWith(dirs []string, syncDirectory, flushDevice func(*os.File) error) error {
	// Retain a descriptor on each filesystem until its final full flush.
	// A batch spanning mounted filesystems must flush each, not just the
	// filesystem containing the transaction manifest.
	volumes := map[int32]*os.File{}
	defer func() {
		for _, f := range volumes {
			_ = f.Close()
		}
	}()
	for _, dir := range dirs {
		f, err := os.Open(dir)
		if err != nil {
			return err
		}
		if err = syncDirectory(f); err != nil {
			_ = f.Close()
			return err
		}
		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return err
		}
		device := info.Sys().(*syscall.Stat_t).Dev
		if volumes[device] == nil {
			volumes[device] = f
		} else {
			_ = f.Close()
		}
	}
	for _, f := range volumes {
		if err := flushDevice(f); err != nil {
			return err
		}
	}
	return nil
}
