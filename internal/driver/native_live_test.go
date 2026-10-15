package driver_test

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const nativeLiveFile = "../../testdata/native_live.txt"

var (
	nativeLiveRe  = regexp.MustCompile(`(?m)^kigumi-live: (\d+)\n`)
	nativeLiveMu  sync.Mutex
	nativeLiveGot = map[string]int64{}
	nativeLiveRan = map[string]bool{}
)

func countsNativeLive() bool {
	return slices.Contains(strings.Fields(os.Getenv("KIGUMI_CFLAGS")), "-DKIGUMI_RC_CHECK")
}

func loadNativeLive(t *testing.T) map[string]int64 {
	data, err := os.ReadFile(nativeLiveFile)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]int64{}
	for _, l := range strings.Split(string(data), "\n") {
		name, n, ok := strings.Cut(l, " ")
		if !ok {
			continue
		}
		v, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			t.Fatalf("%s: %q", nativeLiveFile, l)
		}
		known[name] = v
	}
	return known
}

func splitNativeLive(stderr string) (string, int64, bool) {
	m := nativeLiveRe.FindStringSubmatch(stderr)
	if m == nil {
		return stderr, 0, false
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	return nativeLiveRe.ReplaceAllString(stderr, ""), n, true
}

func noteNativeLive(t *testing.T, name string, n int64, known map[string]int64) {
	nativeLiveMu.Lock()
	nativeLiveRan[name] = true
	if n != 0 {
		nativeLiveGot[name] = n
	}
	nativeLiveMu.Unlock()
	if n > known[name] && os.Getenv("UPDATE") == "" {
		t.Errorf("%d objects live at exit, %d allowed", n, known[name])
	}
}

func finishNativeLive(t *testing.T, known map[string]int64) {
	nativeLiveMu.Lock()
	defer nativeLiveMu.Unlock()
	if os.Getenv("UPDATE") != "" {
		merged := map[string]int64{}
		for name, n := range known {
			if !nativeLiveRan[name] {
				merged[name] = n
			}
		}
		for name, n := range nativeLiveGot {
			merged[name] = n
		}
		var names []string
		for name := range merged {
			names = append(names, name)
		}
		sort.Strings(names)
		var sb strings.Builder
		for _, name := range names {
			fmt.Fprintf(&sb, "%s %d\n", name, merged[name])
		}
		os.WriteFile(nativeLiveFile, []byte(sb.String()), 0o644)
		return
	}
	for name, n := range known {
		if nativeLiveRan[name] && nativeLiveGot[name] < n {
			t.Errorf("%s: %d objects live at exit, below the %d listed (UPDATE=1 to tighten)", name, nativeLiveGot[name], n)
		}
	}
}
