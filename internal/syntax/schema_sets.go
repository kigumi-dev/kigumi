package syntax

var (
	lists = set(List)

	literalSet = set(IntLit, FloatLit, CharLit, BoolLit, StringLit, ByteStringLit)

	exprSet = set(Ident, ShellLit, Paren, Unary, Binary, IsExpr, CallExpr, WsCallExpr,
		BracketExpr, MemberExpr, OptMemberExpr, TryExpr, RecordLit, TupleLit, UnitLit,
		Lambda, Block, IfExpr, IfLetExpr, MatchExpr, ForExpr, ReturnExpr, FailExpr,
		BreakExpr, ContinueExpr, AwaitExpr, ComptimeExpr, UnsafeExpr, AllocatorExpr,
		ContextExpr, ContractExpr, BorrowExpr, MoveExpr, AsmExpr).or(literalSet)

	// typeSet includes the literals a const generic argument is written with.
	typeSet = set(TypePath, TypeLifetime, TypeOptional, TypeResult, TypeFn, TypePtr,
		TypeRef, TypeTuple, Paren, IntLit, BoolLit)

	patSet = set(PatWildcard, PatBind, PatLit, PatCtor, PatRecord, PatOr, PatTuple, PatRange)

	stmtSet = set(LetStmt, ExprStmt, AssignStmt, DeferStmt, ErrdeferStmt)

	declSet = set(FnDecl, TypeDecl, InterfaceDecl, ConstDecl, StaticDecl, ContextKeyDecl,
		ImportDecl, TestDecl, AbiBlock, ContractBlock, InstantiateDecl)

	argSet     = exprSet.or(set(NamedArg, Spread))
	exprOrType = exprSet.or(typeSet)
	rangeBound = literalSet.or(set(Ident, Unary))
	declOrStmt = declSet.or(stmtSet)
)
