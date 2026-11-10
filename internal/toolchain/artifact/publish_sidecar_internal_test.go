package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPublishWithFingerprintWritesSidecarFirst: the sidecar lands before the output it describes, so no reader ever finds an artifact
// without the fingerprint of the build that produced it.
func TestPublishWithFingerprintWritesSidecarFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "lib.a")
	want := Fingerprint{CompilerRelease: "0.1.0", Target: "x86_64-linux-musl", RuntimeAbi: "kgabi1:abc"}
	ok, err := PublishWithFingerprint(out, want, func(staged string) error {
		time.Sleep(2 * time.Millisecond)
		return os.WriteFile(staged, []byte("archive"), 0o644)
	})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	side, err := os.Stat(FingerprintSidecarPath(out))
	if err != nil {
		t.Fatal(err)
	}
	art, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if side.ModTime().After(art.ModTime()) {
		t.Errorf("the sidecar was written after the artifact (sidecar %v, artifact %v)", side.ModTime(), art.ModTime())
	}
	got, ok, err := ReadFingerprintSidecar(out)
	if err != nil || !ok || got.Target != want.Target || got.RuntimeAbi != want.RuntimeAbi {
		t.Fatalf("sidecar = %+v, ok=%v err=%v", got, ok, err)
	}
}

// TestPublishWithFingerprintKeepsPreviousSidecarOnFailure: a failed build leaves both the old artifact and the fingerprint that
// describes it, never a sidecar for a build that never landed.
func TestPublishWithFingerprintKeepsPreviousSidecarOnFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out := filepath.Join(dir, "lib.a")
	good := Fingerprint{CompilerRelease: "0.1.0", Target: "x86_64-linux-musl"}
	if ok, err := PublishWithFingerprint(out, good, func(staged string) error {
		return os.WriteFile(staged, []byte("good"), 0o644)
	}); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	other := Fingerprint{CompilerRelease: "0.1.0", Target: "aarch64-freestanding"}
	ok, err := PublishWithFingerprint(out, other, func(staged string) error {
		return errors.New("archive failed")
	})
	if ok || err == nil {
		t.Fatalf("a failing build must report failure, got ok=%v err=%v", ok, err)
	}
	body, err := os.ReadFile(out)
	if err != nil || string(body) != "good" {
		t.Fatalf("the artifact must survive a failed build, got %q (%v)", body, err)
	}
	got, ok, err := ReadFingerprintSidecar(out)
	if err != nil || !ok || got.Target != good.Target {
		t.Fatalf("the sidecar must keep describing the artifact in place, got %+v ok=%v err=%v", got, ok, err)
	}
}
