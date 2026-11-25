package module

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// LockHeader is the mandatory first record of a lock. Kind says whether the
// file describes one module (or script) or a workspace; format selects the
// codec; resolver names the selection meaning; inputs is the digest of the
// root/member/local resolution inputs.
type LockHeader struct {
	Kind     LockKind
	Format   int
	Resolver string
	Inputs   string
}

// LockFormatCodec is one format's strict reader and canonical writer. A
// reader is never used to try an unknown format: the header's format selects
// exactly one codec.
type LockFormatCodec struct {
	Format int
	Decode func(name string, kind LockKind, header LockHeader, src []byte) (LockState, error)
	Encode func(state LockState) ([]byte, error)
}

// LockCodecs is a format registry. Production builds register only the
// published format; tests register synthetic future formats without exposing
// them to commands.
type LockCodecs struct {
	byFormat map[int]LockFormatCodec
}

// NewLockCodecs returns an empty format registry.
func NewLockCodecs() *LockCodecs {
	return &LockCodecs{byFormat: map[int]LockFormatCodec{}}
}

// Register adds a format codec. A format may have only one codec, and the
// callbacks must be present; otherwise the registry would be ambiguous.
func (c *LockCodecs) Register(codec LockFormatCodec) error {
	if c.byFormat == nil {
		c.byFormat = map[int]LockFormatCodec{}
	}
	if codec.Format <= 0 {
		return fmt.Errorf("lock codec format must be positive, got %d", codec.Format)
	}
	if codec.Decode == nil || codec.Encode == nil {
		return fmt.Errorf("lock codec for format %d needs both Decode and Encode", codec.Format)
	}
	if _, dup := c.byFormat[codec.Format]; dup {
		return fmt.Errorf("lock codec for format %d is registered twice", codec.Format)
	}
	c.byFormat[codec.Format] = codec
	return nil
}

// Decode parses src as kind, using only the codec named by the header.
func (c *LockCodecs) Decode(name string, kind LockKind, src []byte) (LockState, error) {
	if !kind.Valid() {
		return LockState{}, fmt.Errorf("%w: %d", ErrLockKindMismatch, uint8(kind))
	}
	header, err := parseLockHeader(name, src)
	if err != nil {
		return LockState{}, err
	}
	if header.Kind != kind {
		return LockState{}, fmt.Errorf("%s: %w: header is %s, caller expects %s", name, ErrLockKindMismatch, header.Kind, kind)
	}
	codec, ok := c.byFormat[header.Format]
	if !ok {
		return LockState{}, fmt.Errorf("%s: %w: %d", name, ErrLockUnsupportedFormat, header.Format)
	}
	state, err := codec.Decode(name, kind, header, src)
	if err != nil {
		return LockState{}, err
	}
	if state.Kind != header.Kind || state.Format != header.Format || state.Resolver != header.Resolver || state.Inputs != header.Inputs {
		return LockState{}, fmt.Errorf("%s: %w: codec changed the header", name, ErrLockInvalid)
	}
	return state, nil
}

// Encode writes state in the codec registered for its format.
func (c *LockCodecs) Encode(state LockState) ([]byte, error) {
	if !state.Kind.Valid() {
		return nil, fmt.Errorf("%w: %d", ErrLockKindMismatch, uint8(state.Kind))
	}
	codec, ok := c.byFormat[state.Format]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrLockUnsupportedFormat, state.Format)
	}
	return codec.Encode(state)
}

// defaultLockCodecs is the production registry: the published first format
// only. A later format must be added with its own codec and golden fixtures.
var defaultLockCodecs = func() *LockCodecs {
	c := NewLockCodecs()
	if err := c.Register(lockFormatCodec()); err != nil {
		panic(err)
	}
	return c
}()

// DecodeLock decodes a published-format (or registered later-format) lock.
// name is used in diagnostics, usually the file path or `mod.lock.kg`. A
// headerless or otherwise unrecognized file is an error; no other decoder is
// tried.
func DecodeLock(name string, kind LockKind, src []byte) (LockState, error) {
	return defaultLockCodecs.Decode(name, kind, src)
}

// EncodeLock writes a lock canonically: header first, then Source,
// Resolution and Lock records, each group sorted, so the same input and
// snapshot produce the same bytes.
func EncodeLock(state LockState) ([]byte, error) {
	return defaultLockCodecs.Encode(state)
}

