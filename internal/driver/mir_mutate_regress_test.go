package driver_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/mir"
)

func TestFixedMutants(t *testing.T) {
	for i, c := range pinnedMutants {
		t.Run(c.base+"/"+strings.Fields(c.recipe)[0]+"/"+strconv.Itoa(i), func(t *testing.T) {
			rec, err := parseRecipe(c.recipe)
			if err != nil {
				t.Fatal(err)
			}
			base := analyzeFixture(t, fixtureFile(c.base))
			p := cloneProgram(base)
			what, err := newMutator(base).replay(p, rec)
			if err != nil {
				t.Fatalf("%v: pin it again from a fresh fuzz finding", err)
			}
			err = p.Verify()
			if c.rule != "" {
				if ve, ok := err.(*mir.Error); !ok || ve.Rule != c.rule {
					t.Fatalf("%s: got %v, want rule %s", what, err, c.rule)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: rejected: %v", what, err)
			}
			if f := emitMutant(p); f != "" {
				t.Fatalf("%s: %s", what, f)
			}
			if _, f := runMutant(p); f != "" {
				t.Fatalf("%s: %s", what, f)
			}
		})
	}
}

// A recipe replays what its seed drew, through the text form, on a fresh copy of the base.
func TestRecipeReplaysTheSeed(t *testing.T) {
	for _, name := range mutantBases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base := analyzeFixture(t, fixtureFile(name))
			m := newMutator(base)
			for seed := int64(0); seed < 60; seed++ {
				p, _, rec := mutantOf(base, m, seed)
				text := rec.format(p)
				parsed, err := parseRecipe(text)
				if err != nil {
					t.Fatalf("seed %d: %v\n%s", seed, err, text)
				}
				q := cloneProgram(base)
				if _, err := m.replay(q, parsed); err != nil {
					t.Fatalf("seed %d: %v\n%s", seed, err, text)
				}
				if !reflect.DeepEqual(allFuncs(p), allFuncs(q)) {
					t.Fatalf("seed %d: the replayed program differs\n%s", seed, text)
				}
			}
		})
	}
}

func TestRecipeGoesStaleLoudly(t *testing.T) {
	base := analyzeFixture(t, fixtureFile("adt"))
	m := newMutator(base)
	for name, text := range map[string]string{
		"opcode moved":   `opcode-any "main.Point.bump" b0/0 move | i32`,
		"function gone":  `opcode-any "main.Point.nope" b0/0 cellget | i32`,
		"site gone":      `opcode-any "main.Point.bump" b0/99 cellget | i32`,
		"choice too big": `opcode-any "main.Point.bump" b0/0 cellget | i9999`,
		"choice missing": `opcode-any "main.Point.bump" b0/0 cellget`,
		"wrong choice":   `opcode-any "main.Point.bump" b0/0 cellget | l"x"`,
		"unused choice":  `opcode-any "main.Point.bump" b0/0 cellget | i32 i1`,
		"unknown local":  `operand "main.Point.bump" b0/0 cellget | i0 l"nothing"`,
		"unknown edit":   `jiggle "main.Point.bump" b0/0 cellget`,
	} {
		t.Run(name, func(t *testing.T) {
			rec, err := parseRecipe(text)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.replay(cloneProgram(base), rec); err == nil {
				t.Fatal("replayed a recipe that no longer describes the program")
			}
		})
	}
}
