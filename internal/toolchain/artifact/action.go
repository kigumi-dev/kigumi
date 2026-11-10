package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ActionKeyVersion moves when the key's shape or the invocation fields it
// covers change, so an entry recorded by another layout never matches.
const ActionKeyVersion = 1

// Action is the identity of one external tool invocation: everything a
// cached result may depend on. A differing component must split the key, so
// a recorded output can never stand in for a different action.
type Action struct {
	// Tool names the program: a Framework and tool ("framework:toolchain:cc"),
	// or a built artifact ("artifact:writer").
	Tool string
	// Version is the tool's declared version; "" when the toolchain carries none.
	Version string
	// Runner is the toolchain build that composes the action, so upgrading
	// the driver invalidates entries it may now invoke differently.
	Runner string
	// Digest is the sha256 of the executable file the action runs.
	Digest string
	// Args are the fixed prefix arguments and the step's own arguments in
	// order, with $in/$out/$secret placeholders kept symbolic so the key
	// does not move with the output directory.
	Args []string
	// Env is every variable the child process sees.
	Env []string
	// Inputs are the content digests of the declared inputs, in order.
	Inputs []string
	// Outputs are the declared output names, in order.
	Outputs []string
}

// Key is the stable digest that names this action. Every field is
// length-prefixed, so ["ab","c"] and ["a","bc"] never collide, and the
// environment is canonicalized first, so a reordered allowlist keys the same.
func (a Action) Key() string {
	h := sha256.New()
	field := func(s string) {
		h.Write([]byte(strconv.Itoa(len(s))))
		h.Write([]byte{':'})
		h.Write([]byte(s))
	}
	list := func(xs []string) {
		field(strconv.Itoa(len(xs)))
		for _, s := range xs {
			field(s)
		}
	}
	field("kigumi-action")
	field(strconv.Itoa(ActionKeyVersion))
	field(a.Tool)
	field(a.Version)
	field(a.Runner)
	field(a.Digest)
	list(a.Args)
	list(CanonicalEnv(a.Env))
	list(a.Inputs)
	list(a.Outputs)
	return hex.EncodeToString(h.Sum(nil))
}

// CanonicalEnv is the environment a tool action runs with: the last value of
// each variable wins and the result is sorted by name, so a reordered
// allowlist neither splits the key nor changes what the child sees. An entry
// without '=' names no variable and is dropped.
func CanonicalEnv(env []string) []string {
	last := make(map[string]string, len(env))
	names := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if _, seen := last[name]; !seen {
			names = append(names, name)
		}
		last[name] = kv
	}
	sort.Strings(names)
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = last[name]
	}
	return out
}

// DigestFile is the sha256 hex of path's content, the digest action keys use
// for tool executables and declared inputs. The file is read once, streaming,
// so a large compiler binary is never held whole in memory.
func DigestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
