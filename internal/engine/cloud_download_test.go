package engine

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/syncexchange"
)

func TestFolderExchangeRequestsCloudContentAndImportsOnRetry(t *testing.T) {
	for _, kind := range []string{"identity", "manifest", "blob"} {
		t.Run(kind, func(t *testing.T) {
			env := newEnrollEnvironment(t)
			env.enroll(t)
			desk := env.open(t, env.workingCopy, enrollingDevice)
			peer := env.join(t)
			captureIn(t, peer, "Awaiting cloud delivery")
			exchangeNow(t, peer)
			manifests := exchangeManifests(t, env.exchange, joiningDevice)
			manifest := manifests[len(manifests)-1]
			rel := "workspace.json"
			switch kind {
			case "manifest":
				rel = syncexchange.ManifestPath(manifest.DeviceID, manifest.DeviceSeq, manifest.TxnID)
			case "blob":
				for _, digest := range syncexchange.RequiredBlobs(manifest) {
					if _, err := desk.SyncCommits().SyncHistory().Blob(digest); err != nil {
						rel = syncexchange.BlobPath(digest)
						break
					}
				}
				if rel == "workspace.json" {
					t.Fatal("fixture has no incoming blob")
				}
			}
			path := filepath.Join(env.exchange, syncexchange.ExchangeRoot, filepath.FromSlash(rel))
			placeholder := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".icloud")
			if err := os.Rename(path, placeholder); err != nil {
				t.Fatal(err)
			}
			calls := 0
			opts := syncexchange.FolderOptions{
				Folder: env.exchange, WorkspaceID: desk.State().Catalog().WorkspaceID,
				RequestDownloads: func(paths []string) error {
					calls++
					if !slices.Contains(paths, path) {
						t.Fatalf("requests %v omit %s", paths, path)
					}
					return os.Rename(placeholder, path)
				},
			}
			report, err := syncexchange.ExchangeFolder(desk.SyncCommits(), opts)
			if kind != "identity" && err != nil {
				t.Fatal(err)
			}
			if report == nil || report.Imported != 0 || !slices.Contains(report.Unavailable, rel) || calls != 1 {
				t.Fatalf("request was mistaken for delivery: %+v, calls %d, err %v", report, calls, err)
			}
			report, err = syncexchange.ExchangeFolder(desk.SyncCommits(), opts)
			if err != nil || report.Imported != 1 || len(report.Unavailable) != 0 || calls != 1 {
				t.Fatalf("delivered content did not import: %+v, calls %d, err %v", report, calls, err)
			}
			requireSameTree(t, "cloud delivery", syncedDigests(t, peer.DataDir()), syncedDigests(t, desk.DataDir()))
		})
	}
}

func TestFolderExchangePersistsDownloadRequestFailure(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, enrollingDevice)
	path := filepath.Join(env.exchange, syncexchange.ExchangeRoot, "workspace.json")
	if err := os.Rename(path, filepath.Join(filepath.Dir(path), ".workspace.json.icloud")); err != nil {
		t.Fatal(err)
	}
	report, _ := syncexchange.ExchangeFolder(desk.SyncCommits(), syncexchange.FolderOptions{
		Folder: env.exchange, WorkspaceID: desk.State().Catalog().WorkspaceID,
		RequestDownloads: func([]string) error { return errors.New("download provider offline") },
	})
	if report == nil || !strings.Contains(strings.Join(report.Errors, " "), "download provider offline") {
		t.Fatalf("download failure was hidden: %+v", report)
	}
	persisted, _, err := syncexchange.LoadFolderStatus(desk.SyncCommits().SyncHistory())
	if err != nil || persisted == nil || !slices.Contains(persisted.Errors, "download provider offline") {
		t.Fatalf("download failure not persisted: %+v (%v)", persisted, err)
	}
}
