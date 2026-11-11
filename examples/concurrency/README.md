# concurrency

Async tasks, structured children, cooperative cancellation, a blocking-pool
boundary and atomics.

## Prerequisites

None. The example is deterministic: it prints values after joins, never
interleaved timing.

## Run

```
kigumi run examples/concurrency
kigumi run --blocking-executor isr examples/concurrency
```

The second form is the catalog's `blocking-executor` mode: the strict `isr`
profile rejects a direct-blocking operation reachable from an async body. This
program stays legal by keeping CPU work in the blocking pool instead of calling
blocking APIs from a task.

## Test

```
kigumi test examples/concurrency
```

There are no `_test.kg` blocks; the run plus `expect.txt` is the check.

## Expected

`expect.txt` records the lazy-future result, a parent that abandons two
children (cancelled and joined at scope exit), `cancelRequested` staying true,
a cancelled unpolled child reporting its own value, a blocking-pool result and
a refused second job at capacity, and the final atomic counter.

Note: under `currentThreadExecutor`, `BlockingPool.run` may run its work on the
calling thread; the example demonstrates the pool boundary and capacity
accounting, while a literal worker-thread offload needs
`task.threadPoolExecutor`, which strict profiles reject.

## Next

`ffi_native` drops below the runtime to the C ABI.
