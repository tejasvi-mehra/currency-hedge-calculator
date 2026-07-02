package metrics

import (
	"sort"
	"sync"
	"time"
)

// Snapshot is a point-in-time view of in-memory operational metrics.
type Snapshot struct {
	RequestCount             int64   `json:"request_count"`
	ErrorCount               int64   `json:"error_count"`
	FXCacheHitCount          int64   `json:"fx_cache_hit_count"`
	FXCacheMissCount         int64   `json:"fx_cache_miss_count"`
	FXUpstreamFailureCount   int64   `json:"fx_upstream_failure_count"`
	IdempotencyReplayCount   int64   `json:"idempotency_replay_count"`
	IdempotencyConflictCount int64   `json:"idempotency_conflict_count"`
	HighRiskExposureTotal    float64 `json:"high_risk_exposure_total"`
	P95LatencyMilliseconds   float64 `json:"p95_latency_milliseconds"`
}

// Collector tracks lightweight API and domain metrics in memory.
type Collector struct {
	mu                       sync.Mutex
	requestCount             int64
	errorCount               int64
	fxCacheHitCount          int64
	fxCacheMissCount         int64
	fxUpstreamFailureCount   int64
	idempotencyReplayCount   int64
	idempotencyConflictCount int64
	highRiskExposureTotal    float64
	latencySamples           []float64
}

// NewCollector constructs an in-memory metrics collector.
func NewCollector() *Collector {
	return &Collector{
		latencySamples: make([]float64, 0, 1024),
	}
}

func (c *Collector) AddRequest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestCount++
}

func (c *Collector) AddError() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorCount++
}

func (c *Collector) AddFXCacheHit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fxCacheHitCount++
}

func (c *Collector) AddFXCacheMiss() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fxCacheMissCount++
}

func (c *Collector) AddFXUpstreamFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fxUpstreamFailureCount++
}

func (c *Collector) AddIdempotencyReplay() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.idempotencyReplayCount++
}

func (c *Collector) AddIdempotencyConflict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.idempotencyConflictCount++
}

func (c *Collector) AddHighRiskExposure(amount float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.highRiskExposureTotal += amount
}

func (c *Collector) AddLatency(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latencySamples = append(c.latencySamples, float64(d.Milliseconds()))
	if len(c.latencySamples) > 4096 {
		c.latencySamples = c.latencySamples[len(c.latencySamples)-4096:]
	}
}

// Snapshot returns a metrics snapshot and derived p95 latency.
func (c *Collector) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	samples := make([]float64, len(c.latencySamples))
	copy(samples, c.latencySamples)
	p95 := 0.0
	if len(samples) > 0 {
		sort.Float64s(samples)
		index := int(float64(len(samples)-1) * 0.95)
		if index < 0 {
			index = 0
		}
		p95 = samples[index]
	}

	return Snapshot{
		RequestCount:             c.requestCount,
		ErrorCount:               c.errorCount,
		FXCacheHitCount:          c.fxCacheHitCount,
		FXCacheMissCount:         c.fxCacheMissCount,
		FXUpstreamFailureCount:   c.fxUpstreamFailureCount,
		IdempotencyReplayCount:   c.idempotencyReplayCount,
		IdempotencyConflictCount: c.idempotencyConflictCount,
		HighRiskExposureTotal:    c.highRiskExposureTotal,
		P95LatencyMilliseconds:   p95,
	}
}
