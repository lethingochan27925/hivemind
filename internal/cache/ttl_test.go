package cache

import (
	"errors"
	"testing"
	"time"
)

func TestTTLServesFreshValueWithoutRefetching(t *testing.T) {
	c := NewTTL[int](time.Hour)
	calls := 0
	fetch := func() (int, time.Duration, error) {
		calls++
		return 42, 0, nil
	}
	for i := 0; i < 5; i++ {
		v, err := c.Get(fetch)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if v != 42 {
			t.Fatalf("Get returned %d, want 42", v)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times, want 1 (subsequent Gets should be served from cache)", calls)
	}
}

func TestTTLRefetchesAfterExpiry(t *testing.T) {
	c := NewTTL[int](10 * time.Millisecond)
	calls := 0
	fetch := func() (int, time.Duration, error) {
		calls++
		return calls, 0, nil
	}
	first, _ := c.Get(fetch)
	if first != 1 {
		t.Fatalf("first Get = %d, want 1", first)
	}
	time.Sleep(20 * time.Millisecond)
	second, _ := c.Get(fetch)
	if second != 2 {
		t.Fatalf("second Get (after TTL expiry) = %d, want 2 (a fresh fetch)", second)
	}
}

// TestTTLNeverCachesAnError is the regression this whole package exists to
// prevent: a transient failure (SSM eventually-consistent, a network blip)
// must not freeze the bad state for the cache's full TTL - the very next
// caller gets to try again immediately.
func TestTTLNeverCachesAnError(t *testing.T) {
	c := NewTTL[int](time.Hour)
	wantErr := errors.New("transient")
	calls := 0
	fetch := func() (int, time.Duration, error) {
		calls++
		if calls == 1 {
			return 0, 0, wantErr
		}
		return 99, 0, nil
	}
	if _, err := c.Get(fetch); !errors.Is(err, wantErr) {
		t.Fatalf("first Get error = %v, want %v", err, wantErr)
	}
	v, err := c.Get(fetch)
	if err != nil {
		t.Fatalf("second Get: unexpected error %v", err)
	}
	if v != 99 {
		t.Fatalf("second Get = %d, want 99 (the retry after the cached error)", v)
	}
	if calls != 2 {
		t.Fatalf("fetch called %d times, want 2 (no error should have been cached)", calls)
	}
}

// TestTTLNegativeTTLIsNotCached covers the "valid result, but don't cache
// it" case cloudcost.go and pipeline.go both need for degraded/passthrough
// responses.
func TestTTLNegativeTTLIsNotCached(t *testing.T) {
	c := NewTTL[string](time.Hour)
	calls := 0
	fetch := func() (string, time.Duration, error) {
		calls++
		return "degraded", -1, nil
	}
	first, err := c.Get(fetch)
	if err != nil || first != "degraded" {
		t.Fatalf("first Get = (%q, %v), want (\"degraded\", nil)", first, err)
	}
	second, err := c.Get(fetch)
	if err != nil || second != "degraded" {
		t.Fatalf("second Get = (%q, %v), want (\"degraded\", nil)", second, err)
	}
	if calls != 2 {
		t.Fatalf("fetch called %d times, want 2 (a ttl<0 result must never be served from cache)", calls)
	}
}

// TestTTLPositiveTTLOverridesDefault covers pricing.go's need: a "static"
// fallback price should be cached for a short fallbackTTL, not the full 12h
// cacheTTL a real Pricing API hit would earn.
func TestTTLPositiveTTLOverridesDefault(t *testing.T) {
	c := NewTTL[int](time.Hour)
	calls := 0
	fetch := func() (int, time.Duration, error) {
		calls++
		return calls, 5 * time.Millisecond, nil
	}
	first, _ := c.Get(fetch)
	if first != 1 {
		t.Fatalf("first Get = %d, want 1", first)
	}
	// Still within the 5ms override - should be served from cache.
	second, _ := c.Get(fetch)
	if second != 1 || calls != 1 {
		t.Fatalf("Get before override expiry re-fetched (calls=%d), want cached value", calls)
	}
	time.Sleep(10 * time.Millisecond)
	third, _ := c.Get(fetch)
	if third != 2 || calls != 2 {
		t.Fatalf("Get after override expiry = %d (calls=%d), want a fresh fetch", third, calls)
	}
}

func TestTTLConcurrentMissesCollapseIntoOneFetch(t *testing.T) {
	c := NewTTL[int](time.Hour)
	var calls int
	start := make(chan struct{})
	fetch := func() (int, time.Duration, error) {
		calls++
		<-start // block until every goroutine has piled up on the mutex
		return 7, 0, nil
	}

	// The first Get takes the mutex and blocks inside fetch; every other
	// concurrent Get must queue on the same mutex rather than running fetch
	// itself - proving the cache coalesces concurrent misses the way a
	// singleflight.Group would, per the type's doc comment.
	done := make(chan int, 10)
	go func() {
		v, _ := c.Get(fetch)
		done <- v
	}()
	time.Sleep(20 * time.Millisecond) // let the first Get take the lock and block

	for i := 0; i < 9; i++ {
		go func() {
			v, _ := c.Get(fetch)
			done <- v
		}()
	}
	time.Sleep(20 * time.Millisecond) // let the other 9 queue up on the mutex
	close(start)

	for i := 0; i < 10; i++ {
		if v := <-done; v != 7 {
			t.Fatalf("Get returned %d, want 7", v)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times, want exactly 1 for 10 concurrent misses", calls)
	}
}
