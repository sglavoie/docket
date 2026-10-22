package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/synclog"
)

func TestExchangeRefusesReusedAuthorSequences(t *testing.T) {
	for _, scenario := range []string{"received", "own", "outgoing"} {
		t.Run(scenario, func(t *testing.T) {
			env := newEnrollEnvironment(t)
			env.enroll(t)
			desk, peer := env.open(t, env.workingCopy, enrollingDevice), env.join(t)
			captureIn(t, peer, "Keep this phone change")
			exchangeNow(t, peer)
			exchangeNow(t, desk)
			author := peer.SyncCommits().SyncHistory().DeviceID()
			if scenario == "own" {
				author = enrollingDevice
			}
			manifest := *exchangeManifests(t, env.exchange, author)[0]
			if scenario == "outgoing" {
				captureIn(t, desk, "Keep this unpublished desktop change")
				pending, err := desk.SyncCommits().SyncHistory().Pending()
				if err != nil || len(pending) != 1 {
					t.Fatalf("pending: %v, %v", pending, err)
				}
				manifest = *pending[0]
			} else {
				captureIn(t, desk, "Do not publish into a fork")
			}
			created, err := time.Parse(time.RFC3339Nano, manifest.CreatedAt)
			if err != nil {
				t.Fatal(err)
			}
			manifest.CreatedAt = created.Add(time.Second).Format(time.RFC3339Nano)
			manifest.TxnID, err = manifest.ComputeTxnID()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := manifest.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = synclog.DecodeManifest(encoded); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(env.exchange, syncexchange.ExchangeRoot,
				syncexchange.ManifestPath(manifest.DeviceID, manifest.DeviceSeq, manifest.TxnID))
			if err = os.WriteFile(path, encoded, 0o600); err != nil {
				t.Fatal(err)
			}
			before := treeDigests(t, env.exchange)
			report, err := desk.SyncExchange(SyncExchangeOptions{Folder: env.exchange})
			if err == nil || !strings.Contains(err.Error(), "reused sequence") || report == nil || report.Succeeded {
				t.Fatalf("fork was hidden: %+v, %v", report, err)
			}
			if len(report.Published) != 0 || report.Imported != 0 || report.OutstandingUp != 1 {
				t.Fatalf("fork exchanged changes: %+v", report)
			}
			requireSameTree(t, "exchange after refusal", before, treeDigests(t, env.exchange))
			persisted, _, err := syncexchange.LoadFolderStatus(desk.SyncCommits().SyncHistory())
			if err != nil || persisted.Succeeded || len(persisted.Errors) == 0 {
				t.Fatalf("repair was not reported: %+v, %v", persisted, err)
			}
		})
	}
}

func TestJoinRetriesHistoryListedBeforeItsGenesis(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	peer, _, err := Join(JoinConfig{DataDir: filepath.Join(env.root, "early-peer"), Exchange: env.exchange,
		Machine: ExplicitMachine(env.home, "aaa-peer"), Clock: env.clock})
	if err != nil {
		t.Fatal(err)
	}
	captureIn(t, peer, "This author's name sorts before the genesis author")
	exchangeNow(t, peer)
	joined, report, err := JoinInbox(JoinInboxConfig{DataDir: filepath.Join(env.root, "fresh-phone"),
		Exchange: env.exchange, WorkspaceID: peer.State().Catalog().WorkspaceID,
		InboxDir: filepath.Join(env.exchange, syncexchange.ExchangeRoot),
		Machine:  ExplicitMachine(env.home, "fresh-phone"), Clock: env.clock})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) < 2 {
		t.Fatal("join must exercise a multi-manifest history")
	}
	batchID := report.Results[0].LocalTxnID
	for _, result := range report.Results {
		if result.Outcome != syncexchange.Imported {
			t.Fatalf("join needed another pass despite all history being present: %+v", result)
		}
		if batchID == "" || result.LocalTxnID != batchID {
			t.Fatalf("complete initial history was not committed together: %+v", report.Results)
		}
	}
	requireSameTree(t, "first-pass join", syncedDigests(t, peer.DataDir()), syncedDigests(t, joined.DataDir()))
	captureIn(t, joined, "Created after the initial batch")
	exchangeNow(t, joined)
	exchangeNow(t, peer)
	requireSameTree(t, "post-join sync", syncedDigests(t, peer.DataDir()), syncedDigests(t, joined.DataDir()))
}

func TestInitialImportFallsBackWhenLatestContentIsMissing(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, enrollingDevice)
	captureIn(t, desk, "First delivered edit")
	exchangeNow(t, desk)
	captureIn(t, desk, "Waiting for its content")
	pending, err := desk.SyncCommits().SyncHistory().Pending()
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending: %+v, %v", pending, err)
	}
	latest := pending[0]
	exchangeNow(t, desk)
	var digest string
	for _, effect := range latest.Effects {
		if effect.Kind == synclog.EffectProjectDocument && effect.Blob != nil {
			digest = *effect.Blob
		}
	}
	if digest == "" {
		t.Fatal("capture must change project content")
	}
	inbox := filepath.Join(env.exchange, syncexchange.ExchangeRoot)
	path := filepath.Join(inbox, syncexchange.BlobPath(digest))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(env.root, "partial-join")
	service, err := joiningCommits(root, "partial-phone", env.clock)
	if err != nil {
		t.Fatal(err)
	}
	report, err := syncexchange.ImportInbox(service, inbox)
	if err != nil {
		t.Fatal(err)
	}
	imported, waiting := 0, 0
	for _, result := range report.Results {
		switch result.Outcome {
		case syncexchange.Imported:
			imported++
		case syncexchange.Incomplete:
			waiting++
			if result.TxnID != latest.TxnID || len(result.Missing) == 0 {
				t.Fatalf("wrong incomplete result: %+v", result)
			}
		default:
			t.Fatalf("unexpected fallback result: %+v", result)
		}
	}
	if imported < 2 || waiting != 1 {
		t.Fatalf("complete prefix not imported: %+v", report.Results)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := syncexchange.ImportInbox(service, inbox); err != nil {
		t.Fatal(err)
	}
	requireSameTree(t, "completed partial join", syncedDigests(t, desk.DataDir()), syncedDigests(t, root))
}
