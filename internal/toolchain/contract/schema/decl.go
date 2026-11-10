package schema

// Kind discriminates a top-level Schema IR declaration.
type Kind string

const (
	KindFn        Kind = "fn"
	KindMethod    Kind = "method"
	KindType      Kind = "type"
	KindConst     Kind = "const"
	KindStatic    Kind = "static"
	KindInterface Kind = "interface"
)

// Visibility mirrors sem.VisLevel as a stable, versioned string instead of
// a small integer a future level could renumber.
type Visibility string

const (
	VisPrivate Visibility = "private"
	VisSuper   Visibility = "super"
	VisModule  Visibility = "module"
	VisPub     Visibility = "pub"
)

// Module is one query's result: every declaration of its Envelope.Scope,
// in canonical (source) order.
type Module struct {
	Envelope     Envelope       `json:"envelope"`
	Declarations []*Declaration `json:"declarations"`
}

// Declaration is one nominal declaration: a function, method, type,
// interface, const or static.
type Declaration struct {
	ID           string              `json:"id"`
	Package      string              `json:"package"`
	Kind         Kind                `json:"kind"`
	Name         string              `json:"name"`
	Visibility   Visibility          `json:"visibility"`
	Display      string              `json:"display"`
	Doc          string              `json:"doc,omitempty"`
	Metadata     []Metadata          `json:"metadata,omitempty"`
	Params       []*Member           `json:"params,omitempty"`
	Fields       []*Member           `json:"fields,omitempty"`
	Variants     []*Variant          `json:"variants,omitempty"`
	Contracts    *Contracts          `json:"contracts,omitempty"`
	Origin       Origin              `json:"origin"`
	SchemaDecl   *SchemaDecl         `json:"schemaDecl,omitempty"`
	Availability *TargetAvailability `json:"availability,omitempty"`
	// Return is a function or method's result type; nil means Void or Never.
	Return *TypeRef `json:"return,omitempty"`
	// Form names a type declaration's shape ("record", "resource", "adt",
	// "opaque", "defined"); empty for a non-type.
	Form string `json:"form,omitempty"`
	// Underlying is a defined type's representation (`type A = T`).
	Underlying *TypeRef `json:"underlying,omitempty"`
	// Layout is a `layout(...)` tag: "C" or "packed"; empty otherwise.
	Layout string `json:"layout,omitempty"`
	// Align is `layout(C, align: N)`'s N, 0 when unset.
	Align int64 `json:"align,omitempty"`
	// Opaque marks a foreign type a `extern(...)` block declares with no
	// body: C sees an incomplete struct, never its fields.
	Opaque bool `json:"opaque,omitempty"`
	// ABI marks a declaration placed in an `extern(...)` or `export(...)`
	// block; nil for a Kigumi-only declaration.
	ABI *ABI `json:"abi,omitempty"`
}

// ABI names the foreign block a declaration belongs to and the external
// details a binding generator needs.
type ABI struct {
	// Block is "extern" or "export".
	Block string `json:"block"`
	// Name is the ABI name written in the block: "C", or "interrupt".
	Name string `json:"name"`
	// Symbol is the external spelling (`@linkName` or the source name).
	Symbol string `json:"symbol,omitempty"`
	// Naked marks `export(C, naked)`: a body made of one asm block.
	Naked bool `json:"naked,omitempty"`
	// Variadic marks a C `...` signature.
	Variadic bool `json:"variadic,omitempty"`
}

// TargetAvailability is a declaration's platform-file tag; nil means
// every target. OS/Arch can also hold the pseudo-names "hosted"/"bare".
type TargetAvailability struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}

// Member is one parameter, field or variant payload slot.
type Member struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     *TypeRef   `json:"type"`
	Metadata []Metadata `json:"metadata,omitempty"`
	// Const marks a `const N: T` type parameter: a derive generator's
	// generic binder gives it no protocol bound, unlike a type parameter.
	Const bool `json:"const,omitempty"`
	// Escaping marks a callback parameter the callee may keep after it returns:
	// declared `escaping`, or implied by async, extern or a bodiless std function.
	Escaping bool `json:"escaping,omitempty"`
	// Copy mirrors sem's Copy classification for a variant payload slot; a
	// derive generator uses it to pick a pattern binding's mode (value vs reference).
	Copy bool `json:"copy,omitempty"`
}

// Variant is one ADT constructor and its named payload.
type Variant struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Payload []*Member `json:"payload,omitempty"`
}

// Metadata is one item (`json.name("id")`); SchemaID is the resolved
// schema's stable identity, never re-derived from Name. Unresolved marks an
// item the checker could not attribute to a `@schema` function: SchemaID is
// then empty and a generator must not treat the item as declared metadata.
type Metadata struct {
	SchemaID   string `json:"schemaId"`
	Name       string `json:"name"`
	Args       []any  `json:"args,omitempty"`
	Unresolved bool   `json:"unresolved,omitempty"`
}

// SchemaDecl marks a type declared `@schema`: the identity other
// declarations' Metadata.SchemaID points at.
type SchemaDecl struct {
	Repeatable bool   `json:"repeatable"`
	Ordered    bool   `json:"ordered"`
	Target     string `json:"target,omitempty"`
}

// Origin locates a declaration's source; Generated marks a derive artifact.
type Origin struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Generated bool   `json:"generated"`
}
