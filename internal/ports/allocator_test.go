package ports

import (
	"context"
	"strings"
	"testing"
)

// mockPortStore implements PortStore for testing.
type mockPortStore struct {
	ports  []int
	owners map[int]string
}

func (m *mockPortStore) UsedPorts(_ context.Context) ([]int, error) {
	return m.ports, nil
}

func (m *mockPortStore) PortOwner(_ context.Context, port int) (string, error) {
	if m.owners != nil {
		if name, ok := m.owners[port]; ok {
			return name, nil
		}
	}
	return "", nil
}

func TestNextEmptyStore(t *testing.T) {
	store := &mockPortStore{}
	a := New(3000, 4000, store)

	port, err := a.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 3000 {
		t.Errorf("port = %d, want 3000", port)
	}
}

func TestNextSkipsUsed(t *testing.T) {
	store := &mockPortStore{ports: []int{3000, 3001, 3003}}
	a := New(3000, 4000, store)

	port, err := a.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 3002 {
		t.Errorf("port = %d, want 3002", port)
	}
}

func TestNextExhaustedRange(t *testing.T) {
	store := &mockPortStore{ports: []int{3000, 3001, 3002}}
	a := New(3000, 3003, store)

	_, err := a.Next(context.Background())
	if err == nil {
		t.Fatal("expected error for exhausted range")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Errorf("error should mention exhaustion, got: %v", err)
	}
}

func TestRequestAvailable(t *testing.T) {
	store := &mockPortStore{owners: map[int]string{}}
	a := New(3000, 4000, store)

	err := a.Request(context.Background(), 3500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestConflict(t *testing.T) {
	store := &mockPortStore{owners: map[int]string{3000: "api"}}
	a := New(3000, 4000, store)

	err := a.Request(context.Background(), 3000)
	if err == nil {
		t.Fatal("expected error for conflicting port")
	}
	if !strings.Contains(err.Error(), "api") {
		t.Errorf("error should name the conflicting service, got: %v", err)
	}
}

func TestRequestOutOfRange(t *testing.T) {
	store := &mockPortStore{}
	a := New(3000, 4000, store)

	err := a.Request(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for out-of-range port")
	}
	if !strings.Contains(err.Error(), "outside") {
		t.Errorf("error should mention outside range, got: %v", err)
	}
}
