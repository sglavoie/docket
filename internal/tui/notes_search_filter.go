package tui

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// filterNotes filters notes based on the current search query.
func (m *NotesSearchModel) filterNotes() {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		m.filtered = make([]notesSearchResult, len(m.notes))
		for index, note := range m.notes {
			m.filtered[index] = notesSearchResult{note: note, score: 1}
		}
		m.cursor = ClampCursor(m.cursor, len(m.filtered))
		return
	}

	var scored []notesSearchResult
	for _, note := range m.notes {
		titleScore, titlePositions := fuzzy.ScoreTermsWithPositions(note.Title, query)
		if titleScore > 0 {
			titleScore *= 2
		}

		bodyScore := scoreBodyTerms(note.Body, query)
		tagScore := bestTagScore(note.Tags, query)
		bestScore := max(tagScore, max(bodyScore, titleScore))
		if bestScore == 0 {
			continue
		}

		snippet := ""
		var bodyPositions []int
		if bodyScore > 0 {
			snippet, bodyPositions = extractSnippets(note.Body, query)
		}

		if titleScore <= 0 {
			titlePositions = nil
		}

		scored = append(scored, notesSearchResult{
			note:           note,
			score:          bestScore,
			titlePositions: titlePositions,
			bodySnippet:    snippet,
			bodyPositions:  bodyPositions,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	m.filtered = scored
	m.cursor = ClampCursor(m.cursor, len(m.filtered))
}

func bestTagScore(tags []string, query string) int {
	best := 0
	for _, tag := range tags {
		score := fuzzy.ScoreTerms(tag, query)
		if score > best {
			best = score
		}
	}
	return best
}

// scoreBodyTerms scores body text against a multi-term query using substring
// matching. The fuzzy scorer can reject valid body matches because its
// "too scattered" heuristic fires when early characters in a long text match
// before the actual contiguous occurrence. This function checks for case-
// insensitive substring containment of each term, requiring all terms to match.
func scoreBodyTerms(body, query string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		return 1
	}

	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return 1
	}

	lowerBody := strings.ToLower(body)
	total := 0
	for _, term := range terms {
		if strings.Contains(lowerBody, term) {
			total += len(term) * 4
			continue
		}

		score := fuzzy.Score(body, term)
		if score == 0 {
			return 0
		}
		total += score
	}

	return total
}

// extractSnippets extracts up to 3 snippet windows (one per distinct term match)
// and returns the joined snippet text along with rune-based highlight positions.
func extractSnippets(body, query string) (string, []int) {
	lowerBody := strings.ToLower(body)
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return "", nil
	}

	type window struct {
		start int
		end   int
	}

	seen := make(map[int]bool)
	var windows []window
	for _, term := range terms {
		index := strings.Index(lowerBody, term)
		if index < 0 || seen[index] {
			continue
		}

		seen[index] = true
		windows = append(windows, window{
			start: max(index-30, 0),
			end:   min(index+len(term)+50, len(body)),
		})
		if len(windows) >= 3 {
			break
		}
	}

	if len(windows) == 0 {
		if len(body) > 120 {
			return body[:120] + "...", nil
		}
		return body, nil
	}

	sort.Slice(windows, func(i, j int) bool {
		return windows[i].start < windows[j].start
	})

	merged := []window{windows[0]}
	for index := 1; index < len(windows); index++ {
		last := &merged[len(merged)-1]
		if windows[index].start <= last.end {
			if windows[index].end > last.end {
				last.end = windows[index].end
			}
			continue
		}
		merged = append(merged, windows[index])
	}

	var result strings.Builder
	var positions []int
	for index, currentWindow := range merged {
		if index > 0 {
			result.WriteString(" … ")
		}

		segment := body[currentWindow.start:currentWindow.end]
		prefix := ""
		suffix := ""
		if currentWindow.start > 0 {
			prefix = "..."
		}
		if currentWindow.end < len(body) {
			suffix = "..."
		}

		segmentStart := result.Len() + len(prefix)
		result.WriteString(prefix)
		result.WriteString(segment)
		result.WriteString(suffix)

		lowerSegment := strings.ToLower(segment)
		for _, term := range terms {
			offset := 0
			for {
				matchIndex := strings.Index(lowerSegment[offset:], term)
				if matchIndex < 0 {
					break
				}

				absoluteOffset := segmentStart + offset + matchIndex
				runeStart := len([]rune(result.String()[:absoluteOffset]))
				for runeIndex := range []rune(term) {
					positions = append(positions, runeStart+runeIndex)
				}
				offset += matchIndex + len(term)
			}
		}
	}

	return result.String(), positions
}
