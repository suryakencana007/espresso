package espresso

import (
	"container/list"
	"reflect"
	"sync"
	"sync/atomic"
)

// DefaultHandlerCacheSize is the upper bound on the package-level reflection
// cache used by Handler() when no other size has been set. The reflection
// cache memoizes signature analysis per distinct handler reflect.Type so
// repeated registrations of the same handler shape skip the parse.
//
// 1024 is sized for the typical app: routes registered at startup, no
// dynamic registration, well under the bound. Apps that synthesize handler
// types at runtime (plugin hosts, per-tenant codegen, reflect.MakeFunc
// scenarios) should call SetHandlerCacheSize with a value matching their
// expected working set, and OnHandlerCacheEvict to observe eviction
// pressure.
const DefaultHandlerCacheSize = 1024

// boundedHandlerCache is an LRU-bounded reflection cache. When Store would
// exceed maxSize, the least-recently-used entry is evicted and (if set) the
// onEvict hook fires synchronously with the evicted reflect.Type.
//
// In-flight requests are unaffected by eviction: handlerInfo values are
// immutable after creation, and request-side handlers hold the *handlerInfo
// pointer directly via closure (captured at registration). Eviction only
// drops the cache's reference; any pointer the caller already obtained
// continues to function until garbage-collected.
//
// The cache is package-global. Sibling Routers share the same cache (the
// reflect.Type key carries enough identity that this is correct — two
// routers registering the same handler shape benefit from each other's
// cache hit). For per-Router config of the cache, see SetHandlerCacheSize.
type boundedHandlerCache struct {
	mu      sync.Mutex
	maxSize int
	ll      *list.List // front = most-recently-used, *cacheEntry
	entries map[reflect.Type]*list.Element
	onEvict atomic.Pointer[evictHook]
}

// cacheEntry is the value stored in the LRU list element.
type cacheEntry struct {
	key reflect.Type
	val *handlerInfo
}

// evictHook is wrapped in atomic.Pointer so the hook can be replaced
// concurrently without locking the cache.
type evictHook func(reflect.Type)

func newBoundedHandlerCache(maxSize int) *boundedHandlerCache {
	if maxSize <= 0 {
		maxSize = DefaultHandlerCacheSize
	}
	return &boundedHandlerCache{
		maxSize: maxSize,
		ll:      list.New(),
		entries: make(map[reflect.Type]*list.Element, maxSize),
	}
}

// Load returns the cached *handlerInfo for k, marking it as most-recently-used
// on hit. The bool is false when k is not present.
func (c *boundedHandlerCache) Load(k reflect.Type) (*handlerInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[k]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).val, true //nolint:errcheck // type guaranteed at insertion
}

// Store inserts or refreshes (k, v). If insertion would exceed maxSize,
// the least-recently-used entry is evicted; the onEvict hook (if set)
// fires synchronously with the evicted key.
func (c *boundedHandlerCache) Store(k reflect.Type, v *handlerInfo) {
	c.mu.Lock()

	if el, ok := c.entries[k]; ok {
		// Existing entry: refresh value + move to front.
		el.Value.(*cacheEntry).val = v //nolint:errcheck // type guaranteed at insertion
		c.ll.MoveToFront(el)
		c.mu.Unlock()
		return
	}

	// New entry. Evict if at capacity.
	var evicted reflect.Type
	if c.ll.Len() >= c.maxSize {
		oldest := c.ll.Back()
		if oldest != nil {
			entry := oldest.Value.(*cacheEntry) //nolint:errcheck // type guaranteed at insertion
			evicted = entry.key
			c.ll.Remove(oldest)
			delete(c.entries, entry.key)
		}
	}

	el := c.ll.PushFront(&cacheEntry{key: k, val: v})
	c.entries[k] = el
	c.mu.Unlock()

	// Fire eviction hook outside the lock to avoid pinning the cache
	// during user code; the hook may be slow.
	if evicted != nil {
		if hookPtr := c.onEvict.Load(); hookPtr != nil {
			(*hookPtr)(evicted)
		}
	}
}

// SetMaxSize updates the upper bound. If the new bound is smaller than the
// current size, the least-recently-used entries are evicted down to fit.
// Each eviction fires the onEvict hook.
func (c *boundedHandlerCache) SetMaxSize(n int) {
	if n <= 0 {
		n = DefaultHandlerCacheSize
	}
	c.mu.Lock()

	c.maxSize = n
	var evictedKeys []reflect.Type
	for c.ll.Len() > n {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(*cacheEntry) //nolint:errcheck // type guaranteed at insertion
		evictedKeys = append(evictedKeys, entry.key)
		c.ll.Remove(oldest)
		delete(c.entries, entry.key)
	}
	c.mu.Unlock()

	if len(evictedKeys) > 0 {
		if hookPtr := c.onEvict.Load(); hookPtr != nil {
			for _, k := range evictedKeys {
				(*hookPtr)(k)
			}
		}
	}
}

// SetOnEvict registers (or replaces) the eviction callback. Pass nil to
// clear. The callback fires synchronously after each eviction outside the
// cache mutex.
func (c *boundedHandlerCache) SetOnEvict(fn func(reflect.Type)) {
	if fn == nil {
		c.onEvict.Store(nil)
		return
	}
	hook := evictHook(fn)
	c.onEvict.Store(&hook)
}

// Len returns the number of entries currently held. Primarily for tests.
func (c *boundedHandlerCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

// SetHandlerCacheSize updates the upper bound on the package-level handler
// reflection cache. The default is DefaultHandlerCacheSize (1024).
//
// Pass 0 or a negative value to reset to the default. Lowering the bound
// below the current size triggers eviction of the coldest entries.
//
// This is package-global state. Apps that have a clear "configure once at
// startup" boundary should call this from main(); apps that want different
// limits per Router will need to wait for the cache-per-Router move
// (currently out of scope for v2.0).
func SetHandlerCacheSize(n int) {
	handlerCache.SetMaxSize(n)
}

// OnHandlerCacheEvict registers a callback fired once per cache eviction
// with the evicted reflect.Type. Pass nil to clear. The callback runs
// synchronously outside the cache lock; keep it cheap (e.g., metric
// increment) to avoid blocking subsequent registrations.
//
// Use cases:
//   - Plugin hosts that want to log when handler signatures churn.
//   - Production observability — alert if eviction rate climbs (signals
//     dynamic-registration patterns the operator may not have planned for).
func OnHandlerCacheEvict(fn func(reflect.Type)) {
	handlerCache.SetOnEvict(fn)
}
