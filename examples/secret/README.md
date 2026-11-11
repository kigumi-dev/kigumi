# secret

Constant-time secret handling with `std/secret`, plus SHA-256 and HMAC from
`std/crypto`.

## Prerequisites

None beyond the toolchain. The example is checked on the VM only: the
constant-time shadow checker is a VM feature.

## Run

```
kigumi run examples/secret
kigumi run --secret-san examples/secret
```

`--secret-san` turns on the shadow checker: branching on a value that came
from a secret primitive without an explicit `declassify` becomes a diagnostic.
Both invocations must print the same bytes.

## Test

```
kigumi test examples/secret
```

There are no `_test.kg` blocks here; the run itself is the check, together
with `expect.txt`.

## Expected

`expect.txt` holds the exact stdout: constant-time comparisons, a
`SecretBuffer` compare and declassify, secret-index lookups and updates, and
the digests of a message, an HMAC and a declassified buffer.

## Next

`http_service` shows the same explicit-boundary style at the network edge.
