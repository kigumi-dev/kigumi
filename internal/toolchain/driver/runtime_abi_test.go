package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/compiler/mir"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/version"
)

const (
	singleThreadSrc = "print \"single\"\n"
	blockingOnlySrc = "import task from std/task\n\nlet pool = task.BlockingPool.new(1)\nprint \"${pool.run(() => 1) is Ok(_)}\"\n"
	threadPoolSrc   = "import task from std/task\n\nasync fn leaf(n: Int) -> Int {\n    n\n}\n\nlet pool = task.threadPoolExecutor(2)\nprint \"pool ${pool.runToCompletion(leaf(41))}\"\n"
)

func profileOf(t *testing.T, src string, cflags ...string) driver.RuntimeProfile {
	t.Helper()
	var stderr bytes.Buffer
	p, ok, err := driver.RuntimeProfileOf(loadTryConstructorModule(t, src), &stderr, buildconfig.HostTarget(), cflags)
	if err != nil || !ok {
		t.Fatalf("profile: ok=%v err=%v\n%s", ok, err, stderr.String())
	}
	return p
}

func hasFlag(p driver.RuntimeProfile, flag string) bool { return slices.Contains(p.Flags, flag) }

// The atomic refcount profile is on exactly when the program can reach the one primitive that creates threads, and the fingerprint tells the two runtimes apart.
func TestRuntimeProfileFollowsThreadPrimitive(t *testing.T) {
	t.Parallel()
	single, blocking, pooled := profileOf(t, singleThreadSrc), profileOf(t, blockingOnlySrc), profileOf(t, threadPoolSrc)
	for name, p := range map[string]driver.RuntimeProfile{"single-threaded": single, "blocking pool": blocking} {
		if hasFlag(p, "KIGUMI_THREADS") || hasFlag(p, "KIGUMI_CAP_TASK_THREADS") {
			t.Errorf("%s program's runtime is built with the thread profile: %v", name, p.Flags)
		}
	}
	if !hasFlag(blocking, "KIGUMI_CAP_TASK_BLOCKING_POOL") {
		t.Errorf("the blocking pool program lost its capability: %v", blocking.Flags)
	}
	if !hasFlag(pooled, "KIGUMI_THREADS") || !hasFlag(pooled, "KIGUMI_CAP_TASK_THREADS") {
		t.Errorf("thread pool program's runtime lacks the thread profile: %v", pooled.Flags)
	}
	seen := map[string]string{}
	for name, p := range map[string]driver.RuntimeProfile{"single": single, "blocking": blocking, "pooled": pooled} {
		if !strings.HasPrefix(p.Fingerprint, "kgabi1:") || len(p.Fingerprint) != len(llgen.AbiPlaceholder) {
			t.Errorf("%s: fingerprint %q is not in the fixed format", name, p.Fingerprint)
		}
		if other, dup := seen[p.Fingerprint]; dup {
			t.Errorf("%s and %s share fingerprint %s", name, other, p.Fingerprint)
		}
		seen[p.Fingerprint] = name
	}
}

func TestRuntimeFingerprintTracksFlagsOnly(t *testing.T) {
	t.Parallel()
	base := profileOf(t, singleThreadSrc)
	if again := profileOf(t, singleThreadSrc); again.Fingerprint != base.Fingerprint {
		t.Errorf("two compiles of one program disagree: %s vs %s", base.Fingerprint, again.Fingerprint)
	}
	if noise := profileOf(t, singleThreadSrc, "-O2", "-DFOO=1", "-Wall"); noise.Fingerprint != base.Fingerprint {
		t.Errorf("flags outside KIGUMI_ moved the fingerprint")
	}
	rc := profileOf(t, singleThreadSrc, "-DKIGUMI_RC_CHECK")
	if rc.Fingerprint == base.Fingerprint || !hasFlag(rc, "KIGUMI_RC_CHECK") {
		t.Errorf("-DKIGUMI_RC_CHECK did not change the runtime profile: %v", rc.Flags)
	}
	if tsan := profileOf(t, singleThreadSrc, "-fsanitize=thread"); tsan.Fingerprint == base.Fingerprint {
		t.Errorf("a sanitizer build shares the plain runtime's fingerprint")
	}
	override := profileOf(t, singleThreadSrc, "-UKIGUMI_ABI_FINGERPRINT", "-DKIGUMI_ABI_FINGERPRINT=\"kgabi1:stale\"")
	if override.Fingerprint != base.Fingerprint {
		t.Errorf("the fingerprint macro itself fed the fingerprint")
	}
}

