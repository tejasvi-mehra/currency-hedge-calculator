package rates

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type cacheItem struct {
	quote     Quote
	expiresAt time.Time
}

// MemoryCache is a TTL cache for FX quotes.
type MemoryCache struct {
	ttl   time.Duration
	mutex sync.RWMutex
	items map[string]cacheItem
}

// NewMemoryCache creates a quote cache with fixed TTL.
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &MemoryCache{
		ttl:   ttl,
		items: map[string]cacheItem{},
	}
}

// Set stores a quote using settlement->presentment key.
func (c *MemoryCache) Set(quote Quote) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.items[c.key(quote.SettlementCurrency, quote.PresentmentCurrency)] = cacheItem{
		quote:     quote,
		expiresAt: time.Now().UTC().Add(c.ttl),
	}
}

// GetFresh returns a quote if it exists and is not expired.
func (c *MemoryCache) GetFresh(settlementCurrency string, presentmentCurrency string) (Quote, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, ok := c.items[c.key(settlementCurrency, presentmentCurrency)]
	if !ok {
		return Quote{}, false
	}
	if time.Now().UTC().After(item.expiresAt) {
		return Quote{}, false
	}
	return item.quote, true
}

// GetAny returns a quote even when expired (used for stale fallback).
func (c *MemoryCache) GetAny(settlementCurrency string, presentmentCurrency string) (Quote, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, ok := c.items[c.key(settlementCurrency, presentmentCurrency)]
	if !ok {
		return Quote{}, false
	}
	return item.quote, true
}

func (c *MemoryCache) key(settlementCurrency string, presentmentCurrency string) string {
	return fmt.Sprintf("%s:%s", strings.ToUpper(strings.TrimSpace(settlementCurrency)), strings.ToUpper(strings.TrimSpace(presentmentCurrency)))
}
