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
	"strconv"
	"testing"
)

func TestGeneric(t *testing.T) {
	bm := NewMap[Bytes, int]()
	bm.Set([]byte("k"), 1)
	if bm.Val([]byte("k")) != 1 {
		t.Fatal("value not set")
	}

	bs := NewSet[Bytes]()
	bs.Add([]byte("k"))
	if !bs.Has([]byte("k")) {
		t.Fatal("key not set")
	}
}

func TestNestedMap(t *testing.T) {
	m := NewStringMap[int]()
	mm := NewStringMap[StringMap[int]]()
	m.Set("v", 1)
	mm.Set("m", m)
	if mv := mm.Val("m"); mv.Nil() || mv.Len() != 1 {
		t.Fatalf("missing m")
	} else if v, ok := mv.Get("v"); !ok {
		t.Fatalf("missing v")
	} else if v != 1 {
		t.Fatalf("invalid v")
	}
	if mm.Val("m").Val("v") != 1 {
		t.Fatalf("invalid v")
	}
	if mm.Val("z").Val("v") != 0 {
		t.Fatalf("invalid zero value")
	}
}

func TestModify(t *testing.T) {
	m := NewStringMap[int]()
	m.Set("k", 1)
	m.Mod("k", func(v *int, ok bool) {
		if !ok {
			t.Fatal("not ok")
		}
		if *v != 1 {
			t.Fatal("not set")
		}
		*v = 2
	})
	if v, ok := m.Get("k"); !ok {
		t.Fatal("not set")
	} else if v != 2 {
		t.Fatal("not updated")
	}

	m.Mod("k2", func(v *int, ok bool) {
		if ok {
			t.Fatal("should not be ok")
		}
		if v == nil {
			t.Fatal("not allocated")
		}
		*v = 3
	})
	if v, ok := m.Get("k2"); !ok {
		t.Fatal("not set")
	} else if v != 3 {
		t.Fatal("not updated")
	}
}

func TestCanonicalStructure(t *testing.T) {
	const N = 1000 * 1000
	s1, s2 := NewIntSet(), NewIntSet()
	// Structures should be identical/canonical for a given hash seed:
	s2.seed = s1.seed
	for i := 0; i < N; i++ {
		s1.Add(IntKey(i))
		s2.Add(IntKey(i))
	}
	// If depths are identical, the structures are almost certainly identical:
	if s1.Dep() != s2.Dep() {
		t.Fatal("unequal depths")
	}
}

func TestStringMap(t *testing.T) {
	const N = 1000 * 1000
	m := NewStringMap[int]()

	if m.Nil() {
		t.Fatal("map nil after initialization")
	}

	for test := 0; test < 3; test++ {
		for i := 0; i < N; i++ {
			m.Set(strconv.Itoa(i), i)
		}
		for i := 0; i < N; i++ {
			if v := m.Ptr(strconv.Itoa(i)); v == nil {
				t.Fatalf("value not set (i=%d)", i)
			} else if *v != i {
				t.Fatalf("value invalid (i=%d, v=%d)", i, *v)
			}
		}
		if d := m.Dep(); d <= 1 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
		var visited int
		m.All(func(k string, v *int) bool {
			if v == nil {
				t.Fatal("value nil in callback")
			}
			visited++
			return true
		})
		if visited != N {
			t.Fatalf("invalid count %d", visited)
		}
		if l := m.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N/2; i++ {
			m.Del(strconv.Itoa(i))
		}
		for i := 0; i < N/2; i++ {
			if v := m.Ptr(strconv.Itoa(i)); v != nil {
				t.Fatalf("value not deleted (i=%d)", i)
			}
		}
		for i := N / 2; i < N; i++ {
			if v := m.Ptr(strconv.Itoa(i)); v == nil {
				t.Fatalf("value not set (i=%d)", i)
			} else if *v != i {
				t.Fatalf("value invalid (i=%d, v=%d)", i, *v)
			}
		}
		if l := m.Len(); l != N/2 {
			t.Fatalf("invalid len %d", l)
		}
		for i := 0; i < N/2; i++ {
			m.Set(strconv.Itoa(i), i)
		}
		for i := 0; i < N/2; i++ {
			if v := m.Ptr(strconv.Itoa(i)); v == nil {
				t.Fatalf("value not set (i=%d)", i)
			} else if *v != i {
				t.Fatalf("value invalid (i=%d, v=%d)", i, *v)
			}
		}
		if l := m.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N; i++ {
			m.Del(strconv.Itoa(i))
		}
		for i := 0; i < N; i++ {
			if v := m.Ptr(strconv.Itoa(i)); v != nil {
				t.Fatalf("value not deleted (i=%d)", i)
			}
		}
		if l := m.Len(); l != 0 {
			t.Fatalf("invalid len %d", l)
		}
		if d := m.Dep(); d != 0 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
	}
}

