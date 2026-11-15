package store

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/outline"
)

// TestSyncConvergesUnderRandomWork runs three machines that each make random
// changes, including moves that can loop and deletes, while a courier copies
// their files to one another late, out of order and cut short. Once every
// file has arrived whole, all three must hold the same outline.
func TestSyncConvergesUnderRandomWork(t *testing.T) {
	seeds := 40
	if testing.Short() {
		seeds = 5
	}
	for seed := 1; seed <= seeds; seed++ {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) { fuzzSync(t, int64(seed)) })
	}
}

func fuzzSync(t *testing.T, seed int64) {
	rng := rand.New(rand.NewSource(seed))
	const n = 3
	machines := make([]*machine, n)
	for i := range machines {
		// Each machine has its own copy of the shared folder: files reach
		// it only when the courier brings them.
		machines[i] = newMachine(t, t.TempDir())
		machines[i].now = 1000 + int64(rng.Intn(500)) // clocks disagree
	}
	fields := []string{"text", "state", "flag", "due"}
	positions := []string{"H", "V", "V", "k"} // few on purpose: ties must order the same everywhere
	made := 0

	work := func(m *machine, who int) {
		tree, err := m.s.Load()
		if err != nil {
			t.Fatal(err)
		}
		var all []string
		var walk func(string)
		walk = func(parent string) {
			for _, id := range tree.Children(parent) {
				all = append(all, id)
				walk(id)
			}
		}
		walk("")
		pick := func() string {
			if len(all) == 0 || rng.Intn(6) == 0 {
				return ""
			}
			return all[rng.Intn(len(all))]
		}
		m.now += int64(rng.Intn(40)) - 5 // sometimes backwards
		var op outline.Op
		switch k := rng.Intn(10); {
		case k < 3 || len(all) == 0:
			made++
			op = outline.Op{Kind: outline.Insert, Row: fmt.Sprintf("m%d-%d", who, made), Parent: pick(), Pos: positions[rng.Intn(len(positions))]}
		case k < 6:
			op = set(all[rng.Intn(len(all))], fields[rng.Intn(len(fields))], fmt.Sprintf("v%d", rng.Intn(5)))
		case k < 9:
			op = outline.Op{Kind: outline.Move, Row: all[rng.Intn(len(all))], Parent: pick(), Pos: positions[rng.Intn(len(positions))]}
		default:
			op = set(all[rng.Intn(len(all))], "deleted", []string{"1", ""}[rng.Intn(2)])
		}
		// A machine only makes changes that are valid in its own outline.
		if _, err := tree.Apply(op, m.now); err != nil {
			return
		}
		if err := m.s.Commit([]outline.Op{op}, m.now); err != nil {
			t.Fatal(err)
		}
	}

	// carry copies one machine's file to another's folder. Files only grow,
	// so a partial copy is a longer prefix than the last one, cut anywhere.
	carry := func(from, to *machine, whole bool) {
		name := from.s.device + ".ops"
		data, err := os.ReadFile(filepath.Join(from.dir, name))
		if err != nil {
			return // nothing written yet
		}
		dst := filepath.Join(to.dir, name)
		have := 0
		if old, err := os.ReadFile(dst); err == nil {
			have = len(old)
		}
		upto := len(data)
		if !whole && upto > have {
			upto = have + rng.Intn(upto-have+1)
		}
		if err := os.WriteFile(dst, data[:upto], 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for step := 0; step < 250; step++ {
		i := rng.Intn(n)
		switch rng.Intn(5) {
		case 0:
			machines[i].sync()
		case 1:
			if j := rng.Intn(n); j != i {
				carry(machines[i], machines[j], false)
			}
		default:
			work(machines[i], i)
		}
	}

	// Everything arrives.
	for round := 0; round < 3; round++ {
		for _, m := range machines {
			m.sync()
		}
		for i, from := range machines {
			for j, to := range machines {
				if i != j {
					carry(from, to, true)
				}
			}
		}
	}
	for _, m := range machines {
		m.sync()
	}

	want := machines[0].shape()
	for i, m := range machines[1:] {
		if got := m.shape(); got != want {
			t.Fatalf("machine %d disagrees with machine 0:\n%s\nand:\n%s", i+1, want, got)
		}
		var waiting int
		if err := m.s.db.QueryRow(`SELECT COUNT(*) FROM pending`).Scan(&waiting); err != nil || waiting != 0 {
			t.Fatalf("machine %d still holds %d operations back (%v)", i+1, waiting, err)
		}
	}
	if want == "" {
		t.Fatal("the run made no rows; the test proves nothing")
	}

	// Nothing is lost: every row that is not deleted, and has no deleted
	// row above it, can be reached from the top.
	tree, err := machines[0].s.Load()
	if err != nil {
		t.Fatal(err)
	}
	reachable := map[string]bool{}
	var walk func(string)
	walk = func(parent string) {
		for _, id := range tree.Children(parent) {
			reachable[id] = true
			walk(id)
		}
	}
	walk("")
	rs, err := machines[0].s.db.Query(`SELECT id FROM rows`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	total := 0
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			t.Fatal(err)
		}
		total++
		hidden, steps := false, 0
		for at := id; at != ""; at = tree.Row(at).Parent {
			if steps++; steps > total+1000 {
				t.Fatalf("row %s sits in a loop", id)
			}
			hidden = hidden || tree.Row(at).Deleted()
		}
		if !hidden && !reachable[id] {
			t.Fatalf("row %s is live and cannot be reached", id)
		}
	}
}
