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

// Key may be used as the key type of a generic Map[K, V].
type Key[K any] interface {
	// Equal must return true if the key is equal to a comparison key.
	Equal(K) bool
	// Hash must hash the key 1 or more times. The iter count indicates the number of times the key
	// must be rehashed. For the initial hash the iter count will be zero.
	Hash(seed maphash.Seed, iter uint) uint64
}

// HashBytes hashes key 1 or more times. The iter count indicates the number of times the key
// should be rehashed. For the initial hash the iter count will be zero. HashBytes may be used to
// implement the Hash method for the key type of a generic Map[K, V].
func HashBytes(key []byte, seed maphash.Seed, iter uint) uint64 {
	var hw maphash.Hash
	hw.SetSeed(seed)
	for i := uint8(0); i <= uint8(iter); i++ {
		hw.Write(key)
	}
	return hw.Sum64()
}

// Map maps hashable keys to values. Methods on a map value will panic if
// the map is not initialized. A map value is safe to copy.
type Map[K Key[K], V any] struct {
	*root
}
type kv[K Key[K], V any] struct {
	v V
	k K
}

// NewMap returns an initialized map. The map value is safe to copy.
func NewMap[K Key[K], V any]() Map[K, V] {
	return Map[K, V]{newRoot()}
}

// Nil returns true if m is not initialized.
func (m Map[K, V]) Nil() bool { return m.root == nil }

// Len returns the number of values in m. If m is not initialized, Len returns 0.
func (m Map[K, V]) Len() uint { return m.root.Len() }

// Dep returns the average (mean) depth of all values in m.
// If m is not initialized, Dep returns 0.
func (m Map[K, V]) Dep() float64 { return m.root.Dep() }

// Get returns the value for key, or a zero value and false if the key is missing.
func (m Map[K, V]) Get(key K) (value V, ok bool) {
	if ptr := m.Ptr(key); ptr != nil {
		value, ok = *ptr, true
	}
	return
}

// Val returns the value for key, or a zero value if the key is missing or m is not initialized.
func (m Map[K, V]) Val(key K) (value V) {
	if m.root != nil {
		if ptr := m.Ptr(key); ptr != nil {
			value = *ptr
		}
	}
	return
}

// Ptr returns a pointer to the value for key, or nil if the key is missing.
// The value may be updated through the returned pointer.
func (m Map[K, V]) Ptr(key K) *V {
	hd, l, d := key.Hash(m.seed, 0), &m.link, uint8(0)
	radix := uint8(hd & 0xF)
	bit := uint32(1) << radix
	idx := uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix))) & 0xF
	for l.pmap&bit != 0 { // item present
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize))
		if l.tmap&bit == 0 { // traverse branch
			l = item
			d++
			if d&0xF != 0 { // hash bits available
				hd >>= 4
			} else { // rehash
				hd = key.Hash(m.seed, uint(d>>4))
			}
			radix = uint8(hd & 0xF)
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))&0xF
			continue
		}
		if kv := (*kv[K, V])(item.ptr); key.Equal(kv.k) { // key match
			return &kv.v
		}
		return nil // key mismatch
	}
	return nil // item missing
}

