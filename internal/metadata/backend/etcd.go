package backend

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdConfig configures an etcd backend client.
type EtcdConfig struct {
	Endpoints   []string
	DialTimeout time.Duration
}

type etcdClient struct {
	client *clientv3.Client
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

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd: create client: %w", err)
	}

	return &etcdClient{client: cli}, nil
}

func (c *etcdClient) PutObject(key string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.Put(ctx, key, string(data))
	return err
}

func (c *etcdClient) ReadObject(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, ErrKeyNotFound
	}
	return resp.Kvs[0].Value, nil
}

func (c *etcdClient) DeleteObject(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.Delete(ctx, key)
	return err
}

func (c *etcdClient) ListObjects(prefix string) ([]Entry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		entries = append(entries, Entry{Key: &key})
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.Delete(ctx, prefix, clientv3.WithPrefix())
	return err
}
