package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/storage"
)

// buildGlobalTagList aggregates tags from all active projects.
func (m *GlobalTagManagerModel) buildGlobalTagList() {
	tagMap := make(map[string]*GlobalTagInfo)

	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		projectTagCounts := project.TagCounts()

		// Merge into global map
		for tag, count := range projectTagCounts {
			if info, exists := tagMap[tag]; exists {
				info.TaskCount += count
				info.Projects = append(info.Projects, TagProjectUsage{
					Project:   project,
					TaskCount: count,
				})
			} else {
				tagMap[tag] = &GlobalTagInfo{
					Name:      tag,
					TaskCount: count,
					Projects: []TagProjectUsage{{
						Project:   project,
						TaskCount: count,
					}},
				}
			}
		}
	}

	// Convert map to sorted slice
	m.tags = make([]GlobalTagInfo, 0, len(tagMap))
	for _, info := range tagMap {
		m.tags = append(m.tags, *info)
	}

	sort.Slice(m.tags, func(i, j int) bool {
		return m.tags[i].Name < m.tags[j].Name
	})

	m.cursor = ClampCursor(m.cursor, len(m.tags))
}
