---
title: Pool API Reference
description: Object pooling types and functions
---

# Pool API Reference

Package `pool` provides object pooling utilities to reduce allocations.

```go
import "github.com/suryakencana007/espresso/pool"
```

## BufferPool

Reuse `bytes.Buffer` instances:

```go
type BufferPool struct { ... }

func NewBufferPool(initialCapacity int) *BufferPool
func (p *BufferPool) Get() *bytes.Buffer
func (p *BufferPool) Put(buf *bytes.Buffer)
```

### Global Buffer Pools

```go
func GetBuffer(size int) *bytes.Buffer
func PutBuffer(buf *bytes.Buffer)
```

Size buckets:
- Small: less than or equal to 256 bytes
- Medium: less than or equal to 4KB
- Large: greater than 4KB

Example:
```go
buf := pool.GetBuffer(1024)
defer pool.PutBuffer(buf)

buf.WriteString("Hello")
data := buf.Bytes()
```

## ByteSlicePool

Reuse `[]byte` slices:

```go
type ByteSlicePool struct { ... }

func NewByteSlicePool(initialCapacity int) *ByteSlicePool
func (p *ByteSlicePool) Get() []byte
func (p *ByteSlicePool) Put(slice []byte)
```

### Global Byte Slice Pools

```go
func GetByteSlice(size int) []byte
func PutByteSlice(slice []byte)
```

Example:
```go
data := pool.GetByteSlice(4096)
defer pool.PutByteSlice(data)

data = append(data, content...)
```

## StringSlicePool

Reuse `[]string` slices:

```go
type StringSlicePool struct { ... }

func NewStringSlicePool(initialCapacity int) *StringSlicePool
func (p *StringSlicePool) Get() []string
func (p *StringSlicePool) Put(slice []string)
```

### Global String Slice Functions

```go
func GetStringSlice() []string
func PutStringSlice(slice []string)
```

Example:
```go
tags := pool.GetStringSlice()
defer pool.PutStringSlice(tags)

tags = append(tags, "go", "http", "api")
```

## Limits

Pools discard objects larger than 16MB to prevent memory bloat:

```go
func (p *BufferPool) Put(buf *bytes.Buffer) {
    if buf.Cap() > 16*1024*1024 { // 16MB limit
        return // Discard
    }
    p.pool.Put(buf)
}
```

## Best Practices

1. Always use `defer` to return to pool
2. Don't use objects after `Put()`
3. Reset state if needed (pools call `Reset()` on `Get()` where applicable)
4. Match size buckets for efficiency

```go
// Good
buf := pool.GetBuffer(1024)
defer pool.PutBuffer(buf)
// use buf...

// Bad - using after Put
buf := pool.GetBuffer(1024)
pool.PutBuffer(buf)
buf.WriteString("oops") // Wrong! Buffer may be reused
```

## See Also

- [Object Pooling Guide](/guide/pooling) - Pooling patterns and performance