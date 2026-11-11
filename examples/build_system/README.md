# build_system

A `build.kg` build graph: a registered tool that generates C, a freestanding
Kigumi library, and a host executable that links both.

## Prerequisites

- A C compiler (zig or clang).
- `sh` for `tools/gen.sh`.
- Native only: the graph is compiled, not interpreted by the VM.

## Run

```
kigumi build examples/build_system --out-dir /tmp/bs -- --big
/tmp/bs/app
```

Without `--big` the generated value is 7 instead of 700. The output directory
receives `core.a` (the freestanding archive), `app`, and the generated `gen.c`.

## Test

```
go test ./internal/toolchain/driver -run TestBuildProgramExample
```

The harness runs `configure`, resolves and plans the graph, checks the plan
names the `core` library, the `gen` tool and an executable, executes it,
requires `core.a` and `app`, and compares `app`'s stdout with `expect.txt`.
The catalog's `reproducible` mode executes the graph with `--reproducible`
semantics (two independent builds compared byte for byte).

## Expected

`expect.txt` is the app's stdout: the generated budget, the ring-buffer sum,
half/lerp results, the newest value, the range and the full flag.

## Files

- `build.kg` — `configure(b: build.Builder)`: framework, library, tool step,
  executable.
- `kigumi.framework.kg` — the `example/gen` framework and its `gen` tool.
- `core/lib.kg` — freestanding fixed-point and ring-buffer code.
- `cmd/app/main.kg` — the host program using `core` and the generated C.
- `tools/gen.sh` — writes a C function returning the requested value.

## Next

The examples end here; `http_service` and `secret` cover the remaining
runtime boundaries.
