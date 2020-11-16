package util

import "strings"

// KVSliceToMap takes a slice of k=v strings and returns a map[k]v
func KVSliceToMap(in []string) map[string]string {
	out := make(map[string]string, len(in))
	for _, v := range in {
		x := strings.Split(v, "=")
		out[x[0]] = x[1]
	}
	return out
}
