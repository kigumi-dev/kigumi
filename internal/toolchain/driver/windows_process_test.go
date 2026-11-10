package driver_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// winProcSrc reaches every Win32 entry point the Windows process platform
// declares: spawn (CreateProcessA, DuplicateHandle, CloseHandle), a blocking
// wait and an exit code (WaitForSingleObject, GetExitCodeProcess), a
// non-blocking wait through waitTimeout, and a kill (TerminateProcess).
const winProcSrc = `import proc from std/proc

let ps = host.processes()
let mut child = ps.spawn("prog.exe", Array.of("a b"))?
let mut slow = ps.spawn("prog.exe", Array.of(""))?
let early = child.tryWait()?
print "${early is None}"
let timed = slow.waitTimeout(5)?
print "${timed is None}"
let mut victim = ps.spawn("prog.exe", Array.of("x"))?
victim.kill()?
print "${victim.wait()?}"
`

// compileForTarget checks src as the entry package for target and returns the
// IR and the diagnostics; ok is false when the compile failed.
func compileForTarget(t *testing.T, target buildconfig.Target, src string) (ir, diag string, ok bool) {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"procwin\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	ir, ok, err = driver.Compile(m, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	return ir, stderr.String(), ok
}

// TestWindowsProcessCalls drives std/proc for x86_64-windows and pins the
// process layer to Win32: the handle lifecycle must go through
// CreateProcessA, DuplicateHandle and CloseHandle, the waits through
// WaitForSingleObject and GetExitCodeProcess, and a kill through
// TerminateProcess. The POSIX syscalls must not leak into the target.
func TestWindowsProcessCalls(t *testing.T) {
	t.Parallel()
	// x86_64 pins the Win32 declarations' exact ABI shapes: HANDLE is a
	// pointer-sized i64, BOOL a C int, and the layout(C) records go by pointer.
	cases := []struct {
		triple    string
		wantDecls []string
	}{
		{"x86_64-windows-gnu", []string{
			"declare i32 @CreateProcessA(ptr, ptr, ptr, ptr, i32, i32, ptr, ptr, ptr, ptr)",
			"declare i32 @DuplicateHandle(i64, i64, i64, ptr, i32, i32, i32)",
			"declare i32 @CloseHandle(i64)",
			"declare i32 @WaitForSingleObject(i64, i32)",
			"declare i32 @GetExitCodeProcess(i64, ptr)",
			"declare i32 @TerminateProcess(i64, i32)",
			"declare i32 @GetLastError()",
		}},
		// The same source compiles for 32-bit Windows, where HANDLE is 32 bits
		// and the layout(C) records are sized for a 4-byte pointer.
		{"x86-windows-gnu", nil},
	}
	for _, c := range cases {
		t.Run(c.triple, func(t *testing.T) {
			t.Parallel()
			ir, diag, ok := compileForTarget(t, mustTarget(t, c.triple, ""), winProcSrc)
			if !ok {
				t.Fatalf("%s compile failed:\n%s", c.triple, diag)
			}
			for _, decl := range c.wantDecls {
				if !strings.Contains(ir, decl) {
					t.Errorf("std/proc on %s does not declare %q", c.triple, decl)
				}
			}
			for _, call := range []string{"@CreateProcessA(", "@DuplicateHandle(", "@CloseHandle(", "@WaitForSingleObject(", "@GetExitCodeProcess(", "@TerminateProcess("} {
				if !strings.Contains(ir, call) {
					t.Errorf("std/proc on %s never calls %s", c.triple, call)
				}
			}
			for _, posix := range []string{"@fork(", "@execvp(", "@waitpid(", "@kill("} {
				if strings.Contains(ir, posix) {
					t.Errorf("std/proc on %s still emits the POSIX call %s", c.triple, posix)
				}
			}
		})
	}
}

// TestPosixProcessCalls keeps the native path on fork/execvp/waitpid/kill: the
// Windows platform file must not change what a POSIX target emits.
func TestPosixProcessCalls(t *testing.T) {
	t.Parallel()
	ir, diag, ok := compileForTarget(t, buildconfig.HostTarget(), winProcSrc)
	if !ok {
		t.Fatalf("host compile failed:\n%s", diag)
	}
	for _, call := range []string{"@fork(", "@execvp(", "@waitpid(", "@kill("} {
		if !strings.Contains(ir, call) {
			t.Errorf("std/proc on the host no longer calls %s", call)
		}
	}
	for _, win32 := range []string{"@CreateProcessA(", "@DuplicateHandle(", "@WaitForSingleObject(", "@TerminateProcess("} {
		if strings.Contains(ir, win32) {
			t.Errorf("std/proc on the host emits the Windows call %s", win32)
		}
	}
}

