package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/list"
	"github.com/sahilm/fuzzy"
)

// multiTermFilter splits the search term on whitespace and requires every
// sub-term to fuzzy-match the target (AND semantics, order-independent).
// A single term (no spaces) behaves identically to list.DefaultFilter.
func multiTermFilter(term string, targets []string) []list.Rank {
	terms := strings.Fields(term)
	if len(terms) == 0 {
		ranks := make([]list.Rank, len(targets))
		for i := range targets {
			ranks[i] = list.Rank{Index: i}
		}
		return ranks
	}
	if len(terms) == 1 {
		return singleTermFilter(terms[0], targets)
	}

	type candidate struct {
		minScore       int
		matchedIndexes []int
	}

	firstMatches := fuzzy.Find(terms[0], targets)
	candidates := make(map[int]*candidate, len(firstMatches))
	for _, m := range firstMatches {
		candidates[m.Index] = &candidate{
			minScore:       m.Score,
			matchedIndexes: slices.Clone(m.MatchedIndexes),
		}
	}

	for _, t := range terms[1:] {
		matches := fuzzy.Find(t, targets)
		matched := make(map[int]fuzzy.Match, len(matches))
		for _, m := range matches {
			matched[m.Index] = m
		}
		for idx, c := range candidates {
			m, ok := matched[idx]
			if !ok {
				delete(candidates, idx)
				continue
			}
			c.matchedIndexes = append(c.matchedIndexes, m.MatchedIndexes...)
			if m.Score < c.minScore {
				c.minScore = m.Score
			}
		}
		if len(candidates) == 0 {
			return nil
		}
	}

	type scored struct {
		rank  list.Rank
		score int
	}
	results := make([]scored, 0, len(candidates))
	for idx, c := range candidates {
		results = append(results, scored{
			rank: list.Rank{
				Index:          idx,
				MatchedIndexes: dedupIndexes(c.matchedIndexes),
			},
			score: c.minScore,
		})
	}
	slices.SortStableFunc(results, func(a, b scored) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return a.rank.Index - b.rank.Index
	})

	ranks := make([]list.Rank, len(results))
	for i, r := range results {
		ranks[i] = r.rank
	}
	return ranks
}

func singleTermFilter(term string, targets []string) []list.Rank {
	matches := fuzzy.Find(term, targets)
	result := make([]list.Rank, len(matches))
	for i, r := range matches {
		result[i] = list.Rank{
			Index:          r.Index,
			MatchedIndexes: r.MatchedIndexes,
		}
	}
	return result
}

func dedupIndexes(indexes []int) []int {
	if len(indexes) == 0 {
		return nil
	}
	slices.Sort(indexes)
	return slices.Compact(indexes)
}
