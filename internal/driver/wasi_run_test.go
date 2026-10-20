package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const wasiRunner = `import { WASI } from 'node:wasi';
import fs from 'node:fs';
const [, , wasm, dir, ...args] = process.argv;
const wasi = new WASI({ version: 'preview1', args: ['prog', ...args], preopens: { '.': dir }, returnOnExit: true });
const inst = await WebAssembly.instantiate(await WebAssembly.compile(fs.readFileSync(wasm)), wasi.getImportObject());
process.exitCode = wasi.start(inst);
`

const wasiHostedProgram = `import task from std/task
import fs from std/fs
import ffi from std/ffi

async fn leaf(n: Int) -> Int {
    n + 1
}

async fn root() -> Int {
    let a = task.spawn(leaf(1))
    let b = task.spawn(leaf(2))
    (await a) + (await b)
}

let args = host.args()
print "args ${args.len()} ${args.get(1) || "?"}"
let files = host.files()
let path = fs.Path.fromString("note.txt")?
files.writeText(path, "wasi\n")?
let body = files.read(path)?
print "read ${body.len()}"
let raw = host.entropy().bytes(16)?
print "entropy ${raw.len()}"
let word = "hi"
let c = ffi.CString.new(&word)?
print "cstring ${ffi.isNull(c.ptr())}"
print "tasks ${task.currentThreadExecutor().runToCompletion(root())}"
`

const wasiHostedWant = "args 2 hello\nread 5\nentropy 16\ncstring false\ntasks 5\n"

type wasiProbe struct {
	fixture string
	src     string
	want    string
}

var wasiProbes = map[string]wasiProbe{
	"host.stdio":         {src: wasiHostedProgram, want: wasiHostedWant},
	"host.args":          {src: wasiHostedProgram, want: wasiHostedWant},
	"host.files":         {src: wasiHostedProgram, want: wasiHostedWant},
	"host.entropy":       {src: wasiHostedProgram, want: wasiHostedWant},
	"ffi.cstring":        {src: wasiHostedProgram, want: wasiHostedWant},
	"ffi.raw":            {src: wasiHostedProgram, want: wasiHostedWant},
	"task.executor":      {src: wasiHostedProgram, want: wasiHostedWant},
	"ffi.pin":            {fixture: "pin"},
	"ffi.handle-table":   {fixture: "handle_table"},
	"task.blocking-pool": {fixture: "blocking_pool"},
	"mem.secure-erase":   {fixture: "secure_erase"},
}

func wasiRuntime(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"wasmtime", "wasmer"} {
		if path, err := exec.LookPath(name); err == nil {
			t.Logf("%s is installed at %s, but this test drives node:wasi only", name, path)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no WASI runtime on this machine: wasmtime, wasmer and node are all absent from PATH")
	}
	return node
}

func TestWasiProvidedSetRuns(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	node := wasiRuntime(t)
	wasi := mustTarget(t, "wasm32-wasi", "")
	runner := filepath.Join(t.TempDir(), "run.mjs")
	if err := os.WriteFile(runner, []byte(wasiRunner), 0o644); err != nil {
		t.Fatal(err)
	}
	ran := map[string]bool{}
	for _, v := range driver.ProvidedCapabilities(wasi) {
		if !v.Provided {
			continue
		}
		probe, ok := wasiProbes[v.Capability]
		if !ok {
			t.Errorf("wasi provides %s but no probe runs it", v.Capability)
			continue
		}
		key := probe.fixture + "\x00" + probe.src
		if ran[key] {
			continue
		}
		ran[key] = true
		t.Run(v.Capability, func(t *testing.T) {
			t.Parallel()
			runWasiProbe(t, node, runner, wasi, probe)
		})
	}
}

func runWasiProbe(t *testing.T, node, runner string, target driver.Target, probe wasiProbe) {
	root, want := t.TempDir(), probe.want
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"wasirun\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	if probe.fixture != "" {
		fxRoot, fxWant, _, _ := loadRunFixture(t, "../../testdata/run/"+probe.fixture+".txtar")
		src, err := os.ReadFile(filepath.Join(fxRoot, "main", "main.kg"))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(root, "main.kg"), src, 0o644)
		want = fxWant
	} else {
		os.WriteFile(filepath.Join(root, "main.kg"), []byte(probe.src), 0o644)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../std", Target: target})
	if err != nil {
		t.Fatal(err)
	}
	wasm := filepath.Join(root, "prog.wasm")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, wasm, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("wasm32-wasi build: %v\n%s", err, buildErr.String())
	}
	dir := t.TempDir()
	cmd := exec.Command(node, "--no-warnings", runner, wasm, dir, "hello")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("running under node:wasi: %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	if out.String() != want {
		t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", out.String(), want)
	}
}
