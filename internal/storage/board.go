package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const boardDirName = "board"

// boardIndex tracks the next short ID for board items.
type boardIndex struct {
	NextShortID int `yaml:"next_short_id"`
}

// BoardData holds loaded board items and index metadata.
type BoardData struct {
	Items       []*models.BoardItem
	NextShortID int
	dataDir     string
}

// BoardDir returns the path to the board directory.
func BoardDir(dataDir string) string {
	return filepath.Join(dataDir, ".tt", boardDirName)
}

// boardIndexPath returns the path to the board index file.
func boardIndexPath(dataDir string) string {
	return filepath.Join(BoardDir(dataDir), "board.yaml")
}

// LoadBoard reads all board items from disk.
func LoadBoard(dataDir string) (*BoardData, error) {
	dir := BoardDir(dataDir)

	bd := &BoardData{
		NextShortID: 1,
		dataDir:     dataDir,
	}

	// Load index
	idxPath := boardIndexPath(dataDir)
	idxData, err := os.ReadFile(idxPath)
	if err == nil {
		var idx boardIndex
		if parseErr := yaml.Unmarshal(idxData, &idx); parseErr == nil && idx.NextShortID > 0 {
			bd.NextShortID = idx.NextShortID
		}
	}

	// Load items
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return bd, nil
	}
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		item, err := loadBoardItem(dir, entry.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "tt: skipping unparseable board item %s: %v\n", entry.Name(), err)
			continue
		}
		bd.Items = append(bd.Items, item)
	}

	// Sort by priority (ascending), then by creation date (newest first)
	sort.Slice(bd.Items, func(i, j int) bool {
		if bd.Items[i].Priority != bd.Items[j].Priority {
			return bd.Items[i].Priority < bd.Items[j].Priority
		}
		return bd.Items[i].CreatedAt.After(bd.Items[j].CreatedAt)
	})

	return bd, nil
}

// SaveBoardItem writes a board item to disk and updates the index.
func SaveBoardItem(dataDir string, item *models.BoardItem) error {
	dir := BoardDir(dataDir)
	if err := ensureDirPath(dir); err != nil {
		return err
	}

	if item.Filename == "" {
		item.Filename = models.GenerateBoardItemFilename(item.ShortID, item.Title)
	}

	content, err := renderBoardItem(item)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, item.Filename)
	return util.AtomicWriteFile(path, []byte(content), FilePermissions)
}

// SaveBoardIndex writes the board index file.
func SaveBoardIndex(dataDir string, nextShortID int) error {
	dir := BoardDir(dataDir)
	if err := ensureDirPath(dir); err != nil {
		return err
	}

	idx := boardIndex{NextShortID: nextShortID}
	data, err := yaml.Marshal(&idx)
	if err != nil {
		return fmt.Errorf("failed to marshal board index: %w", err)
	}

	return util.AtomicWriteFile(boardIndexPath(dataDir), data, FilePermissions)
}

// DeleteBoardItem removes a board item file from disk.
func DeleteBoardItem(dataDir string, item *models.BoardItem) error {
	path := filepath.Join(BoardDir(dataDir), item.Filename)
	return os.Remove(path)
}

// FindBoardItem looks up a board item by short ID or UUID prefix.
func FindBoardItem(bd *BoardData, query string) *models.BoardItem {
	for _, item := range bd.Items {
		if fmt.Sprintf("%d", item.ShortID) == query {
			return item
		}
		if strings.HasPrefix(item.ID, query) {
			return item
		}
	}
	return nil
}

// FindBoardItemsByTaskID returns all board items linked to the given task ID.
func FindBoardItemsByTaskID(dataDir, taskID string) ([]*models.BoardItem, error) {
	bd, err := LoadBoard(dataDir)
	if err != nil {
		return nil, err
	}
	var matched []*models.BoardItem
	for _, item := range bd.Items {
		if item.LinkedTaskID == taskID {
			matched = append(matched, item)
		}
	}
	return matched, nil
}

