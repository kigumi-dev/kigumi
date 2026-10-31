package schema

import (
	"fmt"

	"kigumi/internal/compiler/sem"
)

// GeneratorID names a derive generator; stable across releases, and part
// of a GeneratedUnit's cache key.
type GeneratorID string

const (
	GenEq      GeneratorID = "derive.eq"
	GenHash    GeneratorID = "derive.hash"
	GenEncode  GeneratorID = "derive.json.encode"
	GenDecode  GeneratorID = "derive.json.decode"
	GenDisplay GeneratorID = "derive.display"
)

type GeneratedUnit struct {
	Generator GeneratorID `json:"generator"`
	Origin    *DeclRef    `json:"origin"`
	CacheKey  string      `json:"cacheKey"`
	Text      string      `json:"text"`
}

// Generator is a deterministic function of a declaration's own Schema IR:
// no global registry, no runtime reflection, no cross-declaration reads.
type Generator interface {
	ID() GeneratorID
	Generate(decl *Declaration) (*GeneratedUnit, error)
}

// GenError's Kind is a stable string a caller maps to its own diagnostic code.
type GenError struct {
	Generator GeneratorID
	Decl      *DeclRef
	Kind      string
	Reason    string
}

func (e *GenError) Error() string {
	return fmt.Sprintf("%s on %s: %s", e.Generator, e.Decl.ID, e.Reason)
}

// DiagKind lets sem (which schema can't import back) pick a diagnostic
// code via method-set matching, without a type import cycle.
func (e *GenError) DiagKind() string { return e.Kind }

func declRef(decl *Declaration) *DeclRef {
	return &DeclRef{ID: decl.ID, Package: decl.Package, Name: decl.Name}
}

func cacheKey(id GeneratorID, decl *Declaration) string {
	return digest(struct {
		Generator GeneratorID
		Decl      *Declaration
	}{id, decl})
}

// generators is keyed by sem.DeriveRequest.Interface ("Eq", "Hash",
// "Encode", "Decode").
var generators = map[string]Generator{
	"Eq":      eqGenerator{},
	"Hash":    hashGenerator{},
	"Encode":  encodeGenerator{},
	"Decode":  decodeGenerator{},
	"Display": displayGenerator{},
}

// GenerateDerive matches sem.DeriveGenerator, for sem.CheckWith/driver.AnalyzeWith.
// It runs the generator twice and fails closed if the two runs disagree.
func GenerateDerive(res *sem.Result, req sem.DeriveRequest) (string, error) {
	d, gen, err := deriveInput(res, req)
	if err != nil {
		return "", err
	}
	return runTwice(gen, d)
}

func deriveInput(res *sem.Result, req sem.DeriveRequest) (*Declaration, Generator, error) {
	iface := req.Interface
	d := BuildOne(res, req.Decl)
	if d == nil {
		return nil, nil, &GenError{Generator: GeneratorID("derive." + iface), Decl: &DeclRef{}, Kind: "failed", Reason: "the entity has no declaring source node"}
	}
	gen, ok := generators[iface]
	if !ok {
		return nil, nil, &GenError{Generator: GeneratorID("derive." + iface), Decl: declRef(d), Kind: "unsupported", Reason: "no generator is registered for this interface"}
	}
	return d, gen, nil
}

func runTwice(gen Generator, d *Declaration) (string, error) {
	first, err := gen.Generate(d)
	if err != nil {
		return "", err
	}
	second, err := gen.Generate(d)
	if err != nil {
		return "", err
	}
	if first.Text != second.Text {
		return "", &GenError{Generator: gen.ID(), Decl: declRef(d), Kind: "failed", Reason: "two runs over the same declaration produced different source"}
	}
	return first.Text, nil
}
