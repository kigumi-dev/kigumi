# Examples

Eight examples, each built around one theme. Read them in `order`: every one
is a complete Kigumi module with `mod.kg` at its root, a runnable `main.kg`
(or a `build.kg` build program), and a deterministic expected output where the
harness can run it.

```
./examples/tour/main.kg                      # a shebang entry runs directly
kigumi run examples/tour                     # through the VM
kigumi build examples/tour -o tour && ./tour # as a native executable
kigumi test examples/data_tools              # its *_test.kg blocks
kigumi build examples/build_system --out-dir /tmp/bs -- --big
```

## Catalog

`catalog.json` is the single source for the table below and for the harness:

- `order` / `level`: reading order, independent of directory names.
- `entry`: the entry file under the example root.
- `oracle`: `run-output` (run and compare stdout with `expect.txt`),
  `test-only` (only `_test.kg` checks), or `build-graph` (driven by `build.kg`).
- `backends`: `vm`, `native`, or both. A native-only example still type-checks.
- `modes`: extra runner modes, such as `secret-san` (VM constant-time shadow
  checker), `blocking-executor` (the strict async profile) or `reproducible`
  (a build-graph byte-for-byte rebuild).
- `requires`: capability codes (`c-compiler`, `filesystem`, `shell`,
  `network-loopback`, `linux-x86-64`); a missing capability skips the example
  with that code instead of silently passing.
- `features`: canonical tags from `features.json`.
- `blocked_by`: in `features.json`, an unimplemented spec ID that no example
  may claim. Blocked features are not faked.

`features.json` lists every tag some example must demonstrate plus the blocked
tags and their IDs (for example `pub(in ...)` was removed in v2.8 and reports
`E133`, so it is blocked rather than written into an example).

## Files the harness uses

`expect.txt` is the exact stdout the example must print, on every declared
backend and mode. `args.txt` (whitespace-separated) supplies argv and
`stdin.txt` supplies input. The runner uses a fresh temporary working directory,
so examples write only relative paths and never print temp names, ports or
timestamps.

| Example | Level | Shows | Covers |
|---|---|---|---|
| [tour](tour/) | beginner | A guided tour of Kigumi: entry, numeric widths, text, records, patterns, generics, errors, loops and comptime. | lang.entry, lang.binding, lang.numeric, lang.numeric.i128, lang.numeric.ops, lang.text, lang.text.buffers, lang.record, lang.adt, lang.pattern, lang.option, lang.generics, lang.error, lang.loop, lang.comptime |
| [ownership](ownership/) | intermediate | Ownership and memory: resources, defer, partial moves, borrows, Box, Shared/Weak, arenas and stack arrays. | mem.resource, mem.move, mem.borrow, mem.rc, mem.box, mem.arena, mem.effects, mem.fixed |
| [data_tools](data_tools/) | intermediate | A data tool: std/cli, stdin, files, a shell pipeline, JSON/schema, visibility, Require/Replace and module tests. | io.cli, io.stdin, io.fs, io.shell, io.json, module.visibility, module.require, module.test, lang.error |
| [concurrency](concurrency/) | advanced | Async, await, task.spawn, structured children, cancellation, a blocking pool and atomics. | async.future, async.cancel, async.blocking, sync.atomic |
| [ffi_native](ffi_native/) | advanced | The C ABI: Header/Source/Link, extern(C), layout(C), export(C), CString/CValue/Pin, asm and dl. | ffi.extern, ffi.layout, ffi.export, ffi.memory, ffi.link, ffi.asm, ffi.dl |
| [build_system](build_system/) | advanced | build.kg: a Framework, a code-generating Tool, a freestanding archive and a linked host executable. | build.graph, build.freestanding, build.reproducible |
| [http_service](http_service/) | advanced | std/net/http: a loopback server and client with routing, middleware, JSON, status codes, timeouts and shutdown. | net.http, io.json |
| [secret](secret/) | advanced | std/secret and std/crypto: SecretU, Choice, constant-time compare/select, SecretBuffer, SecretIndex, SHA-256 and HMAC. | secret.ct, secret.buffer, crypto.hash |

## Size and coverage

The set is 1,860 lines across the eight module roots, with a 154–320 line range.
`features.json` requires 49 tags; the catalog covers all of them, and the three
blocked tags do not appear in an example.

| Example | Lines | Test files |
|---|---:|---:|
| tour | 295 | 0 |
| ownership | 229 | 0 |
| data_tools | 320 | 2 |
| concurrency | 175 | 0 |
| ffi_native | 250 | 0 |
| build_system | 188 | 0 |
| http_service | 249 | 0 |
| secret | 154 | 0 |

## Checking the examples

```
go test ./internal/toolchain/driver -run 'TestExamples|TestExampleModules|TestUnitExamplesMatchWholeCheck|TestExampleCoverage|TestExampleDensity|TestExamplesReadmeMatchesCatalog|TestBuildProgramExample'
kigumi fmt --check examples
```

`TestExampleModules` runs each run-output example on its backends and modes,
`TestBuildProgramExample` executes each build-graph example, `TestExamples`
parses and formats every `.kg`, `TestUnitExamplesMatchWholeCheck` compares unit
and whole-module checks, `TestExampleCoverage` checks `features.json` against
the catalog, and `TestExampleDensity` keeps the set from decaying into
fragments or growing into one unreadable program.