// A registry that differs in one op yields another fingerprint, another generated header and so another runtime to compile.
func TestRegistryFingerprintTracksEdits(t *testing.T) {
	t.Parallel()
	layout, err := llgen.RuntimeLayout()
	if err != nil {
		t.Fatal(err)
	}
	alt, err := mir.ParseStdOpRegistry(mirRegistryText(t) + fmt.Sprintf("op %d", len(mir.Registry.Ops)) + " ZZ_NEW primitive () -> unit\nkey zz.new\n")
	if err != nil {
		t.Fatal(err)
	}
	flags := []string{"KIGUMI_UNWIND"}
	if llgen.AbiFingerprint(alt, layout, version.AbiRevision, flags) == llgen.AbiFingerprint(mir.Registry, layout, version.AbiRevision, flags) {
		t.Error("an added op left the fingerprint unchanged")
	}
	if llgen.StdOpHeader(alt) == llgen.StdOpHeader(mir.Registry) || !strings.Contains(llgen.StdOpHeader(alt), "OP_ZZ_NEW = ") {
		t.Error("an added op did not reach the generated C header")
	}
	renamed, err := mir.ParseStdOpRegistry(strings.Replace(mirRegistryText(t), "op 0 LEN primitive (string|bytes)", "op 0 LEN primitive (any)", 1))
	if err != nil {
		t.Fatal(err)
	}
	if llgen.AbiFingerprint(renamed, layout, version.AbiRevision, flags) == llgen.AbiFingerprint(mir.Registry, layout, version.AbiRevision, flags) {
		t.Error("a changed descriptor left the fingerprint unchanged")
	}
	commented, err := mir.ParseStdOpRegistry("# a comment\n\n" + mirRegistryText(t))
	if err != nil {
		t.Fatal(err)
	}
	if llgen.AbiFingerprint(commented, layout, version.AbiRevision, flags) != llgen.AbiFingerprint(mir.Registry, layout, version.AbiRevision, flags) {
		t.Error("a comment moved the fingerprint")
	}
}

func embeddedFingerprints(t *testing.T, exe string) []string {
	t.Helper()
	bin, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, m := range regexp.MustCompile(`kgabi1:[0-9a-f]{32}`).FindAll(bin, -1) {
		if !slices.Contains(found, string(m)) {
			found = append(found, string(m))
		}
	}
	return found
}

// A registry edit changes the fingerprint the runtime is compiled under, so the binary carries the new one and a bare archive built over the old one reads as stale.
// A zig shim on PATH logs every compile of the runtime source. It swaps the package registry and PATH, so it is not parallel.
func TestRegistryChangeInvalidatesRuntime(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	run := func(exe string) {
		t.Helper()
		if out, err := exec.Command(exe).Output(); err != nil || string(out) != "single\n" {
			t.Fatalf("run: %q, %v", out, err)
		}
	}
	runtimeCompiles := logRuntimeCompiles(t)
	first := buildProgram(t, singleThreadSrc)
	compiled := runtimeCompiles()
	again := buildProgram(t, singleThreadSrc)
	before := embeddedFingerprints(t, first)
	if cached := runtimeCompiles()[len(compiled):]; len(compiled) != 1 || len(cached) != 1 || cached[0] != compiled[0] || compiled[0] != before[0] {
		t.Fatalf("the runtime was compiled as %v then %v, want one compile under %v each time", compiled, cached, before)
	}
	if len(before) != 1 || !slices.Equal(before, embeddedFingerprints(t, again)) {
		t.Fatalf("two builds of one program disagree: %v vs %v", before, embeddedFingerprints(t, again))
	}
	archive := filepath.Join(t.TempDir(), "rt.a")
	bareBuild := func() (string, driver.ArtifactFingerprint) {
		t.Helper()
		target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-freestanding", "")
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"abi\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
		os.WriteFile(filepath.Join(root, "main.kg"), []byte(singleThreadSrc), 0o644)
		std, _ := filepath.Abs("../../../std")
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		ok, err := driver.BuildWith(m, archive, &stderr, driver.BuildOptions{Target: target})
		if err != nil || !ok {
			t.Fatalf("bare build: %v\n%s", err, stderr.String())
		}
		fp, _, err := driver.ReadFingerprintSidecar(archive)
		if err != nil {
			t.Fatal(err)
		}
		return stderr.String(), fp
	}
	_, bareBefore := bareBuild()
	alt, err := mir.ParseStdOpRegistry(mirRegistryText(t) + fmt.Sprintf("op %d", len(mir.Registry.Ops)) + " ZZ_NEW primitive () -> unit\nkey zz.new\n")
	if err != nil {
		t.Fatal(err)
	}
	saved := mir.Registry
	mir.Registry = alt
	defer func() { mir.Registry = saved }()
	seen := len(runtimeCompiles())
	edited := buildProgram(t, singleThreadSrc)
	run(edited)
	after := embeddedFingerprints(t, edited)
	if len(after) != 1 || after[0] == before[0] {
		t.Errorf("the build after a registry edit carries %v, want a fingerprint other than %v", after, before)
	}
	if recompiled := runtimeCompiles()[seen:]; len(recompiled) != 1 || recompiled[0] == before[0] || recompiled[0] != after[0] {
		t.Errorf("the build after a registry edit compiled the runtime as %v, want one compile under %v and not %v", recompiled, after, before)
	}
	note, bareAfter := bareBuild()
	if !strings.Contains(note, "note: rebuilding") || !strings.Contains(note, "runtime ABI") {
		t.Errorf("a bare archive rebuilt after a registry edit did not report the stale one:\n%s", note)
	}
	if bareAfter.RuntimeAbi == bareBefore.RuntimeAbi || bareAfter.AbiBase == bareBefore.AbiBase {
		t.Errorf("the archive sidecar kept its runtime fingerprint across a registry edit: %+v", bareAfter)
	}
}

