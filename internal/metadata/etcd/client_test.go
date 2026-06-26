package etcd

import (
	"net/http"
	"testing"
	"time"
)

func TestNewClient_NoEndpoints(t *testing.T) {
	_, err := NewClient(Config{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewClient_DialTimeout(t *testing.T) {
	store, err := NewClient(Config{Endpoints: []string{"http://localhost:2379"}, DialTimeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	cl := store.(*Client)
	if cl.hc.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", cl.hc.Timeout)
	}
}

func TestNewClient_DefaultDialTimeout(t *testing.T) {
	store, err := NewClient(Config{Endpoints: []string{"http://localhost:2379"}})
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	cl := store.(*Client)
	if cl.hc.Timeout != 5*time.Second {
		t.Errorf("expected default timeout 5s, got %v", cl.hc.Timeout)
	}
}

func TestB64(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"hello", "aGVsbG8="},
		{"test/key", "dGVzdC9rZXk="},
	}
	for _, tt := range tests {
		got := b64(tt.input)
		if got != tt.want {
			t.Errorf("b64(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPrefixRangeEnd(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"volumes/", "dm9sdW1lczA="},
		{"a", "Yg=="},
		{"", ""},
	}
	for _, tt := range tests {
		got := prefixRangeEnd(tt.prefix)
		if got != tt.want {
			t.Errorf("prefixRangeEnd(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}

func TestURLFor(t *testing.T) {
	c := &Client{
		endpoints: []string{"http://etcd:2379"},
		hc:        &http.Client{},
	}
	got := c.urlFor("kv/put")
	want := "http://etcd:2379/v3/kv/put"
	if got != want {
		t.Errorf("urlFor = %q, want %q", got, want)
	}
}

func TestURLFor_WithTrailingSlash(t *testing.T) {
	c := &Client{
		endpoints: []string{"http://etcd:2379/"},
		hc:        &http.Client{},
	}
	got := c.urlFor("kv/range")
	want := "http://etcd:2379/v3/kv/range"
	if got != want {
		t.Errorf("urlFor = %q, want %q", got, want)
	}
}

func TestEndpoint(t *testing.T) {
	c := &Client{
		endpoints: []string{"http://localhost:2379", "http://backup:2379"},
	}
	if got := c.endpoint(); got != "http://localhost:2379" {
		t.Errorf("endpoint = %q, want first endpoint", got)
	}
}
