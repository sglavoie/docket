package syncmerge_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

// Retained history can include another device identity's branch before its
// materialized merge has been written to this device's workspace. The tracked
// digests still describe the actual, recorded files and must permit the next
// incoming transaction to reconcile all retained branches.
func TestIncomingReconcilesIndexedBranchesAgainstTrackedWorkspace(t *testing.T) {
	for _, changesPlanning := range []bool{false, true} {
		name := "unrelated_incoming"
		if changesPlanning {
			name = "planning_incoming"
		}
		t.Run(name, func(t *testing.T) {
			c := newCluster(t, "device-a", "device-b", "device-c")
			a, b, peer := c.r("device-a"), c.r("device-b"), c.r("device-c")
			a.commit(a.planningArtifact(func(p *config.PlanningState) {
				p.Today = append(p.Today, config.ProjectTaskLocator{ProjectID: atlasID, TaskID: awaitCert})
			}))
			b.commit(b.planningArtifact(func(p *config.PlanningState) {
				p.Today = append(p.Today, config.ProjectTaskLocator{ProjectID: atlasID, TaskID: darkIcons})
			}))
			copyTree(t, b.history.Dir(), a.history.Dir())
			if changesPlanning {
				peer.commit(peer.planningArtifact(func(p *config.PlanningState) { p.Next = nil }))
			} else {
				peer.editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
					findTask(t, doc, shipBeta).Priority = 3
				})
			}
			observed, err := a.svc.CommitExternalChanges()
			if err != nil || observed.Imported != nil || len(observed.Held) != 0 {
				t.Fatalf("recorded workspace was treated as an external edit: %+v, %v", observed, err)
			}
			a.pull(peer)
			observer := c.clone("observer")
			observer.pull(a, b, peer)
			requireConverged(t, []*replica{a, observer})
		})
	}
}

// An older device identity can leave indexed transactions in the same working
// copy without advancing the active device's cached heads. A resolution must
// acknowledge those writers too, or each attempt becomes another competitor.
func TestResolutionAcknowledgesAllIndexedBranches(t *testing.T) {
	c := newCluster(t, "device-a", "device-b")
	a, b := c.r("device-a"), c.r("device-b")
	for _, r := range []*replica{a, b} {
		r.editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, shipBeta).Title = "Title from " + r.device
		})
	}
	// Copying the peer's retained history reproduces the shared index with
	// device-local tracking that has not incorporated that branch.
	copyTree(t, b.history.Dir(), a.history.Dir())
	conflicts, conflictErr := a.svc.SyncConflicts()
	if conflictErr != nil || len(conflicts) != 1 {
		t.Fatalf("setup conflicts: %+v, %v", conflicts, conflictErr)
	}
	current := a.read(conflicts[0].Path)
	if _, err := a.svc.CommitSyncResolution(conflicts[0].ID, current); err != nil {
		t.Fatal(err)
	}
	for _, r := range []*replica{a, c.open(a.device, a.root)} {
		if open, err := r.svc.SyncConflicts(); err != nil || len(open) != 0 {
			t.Fatalf("keeping current created another conflict: %+v, %v", open, err)
		}
		if !bytes.Equal(current, r.read(conflicts[0].Path)) {
			t.Fatal("keeping current changed the file")
		}
	}
	b.pull(a)
	if open := requireConverged(t, []*replica{a, b}); len(open) != 0 {
		t.Fatalf("resolution reopened on the peer: %+v", open)
	}
	// The old identity's branch may never have reached the exchange. The active
	// outbox must carry it so a fresh peer can apply the resolution on its own.
	pending, err := a.history.Pending()
	if err != nil || len(pending) != 3 {
		t.Fatalf("resolution did not queue its retained ancestors: %d, %v", len(pending), err)
	}
	observer := c.clone("new-peer")
	observer.receive(a.history, pending...)
	if open := requireConverged(t, []*replica{a, observer}); len(open) != 0 {
		t.Fatalf("fresh peer did not receive the full resolution: %+v", open)
	}
	var published []string
	for _, m := range pending {
		published = append(published, m.TxnID)
	}
	if err := a.svc.CommitSyncPublished(published); err != nil {
		t.Fatal(err)
	}
	if pending, err := a.history.Pending(); err != nil || len(pending) != 0 {
		t.Fatalf("ancestor publication did not drain the active outbox: %d, %v", len(pending), err)
	}
}