// boardFrontmatter holds the YAML frontmatter fields for board items.
type boardFrontmatter struct {
	ID           string                 `yaml:"id"`
	ShortID      int                    `yaml:"short_id"`
	Title        string                 `yaml:"title"`
	URL          string                 `yaml:"url,omitempty"`
	Source       string                 `yaml:"source,omitempty"`
	Priority     int                    `yaml:"priority"`
	Status       models.BoardItemStatus `yaml:"status"`
	Tags         []string               `yaml:"tags,omitempty"`
	LinkedTaskID string                 `yaml:"linked_task_id,omitempty"`
	ProjectID    string                 `yaml:"project_id,omitempty"`
	ProjectName  string                 `yaml:"project_name,omitempty"`
	CreatedAt    string                 `yaml:"created_at"`
	UpdatedAt    string                 `yaml:"updated_at"`
	DoneAt       string                 `yaml:"done_at,omitempty"`
}

// loadBoardItem reads a single board item from its file.
func loadBoardItem(dir, filename string) (*models.BoardItem, error) {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseBoardItem(filename, string(data))
}

// parseBoardItem parses a markdown file with YAML frontmatter into a BoardItem.
func parseBoardItem(filename, content string) (*models.BoardItem, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("invalid frontmatter in %s: %w", filename, err)
	}

	var meta boardFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("invalid YAML in %s: %w", filename, err)
	}

	item := &models.BoardItem{
		ID:           meta.ID,
		ShortID:      meta.ShortID,
		Title:        meta.Title,
		URL:          meta.URL,
		Source:       meta.Source,
		Priority:     meta.Priority,
		Status:       meta.Status,
		Tags:         meta.Tags,
		LinkedTaskID: meta.LinkedTaskID,
		ProjectID:    meta.ProjectID,
		ProjectName:  meta.ProjectName,
		Filename:     filename,
		Body:         body,
	}

	if item.Tags == nil {
		item.Tags = []string{}
	}
	if item.Status == "" {
		item.Status = models.BoardItemOpen
	}
	if item.Priority == 0 {
		item.Priority = models.BoardItemPriorityDefault
	}

	if t, err := parseTime(meta.CreatedAt); err == nil {
		item.CreatedAt = t
	}
	if t, err := parseTime(meta.UpdatedAt); err == nil {
		item.UpdatedAt = t
	}
	if meta.DoneAt != "" {
		if t, err := parseTime(meta.DoneAt); err == nil {
			item.DoneAt = &t
		}
	}

	return item, nil
}

// renderBoardItem serializes a BoardItem to markdown with YAML frontmatter.
func renderBoardItem(item *models.BoardItem) (string, error) {
	meta := boardFrontmatter{
		ID:           item.ID,
		ShortID:      item.ShortID,
		Title:        item.Title,
		URL:          item.URL,
		Source:       item.Source,
		Priority:     item.Priority,
		Status:       item.Status,
		Tags:         item.Tags,
		LinkedTaskID: item.LinkedTaskID,
		ProjectID:    item.ProjectID,
		ProjectName:  item.ProjectName,
		CreatedAt:    item.CreatedAt.Format(timeFormat),
		UpdatedAt:    item.UpdatedAt.Format(timeFormat),
	}
	if item.DoneAt != nil {
		meta.DoneAt = item.DoneAt.Format(timeFormat)
	}

	yamlBytes, err := yaml.Marshal(&meta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal board item frontmatter: %w", err)
	}
	fm := strings.TrimSpace(string(yamlBytes))

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fm)
	b.WriteString("\n---\n")
	if item.Body != "" {
		b.WriteString("\n")
		b.WriteString(item.Body)
		if !strings.HasSuffix(item.Body, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// RenderBoardItemContent serializes a BoardItem to markdown with YAML frontmatter (exported for editing).
func RenderBoardItemContent(item *models.BoardItem) (string, error) {
	return renderBoardItem(item)
}

// ParseBoardItemContent parses a markdown string with YAML frontmatter into a BoardItem.
func ParseBoardItemContent(filename, content string) (*models.BoardItem, error) {
	return parseBoardItem(filename, content)
}
