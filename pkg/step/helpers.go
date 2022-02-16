package step

import (
	"encoding/base64"
	"fmt"
)

// GetObjAtPath returns the subtree at keys path in data.
func getObjAtPath(data map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	for _, k := range keys {
		v, ok := data[k]
		if !ok {
			return nil, fmt.Errorf("missing: %s", k)
		}
		data, ok = v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map: %s", k)
		}
	}
	return data, nil
}

// Get returns a string value at m[k]
func get(m map[string]interface{}, k string) (string, error) {
	v, ok := m[k]
	if !ok {
		return "", fmt.Errorf("missing: %s", k)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected string", k)
	}
	return s, nil
}

// Get returns a base 64 encoded []byte value at m[k]
func get64(m map[string]interface{}, k string) ([]byte, error) {
	s, err := get(m, k)
	if err != nil {
		return []byte{}, err
	}
	d, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return []byte{}, fmt.Errorf("%s: %v", k, err)
	}
	return d, nil
}
