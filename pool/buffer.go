package pool

import (
	"bytes"
	"sync"
)

// BufferPool reuses bytes.Buffer instances to reduce allocations.
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a BufferPool with the given initial capacity.
func NewBufferPool(initialCapacity int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, 0, initialCapacity))
			},
		},
	}
}

// Get returns a reset buffer from the pool.
func (p *BufferPool) Get() *bytes.Buffer {
	buf, ok := p.pool.Get().(*bytes.Buffer)
	if !ok {
		return bytes.NewBuffer(make([]byte, 0, 256))
	}
	buf.Reset()
	return buf
}

// Put returns a buffer to the pool unless it is too large.
func (p *BufferPool) Put(buf *bytes.Buffer) {
	if buf.Cap() > 16*1024*1024 {
		return
	}
	p.pool.Put(buf)
}

var (
	smallBufferPool  = NewBufferPool(256)
	mediumBufferPool = NewBufferPool(4 * 1024)
	largeBufferPool  = NewBufferPool(64 * 1024)
)

// GetBuffer returns a pooled buffer sized for the requested capacity bucket.
func GetBuffer(size int) *bytes.Buffer {
	switch {
	case size <= 256:
		return smallBufferPool.Get()
	case size <= 4*1024:
		return mediumBufferPool.Get()
	default:
		return largeBufferPool.Get()
	}
}

// PutBuffer returns a buffer to the matching size bucket.
func PutBuffer(buf *bytes.Buffer) {
	switch {
	case buf.Cap() <= 256:
		smallBufferPool.Put(buf)
	case buf.Cap() <= 4*1024:
		mediumBufferPool.Put(buf)
	default:
		largeBufferPool.Put(buf)
	}
}

// ByteSlicePool reuses byte slices to reduce allocations.
type ByteSlicePool struct {
	pool sync.Pool
}

// NewByteSlicePool creates a ByteSlicePool with the given initial capacity.
func NewByteSlicePool(initialCapacity int) *ByteSlicePool {
	return &ByteSlicePool{
		pool: sync.Pool{
			New: func() any {
				slice := make([]byte, initialCapacity)
				return &slice
			},
		},
	}
}

// Get returns a byte slice from the pool.
func (p *ByteSlicePool) Get() []byte {
	slice, ok := p.pool.Get().(*[]byte)
	if !ok {
		slice = new([]byte)
		*slice = make([]byte, 0, 256)
	}
	return *slice
}

// Put returns a byte slice to the pool unless it is too large.
func (p *ByteSlicePool) Put(slice []byte) {
	if cap(slice) > 16*1024*1024 {
		return
	}
	p.pool.Put(&slice)
}

var (
	smallByteSlicePool  = NewByteSlicePool(256)
	mediumByteSlicePool = NewByteSlicePool(4 * 1024)
	largeByteSlicePool  = NewByteSlicePool(64 * 1024)
)

// GetByteSlice returns a pooled byte slice for the requested size bucket.
func GetByteSlice(size int) []byte {
	switch {
	case size <= 256:
		return smallByteSlicePool.Get()
	case size <= 4*1024:
		return mediumByteSlicePool.Get()
	default:
		return largeByteSlicePool.Get()
	}
}

// PutByteSlice returns a byte slice to the matching size bucket.
func PutByteSlice(slice []byte) {
	switch {
	case cap(slice) <= 256:
		smallByteSlicePool.Put(slice)
	case cap(slice) <= 4*1024:
		mediumByteSlicePool.Put(slice)
	default:
		largeByteSlicePool.Put(slice)
	}
}

// StringSlicePool reuses string slices to reduce allocations.
type StringSlicePool struct {
	pool sync.Pool
}

// NewStringSlicePool creates a StringSlicePool with the given initial capacity.
func NewStringSlicePool(initialCapacity int) *StringSlicePool {
	return &StringSlicePool{
		pool: sync.Pool{
			New: func() any {
				slice := make([]string, 0, initialCapacity)
				return &slice
			},
		},
	}
}

// Get returns an empty string slice from the pool.
func (p *StringSlicePool) Get() []string {
	slice, ok := p.pool.Get().(*[]string)
	if !ok {
		slice = new([]string)
		*slice = make([]string, 0, 16)
	}
	return (*slice)[:0]
}

// Put returns a string slice to the pool unless it is too large.
func (p *StringSlicePool) Put(slice []string) {
	if cap(slice) > 1024 {
		return
	}
	p.pool.Put(&slice)
}

var stringSlicePool = NewStringSlicePool(16)

// GetStringSlice returns a pooled string slice.
func GetStringSlice() []string {
	return stringSlicePool.Get()
}

// PutStringSlice returns a string slice to the shared pool.
func PutStringSlice(slice []string) {
	stringSlicePool.Put(slice)
}
