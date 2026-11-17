package printer_test

import (
	"testing"

	"kigumi/internal/compiler/printer"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func TestShebangAfterBlankLine(t *testing.T) {
	src := "\n#!/usr/bin/env kigumi\nimport text from std/text\nprint text.fromChar(0)\n"
	format := func(src string) string {
		tree := syntax.Parse(token.NewFile("script.kg", []byte(src)))
		if len(tree.Diags) > 0 {
			t.Fatal(tree.Diags)
		}
		return printer.Print(tree)
	}
	once := format(src)
	if twice := format(once); once != twice {
		t.Fatalf("first:\n%s\nsecond:\n%s", once, twice)
	}
}
