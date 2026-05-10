package espresso

import (
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// makeDistinctTypes returns n reflect.Type values that the runtime treats
// as distinct. Each is a function type returning a unique anonymous struct
// whose only field name embeds the iteration index, guaranteeing identity
// uniqueness across the slice (Go's type system distinguishes structs by
// field name + type + tag).
func makeDistinctTypes(n int) []reflect.Type {
	types := make([]reflect.Type, 0, n)
	intT := reflect.TypeFor[int]()
	for i := range n {
		fields := []reflect.StructField{
			{Name: "Field" + strconv.Itoa(i), Type: intT},
		}
		structT := reflect.StructOf(fields)
		funcT := reflect.FuncOf(nil, []reflect.Type{structT}, false)
		types = append(types, funcT)
	}
	// Defensive: confirm uniqueness so a reflect-interning surprise fails
	// loudly in test setup rather than as a flaky assertion downstream.
	seen := make(map[reflect.Type]bool, n)
	for _, t := range types {
		if seen[t] {
			panic("makeDistinctTypes produced a duplicate; test setup is broken")
		}
		seen[t] = true
	}
	return types
}

func TestBoundedHandlerCache_LoadStoreHit(t *testing.T) {
	cache := newBoundedHandlerCache(8)
	types := makeDistinctTypes(3)

	for _, ty := range types {
		cache.Store(ty, &handlerInfo{numIn: 0})
	}

	for _, ty := range types {
		got, ok := cache.Load(ty)
		if !ok {
			t.Errorf("Load(%v) miss, expected hit", ty)
		}
		if got == nil {
			t.Error("Load returned nil *handlerInfo on hit")
		}
	}
	if cache.Len() != 3 {
		t.Errorf("Len = %d, want 3", cache.Len())
	}
}

func TestBoundedHandlerCache_EvictsLRU(t *testing.T) {
	const max = 4
	const total = 10
	cache := newBoundedHandlerCache(max)

	var evicted []reflect.Type
	var mu sync.Mutex
	cache.SetOnEvict(func(t reflect.Type) {
		mu.Lock()
		defer mu.Unlock()
		evicted = append(evicted, t)
	})

	types := makeDistinctTypes(total)
	for _, ty := range types {
		cache.Store(ty, &handlerInfo{numIn: 0})
	}

	if cache.Len() != max {
		t.Errorf("Len = %d, want %d after %d inserts into cap-%d cache", cache.Len(), max, total, max)
	}
	mu.Lock()
	got := len(evicted)
	mu.Unlock()
	if got != total-max {
		t.Errorf("eviction count = %d, want %d", got, total-max)
	}

	// The first (total - max) types should have been evicted (LRU).
	for i := range total - max {
		if _, ok := cache.Load(types[i]); ok {
			t.Errorf("types[%d] should be evicted but was found", i)
		}
	}
	// The last max types should still be present.
	for i := total - max; i < total; i++ {
		if _, ok := cache.Load(types[i]); !ok {
			t.Errorf("types[%d] should be present but was not found", i)
		}
	}
}

func TestBoundedHandlerCache_LoadRefreshesLRUOrder(t *testing.T) {
	const max = 3
	cache := newBoundedHandlerCache(max)
	types := makeDistinctTypes(4)

	// Fill the cache.
	for i := range max {
		cache.Store(types[i], &handlerInfo{numIn: 0})
	}

	// Touch types[0] — promotes it to most-recently-used. LRU is now [1, 2, 0].
	if _, ok := cache.Load(types[0]); !ok {
		t.Fatal("types[0] should be present")
	}

	// Insert types[3] — should evict types[1] (the new LRU), not types[0].
	var evicted reflect.Type
	cache.SetOnEvict(func(t reflect.Type) { evicted = t })
	cache.Store(types[3], &handlerInfo{numIn: 0})

	if evicted != types[1] {
		t.Errorf("evicted = %v, want types[1] = %v", evicted, types[1])
	}
	if _, ok := cache.Load(types[0]); !ok {
		t.Error("types[0] should still be present (was promoted before eviction)")
	}
}

func TestBoundedHandlerCache_StoreUpdatesExisting(t *testing.T) {
	cache := newBoundedHandlerCache(4)
	types := makeDistinctTypes(1)
	t0 := types[0]

	first := &handlerInfo{numIn: 1}
	cache.Store(t0, first)

	second := &handlerInfo{numIn: 2}
	cache.Store(t0, second)

	got, ok := cache.Load(t0)
	if !ok || got != second {
		t.Errorf("Store(existing) did not refresh value; got %v want %v", got, second)
	}
	if cache.Len() != 1 {
		t.Errorf("Len = %d, want 1 after two Stores of same key", cache.Len())
	}
}

func TestBoundedHandlerCache_OnEvictNilSafe(t *testing.T) {
	cache := newBoundedHandlerCache(2)
	cache.SetOnEvict(nil) // explicit nil
	types := makeDistinctTypes(3)
	// Should not panic with no hook installed.
	for _, ty := range types {
		cache.Store(ty, &handlerInfo{numIn: 0})
	}
	cache.SetOnEvict(func(t reflect.Type) {})
	cache.SetOnEvict(nil) // re-clear
	for _, ty := range types {
		cache.Store(ty, &handlerInfo{numIn: 0})
	}
}

func TestBoundedHandlerCache_SetMaxSizeShrinks(t *testing.T) {
	cache := newBoundedHandlerCache(8)
	types := makeDistinctTypes(8)
	for _, ty := range types {
		cache.Store(ty, &handlerInfo{numIn: 0})
	}
	if cache.Len() != 8 {
		t.Fatalf("setup failed: Len = %d, want 8", cache.Len())
	}

	var evicted []reflect.Type
	cache.SetOnEvict(func(t reflect.Type) { evicted = append(evicted, t) })

	cache.SetMaxSize(3)

	if cache.Len() != 3 {
		t.Errorf("Len after shrink = %d, want 3", cache.Len())
	}
	if len(evicted) != 5 {
		t.Errorf("evictions on shrink = %d, want 5", len(evicted))
	}
}

func TestBoundedHandlerCache_SetMaxSizeGrows(t *testing.T) {
	cache := newBoundedHandlerCache(2)
	all := makeDistinctTypes(10)

	// First two entries fill the cache; remaining 8 would normally evict.
	for _, ty := range all[:2] {
		cache.Store(ty, &handlerInfo{numIn: 0})
	}
	cache.SetMaxSize(10) // grow before more inserts so nothing evicts.

	for _, ty := range all[2:] {
		cache.Store(ty, &handlerInfo{numIn: 0})
	}

	if cache.Len() != 10 {
		t.Errorf("Len after grow + fills = %d, want 10", cache.Len())
	}
}

// TestBoundedHandlerCache_ConcurrentStoreLoad exercises the cache under
// concurrent registration and reads. Run with -race.
func TestBoundedHandlerCache_ConcurrentStoreLoad(t *testing.T) {
	const goroutines = 16
	const opsPerG = 200
	cache := newBoundedHandlerCache(64)
	types := makeDistinctTypes(128)

	var hits atomic.Int64
	cache.SetOnEvict(func(t reflect.Type) { /* no-op, just exercise the hook */ })

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range opsPerG {
				ty := types[(g*opsPerG+i)%len(types)]
				if i%2 == 0 {
					cache.Store(ty, &handlerInfo{numIn: g})
				} else {
					if _, ok := cache.Load(ty); ok {
						hits.Add(1)
					}
				}
			}
		}(g)
	}
	wg.Wait()

	// We don't assert on hit count — too flaky under interleaving. The
	// test's purpose is to surface races and panics, which -race catches.
	if cache.Len() > 64 {
		t.Errorf("Len = %d exceeded maxSize=64", cache.Len())
	}
}

