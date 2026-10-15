package outliner

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/textfit"
)

// projectAbbreviations keeps ordinary prefixes unless they collide. Colliding
// names retain progressively more of their suffix, using display cells rather
// than bytes. The loaded population, not the visible rows, determines the names,
// so folding and scrolling cannot change a label.
func projectAbbreviations(names []string, width int) map[string]string {
	names = slices.Clone(names)
	slices.Sort(names)
	names = slices.Compact(names)
	labels := make(map[string]string, len(names))
	counts := make(map[string]int, len(names))
	used := make(map[string]bool, len(names))
	for _, name := range names {
		counts[textfit.TruncateToVisualWidth(name, width)]++
	}
	for _, name := range names {
		label := textfit.TruncateToVisualWidth(name, width)
		if counts[label] == 1 || lipgloss.Width(name) <= width {
			labels[name], used[label] = label, true
		}
	}
	for _, name := range names {
		if _, ok := labels[name]; ok {
			continue
		}
		for head := width / 2; head >= 0; head-- {
			candidate := abbreviateProjectMiddle(name, width, head)
			unique := !used[candidate]
			for _, other := range names {
				if other != name && abbreviateProjectMiddle(other, width, head) == candidate {
					unique = false
					break
				}
			}
			if unique {
				labels[name], used[candidate] = candidate, true
				break
			}
		}
		if _, ok := labels[name]; ok {
			continue
		}
		// Extremely similar names may differ only in their middle. A stable ordinal
		// still keeps them distinct when neither end fits that distinction.
		for ordinal := 1; ; ordinal++ {
			suffix := fmt.Sprintf("~%d", ordinal)
			candidate := textfit.TruncateIntentional(name, max(width-len(suffix), 0), "") + suffix
			if !used[candidate] {
				labels[name], used[candidate] = candidate, true
				break
			}
		}
	}
	return labels
}

func abbreviateProjectMiddle(name string, width, head int) string {
	if lipgloss.Width(name) <= width {
		return name
	}
	tail := max(width-head-1, 0)
	end := lipgloss.Width(name)
	start := max(end-tail, 0)
	suffix := ansi.Cut(name, start, end)
	for lipgloss.Width(suffix) > tail {
		start++
		suffix = ansi.Cut(name, start, end)
	}
	return textfit.TruncateIntentional(name, head, "") + "…" + suffix
}

type projectLabelCache struct {
	key    string
	labels map[string]string
}

func (m *OutlinerModel) abbreviatedProjectSlug(slug string, width int) string {
	names := []string{slug}
	for _, project := range m.Population.Projects {
		if project == nil {
			continue
		}
		name := project.Slug
		if name == "" {
			name = project.Name
		}
		names = append(names, name)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	// Include the resolved slug in the cache key: standalone project previews
	// need not have a loaded cross-project population.
	key := fmt.Sprint(width) + "\x00" + strings.Join(names, "\x00")
	if m.projectLabels.key != key {
		m.projectLabels = projectLabelCache{key: key, labels: projectAbbreviations(names, width)}
	}
	return m.projectLabels.labels[slug]
}