// settleOrders is how many genesis observers receive each scenario's
// transactions, each in its own order: forward, reversed, and seeded shuffles.
const settleOrders = 6

func strPtr(s string) *string { return &s }

// TestSyncReconcileConvergesAcrossDeliveryOrders drives two and three devices
// through concurrent changes and holds every device, and observers receiving
// the same transactions in other orders, to identical valid content and
// identical conflict records.
func TestSyncReconcileConvergesAcrossDeliveryOrders(t *testing.T) {
	t.Run("independent_edits", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		c.r("device-a").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, shipBeta).Title = "Ship beta build today"
		})
		c.r("device-b").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, shipBeta).Priority = 3
			findTask(t, doc, localize).Title = "Localize installer (fr)"
		})
		c.r("device-b").editDoc("home-renovation", func(doc *storage.ProjectDocument) {
			findTask(t, doc, homeTask1).Title = "Paint the hallway"
		})
		replicas := c.settle(settleOrders)
		if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
			t.Fatalf("independent edits left conflicts: %+v", conflicts)
		}
		doc := replicas[0].doc("atlas-launch")
		if task := findTask(t, doc, shipBeta); task.Title != "Ship beta build today" || task.Priority != 3 {
			t.Fatalf("merged task = %q priority %d", task.Title, task.Priority)
		}
	})

	t.Run("competing_edits", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b", "device-c")
		for _, device := range []string{"device-a", "device-b", "device-c"} {
			c.r(device).editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
				findTask(t, doc, shipBeta).Title = "Ship beta from " + device
			})
		}
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		if len(conflicts) != 1 || len(conflicts[0].Candidates) != 3 {
			t.Fatalf("three competing titles = %+v, want one conflict naming three versions", conflicts)
		}
		title := findTask(t, replicas[0].doc("atlas-launch"), shipBeta).Title
		if !slices.Contains([]string{"Ship beta from device-a", "Ship beta from device-b", "Ship beta from device-c"}, title) {
			t.Fatalf("conflicted task holds %q, which no device wrote", title)
		}
	})
}

// TestSyncReconcilePlanningOrder merges concurrent Today and Next changes:
// compatible reorders and additions merge, incompatible reorders keep the base
// order, and one task planned for both collections stays in Today alone.
func TestSyncReconcilePlanningOrder(t *testing.T) {
	c := newCluster(t, "device-a", "device-b")
	a, b := c.r("device-a"), c.r("device-b")
	homeTwo := config.ProjectTaskLocator{ProjectID: homeID, TaskID: homeTask2}
	homeOne := config.ProjectTaskLocator{ProjectID: homeID, TaskID: homeTask1}
	dark := config.ProjectTaskLocator{ProjectID: atlasID, TaskID: darkIcons}
	cert := config.ProjectTaskLocator{ProjectID: atlasID, TaskID: awaitCert}
	a.commit(a.planningArtifact(func(p *config.PlanningState) {
		slices.Reverse(p.Today)
		p.Today = append(p.Today, cert)
		p.Next = append(p.Next, dark)
	}))
	b.commit(b.planningArtifact(func(p *config.PlanningState) {
		p.Today = append(p.Today, dark)
		p.Next = append([]config.ProjectTaskLocator{}, p.Next...)
		slices.Reverse(p.Next)
	}))
	replicas := c.settle(settleOrders)
	conflicts := requireConverged(t, replicas)
	planning := replicas[0].planning()
	wantToday := []config.ProjectTaskLocator{homeTwo, {ProjectID: atlasID, TaskID: shipBeta}, cert, dark}
	if !slices.Equal(planning.Today, wantToday) {
		t.Fatalf("Today = %+v, want %+v", planning.Today, wantToday)
	}
	if slices.Contains(planning.Next, dark) {
		t.Fatalf("Next still holds a task Today kept: %+v", planning.Next)
	}
	if len(planning.Next) == 0 || planning.Next[0] == homeOne {
		t.Fatalf("Next = %+v, want device-b's reversed order", planning.Next)
	}
	if len(conflictsAbout(conflicts, "Today is kept")) != 1 {
		t.Fatalf("conflicts = %+v, want the Today/Next collision recorded", conflicts)
	}

	t.Run("incompatible_reorders_keep_base", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		a, b := c.r("device-a"), c.r("device-b")
		a.commit(a.planningArtifact(func(p *config.PlanningState) { p.Today = append(p.Today, cert) }))
		b.pull(a)
		base := a.planning().Today
		a.commit(a.planningArtifact(func(p *config.PlanningState) {
			p.Today = []config.ProjectTaskLocator{base[2], base[0], base[1]}
		}))
		b.commit(b.planningArtifact(func(p *config.PlanningState) {
			p.Today = []config.ProjectTaskLocator{base[1], base[0], base[2]}
		}))
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		today := replicas[0].planning().Today
		if !slices.Equal(today, base) {
			t.Fatalf("Today = %+v, want the base order %+v kept", today, base)
		}
		if len(conflictsAbout(conflicts, "incompatible")) != 1 {
			t.Fatalf("conflicts = %+v, want one incompatible-reorder conflict", conflicts)
		}
	})
}

