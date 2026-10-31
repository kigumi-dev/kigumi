package syntax

const (
	File NodeKind = exprKindEnd + iota
	FnDecl
	Param
	GenericParam
	TypeDecl
	RecordBody
	ResourceBody
	AdtBody
	AliasBody
	// DefinedBody is the plain `= TypeExpr` type declaration body: a nominal
	// defined type over TypeExpr, as opposed to AliasBody's `= alias TypeExpr`.
	DefinedBody
	Field
	Variant
	VariantField
	Metadata
	MetaItem
	InterfaceDecl
	ConstDecl
	StaticDecl
	ContextKeyDecl
	ImportDecl
	ImportAlias
	TestDecl
	AbiBlock
	ContractBlock
	InstantiateDecl
	LayoutClause
	Attr
	// UnsafeGuarantee is `@unsafeGuarantee(Rule, ...)`: a list of UGRule.
	UnsafeGuarantee
	// UGRule is one `Property ["when" Requirement ("&&" Requirement)*]`;
	// Tok is the property name, Lhs the requirement List (0 if unconditional).
	UGRule
	// UGRequirement is `TypeParam ":" Property ("+" Property)*`; Tok is the
	// type parameter name, Lhs the List of property Ident leaves.
	UGRequirement
	Visibility

	LetStmt
	ExprStmt
	AssignStmt
	DeferStmt
	DeferFailStmt

	List

	TypePath
	// TypeLifetime is a `'a` used as a generic type argument;
	// as a reference type's own lifetime tag it lives on TypeRef instead.
	TypeLifetime
	TypeOptional
	TypeResult
	TypeFn
	TypePtr
	TypeRef
	TypeTuple
	Path

	kindEnd
)

var declKindNames = [...]string{
	"File", "FnDecl", "Param", "GenericParam", "TypeDecl", "RecordBody",
	"ResourceBody", "AdtBody", "AliasBody", "DefinedBody", "Field", "Variant", "VariantField",
	"Metadata", "MetaItem", "InterfaceDecl", "ConstDecl", "StaticDecl", "ContextKeyDecl", "ImportDecl", "ImportAlias", "TestDecl",
	"AbiBlock", "ContractBlock", "Instantiate", "Layout", "Attr",
	"UnsafeGuarantee", "UGRule", "UGRequirement", "Visibility",
	"Let", "ExprStmt", "Assign", "Defer", "DeferFail",
	"List",
	"TypePath", "TypeLifetime", "TypeOptional", "TypeResult", "TypeFn", "TypePtr", "TypeRef", "TypeTuple", "Path",
}
