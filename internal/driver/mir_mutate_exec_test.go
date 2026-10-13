package driver_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

const (
	mutantStepLimit  = 400_000
	mutantDepthLimit = 150
	mutantMemory     = 64 << 20
	mutantTimeout    = 20 * time.Second
)

func crashSite(stack []byte) string {
	lines := strings.Split(string(stack), "\n")
	start := 0
	for i, l := range lines {
		if strings.HasPrefix(l, "panic(") || strings.HasPrefix(l, "runtime.gopanic") {
			start = i
		}
	}
	var out []string
	for i := start; i < len(lines) && len(out) < 3; i++ {
		if strings.HasPrefix(lines[i], "kigumi/") && !strings.Contains(lines[i], "driver_test.") && i+1 < len(lines) {
			loc := strings.Fields(lines[i+1])[0]
			out = append(out, loc[strings.LastIndex(loc, "internal/"):])
		}
	}
	return strings.Join(out, " < ")
}

func guard(what string, run func()) (finding string) {
	defer func() {
		if rec := recover(); rec != nil {
			finding = fmt.Sprintf("%s crashed: %v at %s", what, rec, crashSite(debug.Stack()))
		}
	}()
	run()
	return ""
}

func emitMutant(p *mir.Program) string {
	var ir string
	if f := guard("llgen", func() { ir = llgen.EmitProgram(p, llgen.EmitOptions{}).IR }); f != "" {
		return f
	}
	if os.Getenv("KIGUMI_MIR_FUZZ_IR") == "" {
		return ""
	}
	return compileIR(ir)
}

func compileIR(ir string) string {
	cc, err := exec.LookPath("zig")
	if err != nil {
		return ""
	}
	dir, err := os.MkdirTemp("", "kigumi-fuzz-ir")
	if err != nil {
		return "temp dir: " + err.Error()
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "m.ll")
	if err := os.WriteFile(src, []byte(ir), 0o644); err != nil {
		return "write IR: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, cc, "cc", "-c", "-O0", "-Wno-override-module", src, "-o", filepath.Join(dir, "m.o")).CombinedOutput()
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(string(out))
	if len(msg) > 400 {
		msg = msg[:400]
	}
	return "the C compiler refused the IR: " + msg
}

type vmOutcome string

const (
	vmExit0    vmOutcome = "exit 0"
	vmExitN    vmOutcome = "exit nonzero"
	vmPanicked vmOutcome = "kigumi panic"
	vmRefused  vmOutcome = "unsupported"
)

func runMutant(p *mir.Program) (vmOutcome, string) {
	type result struct {
		out     vmOutcome
		finding string
	}
	done := make(chan result, 1)
	go func() {
		var res result
		res.finding = guard("vm", func() {
			code, err := vm.Run(p, vm.Options{Sandbox: true, StepLimit: mutantStepLimit, DepthLimit: mutantDepthLimit, MemoryLimit: mutantMemory}, io.Discard, io.Discard)
			var unsupported *vm.Unsupported
			switch {
			case errors.As(err, &unsupported):
				res.out = vmRefused
			case err != nil || code == 1:
				res.out = vmExitN
			case code == 2:
				res.out = vmPanicked
			default:
				res.out = vmExit0
			}
		})
		done <- res
	}()
	select {
	case r := <-done:
		return r.out, r.finding
	case <-time.After(mutantTimeout):
		return "", fmt.Sprintf("vm ran past %s inside the step budget", mutantTimeout)
	}
}
