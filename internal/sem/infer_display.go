package sem

// requestDisplay asks for the derived `display` of t when `std/fmt` is loaded, so
// `p.display()` works without a generic bound; the first pass reports nothing.
func (c *checker) requestDisplay(t TypeID) bool {
	pkg, ok := c.r.pathIndex["std/fmt"]
	if !ok {
		return false
	}
	b, ok := c.r.Scopes[c.r.Packages[pkg].Scope].Names["Display"]
	if !ok || b.Ent == 0 {
		return false
	}
	return c.r.requestDerive(t, c.r.Types.Iface(b.Ent, nil))
}

// noteDerivedDisplayUse counts the file's `std/fmt` import as used when m is a generated `display`.
func (c *checker) importsFmt() bool {
	for i := range c.r.Entities {
		e := &c.r.Entities[i]
		if e.Kind == EntImport && e.File == c.f && e.Target != 0 && c.r.Entities[e.Target].Kind == EntPackage && c.r.Packages[c.r.Entities[e.Target].Pkg].Path == "std/fmt" {
			return true
		}
	}
	return false
}

func (c *checker) displayMissingHelp(args []any) string {
	if !c.importsFmt() {
		return "a type gets a derived `display` once `std/fmt` is imported: `import fmt from std/fmt`"
	}
	if len(args) > 0 {
		if t, ok := args[0].(TypeID); ok && c.r.Types.Kind(t) == KPrim {
			return "`std/fmt` derives `display` for records and enums only; interpolate a built-in value as `\"${value}\"`"
		}
	}
	return "`std/fmt` is imported, but this type has no derivable `display`; define `fn Type.display(self) -> String` or print it with `debug`"
}

func (c *checker) noteDerivedDisplayUse(m EntityID) {
	if c.r.Entities[m].Name != "display" || !c.r.Files[c.r.Entities[m].File].Tree.File.Generated {
		return
	}
	for i := range c.r.Entities {
		e := &c.r.Entities[i]
		if e.Kind == EntImport && e.File == c.f && e.Target != 0 && c.r.Entities[e.Target].Kind == EntPackage && c.r.Packages[c.r.Entities[e.Target].Pkg].Path == "std/fmt" {
			e.Flags |= EfUsed
		}
	}
}

// displayHelp names the step that works for t: its own `display`, else `debug`.
func (c *checker) displayHelp(t TypeID) string {
	if c.r.memberSet(t, "display") != 0 {
		return "call `.display()`"
	}
	return ""
}
