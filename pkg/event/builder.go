package event

import (
	"reflect"
	"strings"
	"time"
)

// Builder provides a fluent interface for constructing Event instances,
// primarily intended for testing and example code.
type Builder struct {
	event Event
}

// NewBuilder creates a new Event builder with default values.
func NewBuilder() *Builder {
	return &Builder{
		event: Event{
			Payload:   make(map[string]interface{}),
			Metadata:  Metadata{},
			Timestamp: time.Now(),
			Headers:   make(map[string]string),
		},
	}
}

// WithPayload sets the entire payload map, replacing any existing payload.
// Note: this performs a shallow copy of the top-level map: keys and their
// values are copied into a new map, but nested/reference types (maps,
// slices, pointers, etc.) are NOT deep-copied and will remain shared with
// the caller. Callers must be aware of shared mutable state when passing
// complex payloads.
func (b *Builder) WithPayload(payload map[string]interface{}) *Builder {
	b.event.Payload = make(map[string]interface{})
	for k, v := range payload {
		b.event.Payload[k] = v
	}
	return b
}

// WithField sets a single field in the payload, supporting nested paths using dot notation.
// Creates intermediate maps as needed for nested paths.
func (b *Builder) WithField(path string, value interface{}) *Builder {
	keys := splitPath(path)
	current := b.event.Payload

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

	return b
}

// WithMetadata sets the Kafka metadata for the event.
func (b *Builder) WithMetadata(metadata Metadata) *Builder {
	// Defensive copy of Metadata so caller cannot mutate underlying Key slice.
	var m Metadata = metadata
	if metadata.Key != nil {
		keyCopy := make([]byte, len(metadata.Key))
		copy(keyCopy, metadata.Key)
		m.Key = keyCopy
	}
	b.event.Metadata = m
	return b
}

// WithTopic sets the Kafka topic in metadata.
func (b *Builder) WithTopic(topic string) *Builder {
	b.event.Metadata.Topic = topic
	return b
}

// WithPartition sets the Kafka partition in metadata.
func (b *Builder) WithPartition(partition int32) *Builder {
	b.event.Metadata.Partition = partition
	return b
}

// WithOffset sets the Kafka offset in metadata.
func (b *Builder) WithOffset(offset int64) *Builder {
	b.event.Metadata.Offset = offset
	return b
}

// WithKey sets the Kafka message key in metadata.
func (b *Builder) WithKey(key []byte) *Builder {
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	b.event.Metadata.Key = keyCopy
	return b
}

// WithHeaders sets the entire headers map, replacing any existing headers.
func (b *Builder) WithHeaders(headers map[string]string) *Builder {
	b.event.Headers = make(map[string]string)
	for k, v := range headers {
		b.event.Headers[k] = v
	}
	return b
}

// WithHeader sets a single header key-value pair.
func (b *Builder) WithHeader(key, value string) *Builder {
	b.event.Headers[key] = value
	return b
}

// WithTimestamp sets the event timestamp.
func (b *Builder) WithTimestamp(timestamp time.Time) *Builder {
	b.event.Timestamp = timestamp
	return b
}

// Build returns the constructed Event instance.
func (b *Builder) Build() Event {
	// Create a deep-copied Event so callers cannot mutate internal builder state
	// by changing maps/slices after Build is called.
	out := Event{
		Timestamp: b.event.Timestamp,
	}

	// Copy Metadata (including Key slice)
	out.Metadata = b.event.Metadata
	if b.event.Metadata.Key != nil {
		kc := make([]byte, len(b.event.Metadata.Key))
		copy(kc, b.event.Metadata.Key)
		out.Metadata.Key = kc
	}

	// Deep-copy Payload (maps and nested maps/slices)
	if b.event.Payload != nil {
		out.Payload = make(map[string]interface{}, len(b.event.Payload))
		for k, v := range b.event.Payload {
			out.Payload[k] = deepCopyValue(v)
		}
	} else {
		out.Payload = nil
	}

	// Copy headers
	if b.event.Headers != nil {
		out.Headers = make(map[string]string, len(b.event.Headers))
		for k, v := range b.event.Headers {
			out.Headers[k] = v
		}
	} else {
		out.Headers = nil
	}

	return out
}

// deepCopyValue performs a deep copy of the given value using reflection.
// It recursively copies maps, slices, arrays, and structs (exported fields only).
// Pointers are dereferenced and their targets copied.
// Primitive types and unsupported types (channels, functions, etc.) are returned as-is.
// Nil values are handled safely.
func deepCopyValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	return deepCopyReflect(val).Interface()
}

func deepCopyReflect(val reflect.Value) reflect.Value {
	switch val.Kind() {
	case reflect.Map:
		if val.IsNil() {
			return val
		}
		newMap := reflect.MakeMap(val.Type())
		for _, key := range val.MapKeys() {
			newKey := deepCopyReflect(key)
			newValue := deepCopyReflect(val.MapIndex(key))
			newMap.SetMapIndex(newKey, newValue)
		}
		return newMap
	case reflect.Slice:
		if val.IsNil() {
			return val
		}
		newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())
		for i := 0; i < val.Len(); i++ {
			newSlice.Index(i).Set(deepCopyReflect(val.Index(i)))
		}
		return newSlice
	case reflect.Array:
		newArray := reflect.New(val.Type()).Elem()
		for i := 0; i < val.Len(); i++ {
			newArray.Index(i).Set(deepCopyReflect(val.Index(i)))
		}
		return newArray
	case reflect.Struct:
		newStruct := reflect.New(val.Type()).Elem()
		for i := 0; i < val.NumField(); i++ {
			if val.Type().Field(i).IsExported() { // only exported fields
				newStruct.Field(i).Set(deepCopyReflect(val.Field(i)))
			}
		}
		return newStruct
	case reflect.Ptr:
		if val.IsNil() {
			return val
		}
		newPtr := reflect.New(val.Type().Elem())
		newPtr.Elem().Set(deepCopyReflect(val.Elem()))
		return newPtr
	default:
		// Primitives, interfaces, channels, functions, etc. - return as-is
		return val
	}
}

// splitPath splits a dot-separated path into individual keys.
// Empty path returns empty slice.
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			// skip empty segments caused by consecutive/leading/trailing dots
			continue
		}
		out = append(out, p)
	}
	return out
}
