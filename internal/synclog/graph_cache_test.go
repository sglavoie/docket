package synclog

import (
	"fmt"
	"reflect"
	"testing"
)

// TestOpenConflictsCacheFollowsEveryImport pins the history's graph cache: a
// History that derived conflicts before an import derives exactly what a fresh
// History over the same directory derives after it, so neither a reused manifest
// nor a skipped index walk can hide a transaction the state file admitted.
func TestOpenConflictsCacheFollowsEveryImport(t *testing.T) {
	for seed := int64(0); seed < 6; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			s := seededHistory(t, seed, 30)
			h := bootstrapTestHistory(t)
			for i, m := range s.manifests {
				plan, err := h.PlanReconcile(m, s.blobs, synthMerger{}, fmt.Sprintf("local-%d", i))
				if err != nil {
					t.Fatal(err)
				}
				applyBootstrapTestPlan(t, plan)
				cached, err := h.OpenConflicts(synthMerger{})
				if err != nil {
					t.Fatal(err)
				}
				fresh, err := New(Config{DataDir: h.dataDir, DeviceID: "observer", Schemas: h.schemas})
				if err != nil {
					t.Fatal(err)
				}
				want, err := fresh.OpenConflicts(synthMerger{})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cached, want) {
					t.Fatalf("after import %d the cached history derived\n%+v\nwant\n%+v", i, cached, want)
				}
			}
			if len(h.manifests.indexed) != len(s.manifests) {
				t.Fatalf("cache holds %d manifests, want %d", len(h.manifests.indexed), len(s.manifests))
			}
		})
	}
}
