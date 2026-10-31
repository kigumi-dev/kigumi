// Package syntax lexes and parses source files into a flat, lossless tree.
//
// Parse always returns a Tree; errors are collected in Tree.Diags and the
// parser resynchronizes at statement boundaries. Nodes reference children and
// tokens by index only, and token index 0 is a zero-width BOF sentinel so that
// 0 can mean "absent" in every slot. Semantic passes keep their results in
// side tables keyed by NodeID and never write into the tree.
//
// Kinds, layouts and named slots are declared once in schema*.go, which
// generates views_gen_*.go; other packages read the tree only through views.
//
// Ranges `a..b` are Binary nodes whose operator token is DotDot. A ForExpr
// stores the condition or the iterable in its "head" slot; "pattern" is 0 for
// the condition and infinite forms. Receiver type parameters of
// `fn Box[T].value` are GenericParams in the "generics" slot and the receiver
// TypePath refers to them by name.
//
// Helpers: View.Children walks children in slot order, StringParts splits an
// interpolated string, DecodeString/DecodeChar/ParseInt/ParseFloat decode
// literals, VisKind decodes a Visibility node, HasErrors and
// HasTopLevelStatements summarize a file, Dump renders the tree for golden
// tests.
package syntax