// TestBoundedHandlerCache_PackageLevelSetters drives the public API surface.
// Saves and restores the package-global cache state so it doesn't leak.
func TestBoundedHandlerCache_PackageLevelSetters(t *testing.T) {
	t.Cleanup(func() {
		SetHandlerCacheSize(DefaultHandlerCacheSize)
		OnHandlerCacheEvict(nil)
	})

	var evicted atomic.Int64
	OnHandlerCacheEvict(func(t reflect.Type) { evicted.Add(1) })

	SetHandlerCacheSize(2)
	types := makeDistinctTypes(5)
	for _, ty := range types {
		handlerCache.Store(ty, &handlerInfo{numIn: 0})
	}

	if got := evicted.Load(); got < 3 {
		t.Errorf("evictions through public hook = %d, want >= 3 (5 inserts into cap-2 cache)", got)
	}
}

// TestSetHandlerCacheSize_ZeroResetsToDefault locks the documented behavior.
func TestSetHandlerCacheSize_ZeroResetsToDefault(t *testing.T) {
	t.Cleanup(func() { SetHandlerCacheSize(DefaultHandlerCacheSize) })
	SetHandlerCacheSize(50)
	SetHandlerCacheSize(0)
	if handlerCache.maxSize != DefaultHandlerCacheSize {
		t.Errorf("maxSize after Set(0) = %d, want %d", handlerCache.maxSize, DefaultHandlerCacheSize)
	}
}
