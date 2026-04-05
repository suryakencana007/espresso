// Package pool provides object pooling utilities for memory optimization.
//
// Object pools reduce garbage collection pressure by reusing allocated objects.
// This is particularly useful for high-throughput scenarios where objects are
// frequently allocated and discarded.
//
// Example:
//
//	buf := pool.GetBuffer(256)
//	defer pool.PutBuffer(buf)
//	buf.WriteString("Hello, World!")
package pool
