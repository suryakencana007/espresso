# Object Pooling

Espresso uses object pooling to reduce allocations and improve performance in high-throughput applications.

## Overview

Object pooling reuses objects instead of creating new ones for each request. This reduces garbage collection pressure and improves throughput.

## Buffer Pools

### ByteBuffer Pool

```go
import "github.com/suryakencana007/espresso/pool"

func main() {
    // Create pool with initial capacity
    bufPool := pool.NewBufferPool(1024) // 1KB initial capacity
    
    // Get buffer from pool
    buf := bufPool.Get()
    defer bufPool.Put(buf) // Return to pool when done
    
    // Use buffer
    buf.WriteString("Hello")
    buf.Write(data)
    
    // Buffer is automatically reset when Get() is called
}
```

### Sized Buffer Pools

Espresso provides pre-sized pools for different use cases:

```go
// Small buffers (<= 256 bytes)
buf := pool.GetBuffer(100)   // Returns small buffer
pool.PutBuffer(buf)

// Medium buffers (<= 4KB)
buf := pool.GetBuffer(2000)   // Returns medium buffer
pool.PutBuffer(buf)

// Large buffers (> 4KB)
buf := pool.GetBuffer(10000)  // Returns large buffer
pool.PutBuffer(buf)
```

### Byte Slice Pool

```go
// For raw byte operations
slicePool := pool.NewByteSlicePool(1024)

slice := slicePool.Get()
defer slicePool.Put(slice)

// Use slice
slice = append(slice, data...)
```

### Global Byte Slice Functions

```go
// Get from appropriate sized pool
data := pool.GetByteSlice(1024)
defer pool.PutByteSlice(data)
```

## String Slice Pool

```go
// For accumulating strings
ssPool := pool.NewStringSlicePool(16)

slice := ssPool.Get()    // Returns empty slice
defer ssPool.Put(slice)

slice = append(slice, "item1", "item2", "item3")
```

### Global String Slice Functions

```go
slice := pool.GetStringSlice()
defer pool.PutStringSlice(slice)
```

## Extractor Pooling

Extractors implement `Reset()` for pooling:

```go
// JSON extractor pooling
var jsonPool = sync.Pool{
    New: func() any { return &espresso.JSON[MyRequest]{} },
}

func handler(ctx context.Context, rawReq *http.Request) (Response, error) {
    req := jsonPool.Get().(*espresso.JSON[MyRequest])
    defer func() {
        req.Reset()
        jsonPool.Put(req)
    }()
    
    if err := req.Extract(rawReq); err != nil {
        return nil, err
    }
    
    // Use req.Data
}
```

## When to Use Pooling

| Use Case | Should Pool? | Why |
|----------|-------------|-----|
| Small requests (< 1KB) | No | Allocation cost negligible |
| Large requests (> 64KB) | Yes | Reduces allocation overhead |
| High-throughput APIs | Yes | Reduces GC pressure |
| Streaming data | Yes | Buffers reused efficiently |
| One-off scripts | No | Pooling overhead > benefit |

## Pool Sizing

### Size Buckets

Espresso uses three size categories:

| Category | Size Range | Use Case |
|----------|------------|----------|
| Small | <= 256 bytes | Headers, small JSON |
| Medium | <= 4 KB | Typical API requests |
| Large | > 4 KB | File uploads, large payloads |

### Configuration

Create custom-sized pools:

```go
// For known payload sizes
smallPool := pool.NewBufferPool(256)   // Request headers
mediumPool := pool.NewBufferPool(4096)  // Standard requests
largePool := pool.NewBufferPool(65536)  // File uploads

// Get appropriate pool
func getPool(size int) *pool.BufferPool {
    switch {
    case size <= 256:
        return smallPool
    case size <= 4096:
        return mediumPool
    default:
        return largePool
    }
}
```

## Memory Limits

Pools automatically discard oversized buffers:

```go
// Buffers larger than 16MB are not returned to pool
// This prevents memory bloat
pool.PutBuffer(hugeBuffer) // Buffer discarded if cap > 16MB
```

## Performance Metrics

Benchmark with and without pooling:

```go
// Without pooling
func BenchmarkNoPool(b *testing.B) {
    for i := 0; i < b.N; i++ {
        buf := make([]byte, 1024)
        // use buf
        _ = buf
    }
}

// With pooling
func BenchmarkWithPool(b *testing.B) {
    pool := pool.NewByteSlicePool(1024)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buf := pool.Get()
        // use buf
        pool.Put(buf)
    }
}
```

Typical results show significant improvement:

```
BenchmarkNoPool-8          5000000   230 ns/op   1024 B/op   1 allocs/op
BenchmarkWithPool-8       50000000    25 ns/op      0 B/op   0 allocs/op
```

## Best Practices

1. **Always return to pool**: Use `defer` to ensure cleanup
2. **Don't hold references**: After Put(), don't use the object
3. **Match pool sizes**: Get from the right-sized pool
4. **Reset in Get()**: Pools reset objects before returning
5. **Limit pool growth**: Discard oversized objects

```go
// Good
buf := pool.GetBuffer(size)
defer pool.PutBuffer(buf)
// use buf...

// Bad - using after Put
buf := pool.GetBuffer(size)
pool.PutBuffer(buf)
buf.WriteString("oops") // Wrong!
```

## Integration with Middleware

Use pooling in custom middleware:

```go
func PooledMiddleware() httpmiddleware.Middleware {
    pool := pool.NewByteSlicePool(4096)
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            data := pool.Get()
            defer pool.Put(data)
            
            // Process request with pooled buffer
            // ...
            
            next.ServeHTTP(w, r)
        })
    }
}
```

## Debugging Pool Usage

Track pool statistics:

```go
type TrackedBufferPool struct {
    pool    sync.Pool
    gets    atomic.Int64
    puts    atomic.Int64
    created atomic.Int64
}

func (p *TrackedBufferPool) Get() *bytes.Buffer {
    p.gets.Add(1)
    // ...
}

func (p *TrackedBufferPool) Stats() (gets, puts, created int64) {
    return p.gets.Load(), p.puts.Load(), p.created.Load()
}
```

## Alternative: sync.Pool

For general-purpose pooling:

```go
var myPool = sync.Pool{
    New: func() any {
        return &MyObject{}
    },
}

obj := myPool.Get().(*MyObject)
defer myPool.Put(obj)

// Reset object
obj.Reset()
```

Note: `sync.Pool` may clear objects during GC, so always reset on Get().