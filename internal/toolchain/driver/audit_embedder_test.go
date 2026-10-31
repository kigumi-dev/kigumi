package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

func buildFreestanding(t *testing.T, manifestExtra string) error {
	t.Helper()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := writeAuditModule(t, "hal", "extern void hal_gpio_write(int pin, int level);\nvoid blink(void) { hal_gpio_write(3, 1); }\n")
	mod, _ := os.ReadFile(filepath.Join(root, "mod.kg"))
	os.WriteFile(filepath.Join(root, "mod.kg"), append(mod, manifestExtra...), 0o644)
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	var buildErr bytes.Buffer
	_, err = driver.BuildWith(m, filepath.Join(root, "hal.a"), &buildErr, driver.BuildOptions{Target: target})
	return err
}

func TestAuditAcceptsEmbedderSymbols(t *testing.T) {
	t.Parallel()
	err := buildFreestanding(t, "")
	if err == nil || !strings.Contains(err.Error(), "hal_gpio_write") || !strings.Contains(err.Error(), "Embedder record") {
		t.Fatalf("an unlisted symbol must fail the audit and point at the Embedder record, got %v", err)
	}
	if err := buildFreestanding(t, "Embedder { symbol: \"hal_gpio_write\" }\n"); err != nil {
		t.Fatalf("an Embedder symbol must pass the audit: %v", err)
	}
}

func TestEmbedderRecordParses(t *testing.T) {
	parse := func(src string) (driver.ModFile, []diag.Diagnostic) {
		return driver.ParseModFile(token.NewFile("mod.kg", []byte("Module { name: \"a\", kigumi: \"0.1\" }\n"+src)))
	}
	mf, ds := parse("Embedder { symbol: \"hal_gpio_write\" }\nEmbedder { symbol: \"_start\" }\n")
	if len(ds) != 0 || len(mf.Embedder) != 2 || mf.Embedder[0] != "hal_gpio_write" || mf.Embedder[1] != "_start" {
		t.Fatalf("got %v %v", mf.Embedder, ds)
	}
	for _, bad := range []string{"Embedder { }\n", "Embedder { symbol: \"1x\" }\n", "Embedder { symbol: \"a-b\" }\n", "Embedder { name: \"x\" }\n"} {
		if _, ds := parse(bad); len(ds) == 0 {
			t.Errorf("%q must be rejected", bad)
		}
	}
}
