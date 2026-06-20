package item

import "charm.land/bubbles/v2/list"

var (
	TypeOrder       = []string{"action", "dir", "window"}
	statusTypeOrder = []string{"error", "loading"}
)

func GroupAndOrder(items []Item, bellToTop bool) []list.Item {
	buckets := make(map[string][]Item)
	var bellWindows []Item
	var unknownTypes []string

	for _, it := range items {
		if bellToTop && isBellWindow(it) {
			bellWindows = append(bellWindows, it)
			continue
		}
		if !isKnownType(it.Type) {
			if _, ok := buckets[it.Type]; !ok {
				unknownTypes = append(unknownTypes, it.Type)
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

	for _, typ := range statusTypeOrder {
		appendBucket(typ)
	}
	for _, it := range bellWindows {
		result = append(result, it)
	}
	for _, typ := range TypeOrder {
		appendBucket(typ)
	}
	for _, typ := range unknownTypes {
		appendBucket(typ)
	}

	return result
}

func isBellWindow(it Item) bool {
	return it.Type == "window" && it.Data["bell"] == "1"
}

func isKnownType(typ string) bool {
	for _, knownType := range statusTypeOrder {
		if typ == knownType {
			return true
		}
	}
	for _, knownType := range TypeOrder {
		if typ == knownType {
			return true
		}
	}
	return false
}