// Set adds or updates the value for key.
func (m Map[K, V]) Set(key K, value V) {
	hd, l, d := key.Hash(m.seed, 0), &m.link, uint8(0)
	radix := uint8(hd & 0xF)
	bit := uint32(1) << radix
	idx := uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix))) & 0xF
	for l.pmap&bit != 0 { // item present
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize))
		if l.tmap&bit == 0 { // traverse branch
			l = item
			d++
			if d&0xF != 0 { // hash bits available
				hd >>= 4
			} else { // rehash
				hd = key.Hash(m.seed, uint(d>>4))
			}
			radix = uint8(hd & 0xF)
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))&0xF
			continue
		}
		ckv := (*kv[K, V])(item.ptr)
		ckey := ckv.k
		if key.Equal(ckey) { // update existing
			ckv.v = value
			return
		}
		// rehash conflicting key
		chd := ckey.Hash(m.seed, uint(d%(64/4))) >> (4 * (d % (64 / 4)))
		// replace with new branch until non-colliding
		l.tmap &^= bit
		m.dep -= uint64(d) // conflicting key depth
		for {
			d++
			if d&0xF != 0 { // hash bits available
				hd >>= 4
				chd >>= 4
			} else { // rehash
				hd, chd = key.Hash(m.seed, uint(d>>4)), ckey.Hash(m.seed, uint(d>>4))
			}
			kbit, cbit := uint32(1)<<uint8(hd&0xF), uint32(1)<<uint8(chd&0xF)
			item.pmap = kbit | cbit
			if kbit != cbit { // non-colliding
				item.tmap = item.pmap
				item.ptr = newLinkArray(2)
				kv := &kv[K, V]{value, key}
				if pair := (*[2]link)(item.ptr); kbit < cbit {
					pair[0].ptr, pair[1].ptr = unsafe.Pointer(kv), unsafe.Pointer(ckv)
				} else {
					pair[0].ptr, pair[1].ptr = unsafe.Pointer(ckv), unsafe.Pointer(kv)
				}
				m.len++
				m.dep += uint64(d) * 2
				return // item added
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
			ptr: unsafe.Pointer(&kv[K, V]{k: key, v: value}),
		}
	} else { // array full or empty
		src := l.ptr
		l.ptr = newLinkArray(count + 1)
		for before := uint8(0); before < idx; before++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(before)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(before)*linkSize))
		}
		*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize)) = link{
			ptr: unsafe.Pointer(&kv[K, V]{k: key, v: value}),
		}
		for after := idx; after < count; after++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after+1)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(after)*linkSize))
		}
	}
	l.pmap |= bit
	l.tmap |= bit
	m.len++
	m.dep += uint64(d)
}

// Mod modifies the value for key using the mod callback. The mod callback receives
// a pointer to the existing or new value for key, and true if the key existed.
func (m Map[K, V]) Mod(key K, mod func(*V, bool)) {
	hd, l, d := key.Hash(m.seed, 0), &m.link, uint8(0)
	radix := uint8(hd & 0xF)
	bit := uint32(1) << radix
	idx := uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix))) & 0xF
	for l.pmap&bit != 0 { // item present
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize))
		if l.tmap&bit == 0 { // traverse branch
			l = item
			d++
			if d&0xF != 0 { // hash bits available
				hd >>= 4
			} else { // rehash
				hd = key.Hash(m.seed, uint(d>>4))
			}
			radix = uint8(hd & 0xF)
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))&0xF
			continue
		}
		ckv := (*kv[K, V])(item.ptr)
		ckey := ckv.k
		if key.Equal(ckey) { // update existing
			mod(&ckv.v, true)
			return
		}
		// rehash conflicting key
		chd := key.Hash(m.seed, uint(d%(64/4))) >> (4 * (d % (64 / 4)))
		// replace with new branch until non-colliding
		l.tmap &^= bit
		m.dep -= uint64(d) // conflicting key depth
		for {
			d++
			if d&0xF != 0 { // hash bits available
				hd >>= 4
				chd >>= 4
			} else { // rehash
				hd, chd = key.Hash(m.seed, uint(d>>4)), ckey.Hash(m.seed, uint(d>>4))
			}
			kbit, cbit := uint32(1)<<uint8(hd&0xF), uint32(1)<<uint8(chd&0xF)
			item.pmap = kbit | cbit
			if kbit != cbit { // non-colliding
				item.tmap = item.pmap
				item.ptr = newLinkArray(2)
				kv := &kv[K, V]{k: key}
				mod(&kv.v, false)
				if pair := (*[2]link)(item.ptr); kbit < cbit {
					pair[0].ptr, pair[1].ptr = unsafe.Pointer(kv), unsafe.Pointer(ckv)
				} else {
					pair[0].ptr, pair[1].ptr = unsafe.Pointer(ckv), unsafe.Pointer(kv)
				}
				m.len++
				m.dep += uint64(d) * 2
				return // item added
			}
			// handle collision at new level
			item.ptr = newLinkArray(1)
			item = (*link)(item.ptr)
		}
	}
	kv := &kv[K, V]{k: key}
	mod(&kv.v, false)
	count := uint8(bits.OnesCount32(l.pmap))
	if (count != 0 && count%4 != 0) || d == 0 { // array slot available
		for after := int(count) - 1; after >= int(idx); after-- {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after+1)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after)*linkSize))
		}
		*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize)) = link{
			ptr: unsafe.Pointer(kv),
		}
	} else { // array full or empty
		src := l.ptr
		l.ptr = newLinkArray(count + 1)
		for before := uint8(0); before < idx; before++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(before)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(before)*linkSize))
		}
		*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize)) = link{
			ptr: unsafe.Pointer(kv),
		}
		for after := idx; after < count; after++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after+1)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(after)*linkSize))
		}
	}
	l.pmap |= bit
	l.tmap |= bit
	m.len++
	m.dep += uint64(d)
}

