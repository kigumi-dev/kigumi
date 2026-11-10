package driver_test

import (
	"io"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func BenchmarkReplWarmCell(b *testing.B) {
	root := b.TempDir()
	std, _ := filepath.Abs("../../../std")
	r, err := driver.NewRepl(root, driver.ReplOptions{
		Load: driver.LoadOptions{StdRoot: std, Compiler: testCompilerAt(root)},
		Out:  io.Discard,
		Err:  io.Discard,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	if res, err := r.Eval("let x = 1", true); err != nil || res.Status != 0 {
		b.Fatalf("warmup: %+v %v", res, err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := r.Eval("x + 1", true)
		if err != nil || res.Status != 0 || res.Display != "2" {
			b.Fatalf("cell: %+v %v", res, err)
		}
	}
}

func BenchmarkReplDeclaration(b *testing.B) {
	root := b.TempDir()
	std, _ := filepath.Abs("../../../std")
	r, err := driver.NewRepl(root, driver.ReplOptions{
		Load: driver.LoadOptions{StdRoot: std, Compiler: testCompilerAt(root)},
		Out:  io.Discard,
		Err:  io.Discard,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := r.Eval("fn f"+itoa(i)+"() -> Int { 1 }", true)
		if err != nil || res.Status != 0 {
			b.Fatalf("decl: %+v %v", res, err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
