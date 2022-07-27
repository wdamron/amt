// The MIT License (MIT)
//
// Copyright (c) 2022 West Damron
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package amt implements the Hash Array Mapped Trie (HAMT) in Go (1.18+ generics).
//
// See "Ideal Hash Trees" (Phil Bagwell, 2001) for an overview of the implementation, advantages,
// and disadvantages of HAMTs.
//
// The AMT implementation has a natural cardinality of 16 for the root trie and all sub-tries;
// each AMT level is indexed by 4 hash bits. The depth of a map or set will be on the order of log16(N).
//
// This package uses unsafe pointers/pointer-arithmetic extensively, so it is inherently unsafe and not guaranteed
// to work today or tomorrow. Unsafe pointers enable a compact memory layout with fewer allocations, and effectively reduce
// the depth of a map or set by reducing the number of pointers dereferenced along the path to a key or value.
// No attention is paid to 32-bit architectures since it's now the year 2000, but compatibility may still be there.
//
// An alternative approach, using an interface type to represent either a key-value pair or entry slice (sub-trie),
// has a few drawbacks. Interface values are the size of 2 pointers (versus 1 when using unsafe pointers),
// which would increase the memory overhead for key-value/sub-trie entries by 50% (e.g. 24 bytes versus 16 bytes
// on 64-bit architectures). If the interface value is assigned a slice of entries (sub-trie), a new allocation
// (the size of 3 pointers) is required for the slice-header before it can be wrapped into the interface value.
// Accessing an entry slice (sub-trie) through an interface value requires (1) dereferencing the interface's data
// pointer to get to the slice-header (among other things), then (2) dereferencing the slice-header's data pointer
// to access an entry in the slice. Unsafe pointers eliminate the extra allocation and overhead of (1), allowing
// entries to point directly to either a key-value struct or an array of entries. Generics enable a type-safe
// implementation, where the key-value type of a map or set is fixed after instantiation.
//
// The memory layouts of Go interfaces and slices are detailed in the following articles:
//
//   Go Data Structures: Interfaces (Russ Cox): https://research.swtch.com/interfaces
//   Go Slices: usage and internals (Andrew Gerrand): https://go.dev/blog/slices-intro
//   Go internals: invariance and memory layout of slices (Eli Bendersky): https://eli.thegreenplace.net/2021/go-internals-invariance-and-memory-layout-of-slices/
package amt

import (
	"hash/maphash"
	"unsafe"
)

// root contains the root level of a map or set. Each root allocation is 512 bytes
// on 64-bit architectures. Multiples of 64 bytes will likely be 64-byte (cache)
// aligned by the memory allocator. See runtime/sizeclasses.go.
type root struct {
	link
	seed  maphash.Seed
	len   uint64
	dep   uint64
	_     [3]uint64    // pad to 64-byte alignment
	items [16]link     // referenced by link
	path  [12]pathLink // scratch for traversal path during deletion
}

func newRoot() *root {
	r := &root{seed: maphash.MakeSeed()}
	r.link.ptr = unsafe.Pointer(&r.items)
	return r
}

// Len returns the number of values in r.
func (r *root) Len() uint {
	if r == nil {
		return 0
	}
	return uint(r.len)
}

// Dep returns the average (mean) depth of all values in r.
func (r *root) Dep() float64 {
	if r == nil || r.len == 0 {
		return 0
	}
	return float64(r.dep) / float64(r.len)
}

// link is an Array Mapped Trie (AMT) level with up to 16 items or a key-value
// pointer within a level.
//
// The "Ideal Hash Trees" paper suggests pointer-tagging to differentiate
// key-values and sub-tries. To avoid GC issues, the links instead
// contain 2 bitmaps: one indicating the presence of each item, and another
// indicating the type of each item. The bitmaps are 16-bit, but the bitmap
// fields are 32-bit so they may be used to store an inline key up to 64-bits
// (e.g. for integer maps/sets). If the bitmap fields were uint16, the size of
// the link struct would change from 16 bytes to 12 bytes on 64-bit architectures;
// this size reduction may be beneficial, though it would lead to unnatural
// alignment for a portion of the pointer fields in a link array; the performance
// of each approach has not yet been compared. As a minor detail, the compiler
// has the opportunity to emit 16-byte move instructions when copying links.
type link struct {
	ptr  unsafe.Pointer // *[4|8|12|16]link | *kv
	pmap uint32         // uint16 ptr table presence bits
	tmap uint32         // uint16 ptr table type bits (0: *[...]link, 1: *kv)
}

const linkSize = unsafe.Sizeof(link{})

// Allocate an array of 4, 8, 12, or 16 links. Each block of 4 links is 64-bytes
// on 64-bit architectures, which is a typical cache line on 64-bit architectures.
// Multiples of 64 bytes will likely be 64-byte (cache) aligned by the memory allocator.
// See runtime/sizeclasses.go.
func newLinkArray(capacity uint8) unsafe.Pointer {
	switch {
	case capacity <= 4:
		return unsafe.Pointer(new([4]link))
	case capacity <= 8:
		return unsafe.Pointer(new([8]link))
	case capacity <= 12:
		return unsafe.Pointer(new([12]link))
	default:
		return unsafe.Pointer(new([16]link))
	}
}

// pathLink references a branch traversed during deletion.
type pathLink struct {
	radix uint8
	*link
}
