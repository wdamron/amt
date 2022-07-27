# package amt

Package `amt` implements the Hash Array Mapped Trie (HAMT) in Go (1.18+ generics).

See "Ideal Hash Trees" (Phil Bagwell, 2001) for an overview of the implementation, advantages,
and disadvantages of HAMTs.

The AMT implementation has a natural cardinality of 16 for the root trie and all sub-tries;
each AMT level is indexed by 4 hash bits. The depth of a map or set will be on the order of log16(N).

**This package uses unsafe pointers/pointer-arithmetic extensively**, so it is inherently unsafe and not guaranteed
to work today or tomorrow. Unsafe pointers enable a compact memory layout with fewer allocations, and effectively reduce
the depth of a map or set by reducing the number of pointers dereferenced along the path to a key or value.
No attention is paid to 32-bit architectures since it's now the year 2000, but compatibility may still be there.

An alternative approach, using an interface type to represent either a key-value pair or entry slice (sub-trie),
has a few drawbacks. Interface values are the size of 2 pointers (versus 1 when using unsafe pointers),
which would increase the memory overhead for key-value/sub-trie entries by 50% (e.g. 24 bytes versus 16 bytes
on 64-bit architectures). If the interface value is assigned a slice of entries (sub-trie), a new allocation
(the size of 3 pointers) is required for the slice-header before it can be wrapped into the interface value. 
Accessing an entry slice (sub-trie) through an interface value requires _(1)_ dereferencing the interface's data 
pointer to get to the slice-header (among other things), then _(2)_ dereferencing the slice-header's data pointer 
to access an entry in the slice. Unsafe pointers eliminate the extra allocation and overhead of _(1)_, allowing 
entries to point directly to either a key-value struct or an array of entries. Generics enable a type-safe 
implementation, where the key-value type of a map or set is fixed after instantiation.

```go
import "github.com/wdamron/amt"
```

## More Info

* [Paper (PDF): Ideal Hash Trees (Phil Bagwell, 2001)](https://lampwww.epfl.ch/papers/idealhashtrees.pdf)
* [Docs (pkg.go.dev)](https://pkg.go.dev/github.com/wdamron/amt)
* [Hash Array Mapped Trie (Wikipedia)](https://en.wikipedia.org/wiki/Hash_array_mapped_trie)

The memory layouts of Go interfaces and slices are detailed in the following articles:

* [Go Data Structures: Interfaces (Russ Cox)](https://research.swtch.com/interfaces)
* [Go Slices: usage and internals (Andrew Gerrand)](https://go.dev/blog/slices-intro)
* [Go internals: invariance and memory layout of slices (Eli Bendersky)](https://eli.thegreenplace.net/2021/go-internals-invariance-and-memory-layout-of-slices/)
