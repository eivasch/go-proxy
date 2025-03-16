package main

import (
	"fmt"
	"sync"
	"time"
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
	defaultTTLSec := 5

	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[key] = value

	go func() {
		time.Sleep(time.Duration(defaultTTLSec) * time.Second)
		sc.mu.Lock()
		defer sc.mu.Unlock()
		delete(sc.cache, key)

		fmt.Printf("Key %s deleted from cache\n", key)
	}()
}
