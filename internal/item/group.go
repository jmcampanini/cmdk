package item

import "charm.land/bubbles/v2/list"

var TypeOrder = []string{"action", "dir", "window"}

func GroupAndOrder(items []Item, bellToTop bool) []list.Item {
	var bellItems []Item
	buckets := make(map[string][]Item)
	var bucketOrder []string
	seenBucket := make(map[string]bool)
	for _, it := range items {
		if bellToTop && it.Data["bell"] == "1" {
			bellItems = append(bellItems, it)
			continue
		}
		bucketKey := it.Type
		if bucketKey == "loading" || bucketKey == "error" {
			if st := it.Data["source_type"]; st != "" {
				bucketKey = st
			}
		}
		if !seenBucket[bucketKey] {
			seenBucket[bucketKey] = true
			bucketOrder = append(bucketOrder, bucketKey)
		}
		buckets[bucketKey] = append(buckets[bucketKey], it)
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

	for _, bucketKey := range bucketOrder {
		if !seen[bucketKey] {
			seen[bucketKey] = true
			for _, unknown := range buckets[bucketKey] {
				result = append(result, unknown)
			}
		}
	}

	return result
}