// TestWindowsProcessLayouts pins std/platform/process_windows.kg's two layout(C)
// records against mingw-w64's processthreadsapi.h on x86_64:
//
//	typedef struct _STARTUPINFOA {
//	  DWORD cb; LPSTR lpReserved, lpDesktop, lpTitle;
//	  DWORD dwX, dwY, dwXSize, dwYSize, dwXCountChars, dwYCountChars,
//	        dwFillAttribute, dwFlags;
//	  WORD wShowWindow, cbReserved2; LPBYTE lpReserved2;
//	  HANDLE hStdInput, hStdOutput, hStdError;
//	} STARTUPINFOA;
//
//	typedef struct _PROCESS_INFORMATION {
//	  HANDLE hProcess; HANDLE hThread; DWORD dwProcessId; DWORD dwThreadId;
//	} PROCESS_INFORMATION;
func TestWindowsProcessLayouts(t *testing.T) {
	t.Parallel()
	src := windowsFile(t, "platform", "process_windows.kg")

	startup := parseLayoutCFields(t, src, "StartupInfoA")
	wantStartup := map[string]int64{
		"cb": 0, "reserved": 8, "desktop": 16, "title": 24,
		"x": 32, "y": 36, "xSize": 40, "ySize": 44, "xChars": 48, "yChars": 52,
		"fillAttribute": 56, "flags": 60, "showWindow": 64, "reserved2Count": 66,
		"reserved2": 72, "stdInput": 80, "stdOutput": 88, "stdError": 96,
	}
	offsets := layoutCOffsets(startup, windowsAmd64Size)
	for name, want := range wantStartup {
		if got, ok := offsets[name]; !ok || got != want {
			t.Errorf("STARTUPINFOA.%s offset = %d (present %v), want %d", name, got, ok, want)
		}
	}
	if got := offsets["stdError"] + windowsAmd64Size(fieldType(startup, "stdError")); got != 104 {
		t.Errorf("sizeof(STARTUPINFOA) = %d, want 104", got)
	}

	// cb must be the record's C size on the running target, not a literal
	// that would be wrong on 32-bit Windows.
	if !regexp.MustCompile(`info\.cb = ffi\.sizeOf\(&info\)`).MatchString(src) {
		t.Error("std/platform/process_windows.kg does not derive STARTUPINFOA.cb from ffi.sizeOf")
	}

	info := parseLayoutCFields(t, src, "ProcessInformation")
	wantInfo := map[string]int64{"process": 0, "thread": 8, "processId": 16, "threadId": 20}
	infoOffsets := layoutCOffsets(info, windowsAmd64Size)
	for name, want := range wantInfo {
		if got, ok := infoOffsets[name]; !ok || got != want {
			t.Errorf("PROCESS_INFORMATION.%s offset = %d (present %v), want %d", name, got, ok, want)
		}
	}
	if got := infoOffsets["threadId"] + windowsAmd64Size(fieldType(info, "threadId")); got != 24 {
		t.Errorf("sizeof(PROCESS_INFORMATION) = %d, want 24", got)
	}
}

func fieldType(fields []layoutCField, name string) string {
	for _, f := range fields {
		if f.name == name {
			return f.typ
		}
	}
	return ""
}

