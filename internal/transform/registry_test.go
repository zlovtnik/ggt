package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestRegistryConcurrency exercises concurrent Register/Create/Registered/Unregister
// operations to ensure the registry's mutex protects against races.
func TestRegistryConcurrency(t *testing.T) {
	// Clean up any registrations we may leave behind (only our prefix)
	defer func() {
		for _, name := range Registered() {
			if strings.HasPrefix(name, "concurrent-") {
				Unregister(name)
			}
		}
	}()

	var wg sync.WaitGroup
	// spawn many goroutines to exercise concurrent access
	const goroutines = 500
	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// use unique names per goroutine to avoid benign races where another
			// goroutine unregisters the same name between Register and Create.
			name := fmt.Sprintf("concurrent-%d", i)

			// register a simple factory
			Register(name, func(cfg json.RawMessage) (Transform, error) {
				return NewFunc("noop", func(ctx context.Context, e interface{}) (interface{}, error) {
					return e, nil
				}), nil
			})

			// create an instance
			if _, err := Create(name, nil); err != nil {
				errCh <- fmt.Errorf("Create(%s) error: %w", name, err)
				return
			}

			// snapshot registered names (no-op, just exercise read)
			_ = Registered()

			// unregister
			Unregister(name)
		}(i)
	}
	wg.Wait()
	close(errCh)

	// Fail test if any goroutine reported an error
	for err := range errCh {
		t.Error(err)
	}

	// Verify no leftover registrations with our test prefix
	for _, name := range Registered() {
		if strings.HasPrefix(name, "concurrent-") {
			t.Errorf("leftover registration: %s", name)
		}
	}
}
