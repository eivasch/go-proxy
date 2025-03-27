package memorycache

import (
	"context"
	"log"
	"sync"
	"time"
)

type SafeCache struct {
	mu    sync.RWMutex
	cache map[string]*item
}

func NewSafeCache() *SafeCache {
	return &SafeCache{
		cache: make(map[string]*item),
	}
}

func (sc *SafeCache) Get(key string) ([]byte, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	item, ok := sc.cache[key]
	if !ok {
		return nil, false
	}

	item.mu.Lock()
	defer item.mu.Unlock()
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

func (sc *SafeCache) DeletingLoop(defaultTTLSec int, ctx context.Context) {
	deletedItemsChan := make(chan string)

	go func() {
		defer close(deletedItemsChan)
		ticker := time.NewTicker(time.Duration(defaultTTLSec) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sc.cleanupExpiredItems(defaultTTLSec, deletedItemsChan, ctx)
			}
		}
	}()

	for key := range deletedItemsChan {
		sc.mu.Lock()
		delete(sc.cache, key)
		sc.mu.Unlock()
	}
}

func (sc *SafeCache) cleanupExpiredItems(ttlSec int, deletedItemsChan chan<- string, ctx context.Context) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	now := time.Now()
	for key, item := range sc.cache {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !item.deleted && now.Sub(item.lastAccessed) > time.Duration(ttlSec)*time.Second {
			log.Printf("Key %s expired\n", key)
			item.safeDelete()
			deletedItemsChan <- key
		}
	}
}