// windowsProcessPureSrc is a comptime unit test of the Windows process
// platform's pure state machine: command-line quoting, the signal-to-status
// mapping, the wait result classification, handle token encoding and error
// text. It is evaluated by the compiler for the Windows target and needs no
// Windows host, so a wrong transition fails this test at compile time.
const windowsProcessPureSrc = `import platform from std/platform
import test from std/test

const quoting: Bool = comptime {
    test.assert((platform.quoteArgument("plain") || "x") == "plain", "plain stays unquoted")
    test.assert((platform.quoteArgument("a b") || "x") == "\"a b\"", "space is quoted")
    test.assert((platform.quoteArgument("a\tb") || "x") == "\"a\tb\"", "tab is quoted")
    test.assert((platform.quoteArgument("") || "x") == "\"\"", "empty is an empty quoted argument")
    test.assert((platform.quoteArgument("a\"b") || "x") == "\"a\\\"b\"", "quote is escaped")
    test.assert((platform.quoteArgument("a b\\") || "x") == "\"a b\\\\\"", "trailing backslash is doubled")
    test.assert((platform.quoteArgument("a b\\\"c") || "x") == "\"a b\\\\\\\"c\"", "backslash before a quote is doubled")
    test.assert((platform.quoteArgument("日本 語") || "x").len() == 12, "multibyte bytes survive quoting")
    true
}

const commandLines: Bool = comptime {
    test.assert((platform.commandLine("prog.exe", Array.of()) || "x") == "prog.exe", "program only")
    test.assert((platform.commandLine("C:\\a b\\prog.exe", Array.of("one", "two three")) || "x") == "\"C:\\a b\\prog.exe\" one \"two three\"", "program and arguments")
    test.assert(match platform.commandLine("prog.exe", Array.of("a\0b")) { Ok(_) -> false, Err(_) -> true }, "NUL argument is refused")
    true
}

const signals: Bool = comptime {
    test.assert((platform.signalStatus(15) || 0) == 143, "Terminate reports 128 + 15")
    test.assert((platform.signalStatus(9) || 0) == 137, "Kill reports 128 + 9")
    test.assert(match platform.signalStatus(2) { Ok(_) -> false, Err(_) -> true }, "Interrupt is an explicit error")
    test.assert(match platform.signalStatus(0) { Ok(_) -> false, Err(_) -> true }, "an unknown number is an explicit error")
    true
}

const waits: Bool = comptime {
    test.assert(platform.waitOutcome(0) is platform.WaitOutcome.Exited, "WAIT_OBJECT_0 is an exit")
    test.assert(platform.waitOutcome(258) is platform.WaitOutcome.StillRunning, "WAIT_TIMEOUT is still running")
    test.assert(platform.waitOutcome(4294967295) is platform.WaitOutcome.Failed, "WAIT_FAILED is a failure")
    test.assert(platform.waitOutcome(128) is platform.WaitOutcome.Failed, "WAIT_ABANDONED is not an exit for a process")
    true
}

const tokens: Bool = comptime {
    test.assert(platform.handleToken(platform.tokenHandle(4)) == 4, "a positive token round-trips")
    test.assert(platform.handleToken(platform.tokenHandle(-1)) == -1, "a sign-extended token round-trips")
    test.assert(platform.tokenHandle(4) == 4, "a positive token widens unchanged")
    test.assert(platform.tokenHandle(-1) != 0, "a negative token widens to a full handle value")
    true
}

const errors: Bool = comptime {
    test.assert(platform.winErrorText(5).contains("denied"), "ERROR_ACCESS_DENIED has text")
    test.assert(platform.winErrorText(193).contains("Win32"), "ERROR_BAD_EXE_FORMAT has text")
    test.assert(platform.winErrorText(4242).contains("4242"), "an unknown code keeps its number")
    true
}

print "ok"
`

func TestWindowsProcessPureStateMachine(t *testing.T) {
	t.Parallel()
	_, diag, ok := compileForTarget(t, mustTarget(t, "x86_64-windows-gnu", ""), windowsProcessPureSrc)
	if !ok {
		t.Fatalf("the Windows process state machine's pure checks failed:\n%s", diag)
	}
}

// TestWindowsProcessPureStateMachineIsLive is the negative control: a
// deliberately wrong expectation must stop the compile, so a passing
// TestWindowsProcessPureStateMachine means the comptime blocks really ran.
func TestWindowsProcessPureStateMachineIsLive(t *testing.T) {
	t.Parallel()
	const wrong = `import platform from std/platform
import test from std/test

const wrongQuoting: Bool = comptime {
    test.assert((platform.quoteArgument("a b") || "x") == "a b", "quoting is not applied")
    true
}

print "ok"
`
	_, diag, ok := compileForTarget(t, mustTarget(t, "x86_64-windows-gnu", ""), wrong)
	if ok {
		t.Fatal("a wrong pure expectation compiled; the comptime checks are not running")
	}
	if !strings.Contains(diag, "E2240") || !strings.Contains(diag, "quoting is not applied") {
		t.Fatalf("a wrong expectation failed for another reason:\n%s", diag)
	}
}
