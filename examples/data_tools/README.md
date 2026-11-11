# data_tools

A small data tool: CLI flags and a positional, stdin input, file and `OsString`
handling, a shell pipeline, JSON output, package visibility, a required local
module, and `_test.kg` blocks.

## Prerequisites

`filesystem` and `shell` (the example writes a file under a fresh working
directory and runs a `grep` stage).

## Run

```
kigumi run examples/data_tools -- -m 2 release < examples/data_tools/stdin.txt
```

`args.txt` carries `-m 2 release` for the harness. The input is a
git-log-shaped fixture (`author<TAB>day`); nothing calls `git`, so the result is
deterministic.

## Test

```
kigumi test examples/data_tools
```

Five `std/test` blocks cover line parsing and the number formatting helpers.

## Expected

`expect.txt` records the parsed report, the ranked authors, the JSON document
(field names from `json.name`, `None` omitted by `json.omitNone`), the file
round trip through the temporary directory, the shell pipeline output, and the
`error.as[ParseError]` failure message.

## Packages

- `parse/` — line parsing and `tally`; `pub(super)` helper used by the root.
- `report/` — the report shape, `Display` implementation and `build`.
- `numbers/` — number formatting with its own tests; `pub(module)` helper.
- `units/` — a separate module required through `mod.kg` and resolved by
  `mod.local.kg` (`Replace`), so `--release` policy can be seen on it.

## Next

`concurrency` keeps the same deterministic-output discipline while adding
tasks.
