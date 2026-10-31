package schema

import "fmt"

// jsonShapeError rejects a record whose fields or metadata the generated codec cannot honor.
func jsonShapeError(id GeneratorID, decl *Declaration, encode bool) error {
	seen := map[string]string{}
	for _, f := range decl.Fields {
		key := jsonKey(f)
		if first, dup := seen[key]; dup {
			return genFailure(id, decl, fmt.Sprintf("fields `%s` and `%s` share the JSON key %q", first, f.Name, key))
		}
		seen[key] = f.Name
		if encode && jsonOmitNone(f) && (f.Type == nil || f.Type.WellKnown != WKOption) {
			return genFailure(id, decl, fmt.Sprintf("`json.omitNone` on field `%s` needs an Option type", f.Name))
		}
	}
	return nil
}

func genFailure(id GeneratorID, decl *Declaration, reason string) error {
	return &GenError{Generator: id, Decl: declRef(decl), Kind: "failed", Reason: reason}
}
