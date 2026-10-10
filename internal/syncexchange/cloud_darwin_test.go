package syncexchange

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloudDownloadRequestAcceptsLiteralLocalPaths(t *testing.T) {
	// Exercise the actual Foundation bridge with non-cloud files. It must
	// ignore them, and paths containing script metacharacters stay data.
	path := filepath.Join(t.TempDir(), "space ' quote \" $(not-a-command).txt")
	if err := os.WriteFile(path, []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requestCloudDownloads([]string{path, path + "-missing"}); err != nil {
		t.Fatal(err)
	}
	if cloudDataless(path) || cloudDataless(path+"-missing") {
		t.Fatal("ordinary local files were classified as cloud placeholders")
	}
}
