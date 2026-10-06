package syncmerge_test

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
)

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

// TestSyncReconcileProjectMoves lands an edit made under a project's old slug
// in the directory a concurrent rename moved it to.
func TestSyncReconcileProjectMoves(t *testing.T) {
	c := newCluster(t, "device-a", "device-b")
	a, b := c.r("device-a"), c.r("device-b")
	a.renameProject("atlas-launch", "atlas")
	b.editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
		findTask(t, doc, shipBeta).Title = "Ship beta (moved)"
	})
	b.commit(b.put("projects/atlas-launch/notes/2026-10-06-120000_11111111-launch-retro.md", []byte("# Launch retro\n")))
	replicas := c.settle(settleOrders)
	if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
		t.Fatalf("a rename and an edit left conflicts: %+v", conflicts)
	}
	r := replicas[0]
	if r.exists("projects/atlas-launch/tasks.md") || r.exists("projects/atlas-launch/notes/2026-10-06-120000_11111111-launch-retro.md") {
		t.Fatal("files remain under the old slug")
	}
	if findTask(t, r.doc("atlas"), shipBeta).Title != "Ship beta (moved)" {
		t.Fatal("the edit made under the old slug did not follow the rename")
	}
	if !r.exists("projects/atlas/notes/2026-10-06-120000_11111111-launch-retro.md") ||
		!r.exists("projects/atlas/notes/2026-09-03-090000_0a1b2c3d-release-runbook.md") {
		t.Fatal("notes did not follow the rename")
	}

	t.Run("cross_project_task_move", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		a, b := c.r("device-a"), c.r("device-b")
		moved := findTask(t, a.doc("atlas-launch"), darkIcons)
		atlas, home := a.doc("atlas-launch"), a.doc("home-renovation")
		dropTask(atlas, darkIcons)
		home.Tasks = append(home.Tasks, moved)
		moved.ShortID = 3
		a.commit(a.docArtifact("atlas-launch", atlas), a.docArtifact("home-renovation", home))
		b.editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, darkIcons).Title = "Dark mode icons v2"
		})
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		if !hasTask(replicas[0].doc("home-renovation"), darkIcons) {
			t.Fatal("the move lost its destination")
		}
		if findTask(t, replicas[0].doc("atlas-launch"), darkIcons).Title != "Dark mode icons v2" {
			t.Fatal("the edit made at the source was lost")
		}
		if len(conflictsAbout(conflicts, "delete_edit")) != 1 {
			t.Fatalf("conflicts = %+v, want the move/edit recorded", conflicts)
		}
	})
}

// TestSyncReconcileHierarchyInvariants keeps merged hierarchies valid: a
// concurrent cycle reverts to the base, and a parent deleted while a child was
// added beneath it survives.
func TestSyncReconcileHierarchyInvariants(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		c.r("device-a").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, localize).ParentID = strPtr(darkIcons)
		})
		c.r("device-b").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, darkIcons).ParentID = strPtr(localize)
		})
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		doc := replicas[0].doc("atlas-launch")
		if findTask(t, doc, localize).ParentID != nil || findTask(t, doc, darkIcons).ParentID != nil {
			t.Fatal("a concurrent cycle was not reverted to the base hierarchy")
		}
		if len(conflictsAbout(conflicts, "invalid hierarchy")) != 1 {
			t.Fatalf("conflicts = %+v, want one hierarchy conflict", conflicts)
		}
	})

	t.Run("depth", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		c.r("device-a").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			parent := proofread
			for i := 0; i < 3; i++ {
				child := newTask(doc, "0a7e5d10-2c3b-4f6a-8e9d-00000000a00"+string(rune('1'+i)), "Depth A", strPtr(parent))
				parent = child.ID
			}
		})
		c.r("device-b").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			findTask(t, doc, shipBeta).ParentID = strPtr(localize)
			findTask(t, doc, localize).ParentID = strPtr(awaitCert)
		})
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		if len(conflictsAbout(conflicts, "invalid hierarchy")) != 1 {
			t.Fatalf("conflicts = %+v, want the too-deep merge reverted", conflicts)
		}
		if findTask(t, replicas[0].doc("atlas-launch"), shipBeta).ParentID != nil {
			t.Fatal("reparenting that nested too deep was kept")
		}
	})

	t.Run("deleted_parent_gains_child", func(t *testing.T) {
		c := newCluster(t, "device-a", "device-b")
		c.r("device-a").editDoc("atlas-launch", func(doc *storage.ProjectDocument) { dropTask(doc, localize) })
		c.r("device-b").editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			newTask(doc, "0a7e5d10-2c3b-4f6a-8e9d-00000000b001", "Translate strings", strPtr(localize))
		})
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		doc := replicas[0].doc("atlas-launch")
		if !hasTask(doc, localize) || *findTask(t, doc, "0a7e5d10-2c3b-4f6a-8e9d-00000000b001").ParentID != localize {
			t.Fatal("a parent deleted while a child was added beneath it was not kept")
		}
		if len(conflictsAbout(conflicts, "still has")) != 1 {
			t.Fatalf("conflicts = %+v, want the kept parent recorded", conflicts)
		}
	})
}

