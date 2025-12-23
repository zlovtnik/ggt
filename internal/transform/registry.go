package transform

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// TransformFactory builds a Transform from raw config.
type TransformFactory func(cfg json.RawMessage) (Transform, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]TransformFactory{}
)

var ErrUnknownTransform = errors.New("unknown transform type")

// Register registers a factory under the given name. Calling Register twice for
// the same name will overwrite the previous factory.
func Register(name string, f TransformFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = f
}

// Create constructs a Transform by name using the provided raw config.
func Create(name string, cfg json.RawMessage) (Transform, error) {
	registryMu.RLock()
	f, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTransform, name)
	}
	return f(cfg)
}

// Registered returns a copy of the currently registered transform names.
func Registered() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Unregister removes a previously registered factory by name. It returns true
// when a factory was removed, or false if no factory existed for the name.
func Unregister(name string) bool {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[name]; !ok {
		return false
	}
	delete(registry, name)
	return true
}
