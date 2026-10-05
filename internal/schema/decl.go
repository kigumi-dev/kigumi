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
}

// Variant is one ADT constructor and its named payload.
type Variant struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Payload []*Member `json:"payload,omitempty"`
}

// Metadata is one resolved item (`json.name("id")`); SchemaID is the
// resolved schema's stable identity, never re-derived from Name.
type Metadata struct {
	SchemaID string `json:"schemaId"`
	Name     string `json:"name"`
	Args     []any  `json:"args,omitempty"`
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
