package borrow

import (
	"testing"
)

func TestKeptReferentDeclaredSignature(t *testing.T) {
	p, _ := srcProgram(t, `import cli from std/cli
fn flags(p: &mut cli.Parser, n: &mut Int) { p.flag("n", n) }
fn strings(p: &mut cli.Parser, n: &mut Array[String], s: &mut String?) {
    p.flag("n", n)
    p.arg("s", s)
}
`)
	if p.HasErrors() {
		t.Fatal(p.Render())
	}
}

func TestKeptWitnessDeclaredSignature(t *testing.T) {
	p, _ := srcProgram(t, `
import io from std/io
import net from std/net
fn readExistential(mut r: io.Reader, max: usize) -> Bytes! {
    let mut buf = Array.empty[u8]()
    let _ = r.read(&mut buf, max)?
    Bytes.fromArray(&buf)
}
fn use(mut r: net.Conn) -> Bytes! { readExistential(r, 64) }
fn copy(mut r: net.Conn, mut w: net.Conn) -> usize! { io.copy(r, w) }
`)
	if p.HasErrors() {
		t.Fatal(p.Render())
	}
}
