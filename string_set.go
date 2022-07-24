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

package amt

import (
	"hash/maphash"
	"math/bits"
	"unsafe"
)

// StringSet contains a set of strings. Methods on a set value will panic if
// the set is not initialized. A set value is safe to copy.
type StringSet struct {
	*root
}

// NewStringSet returns an initialized set. The set value is safe to copy.
func NewStringSet() StringSet {
	return StringSet{newRoot()}
}

// Nil returns true if s is not initialized.
func (s StringSet) Nil() bool { return s.root == nil }

// Len returns the number of keys in s. If s is not initialized, Len returns 0.
func (s StringSet) Len() uint { return s.root.Len() }

// Dep returns the average (mean) depth of all keys in s.
// If s is not initialized, Dep returns 0.
func (s StringSet) Dep() float64 { return s.root.Dep() }

// Has returns true if s contains key.
func (s StringSet) Has(key string) bool {
	var hw maphash.Hash
	hw.SetSeed(s.seed)
	hw.WriteString(key)
	hd, l, d := hw.Sum64(), &s.link, uint8(0)
	radix := uint8(hd & 0xF)
	bit, idx := uint32(1)<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
	for l.pmap&bit != 0 { // item present
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize))
		if l.tmap&bit == 0 { // traverse branch
			l = item
			d++
			if d%(64/4) != 0 { // hash bits available
				hd >>= 4
			} else { // rehash
				hw.WriteString(key)
				hd = hw.Sum64()
			}
			radix = uint8(hd & 0xF)
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
			continue
		}
		if kv := (*strkv[struct{}])(item.ptr); kv.k == key { // key match
			return true
		}
		return false // key mismatch
	}
	return false // item missing
}

// Add adds key to s.
func (s StringSet) Add(key string) {
	var hw maphash.Hash
	hw.SetSeed(s.seed)
	hw.WriteString(key)
	hd, l, d := hw.Sum64(), &s.link, uint8(0)
	radix := uint8(hd & 0xF)
	bit, idx := uint32(1)<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
	for l.pmap&bit != 0 { // item present
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize))
		if l.tmap&bit == 0 { // traverse branch
			l = item
			d++
			if d%(64/4) != 0 { // hash bits available
				hd >>= 4
			} else { // rehash
				hw.WriteString(key)
				hd = hw.Sum64()
			}
			radix = uint8(hd & 0xF)
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
			continue
		}
		ckv := (*strkv[struct{}])(item.ptr)
		ckey := ckv.k
		if ckey == key { // exists
			return
		}
		// rehash conflicting key
		var chw maphash.Hash
		chw.SetSeed(s.seed)
		for cd := uint8(0); cd <= d; cd += (64 / 4) {
			chw.WriteString(ckey)
		}
		chd := chw.Sum64() >> (4 * (d % (64 / 4)))
		// replace with new branch until non-colliding
		l.tmap &^= bit
		s.dep -= uint64(d) // conflicting key depth
		for {
			d++
			if d%(64/4) != 0 { // hash bits available
				hd >>= 4
				chd >>= 4
			} else { // rehash keys
				hw.WriteString(key)
				chw.WriteString(ckey)
				hd, chd = hw.Sum64(), chw.Sum64()
			}
			kbit, cbit := uint32(1)<<uint8(hd&0xF), uint32(1)<<uint8(chd&0xF)
			item.pmap = kbit | cbit
			if kbit != cbit { // non-colliding
				item.tmap = item.pmap
				item.ptr = newLinkArray(2)
				kv := &strkv[struct{}]{k: key}
				if pair := (*[2]link)(item.ptr); kbit < cbit {
					pair[0].ptr, pair[1].ptr = unsafe.Pointer(kv), unsafe.Pointer(ckv)
				} else {
					pair[0].ptr, pair[1].ptr = unsafe.Pointer(ckv), unsafe.Pointer(kv)
				}
				s.len++
				s.dep += uint64(d) * 2
				return // key added
			}
			// handle collision at new level
			item.ptr = newLinkArray(1)
			item = (*link)(item.ptr)
		}
	}
	count := uint8(bits.OnesCount32(l.pmap))
	if (count != 0 && count%4 != 0) || d == 0 { // array slot available
		for after := int(count) - 1; after >= int(idx); after-- {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after+1)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after)*linkSize))
		}
		*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize)) = link{
			ptr: unsafe.Pointer(&strkv[struct{}]{k: key}),
		}
	} else { // array full or empty
		src := l.ptr
		l.ptr = newLinkArray(count + 1)
		for before := uint8(0); before < idx; before++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(before)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(before)*linkSize))
		}
		*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize)) = link{
			ptr: unsafe.Pointer(&strkv[struct{}]{k: key}),
		}
		for after := idx; after < count; after++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after+1)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(after)*linkSize))
		}
	}
	l.pmap |= bit
	l.tmap |= bit
	s.len++
	s.dep += uint64(d)
}

