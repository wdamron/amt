# package amt

Package `amt` implements the Hash Array Mapped Trie (HAMT) in Go (1.18+ generics).

See "Ideal Hash Trees" (Phil Bagwell, 2001).

The AMT implementation has a natural cardinality of 16 for the root trie and all sub-tries;
each AMT level is indexed by 4 hash bits. The depth of a map or set will be on the order of log16(N).

```go
import "github.com/wdamron/amt"
```

## More Info

* [Ideal Hash Trees (Phil Bagwell, 2001)](https://lampwww.epfl.ch/papers/idealhashtrees.pdf)
* [Docs (pkg.go.dev)](https://pkg.go.dev/github.com/wdamron/amt)
