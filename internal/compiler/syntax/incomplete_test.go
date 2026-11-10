package syntax

import "testing"

func TestIncomplete(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		// Complete, valid input.
		{name: "empty", src: ""},
		{name: "whitespace only", src: "  \n\t\n"},
		{name: "comment only", src: "// a note\n"},
		{name: "let int", src: "let x = 1"},
		{name: "let call", src: "let x = foo(1, 2)"},
		{name: "let trailing comma before close", src: "let xs = [1, 2,]"},
		{name: "binary expression", src: "let x = 1 + 2 * 3"},
		{name: "pipeline", src: "let f = x |> g |> h"},
		{name: "fn body", src: "fn f() {}"},
		{name: "fn body multiline", src: "fn f() {\n\tlet x = 1\n}\n"},
		{name: "if else", src: "if x { 1 } else { 2 }"},
		{name: "match", src: "match x { 1 => 2, _ => 3 }"},
		{name: "char literal", src: "let c = 'a'"},
		{name: "fallible type", src: "fn f() -> T!"},
		{name: "fallible binding", src: "let t: T! = y"},
		{name: "try postfix", src: "x?"},
		{name: "return", src: "return"},
		{name: "multiline call", src: "let x = foo(\n\t1,\n)\n"},
		{name: "braces in string", src: `let s = "a{b[c(}"`},
		{name: "comment text in string", src: `let s = "/* not a comment */"`},
		{name: "block comment with delimiters", src: "/* {[( */ let x = 1"},
		{name: "line comment with delimiters", src: "// {[(\nlet x = 1"},
		{name: "interpolation", src: `let s = "${f({a: 1})}"`},
		{name: "interpolation braces", src: `let s = "${{}}"`},

		// Unterminated literals and comments.
		{name: "unterminated string", src: `let s = "abc`, want: true},
		{name: "unterminated byte string", src: `let s = b"abc`, want: true},
		{name: "unterminated interpolation", src: `let s = "${x +`, want: true},
		{name: "unterminated interpolation string quirk", src: `let s = "${x"`, want: true},
		{name: "unterminated char", src: "let c = '1", want: true},
		{name: "unterminated char in expression", src: "let c = 'a", want: true},
		{name: "unterminated block comment", src: "/* unfinished", want: true},
		{name: "unterminated nested block comment", src: "/* a /* nested */", want: true},

		// Unclosed delimiters.
		{name: "open brace", src: "let x = {", want: true},
		{name: "open paren", src: "f(1, 2", want: true},
		{name: "open bracket", src: "let xs = [1, 2", want: true},
		{name: "open brace multiline", src: "if c {\n\tx\n", want: true},
		{name: "match open", src: "match x {", want: true},
		{name: "interface open", src: "type I interface {", want: true},

		// Trailing operators and punctuation.
		{name: "trailing pipe", src: "x |>", want: true},
		{name: "trailing plus", src: "x +", want: true},
		{name: "trailing and", src: "a &&", want: true},
		{name: "trailing assign", src: "let x =", want: true},
		{name: "trailing compound assign", src: "x +=", want: true},
		{name: "trailing comma", src: "f(a,", want: true},
		{name: "trailing colon", src: "let x:", want: true},
		{name: "trailing dot", src: "let y = t.", want: true},
		{name: "trailing range", src: "for i in 0..", want: true},
		{name: "trailing arrow", src: "fn f() ->", want: true},
		{name: "trailing fat arrow", src: "match x { 1 =>", want: true},
		{name: "trailing tilde", src: "x ~", want: true},

		// Trailing keywords that need a following construct.
		{name: "trailing fn", src: "fn", want: true},
		{name: "trailing if", src: "if", want: true},
		{name: "trailing else", src: "if c {} else", want: true},
		{name: "trailing let", src: "let", want: true},
		{name: "trailing match", src: "match", want: true},

		// Declarations that have not reached their initializer.
		{name: "let without value", src: "let x", want: true},
		{name: "let mut without value", src: "let mut x", want: true},
		{name: "const without value", src: "const X", want: true},
		{name: "type without body", src: "type X", want: true},

		// Malformed but not unfinished: let the parser report a real error.
		{name: "mismatched close", src: "let x = )"},
		{name: "stray close brace", src: "}"},
		{name: "missing pattern", src: "let = 3"},
		{name: "mismatched inside expression", src: "let x = 1 + )"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := Incomplete([]byte(tt.src))
			if got != tt.want {
				t.Fatalf("Incomplete(%q) = %v, %q; want %v", tt.src, got, reason, tt.want)
			}
			if tt.want && reason == "" {
				t.Fatalf("Incomplete(%q) = true with empty reason", tt.src)
			}
			if !tt.want && reason != "" {
				t.Fatalf("Incomplete(%q) = false with reason %q", tt.src, reason)
			}
		})
	}
}
