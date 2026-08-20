// Package keys moves modeled fields in and out of raw JSON payloads.
package keys

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Take decodes and removes a valid, non-null key. Invalid values stay untouched
// so they can round-trip through preservation.
func Take[T any](source map[string]json.RawMessage, key string, target *T) bool {
	raw, present := source[key]
	if !present || IsNull(raw) || json.Unmarshal(raw, target) != nil {
		return false
	}
	delete(source, key)
	return true
}

// IsNull reports whether a raw value is JSON's own word for nothing.
func IsNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// WriteIfSet writes a key only where the writer has something to say with it.
func WriteIfSet(target map[string]json.RawMessage, key string, set bool, value any) {
	if set {
		target[key] = Must(value)
	}
}

// MergeAbsent adds the keys a target does not already carry. Content the
// writer already wrote wins, because a preserved copy of something the creator
// can now edit would put a stale value back on top of a live one.
func MergeAbsent(target, source map[string]json.RawMessage) {
	for key, value := range source {
		if _, written := target[key]; !written {
			target[key] = value
		}
	}
}

// Object reads an object out of raw JSON, giving back an empty one where there
// is nothing to read.
func Object(raw json.RawMessage) map[string]json.RawMessage {
	object := make(map[string]json.RawMessage)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &object)
	}
	return object
}

// Must encodes a value a writer built itself. Every call site passes a string,
// a bool, a number, or a list or map of those, none of which can fail.
func Must(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("write a JSON value: %v", err))
	}
	return encoded
}