// Del deletes key from s.
func (s StringSet) Del(key string) {
	path := s.path[:0]
	var hw maphash.Hash
	hw.SetSeed(s.seed)
	hw.WriteString(key)
	hd, l, d := hw.Sum64(), &s.link, uint8(0)
	radix := uint8(hd & 0xF)
	bit, idx := uint32(1)<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
	for l.pmap&bit != 0 { // item present
		path = append(path, pathLink{radix, l})
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize))
		if l.tmap&bit == 0 { // traverse branch
			l = item
			d++
			if d%(64/4) != 0 { // hash bits available
				hd >>= 4
			} else { // rehash
				hw.WriteString(key)
				hd = hw.Sum64()
			}
			radix = uint8(hd & 0xF)
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
			continue
		}
		if (*strkv[struct{}])(item.ptr).k != key { // key missing
			return
		}
		l.pmap &^= bit
		l.tmap &^= bit
		s.len--
		s.dep -= uint64(d)
		path[d].link = nil
		count := uint8(bits.OnesCount32(l.pmap))
		// unlink empty branches up to the root
		for count == 0 && d != 0 {
			l.ptr = nil
			d--
			l, radix = path[d].link, path[d].radix
			path[d].link = nil
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
			l.pmap &^= bit
			l.tmap &^= bit
			count = uint8(bits.OnesCount32(l.pmap))
		}
		// shift items back
		src := l.ptr
		if count%4 == 0 && d != 0 { // copy all items when reallocating
			l.ptr = newLinkArray(count)
			for before := uint8(0); before < idx; before++ {
				*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(before)*linkSize)) =
					*(*link)(unsafe.Pointer(uintptr(src) + uintptr(before)*linkSize))
			}
		}
		for after := idx; after < count; after++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(after+1)*linkSize))
		}
		// replace single-valued branches with key-values up to the root
		for count == 1 && l.pmap == l.tmap && d != 0 {
			kv := (*[1]link)(l.ptr)[0].ptr // *kv
			s.dep--
			d--
			l, radix = path[d].link, path[d].radix
			path[d].link = nil
			bit, idx = uint32(1)<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))
			l.tmap |= bit
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize)) = link{ptr: kv}
			count = uint8(bits.OnesCount32(l.pmap))
		}
		// clear the path to prevent leaks
		for d != 0 {
			d--
			path[d].link = nil
		}
		return // item removed
	}
}

// All ranges over keys in s, applying the do callback to each key until
// the callback returns false or all keys have been visited. The iteration order
// is not randomized for each call.
func (s StringSet) All(do func(string) bool) {
	stringSetScan(&s.link, do)
}

func stringSetScan(l *link, do func(string) bool) bool {
	pmap, tmap := l.pmap, l.tmap
	count := uint8(bits.OnesCount32(pmap))
	for i := uint8(0); i < count; i++ {
		bit := uint32(1) << uint8(bits.TrailingZeros32(pmap))
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(i)*linkSize))
		if tmap&bit != 0 {
			kv := (*strkv[struct{}])(item.ptr)
			if !do(kv.k) {
				return false
			}
		} else if !stringSetScan(item, do) {
			return false
		}
		pmap &^= bit
	}
	return true
}