// TestSyncReconcileDeleteEdit keeps a change over a concurrent deletion, for a
// task and for a whole project.
func TestSyncReconcileDeleteEdit(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		c.r("device-a").editDoc("atlas-launch", func(doc *storage.ProjectDocument) { dropTask(doc, localize) })
		c.r("device-a").editDoc("atlas-launch", func(doc *storage.ProjectDocument) { dropTask(doc, darkIcons) })
		c.r("device-b").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, localize).Title = "Localize installer in French"
		})
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		doc := replicas[0].doc("atlas-launch")
		if findTask(t, doc, localize).Title != "Localize installer in French" {
			t.Fatal("the edit lost to a concurrent deletion")
		}
		if hasTask(doc, darkIcons) {
			t.Fatal("an uncontested deletion was undone")
		}
		if len(conflictsAbout(conflicts, "delete_edit")) != 1 {
			t.Fatalf("conflicts = %+v, want one delete_edit", conflicts)
		}
	})

	t.Run("project", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		c.r("device-a").deleteProject("home-renovation", homeID)
		c.r("device-b").editDoc("home-renovation", func(doc *storage.ProjectDocument) {
			findTask(t, doc, homeTask1).Title = "Order the tiles"
		})
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		catalog, _ := replicas[0].catalog()
		kept := false
		for _, p := range catalog.Projects {
			kept = kept || (p.ID == homeID && p.Slug == "home-renovation")
		}
		if !kept {
			t.Fatalf("catalog dropped a project whose tasks changed concurrently: %+v", catalog.Projects)
		}
		if findTask(t, replicas[0].doc("home-renovation"), homeTask1).Title != "Order the tiles" {
			t.Fatal("the project's edited tasks.md was not kept")
		}
		if len(conflictsAbout(conflicts, "project is kept")) != 1 {
			t.Fatalf("conflicts = %+v, want the restored project recorded", conflicts)
		}
		if slices.ContainsFunc(replicas[0].planning().Today, func(l config.ProjectTaskLocator) bool { return l.ProjectID == homeID }) {
			t.Fatal("planning kept references the deleting device removed without competition")
		}
	})

	t.Run("project_uncontested", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		c.r("device-a").deleteProject("home-renovation", homeID)
		c.r("device-b").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, shipBeta).Title = "Ship it"
		})
		replicas := c.settle(settleOrders)
		if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
			t.Fatalf("an uncontested project deletion left conflicts: %+v", conflicts)
		}
		if replicas[0].exists("projects/home-renovation/tasks.md") {
			t.Fatal("an uncontested project deletion was undone")
		}
	})
}
