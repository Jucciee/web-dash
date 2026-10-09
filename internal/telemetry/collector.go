package telemetry

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"homedashboard/internal/domain"
)

type Config struct{ DockerHost, HostProc, HostSys, HostRoot, HostDisk, HostName string }

type Collector struct {
	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc
	host   cache[domain.HostSnapshot]
	docker cache[domain.DockerSnapshot]
}

func New(cfg Config) *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{cfg: cfg, ctx: ctx, cancel: cancel}
}

func (c *Collector) Close() { c.cancel() }

// Collection happens only when a visible page requests it. Concurrent tabs
// share a bounded read; closing a tab does not abort another tab's read.
type cache[T any] struct {
	mu      sync.Mutex
	value   T
	expires time.Time
	group   singleflight.Group
}

func (cache *cache[T]) get(ctx, lifetime context.Context, collect func(context.Context) T) (T, error) {
	read := func() (T, bool) {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return cache.value, time.Now().Before(cache.expires)
	}
	if value, fresh := read(); fresh {
		return value, nil
	}
	result := cache.group.DoChan("snapshot", func() (any, error) {
		if value, fresh := read(); fresh {
			return value, nil
		}
		bounded, cancel := context.WithTimeout(lifetime, 12*time.Second)
		defer cancel()
		value := collect(bounded)
		cache.mu.Lock()
		cache.value, cache.expires = value, time.Now().Add(5*time.Second)
		cache.mu.Unlock()
		return value, nil
	})
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case result := <-result:
		return result.Val.(T), result.Err
	}
}

func (c *Collector) Host(ctx context.Context) (domain.HostSnapshot, error) {
	return c.host.get(ctx, c.ctx, c.collectHost)
}

func (c *Collector) Docker(ctx context.Context) (domain.DockerSnapshot, error) {
	return c.docker.get(ctx, c.ctx, c.collectDocker)
}
