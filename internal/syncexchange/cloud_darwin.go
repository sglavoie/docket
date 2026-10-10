package syncexchange

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// SF_DATALESS is defined in Darwin's sys/stat.h. Reading one of these files
// can block on cloud delivery; ask Foundation to download it and retry later.
func cloudDataless(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Flags&0x40000000 != 0
}

// Use the system JavaScript/Objective-C bridge so standalone and cross-compiled
// Go binaries can call Foundation without cgo or an installed Swift helper.
// Paths are arguments, never executable source. This requests delivery only;
// it neither waits for downloads nor modifies the exchange's contents.
const cloudDownloadScript = `ObjC.import("Foundation");
function run(paths) {
    var manager = $.NSFileManager.defaultManager;
    var failed = [];
    paths.forEach(function(path) {
        var url = $.NSURL.fileURLWithPath(path);
        if (manager.isUbiquitousItemAtURL(url) &&
            !manager.startDownloadingUbiquitousItemAtURLError(url, null)) {
            failed.push(path);
        }
    });
    return JSON.stringify(failed);
}`

func requestCloudDownloads(paths []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := append([]string{"-l", "JavaScript", "-e", cloudDownloadScript, "--"}, paths...)
	output, err := exec.CommandContext(ctx, "/usr/bin/osascript", args...).Output()
	if err != nil {
		return fmt.Errorf("requesting iCloud downloads: %w", err)
	}
	var failed []string
	if err := json.Unmarshal(output, &failed); err != nil {
		return fmt.Errorf("reading iCloud download response: %w", err)
	}
	if len(failed) > 0 {
		return fmt.Errorf("iCloud refused %d download requests; first: %s", len(failed), failed[0])
	}
	return nil
}
