package ir

import (
	"strconv"
	"strings"
	"testing"
)

// TestBuilderRendersRepresentativeModule pins the exact text the builder
// produces for a small mix of calls and a descriptor global, including the
// TObjSlot storage that rt.h must agree with.
func TestBuilderRendersRepresentativeModule(t *testing.T) {
	rtUnit := RuntimeFn{Name: "rt_unit", Ret: TPtr}
	rtInt := RuntimeFn{Name: "rt_int", Params: []Type{TI64, TI32}, Ret: TPtr}
	rtPanicIndex := RuntimeFn{Name: "rt_panic_index", Params: []Type{TI64, TI64, TPtr}}

	var body strings.Builder
	tmps := 0
	var noted []string
	b := NewBuilder(&body, func() string {
		tmps++
		return "%t" + strconv.Itoa(tmps)
	}, func(fn RuntimeFn) { noted = append(noted, fn.Name) })

	b.Call(rtUnit)
	v := b.Call(rtInt, Value{Type: TI64, Text: "7"}, Value{Type: TI32, Text: "0"})
	b.CallAs("%idx", rtPanicIndex, Value{Type: TI64, Text: v.Text}, Value{Type: TI64, Text: "3"}, Value{Type: TPtr, Text: "null"})
	st := StructType{Name: "%rt_variant", Fields: []Type{TPtr, TPtr, TI32, TI32, TObjSlot}}
	b.GlobalStruct("@desc", st,
		Value{Type: TPtr, Text: "null"}, Value{Type: TPtr, Text: "null"}, Value{Type: TI32, Text: "1"}, Value{Type: TI32, Text: "0"}, Value{Type: TObjSlot, Text: "zeroinitializer"})

	want := `  %t1 = call ptr @rt_unit()
  %t2 = call ptr @rt_int(i64 7, i32 0)
  %idx = call void @rt_panic_index(i64 %t2, i64 3, ptr null)
@desc = global %rt_variant { ptr null, ptr null, i32 1, i32 0, [64 x i8] zeroinitializer }
`
	if got := body.String(); got != want {
		t.Errorf("builder output:\n%s\nwant:\n%s", got, want)
	}
	if strings.Join(noted, ",") != "rt_unit,rt_int,rt_panic_index" {
		t.Errorf("noted calls = %v", noted)
	}
}

func TestBuilderRejectsBadCalls(t *testing.T) {
	fn := RuntimeFn{Name: "rt_pair", Params: []Type{TPtr, TI64}, Ret: TVoid}
	for _, tc := range []struct {
		name string
		call func(b Builder)
	}{
		{"arity", func(b Builder) { b.Call(fn, Value{Type: TPtr, Text: "null"}) }},
		{"type", func(b Builder) { b.Call(fn, Value{Type: TPtr, Text: "null"}, Value{Type: TI32, Text: "1"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("bad call did not panic")
				}
			}()
			var sb strings.Builder
			tc.call(NewBuilder(&sb, func() string { return "%t" }, nil))
		})
	}
}

func TestRenderExterns(t *testing.T) {
	fns := []RuntimeFn{
		{Name: "rt_unit", Ret: TPtr},
		{Name: "rt_len", Params: []Type{TPtr}, Ret: TI64},
		{Name: "rt_panic", Params: []Type{TPtr}, Ret: TVoid},
	}
	want := "declare ptr @rt_unit()\ndeclare i64 @rt_len(ptr)\ndeclare void @rt_panic(ptr)\n\n"
	if got := RenderExterns(fns); got != want {
		t.Errorf("RenderExterns:\n%s\nwant:\n%s", got, want)
	}
}

func TestTypeStrings(t *testing.T) {
	for _, tc := range []struct {
		t    Type
		want string
	}{
		{TVoid, "void"},
		{TPtr, "ptr"},
		{TCPtr, "ptr"},
		{TI1, "i1"},
		{TI8, "i8"},
		{TI16, "i16"},
		{TI32, "i32"},
		{TI64, "i64"},
		{TI128, "i128"},
		{TDouble, "double"},
		{TFloat, "float"},
		{TObjSlot, "[64 x i8]"},
	} {
		if got := tc.t.String(); got != tc.want {
			t.Errorf("%d.String() = %q, want %q", tc.t, got, tc.want)
		}
	}
	if ObjSlotBytes != 64 {
		t.Errorf("ObjSlotBytes = %d, want 64 (RT_OBJ_SLOT)", ObjSlotBytes)
	}
	if got := (Value{Type: TI32, Text: "7"}).Render(); got != "i32 7" {
		t.Errorf("Value.Render() = %q", got)
	}
}
