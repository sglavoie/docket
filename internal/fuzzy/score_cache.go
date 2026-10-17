package fuzzy

import (
	"container/list"
	"strings"
	"sync"
	"unicode/utf8"
)

// ScoreCache bounds retained text, decoded runes and recent scores. Text content
// is the identity, so a changed description/comment cannot reuse its old score.
// Each owner chooses a budget; there is no process-wide retained search data.
// The mutex also makes Terms.WithCache safe for concurrent readers.
type ScoreCache struct {
	mu           sync.Mutex
	budget, used int
	preparedUsed int
	entries      map[string]*list.Element
	recent       list.List
	joined       map[joinedTextKey]*list.Element
	joinedRecent list.List
	joinedUsed   int
	joinedBudget int
}

type cachedScore struct {
	query     string
	sensitive bool
	score     int
}

type cachedText struct {
	text                            string
	original, folded                []rune
	originalASCII, foldedASCII      [2]uint64
	scores                          [8]cachedScore
	count, next, cost, preparedCost int
}

func NewScoreCache(bytes int) *ScoreCache {
	bytes = max(0, bytes)
	return &ScoreCache{budget: bytes - bytes/4, entries: make(map[string]*list.Element),
		joinedBudget: bytes / 4, joined: make(map[joinedTextKey]*list.Element)}
}

// Most compound fields are a handful of comments or checklist items. Their
// exact components are comparable keys: edits invalidate them without relying
// on timestamps, pointer identity, or a collision-prone digest.
type joinedTextKey struct {
	parts [4]string
	count int
}

type joinedText struct {
	key  joinedTextKey
	text string
	cost int
}

func (c *ScoreCache) join(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) > 4 {
		return strings.Join(parts, " ")
	}
	key := joinedTextKey{count: len(parts)}
	copy(key.parts[:], parts)
	cost := 256 + 2*(len(parts)-1)
	for _, part := range parts {
		cost += 2 * len(part)
	}
	if cost > c.joinedBudget {
		return strings.Join(parts, " ")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry := c.joined[key]; entry != nil {
		c.joinedRecent.MoveToFront(entry)
		return entry.Value.(*joinedText).text
	}
	for c.joinedUsed+cost > c.joinedBudget {
		old := c.joinedRecent.Back()
		value := old.Value.(*joinedText)
		delete(c.joined, value.key)
		c.joinedUsed -= value.cost
		c.joinedRecent.Remove(old)
	}
	for i := range parts {
		key.parts[i] = strings.Clone(parts[i])
	}
	value := &joinedText{key: key, text: strings.Join(key.parts[:key.count], " "), cost: cost}
	c.joined[key] = c.joinedRecent.PushFront(value)
	c.joinedUsed += cost
	return value.text
}

func (c *ScoreCache) score(q Terms, text string) int {
	// Include worst-case query storage and map/list overhead in the bound.
	cost := len(text) + 8*(64+32) + 256
	if cost > c.budget {
		return q.scoreUncached(text)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	element := c.entries[text]
	if element == nil {
		for c.used+cost > c.budget {
			old := c.recent.Back()
			entry := old.Value.(*cachedText)
			delete(c.entries, entry.text)
			c.used -= entry.cost
			c.preparedUsed -= entry.preparedCost
			c.recent.Remove(old)
		}
		entry := &cachedText{text: strings.Clone(text), cost: cost}
		for _, r := range text {
			if r < 128 {
				entry.originalASCII[r/64] |= 1 << (r % 64)
			}
			r = foldRune(r)
			if r < 128 {
				entry.foldedASCII[r/64] |= 1 << (r % 64)
			}
		}
		element = c.recent.PushFront(entry)
		c.entries[entry.text] = element
		c.used += cost
	} else {
		c.recent.MoveToFront(element)
	}
	entry := element.Value.(*cachedText)
	for _, result := range entry.scores[:entry.count] {
		if result.query == q.query && result.sensitive == q.matcher.caseSensitive {
			return result.score
		}
	}
	present := entry.foldedASCII
	if q.matcher.caseSensitive {
		present = entry.originalASCII
	}
	score := 0
	if containsRequiredASCII(q, present) {
		// Decode only candidates that can match. Keep decoded buffers for
		// reused text only, and cap them at half of the score partition so
		// a large no-match population cannot evict all of its cheap scores.
		if entry.original == nil && entry.count > 0 {
			preparedCost := 8 * utf8.RuneCountInString(text)
			if c.preparedUsed+preparedCost <= c.budget/2 && c.used+preparedCost <= c.budget {
				entry.original = []rune(text)
				entry.folded = make([]rune, len(entry.original))
				for i, r := range entry.original {
					entry.folded[i] = foldRune(r)
				}
				entry.preparedCost = preparedCost
				entry.cost += preparedCost
				c.preparedUsed += preparedCost
				c.used += preparedCost
			}
		}
		if entry.original == nil {
			score = q.scoreUncached(text)
		} else {
			runes := entry.folded
			if q.matcher.caseSensitive {
				runes = entry.original
			}
			score = scoreCachedTerms(q, entry.original, runes)
		}
	}
	// An unusually long pasted query must not escape the retention budget.
	if len(q.query) <= 64 {
		entry.scores[entry.next] = cachedScore{strings.Clone(q.query), q.matcher.caseSensitive, score}
		entry.next = (entry.next + 1) % len(entry.scores)
		entry.count = min(entry.count+1, len(entry.scores))
	}
	return score
}

func containsRequiredASCII(q Terms, present [2]uint64) bool {
	for _, pattern := range q.patterns {
		// Presence is a necessary condition only. Never prune by a previous
		// fuzzy miss: a longer pattern also widens the permitted density window.
		for _, r := range pattern {
			if r < 128 && present[r/64]&(1<<(r%64)) == 0 {
				return false
			}
		}
	}
	return true
}

func scoreCachedTerms(q Terms, original, folded []rune) int {
	total := 0
	for _, pattern := range q.patterns {
		score := scoreSubsequence(folded, original, pattern)
		if score == 0 {
			return 0
		}
		total += score
	}
	return total
}
