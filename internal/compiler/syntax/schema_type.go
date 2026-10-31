package syntax

var typeSchema = []kindSchema{
	{TypePath, "TypePath", layLR, "Head", []slot{ch("Path", set(Path)), olst("Args", typeSet.or(set(GenericParam)))}},
	{TypeLifetime, "TypeLifetime", layLeaf, "Name", nil},
	{TypeOptional, "TypeOptional", layL, "Op", []slot{ch("Inner", typeSet)}},
	{TypeResult, "TypeResult", layL, "Op", []slot{ch("Inner", typeSet)}},
	{TypeFn, "TypeFn", layRec, "Head", []slot{num("Mods"), otk("Abi"), lst("Params", typeSet), num("Flags"), ch("Ret", typeSet)}},
	{TypePtr, "TypePtr", layFlagR, "Op", []slot{num("Flags"), ch("Pointee", typeSet)}},
	{TypeRef, "TypeRef", layFlagR, "Op", []slot{num("Flags"), ch("Pointee", typeSet)}},
	{TypeTuple, "TypeTuple", layList, "Open", []slot{items(typeSet)}},
	{Path, "Path", layPath, "Head", []slot{toks()}},
}