// logRuntimeCompiles puts a zig shim first on PATH; the returned func lists the fingerprint every compile of the runtime source so far was given.
func logRuntimeCompiles(t *testing.T) func() []string {
	t.Helper()
	real, err := exec.LookPath("zig")
	if err != nil {
		t.Skip("no zig on PATH")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	shim := "#!/bin/sh\necho \"$*\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "zig"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	fingerprint := regexp.MustCompile(`kgabi1:[0-9a-f]{32}`)
	return func() []string {
		data, _ := os.ReadFile(log)
		var out []string
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "rt_core.c") {
				out = append(out, fingerprint.FindString(line))
			}
		}
		return out
	}
}

func mirRegistryText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../compiler/mir/stdops.reg")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func buildProgram(t *testing.T, src string, cflags ...string) string {
	t.Helper()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	ok, err := driver.BuildWith(loadTryConstructorModule(t, src), exe, &buildErr, driver.BuildOptions{CFlags: cflags})
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	return exe
}

// The fingerprint is compiled into the runtime object as a symbol, so the linked program carries exactly the one its build computed.
func TestBuiltProgramCarriesRuntimeFingerprint(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct{ src, out string }{"single": {singleThreadSrc, "single\n"}, "pooled": {threadPoolSrc, "pool 41\n"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := profileOf(t, c.src)
			exe := buildProgram(t, c.src)
			bin, err := os.ReadFile(exe)
			if err != nil {
				t.Fatal(err)
			}
			if got := bytes.Count(bin, []byte(want.Fingerprint)); got < 2 {
				t.Errorf("binary holds fingerprint %s %d times, want the runtime's symbol and the program's expected value", want.Fingerprint, got)
			}
			out, err := exec.Command(exe).Output()
			if err != nil || string(out) != c.out {
				t.Errorf("run: %q, %v; want %q", out, err, c.out)
			}
		})
	}
}

// A runtime object from another build, standing in as a stale cache, makes the generated main stop before running anything.
func TestStartupRejectsMismatchedRuntime(t *testing.T) {
	t.Parallel()
	stale := "kgabi1:" + strings.Repeat("f", 32)
	exe := buildProgram(t, singleThreadSrc, "-UKIGUMI_ABI_FINGERPRINT", "-DKIGUMI_ABI_FINGERPRINT=\""+stale+"\"")
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(exe)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if exitCodeOf(err) != 2 || stdout.Len() != 0 {
		t.Fatalf("exit %d, stdout %q; want exit 2 and no output", exitCodeOf(err), stdout.String())
	}
	want := profileOf(t, singleThreadSrc).Fingerprint
	for _, frag := range []string{"runtime ABI mismatch", "E1880", want, stale} {
		if !strings.Contains(stderr.String(), frag) {
			t.Errorf("stderr lacks %q:\n%s", frag, stderr.String())
		}
	}
}

// A runtime archive built by another toolchain is rejected before the link starts, with one diagnostic.
func TestLinkRejectsStaleRuntimeArchive(t *testing.T) {
	t.Parallel()
	lib := filepath.Join(t.TempDir(), "libold.a")
	if err := os.WriteFile(lib, []byte("!<arch>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		fp   driver.ArtifactFingerprint
		want string
	}{
		{"other runtime", driver.ArtifactFingerprint{RuntimeAbi: "kgabi1:" + strings.Repeat("a", 32)}, "kgabi1:" + strings.Repeat("a", 32)},
		{"older toolchain", driver.ArtifactFingerprint{AbiRevision: 1}, "none"},
	} {
		if err := driver.WriteFingerprintSidecar(lib, c.fp); err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(t.TempDir(), "prog")
		var buildErr bytes.Buffer
		ok, err := driver.BuildWith(loadTryConstructorModule(t, singleThreadSrc), exe, &buildErr, driver.BuildOptions{CFlags: []string{lib}})
		if ok || err == nil || !strings.Contains(err.Error(), "E1880") || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ok=%v err=%v, want E1880 naming %s", c.name, ok, err, c.want)
		}
		if _, statErr := os.Stat(exe); statErr == nil {
			t.Errorf("%s: a stale archive still produced an executable", c.name)
		}
	}
}
