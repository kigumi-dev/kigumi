# Examples

Each directory is a small Kigumi module: `mod.kg` marks the module root,
`main.kg` next to it is the entry script, and libraries sit in their own
directories (see `testing/numbers/`). Every `main.kg` starts with
`#!/usr/bin/env kigumi` and is executable, so it can run directly, through the
VM, or as a native executable:

```
./examples/hello/main.kg
kigumi run examples/hello
kigumi build examples/hello -o hello && ./hello
kigumi test examples/testing
```

`catalog.json` declares what each example is checked by: `run-output` runs it
through the VM and as a native build and compares stdout against
`expect.txt`, `test-only` relies on its `_test.kg` blocks instead, and
`build-graph` is driven by its own `build.kg`-specific test. `go test
./internal/driver` checks every example against its catalog entry, so a
missing or unreadable `expect.txt` fails the build unless the catalog says
why; the examples double as an end-to-end test suite. `args.txt` and
`stdin.txt` supply arguments and input where an example needs them.

Start at the top and read down.

| Example | Shows |
|---|---|
| [hello](hello/) | The smallest program. |
| [basics](basics/) | Bindings, integers and floats, `if`/`match`, ranges, strings. |
| [fizzbuzz](fizzbuzz/) | Functions and `if` as an expression. |
| [fibonacci](fibonacci/) | Recursion and loops with mutable bindings. |
| [records_and_variants](records_and_variants/) | Records with methods, sum types, `match` and `is` patterns. |
| [closures](closures/) | Lambdas, captures, `map`/`filter`/`fold`, generic function composition. |
| [generics](generics/) | Interfaces, generic functions with bounds, boxed interface values, `Map`. |
| [errors](errors/) | Error variants, `?`, `||` fallbacks, `fail`, `error.context`. |
| [resources](resources/) | Resource types with `drop`/`close`, `defer` and `defer(fail)`. |
| [sorting](sorting/) | `sortedBy` with comparison closures, `compareTo`. |
| [cli_args](cli_args/) | Reading command line arguments with `std/cli`: flags, positional arguments and help (`args.txt`). |
| [word_count](word_count/) | Reading stdin, `Map` tallies, sorting entries (`stdin.txt`). |
| [file_io](file_io/) | `fs`: directories, writing, reading, listing, removing. |
| [json](json/) | Encoding records with field metadata. |
| [shell_pipeline](shell_pipeline/) | `$"..."` command plans, `grep` stages, capturing output. |
| [testing](testing/) | A library package with `_test.kg` blocks run by `kigumi test`. |
| [game_of_life](game_of_life/) | A larger program: consts, `usize` indices, nested loops. |
| [dependency](dependency/) | `mod.kg` with `Require`, `mod.local.kg` with `Replace`: importing another module's packages. |
| [ffi](ffi/) | Calling C functions through `extern(C)` (`kigumi build` only). |
| [async](async/) | `async fn` calls as lazy futures, `await`, `task.spawn`, and the current-thread executor. |
| [http_api](http_api/) | `std/http`: a JSON API with routing, path parameters and middleware, called by the client in the same program, then shut down. |
| [comptime](comptime/) | `comptime { }` blocks evaluated by the compiler into literals. |
| [build_program](build_program/) | `build.kg`: a freestanding library, a registered tool generating C, and a host executable from one graph (`kigumi build` only). |
| [repostat](repostat/) | The example program from the language spec: shell, JSON, allocators, several packages. |
