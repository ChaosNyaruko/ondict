package sources

import (
	"sort"
	"strings"
	"unicode"
)

type CompletionMode string

const (
	CompletionPrefix CompletionMode = "prefix"
	CompletionFuzzy  CompletionMode = "fzf"
)

func ParseCompletionMode(mode string) CompletionMode {
	switch strings.ToLower(mode) {
	case "fzf", "fuzzy":
		return CompletionFuzzy
	default:
		return CompletionPrefix
	}
}

func Complete(query string, mode CompletionMode, limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	if query == "" {
		return nil
	}

	if len(*G) == 1 && (*G)[0].MdxFile == "vocab.db" {
		if dict, ok := (*G)[0].MdxDict.(*DBDict); ok {
			return dict.Complete(query, mode, limit)
		}
	}

	return rankCompletions(allWords(), query, mode, limit)
}

func allWords() []string {
	var words []string
	for _, g := range *G {
		words = append(words, g.MdxDict.Keys()...)
	}
	return words
}

func (d *DBDict) Complete(query string, mode CompletionMode, limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	if mode == CompletionPrefix {
		words := d.WordsWithPrefix(query)
		if len(words) > limit {
			return words[:limit]
		}
		return words
	}
	return rankCompletions(d.Keys(), query, mode, limit)
}

type scoredWord struct {
	word  string
	score int
}

func rankCompletions(words []string, query string, mode CompletionMode, limit int) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	lquery := strings.ToLower(query)
	seen := make(map[string]struct{}, len(words))
	matches := make([]scoredWord, 0, limit)

	for _, word := range words {
		lword := strings.ToLower(word)
		if _, ok := seen[lword]; ok {
			continue
		}
		seen[lword] = struct{}{}

		switch mode {
		case CompletionFuzzy:
			score, ok := fuzzyMatchScore(word, query)
			if ok {
				matches = append(matches, scoredWord{word: word, score: score})
			}
		default:
			if strings.HasPrefix(lword, lquery) {
				matches = append(matches, scoredWord{word: word, score: prefixScore(word, query)})
			}
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if len(matches[i].word) != len(matches[j].word) {
			return len(matches[i].word) < len(matches[j].word)
		}
		return strings.ToLower(matches[i].word) < strings.ToLower(matches[j].word)
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	res := make([]string, 0, len(matches))
	for _, match := range matches {
		res = append(res, match.word)
	}
	return res
}

func prefixScore(word string, query string) int {
	score := 1000
	score -= len(word)
	if strings.EqualFold(word, query) {
		score += 500
	}
	return score
}

// fuzzyMatchScore implements a lightweight fzf-like subsequence matcher.
//
// A candidate matches when every rune in query appears in order inside word.
// After that, we rank the candidate with a few simple heuristics:
//   - exact prefix match gets the largest bonus
//   - earlier match start ranks higher
//   - tighter match span ranks higher
//   - consecutive matched runes rank higher
//   - word-boundary / camelCase hits rank higher
//   - longer candidates receive a small penalty
//
// This is intentionally much simpler than upstream fzf's matcher, but keeps
// the main properties that make completion feel good in practice.
func fuzzyMatchScore(word string, query string) (int, bool) {
	pattern := []rune(strings.ToLower(strings.TrimSpace(query)))
	candidate := []rune(strings.ToLower(word))
	if len(pattern) == 0 {
		return 0, false
	}

	// Greedily record the earliest ordered subsequence match positions.
	pos := make([]int, 0, len(pattern))
	pi := 0
	for i, r := range candidate {
		if r == pattern[pi] {
			pos = append(pos, i)
			pi++
			if pi == len(pattern) {
				break
			}
		}
	}
	if pi != len(pattern) {
		return 0, false
	}

	score := 0
	// Matches starting at the beginning should usually win.
	if pos[0] == 0 {
		score += 80
	}
	// Exact prefix is stronger than a generic early subsequence hit.
	if strings.HasPrefix(strings.ToLower(word), string(pattern)) {
		score += 120
	}

	span := pos[len(pos)-1] - pos[0] + 1
	// Prefer compact matches over scattered ones.
	score += max(0, 120-(span-len(pattern))*8)
	// Earlier first hit is better.
	score += max(0, 80-pos[0]*4)

	for i, p := range pos {
		// Reward adjacent hits like "app" in "application".
		if i > 0 && p == pos[i-1]+1 {
			score += 18
		}
		// Reward natural token starts: spaces, punctuation, or camelCase edges.
		if isWordBoundary([]rune(word), p) {
			score += 12
		}
	}

	// All else being equal, shorter candidates are usually better completions.
	score -= len(candidate)
	return score, true
}

func isWordBoundary(word []rune, idx int) bool {
	if idx <= 0 {
		return true
	}
	prev := word[idx-1]
	cur := word[idx]
	if prev == ' ' || prev == '-' || prev == '_' || prev == '/' || prev == '.' {
		return true
	}
	return unicode.IsLower(prev) && unicode.IsUpper(cur)
}
