# package amt

Package `amt` implements the Hash Array Mapped Trie (HAMT) in Go (1.18+ generics).

See "Ideal Hash Trees" (Phil Bagwell, 2001) for an overview of the implementation, advantages,
and disadvantages of HAMTs.

The AMT implementation has a natural cardinality of 16 for the root trie and all sub-tries;
each AMT level is indexed by 4 hash bits. The depth of a map or set will be on the order of log16(N).

**This package uses unsafe pointers/pointer-arithmetic extensively**, so it is inherently unsafe and not guaranteed
to work in all cases. Unsafe pointers enable a compact memory layout, fewer allocations, and effectively reduce
the depth of a map or set by reducing the number of pointers dereferenced along the path to a key or value.

An alternative approach, using an interface type to represent either a key-value pair or entry slice (sub-trie),
has a few drawbacks. Interface values are the size of 2 pointers (versus 1 when using unsafe pointers),
which would increase the memory overhead for key-value/sub-trie entries by 50% (24 bytes versus 16 bytes).
If the interface value is assigned a slice of entries (sub-trie), a new allocation (24 bytes) is required
for the slice-header before it can be wrapped into the interface value. Accessing an entry slice (sub-trie)
through an interface value requires (1) dereferencing the interface's data pointer to get to the slice-header
(among other things), then (2) dereferencing the slice-header's data pointer to access an entry in the slice.
Unsafe pointers eliminate the extra allocation and overhead of (1), allowing entries to point directly
to either a key-value struct or an array of entries. Generics enable a type-safe implementation, where the
key-value type of a map or set is fixed after instantiation.

```go
import "github.com/wdamron/amt"
```

## More Info

* [Ideal Hash Trees (Phil Bagwell, 2001)](https://lampwww.epfl.ch/papers/idealhashtrees.pdf)
* [Docs (pkg.go.dev)](https://pkg.go.dev/github.com/wdamron/amt)