// TestSyncReconcileShortIDAllocation keeps both of two tasks created
// concurrently under one short id, with their UUIDs, and renumbers one
// deterministically.
func TestSyncReconcileShortIDAllocation(t *testing.T) {
	c := newCluster(t, "device-a", "device-b", "device-c")
	ids := map[string]string{
		"device-a": "0a7e5d10-2c3b-4f6a-8e9d-00000000c00a",
		"device-b": "0a7e5d10-2c3b-4f6a-8e9d-00000000c00b",
		"device-c": "0a7e5d10-2c3b-4f6a-8e9d-00000000c00c",
	}
	for device, id := range ids {
		c.r(device).editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
			if task := newTask(doc, id, "Captured on "+device, nil); task.ShortID != 10 {
				t.Fatalf("minted short id %d, want 10", task.ShortID)
			}
		})
	}
	replicas := c.settle(settleOrders)
	if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
		t.Fatalf("concurrent short ids are resolved, not conflicts: %+v", conflicts)
	}
	doc := replicas[0].doc("atlas-launch")
	got := map[string]int{}
	for _, id := range ids {
		got[id] = findTask(t, doc, id).ShortID
	}
	want := map[string]int{ids["device-a"]: 10, ids["device-b"]: 11, ids["device-c"]: 12}
	for id, short := range want {
		if got[id] != short {
			t.Fatalf("short ids = %v, want %v (smallest UUID keeps the number)", got, want)
		}
	}
}

// TestSyncReconcileMultipleCommonAncestors builds a criss-cross history: two
// devices exchange concurrent edits, then each edits again on top of both, so
// the newest edits share two common ancestors. The merge base is the merge of
// those ancestors, and a third device that saw nothing until the end agrees.
func TestSyncReconcileMultipleCommonAncestors(t *testing.T) {
	c := newCluster(t, "device-a", "device-b", "device-c")
	a, b, cc := c.r("device-a"), c.r("device-b"), c.r("device-c")
	a.editDoc("atlas-launch", func(doc *storage.ProjectDocument) { findTask(t, doc, shipBeta).Priority = 3 })
	b.editDoc("atlas-launch", func(doc *storage.ProjectDocument) { findTask(t, doc, localize).Priority = 2 })
	a.pull(b)
	b.pull(a)
	a.editDoc("atlas-launch", func(doc *storage.ProjectDocument) { findTask(t, doc, darkIcons).Title = "Dark icons" })
	b.editDoc("atlas-launch", func(doc *storage.ProjectDocument) { findTask(t, doc, awaitCert).Title = "Await cert" })
	cc.editDoc("atlas-launch", func(doc *storage.ProjectDocument) { findTask(t, doc, notes).Title = "Release notes" })

	newest := a.outgoing()[1]
	if len(newest.Parents) != 2 {
		t.Fatalf("device-a's second edit has parents %v, want both first edits", newest.Parents)
	}
	replicas := c.settle(settleOrders)
	if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
		t.Fatalf("criss-cross independent edits left conflicts: %+v", conflicts)
	}
	doc := replicas[0].doc("atlas-launch")
	if findTask(t, doc, shipBeta).Priority != 3 || findTask(t, doc, localize).Priority != 2 ||
		findTask(t, doc, darkIcons).Title != "Dark icons" || findTask(t, doc, awaitCert).Title != "Await cert" ||
		findTask(t, doc, notes).Title != "Release notes" {
		t.Fatal("the criss-cross merge lost an edit")
	}
}

