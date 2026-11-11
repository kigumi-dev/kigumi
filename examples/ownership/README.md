# ownership

Ownership and memory: resources and cleanup, moves, borrows, reference
counting, allocation and fixed-size storage.

## Prerequisites

None.

## Run

```
kigumi run examples/ownership
kigumi build examples/ownership -o ownership && ./ownership
```

## Test

```
kigumi test examples/ownership
```

There are no `_test.kg` blocks; the run plus `expect.txt` is the check.

## Expected

`expect.txt` records, in order: a railed resource closed and dropped on the
success path, `defer(fail)` firing only on the failing path, a partial move
followed by a field reinitialization, two disjoint `&mut` field borrows and a
read-only slice, `Box` use/move-out, `Shared`/`Weak` upgrade and a reference
cycle broken by `Weak`, a `noalloc` function and a `pure?` field holding both
pure and printing callbacks, nested arena scopes with reset/close, and
`FixedArray`/`BoundedArray` stack storage.

`pool/pool.kg` holds the `Box`/`Shared`/`Weak` pool and the single named
lifetime helper (`larger['a]`).

## Next

`data_tools` applies these guarantees to a real CLI program.
