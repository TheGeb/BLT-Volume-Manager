package backend

import (
	"testing"
)

func TestNewClient_NoEndpoints(t *testing.T) {
	t.Parallel()
	_, err := NewEtcdClient(EtcdConfig{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewClient_WithEndpoints(t *testing.T) {
	t.Parallel()
	store, err := NewEtcdClient(EtcdConfig{Endpoints: []string{"http://localhost:2379"}})
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}
