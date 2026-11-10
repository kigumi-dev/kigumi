package syntax

var patSchema = []kindSchema{
	{PatWildcard, "PatWildcard", layLeaf, "Name", nil},
	{PatBind, "PatBind", layLeaf, "Name", nil},
	{PatLit, "PatLit", layL, "Head", []slot{ch("Lit", literalSet.or(set(Unary)))}},
	{PatCtor, "PatCtor", layLR, "Head", []slot{ch("Path", set(Path)), olst("Args", patSet)}},
	{PatRecord, "PatRecord", layLR, "Head", []slot{ch("Path", set(Path)), lst("Fields", set(PatField))}},
	{PatOr, "PatOr", layList, "Head", []slot{items(patSet)}},
	{PatField, "PatField", layL, "Name", []slot{och("Sub", patSet)}},
	{PatTuple, "PatTuple", layList, "Open", []slot{items(patSet)}},
	{PatRange, "PatRange", layLR, "Op", []slot{ch("Lo", rangeBound), ch("Hi", rangeBound)}},
}

var shellSchema = []kindSchema{
	{ShellPlan, "ShellPlan", layList, "Str", []slot{items(set(ShellCmd))}},
	{ShellCmd, "ShellCmd", layList, "Str", []slot{items(set(ShellWord, ShellRedirect))}},
	{ShellWord, "ShellWord", layList, "Str", []slot{items(set(ShellText, ShellInterp))}},
	{ShellText, "ShellText", layRec, "", []slot{num("Start"), num("End"), numAs("Quote", "ShellQuote")}},
	{ShellInterp, "ShellInterp", layL, "Str", []slot{ch("Expr", exprSet)}},
	{ShellRedirect, "ShellRedirect", layRec, "Str", []slot{num("Op"), num("Fd"), och("Target", set(ShellWord))}},
}
