package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const isrDepHandler = `
import {ack, notify} from buflib

export(interrupt) {
    pub fn uart() {
        ack()
    }
    pub fn timer() {
        notify()
    }
}
`

func checkIsrDeps(t *testing.T, root sem.InterruptConfig, deps ...sem.DepInterrupt) *sem.Result {
	t.Helper()
	pkg := func(path, module, src string, std bool) *sem.Package {
		p := &sem.Package{Path: path, Module: module, Std: std}
		p.Entry = syntax.Parse(token.NewFile(path+"/"+path+".kg", []byte(src)))
		p.Files = []*syntax.Tree{p.Entry}
		return p
	}
	pkgs := append(testkit.LoadStd(t, "../../../std"), pkg("buflib", "", isrDeviceLib, true), pkg("deplib", "deplib", isrDepHandler, false), pkg("main", "", "", false))
	return sem.CheckWith(&sem.Module{Packages: pkgs, IsrContracts: isrDeviceContracts, Interrupt: root, DepInterrupt: deps}, schema.GenerateDerive)
}

func depRecord(handlers map[string][]string) sem.DepInterrupt {
	return sem.DepInterrupt{Module: "deplib", Config: sem.InterruptConfig{Handlers: handlers}}
}

func TestInterruptDependencyNarrowsItsHandler(t *testing.T) {
	root := sem.InterruptConfig{Root: []string{isrAck, isrNotify}}
	if out := checkIsrDeps(t, root).Render(); strings.Contains(out, "E10") {
		t.Fatalf("both handlers use the root common configuration:\n%s", out)
	}
	res := checkIsrDeps(t, root, depRecord(map[string][]string{"deplib.uart": {isrNotify}, "deplib.timer": {isrNotify}}))
	out := res.Render()
	for _, want := range []string{"E1088", "handler `uart` reaches `" + isrAck + "`", "`uart` uses its own interrupt configuration, which permits `" + isrNotify + "`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("a dependency record must narrow its handler (missing %q):\n%s", want, out)
		}
	}
	if strings.Contains(out, "handler `timer`") {
		t.Fatalf("timer is still permitted its sync:\n%s", out)
	}
}

func TestInterruptDependencyCannotWidenRoot(t *testing.T) {
	root := sem.InterruptConfig{Root: []string{isrNotify}}
	out := checkIsrDeps(t, root, depRecord(map[string][]string{"deplib.uart": {isrAck}})).Render()
	for _, want := range []string{"E1091", "the `Interrupt` record of dependency `deplib` for handler `deplib.uart` names `" + isrAck + "`, which is outside the root limit", "E1088", "handler `uart` reaches `" + isrAck + "`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("a dependency record outside the root limit must be rejected and grant nothing (missing %q):\n%s", want, out)
		}
	}
	out = checkIsrDeps(t, root, depRecord(map[string][]string{"deplib.uart": {"mmio nope.op"}})).Render()
	if !strings.Contains(out, "E1091") || !strings.Contains(out, "matches no reviewed contract") {
		t.Fatalf("a dependency record naming an unknown permission must be rejected:\n%s", out)
	}
}

func TestInterruptDependencyRootRecordCannotLoosen(t *testing.T) {
	root := sem.InterruptConfig{Root: []string{isrAck, isrNotify}, Handlers: map[string][]string{"deplib.uart": {isrAck, isrNotify}}}
	out := checkIsrDeps(t, root, depRecord(map[string][]string{"deplib.uart": {isrNotify}})).Render()
	if !strings.Contains(out, "E1088") || !strings.Contains(out, "handler `uart` reaches `"+isrAck+"`") {
		t.Fatalf("the root's handler record must not widen what the dependency's record narrowed:\n%s", out)
	}
}

func TestInterruptDependencyRecordShape(t *testing.T) {
	root := sem.InterruptConfig{Root: []string{isrAck, isrNotify}}
	common := sem.DepInterrupt{Module: "deplib", Config: sem.InterruptConfig{Root: []string{isrAck}}}
	out := checkIsrDeps(t, root, common).Render()
	if strings.Contains(out, "E2141") || strings.Contains(out, "error") {
		t.Fatalf("a dependency's common record is its own root limit and must not affect the consumer:\n%s", out)
	}
	out = checkIsrDeps(t, root, depRecord(map[string][]string{"main.uart": nil})).Render()
	if !strings.Contains(out, "E2141") || !strings.Contains(out, "names handler `main.uart`, which belongs to another module") {
		t.Fatalf("a dependency must not configure another module's handler:\n%s", out)
	}
	out = checkIsrDeps(t, root, depRecord(map[string][]string{"deplib.gone": nil})).Render()
	if !strings.Contains(out, "E1092") || !strings.Contains(out, "names handler `deplib.gone`") {
		t.Fatalf("a dependency record keyed to a missing handler must be rejected:\n%s", out)
	}
}

func TestInterruptConfigWithoutMatchingHandler(t *testing.T) {
	out := checkIsrWith(t, "", "", isrDeviceContracts, sem.InterruptConfig{Root: []string{"mmio nope.op"}}).Render()
	if !strings.Contains(out, "E1091") || !strings.Contains(out, "the root `Interrupt` record names `mmio nope.op`, which matches no reviewed contract") {
		t.Fatalf("a root permission matching no contract must be reported even with no handler:\n%s", out)
	}
	cfg := sem.InterruptConfig{Handlers: map[string][]string{"main.gone": {"mmio nope.op", isrNotify}}}
	out = checkIsrWith(t, isrDeviceLib, "", isrDeviceContracts, cfg).Render()
	for _, want := range []string{"E1091", "the `Interrupt` record for handler `main.gone` names `mmio nope.op`, which matches no reviewed contract", "names `" + isrNotify + "`, which is outside the root limit", "E1092", "names handler `main.gone`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("a record for a missing handler must still be checked (missing %q):\n%s", want, out)
		}
	}
}
