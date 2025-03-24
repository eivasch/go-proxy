package memorycache

import (
	"context"
	"testing"
	"time"
)

// TestSafeCache_SetAndGet verifies the basic functionality of setting values in the cache
// and retrieving them, as well as checking behavior when accessing non-existent keys.
func TestSafeCache_SetAndGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewSafeCache(ctx)

	key := "test-key"
	value := []byte("test-value")

	// Test setting and getting a value
	cache.Set(key, value)

	got, exists := cache.Get(key)
	if !exists {
		t.Errorf("Get(%q) returned exists=false, want true", key)
	}

	if string(got) != string(value) {
		t.Errorf("Get(%q) = %q, want %q", key, got, value)
	}

	// Test getting a non-existent key
	_, exists = cache.Get("non-existent-key")
	if exists {
		t.Errorf("Get on non-existent key returned exists=true, want false")
	}
}

// TestSafeCache_Expiration tests the cache's ability to identify and mark expired items,
// ensuring they are properly flagged for deletion and no longer accessible after expiration.
func TestSafeCache_Expiration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewSafeCache(ctx)

	// Set a short TTL for testing
	ttl := 2 // seconds
	deletedChan := make(chan string, 10)

	key := "expiring-key"
	value := []byte("expiring-value")

	cache.Set(key, value)
	item := cache.cache[key]

	// Manually set lastAccessed to a time in the past
	item.mu.Lock()
	item.lastAccessed = time.Now().Add(-time.Duration(ttl+1) * time.Second)
	item.mu.Unlock()

	cache.cleanupExpiredItems(ttl, deletedChan)

	deleted := false
	select {
	case deletedKey := <-deletedChan:
		if deletedKey == key {
			deleted = true
		}
	case <-time.After(time.Second):
	}

	if !deleted {
		t.Errorf("Key %q was not sent to delete channel after expiration", key)
	}

	item.mu.RLock()
	isDeleted := item.deleted
	item.mu.RUnlock()

	if !isDeleted {
		t.Errorf("Item not marked as deleted after expiration")
	}

	// Verify Get returns not found
	_, exists := cache.Get(key)
	if exists {
		t.Errorf("Get on expired key returned exists=true, want false")
	}
}

// TestSafeCache_DeletedItems verifies that items marked as deleted are properly
// handled and can no longer be retrieved from the cache.
func TestSafeCache_DeletedItems(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewSafeCache(ctx)
	key := "to-delete"
	value := []byte("delete-me")

	cache.Set(key, value)

	// Get the item and mark it as deleted
	item, ok := cache.cache[key]
	if !ok {
		t.Fatalf("Item not found in cache after Set")
	}

	item.safeDelete()

	// Try to get the item - should return not found
	_, exists := cache.Get(key)
	if exists {
		t.Errorf("Get returned exists=true for deleted item, want false")
	}
}

// TestSafeCache_ConcurrentAccess ensures the cache handles concurrent operations
// from multiple goroutines without race conditions or crashes.
func TestSafeCache_ConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewSafeCache(ctx)

	// Start the background cleanup with a TTL
	const ttlSeconds = 5
	go cache.DeletingLoop(ttlSeconds)

	const concurrency = 10
	const operations = 100

	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			for j := 0; j < operations; j++ {
				key := "key" + string(rune('A'+id)) + string(rune('0'+j%10))

				// Alternate between set and get operations
				if j%2 == 0 {
					cache.Set(key, []byte("value-"+key))
				} else {
					cache.Get(key)
				}

				// Small sleep to allow other goroutines to run
				time.Sleep(time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// No crash means the test passes
}

// TestSafeCache_Cancellation verifies that the background cleanup goroutines
// exit properly when the context is cancelled, preventing resource leaks.
func TestSafeCache_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cache := NewSafeCache(ctx)

	const ttlSeconds = 5
	finished := make(chan bool)
	go func() {
		cache.DeletingLoop(ttlSeconds)
		finished <- true
	}()

	cancel()

	// Check if DeletingLoop exits
	select {
	case <-finished:
		// Success
	case <-time.After(time.Second):
		t.Errorf("DeletingLoop did not exit after context cancellation")
	}

	// Test cleanup exits on cancellation
	deletedChan := make(chan string)
	cleanupDone := make(chan bool)
	go func() {
		cache.cleanupExpiredItems(ttlSeconds, deletedChan)
		cleanupDone <- true
	}()

	select {
	case <-cleanupDone:
		// Success
	case <-time.After(time.Second):
		t.Errorf("cleanupExpiredItems did not exit after context cancellation")
	}
}

// TestSafeCache_UpdateExistingItem confirms that updating an existing cache entry
// properly replaces the old value and doesn't create duplicate entries.
func TestSafeCache_UpdateExistingItem(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewSafeCache(ctx)

	key := "update-test"
	originalValue := []byte("original-value")
	updatedValue := []byte("updated-value")

	cache.Set(key, originalValue)

	gotOriginal, exists := cache.Get(key)
	if !exists {
		t.Fatalf("Get(%q) returned exists=false, want true", key)
	}
	if string(gotOriginal) != string(originalValue) {
		t.Errorf("Got %q, want %q", gotOriginal, originalValue)
	}

	cache.Set(key, updatedValue)

	gotUpdated, exists := cache.Get(key)
	if !exists {
		t.Fatalf("Get(%q) after update returned exists=false, want true", key)
	}
	if string(gotUpdated) != string(updatedValue) {
		t.Errorf("After update got %q, want %q", gotUpdated, updatedValue)
	}

	count := 0
	for k := range cache.cache {
		if k == key {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Found %d entries for key %q, expected 1", count, key)
	}
}

// TestSafeCache_ExtendTTLOnAccess verifies that accessing a cache item extends its
// time-to-live by updating the lastAccessed timestamp, preventing premature expiration.
func TestSafeCache_ExtendTTLOnAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := NewSafeCache(ctx)
	ttl := 2 // seconds
	deletedChan := make(chan string, 10)

	key := "access-extends-ttl"
	value := []byte("test-value")

	cache.Set(key, value)
	item := cache.cache[key]

	// Set lastAccessed to be almost expired
	item.mu.Lock()
	item.lastAccessed = time.Now().Add(-time.Duration(ttl-1) * time.Second)
	oldAccessTime := item.lastAccessed
	item.mu.Unlock()

	// Access the item, which should refresh the lastAccessed time
	_, _ = cache.Get(key)

	// Verify lastAccessed was updated
	item.mu.RLock()
	newAccessTime := item.lastAccessed
	item.mu.RUnlock()

	if !newAccessTime.After(oldAccessTime) {
		t.Errorf("lastAccessed time was not updated after Get operation")
	}

	// Run cleanup - item should not be deleted because lastAccessed was refreshed
	cache.cleanupExpiredItems(ttl, deletedChan)

	select {
	case deletedKey := <-deletedChan:
		t.Errorf("Key %q was incorrectly deleted despite recent access", deletedKey)
	case <-time.After(100 * time.Millisecond):
		// This is the expected path - no deletion
	}

	_, exists := cache.Get(key)
	if !exists {
		t.Errorf("Item was not accessible after cleanup, despite recent access")
	}
}
