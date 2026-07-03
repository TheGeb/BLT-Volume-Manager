package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// TODO: Integrate/wrap official etcd client, go.etcd.io/etcd/client/v3

// EtcdConfig configures an etcd backend client.
type EtcdConfig struct {
	Endpoints   []string
	DialTimeout time.Duration
}

// etcdClient implements KeyValueStore backed by etcd's v3 gRPC-gateway API.
type etcdClient struct {
	endpoints []string
	hc        *http.Client
}

var _ KeyValueStore = (*etcdClient)(nil)

func NewEtcdClient(cfg EtcdConfig) (KeyValueStore, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd: at least one endpoint required")
	}
	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	return &etcdClient{
		endpoints: cfg.Endpoints,
		hc: &http.Client{
			Timeout: dialTimeout,
		},
	}, nil
}

func (c *etcdClient) endpoint() string { return c.endpoints[0] }

func (c *etcdClient) urlFor(path string) string {
	ep := c.endpoint()
	if !strings.HasSuffix(ep, "/") {
		ep += "/"
	}
	return ep + "v3/" + path
}

// ---------------------------------------------------------------------------
// etcd v3 gRPC-gateway request/response helpers
// ---------------------------------------------------------------------------

type kvPut struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type kvRange struct {
	Key       string `json:"key"`
	RangeEnd  string `json:"range_end,omitempty"`
	CountOnly bool   `json:"count_only,omitempty"`
}

type kvDelete struct {
	Key      string `json:"key"`
	RangeEnd string `json:"range_end,omitempty"`
}

type kvResponse struct {
	Kvs   []kvEntry `json:"kvs"`
	Count int       `json:"count"`
}

type kvEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	ModRev    int64  `json:"mod_revision"`
	CreateRev int64  `json:"create_revision"`
	Version   int64  `json:"version"`
}

type etcdError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Cause   string `json:"cause"`
	Code    int    `json:"code"`
}

func (c *etcdClient) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("etcd: marshal request: %w", err)
		}
	}

	u := c.urlFor(path)
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("etcd: create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("etcd: %s %s: %w", method, u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("etcd: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var ee etcdError
		if json.Unmarshal(respBody, &ee) == nil && ee.Error != "" {
			return nil, fmt.Errorf("etcd: %s", ee.Error)
		}
		return nil, fmt.Errorf("etcd: %s %s: status=%d body=%s", method, u, resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func prefixRangeEnd(prefix string) string {
	end := []byte(prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return base64.StdEncoding.EncodeToString(end[:i+1])
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// KeyValueStore implementation
// ---------------------------------------------------------------------------

func (c *etcdClient) PutObject(key string, data []byte) error {
	body := kvPut{Key: b64(key), Value: b64(string(data))}
	_, err := c.do(context.Background(), http.MethodPost, "kv/put", body)
	return err
}

func (c *etcdClient) ReadObject(key string) ([]byte, error) {
	body := kvRange{Key: b64(key)}
	data, err := c.do(context.Background(), http.MethodPost, "kv/range", body)
	if err != nil {
		return nil, err
	}
	var resp kvResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("etcd: decode range response: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, ErrKeyNotFound
	}
	val, err := base64.StdEncoding.DecodeString(resp.Kvs[0].Value)
	if err != nil {
		return nil, fmt.Errorf("etcd: decode value: %w", err)
	}
	return val, nil
}

func (c *etcdClient) DeleteObject(key string) error {
	body := kvDelete{Key: b64(key)}
	_, err := c.do(context.Background(), http.MethodPost, "kv/deleterange", body)
	return err
}

func (c *etcdClient) ListObjects(prefix string) ([]Entry, error) {
	body := kvRange{Key: b64(prefix), RangeEnd: prefixRangeEnd(prefix)}
	data, err := c.do(context.Background(), http.MethodPost, "kv/range", body)
	if err != nil {
		return nil, err
	}
	var resp kvResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("etcd: decode range response: %w", err)
	}

	entries := make([]Entry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		k, err := base64.StdEncoding.DecodeString(kv.Key)
		if err != nil {
			continue
		}
		keyStr := string(k)
		entries = append(entries, Entry{Key: &keyStr})
	}
	return entries, nil
}

func (c *etcdClient) ListCommonPrefixes(prefix, delimiter string) ([]string, error) {
	objects, err := c.ListObjects(prefix)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var prefixes []string
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		rest := strings.TrimPrefix(*obj.Key, prefix)
		idx := strings.Index(rest, delimiter)
		if idx < 0 {
			continue
		}
		common := prefix + rest[:idx+len(delimiter)]
		if !seen[common] {
			seen[common] = true
			prefixes = append(prefixes, common)
		}
	}
	sort.Strings(prefixes)
	return prefixes, nil
}

func (c *etcdClient) DeleteObjectsWithPrefix(prefix string) error {
	body := kvDelete{Key: b64(prefix), RangeEnd: prefixRangeEnd(prefix)}
	_, err := c.do(context.Background(), http.MethodPost, "kv/deleterange", body)
	return err
}
