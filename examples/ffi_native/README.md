# ffi_native

The C ABI from Kigumi: header import, `extern(C)`, `layout(C)`, `export(C)`,
foreign memory, asm, and `dl`.

## Prerequisites

- A C compiler (zig or clang) for `kigumi build`.
- Linux x86-64 for the inline-asm and naked-function section.
- Native only: the VM has no FFI boundary, so this example is checked by
  building and running the executable.

## Run

```
kigumi build examples/ffi_native -o /tmp/ffi_native
/tmp/ffi_native
```

(`kigumi run` refuses it: FFI, asm and `dl` are native-only.)

## Test

```
kigumi test examples/ffi_native
```

The build+run against `expect.txt` is the check.

## Expected

`expect.txt` records the C struct size/alignment, values passed to and from C,
the exported callback re-entered from C, `CString`/`CValue`/`Pin` round trips,
the `@linkName` symbol, the naked function and inline asm results, and a libc
`abs` resolved through `std/ffi/dl`. The header import may warn that some
system-header macros were skipped; that warning does not affect the output.

## Next

`build_system` shows how a build graph compiles and links such artifacts.
