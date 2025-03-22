package memorycache

import (
	"sync"
	"time"
)

type item struct {
	Value        []byte
	deleted      bool
	lastAccessed time.Time
	mu 		 sync.RWMutex
}

func newItem(value []byte) *item {
	return &item{
		Value:        value,
		lastAccessed: time.Now(),
		deleted: 	false,
	}
}

func (i *item) safeDelete() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.deleted = true
}
