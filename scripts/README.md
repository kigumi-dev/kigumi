# scripts

Local, sudo-free entry points for contributors. CI owns the full matrix
(`.github/workflows/ci.yml`); this directory only reproduces the checks a
change needs before a pull request, with the same commands.

## dev-test.sh

```
scripts/dev-test.sh env      # resolved toolchain, optional tools, TMPDIR and `kigumi doctor`
scripts/dev-test.sh check    # gofmt, `kigumi fmt --check`, go vet (CI's first three steps)
scripts/dev-test.sh quick    # unit tests of every package except internal/toolchain/driver
scripts/dev-test.sh full     # go test ./... (unit tests and the whole corpus; long)
scripts/dev-test.sh aot      # the pinned-zig AOT corpus in internal/toolchain/driver
scripts/dev-test.sh ct       # SecretSan and constant-time tests
```

Requirements: Go 1.26 or newer (the `go.mod` directive) and, for `full`,
`aot` and `ct`, the zig release named in `ZIG_VERSION`. The runner reuses the
managed store at `~/.local/share/kigumi/zig/<version>` (honouring
`KIGUMI_ZIG_HOME` and `XDG_DATA_HOME`) and runs `kigumi zig install` once when
it is absent, so the AOT and constant-time checks compile with the pinned zig
even when PATH carries a different one. Nothing needs root.

Options may appear before or after the command:

- `--tmpdir DIR` or `KIG_TEST_TMPDIR`: the `TMPDIR` every child process sees.
  The default is `$TMPDIR/kigumi-dev`, else `~/.cache/kigumi-dev`. `full` and
  `aot` refuse to start with less than 4 GiB free (`KIG_TEST_MIN_FREE_MIB`
  overrides the floor) because the corpus builds many binaries.
- `--shard I/N`: sets `KIGUMI_TEST_SHARD`, slicing every corpus loop into `N`
  parts (`full`/`aot` become correspondingly shorter).
- `--timeout DUR`: the `go test -timeout` value; each command has a default.
- `--`: every remaining argument is passed to `go test`, for example
  `scripts/dev-test.sh quick -- -count=1 -v`.

A missing required tool is an error rather than a skip. The runner sets
`umask 022` like the CI runners, because corpus fixtures and the formatter
tests create files with explicit modes and compare them back. `env` reports the
optional tools — qemu-system-x86_64, qemu-arm, OVMF, OpenSSL, the SGX SDK and
pnpm — and only the checks that use them require them. `ct` honours
`KIGUMI_SECRET_SAN_CORPUS=1` to rerun every corpus fixture under
`--secret-san`, the sweep the hermetic CI job performs; without it the command
stays targeted at the SecretSan and constant-time tests.

Mapping to CI:

| command | CI job |
|---|---|
| `check` | `hermetic` steps 1–3 |
| `quick` | `hermetic`, limited to the packages other than `internal/toolchain/driver` |
| `full` | `hermetic` plus `aot` (one process instead of the matrix) |
| `aot` | `aot` (`default` entry; set `KIGUMI_CFLAGS=-DKIGUMI_RC_CHECK` for the rc-check entry) |
| `ct` | the SecretSan and constant-time subset of `hermetic`/`aot` |

`cross`, `uefi-boot`, `race`, `benchmarks`, the nightly sweeps and the editor
checks stay CI-only; run their commands from `ci.yml` when a change touches
them.
