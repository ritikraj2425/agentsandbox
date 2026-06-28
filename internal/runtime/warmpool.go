// Package runtime defines the standard execution interface for all sandbox backends.
//
// This file implements the WarmPool manager, which pre-boots a pool of microVMs
// (or containers) and keeps them in a paused/ready state. When an execution
// request arrives, a pre-booted VM is immediately resumed instead of cold-booting
// a new one, reducing startup latency from ~1s to <50ms.
//
// The WarmPool is generic and works with any runtime backend that implements
// the WarmableRuntime interface, though it is primarily designed for Firecracker
// microVMs where boot time is the dominant latency component.

package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// WarmableRuntime extends the base Runtime interface with lifecycle methods
// that support pre-booting and pausing execution environments. Backends that
// implement this interface can be managed by the WarmPool.
type WarmableRuntime interface {
	Runtime

	// Boot starts the execution environment (VM/container) and leaves it
	// in a ready-to-execute state. Returns an opaque instance ID.
	Boot(ctx context.Context) (instanceID string, err error)

	// Execute runs a command on an already-booted instance identified by
	// its instance ID. Returns the Observation.
	Execute(ctx context.Context, instanceID string, action protocol.Action) (protocol.Observation, error)

	// Destroy tears down the instance and releases all associated resources.
	Destroy(instanceID string) error
}

// WarmPoolConfig configures the behavior of a WarmPool.
type WarmPoolConfig struct {
	// PoolSize is the number of pre-booted instances to maintain.
	PoolSize int

	// BootTimeout is the maximum time to wait for a single instance to boot.
	BootTimeout time.Duration

	// ReplenishInterval is how often the pool checks if it needs to boot
	// new instances to maintain the target pool size.
	ReplenishInterval time.Duration
}

// DefaultWarmPoolConfig provides sensible defaults for production use.
var DefaultWarmPoolConfig = WarmPoolConfig{
	PoolSize:          5,
	BootTimeout:       30 * time.Second,
	ReplenishInterval: 5 * time.Second,
}

// WarmPool manages a pool of pre-booted execution environments to minimize
// cold-start latency for incoming action requests.
type WarmPool struct {
	runtime WarmableRuntime
	config  WarmPoolConfig

	// mu protects the ready queue.
	mu    sync.Mutex
	ready []string // Instance IDs of booted, idle instances.

	// stopCh signals the background replenisher to shut down.
	stopCh chan struct{}
	wg     sync.WaitGroup

	// stats tracks pool performance metrics.
	stats WarmPoolStats
}

// WarmPoolStats provides observability into pool behavior.
type WarmPoolStats struct {
	mu             sync.Mutex
	TotalBooted    int64         // Total instances booted since pool creation.
	TotalServed    int64         // Total requests served from warm instances.
	TotalColdStart int64         // Total requests that required a cold boot.
	AvgBootTime    time.Duration // Rolling average boot time.
	bootTimes      []time.Duration
}

// NewWarmPool creates a WarmPool for the given WarmableRuntime and starts the
// background replenishment goroutine. Call Stop() to shut down cleanly.
func NewWarmPool(rt WarmableRuntime, cfg WarmPoolConfig) (*WarmPool, error) {
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = DefaultWarmPoolConfig.PoolSize
	}
	if cfg.BootTimeout == 0 {
		cfg.BootTimeout = DefaultWarmPoolConfig.BootTimeout
	}
	if cfg.ReplenishInterval == 0 {
		cfg.ReplenishInterval = DefaultWarmPoolConfig.ReplenishInterval
	}

	pool := &WarmPool{
		runtime: rt,
		config:  cfg,
		ready:   make([]string, 0, cfg.PoolSize),
		stopCh:  make(chan struct{}),
	}

	// Pre-boot the initial pool synchronously so the first request is fast.
	for i := 0; i < cfg.PoolSize; i++ {
		id, err := pool.bootOne()
		if err != nil {
			// If we can't boot any, clean up and fail.
			pool.drainAll()
			return nil, fmt.Errorf("failed to pre-boot warm pool instance %d: %w", i, err)
		}
		pool.mu.Lock()
		pool.ready = append(pool.ready, id)
		pool.mu.Unlock()
	}

	// Start background replenishment.
	pool.wg.Add(1)
	go pool.replenisher()

	return pool, nil
}

