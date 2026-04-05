package pool

import (
	"bytes"
	"sync"
	"testing"
)

func TestBufferPool_GetPut(t *testing.T) {
	pool := NewBufferPool(256)

	buf := pool.Get()
	if buf == nil {
		t.Error("expected non-nil buffer")
	}
	if buf.Len() != 0 {
		t.Error("expected empty buffer")
	}

	data := []byte("test data")
	n, err := buf.Write(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}

	pool.Put(buf)

	buf = pool.Get()
	if buf.Len() != 0 {
		t.Error("expected empty buffer after pool return")
	}

	pool.Put(buf)
}

func TestBufferPool_LargeBuffers(t *testing.T) {
	pool := NewBufferPool(1024)

	largeBuf := bytes.NewBuffer(make([]byte, 17*1024*1024))

	pool.Put(largeBuf)

	buf := pool.Get()
	if buf == nil {
		t.Error("expected non-nil buffer")
	}
	pool.Put(buf)
}

func TestGetBuffer_SizeSelection(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"small size", 100},
		{"medium size", 1024},
		{"large size", 10 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := GetBuffer(tt.size)
			if buf == nil {
				t.Error("expected non-nil buffer")
			}
			if buf.Len() != 0 {
				t.Error("expected empty buffer")
			}
			PutBuffer(buf)
		})
	}
}

func TestByteSlicePool_GetPut(t *testing.T) {
	pool := NewByteSlicePool(256)

	slice := pool.Get()
	if slice == nil {
		t.Error("expected non-nil slice")
	}
	if len(slice) != cap(slice) {
		t.Errorf("expected length %d to equal capacity %d", len(slice), cap(slice))
	}

	copy(slice, "test")

	pool.Put(slice)

	slice = pool.Get()
	if cap(slice) < 256 {
		t.Errorf("expected capacity >= 256, got %d", cap(slice))
	}

	pool.Put(slice)
}

func TestByteSlicePool_LargeSlices(t *testing.T) {
	pool := NewByteSlicePool(1024)

	largeSlice := make([]byte, 17*1024*1024)

	pool.Put(largeSlice)

	slice := pool.Get()
	if slice == nil {
		t.Error("expected non-nil slice")
	}
	pool.Put(slice)
}

func TestGetByteSlice_SizeSelection(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"small size", 100},
		{"medium size", 1024},
		{"large size", 10 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice := GetByteSlice(tt.size)
			if slice == nil {
				t.Error("expected non-nil slice")
			}
			PutByteSlice(slice)
		})
	}
}

func TestStringSlicePool_GetPut(t *testing.T) {
	pool := NewStringSlicePool(16)

	slice := pool.Get()
	if slice == nil {
		t.Error("expected non-nil slice")
	}
	if len(slice) != 0 {
		t.Error("expected empty slice")
	}
	if cap(slice) < 16 {
		t.Errorf("expected capacity >= 16, got %d", cap(slice))
	}

	slice = append(slice, "a", "b", "c")

	pool.Put(slice)

	slice = pool.Get()
	if len(slice) != 0 {
		t.Error("expected empty slice after pool return")
	}

	pool.Put(slice)
}

func TestStringSlicePool_LargeSlices(t *testing.T) {
	pool := NewStringSlicePool(16)

	largeSlice := make([]string, 1025)

	pool.Put(largeSlice)

	slice := pool.Get()
	if slice == nil {
		t.Error("expected non-nil slice")
	}
	pool.Put(slice)
}

func TestGetStringSlice_PutStringSlice(t *testing.T) {
	slice := GetStringSlice()
	if slice == nil {
		t.Error("expected non-nil slice")
	}
	if len(slice) != 0 {
		t.Error("expected empty slice")
	}

	slice = append(slice, "test1", "test2")

	PutStringSlice(slice)

	slice = GetStringSlice()
	if len(slice) != 0 {
		t.Error("expected empty slice")
	}

	PutStringSlice(slice)
}

func TestBufferPool_Concurrent(t *testing.T) {
	pool := NewBufferPool(256)

	var wg sync.WaitGroup
	const numGoroutines = 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			buf := pool.Get()
			_, _ = buf.WriteString("test")
			pool.Put(buf)
		}()
	}

	wg.Wait()
}

func TestByteSlicePool_Concurrent(t *testing.T) {
	pool := NewByteSlicePool(256)

	var wg sync.WaitGroup
	const numGoroutines = 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			slice := pool.Get()
			_ = slice[0:10]
			pool.Put(slice)
		}()
	}

	wg.Wait()
}

func TestStringSlicePool_Concurrent(t *testing.T) {
	pool := NewStringSlicePool(16)

	var wg sync.WaitGroup
	const numGoroutines = 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			slice := pool.Get()
			slice = append(slice, "test")
			pool.Put(slice)
		}()
	}

	wg.Wait()
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool(256)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		_, _ = buf.WriteString("test data")
		pool.Put(buf)
	}
}

func BenchmarkBufferPool_NewBuffer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(make([]byte, 0, 256))
		_, _ = buf.WriteString("test data")
		_ = buf
	}
}

func BenchmarkByteSlicePool_GetPut(b *testing.B) {
	pool := NewByteSlicePool(256)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slice := pool.Get()
		copy(slice, "test")
		pool.Put(slice)
	}
}

func BenchmarkByteSlicePool_NewSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slice := make([]byte, 256)
		copy(slice, "test")
		_ = slice
	}
}

func BenchmarkStringSlicePool_GetPut(b *testing.B) {
	pool := NewStringSlicePool(16)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slice := pool.Get()
		slice = append(slice, "test")
		pool.Put(slice)
	}
}

func BenchmarkStringSlicePool_NewSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		slice := make([]string, 0, 16)
		slice = append(slice, "test")
		_ = slice
	}
}
