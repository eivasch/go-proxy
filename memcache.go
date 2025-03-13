package main

import (
	"sync"
)

type SafeCache struct {
	mu    sync.RWMutex
	cache map[string][]byte
}

func (sc *SafeCache) Get(key string) ([]byte, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	val, ok := sc.cache[key]
	return val, ok
}

func (sc *SafeCache) Set(key string, value []byte) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[key] = value
}
