package syntax

var stmtSchema = []kindSchema{
	{LetStmt, "Let", layRec, "Kw", []slot{
		num("Flags"), ch("Pattern", patSet), och("Type", typeSet), ch("Init", exprSet), och("Else", blk),
	}},
	{ExprStmt, "ExprStmt", layL, "Head", []slot{ch("Expr", exprSet)}},
	{AssignStmt, "Assign", layLR, "Op", []slot{ch("Target", exprSet), ch("Value", exprSet)}},
	{DeferStmt, "Defer", layL, "Kw", []slot{ch("Body", stmtSet.or(blk))}},
	{ErrdeferStmt, "Errdefer", layL, "Kw", []slot{ch("Body", stmtSet.or(blk))}},
	{List, "List", layList, "Open", []slot{items(anyKind)}},
}
