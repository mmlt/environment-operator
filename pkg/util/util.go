package util

import "strings"

// KVSliceToMap takes a slice of k=v strings and returns a map[k]v
func KVSliceToMap(in []string) map[string]string {
	out := make(map[string]string, len(in))
	for _, v := range in {
		x := strings.SplitN(v, "=", 2)
		out[x[0]] = x[1]
	}
	return out
}

// KVSliceMergeMap takes a slice of k=v strings and a map, merges them and returns k=v slice.
func KVSliceMergeMap(base []string, extra map[string]string) []string {
	m := KVSliceToMap(base)

	for k, v := range extra {
		m[k] = v
	}

	return KVSliceFromMap(m)
}

// KVSliceFromMap takes a map and returns a k=v slice.
func KVSliceFromMap(in map[string]string) []string {
	r := make([]string, 0, len(in))
	for k, v := range in {
		r = append(r, k+"="+v)
	}
	return r
}
