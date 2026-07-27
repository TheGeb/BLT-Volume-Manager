package backend

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdConfig configures an etcd backend client.
type EtcdConfig struct {
	Endpoints      []string
	DialTimeout    time.Duration
	RequestTimeout time.Duration
}

type EtcdClient struct {
	client *clientv3.Client
	cfg    EtcdConfig
}

func NewEtcdClient(cfg EtcdConfig) (*EtcdClient, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd: at least one endpoint required")
	}
	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd: create client: %w", err)
	}

	return &EtcdClient{client: cli, cfg: cfg}, nil
}

func (c *EtcdClient) PutObject(ctx context.Context, key string, data []byte) error {
	putCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	_, err := c.client.Put(putCtx, key, string(data))
	return err
}

func (c *EtcdClient) ReadObject(ctx context.Context, key string) ([]byte, error) {
	getCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(getCtx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, ErrKeyNotFound
	}
	return resp.Kvs[0].Value, nil
}

func (c *EtcdClient) DeleteObject(ctx context.Context, key string) error {
	delCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	_, err := c.client.Delete(delCtx, key)
	return err
}

func (c *EtcdClient) ListObjects(ctx context.Context, prefix string) ([]Entry, error) {
	listCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(listCtx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		entries = append(entries, Entry{Key: &key, ModificationCounter: &kv.ModRevision})
	}
	return entries, nil
}

func (c *EtcdClient) DeleteObjectsWithPrefix(ctx context.Context, prefix string) error {
	delCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	_, err := c.client.Delete(delCtx, prefix, clientv3.WithPrefix())
	return err
}
