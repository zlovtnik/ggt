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
	payloadCopy := deepCopyMap(e.Payload)

	headersCopy := make(map[string]string, len(e.Headers))
	for k, v := range e.Headers {
		headersCopy[k] = v
	}

	keyCopy := make([]byte, len(e.Metadata.Key))
	copy(keyCopy, e.Metadata.Key)

	metadataCopy := e.Metadata
	metadataCopy.Key = keyCopy

	return Event{
		Payload:   payloadCopy,
		Metadata:  metadataCopy,
		Timestamp: e.Timestamp,
		Headers:   headersCopy,
	}
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			result[k] = deepCopyMap(nested)
		} else {
			result[k] = v
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

	for i, key := range keys {
		value, ok := current[key]
		if !ok {
			return nil, false
		}

		if i == len(keys)-1 {
			return value, true
		}

		nested, ok := value.(map[string]interface{})
		if !ok {
			return nil, false
		}

		current = nested
	}

	return nil, false
}

func (e Event) SetField(path string, value interface{}) Event {
	if path == "" {
		return e
	}

	result := e.Clone()
	if result.Payload == nil {
		result.Payload = make(map[string]interface{})
	}

	keys := strings.Split(path, ".")
	current := result.Payload

	for i, key := range keys {
		if i == len(keys)-1 {
			current[key] = value
			break
		}

		next, ok := current[key].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[key] = next
		}

		current = next
	}

	return result
}

func (e Event) RemoveField(path string) Event {
	if path == "" {
		return e
	}

	result := e.Clone()
	if result.Payload == nil {
		return result
	}

	keys := strings.Split(path, ".")
	current := result.Payload

	for i, key := range keys {
		if i == len(keys)-1 {
			delete(current, key)
			return result
		}

		nested, ok := current[key].(map[string]interface{})
		if !ok {
			return result
		}

		current = nested
	}

	return result
}
