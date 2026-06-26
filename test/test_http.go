package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func DoRequest(t *testing.T, baseURL, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func DoOK(t *testing.T, baseURL, method, path string, body any) map[string]any {
	t.Helper()
	resp := DoRequest(t, baseURL, method, path, body)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: expected 200, got %d: %s", method, path, resp.StatusCode, string(b))
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return m
}

func DoArray(t *testing.T, baseURL, method, path string) []any {
	t.Helper()
	resp := DoRequest(t, baseURL, method, path, nil)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: expected 200, got %d: %s", method, path, resp.StatusCode, string(b))
	}
	var arr []any
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("decode array response: %v", err)
	}
	return arr
}

func DoErr(t *testing.T, baseURL, method, path string, body any, wantCode int) map[string]any {
	t.Helper()
	resp := DoRequest(t, baseURL, method, path, body)
	if resp.StatusCode != wantCode {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, wantCode, resp.StatusCode, string(b))
	}
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return m
}