// ReadLockStateFile reads and decodes a lock without writing anything. A
// missing file, an empty file and a valid zero-dependency header are three
// distinct results; an empty file is never treated as an empty graph.
func ReadLockStateFile(path string, kind LockKind) (LockState, error) {
	src, ok, err := readFileMaybe(path)
	if err != nil {
		return LockState{}, err
	}
	if !ok {
		return LockState{}, fmt.Errorf("%s: %w", path, ErrLockMissing)
	}
	return DecodeLock(path, kind, src)
}

func firstRecordKind(t *syntax.Tree) (string, bool) {
	root := t.List(t.Root)
	if root.Len() == 0 {
		return "", false
	}
	st := root.At(0)
	if st.Kind() != syntax.ExprStmt || st.AsExprStmt().Expr().Kind() != syntax.RecordLit {
		return "", false
	}
	head := st.AsExprStmt().Expr().AsRecordLit().Head()
	if head.Kind() != syntax.Ident {
		return "", false
	}
	return head.Tok().Text(), true
}

func parseLockHeader(name string, src []byte) (LockHeader, error) {
	if len(bytes.TrimSpace(src)) == 0 {
		return LockHeader{}, fmt.Errorf("%s: %w", name, ErrLockEmpty)
	}
	f := token.NewFile(name, src)
	t := syntax.Parse(f)
	root := t.List(t.Root)
	firstKind, hasFirst := firstRecordKind(t)
	if !hasFirst || (firstKind != "ModuleLock" && firstKind != "WorkspaceLock") {
		if len(t.Diags) > 0 {
			return LockHeader{}, fmt.Errorf("%s: %w: %s", name, ErrLockInvalid, diag.RenderAll(f, t.Diags[:1]))
		}
		if !hasFirst {
			return LockHeader{}, fmt.Errorf("%s: %w", name, ErrLockMissingHeader)
		}
		return LockHeader{}, fmt.Errorf("%s: %w: first record is %s, expected ModuleLock or WorkspaceLock", name, ErrLockMissingHeader, firstKind)
	}
	recs, ds := recordsOf(t, root, 1, lockHeaderFields)
	ds = append(append([]diag.Diagnostic{}, t.Diags...), ds...)
	if len(ds) > 0 {
		return LockHeader{}, fmt.Errorf("%s: %w: %s", name, ErrLockInvalid, diag.RenderAll(f, ds[:1]))
	}
	if len(recs) == 0 {
		return LockHeader{}, fmt.Errorf("%s: %w", name, ErrLockMissingHeader)
	}
	rec := recs[0]
	kind, err := ParseLockKind(rec.Kind)
	if err != nil {
		return LockHeader{}, fmt.Errorf("%s: %w: first record is %s, expected ModuleLock or WorkspaceLock", name, ErrLockMissingHeader, rec.Kind)
	}
	formatText, ok := rec.Fields["format"]
	if !ok {
		return LockHeader{}, fmt.Errorf("%s: %w: header is missing format", name, ErrLockInvalid)
	}
	format, err := parseLockFormatText(formatText)
	if err != nil {
		return LockHeader{}, fmt.Errorf("%s: %w: %v", name, ErrLockInvalid, err)
	}
	resolver, ok := rec.Fields["resolver"]
	if !ok || resolver == "" {
		return LockHeader{}, fmt.Errorf("%s: %w: header is missing resolver", name, ErrLockInvalid)
	}
	if resolver != LockResolverOrderedV1 {
		return LockHeader{}, fmt.Errorf("%s: %w: %q", name, ErrLockUnknownResolver, resolver)
	}
	inputs, ok := rec.Fields["inputs"]
	if !ok {
		return LockHeader{}, fmt.Errorf("%s: %w: header is missing inputs", name, ErrLockInvalid)
	}
	if !ValidLockH1(inputs) {
		return LockHeader{}, fmt.Errorf("%s: %w: inputs %q is not an h1: SHA-256 digest", name, ErrLockInvalid, inputs)
	}
	return LockHeader{Kind: kind, Format: format, Resolver: resolver, Inputs: inputs}, nil
}

func parseLockFormatText(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("format is empty")
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("format %q has a leading zero", s)
	}
	if s[0] == '+' || s[0] == '-' {
		return 0, fmt.Errorf("format %q is not a positive decimal integer", s)
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("format %q is not a positive decimal integer", s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("format %q is not a positive decimal integer that fits", s)
	}
	return n, nil
}

// SortedFormatList is a diagnostic helper: the formats a registry can read.
func (c *LockCodecs) SortedFormatList() []int {
	out := make([]int, 0, len(c.byFormat))
	for format := range c.byFormat {
		out = append(out, format)
	}
	sort.Ints(out)
	return out
}