// TestSyncReconcileSimultaneousResolutions resolves one conflict on two
// devices at once: agreeing resolutions coalesce and close it everywhere,
// disagreeing ones compete again as one new conflict every device derives.
func TestSyncReconcileSimultaneousResolutions(t *testing.T) {
	setup := func(t *testing.T) (*cluster, synclog.Conflict) {
		c := newCluster(t, "device-a", "device-b")
		for _, device := range []string{"device-a", "device-b"} {
			c.r(device).editDoc("atlas-launch", func(doc *storage.ProjectDocument) {
				findTask(t, doc, shipBeta).Title = "Title from " + device
			})
		}
		c.r("device-a").pull(c.r("device-b"))
		c.r("device-b").pull(c.r("device-a"))
		conflicts := requireConverged(t, []*replica{c.r("device-a"), c.r("device-b")})
		if len(conflicts) != 1 {
			t.Fatalf("setup conflicts = %+v", conflicts)
		}
		return c, conflicts[0]
	}
	resolve := func(t *testing.T, r *replica, conflict synclog.Conflict, title string) {
		doc := r.doc("atlas-launch")
		findTask(t, doc, shipBeta).Title = title
		content, err := doc.WriteCurrent()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.svc.CommitSyncResolution(conflict.ID, []byte(content)); err != nil {
			t.Fatalf("%s resolves: %v", r.device, err)
		}
		if open, _ := r.svc.SyncConflicts(); len(open) != 0 {
			t.Fatalf("%s still sees %+v after resolving", r.device, open)
		}
	}

	t.Run("agree", func(t *testing.T) {
		c, conflict := setup(t)
		resolve(t, c.r("device-a"), conflict, "Agreed title")
		resolve(t, c.r("device-b"), conflict, "Agreed title")
		replicas := c.settle(settleOrders)
		if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
			t.Fatalf("agreeing resolutions left conflicts: %+v", conflicts)
		}
		if findTask(t, replicas[0].doc("atlas-launch"), shipBeta).Title != "Agreed title" {
			t.Fatal("the agreed resolution did not land")
		}
	})

	t.Run("keep_current", func(t *testing.T) {
		c, conflict := setup(t)
		a := c.r("device-a")
		if _, err := a.svc.CommitSyncResolution(conflict.ID, a.read(conflict.Path)); err != nil {
			t.Fatalf("resolve by keeping the current version: %v", err)
		}
		replicas := c.settle(settleOrders)
		if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
			t.Fatalf("keeping the current version did not resolve: %+v", conflicts)
		}
	})

	t.Run("disagree", func(t *testing.T) {
		c, conflict := setup(t)
		resolve(t, c.r("device-a"), conflict, "Resolved by device-a")
		resolve(t, c.r("device-b"), conflict, "Resolved by device-b")
		replicas := c.settle(settleOrders)
		conflicts := requireConverged(t, replicas)
		if len(conflicts) != 1 || conflicts[0].ID == conflict.ID {
			t.Fatalf("disagreeing resolutions = %+v, want one new conflict", conflicts)
		}
		resolutions := append(c.r("device-a").outgoing(), c.r("device-b").outgoing()...)
		var resolutionIDs []string
		for _, m := range resolutions {
			if m.Kind == synclog.KindResolution {
				resolutionIDs = append(resolutionIDs, m.TxnID)
			}
		}
		var writers []string
		for _, candidate := range conflicts[0].Candidates {
			writers = append(writers, candidate.Writers...)
		}
		slices.Sort(resolutionIDs)
		slices.Sort(writers)
		if !slices.Equal(writers, resolutionIDs) {
			t.Fatalf("the new conflict's writers %v are not the two resolutions %v", writers, resolutionIDs)
		}
	})
}
