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

// TestUEFIBoots is the QEMU/OVMF half of the EFI contract; CI sets KIGUMI_UEFI_BOOT=1 after
// installing qemu and OVMF, while ordinary driver runs skip it.
func TestUEFIBoots(t *testing.T) {
	if os.Getenv("KIGUMI_UEFI_BOOT") == "" {
		t.Skip("set KIGUMI_UEFI_BOOT=1 to run the firmware boot")
	}
	if runtime.GOOS != "linux" {
		t.Skip("OVMF boot test needs qemu-system-x86_64")
	}
	requireZig(t)
	qemu, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		t.Skip("qemu-system-x86_64 is not on PATH")
	}
	ovmf := os.Getenv("OVMF")
	if ovmf == "" {
		for _, p := range []string{"/usr/share/ovmf/OVMF.fd", "/usr/share/OVMF/OVMF_CODE.fd", "/usr/share/OVMF/OVMF_CODE_4M.fd"} {
			if _, err := os.Stat(p); err == nil {
				ovmf = p
				break
			}
		}
	}
	if ovmf == "" {
		t.Skip("no OVMF firmware image")
	}

	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"uefiboot\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-uefi", "")
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
	run(cc[0], "cc", "-target", "x86_64-uefi", "-ffreestanding", "-fno-stack-protector", "-fno-sanitize=all", "-mno-red-zone", "-c", shim, "-o", obj)
	exe := filepath.Join(root, "BOOTX64.EFI")
	run(cc[0], "lld-link", "/subsystem:efi_application", "/entry:efi_main", "/nodefaultlib", "/machine:x64", "/out:"+exe, obj, archive)
	esp := filepath.Join(root, "esp", "EFI", "BOOT")
	if err := os.MkdirAll(esp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(esp, "BOOTX64.EFI"), mustRead(t, exe), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, qemu,
		"-machine", "q35", "-cpu", "max", "-m", "256", "-bios", ovmf,
		"-drive", "format=raw,file=fat:rw:"+filepath.Join(root, "esp"),
		"-device", "isa-debug-exit,iobase=0x501,iosize=0x04",
		"-serial", "stdio", "-display", "none", "-no-reboot")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("qemu timed out\n%s", out)
	}
	if !strings.Contains(string(out), "EFI-RET") {
		t.Fatalf("firmware boot did not reach EFI-RET (qemu err %v)\n%s", err, out)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
