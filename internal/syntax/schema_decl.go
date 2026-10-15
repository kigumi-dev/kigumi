package syntax

var declSchema = []kindSchema{
	{File, "File", layList, "", []slot{items(declOrStmt)}},
	{FnDecl, "FnDecl", layRec, "Kw", []slot{
		otk("Doc"), olst("Attrs", set(Attr, UnsafeGuarantee)), num("Mods"), och("Vis", set(Visibility)),
		och("Recv", set(TypePath)), tk("Name"), olst("Generics", set(GenericParam)), lst("Params", set(Param)),
		och("Ret", typeSet), och("Body", blk),
	}},
	{Param, "Param", layRec, "Name", []slot{olst("Attrs", set(Attr, UnsafeGuarantee)), num("Flags"), och("Type", typeSet)}},
	{GenericParam, "GenericParam", layFlagR, "Name", []slot{num("Flags"), och("Bound", typeSet.or(lists))}},
	{TypeDecl, "TypeDecl", layRec, "Kw", []slot{
		otk("Doc"), olst("Attrs", set(Attr, UnsafeGuarantee)), och("Vis", set(Visibility)), tk("Name"),
		olst("Generics", set(GenericParam)), och("Layout", set(LayoutClause)),
		och("Body", set(RecordBody, ResourceBody, AdtBody, AliasBody, DefinedBody)),
	}},
	{RecordBody, "RecordBody", layList, "Open", []slot{items(set(Field))}},
	{ResourceBody, "ResourceBody", layList, "Kw", []slot{items(set(Field))}},
	{AdtBody, "AdtBody", layList, "Head", []slot{items(set(Variant))}},
	{AliasBody, "AliasBody", layL, "Kw", []slot{ch("Type", typeSet)}},
	{DefinedBody, "DefinedBody", layL, "Head", []slot{ch("Type", typeSet)}},
	{Field, "Field", layRec, "Name", []slot{och("Vis", set(Visibility)), num("Flags"), och("Type", typeSet), och("Metadata", set(Metadata))}},
	{Variant, "Variant", layL, "Name", []slot{olst("Fields", set(VariantField))}},
	{VariantField, "VariantField", layFlagR, "Name", []slot{num("Flags"), ch("Type", typeSet)}},
	{Metadata, "Metadata", layList, "Open", []slot{items(set(MetaItem))}},
	{MetaItem, "MetaItem", layLR, "Head", []slot{ch("Path", set(Path)), olst("Args", exprSet)}},
	{InterfaceDecl, "InterfaceDecl", layRec, "Kw", []slot{
		otk("Doc"), olst("Attrs", set(Attr, UnsafeGuarantee)), och("Vis", set(Visibility)), tk("Name"),
		olst("Generics", set(GenericParam)), lst("Members", declOrStmt),
	}},
	{ConstDecl, "ConstDecl", layRec, "Kw", []slot{
		otk("Doc"), olst("Attrs", set(Attr, UnsafeGuarantee)), och("Vis", set(Visibility)), tk("Name"),
		och("Type", typeSet), ch("Value", exprSet),
	}},
	{StaticDecl, "StaticDecl", layRec, "Kw", []slot{
		otk("Doc"), olst("Attrs", set(Attr, UnsafeGuarantee)), num("Flags"), och("Vis", set(Visibility)), tk("Name"),
		och("Type", typeSet), och("Value", exprSet),
	}},
	{ContextKeyDecl, "ContextKeyDecl", layRec, "Kw", []slot{
		otk("Doc"), olst("Attrs", set(Attr, UnsafeGuarantee)), och("Vis", set(Visibility)), tk("Name"), och("Type", typeSet),
	}},
	{ImportDecl, "ImportDecl", layRec, "Kw", []slot{och("Vis", set(Visibility)), ch("Binding", set(Ident, List)), ch("Path", set(Path))}},
	{ImportAlias, "ImportAlias", layLR, "", []slot{ch("Name", set(Ident)), ch("Alias", set(Ident))}},
	{TestDecl, "TestDecl", layTokR, "Kw", []slot{tk("Name"), ch("Body", blk)}},
	{AbiBlock, "AbiBlock", layLR, "Kw", []slot{lst("Abis", set(Ident)), lst("Decls", declOrStmt)}},
	{ContractBlock, "ContractBlock", layFlagR, "Head", []slot{num("Mods"), ch("Body", lists)}},
	{InstantiateDecl, "InstantiateDecl", layTokR, "Name", []slot{tk("Kw"), ch("Type", typeSet)}},
	{LayoutClause, "LayoutClause", layL, "Kw", []slot{lst("Args", exprSet.or(set(FieldInit)))}},
	{Attr, "Attr", layLR, "At", []slot{ch("Path", set(Path)), olst("Args", argSet)}},
	{UnsafeGuarantee, "UnsafeGuarantee", layList, "Head", []slot{items(set(UGRule))}},
	{UGRule, "UGRule", layL, "Property", []slot{olst("Requirements", set(UGRequirement))}},
	{UGRequirement, "UGRequirement", layL, "Param", []slot{lst("Properties", set(Ident))}},
	{Visibility, "Visibility", layTokR, "Pub", []slot{otk("Qualifier"), och("Path", set(Path))}},
}