// Del deletes the value for key.
func (m Map[K, V]) Del(key K) {
	path := m.path[:0]
	hd, l, d := key.Hash(m.seed, 0), &m.link, uint8(0)
	radix := uint8(hd & 0xF)
	bit := uint32(1) << radix
	idx := uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix))) & 0xF
	for l.pmap&bit != 0 { // item present
		path = append(path, pathLink{radix, l})
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize))
		if l.tmap&bit == 0 { // traverse branch
			l = item
			d++
			if d&0xF != 0 { // hash bits available
				hd >>= 4
			} else { // rehash
				hd = key.Hash(m.seed, uint(d>>4))
			}
			radix = uint8(hd & 0xF)
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))&0xF
			continue
		}
		if !key.Equal((*kv[K, V])(item.ptr).k) { // key missing
			return
		}
		l.pmap &^= bit
		l.tmap &^= bit
		m.len--
		m.dep -= uint64(d)
		path[d].link = nil
		count := uint8(bits.OnesCount32(l.pmap))
		// unlink empty branches up to the root
		for count == 0 && d != 0 {
			l.ptr = nil
			d--
			l, radix = path[d].link, path[d].radix
			path[d].link = nil
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))&0xF
			l.pmap &^= bit
			l.tmap &^= bit
			count = uint8(bits.OnesCount32(l.pmap))
		}
		// shift items back
		src := l.ptr
		if count%4 == 0 && d != 0 {
			l.ptr = newLinkArray(count)
		}
		for before := uint8(0); before < idx; before++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(before)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(before)*linkSize))
		}
		for after := idx; after < count; after++ {
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(after)*linkSize)) =
				*(*link)(unsafe.Pointer(uintptr(src) + uintptr(after+1)*linkSize))
		}
		// replace single-valued branches with key-values up to the root
		for count == 1 && l.pmap == l.tmap && d != 0 {
			kv := (*[1]link)(l.ptr)[0].ptr // *kv
			m.dep--
			d--
			l, radix = path[d].link, path[d].radix
			path[d].link = nil
			bit, idx = 1<<radix, uint8(bits.OnesCount32(l.pmap&^(^uint32(0)<<radix)))&0xF
			l.tmap |= bit
			*(*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(idx)*linkSize)) = link{ptr: kv}
			count = uint8(bits.OnesCount32(l.pmap))
		}
		return // item removed
	}
}

// All ranges over values in m, applying the do callback to each value until
// the callback returns false or all values have been visited. The iteration order
// is not randomized for each call.
func (m Map[K, V]) All(do func(K, *V) bool) {
	mapScan(&m.link, do)
}

func mapScan[K Key[K], V any](l *link, do func(K, *V) bool) bool {
	pmap, tmap := l.pmap, l.tmap
	count := uint8(bits.OnesCount32(pmap))
	for i := uint8(0); i < count; i++ {
		bit := uint32(1) << uint8(bits.TrailingZeros32(pmap))
		item := (*link)(unsafe.Pointer(uintptr(l.ptr) + uintptr(i)*linkSize))
		if tmap&bit != 0 {
			kv := (*kv[K, V])(item.ptr)
			if !do(kv.k, &kv.v) {
				return false
			}
		} else if !mapScan(item, do) {
			return false
		}
		pmap &^= bit
	}
	return true
}
