package llgen

import "testing"

func TestMarkNoUnwindTagsOnlyDefines(t *testing.T) {
	in := "declare void @f()\ndefine ptr @g(ptr %p) {\nentry:\n  ret ptr %p\n}\ndefine internal void @h() {\n  ret void\n}\n"
	want := "declare void @f()\ndefine ptr @g(ptr %p) nounwind {\nentry:\n  ret ptr %p\n}\ndefine internal void @h() nounwind {\n  ret void\n}\n"
	if got := markNoUnwind(in); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
