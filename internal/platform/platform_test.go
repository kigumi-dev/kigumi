package platform_test

import (
	"testing"

	"kigumi/internal/platform"
)

func TestParseUntagged(t *testing.T) {
	if _, _, tagged := platform.Parse("main.kg"); tagged {
		t.Fatal("main.kg should carry no platform tag")
	}
}

func TestParseOS(t *testing.T) {
	os, arch, tagged := platform.Parse("net_linux.kg")
	if !tagged || os != "linux" || arch != "" {
		t.Fatalf("got os=%q arch=%q tagged=%v, want linux/\"\"/true", os, arch, tagged)
	}
}

func TestParseArch(t *testing.T) {
	os, arch, tagged := platform.Parse("util_arm64.kg")
	if !tagged || os != "" || arch != "arm64" {
		t.Fatalf("got os=%q arch=%q tagged=%v, want \"\"/arm64/true", os, arch, tagged)
	}
}

func TestParseOSArch(t *testing.T) {
	os, arch, tagged := platform.Parse("x_linux_arm64.kg")
	if !tagged || os != "linux" || arch != "arm64" {
		t.Fatalf("got os=%q arch=%q tagged=%v, want linux/arm64/true", os, arch, tagged)
	}
}

func TestParsePseudoNames(t *testing.T) {
	if os, _, tagged := platform.Parse("main_hosted.kg"); !tagged || os != "hosted" {
		t.Fatalf("got os=%q tagged=%v, want hosted/true", os, tagged)
	}
	if os, _, tagged := platform.Parse("main_bare.kg"); !tagged || os != "bare" {
		t.Fatalf("got os=%q tagged=%v, want bare/true", os, tagged)
	}
}

func TestParseAgreesWithFile(t *testing.T) {
	names := []string{"main.kg", "net_linux.kg", "util_arm64.kg", "x_linux_arm64.kg", "main_hosted.kg", "main_bare.kg", "main_test.kg"}
	targets := []struct{ os, arch string }{{"linux", "amd64"}, {"freestanding", "arm64"}, {"darwin", "arm64"}}
	for _, name := range names {
		_, _, tagged := platform.Parse(name)
		for _, tgt := range targets {
			wantTagged, _ := platform.File(name, tgt.os, tgt.arch)
			if tagged != wantTagged {
				t.Errorf("Parse(%q) tagged=%v disagrees with File(%q, %q, %q) tagged=%v", name, tagged, name, tgt.os, tgt.arch, wantTagged)
			}
		}
	}
}
