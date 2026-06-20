package item

import "charm.land/bubbles/v2/list"

var TypeOrder = []string{"action", "dir", "window"}

func GroupAndOrder(items []Item, bellToTop bool) []list.Item {
	buckets := make(map[string][]Item)
	var bellItems []Item
	var unknownOrder []string
	known := knownTypes()

	for _, it := range items {
		if bellToTop && it.Type == "window" && it.Data["bell"] == "1" {
			bellItems = append(bellItems, it)
			continue
		}
		if !known[it.Type] {
			if _, ok := buckets[it.Type]; !ok {
				unknownOrder = append(unknownOrder, it.Type)
			}
		}
		buckets[it.Type] = append(buckets[it.Type], it)
	}

	result := make([]list.Item, 0, len(items))
	appendBucket := func(typ string) {
		for _, it := range buckets[typ] {
			result = append(result, it)
		}
	}

	appendBucket("error")
	appendBucket("loading")
	for _, it := range bellItems {
		result = append(result, it)
	}
	for _, typ := range TypeOrder {
		appendBucket(typ)
	}
	for _, typ := range unknownOrder {
		appendBucket(typ)
	}

	return result
}

func knownTypes() map[string]bool {
	known := make(map[string]bool, len(TypeOrder)+2)
	known["error"] = true
	known["loading"] = true
	for _, typ := range TypeOrder {
		known[typ] = true
	}
	return known
}
