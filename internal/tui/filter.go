package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/list"
)

type scoredRank struct {
	rank  list.Rank
	score int
}

func ranksByScore(results []scoredRank) []list.Rank {
	slices.SortStableFunc(results, func(a, b scoredRank) int {
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

// multiTermFilter splits the search term on whitespace and requires every
// sub-term to fuzzy-match the target (AND semantics, order-independent).
func multiTermFilter(term string, targets []string) []list.Rank {
	terms := strings.Fields(term)
	slices.Sort(terms)
	terms = slices.Compact(terms)
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
		sumScore       int
		matchedIndexes []int
	}

	candidates := make(map[int]*candidate, len(targets))
	for i, t := range targets {
		r := FuzzyMatch(false, t, terms[0])
		if r.Score > 0 {
			candidates[i] = &candidate{
				sumScore:       r.Score,
				matchedIndexes: r.Positions,
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	for _, term := range terms[1:] {
		for idx, c := range candidates {
			r := FuzzyMatch(false, targets[idx], term)
			if r.Score == 0 {
				delete(candidates, idx)
				continue
			}
			c.sumScore += r.Score
			c.matchedIndexes = append(c.matchedIndexes, r.Positions...)
		}
		if len(candidates) == 0 {
			return nil
		}
	}

	results := make([]scoredRank, 0, len(candidates))
	for idx, c := range candidates {
		results = append(results, scoredRank{
			rank: list.Rank{
				Index:          idx,
				MatchedIndexes: dedupIndexes(c.matchedIndexes),
			},
			score: c.sumScore,
		})
	}
	return ranksByScore(results)
}

func singleTermFilter(term string, targets []string) []list.Rank {
	var results []scoredRank
	for i, t := range targets {
		r := FuzzyMatch(false, t, term)
		if r.Score > 0 {
			results = append(results, scoredRank{
				rank: list.Rank{
					Index:          i,
					MatchedIndexes: r.Positions,
				},
				score: r.Score,
			})
		}
	}
	return ranksByScore(results)
}

func dedupIndexes(indexes []int) []int {
	if len(indexes) == 0 {
		return nil
	}
	slices.Sort(indexes)
	return slices.Compact(indexes)
}
