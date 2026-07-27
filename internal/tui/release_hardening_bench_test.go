package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func BenchmarkReleaseFinderSearchLargeCatalog(b *testing.B) {
	project := scopeTestProject("release", "Release Project", releaseTaskIDs(2000)...)
	project.Tasks[len(project.Tasks)-1].Title = "国際化 release latency needle"
	finder, err := NewFinder(
		config.NewMachineState(),
		FinderTaskProvider{Projects: []*models.Project{project}},
	)
	if err != nil {
		b.Fatalf("NewFinder: %v", err)
	}
	b.ReportMetric(float64(len(finder.Items())), "items")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if results := finder.Results("latency needle"); len(results.Results) != 1 {
			b.Fatalf("Finder results = %d, want 1", len(results.Results))
		}
	}
}

func BenchmarkReleaseOutlinerForcedRebuildLargeCatalog(b *testing.B) {
	project := scopeTestProject("release", "Release Project", releaseTaskIDs(2000)...)
	outliner := scopeTestOutliner(project)
	b.ReportMetric(float64(len(project.Tasks)), "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		outliner.crossForceRebuild = true
		outliner.crossProjectRebuildRows()
	}
}

func BenchmarkReleaseOutlinerMemoizedRebuildLargeCatalog(b *testing.B) {
	project := scopeTestProject("release", "Release Project", releaseTaskIDs(2000)...)
	outliner := scopeTestOutliner(project)
	b.ReportMetric(float64(len(project.Tasks)), "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		outliner.crossProjectRebuildRows()
	}
}

func releaseTaskIDs(count int) []string {
	ids := make([]string, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("release-task-%04d", i+1)
	}
	return ids
}
