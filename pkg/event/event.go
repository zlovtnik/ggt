package event

import (
	"strings"
	"time"
)

type Event struct {
	Payload   map[string]interface{}
	Metadata  Metadata
	Timestamp time.Time
	Headers   map[string]string
}

type Metadata struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
}

func (e Event) Clone() Event {
	return Event{
		Payload:   deepCopyMap(e.Payload),
		Metadata:  cloneMetadata(e.Metadata),
		Timestamp: e.Timestamp,
		Headers:   cloneStringMap(e.Headers),
	}
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}

	return result
}

func cloneMetadata(meta Metadata) Metadata {
	metadataCopy := meta
	if meta.Key != nil {
		keyCopy := make([]byte, len(meta.Key))
		copy(keyCopy, meta.Key)
		metadataCopy.Key = keyCopy
	} else {
		metadataCopy.Key = nil
	}

	return metadataCopy
}

func cloneMapShallow(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}

	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}

	return result
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = deepCopyMap(val)
		case []interface{}:
			result[k] = deepCopySlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

func deepCopySlice(s []interface{}) []interface{} {
	if s == nil {
		return nil
	}
	result := make([]interface{}, len(s))
	for i, item := range s {
		switch val := item.(type) {
		case map[string]interface{}:
			result[i] = deepCopyMap(val)
		case []interface{}:
			result[i] = deepCopySlice(val)
		default:
			result[i] = item
		}
	}
	return result
}

func (e Event) GetField(path string) (interface{}, bool) {
	if path == "" || e.Payload == nil {
		return nil, false
	}

	current := e.Payload
	keys := strings.Split(path, ".")

	for _, key := range keys[:len(keys)-1] {
		if nested, ok := current[key].(map[string]interface{}); ok {
			current = nested
		} else {
			return nil, false
		}
	}

	if value, ok := current[keys[len(keys)-1]]; ok {
		return value, true
	}

	return nil, false
}

func (e Event) SetField(path string, value interface{}) Event {
	if path == "" {
		return e
	}

	keys := strings.Split(path, ".")
	if len(keys) == 0 {
		return e
	}

	result := Event{
		Payload:   cloneMapShallow(e.Payload),
		Metadata:  cloneMetadata(e.Metadata),
		Timestamp: e.Timestamp,
		Headers:   cloneStringMap(e.Headers),
	}

	if result.Payload == nil {
		result.Payload = make(map[string]interface{})
	}

	currentNew := result.Payload
	var currentOrig map[string]interface{}
	if e.Payload != nil {
		currentOrig = e.Payload
	}

	for i, key := range keys {
		if i == len(keys)-1 {
			currentNew[key] = value
			break
		}

		var origNext map[string]interface{}
		if currentOrig != nil {
			if val, ok := currentOrig[key]; ok {
				if typed, ok := val.(map[string]interface{}); ok {
					origNext = typed
				}
			}
		}

		nextMap := cloneMapShallow(origNext)
		if nextMap == nil {
			nextMap = make(map[string]interface{})
		}

		currentNew[key] = nextMap
		currentNew = nextMap
		currentOrig = origNext
	}

	return result
}

func (e Event) RemoveField(path string) Event {
	if path == "" || e.Payload == nil {
		return e
	}

	keys := strings.Split(path, ".")
	if len(keys) == 0 {
		return e
	}

	// Verify the field exists and path traversal is valid before allocating new structures.
	currentOrig := e.Payload
	for i, key := range keys {
		val, ok := currentOrig[key]
		if !ok {
			return e
		}

		if i == len(keys)-1 {
			break
		}

		nested, ok := val.(map[string]interface{})
		if !ok {
			return e
		}
		currentOrig = nested
	}

	result := Event{
		Payload:   cloneMapShallow(e.Payload),
		Metadata:  cloneMetadata(e.Metadata),
		Timestamp: e.Timestamp,
		Headers:   cloneStringMap(e.Headers),
	}

	currentNew := result.Payload
	currentOrig = e.Payload

	for i, key := range keys {
		if i == len(keys)-1 {
			delete(currentNew, key)
			break
		}

		origVal, _ := currentOrig[key]
		origMap, _ := origVal.(map[string]interface{})

		clonedNext := cloneMapShallow(origMap)
		if clonedNext == nil {
			clonedNext = make(map[string]interface{})
		}

		currentNew[key] = clonedNext
		currentNew = clonedNext
		currentOrig = origMap
	}

	return result
}
