package item

import "charm.land/bubbles/v2/list"

var TypeOrder = []string{"action", "dir", "window"}

func GroupAndOrder(items []Item, bellToTop bool) []list.Item {
	var bellItems []Item
	buckets := make(map[string][]Item)
	var bucketOrder []string
	for _, it := range items {
		if bellToTop && it.Data["bell"] == "1" {
			bellItems = append(bellItems, it)
			continue
		}
		bucketKey := orderBucketKey(it)
		if _, ok := buckets[bucketKey]; !ok {
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

func orderBucketKey(it Item) string {
	switch it.Type {
	case "loading", "error":
		if sourceType := it.Data["source_type"]; sourceType != "" {
			return sourceType
		}
	}
	return it.Type
}
