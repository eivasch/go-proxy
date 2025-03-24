package memorycache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type SafeCache struct {
	mu    sync.RWMutex
	cache map[string]*item
	ctx   context.Context
}

func NewSafeCache(ctx context.Context) *SafeCache {
	return &SafeCache{
		cache: make(map[string]*item),
		ctx:   ctx,
	}
}

func (sc *SafeCache) Get(key string) ([]byte, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	item, ok := sc.cache[key]
	if !ok {
		return nil, false
	}

	item.mu.RLock()
	defer item.mu.RUnlock()
	if item.deleted {
		return nil, false
	}

	item.lastAccessed = time.Now()
	return item.Value, true
}

func (sc *SafeCache) Set(key string, value []byte) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[key] = newItem(value)
}

func (sc *SafeCache) DeletingLoop(defaultTTLSec int) {
	ticker := time.NewTicker(time.Duration(defaultTTLSec) * time.Second)
	defer ticker.Stop()

	deletedItemsChan := make(chan string)
	defer close(deletedItemsChan)

	for {
		select {
		case <-sc.ctx.Done():
			return
		case key := <-deletedItemsChan:
			sc.mu.Lock()
			delete(sc.cache, key)
			sc.mu.Unlock()
		case <-ticker.C:
			// todo: don't start processing if a previous run is still in progress
			go sc.cleanupExpiredItems(defaultTTLSec, deletedItemsChan)
		}
	}
}

func (sc *SafeCache) cleanupExpiredItems(ttlSec int, deletedItemsChan chan<- string) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	now := time.Now()
	for key, item := range sc.cache {
		select {
		case <-sc.ctx.Done():
			return
		default:
		}

		if !item.deleted && now.Sub(item.lastAccessed) > time.Duration(ttlSec)*time.Second {
			fmt.Printf("Key %s expired\n", key)
			item.safeDelete()
			deletedItemsChan <- key
		}
	}
}