func TestStringSet(t *testing.T) {
	const N = 1000 * 1000
	s := NewStringSet()

	if s.Nil() {
		t.Fatal("set nil after initialization")
	}

	for test := 0; test < 3; test++ {
		for i := 0; i < N; i++ {
			s.Add(strconv.Itoa(i))
		}
		for i := 0; i < N; i++ {
			if !s.Has(strconv.Itoa(i)) {
				t.Fatalf("key not set (i=%d)", i)
			}
		}
		if d := s.Dep(); d <= 1 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
		var visited int
		s.All(func(k string) bool {
			visited++
			return true
		})
		if visited != N {
			t.Fatalf("invalid count %d", visited)
		}
		if l := s.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N/2; i++ {
			s.Del(strconv.Itoa(i))
		}
		for i := 0; i < N/2; i++ {
			if s.Has(strconv.Itoa(i)) {
				t.Fatalf("key not deleted (i=%d)", i)
			}
		}
		for i := N / 2; i < N; i++ {
			if !s.Has(strconv.Itoa(i)) {
				t.Fatalf("key not set (i=%d)", i)
			}
		}
		if l := s.Len(); l != N/2 {
			t.Fatalf("invalid len %d", l)
		}
		for i := 0; i < N/2; i++ {
			s.Add(strconv.Itoa(i))
		}
		for i := 0; i < N/2; i++ {
			if !s.Has(strconv.Itoa(i)) {
				t.Fatalf("key not set (i=%d)", i)
			}
		}
		if l := s.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N; i++ {
			s.Del(strconv.Itoa(i))
		}
		for i := 0; i < N; i++ {
			if s.Has(strconv.Itoa(i)) {
				t.Fatalf("key not deleted (i=%d)", i)
			}
		}
		if l := s.Len(); l != 0 {
			t.Fatalf("invalid len %d", l)
		}
		if d := s.Dep(); d != 0 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
	}
}

func TestIntMap(t *testing.T) {
	const N = 1000 * 1000
	m := NewIntMap[int]()

	if m.Nil() {
		t.Fatal("map nil after initialization")
	}

	for test := 0; test < 3; test++ {
		for i := 0; i < N; i++ {
			m.Set(IntKey(i), i)
		}
		for i := 0; i < N; i++ {
			if v := m.Ptr(IntKey(i)); v == nil {
				t.Fatalf("value not set (i=%d)", i)
			} else if *v != i {
				t.Fatalf("value invalid (i=%d, v=%d)", i, *v)
			}
		}
		if d := m.Dep(); d <= 1 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
		var visited int
		m.All(func(k IntKey, v *int) bool {
			if v == nil {
				t.Fatal("value nil in callback")
			}
			visited++
			return true
		})
		if visited != N {
			t.Fatalf("invalid count %d", visited)
		}
		if l := m.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N/2; i++ {
			m.Del(IntKey(i))
		}
		for i := 0; i < N/2; i++ {
			if v := m.Ptr(IntKey(i)); v != nil {
				t.Fatalf("value not deleted (i=%d)", i)
			}
		}
		for i := N / 2; i < N; i++ {
			if v := m.Ptr(IntKey(i)); v == nil {
				t.Fatalf("value not set (i=%d)", i)
			} else if *v != i {
				t.Fatalf("value invalid (i=%d, v=%d)", i, *v)
			}
		}
		if l := m.Len(); l != N/2 {
			t.Fatalf("invalid len %d", l)
		}
		for i := 0; i < N/2; i++ {
			m.Set(IntKey(i), i)
		}
		for i := 0; i < N/2; i++ {
			if v := m.Ptr(IntKey(i)); v == nil {
				t.Fatalf("value not set (i=%d)", i)
			} else if *v != i {
				t.Fatalf("value invalid (i=%d, v=%d)", i, *v)
			}
		}
		if l := m.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N; i++ {
			m.Del(IntKey(i))
		}
		for i := 0; i < N; i++ {
			if v := m.Ptr(IntKey(i)); v != nil {
				t.Fatalf("value not deleted (i=%d)", i)
			}
		}
		if l := m.Len(); l != 0 {
			t.Fatalf("invalid len %d", l)
		}
		if d := m.Dep(); d != 0 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
	}
}

func TestIntSet(t *testing.T) {
	const N = 1000 * 1000
	s := NewIntSet()

	if s.Nil() {
		t.Fatal("set nil after initialization")
	}

	for test := 0; test < 3; test++ {
		for i := 0; i < N; i++ {
			s.Add(IntKey(i))
		}
		for i := 0; i < N; i++ {
			if !s.Has(IntKey(i)) {
				t.Fatalf("key not set (i=%d)", i)
			}
		}
		if d := s.Dep(); d <= 1 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
		var visited int
		s.All(func(k IntKey) bool {
			visited++
			return true
		})
		if visited != N {
			t.Fatalf("invalid count %d", visited)
		}
		if l := s.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N/2; i++ {
			s.Del(IntKey(i))
		}
		for i := 0; i < N/2; i++ {
			if s.Has(IntKey(i)) {
				t.Fatalf("key not deleted (i=%d)", i)
			}
		}
		for i := N / 2; i < N; i++ {
			if !s.Has(IntKey(i)) {
				t.Fatalf("key not set (i=%d)", i)
			}
		}
		if l := s.Len(); l != N/2 {
			t.Fatalf("invalid len %d", l)
		}
		for i := 0; i < N/2; i++ {
			s.Add(IntKey(i))
		}
		for i := 0; i < N/2; i++ {
			if !s.Has(IntKey(i)) {
				t.Fatalf("key not set (i=%d)", i)
			}
		}
		if l := s.Len(); l != N {
			t.Fatalf("invalid len %d", l)
		}

		for i := 0; i < N; i++ {
			s.Del(IntKey(i))
		}
		for i := 0; i < N; i++ {
			if s.Has(IntKey(i)) {
				t.Fatalf("key not deleted (i=%d)", i)
			}
		}
		if l := s.Len(); l != 0 {
			t.Fatalf("invalid len %d", l)
		}
		if d := s.Dep(); d != 0 {
			t.Fatalf("depth invalid (d=%v)", d)
		}
	}
}
