package driver_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// uefiBootCase is one EFI machine: the triple to build, the lld-link machine
// and firmware application name, the QEMU system emulator that runs it, and
// the C shim flags that target's ABI needs.
type uefiBootCase struct {
	triple    string
	machine   string
	app       string
	qemu      string
	shimFlags []string
}

// TestUEFIBoots is the QEMU/OVMF half of the EFI contract; CI sets KIGUMI_UEFI_BOOT=1 after
// installing qemu, OVMF (x86_64) and ovmf-ia32 (x86), while ordinary driver runs skip it.
func TestUEFIBoots(t *testing.T) {
	if os.Getenv("KIGUMI_UEFI_BOOT") == "" {
		t.Skip("set KIGUMI_UEFI_BOOT=1 to run the firmware boot")
	}
	if runtime.GOOS != "linux" {
		t.Skip("OVMF boot test needs qemu-system-x86_64")
	}
	requireZig(t)
	// A 32-bit UEFI application is the one 32-bit target with a firmware
	// runner: the same target whose archive needs the runtime's own 64-bit
	// division helpers instead of an MSVC CRT.
	for _, c := range []uefiBootCase{
		{triple: "x86_64-uefi", machine: "x64", app: "BOOTX64.EFI", qemu: "qemu-system-x86_64", shimFlags: []string{"-mno-red-zone"}},
		{triple: "x86-uefi", machine: "x86", app: "BOOTIA32.EFI", qemu: "qemu-system-i386"},
	} {
		t.Run(c.machine, func(t *testing.T) { bootUEFI(t, c) })
	}
}

func bootUEFI(t *testing.T, c uefiBootCase) {
	t.Helper()
	qemu, err := exec.LookPath(c.qemu)
	if err != nil {
		t.Skipf("%s is not on PATH", c.qemu)
	}
	root := t.TempDir()
	// x86 has only split firmware images (a 4 MiB code flash plus a variable
	// store); x86_64 also has a unified OVMF.fd that -bios can map.
	var fwArgs []string
	if c.machine == "x86" {
		code, vars := uefi32Firmware(t, root)
		fwArgs = []string{
			"-drive", "if=pflash,format=raw,readonly=on,file=" + code,
			"-drive", "if=pflash,format=raw,file=" + vars,
		}
	} else {
		ovmf := uefi64Firmware(t)
		fwArgs = []string{"-bios", ovmf}
	}

	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"uefiboot\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	target, err := buildconfig.ParseTarget(testCompiler(), c.triple, "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "app.a")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, archive, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	shim, _ := filepath.Abs("testdata/uefi_shim.c")
	obj := filepath.Join(root, "shim.obj")
	cc := testCompiler().CCompiler()
	run := func(name string, args ...string) {
		t.Helper()
		out, err := exec.Command(name, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	ccArgs := append([]string{"cc", "-target", c.triple, "-ffreestanding", "-fno-stack-protector", "-fno-sanitize=all"}, c.shimFlags...)
	ccArgs = append(ccArgs, "-c", shim, "-o", obj)
	run(cc[0], ccArgs...)
	exe := filepath.Join(root, c.app)
	run(cc[0], "lld-link", "/subsystem:efi_application", "/entry:efi_main", "/nodefaultlib", "/machine:"+c.machine, "/out:"+exe, obj, archive)
	esp := filepath.Join(root, "esp", "EFI", "BOOT")
	if err := os.MkdirAll(esp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(esp, c.app), mustRead(t, exe), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	args := append([]string{"-machine", "q35", "-cpu", "max", "-m", "256"}, fwArgs...)
	args = append(args,
		"-drive", "format=raw,file=fat:rw:"+filepath.Join(root, "esp"),
		"-device", "isa-debug-exit,iobase=0x501,iosize=0x04",
		"-serial", "stdio", "-display", "none", "-no-reboot")
	cmd := exec.CommandContext(ctx, qemu, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("qemu timed out\n%s", out)
	}
	if !strings.Contains(string(out), "EFI-RET") {
		t.Fatalf("firmware boot did not reach EFI-RET (qemu err %v)\n%s", err, out)
	}
}

// uefi64Firmware finds the 64-bit OVMF image CI installs or a local one.
func uefi64Firmware(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("OVMF"); p != "" {
		return p
	}
	for _, p := range []string{"/usr/share/ovmf/OVMF.fd", "/usr/share/OVMF/OVMF.fd", "/usr/share/OVMF/OVMF_CODE.fd", "/usr/share/OVMF/OVMF_CODE_4M.fd"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no 64-bit OVMF firmware image")
	return ""
}

// uefi32Firmware returns the 32-bit code image and a writable copy of its
// variable store. QEMU needs the pair as pflash drives because the image is
// too large for the legacy -bios mapping.
func uefi32Firmware(t *testing.T, root string) (code, vars string) {
	t.Helper()
	code = os.Getenv("OVMF32")
	if code == "" {
		for _, p := range []string{"/usr/share/OVMF/OVMF32_CODE_4M.fd", "/usr/share/ovmf/OVMF32_CODE_4M.fd", "/usr/share/qemu/OVMF32_CODE_4M.fd"} {
			if _, err := os.Stat(p); err == nil {
				code = p
				break
			}
		}
	}
	if code == "" {
		t.Skip("no 32-bit OVMF code image (install ovmf-ia32)")
	}
	source := os.Getenv("OVMF32_VARS")
	if source == "" {
		for _, p := range []string{"/usr/share/OVMF/OVMF32_VARS_4M.fd", "/usr/share/ovmf/OVMF32_VARS_4M.fd", "/usr/share/qemu/OVMF32_VARS_4M.fd"} {
			if _, err := os.Stat(p); err == nil {
				source = p
				break
			}
		}
	}
	if source == "" {
		t.Skip("no 32-bit OVMF variable store (install ovmf-ia32)")
	}
	vars = filepath.Join(root, "OVMF32_VARS.fd")
	if err := os.WriteFile(vars, mustRead(t, source), 0o644); err != nil {
		t.Fatal(err)
	}
	return code, vars
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
