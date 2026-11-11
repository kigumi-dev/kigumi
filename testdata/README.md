# testdata

This directory holds three kinds of files, and nothing else at the root:

| Path | Role |
|---|---|
| `manifest.json` | generated index of every `.txtar` / `.kg` fixture |
| `fmt/`, `parser/`, `sem/`, `mir/`, `run/`, `oom/` | fixture corpora, one directory per stage |
| `native/header_import/` | C header import modules (`mod.kg`, headers, expected output) |
| `conformance/` | external vector corpora (`crypto`, `cvm`, `sgx`, `unicode`) |
| `golden/` | checked-in generator and baseline output (`hir`, `plan`, `schema`, `semantic_owners.json`) |

There is no `live/` baseline: `vm_live.txt` and `native_live.txt` were empty, so
they and the tests that only skipped on them were removed. The VM corpus still
asserts that exit-0 fixtures leave no live object; the native corpus asserts the
same when built with `-DKIGUMI_RC_CHECK`.

## Fixtures

A fixture is one oracle, not one example: do not merge two fixtures because
their sources overlap. The corpora are:

| Kind | Stage | Extension | Run by |
|---|---|---|---|
| `parser` | `parse` | `.txtar` | `TestParserCorpus` |
| `sem` | `check` / `driver` / `planned` | `.txtar` | `TestSemCorpus`, `TestSemDriverFixtures` |
| `mir` | `mir` | `.txtar` | `TestMirCorpus` |
| `fmt` | `format` | `.txtar` | `TestFmtCorpus` |
| `run` | `run` | `.txtar` | `TestRunCorpusEngines`, `TestAotCorpus` |
| `oom` | `fault` | `.kg` | `TestFaultInjectionTryFamily*` |

### IDs and groups

A fixture's ID is `<kind>/<basename>` and never changes when the file moves.
The path is `testdata/<kind>/<group>/<basename>.<ext>`; the group is exactly one
directory. Groups name a responsibility, not a filename prefix, and there is no
`misc` group. Every corpus directory stays under 300 files.

`group` in the manifest must equal the first directory under `testdata/<kind>`,
and a basename must be unique within its kind. `internal/devkit/fixture`
enforces both. Go tests never name a corpus directory: they list a corpus with
`fixture.Corpus(root, kind)` and locate one fixture with `fixture.Find(root, id)`.
This keeps a group move to a `git mv` plus manifest regeneration.

Shards hash IDs, not paths, so moving a fixture between groups never reshuffles
CI. Tests pass corpus paths through `fixture.ShardPaths`.

Current groups:

| Kind | Groups |
|---|---|
| `run` | `async`, `compiler`, `concurrency`, `errors`, `ffi`, `language`, `modules`, `ownership`, `runtime`, `std`, `types` |
| `sem` | `async`, `diagnostics`, `effects`, `ffi`, `modules`, `ownership`, `runtime`, `std`, `types` |
| `mir` | `borrow`, `codegen`, `lifetime`, `move` |
| `parser` | `declarations`, `diagnostics`, `expressions`, `literals`, `patterns`, `statements` |
| `fmt` | `comments`, `declarations`, `expressions`, `literals`, `statements`, `types` |
| `oom` | `alloc`, `array`, `builder`, `bytes`, `map`, `os`, `shared`, `string`, `text` |

### Manifest

`testdata/manifest.json` is generated; do not hand-edit it. Schema 2 entries
carry `id`, `kind`, `group`, `stage`, `path` and `mode`, plus optional `pair`
and `covers` for the fault corpus.

Regenerate it after adding, moving or removing a fixture or changing a
`stage: driver` / `mir: planned` header:

```console
UPDATE=1 go test -count=1 -run TestManifestFresh ./internal/toolchain/driver/
go test -count=1 -run 'TestManifestFresh|TestManifestConsistency' ./internal/toolchain/driver/
```

`UPDATE=1` here writes only the manifest; fixture bodies have their own golden
tests (`TestParserCorpus`, `TestSemCorpus`, `TestMirCorpus`, `TestFmtCorpus`)
and those must not be run with `UPDATE=1` in the same change.

Stage is derived from the kind (`parse`, `mir`, `format`, `run`, `fault`) or,
for `sem`, from the fixture's `-- module --` header: `stage: driver` and
`mir: planned` become `driver` and `planned`, everything else is `check`. A
fixture carrying both headers is `driver`, because that is the later stage that
runs it.

### The fault corpus

OOM fixtures are `.kg` programs, registered in the same manifest with
`mode: ["fault"]`. A base fixture named `<name>` pairs with `<name>_twice`: the
second file calls the same `try*` function twice, so a refused allocation on
either call is swept. `pair` points both ways and consistency checks that the
two entries live in the same group. The six `*_cells` fixtures have no pair
because they sweep an element copy, not a second call; they omit `pair`.

`covers` lists the std `try*` functions a fixture sweeps. `covers` is preserved
from the checked-in manifest across regeneration (it is authored metadata, not
something the `.kg` source states); the manifest itself is the source of truth.
`TestEveryTryFunctionHasATwoCallSweep` requires every non-inert std try function
to be covered by an entry that also has a pair.

### Adding a fixture

1. Put the file in the group whose responsibility it exercises; keep the
   basename unique within the kind.
2. Give it the sections its kind expects (`-- src --`, `-- expect --`, module
   options, and for `run` the optional `-- engines --` / `-- stderr --`).
3. Regenerate the manifest with `UPDATE=1` (see above).
4. Run the corpus test for the kind, then `TestManifestConsistency`.
5. If a run fixture is native-only, record the skip reason in `mode` / `skip`
   through the generator's VM probe; `-- engines --` and the manifest must agree.
6. For a fault fixture, add its `<name>_twice` partner and set `covers` on its
   manifest entry before the final regeneration.

## Assets

- `native/header_import/*`: self-contained modules imported by the header
  import AOT tests. `mod.kg` uses relative paths inside its own directory; the
  Go test only names the module root.
- `conformance/crypto`, `conformance/cvm`, `conformance/sgx`: externally
  sourced vectors plus a local `manifest.json` whose paths are relative to that
  corpus directory. `conformance/unicode` carries the Unicode grapheme break
  test and its `NOTICE.txt`; the notice travels with the data.
- `golden/`: expected generator output. `golden/hir/scalar.hir` is pinned by
  the VM scalar test, `golden/plan/capability_reasons.golden.json` by the plan
  reasons test, `golden/schema/generator_output.golden` by the derive cache
  test, and `golden/semantic_owners.json` by `TestSemanticOwnersFresh`.
  Regenerate them with their own tests' `UPDATE=1`, never with
  `TestManifestFresh`.
