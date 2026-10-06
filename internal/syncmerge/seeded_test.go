package syncmerge_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestSyncReconcileSeededHistoriesConverge lets three devices make seeded
// random edits — retitles, priorities, creations, leaf deletions,
// reparenting, planning changes — interleaved with partial exchanges that
// build arbitrary concurrent and criss-cross histories, then holds every
// device and every reordered observer to one valid result.
func TestSyncReconcileSeededHistoriesConverge(t *testing.T) {
	for seed := int64(1); seed <= 4; seed++ {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewSource(seed))
			devices := []string{"device-a", "device-b", "device-c"}
			c := newCluster(t, devices...)
			minted := 0
			for step := 0; step < 18; step++ {
				r := c.r(devices[rng.Intn(len(devices))])
				switch op := rng.Intn(8); op {
				case 0, 7:
					other := c.r(devices[rng.Intn(len(devices))])
					if other != r {
						r.pull(other)
					}
				case 5:
					r.commit(r.planningArtifact(func(p *config.PlanningState) { seededPlanning(rng, r, p) }))
				default:
					slug := []string{"atlas-launch", "home-renovation"}[rng.Intn(2)]
					r.editDoc(slug, func(doc *storage.ProjectDocument) {
						minted++
						seededEdit(rng, doc, op, fmt.Sprintf("0a7e5d10-2c3b-4f6a-8e9d-%012x", 0xd000+minted+int(seed)*100))
					})
				}
			}
			all, _ := c.everything()
			concurrent := 0
			for _, m := range all {
				if len(m.Parents) > 1 {
					concurrent++
				}
			}
			conflicts := requireConverged(t, c.settle(2))
			t.Logf("%d transactions, %d merging several heads, %d open conflicts", len(all), concurrent, len(conflicts))
		})
	}
}

func seededEdit(rng *rand.Rand, doc *storage.ProjectDocument, op int, newID string) {
	task := doc.Tasks[rng.Intn(len(doc.Tasks))]
	switch op {
	case 1:
		task.Title = fmt.Sprintf("Seeded title %d", rng.Intn(3))
	case 2:
		task.Priority = rng.Intn(4)
	case 3:
		newTask(doc, newID, "Seeded capture", nil)
	case 4:
		for _, other := range doc.Tasks {
			if other.ParentID != nil && *other.ParentID == task.ID {
				return
			}
		}
		if len(doc.Tasks) > 1 {
			dropTask(doc, task.ID)
		}
	case 6:
		parent := doc.Tasks[rng.Intn(len(doc.Tasks))]
		if task.ParentID != nil || parent.ParentID != nil || parent == task {
			return
		}
		for _, other := range doc.Tasks {
			if other.ParentID != nil && *other.ParentID == task.ID {
				return
			}
		}
		id := parent.ID
		task.ParentID = &id
	}
}

func seededPlanning(rng *rand.Rand, r *replica, p *config.PlanningState) {
	doc := r.doc("atlas-launch")
	task := doc.Tasks[rng.Intn(len(doc.Tasks))]
	if task.IsTerminalState() || task.State() == models.StateBacklog {
		p.Today = append([]config.ProjectTaskLocator{}, p.Today...)
		for i, j := 0, len(p.Today)-1; i < j; i, j = i+1, j-1 {
			p.Today[i], p.Today[j] = p.Today[j], p.Today[i]
		}
		return
	}
	item := config.ProjectTaskLocator{ProjectID: atlasID, TaskID: task.ID}
	p.Today = withoutLocator(p.Today, item)
	p.Next = withoutLocator(p.Next, item)
	if rng.Intn(2) == 0 {
		p.Today = append(p.Today, item)
	} else {
		p.Next = append(p.Next, item)
	}
}

func withoutLocator(items []config.ProjectTaskLocator, drop config.ProjectTaskLocator) []config.ProjectTaskLocator {
	var out []config.ProjectTaskLocator
	for _, item := range items {
		if item != drop {
			out = append(out, item)
		}
	}
	return out
}
