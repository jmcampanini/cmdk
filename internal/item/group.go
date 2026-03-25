package item

import "charm.land/bubbles/v2/list"

var TypeOrder = []string{"action", "cmd", "dir", "window"}

func GroupAndOrder(items []Item, bellToTop bool) []list.Item {
	var bellItems []Item
	buckets := make(map[string][]Item)
	for _, it := range items {
		if bellToTop && it.Data["bell"] == "1" {
			bellItems = append(bellItems, it)
			continue
		}
		buckets[it.Type] = append(buckets[it.Type], it)
	}

	result := make([]list.Item, 0, len(items))
	for _, it := range bellItems {
		result = append(result, it)
	}

	seen := make(map[string]bool, len(TypeOrder))
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
