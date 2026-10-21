package bridge

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"
)

func waitForLocalRequest(t *testing.T, gate *requestGate) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		gate.mu.Lock()
		waiting := gate.localWaiting
		gate.mu.Unlock()
		if waiting > 0 {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("local request never queued")
}

func TestRequestGateYieldsToLocalBeforeMoreSync(t *testing.T) {
	var gate requestGate
	gate.enter(true)
	order := make(chan string, 2)
	go func() { gate.enter(true); order <- "sync"; gate.leave() }()
	go func() { gate.enter(false); order <- "local"; gate.leave() }()
	waitForLocalRequest(t, &gate)
	gate.leave()
	if first := <-order; first != "local" {
		t.Fatalf("first = %s", first)
	}
	if second := <-order; second != "sync" {
		t.Fatalf("second = %s", second)
	}
}

func TestIncomingTransactionLetsLocalEditCommitFirst(t *testing.T) {
	f := newExchangeFixture(t)
	b := f.phone.bridge
	history := f.mac.bridge.workspace.runtime.SyncCommits().SyncHistory()
	manifest, err := history.Transaction(f.outgoing.TxnID)
	if err != nil {
		t.Fatal(err)
	}
	var projects projectsResult
	f.phone.ok(f.phone.call("query.projects", nil), &projects)
	projectID := ""
	for _, project := range projects.Active {
		if project.Slug != "unfiled" {
			projectID = project.ID
			break
		}
	}
	if projectID == "" {
		t.Fatal("fixture needs a second project")
	}
	request, err := json.Marshal(map[string]any{
		"bridge_version": Version, "request_id": "local-during-sync", "op": "command.capture",
		"workspace": f.phone.workspace, "payload": map[string]any{"project_id": projectID, "title": "Local while syncing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b.requests.enter(true)
	reply := make(chan string, 1)
	go func() { reply <- b.Call(string(request)) }()
	waitForLocalRequest(t, &b.requests)
	importer := yieldingImport{b, b.workspace, b.workspace.runtime.SyncCommits()}
	_, err = importer.CommitIncoming(manifest, history)
	b.requests.leave()
	if err != nil {
		t.Fatal(err)
	}
	var decoded response
	if err := json.Unmarshal([]byte(<-reply), &decoded); err != nil {
		t.Fatal(err)
	}
	f.phone.ok(decoded, nil)
	if !f.phone.hasTask("Local while syncing") || !f.phone.hasTask("From the Mac") {
		t.Fatal("local or incoming work lost")
	}
}

func TestIncomingTransactionRefusesAfterWorkspaceClosesWhileYielding(t *testing.T) {
	f := newExchangeFixture(t)
	b := f.phone.bridge
	history := f.mac.bridge.workspace.runtime.SyncCommits().SyncHistory()
	manifest, err := history.Transaction(f.outgoing.TxnID)
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(map[string]any{
		"bridge_version": Version, "request_id": "close-during-sync", "op": "workspace.close", "workspace": f.phone.workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	b.requests.enter(true)
	importer := yieldingImport{b, b.workspace, b.workspace.runtime.SyncCommits()}
	reply := make(chan string, 1)
	go func() { reply <- b.Call(string(request)) }()
	waitForLocalRequest(t, &b.requests)
	_, err = importer.CommitIncoming(manifest, history)
	b.requests.leave()
	if err == nil {
		t.Fatal("old sync pass wrote after close")
	}
	<-reply
	if _, known, err := importer.SyncHistory().Lookup(manifest.TxnID); err != nil || known {
		t.Fatalf("closed workspace received transaction: known=%v, err=%v", known, err)
	}
}
