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
// See "Ideal Hash Trees" (Phil Bagwell, 2001).
//
// The AMT implementation has a natural cardinality of 16 for the root trie and all sub-tries;
// each AMT level is indexed by 4 hash bits. The depth of a map or set will be on the order of log16(N).
package amt

import (
	"hash/maphash"
	"unsafe"
)

// root contains the root level of a map or set. Each root allocation is 512 bytes,
// typically 8 cache lines on 64-bit architectures. Multiples of 64 bytes will likely
// be 64-byte (cache) aligned by the memory allocator.
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

// link is an Array Mapped Trie (AMT) level with up to 16 items
// or a key-value pointer within a level.
type link struct {
	ptr  unsafe.Pointer // *[4|8|12|16]link | *kv
	pmap uint32         // uint16 ptr table presence bits
	tmap uint32         // uint16 ptr table type bits (0: *[...]link, 1: *kv)
}

const linkSize = unsafe.Sizeof(link{})

// Allocate an array of 4, 8, 12, or 16 links. Each block of 4 links is 64 bytes,
// a typical cache line on 64-bit architectures. Multiples of 64 bytes will likely
// be 64-byte (cache) aligned by the memory allocator.
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
