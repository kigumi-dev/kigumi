# tour

A guided tour of Kigumi's core language in nine commented sections, plus two
small library packages (`model/`, `text/`).

## Prerequisites

None.

## Run

```
kigumi run examples/tour
kigumi build examples/tour -o tour && ./tour
```

## Test

```
kigumi test examples/tour
```

There are no `_test.kg` blocks; the run plus `expect.txt` is the check. The
`eprint` line is deliberate: it goes to stderr, which the oracle ignores.

## Expected

`expect.txt` records, section by section: entry and bindings, integer widths
(including `i128`/`u128`) and the wrapping/rotate/saturating/overflow methods,
strings and builders, a record with a private field and `Display`, sum types
and patterns (`..=`, `if let`, `let ... else`, `?.`, `T??`), generics with a
const parameter and closures, custom errors with `?`/`||`/`context`/`as`,
the three loop forms with a value `break`, and a `comptime` table feeding a
small traffic-light state machine.

## Next

`ownership` goes deeper on resources, borrowing and allocation.
