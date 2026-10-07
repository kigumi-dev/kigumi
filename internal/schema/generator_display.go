package schema

import "fmt"

type displayGenerator struct{}

func (displayGenerator) ID() GeneratorID { return GenDisplay }

// Generate forwards to the compiler's own structural debug(): Display's
// contract never promises more than debug() already renders.
func (g displayGenerator) Generate(decl *Declaration) (*GeneratedUnit, error) {
	text := fmt.Sprintf("%spure fn %s.display(self) -> String {\n    debug(&self)\n}\n", protoVis(decl), genericInstance(decl))
	return &GeneratedUnit{Generator: GenDisplay, Origin: declRef(decl), CacheKey: cacheKey(GenDisplay, decl), Text: text}, nil
}