// Acquire retrieves a pre-booted instance from the pool. If the pool is empty,
// it performs a synchronous cold boot (and records it in stats).
func (p *WarmPool) Acquire() (string, error) {
	p.mu.Lock()
	if len(p.ready) > 0 {
		id := p.ready[0]
		p.ready = p.ready[1:]
		p.mu.Unlock()

		p.stats.mu.Lock()
		p.stats.TotalServed++
		p.stats.mu.Unlock()

		return id, nil
	}
	p.mu.Unlock()

	// Cold boot fallback.
	id, err := p.bootOne()
	if err != nil {
		return "", fmt.Errorf("cold boot failed: %w", err)
	}

	p.stats.mu.Lock()
	p.stats.TotalColdStart++
	p.stats.mu.Unlock()

	return id, nil
}

// Release destroys an instance after use. The background replenisher will
// replace it with a fresh pre-booted instance.
func (p *WarmPool) Release(instanceID string) {
	_ = p.runtime.Destroy(instanceID)
}

// Run implements the Runtime interface by acquiring a warm instance, executing
// the action, and releasing the instance.
func (p *WarmPool) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	instanceID, err := p.Acquire()
	if err != nil {
		obs := protocol.NewObservation(action.ID)
		obs.Status = protocol.ObsStatusFailed
		obs.Error = fmt.Sprintf("failed to acquire warm instance: %s", err)
		obs.Backend = p.runtime.Name() + "+warmpool"
		return obs, err
	}
	defer p.Release(instanceID)

	obs, err := p.runtime.Execute(ctx, instanceID, action)
	obs.Backend = p.runtime.Name() + "+warmpool"
	return obs, err
}

// Name returns the underlying runtime name with a "+warmpool" suffix.
func (p *WarmPool) Name() string {
	return p.runtime.Name() + "+warmpool"
}

// Stats returns a snapshot of the pool's performance statistics.
func (p *WarmPool) Stats() WarmPoolStats {
	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()
	return WarmPoolStats{
		TotalBooted:    p.stats.TotalBooted,
		TotalServed:    p.stats.TotalServed,
		TotalColdStart: p.stats.TotalColdStart,
		AvgBootTime:    p.stats.AvgBootTime,
	}
}

// PoolSize returns the current number of ready instances.
func (p *WarmPool) PoolSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.ready)
}

// Stop shuts down the warm pool gracefully, destroying all idle instances
// and stopping the background replenisher.
func (p *WarmPool) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	p.drainAll()
}

// bootOne boots a single instance and records timing statistics.
func (p *WarmPool) bootOne() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.config.BootTimeout)
	defer cancel()

	start := time.Now()
	id, err := p.runtime.Boot(ctx)
	bootTime := time.Since(start)

	if err != nil {
		return "", err
	}

	// Update stats.
	p.stats.mu.Lock()
	p.stats.TotalBooted++
	p.stats.bootTimes = append(p.stats.bootTimes, bootTime)
	// Calculate rolling average.
	var total time.Duration
	for _, t := range p.stats.bootTimes {
		total += t
	}
	p.stats.AvgBootTime = total / time.Duration(len(p.stats.bootTimes))
	p.stats.mu.Unlock()

	return id, nil
}

// replenisher runs in the background, periodically checking if the pool needs
// more instances booted to maintain the target size.
func (p *WarmPool) replenisher() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.config.ReplenishInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.mu.Lock()
			deficit := p.config.PoolSize - len(p.ready)
			p.mu.Unlock()

			for i := 0; i < deficit; i++ {
				id, err := p.bootOne()
				if err != nil {
					break // Back off on failures.
				}
				p.mu.Lock()
				p.ready = append(p.ready, id)
				p.mu.Unlock()
			}
		}
	}
}

// drainAll destroys all ready instances in the pool.
func (p *WarmPool) drainAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range p.ready {
		_ = p.runtime.Destroy(id)
	}
	p.ready = nil
}
