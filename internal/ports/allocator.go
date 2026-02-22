package ports

import (
	"context"
	"fmt"
)

// PortStore is the subset of state.Store needed by the port allocator.
type PortStore interface {
	UsedPorts(ctx context.Context) ([]int, error)
	PortOwner(ctx context.Context, port int) (string, error)
}

// Allocator manages port assignment within a configured range.
type Allocator struct {
	rangeStart int
	rangeEnd   int
	store      PortStore
}

// New creates a port allocator for the given range [rangeStart, rangeEnd).
func New(rangeStart, rangeEnd int, store PortStore) *Allocator {
	return &Allocator{
		rangeStart: rangeStart,
		rangeEnd:   rangeEnd,
		store:      store,
	}
}

// Next returns the lowest free port in the range.
func (a *Allocator) Next(ctx context.Context) (int, error) {
	used, err := a.store.UsedPorts(ctx)
	if err != nil {
		return 0, fmt.Errorf("querying used ports: %w", err)
	}

	usedSet := make(map[int]bool, len(used))
	for _, p := range used {
		usedSet[p] = true
	}

	for port := a.rangeStart; port < a.rangeEnd; port++ {
		if !usedSet[port] {
			return port, nil
		}
	}

	return 0, fmt.Errorf("port range %d-%d exhausted; consider removing unused services or expanding the range in gophercaptain.conf", a.rangeStart, a.rangeEnd)
}

// Request validates that a specific port is available and within range.
func (a *Allocator) Request(ctx context.Context, port int) error {
	if port < a.rangeStart || port >= a.rangeEnd {
		return fmt.Errorf("port %d is outside configured range %d-%d", port, a.rangeStart, a.rangeEnd)
	}

	owner, err := a.store.PortOwner(ctx, port)
	if err != nil {
		return fmt.Errorf("checking port %d: %w", port, err)
	}
	if owner != "" {
		return fmt.Errorf("port %d is already in use by service %q", port, owner)
	}

	return nil
}
