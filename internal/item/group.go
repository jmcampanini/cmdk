package item

import "charm.land/bubbles/v2/list"

var TypeOrder = []string{"cmd", "dir", "window"}

func GroupAndOrder(items []Item) []list.Item {
	buckets := make(map[string][]Item)
	for _, it := range items {
		buckets[it.Type] = append(buckets[it.Type], it)
	}

	var result []list.Item
	seen := make(map[string]bool)

	for _, typ := range TypeOrder {
		seen[typ] = true
		for _, it := range buckets[typ] {
			result = append(result, it)
		}
	}

	for _, it := range items {
		if !seen[it.Type] {
			seen[it.Type] = true
			for _, unknown := range buckets[it.Type] {
				result = append(result, unknown)
			}
		}
	}

	return result
}
